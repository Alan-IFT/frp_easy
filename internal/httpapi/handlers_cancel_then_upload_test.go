// Package httpapi — T-027 QA 补测：AC-http-cancel-then-upload-200（**关键 AC**）
// 端到端集成测试。
//
// 来源：05_CODE_REVIEW.md R-3 指出该关键 AC 此前由"三层组合"保证（cancel 端点契约
// + downloader cancel 行为 + upload 既有 happy path），不是真起 HTTP 集成。
// 本 QA 阶段新建端到端测试，独立 reproducer（不复用 developer 的 fixture）：
//
//	1) 起 httptest 慢 GitHub mock（API + archive 链路，chunk-write + sleep）
//	2) 自建 downloader.Manager、反射注入 unexported apiBaseURL/goos
//	3) 走完整 HTTP 链路：POST download-bin（202）→ 等 downloading
//	   → POST download-cancel/frpc（200, status=canceled）
//	   → POST upload-bin（200, status=200 + sha256 不为空）
//
// 设计参考：docs/features/download-cancel-and-upload-decouple/01_REQUIREMENT_ANALYSIS.md §5.2
// 或归档后 docs/features/_archived/download-cancel-and-upload-decouple/01_REQUIREMENT_ANALYSIS.md §5.2
package httpapi

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/frp-easy/frp-easy/internal/auth"
	"github.com/frp-easy/frp-easy/internal/binloc"
	"github.com/frp-easy/frp-easy/internal/downloader"
	"github.com/frp-easy/frp-easy/internal/storage"
)

// setUnexportedString 用 unsafe 写 m 上指定名称的 unexported string 字段。
// 仅限测试用：QA 必须能在不污染生产 API 表面的情况下注入 mock URL。
func setUnexportedString(t *testing.T, m *downloader.Manager, field, value string) {
	t.Helper()
	v := reflect.ValueOf(m).Elem().FieldByName(field)
	if !v.IsValid() {
		t.Fatalf("field %s not found on Manager", field)
	}
	if v.Kind() != reflect.String {
		t.Fatalf("field %s is not string (kind=%s)", field, v.Kind())
	}
	ptr := unsafe.Pointer(v.UnsafeAddr())
	reflect.NewAt(v.Type(), ptr).Elem().SetString(value)
}

// platformSuffix 根据 runtime.GOOS 返回 GitHub release 资产后缀。
// 让 mock GitHub server 返本机平台的 archive，让 doDownload 实际能走到 io.Copy / 解压阶段。
func platformSuffix() string {
	if runtime.GOOS == "windows" {
		return "_windows_amd64.zip"
	}
	return "_linux_amd64.tar.gz"
}

// platformBinaryHeader 返回本机平台对应的 binary 头字节（ELF 或 PE）。
// validateBinaryHeader 按 runtime.GOOS 校验，因此 upload-bin 测试在 Windows 上必须传 PE。
func platformBinaryHeader() []byte {
	if runtime.GOOS == "windows" {
		return peHeader()
	}
	return elfHeader()
}

// newCancelUploadSlowServer 起一个慢 GitHub mock 让 archive 路径 chunk-write + sleep。
// QA 自写的独立 reproducer（adversarial 要求），不复用 developer 的 newSlowChunkServer。
func newCancelUploadSlowServer(t *testing.T) *httptest.Server {
	t.Helper()
	// 用伪随机字节避免 gzip 压塌（L40）。
	r := rand.New(rand.NewSource(20260523))
	raw := make([]byte, 1<<20) // 1 MiB
	for i := range raw {
		raw[i] = byte(r.Intn(256))
	}
	var gzbuf bytes.Buffer
	gw := gzip.NewWriter(&gzbuf)
	_, _ = gw.Write(raw)
	_ = gw.Close()
	payload := gzbuf.Bytes()
	suffix := platformSuffix() // mock 资产名匹配本机 runtime.GOOS（windows→.zip / linux→.tar.gz）

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			body := fmt.Sprintf(`{
  "tag_name": "v0.99.0",
  "assets": [
    { "name": "frp_0.99.0%s", "browser_download_url": "%s/archive" }
  ]
}`, suffix, srv.URL)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		case r.URL.Path == "/archive":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(payload)))
			w.WriteHeader(http.StatusOK)
			flusher, _ := w.(http.Flusher)
			chunk := 4096
			for off := 0; off < len(payload); off += chunk {
				end := off + chunk
				if end > len(payload) {
					end = len(payload)
				}
				if _, err := w.Write(payload[off:end]); err != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
				select {
				case <-r.Context().Done():
					return
				case <-time.After(80 * time.Millisecond):
				}
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newCancelUploadIntegrationServer 自建 httptest server，持 downloader.Manager 引用。
// 与 newCancelTestServer 等价但导出 downloader 让反射注入可达。
func newCancelUploadIntegrationServer(t *testing.T) (*httptest.Server, *downloader.Manager) {
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
		Version:     "qa-cancel-then-upload",
		Downloader:  dl,
	}
	srv := httptest.NewServer(New(deps))
	t.Cleanup(srv.Close)
	return srv, dl
}

// TestDownloadCancel_ThenUpload_200 实现 01 §5.2 AC-http-cancel-then-upload-200（**关键 AC**）。
//
// QA Adversarial 假设："cancel 返 200 后 state 已经是 canceled" 是 FR-7 不变量。
// 若发生 race（cancel 200 返回但 state 仍 downloading），upload 应该被 409 阻断
// → 本测试 FAIL → 定位回归。
//
// 注意：固定 linux ELF 上传字节让本测试在 Windows 主机也能跑（upload-bin 的
// validateBinaryHeader 会根据 runtime.GOOS 校验 —— 在 Windows 上要传 PE 头）。
// 因此我们走 PE 头 + windows server，让 uploadBin 走平台一致的 happy path。
// 但 downloader.resolveLatestAsset 用 m.goos 注入决定 suffix —— 我们注入 windows
// 让 mock 返 windows zip？太复杂。改方案：注入 m.goos 设为 runtime.GOOS，
// 让 mock 也用对应 suffix。
func TestDownloadCancel_ThenUpload_200(t *testing.T) {
	srv, dl := newCancelUploadIntegrationServer(t)
	cookies, csrf := setupAndLogin(t, srv)

	// 起慢 mock GitHub server
	mock := newCancelUploadSlowServer(t)
	setUnexportedString(t, dl, "apiBaseURL", mock.URL)
	setUnexportedString(t, dl, "goos", runtime.GOOS)

	// Step 1: POST download-bin/frpc → 202
	resp, raw := doJSON(t, srv, "POST", "/api/v1/system/download-bin",
		map[string]any{"kind": "frpc"}, cookies, csrf)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("download-bin status = %d, want 202\nbody=%s", resp.StatusCode, raw)
	}

	// Step 2: 等到 state=downloading（确认 doDownload goroutine 真在跑）
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st, _ := dl.Status("frpc")
		if st.Status == downloader.StatusDownloading && st.Progress > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	pre, _ := dl.Status("frpc")
	if pre.Status != downloader.StatusDownloading {
		t.Fatalf("precondition: status = %q, want downloading", pre.Status)
	}

	// Step 3: POST download-cancel/frpc → 200, status=canceled
	cancelStart := time.Now()
	resp, raw = doJSON(t, srv, "POST", "/api/v1/system/download-cancel/frpc", nil, cookies, csrf)
	cancelElapsed := time.Since(cancelStart)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download-cancel status = %d, want 200\nbody=%s", resp.StatusCode, raw)
	}
	if cancelElapsed > 3500*time.Millisecond {
		t.Errorf("download-cancel took %v (> NFR-1 3s + slack)", cancelElapsed)
	}
	var afterCancel downloader.DownloadState
	if err := json.Unmarshal(raw, &afterCancel); err != nil {
		t.Fatalf("decode cancel body: %v", err)
	}
	if afterCancel.Status != downloader.StatusCanceled {
		t.Fatalf("cancel body.status = %q, want canceled\nbody=%s", afterCancel.Status, raw)
	}

	// Step 4: 立即（不睡眠 / 不重试）POST upload-bin/frpc with 本机平台 binary 头
	// FR-7 不变量：cancel 返 200 时 state 已落地 canceled，upload 不应被 409 阻断。
	body, ct := buildMultipartUpload(t, "frpc", "frpc", platformBinaryHeader(), "kind-first")
	req, err := http.NewRequest("POST", srv.URL+"/api/v1/system/upload-bin", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-CSRF-Token", csrf)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	upResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	upBody, _ := io.ReadAll(upResp.Body)
	upResp.Body.Close()

	if upResp.StatusCode != http.StatusOK {
		// 关键失败信号：cancel 后立即 upload 被 409 = FR-7 违反
		t.Fatalf("upload-bin AFTER cancel returned %d (want 200; FR-7 violated if 409)\nbody=%s",
			upResp.StatusCode, upBody)
	}
	var ur UploadBinResponse
	if err := json.Unmarshal(upBody, &ur); err != nil {
		t.Fatalf("decode upload body: %v", err)
	}
	if !ur.Ok || ur.SHA256 == "" || ur.Kind != "frpc" {
		t.Errorf("upload-bin response unexpected: %+v", ur)
	}
}

// TestDownloadCancel_HTTP200_DuringDownload 是 AC-http-cancel-200（端到端版本）。
//
// 与 TestDownloadCancel_ThenUpload_200 互补：仅验证 cancel 端点在真 downloading 中
// 返 200 + canceled 状态。如果 cancel 端点的 Cancel 调用返回 err 或 state 未切换，
// 这里会 FAIL。
func TestDownloadCancel_HTTP200_DuringDownload(t *testing.T) {
	srv, dl := newCancelUploadIntegrationServer(t)
	cookies, csrf := setupAndLogin(t, srv)
	mock := newCancelUploadSlowServer(t)
	setUnexportedString(t, dl, "apiBaseURL", mock.URL)
	setUnexportedString(t, dl, "goos", runtime.GOOS)

	// Start
	resp, _ := doJSON(t, srv, "POST", "/api/v1/system/download-bin",
		map[string]any{"kind": "frpc"}, cookies, csrf)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("download-bin = %d", resp.StatusCode)
	}
	// 等 downloading
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st, _ := dl.Status("frpc")
		if st.Status == downloader.StatusDownloading {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	// Cancel 端点
	resp, raw := doJSON(t, srv, "POST", "/api/v1/system/download-cancel/frpc", nil, cookies, csrf)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cancel = %d body=%s", resp.StatusCode, raw)
	}
	var st downloader.DownloadState
	_ = json.Unmarshal(raw, &st)
	if st.Status != downloader.StatusCanceled {
		t.Errorf("cancel body.status = %q, want canceled", st.Status)
	}
}

// TestUploadBin_409Message_RuntimeAssert 是 05 R-6 NIT 补测：
// 此前 TestUploadBin_409MessageContainsCancel 是源码静态扫描；本测试真起 server
// + 真触发 409，验证 FR-6 精化文案确实进入 response body（运行时契约）。
func TestUploadBin_409Message_RuntimeAssert(t *testing.T) {
	srv, dl := newCancelUploadIntegrationServer(t)
	cookies, csrf := setupAndLogin(t, srv)
	mock := newCancelUploadSlowServer(t)
	setUnexportedString(t, dl, "apiBaseURL", mock.URL)
	setUnexportedString(t, dl, "goos", runtime.GOOS)

	// 起下载并等到 downloading
	resp, _ := doJSON(t, srv, "POST", "/api/v1/system/download-bin",
		map[string]any{"kind": "frpc"}, cookies, csrf)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("download-bin = %d", resp.StatusCode)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st, _ := dl.Status("frpc")
		if st.Status == downloader.StatusDownloading {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	// 不 cancel，直接 upload → 应 409 + 精化文案
	body, ct := buildMultipartUpload(t, "frpc", "frpc", platformBinaryHeader(), "kind-first")
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/system/upload-bin", body)
	req.Header.Set("Content-Type", ct)
	req.Header.Set("X-CSRF-Token", csrf)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	upResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	upBody, _ := io.ReadAll(upResp.Body)
	upResp.Body.Close()

	// 收尾：cancel 让 goroutine 释放 + TempDir 可清
	t.Cleanup(func() { _ = dl.Cancel("frpc") })

	if upResp.StatusCode != http.StatusConflict {
		t.Fatalf("upload during download = %d, want 409\nbody=%s", upResp.StatusCode, upBody)
	}
	var eb ErrorBody
	_ = json.Unmarshal(upBody, &eb)
	if eb.Error.Code != CodeProcBusy {
		t.Errorf("error.code = %q, want %q", eb.Error.Code, CodeProcBusy)
	}
	// FR-6 精化文案：runtime 断言（含 "取消下载" + "按钮"）
	if !strings.Contains(eb.Error.Message, "取消下载") || !strings.Contains(eb.Error.Message, "按钮") {
		t.Errorf("FR-6 精化文案缺失：error.message = %q", eb.Error.Message)
	}
	// 旧文案不应再出现
	if strings.Contains(eb.Error.Message, "请稍后再上传或取消下载") {
		t.Errorf("旧文案残留: %q", eb.Error.Message)
	}
}
