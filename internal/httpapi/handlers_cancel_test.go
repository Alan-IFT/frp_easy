// Package httpapi — T-027 download-cancel HTTP 集成测试。
// 覆盖 01 §5.2 AC-http-cancel-* + AC-http-upload-during-download-message-updated。
//
// 注意：因 downloader.Manager.apiBaseURL 是 unexported（仅同包测试可注入），
// "真正 downloading → cancel" 的运行时 happy path 由 downloader 包单测覆盖
// （TestCancel_MidDownload 等已 PASS）；本 HTTP 集成测试主要验证 cancel endpoint
// 的契约层（status code / error envelope / 中间件链）以及 upload 409 文案精化。
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/frp-easy/frp-easy/internal/auth"
	"github.com/frp-easy/frp-easy/internal/binloc"
	"github.com/frp-easy/frp-easy/internal/downloader"
	"github.com/frp-easy/frp-easy/internal/storage"
)

// newCancelTestServer 起一个挂了真实 downloader 的 httptest server。
func newCancelTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	root := t.TempDir()
	store, err := storage.Open(t.TempDir())
	if err != nil && !errors.Is(err, storage.ErrCorruptReset) {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	loc := binloc.NewDefault(root)
	rl := auth.NewRateLimiter(store)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	dl := downloader.New(root, logger)

	deps := Dependencies{
		Store:       store,
		Locator:     loc,
		ProcMgr:     nil,
		RateLimiter: rl,
		LogFiles:    map[string]string{"frpc": "", "frps": ""},
		Ready:       func() bool { return true },
		Logger:      logger,
		Version:     "test-0.1.0",
		Downloader:  dl,
	}
	srv := httptest.NewServer(New(deps))
	t.Cleanup(srv.Close)
	return srv
}

// --- AC-http-cancel-no-cookie-401 ---

func TestDownloadCancel_NoCookie(t *testing.T) {
	srv := newCancelTestServer(t)
	resp, _ := doJSON(t, srv, "POST", "/api/v1/system/download-cancel/frpc", nil, nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// --- AC-http-cancel-no-csrf-403 ---

func TestDownloadCancel_NoCSRF(t *testing.T) {
	srv := newCancelTestServer(t)
	cookies, _ := setupAndLogin(t, srv)
	resp, _ := doJSON(t, srv, "POST", "/api/v1/system/download-cancel/frpc", nil, cookies, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

// --- AC-http-cancel-bad-kind-422（F-2 决策：偏离 01 的 400） ---

func TestDownloadCancel_BadKind(t *testing.T) {
	srv := newCancelTestServer(t)
	cookies, csrf := setupAndLogin(t, srv)
	resp, raw := doJSON(t, srv, "POST", "/api/v1/system/download-cancel/frpx", nil, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422\nbody=%s", resp.StatusCode, raw)
	}
	var e ErrorBody
	_ = json.Unmarshal(raw, &e)
	if e.Error.Code != CodeValidationFailed {
		t.Errorf("code = %q, want %q", e.Error.Code, CodeValidationFailed)
	}
	if e.Error.Field != "kind" {
		t.Errorf("field = %q, want kind", e.Error.Field)
	}
}

// --- AC-http-cancel-idle-200 ---

func TestDownloadCancel_Idle_200(t *testing.T) {
	srv := newCancelTestServer(t)
	cookies, csrf := setupAndLogin(t, srv)
	resp, raw := doJSON(t, srv, "POST", "/api/v1/system/download-cancel/frpc", nil, cookies, csrf)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody=%s", resp.StatusCode, raw)
	}
	var st downloader.DownloadState
	_ = json.Unmarshal(raw, &st)
	if st.Status != downloader.StatusIdle {
		t.Errorf("status = %q, want idle", st.Status)
	}
}

// --- AC-http-cancel-downloader-nil-503 ---

func TestDownloadCancel_DownloaderNil(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil && !errors.Is(err, storage.ErrCorruptReset) {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	rl := auth.NewRateLimiter(store)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	deps := Dependencies{
		Store:       store,
		Locator:     binloc.NewDefault(t.TempDir()),
		RateLimiter: rl,
		LogFiles:    map[string]string{"frpc": "", "frps": ""},
		Ready:       func() bool { return true },
		Logger:      logger,
		Version:     "t",
		// Downloader 故意 nil
	}
	srv := httptest.NewServer(New(deps))
	t.Cleanup(srv.Close)
	cookies, csrf := setupAndLogin(t, srv)
	resp, _ := doJSON(t, srv, "POST", "/api/v1/system/download-cancel/frpc", nil, cookies, csrf)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}

// --- AC-http-upload-during-download-message-updated（FR-6） ---
//
// 由于 downloader.Manager.states/cancels/apiBaseURL 都 unexported，本测试用
// 静态源码扫描验证新 409 文案存在；运行时的 409 触发由 downloader 包单测保证
// （setFailed 等终态的 422/409 行为已在其它 upload 测试覆盖）。
func TestUploadBin_409MessageContainsCancel(t *testing.T) {
	data, err := os.ReadFile("handlers_system.go")
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	src := string(data)
	// 关键短语，与 FR-6 一致。source 中是 Go 字符串字面量带 \" 转义。
	if !strings.Contains(src, `取消下载`) || !strings.Contains(src, `按钮后再上传`) {
		t.Errorf("handlers_system.go missing FR-6 精化文案（应含 取消下载 + 按钮后再上传）")
	}
	// 旧文案不应再存在（防回归）
	if strings.Contains(src, "请稍后再上传或取消下载") {
		t.Error("handlers_system.go 仍含旧 409 文案 “请稍后再上传或取消下载”（应已被 T-027 精化替换）")
	}
}
