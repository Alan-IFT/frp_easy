// Package downloader — unit tests.
// C-4 gate condition: ALL tests use net/http/httptest.NewServer to mock the CDN.
// No real external network calls are made.
package downloader

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// frpTestVersion 是测试归档内条目使用的版本号字面量。
// T-014：FRPVersion 常量已移除，下载 URL 改由 GitHub Release API 解析；
// 测试归档内条目路径只需任意稳定字面量即可（extractFromTarGz/Zip 用 filepath.Base 匹配）。
const frpTestVersion = "0.68.1"

// --- helpers ---

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// buildTarGz creates an in-memory tar.gz archive from a map of entryPath → content.
func buildTarGz(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range entries {
		hdr := &tar.Header{
			Name:     name,
			Size:     int64(len(content)),
			Mode:     0755,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar WriteHeader: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar Write: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar Close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip Close: %v", err)
	}
	return buf.Bytes()
}

// buildZip creates an in-memory zip archive from a map of entryPath → content.
func buildZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip Create: %v", err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("zip Write: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip Close: %v", err)
	}
	return buf.Bytes()
}

// waitForDone polls until Status(kind) leaves StatusDownloading or the timeout expires.
func waitForDone(t *testing.T, m *Manager, kind string, timeout time.Duration) DownloadState {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st, _ := m.Status(kind)
		if st.Status != StatusDownloading {
			return st
		}
		time.Sleep(10 * time.Millisecond)
	}
	st, _ := m.Status(kind)
	t.Fatalf("timeout waiting for %s download; current: %+v", kind, st)
	return DownloadState{}
}

// releaseJSON builds a GitHub Release API JSON body for a single asset whose
// browser_download_url points at archivePath on the same test server.
func releaseJSON(tag, assetName, assetURL string) string {
	return `{"tag_name":"` + tag + `","assets":[{"name":"` + assetName +
		`","browser_download_url":"` + assetURL + `"}]}`
}

// newFRPServer spins up an httptest server that serves both the GitHub Release
// API endpoint (/repos/fatedier/frp/releases/latest) and the archive download
// path (/archive). assetName decides which platform suffix the API advertises.
func newFRPServer(t *testing.T, assetName string, archive []byte) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(releaseJSON("v"+frpTestVersion, assetName, srv.URL+"/archive")))
		case r.URL.Path == "/archive":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", itoa(len(archive)))
			w.WriteHeader(http.StatusOK)
			w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

// --- Test 1: tar.gz download success + progress 0→100 ---

func TestDownload_TarGz_Success_Progress(t *testing.T) {
	// Create a valid tar.gz archive containing frpc (linux layout).
	archiveContent := buildTarGz(t, map[string]string{
		"frp_" + frpTestVersion + "_linux_amd64/frpc": "#!/bin/sh\necho frpc binary",
	})

	srv := newFRPServer(t, "frp_"+frpTestVersion+"_linux_amd64.tar.gz", archiveContent)
	defer srv.Close()

	root := t.TempDir()
	m := New(root, discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	st := waitForDone(t, m, "frpc", 5*time.Second)

	if st.Status != StatusSuccess {
		t.Errorf("expected status=%s, got %s (error=%q)", StatusSuccess, st.Status, st.Error)
	}
	if st.Progress != 100 {
		t.Errorf("expected progress=100, got %d", st.Progress)
	}

	// Verify the binary was installed.
	expectedPath := filepath.Join(root, "frp_linux", "frpc")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected binary at %s: %v", expectedPath, err)
	}
}

// --- Test 2: zip format success ---

func TestDownload_Zip_Success(t *testing.T) {
	// Create a valid zip archive containing frpc.exe (windows layout).
	archiveContent := buildZip(t, map[string]string{
		"frp_" + frpTestVersion + "_windows_amd64/frpc.exe": "fake frpc exe content",
	})

	srv := newFRPServer(t, "frp_"+frpTestVersion+"_windows_amd64.zip", archiveContent)
	defer srv.Close()

	root := t.TempDir()
	m := New(root, discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "windows"

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	st := waitForDone(t, m, "frpc", 5*time.Second)

	if st.Status != StatusSuccess {
		t.Errorf("expected status=%s, got %s (error=%q)", StatusSuccess, st.Status, st.Error)
	}
	if st.Progress != 100 {
		t.Errorf("expected progress=100, got %d", st.Progress)
	}

	// Verify the binary was installed.
	expectedPath := filepath.Join(root, "frp_win", "frpc.exe")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected binary at %s: %v", expectedPath, err)
	}
}

// --- Test 3: ErrAlreadyInProgress (concurrent Start) ---

func TestDownload_ErrAlreadyInProgress(t *testing.T) {
	// Blocking server: the API endpoint hangs until unblock is closed.
	unblock := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-unblock:
			// Server unblocked; respond with nothing (resolution will fail, that's OK).
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			// Request was cancelled (e.g., server closed).
		}
	}))
	defer func() {
		select {
		case <-unblock: // already closed
		default:
			close(unblock)
		}
		srv.Close()
	}()

	root := t.TempDir()
	m := New(root, discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"
	// T-025：仅 API 路径 hang（server 唯一 handler），用 apiClient 5s 超时避免主体挂死。
	m.apiClient = &http.Client{Timeout: 5 * time.Second}

	// First Start: state becomes StatusDownloading synchronously.
	if err := m.Start("frpc"); err != nil {
		t.Fatalf("first Start: %v", err)
	}

	// Second Start on same kind: must return ErrAlreadyInProgress.
	err := m.Start("frpc")
	if !errors.Is(err, ErrAlreadyInProgress) {
		t.Errorf("expected ErrAlreadyInProgress, got %v", err)
	}

	// Different kind must NOT be blocked.
	// (frps is independent; if goos=linux and frps also uses linux, frps can start.)
	errFrps := m.Start("frps")
	if errors.Is(errFrps, ErrAlreadyInProgress) {
		t.Errorf("frps should not be blocked by frpc download")
	}

	// Unblock the server so goroutines can terminate.
	close(unblock)
	// Wait for frpc goroutine to finish (we don't care about success/failure here).
	waitForDone(t, m, "frpc", 3*time.Second)
}

// --- Test 4: HTTP non-2xx on archive download → StatusFailed ---

func TestDownload_HTTP404_StatusFailed(t *testing.T) {
	// API endpoint resolves fine, but the archive path returns 404.
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(releaseJSON("v"+frpTestVersion,
				"frp_"+frpTestVersion+"_linux_amd64.tar.gz", srv.URL+"/missing")))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	root := t.TempDir()
	m := New(root, discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	st := waitForDone(t, m, "frpc", 5*time.Second)
	if st.Status != StatusFailed {
		t.Errorf("expected status=%s, got %s", StatusFailed, st.Status)
	}
	if st.Error == "" {
		t.Error("expected non-empty error message")
	}
}

// --- Test 5: Zip Slip attempt → filtered, legitimate binary still extracted ---

func TestDownload_ZipSlip_MaliciousEntryFiltered(t *testing.T) {
	root := t.TempDir()

	// Archive contains:
	//   - malicious entry: "../evil.txt" (contains "..")
	//   - legitimate entry: "frp_dir/frpc.exe" (valid)
	archiveContent := buildZip(t, map[string]string{
		"../evil.txt":      "evil content",
		"frp_dir/frpc.exe": "legit frpc content",
	})

	srv := newFRPServer(t, "frp_"+frpTestVersion+"_windows_amd64.zip", archiveContent)
	defer srv.Close()

	m := New(root, discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "windows"

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	st := waitForDone(t, m, "frpc", 5*time.Second)

	// The legitimate binary should be extracted successfully.
	if st.Status != StatusSuccess {
		t.Errorf("expected status=%s, got %s (error=%q)", StatusSuccess, st.Status, st.Error)
	}

	// The malicious file must NOT exist anywhere outside targetDir.
	// "../evil.txt" relative to frp_win/ would resolve to root/evil.txt.
	evilPath := filepath.Join(root, "evil.txt")
	if _, err := os.Stat(evilPath); !os.IsNotExist(err) {
		t.Errorf("malicious file should not exist at %s", evilPath)
	}

	// The legitimate binary must exist.
	expectedPath := filepath.Join(root, "frp_win", "frpc.exe")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected binary at %s: %v", expectedPath, err)
	}
}

// --- Test 6: Network timeout on archive download → StatusFailed ---

func TestDownload_NetworkTimeout_StatusFailed(t *testing.T) {
	// API endpoint resolves fine; the archive path hangs until client timeout.
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(releaseJSON("v"+frpTestVersion,
				"frp_"+frpTestVersion+"_linux_amd64.tar.gz", srv.URL+"/archive")))
			return
		}
		// /archive hangs indefinitely.
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	root := t.TempDir()
	m := New(root, discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"
	// T-025：archive 路径 hang 不发 header，靠 ResponseHeaderTimeout 兜底；
	// 注入到 downloadClient.Transport 而非 Client.Timeout（拆分后 Client.Timeout=0）。
	m.downloadClient = &http.Client{
		Transport: &http.Transport{ResponseHeaderTimeout: 50 * time.Millisecond},
	}

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait up to 2s (timeout is 50ms, so it should fail very quickly).
	st := waitForDone(t, m, "frpc", 2*time.Second)
	if st.Status != StatusFailed {
		t.Errorf("expected status=%s after timeout, got %s", StatusFailed, st.Status)
	}
	if st.Error == "" {
		t.Error("expected non-empty error message on timeout")
	}
}

// --- itoa helper (avoids fmt import in this file) ---

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

// --- Test 7: Bad kind returns ErrBadKind without starting goroutine ---

func TestDownload_BadKind(t *testing.T) {
	m := New(t.TempDir(), nil)
	err := m.Start("invalid")
	if err != ErrBadKind {
		t.Errorf("want ErrBadKind, got %v", err)
	}
}

// --- Test 8: ParseIPFromJSON parses expected JSON formats ---

func TestParseIPFromJSON(t *testing.T) {
	ip, err := ParseIPFromJSON([]byte(`{"ip":"1.2.3.4"}`))
	if err != nil || ip != "1.2.3.4" {
		t.Errorf("ParseIPFromJSON: ip=%s err=%v", ip, err)
	}
	_, err = ParseIPFromJSON([]byte(`{"ip":""}`))
	if err == nil {
		t.Error("want error for empty ip")
	}
	_, err = ParseIPFromJSON([]byte(`not json`))
	if err == nil {
		t.Error("want error for invalid json")
	}
}

// --- Test 9 (T-014 AC-6): latest 解析成功 —— API 返回最新 tag + 匹配资产 ---

func TestResolveLatest_Success(t *testing.T) {
	// 归档内条目用任意版本字面量；下载 URL 由 API 响应解析得到。
	archiveContent := buildTarGz(t, map[string]string{
		"frp_9.9.9_linux_amd64/frps": "#!/bin/sh\necho frps binary",
	})
	// API 响应里 tag 故意用一个"新"版本号，验证下载 URL 完全来自 API、不依赖任何写死常量。
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(releaseJSON("v9.9.9", "frp_9.9.9_linux_amd64.tar.gz", srv.URL+"/dl")))
		case r.URL.Path == "/dl":
			w.Header().Set("Content-Length", itoa(len(archiveContent)))
			w.WriteHeader(http.StatusOK)
			w.Write(archiveContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	root := t.TempDir()
	m := New(root, discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"

	if err := m.Start("frps"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	st := waitForDone(t, m, "frps", 5*time.Second)
	if st.Status != StatusSuccess {
		t.Errorf("expected status=%s, got %s (error=%q)", StatusSuccess, st.Status, st.Error)
	}
	if _, err := os.Stat(filepath.Join(root, "frp_linux", "frps")); err != nil {
		t.Errorf("expected frps binary installed: %v", err)
	}
}

// --- Test 10 (T-014 AC-6): API 限流 403 → failed + 中文"限流"消息 ---

func TestResolveLatest_RateLimited403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 限流响应体也是合法 JSON（实现必须先判状态码、后解析 JSON）。
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer srv.Close()

	m := New(t.TempDir(), discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	st := waitForDone(t, m, "frpc", 5*time.Second)
	if st.Status != StatusFailed {
		t.Errorf("expected status=%s, got %s", StatusFailed, st.Status)
	}
	if !strings.Contains(st.Error, "限流") {
		t.Errorf("expected error to mention 限流, got %q", st.Error)
	}
}

// --- Test 11 (T-014 AC-6): 资产未匹配 → failed + 中文"未找到匹配"消息 ---

func TestResolveLatest_AssetNotMatched(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// assets[] 里只有 arm64 / darwin 资产，无 linux amd64 后缀项。
		w.Write([]byte(`{"tag_name":"v1.0.0","assets":[` +
			`{"name":"frp_1.0.0_linux_arm64.tar.gz","browser_download_url":"http://x/a"},` +
			`{"name":"frp_1.0.0_darwin_amd64.tar.gz","browser_download_url":"http://x/b"}]}`))
	}))
	defer srv.Close()

	m := New(t.TempDir(), discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	st := waitForDone(t, m, "frpc", 5*time.Second)
	if st.Status != StatusFailed {
		t.Errorf("expected status=%s, got %s", StatusFailed, st.Status)
	}
	if !strings.Contains(st.Error, "未找到匹配") {
		t.Errorf("expected error to mention 未找到匹配, got %q", st.Error)
	}
}

// --- Test 12 (T-014 AC-6): latest API 网络失败 → failed ---

func TestResolveLatest_NetworkFailure(t *testing.T) {
	// 启动后立即关闭 server，使 API 查询请求遭遇连接失败。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	apiURL := srv.URL
	srv.Close() // 此后 apiURL 不可达

	m := New(t.TempDir(), discardLogger())
	m.apiBaseURL = apiURL
	m.goos = "linux"
	// T-025：API 路径走 apiClient；server 已关闭 → dial 直接返回 ECONNREFUSED。
	m.apiClient = &http.Client{Timeout: 2 * time.Second}

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	st := waitForDone(t, m, "frpc", 5*time.Second)
	if st.Status != StatusFailed {
		t.Errorf("expected status=%s, got %s", StatusFailed, st.Status)
	}
	if !strings.Contains(st.Error, "无法访问 GitHub") {
		t.Errorf("expected error to mention 无法访问 GitHub, got %q", st.Error)
	}
}
