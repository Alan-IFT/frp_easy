package httpapi

// T-039 handler tests: cover 200 / 401(→502) / 404 / 503 / aggregation partial-success
// 分支 + autogen fallback 链路 + 用户填值优先 + dashboardEnabled=false → 503。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/frp-easy/frp-easy/internal/frpsadmin"
)

// --- 测试 seam helpers ---

// withFrpsAdminFactory 临时替换 frpsAdminFactory 让 handler 调用 mock httptest server。
// t.Cleanup 自动恢复，防泄漏到其他测试。
func withFrpsAdminFactory(t *testing.T, mockURL string) {
	t.Helper()
	original := frpsAdminFactory
	frpsAdminFactory = func(addr string, port int, user, pass string) *frpsadmin.Client {
		// 忽略 handler 传入的 addr/port（KV 里实际配的可能是 127.0.0.1:7500），
		// 强制走 mock URL。user/pass 仍透传让 mock server 校验 basic auth。
		return frpsadmin.NewWithBaseURL(mockURL, user, pass, 0)
	}
	t.Cleanup(func() { frpsAdminFactory = original })
}

// writeFrpsConfig 把 cfg 序列化写入 KV `frps.config`。
func writeFrpsConfig(t *testing.T, store interface {
	KVSet(ctx context.Context, key, value string) error
}, cfg FrpsConfig) {
	t.Helper()
	b, _ := json.Marshal(cfg)
	if err := store.KVSet(t.Context(), kvFrpsConfig, string(b)); err != nil {
		t.Fatalf("kvset frps.config: %v", err)
	}
}

// writeFrpsAutogen 把 autogen 凭据写入 KV `frps.dashboard.autogen`。
func writeFrpsAutogen(t *testing.T, store interface {
	KVSet(ctx context.Context, key, value string) error
}, user, pass string) {
	t.Helper()
	b, _ := json.Marshal(FrpsDashboardCreds{User: user, Pass: pass})
	if err := store.KVSet(t.Context(), kvFrpsDashboardAutogen, string(b)); err != nil {
		t.Fatalf("kvset frps.dashboard.autogen: %v", err)
	}
}

// waitForFile 轮询 path 内容直至满足 predicate 或超过 deadline，返回最后读到的字符串。
func waitForFile(t *testing.T, path string, deadline time.Duration, predicate func(string) bool) string {
	t.Helper()
	start := time.Now()
	var content string
	for time.Since(start) < deadline {
		if data, err := os.ReadFile(path); err == nil {
			content = string(data)
			if predicate(content) {
				return content
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	return content
}

// --- handler 单测 ---

// TestServerRuntimeInfo_DashboardDisabled_503 验证 FR-2.6 / Q-2：
// DashboardEnabled=false → handler 503 + 友好错误体。
func TestServerRuntimeInfo_DashboardDisabled_503(t *testing.T) {
	srv, store := newTestServerFull(t, nil, nil)
	cookies, _ := setupAndLogin(t, srv)
	writeFrpsConfig(t, store, FrpsConfig{
		BindPort:         7000,
		DashboardEnabled: false,
	})
	resp, body := doJSON(t, srv, "GET", "/api/v1/server/runtime/info", nil, cookies, "")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "未启用") {
		t.Errorf("expected friendly chinese message, got %s", body)
	}
}

// TestServerRuntimeInfo_AutogenFallback 验证 FR-3.5：
// frps.config 中 DashboardEnabled=true 但 user/pass 空 → handler 用 autogen KV 凭据调上游。
func TestServerRuntimeInfo_AutogenFallback(t *testing.T) {
	srv, store := newTestServerFull(t, nil, nil)
	cookies, _ := setupAndLogin(t, srv)

	// mock frps dashboard：捕获 basic auth + 返 200 + JSON
	var seenUser, seenPass string
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUser, seenPass, _ = r.BasicAuth()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"0.58.1","bindPort":7000,"clientCounts":2,"curConns":5}`))
	}))
	defer mock.Close()
	withFrpsAdminFactory(t, mock.URL)

	writeFrpsConfig(t, store, FrpsConfig{
		BindPort:         7000,
		DashboardEnabled: true,
		DashboardAddr:    "127.0.0.1",
		DashboardPort:    7500,
		// user/pass 留空，应触发 fallback
	})
	writeFrpsAutogen(t, store, "frp_easy_auto", "auto-pass-xyz")

	resp, body := doJSON(t, srv, "GET", "/api/v1/server/runtime/info", nil, cookies, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", resp.StatusCode, body)
	}
	if seenUser != "frp_easy_auto" || seenPass != "auto-pass-xyz" {
		t.Errorf("autogen creds not applied: u=%q p=%q", seenUser, seenPass)
	}
	var got frpsadmin.ServerInfo
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Version != "0.58.1" || got.ClientCounts != 2 {
		t.Errorf("response not parsed: %+v", got)
	}
}

// TestServerRuntimeInfo_UserCredsPreferred 验证 FR-3.4：
// 用户在 frps.config 显式填写 user/pass → autogen 不被使用，即使 autogen 也存在。
func TestServerRuntimeInfo_UserCredsPreferred(t *testing.T) {
	srv, store := newTestServerFull(t, nil, nil)
	cookies, _ := setupAndLogin(t, srv)

	var seenUser string
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUser, _, _ = r.BasicAuth()
		_, _ = w.Write([]byte(`{"version":"0.58.1"}`))
	}))
	defer mock.Close()
	withFrpsAdminFactory(t, mock.URL)

	writeFrpsConfig(t, store, FrpsConfig{
		BindPort:         7000,
		DashboardEnabled: true,
		DashboardUser:    "user-explicit",
		DashboardPass:    "pass-explicit",
	})
	writeFrpsAutogen(t, store, "user-auto", "pass-auto")

	resp, body := doJSON(t, srv, "GET", "/api/v1/server/runtime/info", nil, cookies, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", resp.StatusCode, body)
	}
	if seenUser != "user-explicit" {
		t.Errorf("expected user-explicit, autogen incorrectly used: seenUser=%q", seenUser)
	}
}

// TestServerRuntimeInfo_UpstreamUnauthorized_502 验证 FR-2.7：
// 上游 frps 返 401 → handler 502 + 友好诊断文案。
func TestServerRuntimeInfo_UpstreamUnauthorized_502(t *testing.T) {
	srv, store := newTestServerFull(t, nil, nil)
	cookies, _ := setupAndLogin(t, srv)
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer mock.Close()
	withFrpsAdminFactory(t, mock.URL)
	writeFrpsConfig(t, store, FrpsConfig{
		BindPort:         7000,
		DashboardEnabled: true,
		DashboardUser:    "u", DashboardPass: "p",
	})
	resp, body := doJSON(t, srv, "GET", "/api/v1/server/runtime/info", nil, cookies, "")
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status %d body=%s (expected 502)", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "401") {
		t.Errorf("expected 401 in message, got %s", body)
	}
}

// TestServerRuntimeInfo_UpstreamUnavailable_503 验证 FR-2.7：
// 上游 frps 连接失败（关掉 mock）→ handler 503。
func TestServerRuntimeInfo_UpstreamUnavailable_503(t *testing.T) {
	srv, store := newTestServerFull(t, nil, nil)
	cookies, _ := setupAndLogin(t, srv)
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	mock.Close() // 立即关，让端口悬空
	withFrpsAdminFactory(t, mock.URL)
	writeFrpsConfig(t, store, FrpsConfig{
		BindPort:         7000,
		DashboardEnabled: true,
		DashboardUser:    "u", DashboardPass: "p",
	})
	resp, body := doJSON(t, srv, "GET", "/api/v1/server/runtime/info", nil, cookies, "")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d body=%s (expected 503)", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "frps") {
		t.Errorf("expected 'frps' in friendly message, got %s", body)
	}
}

// TestServerRuntimeProxies_Aggregation 验证 FR-2.2 + Q-1（部分成功）：
// mock 对 tcp/http 返回数据，对 xtcp 返 5xx → handler 200 + errors.xtcp 字段。
func TestServerRuntimeProxies_Aggregation(t *testing.T) {
	srv, store := newTestServerFull(t, nil, nil)
	cookies, _ := setupAndLogin(t, srv)
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/proxy/tcp":
			_, _ = w.Write([]byte(`{"proxies":[{"name":"ssh","type":"tcp","status":"online"}]}`))
		case "/api/proxy/http":
			_, _ = w.Write([]byte(`{"proxies":[{"name":"web","type":"http","status":"online"}]}`))
		case "/api/proxy/xtcp":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			// 其它 type 返回空 envelope
			_, _ = w.Write([]byte(`{"proxies":[]}`))
		}
	}))
	defer mock.Close()
	withFrpsAdminFactory(t, mock.URL)
	writeFrpsConfig(t, store, FrpsConfig{
		BindPort: 7000, DashboardEnabled: true,
		DashboardUser: "u", DashboardPass: "p",
	})
	resp, body := doJSON(t, srv, "GET", "/api/v1/server/runtime/proxies", nil, cookies, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s (expected 200; per-type errors should not fail whole request)", resp.StatusCode, body)
	}
	var got ServerRuntimeProxiesResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Proxies["tcp"]) != 1 || got.Proxies["tcp"][0].Name != "ssh" {
		t.Errorf("tcp proxies missing: %+v", got.Proxies)
	}
	if len(got.Proxies["http"]) != 1 {
		t.Errorf("http proxies missing: %+v", got.Proxies)
	}
	if _, ok := got.Errors["xtcp"]; !ok {
		t.Errorf("expected xtcp error in response, got errors=%+v", got.Errors)
	}
}

// TestServerRuntimeProxies_AllFatal_503 验证 FR-2.2 全 fatal 路径：
// mock 对所有 type 返 5xx → handler 503（统一错误映射）。
func TestServerRuntimeProxies_AllFatal_503(t *testing.T) {
	srv, store := newTestServerFull(t, nil, nil)
	cookies, _ := setupAndLogin(t, srv)
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer mock.Close()
	withFrpsAdminFactory(t, mock.URL)
	writeFrpsConfig(t, store, FrpsConfig{
		BindPort: 7000, DashboardEnabled: true,
		DashboardUser: "u", DashboardPass: "p",
	})
	resp, body := doJSON(t, srv, "GET", "/api/v1/server/runtime/proxies", nil, cookies, "")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d body=%s (expected 503 when all type fatal)", resp.StatusCode, body)
	}
}

// TestServerRuntimeProxyDetail_404 验证 FR-2.3 + 上游 404 直传。
func TestServerRuntimeProxyDetail_404(t *testing.T) {
	srv, store := newTestServerFull(t, nil, nil)
	cookies, _ := setupAndLogin(t, srv)
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mock.Close()
	withFrpsAdminFactory(t, mock.URL)
	writeFrpsConfig(t, store, FrpsConfig{
		BindPort: 7000, DashboardEnabled: true,
		DashboardUser: "u", DashboardPass: "p",
	})
	resp, body := doJSON(t, srv, "GET", "/api/v1/server/runtime/proxy/tcp/ghost", nil, cookies, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d body=%s", resp.StatusCode, body)
	}
	var e ErrorBody
	_ = json.Unmarshal(body, &e)
	if e.Error.Code != CodeNotFound {
		t.Errorf("code: %s", e.Error.Code)
	}
}

// TestServerRuntimeProxyDetail_200 验证 200 happy path + url 路径参数透传。
func TestServerRuntimeProxyDetail_200(t *testing.T) {
	srv, store := newTestServerFull(t, nil, nil)
	cookies, _ := setupAndLogin(t, srv)
	var seenPath string
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		_, _ = w.Write([]byte(`{"name":"ssh","type":"tcp","status":"online","clientVersion":"0.58.1"}`))
	}))
	defer mock.Close()
	withFrpsAdminFactory(t, mock.URL)
	writeFrpsConfig(t, store, FrpsConfig{
		BindPort: 7000, DashboardEnabled: true,
		DashboardUser: "u", DashboardPass: "p",
	})
	resp, body := doJSON(t, srv, "GET", "/api/v1/server/runtime/proxy/tcp/ssh", nil, cookies, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", resp.StatusCode, body)
	}
	if seenPath != "/api/proxy/tcp/ssh" {
		t.Errorf("upstream path = %s, want /api/proxy/tcp/ssh", seenPath)
	}
	var got frpsadmin.ProxyDetail
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "ssh" || got.ClientVersion != "0.58.1" {
		t.Errorf("got = %+v", got)
	}
}

// TestServerRuntimeTraffic_200 验证 FR-2.4 + 数组字段。
func TestServerRuntimeTraffic_200(t *testing.T) {
	srv, store := newTestServerFull(t, nil, nil)
	cookies, _ := setupAndLogin(t, srv)
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"ssh","trafficIn":[0,1024,0,2048],"trafficOut":[0,512,0,1024]}`))
	}))
	defer mock.Close()
	withFrpsAdminFactory(t, mock.URL)
	writeFrpsConfig(t, store, FrpsConfig{
		BindPort: 7000, DashboardEnabled: true,
		DashboardUser: "u", DashboardPass: "p",
	})
	resp, body := doJSON(t, srv, "GET", "/api/v1/server/runtime/traffic/ssh", nil, cookies, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d body=%s", resp.StatusCode, body)
	}
	var got frpsadmin.Traffic
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "ssh" || len(got.TrafficIn) != 4 || got.TrafficIn[1] != 1024 {
		t.Errorf("got = %+v", got)
	}
}

// TestServerRuntime_Unauthenticated 验证 FR-2.5：未登录访问 → 401。
func TestServerRuntime_Unauthenticated(t *testing.T) {
	srv, _ := newTestServerFull(t, nil, nil)
	resp, _ := doJSON(t, srv, "GET", "/api/v1/server/runtime/info", nil, nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status %d (expected 401 from SessionAuth)", resp.StatusCode)
	}
}

// --- renderAndApplyFrps autogen fallback 集成测试（AC-6） ---

// TestRenderAndApplyFrps_AutogenFallback 验证 FR-3.3：
// DashboardEnabled=true + user/pass 空 + autogen 有值 → 渲染 frps.toml 含 autogen 凭据。
func TestRenderAndApplyFrps_AutogenFallback(t *testing.T) {
	dir := t.TempDir()
	frpcToml := filepath.Join(dir, "frpc.toml")
	frpsToml := filepath.Join(dir, "frps.toml")
	srv, store := newTestServerFull(t, map[string]string{"frpc": frpcToml, "frps": frpsToml}, nil)
	cookies, csrf := setupAndLogin(t, srv)

	// 预先写 autogen 凭据
	writeFrpsAutogen(t, store, "auto-user-T039", "auto-pass-T039xyz")

	// 通过 PUT /server 写入 frps.config（DashboardEnabled=true / user/pass 空）
	resp, body := doJSON(t, srv, "PUT", "/api/v1/server",
		map[string]any{
			"bindPort":         7000,
			"dashboardEnabled": true,
			"dashboardPort":    7500,
			// user/pass 故意留空 → 应被 fallback 补齐
		}, cookies, csrf)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /server status %d body=%s", resp.StatusCode, body)
	}

	content := waitForFile(t, frpsToml, 3*time.Second, func(s string) bool {
		return strings.Contains(s, "[webServer]") && strings.Contains(s, "auto-user-T039")
	})
	if !strings.Contains(content, "[webServer]") {
		t.Fatalf("frps.toml missing [webServer] section after wait:\n%s", content)
	}
	if !strings.Contains(content, `user = 'auto-user-T039'`) && !strings.Contains(content, `user = "auto-user-T039"`) {
		t.Errorf("autogen user not in rendered toml:\n%s", content)
	}
	if !strings.Contains(content, "auto-pass-T039xyz") {
		t.Errorf("autogen pass not in rendered toml:\n%s", content)
	}
}

// TestRenderAndApplyFrps_UserCredsTakePrecedence 验证 FR-3.4：
// 用户填了 user/pass → 渲染用用户值，autogen 不被使用。
func TestRenderAndApplyFrps_UserCredsTakePrecedence(t *testing.T) {
	dir := t.TempDir()
	frpcToml := filepath.Join(dir, "frpc.toml")
	frpsToml := filepath.Join(dir, "frps.toml")
	srv, store := newTestServerFull(t, map[string]string{"frpc": frpcToml, "frps": frpsToml}, nil)
	cookies, csrf := setupAndLogin(t, srv)

	writeFrpsAutogen(t, store, "auto-shouldnotappear", "auto-pass-shouldnotappear")

	resp, body := doJSON(t, srv, "PUT", "/api/v1/server",
		map[string]any{
			"bindPort":         7000,
			"dashboardEnabled": true,
			"dashboardPort":    7500,
			"dashboardUser":    "my-explicit-user",
			"dashboardPass":    "my-explicit-pw",
		}, cookies, csrf)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /server status %d body=%s", resp.StatusCode, body)
	}

	content := waitForFile(t, frpsToml, 3*time.Second, func(s string) bool {
		return strings.Contains(s, "[webServer]") && strings.Contains(s, "my-explicit-user")
	})
	if !strings.Contains(content, "my-explicit-user") {
		t.Errorf("user-explicit not in toml:\n%s", content)
	}
	if strings.Contains(content, "auto-shouldnotappear") {
		t.Errorf("autogen incorrectly used despite user explicit value:\n%s", content)
	}
}
