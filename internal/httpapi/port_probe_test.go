package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
)

// TestProbeOnePort_Privileged 验证 < 1024 端口直接返回 "privileged"，不实际探测。
func TestProbeOnePort_Privileged(t *testing.T) {
	res := probeOnePort(context.Background(), 22)
	if res.Available {
		t.Error("port 22 should not be available")
	}
	if res.Reason != "privileged" {
		t.Errorf("reason = %q, want privileged", res.Reason)
	}
}

// TestProbeOnePort_Available 先 Listen 拿到 port → 立即 Close 释放 → 探它 → available。
func TestProbeOnePort_Available(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close() // 释放

	res := probeOnePort(context.Background(), port)
	if !res.Available {
		t.Errorf("port %d should be available, got %+v", port, res)
	}
}

// TestProbeOnePort_Occupied Listen 一个 port 不释放 → 探它 → unavailable。
func TestProbeOnePort_Occupied(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	res := probeOnePort(context.Background(), port)
	if res.Available {
		t.Errorf("port %d should NOT be available, got %+v", port, res)
	}
	if res.Reason != "in_use" {
		t.Errorf("reason = %q, want in_use", res.Reason)
	}
}

// TestProbePorts_Handler_HappyPath 整端到端：发 [22, 9999]，22 是 privileged、
// 9999 大概率可用（环境依赖）。
func TestProbePorts_Handler_HappyPath(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	// 找到一个可用端口
	ln, _ := net.Listen("tcp", ":0")
	availPort := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	body := map[string]any{"ports": []int{22, availPort}}
	resp, raw := doJSON(t, srv, "POST", "/api/v1/system/probe-ports", body, cookies, csrf)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody=%s", resp.StatusCode, raw)
	}
	var got PortProbeResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 2 {
		t.Fatalf("results len = %d, want 2", len(got.Results))
	}
	if got.Results[0].Port != 22 || got.Results[0].Available || got.Results[0].Reason != "privileged" {
		t.Errorf("port 22 result = %+v", got.Results[0])
	}
	if got.Results[1].Port != availPort {
		t.Errorf("port[1] = %d, want %d", got.Results[1].Port, availPort)
	}
}

// TestProbePorts_OutOfRange 端口 > 65535 → 422。
func TestProbePorts_OutOfRange(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)
	body := map[string]any{"ports": []int{70000}}
	resp, _ := doJSON(t, srv, "POST", "/api/v1/system/probe-ports", body, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", resp.StatusCode)
	}
}

// TestProbePorts_TooMany 超 64 个端口 → 422。
func TestProbePorts_TooMany(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)
	ports := make([]int, 65)
	for i := range ports {
		ports[i] = 50000 + i
	}
	body := map[string]any{"ports": ports}
	resp, raw := doJSON(t, srv, "POST", "/api/v1/system/probe-ports", body, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422\nbody=%s", resp.StatusCode, raw)
	}
}

// TestProbePorts_EmptyList 空数组 → 200 + results 空。
func TestProbePorts_EmptyList(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)
	body := map[string]any{"ports": []int{}}
	resp, raw := doJSON(t, srv, "POST", "/api/v1/system/probe-ports", body, cookies, csrf)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody=%s", resp.StatusCode, raw)
	}
	var got PortProbeResponse
	_ = json.Unmarshal(raw, &got)
	if len(got.Results) != 0 {
		t.Errorf("results len = %d, want 0", len(got.Results))
	}
}

// TestProbePorts_Dedup 含重复端口 → 去重，results 长度 1。
func TestProbePorts_Dedup(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)
	body := map[string]any{"ports": []int{80, 80, 80}}
	resp, raw := doJSON(t, srv, "POST", "/api/v1/system/probe-ports", body, cookies, csrf)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got PortProbeResponse
	_ = json.Unmarshal(raw, &got)
	if len(got.Results) != 1 {
		t.Errorf("results len = %d, want 1 (dedup)", len(got.Results))
	}
}

// TestProbePorts_ExtraFieldsIgnored 请求里加 host 字段应被忽略（FR-C.3.7 / AC-C.3.7）。
// 由于 PortProbeRequest 没有 host 字段，json 解码就忽略；这里仅断言不出错。
func TestProbePorts_ExtraFieldsIgnored(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)
	// 用 raw 字符串发包，把 host 字段塞进去
	body := []byte(`{"ports":[55555],"host":"8.8.8.8"}`)
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/system/probe-ports",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (host should be silently ignored)", resp.StatusCode)
	}
}

// TestProbePorts_Unauthenticated 未登录 → 401。
func TestProbePorts_Unauthenticated(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	body := map[string]any{"ports": []int{55555}}
	resp, _ := doJSON(t, srv, "POST", "/api/v1/system/probe-ports", body, nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// TestProbePorts_NoCSRF 已登录但无 CSRF → 403。
func TestProbePorts_NoCSRF(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, _ := setupAndLogin(t, srv)
	body := map[string]any{"ports": []int{55555}}
	resp, _ := doJSON(t, srv, "POST", "/api/v1/system/probe-ports", body, cookies, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

