// Package downloader — T-025 adversarial timeout tests.
//
// 覆盖 02_SOLUTION_DESIGN.md §5.2 T1/T2/T3/T4 四类用例：
//   - T1 慢传输（~50 KB/s 真造）不被超时切断 → 证明 Client.Timeout=0 决策正确
//   - T2 死连接（不发 header）由 ResponseHeaderTimeout 兜底快速失败
//   - T3 GitHub API hang 由 apiClient.Timeout 短超时快速失败
//   - T4 双 kind 共享同一 Manager / root / server 串行各跑一遍 → AC-4 对称性
//
// 全部用 httptest.Server 真实 HTTP 行为（遵循 insight L29/L45 / RA AC-5 / Gate C-4）；
// 不引入 interface mock。
package downloader

import (
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// incompressibleBlob 返回长度为 n 的伪随机字节（用固定 seed 确保跨平台稳定）。
// 用于让 tar.gz 压缩后大小接近 n —— 重复字符 / 全空白会被 gzip 压到极小，
// 使 T1 慢传输用例在毫秒内就跑完、达不到 ~5s 真造目标。
func incompressibleBlob(n int) string {
	r := rand.New(rand.NewSource(42))
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[r.Intn(len(charset))]
	}
	return string(b)
}

// newSlowFRPServer 构造一个以指定 chunk + sleep 节奏分块写 archive 的 httptest server。
// /releases/latest 立即返回 release JSON；/archive 按 chunkSize 切片 + sleep
// 模拟国内 CDN 50–200 KB/s 速率（chunkSize=4096, sleep=80ms ≈ 50 KB/s）。
func newSlowFRPServer(t *testing.T, assetName string, archive []byte, chunkSize int, sleep time.Duration) *httptest.Server {
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
			flusher, _ := w.(http.Flusher)
			for off := 0; off < len(archive); off += chunkSize {
				end := off + chunkSize
				if end > len(archive) {
					end = len(archive)
				}
				if _, err := w.Write(archive[off:end]); err != nil {
					return // client closed
				}
				if flusher != nil {
					flusher.Flush()
				}
				if sleep > 0 && end < len(archive) {
					select {
					case <-r.Context().Done():
						return
					case <-time.After(sleep):
					}
				}
			}
		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

// --- T1: 慢传输不被超时切断（AC-1 核心证明） ---
//
// 用生产默认 downloadClient（不注入 m.downloadClient），server 以 ~50 KB/s
// 节奏写 ~256 KB（耗时约 5s）。若回归到旧的 Client.Timeout=60s 行为，本用例
// 仍能通过（5s < 60s）—— 但其根本目的不是证伪 60s，而是证明：
//   (a) 拆分后 downloadClient 仍能跑通完整下载 + 解压 + 安装链路；
//   (b) progressWriter 在长链路中持续推进（AC-6）。
func TestTimeout_SlowDownload_Succeeds(t *testing.T) {
	if testing.Short() {
		t.Skip("slow: ~5s real-time chunked write")
	}

	// 真造 archive：用伪随机不可压缩内容让 tar.gz 压缩后仍保持 ~256 KB
	// （重复字符会被 gzip 压到 KB 级，导致下载在毫秒内完成达不到 ~5s 真造目标）。
	bigBlob := incompressibleBlob(256 * 1024)
	archiveContent := buildTarGz(t, map[string]string{
		"frp_" + frpTestVersion + "_linux_amd64/frpc": bigBlob,
	})
	t.Logf("T1 archive size after gzip: %d bytes", len(archiveContent))

	srv := newSlowFRPServer(t, "frp_"+frpTestVersion+"_linux_amd64.tar.gz",
		archiveContent, 4096, 80*time.Millisecond)
	defer srv.Close()

	root := t.TempDir()
	m := New(root, discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"
	// 关键：不覆盖 m.downloadClient —— 用生产默认值。

	startedAt := time.Now()
	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// AC-6 spot check：在轮询过程中至少抓到一次 0 < progress < 100。
	var sawMidProgress bool
	deadline := time.Now().Add(15 * time.Second)
	var finalStatus DownloadState
	for time.Now().Before(deadline) {
		st, _ := m.Status("frpc")
		if st.Progress > 0 && st.Progress < 100 {
			sawMidProgress = true
		}
		if st.Status != StatusDownloading {
			finalStatus = st
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if finalStatus.Status == "" {
		t.Fatalf("timeout waiting for frpc download (15s); progress snapshot: %+v",
			func() DownloadState { st, _ := m.Status("frpc"); return st }())
	}

	elapsed := time.Since(startedAt)
	t.Logf("T1 elapsed: %s, sawMidProgress=%v", elapsed.Round(time.Millisecond), sawMidProgress)

	if finalStatus.Status != StatusSuccess {
		t.Errorf("expected status=%s, got %s (error=%q)", StatusSuccess, finalStatus.Status, finalStatus.Error)
	}
	if finalStatus.Progress != 100 {
		t.Errorf("expected progress=100, got %d", finalStatus.Progress)
	}
	if !sawMidProgress {
		t.Error("AC-6: expected to observe at least one intermediate 0<progress<100 snapshot during slow download")
	}
	// AC-1 真实下载 + 安装链路完整：二进制必须落盘。
	expectedPath := filepath.Join(root, "frp_linux", "frpc")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected binary at %s: %v", expectedPath, err)
	}
}

// --- T2: 死连接由 ResponseHeaderTimeout 兜底（AC-2） ---
//
// /archive 拿到请求后 select 等 r.Context().Done() 永不发任何 header；
// 注入 downloadClient.Transport.ResponseHeaderTimeout=100ms 让测试快速失败。
// Gate C-2：注入到 downloadClient 而非 apiClient（前者改 Transport 子字段，
// 后者改 Client.Timeout）。
func TestTimeout_DeadConnection_ResponseHeaderTimeout(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(releaseJSON("v"+frpTestVersion,
				"frp_"+frpTestVersion+"_linux_amd64.tar.gz", srv.URL+"/archive")))
			return
		}
		// /archive: hang，永不写 header。
		<-r.Context().Done()
	}))
	defer srv.Close()

	root := t.TempDir()
	m := New(root, discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"
	// Gate Q2：localhost 直连 dial/TLS 阶段 <1ms 不会触发，只设 ResponseHeaderTimeout 即可。
	m.downloadClient = &http.Client{
		Transport: &http.Transport{ResponseHeaderTimeout: 100 * time.Millisecond},
	}

	startedAt := time.Now()
	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	st := waitForDone(t, m, "frpc", 2*time.Second)
	t.Logf("T2 elapsed: %s", time.Since(startedAt).Round(time.Millisecond))

	if st.Status != StatusFailed {
		t.Errorf("expected status=%s, got %s", StatusFailed, st.Status)
	}
	if !strings.Contains(st.Error, "下载超时") {
		t.Errorf("expected error to contain 下载超时 prefix, got %q", st.Error)
	}
}

// --- T3: GitHub API hang 由 apiClient.Timeout 快速失败（AC-3） ---
//
// server 所有路径都 hang（包括 /releases/latest）；注入 apiClient.Timeout=200ms。
// 与既有 Test 12 互补：Test 12 测 "server 已关闭、TCP 立即拒绝"；T3 测 "server 接受
// 连接但永不应答"。两条物理失败路径都覆盖。
func TestTimeout_GitHubAPIHang_ApiClientTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	m := New(t.TempDir(), discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"
	m.apiClient = &http.Client{Timeout: 200 * time.Millisecond}

	startedAt := time.Now()
	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	st := waitForDone(t, m, "frpc", 2*time.Second)
	t.Logf("T3 elapsed: %s", time.Since(startedAt).Round(time.Millisecond))

	if st.Status != StatusFailed {
		t.Errorf("expected status=%s, got %s", StatusFailed, st.Status)
	}
	if !strings.Contains(st.Error, "无法访问 GitHub") {
		t.Errorf("expected error to contain '无法访问 GitHub', got %q", st.Error)
	}
}

// --- T4: 双 kind 对称性（AC-4） ---
//
// 隔离方案（C-6）：选 "一个 Manager + 同一 root + 同一 httptest server" 而非
// "两个 Manager + 两个 root"。理由：
//   1. 真正测 "frpc 与 frps 共享同一对 client 时双方均正常工作"（共享 state map +
//      共享 downloadClient/apiClient 实例）；
//   2. 两个独立 Manager 等于 T1 跑了两遍，对 "双 kind 一致" 没有任何独立证据；
//   3. 但因 server handler 只能返回单一 assetName，故 archive 内同时含 frpc 与
//      frps 两个 entry —— extractFromTarGz 按 filepath.Base 匹配，能正确各取所需。
//
// 因 T1 已证明慢传输不超时（耗时 ~5s），T4 用快传输（chunkSize=64KiB, sleep=0）
// 把单次缩到 ~50ms 量级，避免串行两次跑 10s 拉长 verify_all。
func TestTimeout_BothKinds_Symmetric(t *testing.T) {
	if testing.Short() {
		t.Skip("slow: serial double download with shared Manager")
	}

	// 同一 archive 同时含 frpc 与 frps，按 filepath.Base 匹配各取所需。
	archiveContent := buildTarGz(t, map[string]string{
		"frp_" + frpTestVersion + "_linux_amd64/frpc": "#!/bin/sh\necho frpc binary",
		"frp_" + frpTestVersion + "_linux_amd64/frps": "#!/bin/sh\necho frps binary",
	})
	srv := newFRPServer(t, "frp_"+frpTestVersion+"_linux_amd64.tar.gz", archiveContent)
	defer srv.Close()

	root := t.TempDir()
	m := New(root, discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"
	// 不覆盖 client —— 走生产默认值，证明默认 downloadClient 同时支持两个 kind。

	startedAt := time.Now()
	for _, kind := range []string{"frpc", "frps"} {
		if err := m.Start(kind); err != nil {
			t.Fatalf("Start(%s): %v", kind, err)
		}
		st := waitForDone(t, m, kind, 5*time.Second)
		if st.Status != StatusSuccess {
			t.Errorf("kind=%s: expected status=%s, got %s (error=%q)",
				kind, StatusSuccess, st.Status, st.Error)
		}
		if st.Progress != 100 {
			t.Errorf("kind=%s: expected progress=100, got %d", kind, st.Progress)
		}
		expectedPath := filepath.Join(root, "frp_linux", kind)
		if _, err := os.Stat(expectedPath); err != nil {
			t.Errorf("kind=%s: expected binary at %s: %v", kind, expectedPath, err)
		}
	}
	t.Logf("T4 total elapsed: %s", time.Since(startedAt).Round(time.Millisecond))
}

// TestNewDownloadHTTPClient_NoTotalTimeout 静态守门：直接断言下载专用 client
// 的 Client.Timeout == 0。这是 T-025 修复的根本字段，是 AC-1（慢源 ≥10 分钟
// 不超时）真正生效的唯一保证。
//
// Code Review P1-2：T1 ~3.8s 真造无法反向证伪"未来人误把 Timeout 改回 60s"
// 的回归；本测试就是那条防线——若有人误改 60s，本测试瞬间 FAIL。
func TestNewDownloadHTTPClient_NoTotalTimeout(t *testing.T) {
	c := newDownloadHTTPClient()
	if c.Timeout != 0 {
		t.Fatalf("downloadClient.Timeout 必须为 0（无总超时，T-025 修复核心）；实际 = %v", c.Timeout)
	}
}
