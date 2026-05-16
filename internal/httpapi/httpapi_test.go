package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/frp-easy/frp-easy/internal/auth"
	"github.com/frp-easy/frp-easy/internal/binloc"
	"github.com/frp-easy/frp-easy/internal/storage"
)

// --- helpers ---

type fakeLoc struct {
	missing []string
}

func (f *fakeLoc) FRPCPath() (string, error) {
	for _, m := range f.missing {
		if m == "frpc" {
			return "", errors.New("frpc missing")
		}
	}
	return "/fake/frpc", nil
}
func (f *fakeLoc) FRPSPath() (string, error) {
	for _, m := range f.missing {
		if m == "frps" {
			return "", errors.New("frps missing")
		}
	}
	return "/fake/frps", nil
}
func (f *fakeLoc) Missing() []string { return f.missing }
func (f *fakeLoc) Root() string      { return "/fake/root" }

// 让 fakeLoc 也满足 binloc.Locator interface（编译期检查）。
var _ binloc.Locator = (*fakeLoc)(nil)

func newTestServer(t *testing.T, ready *atomic.Bool, loc binloc.Locator) (*httptest.Server, *storage.Store) {
	t.Helper()
	store, err := storage.Open(t.TempDir())
	if err != nil && !errors.Is(err, storage.ErrCorruptReset) {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if loc == nil {
		loc = &fakeLoc{}
	}
	rl := auth.NewRateLimiter(store)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	deps := Dependencies{
		Store:       store,
		Locator:     loc,
		ProcMgr:     nil, // handler 测试不真起子进程
		RateLimiter: rl,
		LogFiles:    map[string]string{"frpc": "", "frps": ""},
		Ready:       func() bool { return ready == nil || ready.Load() },
		Logger:      logger,
		Version:     "test-0.1.0",
	}
	srv := httptest.NewServer(New(deps))
	t.Cleanup(srv.Close)
	return srv, store
}

func doJSON(t *testing.T, srv *httptest.Server, method, path string, body any, cookies []*http.Cookie, csrf string) (*http.Response, []byte) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, srv.URL+path, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, out
}

// --- AC-1 system/ready 未初始化 ---

func TestSystemReady_Uninitialized(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	resp, body := doJSON(t, srv, "GET", "/api/v1/system/ready", nil, nil, "")
	if resp.StatusCode != 200 {
		t.Fatalf("status %d body=%s", resp.StatusCode, body)
	}
	var got SystemReady
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Initialized {
		t.Error("expected initialized=false")
	}
	if got.Version != "test-0.1.0" {
		t.Errorf("version: %s", got.Version)
	}
}

// --- AC-13 binMissing 非空 ---

func TestSystemReady_BinMissingReported(t *testing.T) {
	loc := &fakeLoc{missing: []string{"frpc"}}
	srv, _ := newTestServer(t, nil, loc)
	_, body := doJSON(t, srv, "GET", "/api/v1/system/ready", nil, nil, "")
	var got SystemReady
	_ = json.Unmarshal(body, &got)
	if len(got.BinMissing) != 1 || got.BinMissing[0] != "frpc" {
		t.Errorf("binMissing: %v", got.BinMissing)
	}
}

// --- AC-2 setup 后凭据非明文 ---

func TestSetup_HashedAndAutoLogin(t *testing.T) {
	srv, store := newTestServer(t, nil, nil)
	resp, body := doJSON(t, srv, "POST", "/api/v1/setup",
		map[string]string{"username": "admin", "password": "VerySafePass123"},
		nil, "")
	if resp.StatusCode != 200 {
		t.Fatalf("setup status %d body=%s", resp.StatusCode, body)
	}
	// 自动登录 → 应 set-cookie
	cookies := resp.Cookies()
	var sid *http.Cookie
	for _, c := range cookies {
		if c.Name == SessionCookieName {
			sid = c
			break
		}
	}
	if sid == nil {
		t.Fatal("expected session cookie")
	}
	// DB 中应保存哈希，不含明文 "VerySafePass123"
	admin, err := store.GetAdmin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if admin == nil {
		t.Fatal("admin nil after setup")
	}
	if strings.Contains(admin.PasswordHash, "VerySafePass123") {
		t.Errorf("PLAINTEXT LEAK in hash: %s", admin.PasswordHash)
	}
	if !strings.HasPrefix(admin.PasswordHash, "$argon2id$") {
		t.Errorf("not argon2id PHC: %s", admin.PasswordHash)
	}
}

// --- AC-3 setup 已 initialized → 409 ---

func TestSetup_AlreadyInitialized409(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	// 第一次
	resp, _ := doJSON(t, srv, "POST", "/api/v1/setup",
		map[string]string{"username": "admin", "password": "VerySafePass123"}, nil, "")
	if resp.StatusCode != 200 {
		t.Fatalf("first setup status %d", resp.StatusCode)
	}
	// 第二次
	resp, body := doJSON(t, srv, "POST", "/api/v1/setup",
		map[string]string{"username": "admin2", "password": "AnotherOne42pw"}, nil, "")
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("second setup status %d body=%s", resp.StatusCode, body)
	}
	var e ErrorBody
	_ = json.Unmarshal(body, &e)
	if e.Error.Code != CodeAlreadyInitialized {
		t.Errorf("code: %s", e.Error.Code)
	}
}

// --- AC-4 5 次失败后 429 + Retry-After ---

func TestLogin_RateLimited(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	// setup
	_, _ = doJSON(t, srv, "POST", "/api/v1/setup",
		map[string]string{"username": "admin", "password": "VerySafePass123"}, nil, "")

	// 5 次错误密码
	var lastResp *http.Response
	for i := 0; i < 5; i++ {
		lastResp, _ = doJSON(t, srv, "POST", "/api/v1/auth/login",
			map[string]string{"username": "admin", "password": "wrongwrongwrong"}, nil, "")
		_ = lastResp
	}
	// 第 6 次应 429
	resp, body := doJSON(t, srv, "POST", "/api/v1/auth/login",
		map[string]string{"username": "admin", "password": "wrongwrongwrong"}, nil, "")
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status %d body=%s", resp.StatusCode, body)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Error("Retry-After header missing")
	}
}

// --- AC-10 重复 proxy name → 422 + field ---

func TestProxy_DuplicateName422(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	// setup + login
	resp, _ := doJSON(t, srv, "POST", "/api/v1/setup",
		map[string]string{"username": "admin", "password": "VerySafePass123"}, nil, "")
	cookies := resp.Cookies()
	// CSRF
	resp, body := doJSON(t, srv, "GET", "/api/v1/auth/csrf", nil, cookies, "")
	var csrfBody CSRFResponse
	_ = json.Unmarshal(body, &csrfBody)
	csrf := csrfBody.CSRFToken

	in := map[string]any{
		"name": "demo-tcp", "type": "tcp",
		"localPort": 22, "remotePort": 6000,
	}
	resp, body = doJSON(t, srv, "POST", "/api/v1/proxies", in, cookies, csrf)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first create status %d body=%s", resp.StatusCode, body)
	}
	// 再来一次同名 → 422 + field=name
	resp, body = doJSON(t, srv, "POST", "/api/v1/proxies", in, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("dup status %d body=%s", resp.StatusCode, body)
	}
	var e ErrorBody
	_ = json.Unmarshal(body, &e)
	if e.Error.Field == "" {
		t.Errorf("expected field in error body, got %+v", e)
	}
}

// --- C-3 ReadyGate：not ready → POST 503 NOT_READY + Retry-After ---

func TestReadyGate_503OnWriteWhenNotReady(t *testing.T) {
	ready := &atomic.Bool{}
	ready.Store(false)
	srv, _ := newTestServer(t, ready, nil)

	resp, body := doJSON(t, srv, "POST", "/api/v1/setup",
		map[string]string{"username": "admin", "password": "VerySafePass123"}, nil, "")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d body=%s", resp.StatusCode, body)
	}
	if resp.Header.Get("Retry-After") != "2" {
		t.Errorf("Retry-After: %s", resp.Header.Get("Retry-After"))
	}
	var e ErrorBody
	_ = json.Unmarshal(body, &e)
	if e.Error.Code != CodeNotReady {
		t.Errorf("code: %s", e.Error.Code)
	}

	// GET 不被 ReadyGate 拦
	resp, _ = doJSON(t, srv, "GET", "/api/v1/system/ready", nil, nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET should pass through: %d", resp.StatusCode)
	}
}

// --- C-5 redact 不漏明文 ---

func TestLogger_RedactSecretFields(t *testing.T) {
	body := []byte(`{"username":"admin","password":"VerySafePass123","nested":{"token":"abc","oldPassword":"x"}}`)
	out := redact(body, redactKeys...)
	s := string(out)
	if strings.Contains(s, "VerySafePass123") {
		t.Errorf("password leaked: %s", s)
	}
	if strings.Contains(s, "abc") {
		t.Errorf("nested token leaked: %s", s)
	}
	if !strings.Contains(s, "\"password\":\"***\"") {
		t.Errorf("password not redacted: %s", s)
	}
	if !strings.Contains(s, "\"token\":\"***\"") {
		t.Errorf("token not redacted: %s", s)
	}
}

// --- 中间件未登录拦截 ---

func TestProtectedRoute_RequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	resp, _ := doJSON(t, srv, "GET", "/api/v1/auth/me", nil, nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// --- CSRF 写接口未带 token → 403 ---

func TestCSRF_MissingToken403(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	resp, _ := doJSON(t, srv, "POST", "/api/v1/setup",
		map[string]string{"username": "admin", "password": "VerySafePass123"}, nil, "")
	cookies := resp.Cookies()
	// 受保护写接口（password 修改）不带 X-CSRF-Token → 403
	resp, _ = doJSON(t, srv, "POST", "/api/v1/auth/password",
		map[string]string{"oldPassword": "VerySafePass123", "newPassword": "NewerSafePass456"},
		cookies, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

// --- /api/v1/mode get / put round trip ---

func TestMode_RoundTrip(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	resp, _ := doJSON(t, srv, "POST", "/api/v1/setup",
		map[string]string{"username": "admin", "password": "VerySafePass123"}, nil, "")
	cookies := resp.Cookies()
	_, body := doJSON(t, srv, "GET", "/api/v1/auth/csrf", nil, cookies, "")
	var csrf CSRFResponse
	_ = json.Unmarshal(body, &csrf)

	resp, body = doJSON(t, srv, "PUT", "/api/v1/mode",
		map[string]bool{"frpc": true, "frps": false}, cookies, csrf.CSRFToken)
	if resp.StatusCode != 200 {
		t.Fatalf("PUT status %d body=%s", resp.StatusCode, body)
	}

	resp, body = doJSON(t, srv, "GET", "/api/v1/mode", nil, cookies, "")
	var st ModeState
	_ = json.Unmarshal(body, &st)
	if !st.Frpc || st.Frps {
		t.Errorf("mode: %+v", st)
	}
}

// --- /server token 默认脱敏 ---

func TestServer_TokenRedactedByDefault(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	resp, _ := doJSON(t, srv, "POST", "/api/v1/setup",
		map[string]string{"username": "admin", "password": "VerySafePass123"}, nil, "")
	cookies := resp.Cookies()
	_, body := doJSON(t, srv, "GET", "/api/v1/auth/csrf", nil, cookies, "")
	var csrf CSRFResponse
	_ = json.Unmarshal(body, &csrf)

	// PUT 一次带 token
	resp, body = doJSON(t, srv, "PUT", "/api/v1/server", map[string]any{
		"bindPort":  7000,
		"authToken": "very-secret-token-xyz",
	}, cookies, csrf.CSRFToken)
	if resp.StatusCode != 200 {
		t.Fatalf("PUT status %d body=%s", resp.StatusCode, body)
	}

	// GET 默认 → token 应为 ***
	resp, body = doJSON(t, srv, "GET", "/api/v1/server", nil, cookies, "")
	if strings.Contains(string(body), "very-secret-token-xyz") {
		t.Errorf("token leaked: %s", body)
	}
	if !strings.Contains(string(body), "\"authToken\":\"***\"") {
		t.Errorf("token not redacted: %s", body)
	}
	// reveal=1 → 真实 token 出现
	resp, body = doJSON(t, srv, "GET", "/api/v1/server?reveal=1", nil, cookies, "")
	if !strings.Contains(string(body), "very-secret-token-xyz") {
		t.Errorf("reveal=1 should show: %s", body)
	}
}
