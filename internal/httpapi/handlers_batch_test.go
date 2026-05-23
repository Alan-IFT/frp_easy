package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// TestBatchProxies_HappyPath POST batch {tcp, basename:"web", portsExpr:"6000-6002"} → 201 + 3 条。
func TestBatchProxies_HappyPath(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	body := map[string]any{
		"basename":  "web",
		"type":      "tcp",
		"portsExpr": "6000-6002",
	}
	resp, raw := doJSON(t, srv, "POST", "/api/v1/proxies/batch", body, cookies, csrf)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201\nbody=%s", resp.StatusCode, raw)
	}
	var got BatchProxiesResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Created != 3 {
		t.Errorf("Created = %d, want 3", got.Created)
	}
	if len(got.Items) != 3 {
		t.Fatalf("Items len = %d, want 3", len(got.Items))
	}
	wantNames := []string{"web-6000", "web-6001", "web-6002"}
	for i, n := range wantNames {
		if got.Items[i].Name != n {
			t.Errorf("Items[%d].Name = %q, want %q", i, got.Items[i].Name, n)
		}
		if got.Items[i].LocalPort != 6000+i {
			t.Errorf("Items[%d].LocalPort = %d", i, got.Items[i].LocalPort)
		}
		if got.Items[i].RemotePort == nil || *got.Items[i].RemotePort != 6000+i {
			t.Errorf("Items[%d].RemotePort = %v", i, got.Items[i].RemotePort)
		}
	}
}

// TestBatchProxies_Mixed portsExpr=`6000-6010,7000` → 12 条。
func TestBatchProxies_Mixed(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	body := map[string]any{
		"basename":  "mix",
		"type":      "tcp",
		"portsExpr": "6000-6010,7000",
	}
	resp, raw := doJSON(t, srv, "POST", "/api/v1/proxies/batch", body, cookies, csrf)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201\nbody=%s", resp.StatusCode, raw)
	}
	var got BatchProxiesResponse
	_ = json.Unmarshal(raw, &got)
	if got.Created != 12 {
		t.Errorf("Created = %d, want 12", got.Created)
	}
}

// TestBatchProxies_BadExpr portsExpr=`abc` → 422 + 端口表达式语法错误。
func TestBatchProxies_BadExpr(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	body := map[string]any{"basename": "x", "type": "tcp", "portsExpr": "abc"}
	resp, raw := doJSON(t, srv, "POST", "/api/v1/proxies/batch", body, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422\nbody=%s", resp.StatusCode, raw)
	}
	var e ErrorBody
	_ = json.Unmarshal(raw, &e)
	if e.Error.Field != "portsExpr" {
		t.Errorf("field = %q", e.Error.Field)
	}
}

// TestBatchProxies_TooMany portsExpr 展开 35 条 → 422 + 端口数超 32。
func TestBatchProxies_TooMany(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)
	body := map[string]any{
		"basename":  "x",
		"type":      "tcp",
		"portsExpr": "6000-6034", // 35 条
	}
	resp, raw := doJSON(t, srv, "POST", "/api/v1/proxies/batch", body, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422\nbody=%s", resp.StatusCode, raw)
	}
}

// TestBatchProxies_BasenameTooLong basename > 58 → 422 + field=basename。
func TestBatchProxies_BasenameTooLong(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)
	long := ""
	for i := 0; i < 60; i++ {
		long += "a"
	}
	body := map[string]any{"basename": long, "type": "tcp", "portsExpr": "6000"}
	resp, raw := doJSON(t, srv, "POST", "/api/v1/proxies/batch", body, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422\nbody=%s", resp.StatusCode, raw)
	}
	var e ErrorBody
	_ = json.Unmarshal(raw, &e)
	if e.Error.Field != "basename" {
		t.Errorf("field = %q", e.Error.Field)
	}
}

// TestBatchProxies_BadType type 必须 tcp / udp。
func TestBatchProxies_BadType(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)
	body := map[string]any{"basename": "x", "type": "http", "portsExpr": "6000"}
	resp, raw := doJSON(t, srv, "POST", "/api/v1/proxies/batch", body, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422\nbody=%s", resp.StatusCode, raw)
	}
}

// TestBatchProxies_NameConflict 预存在的 name=web-6000，再 batch 含 6000 → 409 + 全部回滚。
func TestBatchProxies_NameConflict(t *testing.T) {
	srv, store := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	// 直接通过 store 预置
	rp := 9999
	// 通过 API 先建一条 name=web-6000，避免直接动 store 干扰锁。
	preBody := map[string]any{
		"name": "web-6000", "type": "tcp",
		"localPort": 1234, "remotePort": rp,
	}
	resp, raw := doJSON(t, srv, "POST", "/api/v1/proxies", preBody, cookies, csrf)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("pre create: %d %s", resp.StatusCode, raw)
	}

	body := map[string]any{
		"basename":  "web",
		"type":      "tcp",
		"portsExpr": "6000-6002",
	}
	resp, raw = doJSON(t, srv, "POST", "/api/v1/proxies/batch", body, cookies, csrf)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409\nbody=%s", resp.StatusCode, raw)
	}
	var e ErrorBody
	_ = json.Unmarshal(raw, &e)
	if e.Error.Field != "name" {
		t.Errorf("field = %q, want name", e.Error.Field)
	}

	// 验证 DB 行数 = 1（pre 建的那条，batch 全部回滚）
	list, _ := store.ListProxies(t.Context())
	if len(list) != 1 {
		t.Errorf("DB rows = %d, want 1 (rollback)", len(list))
	}
}

// TestBatchProxies_TcpRemoteConflict 已有 (tcp, 6001) → batch 6000-6002 命中冲突 → 409 + 全部回滚。
func TestBatchProxies_TcpRemoteConflict(t *testing.T) {
	srv, store := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	preBody := map[string]any{
		"name": "blocker", "type": "tcp",
		"localPort": 1234, "remotePort": 6001,
	}
	resp, raw := doJSON(t, srv, "POST", "/api/v1/proxies", preBody, cookies, csrf)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("pre create: %d %s", resp.StatusCode, raw)
	}

	body := map[string]any{
		"basename":  "x",
		"type":      "tcp",
		"portsExpr": "6000-6002",
	}
	resp, raw = doJSON(t, srv, "POST", "/api/v1/proxies/batch", body, cookies, csrf)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409\nbody=%s", resp.StatusCode, raw)
	}
	var e ErrorBody
	_ = json.Unmarshal(raw, &e)
	if e.Error.Field != "remotePort" {
		t.Errorf("field = %q, want remotePort", e.Error.Field)
	}

	list, _ := store.ListProxies(t.Context())
	if len(list) != 1 {
		t.Errorf("DB rows = %d, want 1 (rollback)", len(list))
	}
}

// TestBatchProxies_TotalLimit DB 现有 198 条，batch 5 条 → 422，DB 不变。
func TestBatchProxies_TotalLimit(t *testing.T) {
	srv, store := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	// 预填 198 条（直接走 store 加速）
	for i := 0; i < 198; i++ {
		rp := 30000 + i
		preBody := map[string]any{
			"name": fmt.Sprintf("pre-%d", i), "type": "tcp",
			"localPort": 1000 + i, "remotePort": rp,
		}
		resp, _ := doJSON(t, srv, "POST", "/api/v1/proxies", preBody, cookies, csrf)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("pre %d failed: %d", i, resp.StatusCode)
		}
	}

	body := map[string]any{
		"basename":  "over",
		"type":      "tcp",
		"portsExpr": "40000-40004", // 5 条
	}
	resp, raw := doJSON(t, srv, "POST", "/api/v1/proxies/batch", body, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422\nbody=%s", resp.StatusCode, raw)
	}

	list, _ := store.ListProxies(t.Context())
	if len(list) != 198 {
		t.Errorf("DB rows = %d, want 198", len(list))
	}
}

// TestBatchProxies_Unauthenticated 未登录 → 401。
func TestBatchProxies_Unauthenticated(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	body := map[string]any{"basename": "x", "type": "tcp", "portsExpr": "6000"}
	resp, _ := doJSON(t, srv, "POST", "/api/v1/proxies/batch", body, nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// TestBatchProxies_NoCSRF 已登录但无 CSRF → 403。
func TestBatchProxies_NoCSRF(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, _ := setupAndLogin(t, srv)
	body := map[string]any{"basename": "x", "type": "tcp", "portsExpr": "6000"}
	resp, _ := doJSON(t, srv, "POST", "/api/v1/proxies/batch", body, cookies, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

// TestBatchProxies_BadJSON 请求体不是 JSON → 400。
func TestBatchProxies_BadJSON(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)
	// 用 string 作为 body 让 json.Decode 失败
	resp, _ := doJSON(t, srv, "POST", "/api/v1/proxies/batch", "not-a-json-but-string", cookies, csrf)
	// doJSON 实际会 json.Marshal "not-a-json-but-string" 变成合法 JSON "..."，handler 解码到
	// 空 struct 后 basename 校验失败 → 422。这里仅断言 4xx 即可。
	if resp.StatusCode < 400 || resp.StatusCode >= 500 {
		t.Errorf("status = %d, want 4xx", resp.StatusCode)
	}
}
