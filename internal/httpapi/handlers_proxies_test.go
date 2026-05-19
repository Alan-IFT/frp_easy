package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestCreateProxy_DuplicateName_Returns409 验证 AC-6.3：连续两次 POST 同名 proxy
// 第二次必须返回 409 + code=CONFLICT + 中文消息 + field=name。
func TestCreateProxy_DuplicateName_Returns409(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	rp1 := 16001
	body1 := map[string]any{
		"name":       "dup-409",
		"type":       "tcp",
		"localPort":  22,
		"remotePort": rp1,
	}
	resp, raw := doJSON(t, srv, "POST", "/api/v1/proxies", body1, cookies, csrf)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first POST status %d body=%s", resp.StatusCode, raw)
	}

	// 再插一个同名 proxy（remotePort 故意换成不同，避开 (type,remote_port) 冲突）
	rp2 := 16002
	body2 := map[string]any{
		"name":       "dup-409",
		"type":       "tcp",
		"localPort":  23,
		"remotePort": rp2,
	}
	resp, raw = doJSON(t, srv, "POST", "/api/v1/proxies", body2, cookies, csrf)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("second POST status = %d, want 409\nbody=%s", resp.StatusCode, raw)
	}
	var eb ErrorBody
	if err := json.Unmarshal(raw, &eb); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, raw)
	}
	if eb.Error.Code != CodeConflict {
		t.Errorf("code = %q, want %q", eb.Error.Code, CodeConflict)
	}
	if eb.Error.Message != "代理名称已存在，请改用其它名称" {
		t.Errorf("message = %q, want \"代理名称已存在，请改用其它名称\"", eb.Error.Message)
	}
	if eb.Error.Field != "name" {
		t.Errorf("field = %q, want \"name\"", eb.Error.Field)
	}
}

// TestCreateProxy_DuplicateTypeRemotePort_Returns422 回归保证：name 不冲突、
// (type, remote_port) 冲突仍走 422 兜底分支（保持原有行为不退化）。
func TestCreateProxy_DuplicateTypeRemotePort_Returns422(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	rp := 17000
	body1 := map[string]any{
		"name":       "first-tr",
		"type":       "tcp",
		"localPort":  22,
		"remotePort": rp,
	}
	resp, raw := doJSON(t, srv, "POST", "/api/v1/proxies", body1, cookies, csrf)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first POST: status %d body=%s", resp.StatusCode, raw)
	}

	// name 不同，但 (type=tcp, remotePort=17000) 相同 → 触发 idx_proxies_tcp_remote
	body2 := map[string]any{
		"name":       "second-tr",
		"type":       "tcp",
		"localPort":  23,
		"remotePort": rp,
	}
	resp, raw = doJSON(t, srv, "POST", "/api/v1/proxies", body2, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("second POST status = %d, want 422 (type,remotePort 兜底)\nbody=%s",
			resp.StatusCode, raw)
	}
	var eb ErrorBody
	_ = json.Unmarshal(raw, &eb)
	if eb.Error.Code != CodeConflict {
		t.Errorf("code = %q, want %q (422 仍用 CONFLICT)", eb.Error.Code, CodeConflict)
	}
	// field 应识别为 remotePort（mapProxyWriteError 内 strings.Contains low,"remote_port"）
	if eb.Error.Field != "remotePort" {
		t.Errorf("field = %q, want \"remotePort\"", eb.Error.Field)
	}
}

// TestUpdateProxy_DuplicateName_Returns409 验证 UPDATE 路径同样走 sentinel。
func TestUpdateProxy_DuplicateName_Returns409(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	rp1 := 18001
	rp2 := 18002
	// 创建两个不同名的 proxy
	resp, raw := doJSON(t, srv, "POST", "/api/v1/proxies",
		map[string]any{"name": "alpha", "type": "tcp", "localPort": 22, "remotePort": rp1},
		cookies, csrf)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create alpha: %d body=%s", resp.StatusCode, raw)
	}
	resp, raw = doJSON(t, srv, "POST", "/api/v1/proxies",
		map[string]any{"name": "beta", "type": "tcp", "localPort": 23, "remotePort": rp2},
		cookies, csrf)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create beta: %d body=%s", resp.StatusCode, raw)
	}
	var beta ProxyResponse
	if err := json.Unmarshal(raw, &beta); err != nil {
		t.Fatal(err)
	}

	// 把 beta 改名为 alpha → 409
	resp, raw = doJSON(t, srv, "PUT", "/api/v1/proxies/"+itoa(beta.ID),
		map[string]any{
			"name": "alpha", "type": "tcp", "localPort": 23,
			"remotePort": rp2, "version": beta.Version,
		}, cookies, csrf)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("PUT status = %d, want 409\nbody=%s", resp.StatusCode, raw)
	}
	var eb ErrorBody
	_ = json.Unmarshal(raw, &eb)
	if eb.Error.Message != "代理名称已存在，请改用其它名称" {
		t.Errorf("message = %q", eb.Error.Message)
	}
	if eb.Error.Field != "name" {
		t.Errorf("field = %q", eb.Error.Field)
	}
}

// itoa 用 fmt 的 %d，包内辅助避免引 strconv。
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
