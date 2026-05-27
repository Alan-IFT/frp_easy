package httpapi

// T-040: PUT /api/v1/server allowPorts 校验测试集
//
// 反向构造：前端绕过场景（直接发非法 JSON 给后端），后端必须 422 + field="allowPorts"。
// 覆盖 01 §6 AC-7 + 02 §7.2 所有用例。

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestPutServer_AllowPorts_Valid(t *testing.T) {
	srv, store := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	body := map[string]any{
		"bindPort": 7000,
		"allowPorts": []map[string]int{
			{"start": 6000, "end": 7000},
			{"single": 9000},
			{"start": 10000, "end": 11000},
		},
	}
	resp, raw := doJSON(t, srv, "PUT", "/api/v1/server", body, cookies, csrf)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", resp.StatusCode, raw)
	}
	// 持久化字面：KV 中 frps.config 应含 allowPorts 数组
	v, ok, err := store.KVGet(t.Context(), "frps.config")
	if err != nil || !ok {
		t.Fatalf("KVGet frps.config: ok=%v err=%v", ok, err)
	}
	if !strings.Contains(v, `"start":6000`) || !strings.Contains(v, `"single":9000`) {
		t.Errorf("KV value 缺 allowPorts 字段: %s", v)
	}
}

func TestPutServer_AllowPorts_Empty(t *testing.T) {
	srv, store := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	// allowPorts 留空（不传字段）→ KV 不含 allowPorts 字面（omitempty）
	body := map[string]any{"bindPort": 7000}
	resp, raw := doJSON(t, srv, "PUT", "/api/v1/server", body, cookies, csrf)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", resp.StatusCode, raw)
	}
	v, ok, _ := store.KVGet(t.Context(), "frps.config")
	if !ok {
		t.Fatal("KVGet frps.config missing")
	}
	if strings.Contains(v, "allowPorts") {
		t.Errorf("空 allowPorts 不应在 KV 字面: %s", v)
	}
}

func TestPutServer_AllowPorts_OutOfRange(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	cases := []struct {
		name string
		ap   []map[string]int
	}{
		{"end_65536", []map[string]int{{"start": 1000, "end": 65536}}},
		{"start_zero", []map[string]int{{"start": 0, "end": 100}}},
		{"single_65536", []map[string]int{{"single": 65536}}},
		{"single_zero_only", []map[string]int{{"single": 0}}}, // 三字段全 0 = empty entry
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := map[string]any{"bindPort": 7000, "allowPorts": c.ap}
			resp, raw := doJSON(t, srv, "PUT", "/api/v1/server", body, cookies, csrf)
			if resp.StatusCode != http.StatusUnprocessableEntity {
				t.Fatalf("status %d (want 422) body=%s", resp.StatusCode, raw)
			}
			var eb ErrorBody
			if err := json.Unmarshal(raw, &eb); err != nil {
				t.Fatalf("unmarshal: %v body=%s", err, raw)
			}
			if eb.Error.Field != "allowPorts" {
				t.Errorf("field = %q, want \"allowPorts\"", eb.Error.Field)
			}
			if !strings.Contains(eb.Error.Message, "allowPorts[") {
				t.Errorf("错误文案缺索引定位: %q", eb.Error.Message)
			}
		})
	}
}

func TestPutServer_AllowPorts_StartGreaterThanEnd(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	body := map[string]any{
		"bindPort": 7000,
		"allowPorts": []map[string]int{
			{"start": 80, "end": 70},
		},
	}
	resp, raw := doJSON(t, srv, "PUT", "/api/v1/server", body, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status %d body=%s", resp.StatusCode, raw)
	}
	if !strings.Contains(string(raw), "≤") && !strings.Contains(string(raw), "start=80") {
		t.Errorf("错误文案缺 start/end 定位: %s", raw)
	}
}

func TestPutServer_AllowPorts_Overlap(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	body := map[string]any{
		"bindPort": 7000,
		"allowPorts": []map[string]int{
			{"start": 1000, "end": 2000},
			{"start": 1500, "end": 2500},
		},
	}
	resp, raw := doJSON(t, srv, "PUT", "/api/v1/server", body, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status %d body=%s", resp.StatusCode, raw)
	}
	if !strings.Contains(string(raw), "重叠") {
		t.Errorf("错误文案应含 '重叠': %s", raw)
	}
}

func TestPutServer_AllowPorts_Mutex(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	body := map[string]any{
		"bindPort": 7000,
		"allowPorts": []map[string]int{
			{"start": 100, "end": 200, "single": 80},
		},
	}
	resp, raw := doJSON(t, srv, "PUT", "/api/v1/server", body, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status %d body=%s", resp.StatusCode, raw)
	}
	if !strings.Contains(string(raw), "互斥") {
		t.Errorf("错误文案应含 '互斥': %s", raw)
	}
}

func TestGetServer_AllowPorts_RoundTrip(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	// PUT 一次
	putBody := map[string]any{
		"bindPort": 7000,
		"allowPorts": []map[string]int{
			{"start": 6000, "end": 7000},
			{"single": 9000},
		},
	}
	resp, raw := doJSON(t, srv, "PUT", "/api/v1/server", putBody, cookies, csrf)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status %d body=%s", resp.StatusCode, raw)
	}

	// GET 验证字段
	resp, raw = doJSON(t, srv, "GET", "/api/v1/server", nil, cookies, csrf)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status %d", resp.StatusCode)
	}
	var got FrpsConfig
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, raw)
	}
	if len(got.AllowPorts) != 2 {
		t.Fatalf("AllowPorts len = %d, want 2; body=%s", len(got.AllowPorts), raw)
	}
	if got.AllowPorts[0].Start != 6000 || got.AllowPorts[0].End != 7000 {
		t.Errorf("entry[0]: %+v", got.AllowPorts[0])
	}
	if got.AllowPorts[1].Single != 9000 {
		t.Errorf("entry[1]: %+v", got.AllowPorts[1])
	}
}

func TestPutServer_AllowPorts_TooMany(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	list := make([]map[string]int, 101)
	for i := range list {
		list[i] = map[string]int{"single": 1000 + i}
	}
	body := map[string]any{"bindPort": 7000, "allowPorts": list}
	resp, raw := doJSON(t, srv, "PUT", "/api/v1/server", body, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status %d body=%s", resp.StatusCode, raw)
	}
	if !strings.Contains(string(raw), "100") {
		t.Errorf("错误文案应含上限数字: %s", raw)
	}
}
