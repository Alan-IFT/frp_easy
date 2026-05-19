package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// 三个最低基线安全响应头的期望值（T-007 AC-3）。
var expectedSecurityHeaders = map[string]string{
	"X-Content-Type-Options": "nosniff",
	"X-Frame-Options":        "DENY",
	"Referrer-Policy":        "no-referrer",
}

// assertSecurityHeaders 检查响应包含三个头，值精确匹配。
func assertSecurityHeaders(t *testing.T, resp *http.Response, ctx string) {
	t.Helper()
	for k, want := range expectedSecurityHeaders {
		got := resp.Header.Get(k)
		if got != want {
			t.Errorf("%s: header %s = %q, want %q", ctx, k, got, want)
		}
	}
}

// TestSecurityHeaders_OnHealth 验证 AC-3.1：GET /api/v1/health 响应包含三个头。
// /api/v1/health 在 router.go 中注册在顶层（不经过 ReadyGate 等中间件），
// 但 r.Use(SecurityHeaders()) 挂在路由注册之前 → chi 全局 Use 覆盖此路由。
func TestSecurityHeaders_OnHealth(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	resp, _ := doJSON(t, srv, "GET", "/api/v1/health", nil, nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status: %d", resp.StatusCode)
	}
	assertSecurityHeaders(t, resp, "/api/v1/health")
}

// TestSecurityHeaders_OnPublicAPI 验证 AC-3.1：公开 API 响应包含三个头。
func TestSecurityHeaders_OnPublicAPI(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	resp, _ := doJSON(t, srv, "GET", "/api/v1/system/ready", nil, nil, "")
	defer resp.Body.Close()
	assertSecurityHeaders(t, resp, "/api/v1/system/ready")
}

// TestSecurityHeaders_OnNotFound 验证 AC-3.1：SPA fallback / NotFound 响应包含三个头。
// 注意 NotFound 在 chi 中由 r.NotFound 注册，全局 Use 同样覆盖。
func TestSecurityHeaders_OnNotFound(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	resp, _ := doJSON(t, srv, "GET", "/some/non/existent/path", nil, nil, "")
	defer resp.Body.Close()
	assertSecurityHeaders(t, resp, "NotFound fallback")
}

// TestSecurityHeaders_OnError 验证 AC-3.1：错误响应（401 未认证）仍含三个头。
// 受保护写接口未带 cookie → SessionAuth 返 401，此前响应仍应已 Set 安全头。
func TestSecurityHeaders_OnError(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	resp, _ := doJSON(t, srv, "GET", "/api/v1/proxies", nil, nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
	assertSecurityHeaders(t, resp, "401 error path")
}

// TestSecurityHeaders_OnRoot 验证根路径 / 响应（SPA 占位）含三个头。
func TestSecurityHeaders_OnRoot(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	resp, _ := doJSON(t, srv, "GET", "/", nil, nil, "")
	defer resp.Body.Close()
	assertSecurityHeaders(t, resp, "/ root SPA")
}

// TestSecurityHeaders_DirectMiddleware 直接测试中间件函数本身（无路由依赖），
// 防止未来重构 router.go 时悄无声息地丢失中间件挂载。
func TestSecurityHeaders_DirectMiddleware(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := SecurityHeaders()(next)

	req := httptest.NewRequest("GET", "/anything", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("next handler not called")
	}
	resp := rec.Result()
	defer resp.Body.Close()
	assertSecurityHeaders(t, resp, "direct middleware test")
}
