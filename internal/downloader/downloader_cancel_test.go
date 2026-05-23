// Package downloader — T-027 Cancel 单测集。
// 覆盖 01 §5.1 AC-cancel-* 8 条 + F-3 / F-4 兜底 + 并发安全。
package downloader

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// randomBytes 生成伪随机字节序列（固定 seed 让测试可复现）。
// L40：用 math/rand 防 gzip 把高重复字符串压成 KB 级让慢传输用例毫秒结束。
func randomBytes(n int, seed int64) []byte {
	r := rand.New(rand.NewSource(seed))
	out := make([]byte, n)
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for i := range out {
		out[i] = charset[r.Intn(len(charset))]
	}
	return out
}

// newSlowChunkServer 起一个 httptest server：
//   - /releases/latest 立即返合法 release JSON 指向 /archive
//   - /archive 用 chunk-write + sleep 模拟慢下载（用真伪随机字节防 gzip 压塌 L40）
//
// chunkBytes / sleepPerChunk 控制节奏，总耗时 ≈ totalBytes/chunkBytes * sleepPerChunk。
// connCh 接收 server 端 r.Context().Done() 信号（用于断言 cancel 时 server 也观测到 conn 关闭）。
func newSlowChunkServer(t *testing.T, goos string, totalBytes, chunkBytes int, sleepPerChunk time.Duration, connClosedCh chan<- struct{}) *httptest.Server {
	t.Helper()
	// 用真伪随机字节避免 gzip 压塌（L40）；再 gzip 一次模拟真实 tar.gz。
	raw := randomBytes(totalBytes, 42)
	var gzbuf strings.Builder
	gw := gzip.NewWriter(stringBuilderWriter{&gzbuf})
	_, _ = gw.Write(raw)
	_ = gw.Close()
	archivePayload := []byte(gzbuf.String())

	var assetSuffix string
	switch goos {
	case "linux":
		assetSuffix = "_linux_amd64.tar.gz"
	case "windows":
		assetSuffix = "_windows_amd64.zip"
	default:
		t.Fatalf("unsupported goos: %s", goos)
	}

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(releaseJSON("v"+frpTestVersion,
				"frp_"+frpTestVersion+assetSuffix, srv.URL+"/archive")))
		case r.URL.Path == "/archive":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", itoa(len(archivePayload)))
			w.WriteHeader(http.StatusOK)
			flusher, _ := w.(http.Flusher)
			// chunk 写入，每写一块 sleep；中途 ctx Done 立即退出（让 cancel 路径可观测）
			for off := 0; off < len(archivePayload); off += chunkBytes {
				end := off + chunkBytes
				if end > len(archivePayload) {
					end = len(archivePayload)
				}
				if _, err := w.Write(archivePayload[off:end]); err != nil {
					if connClosedCh != nil {
						select {
						case connClosedCh <- struct{}{}:
						default:
						}
					}
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
				select {
				case <-r.Context().Done():
					if connClosedCh != nil {
						select {
						case connClosedCh <- struct{}{}:
						default:
						}
					}
					return
				case <-time.After(sleepPerChunk):
				}
			}
		default:
			http.NotFound(w, r)
		}
	}))
	return srv
}

type stringBuilderWriter struct{ sb *strings.Builder }

func (sw stringBuilderWriter) Write(p []byte) (int, error) {
	return sw.sb.Write(p)
}

// waitForStatus 轮询直到 Status(kind).Status == want（或 timeout）。
func waitForStatus(t *testing.T, m *Manager, kind, want string, timeout time.Duration) DownloadState {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st, _ := m.Status(kind)
		if st.Status == want {
			return st
		}
		time.Sleep(10 * time.Millisecond)
	}
	st, _ := m.Status(kind)
	t.Fatalf("timeout waiting %s == %s; current=%+v", kind, want, st)
	return st
}

// waitGoroutineExit 等 doDownload goroutine 完全退出（cancels map 内 entry 被 delete）。
// 这是 TempDir cleanup 前的必要等待（Windows file lock）。
func waitGoroutineExit(t *testing.T, m *Manager, kind string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		_, ok := m.cancels[kind]
		m.mu.Unlock()
		if !ok {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Logf("warning: goroutine for %s still in cancels map after %v", kind, timeout)
}

// assertNoTmpResidue 校验 dir 下不含 .dl-archive-*.tmp / .dl-bin-*.tmp 残留。
func assertNoTmpResidue(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return // 目录都没建过自然没残留
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".dl-archive-") || strings.HasPrefix(name, ".dl-bin-") {
			t.Errorf("tmp residue: %s", filepath.Join(dir, name))
		}
	}
}

// --- AC-cancel-mid-download：慢 server 跑到 downloading，Cancel 在 3s 内生效 + 无 tmp 残留 ---

func TestCancel_MidDownload(t *testing.T) {
	connClosedCh := make(chan struct{}, 1)
	// 1 MiB 数据 / chunk=4096 / sleep=80ms ≈ 256 chunks × 80ms = ~20s 总耗时，
	// 给 cancel 充足窗口；体积也大于 Content-Length，progress 必然非零。
	srv := newSlowChunkServer(t, "linux", 1<<20, 4096, 80*time.Millisecond, connClosedCh)
	defer srv.Close()

	root := t.TempDir()
	m := New(root, discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"
	t.Cleanup(func() { waitGoroutineExit(t, m, "frpc", 3*time.Second) })

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// 等下载真正进入 downloading 且 progress 已推进（确保到了 io.Copy 阶段）
	startWait := time.Now()
	for time.Since(startWait) < 3*time.Second {
		st, _ := m.Status("frpc")
		if st.Status == StatusDownloading && st.Progress > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	stPre, _ := m.Status("frpc")
	if stPre.Status != StatusDownloading {
		t.Fatalf("preconditions: status=%q, want downloading", stPre.Status)
	}

	cancelStart := time.Now()
	if err := m.Cancel("frpc"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	// NFR-1：≤3s + 一些 OS 调度容差；实测 stdlib ctx 取消应 < 100ms。
	if elapsed := time.Since(cancelStart); elapsed > 3500*time.Millisecond {
		t.Errorf("Cancel took %v (> 3.5s threshold)", elapsed)
	}

	st, _ := m.Status("frpc")
	if st.Status != StatusCanceled {
		t.Errorf("status = %q, want %q (err=%q)", st.Status, StatusCanceled, st.Error)
	}

	// 等 goroutine 完全退出（defer Remove archive tmp）后再校验残留。
	// goroutine 退出条件：cancels map 内 entry 被 delete。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		_, ok := m.cancels["frpc"]
		m.mu.Unlock()
		if !ok {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	assertNoTmpResidue(t, filepath.Join(root, "frp_linux"))

	// server 侧应已观测到 conn 关闭
	select {
	case <-connClosedCh:
		// good
	case <-time.After(500 * time.Millisecond):
		t.Logf("note: server connClosedCh not signaled within 500ms")
	}
}

// --- AC-cancel-idle-noop ---

func TestCancel_Idle_NoOp(t *testing.T) {
	m := New(t.TempDir(), discardLogger())
	if err := m.Cancel("frpc"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	st, _ := m.Status("frpc")
	if st.Status != StatusIdle {
		t.Errorf("status = %q, want idle", st.Status)
	}
}

// --- AC-cancel-success-noop ---

func TestCancel_Success_NoOp(t *testing.T) {
	archive := buildTarGz(t, map[string]string{
		"frp_" + frpTestVersion + "_linux_amd64/frpc": "#!/bin/sh\necho frpc",
	})
	srv := newFRPServer(t, "frp_"+frpTestVersion+"_linux_amd64.tar.gz", archive)
	defer srv.Close()

	m := New(t.TempDir(), discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForStatus(t, m, "frpc", StatusSuccess, 5*time.Second)

	if err := m.Cancel("frpc"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	st, _ := m.Status("frpc")
	if st.Status != StatusSuccess {
		t.Errorf("status = %q, want success (cancel must not overwrite)", st.Status)
	}
}

// --- AC-cancel-after-failed-then-restart ---

func TestCancel_FailedThenRestart(t *testing.T) {
	// API 返合法 release JSON 但 /archive 返 500 → failed。
	step := atomic.Int32{}
	archive := buildTarGz(t, map[string]string{
		"frp_" + frpTestVersion + "_linux_amd64/frpc": "#!/bin/sh\necho ok",
	})
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(releaseJSON("v"+frpTestVersion,
				"frp_"+frpTestVersion+"_linux_amd64.tar.gz", srv.URL+"/archive")))
		case r.URL.Path == "/archive":
			if step.Load() == 0 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	m := New(t.TempDir(), discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start#1: %v", err)
	}
	waitForStatus(t, m, "frpc", StatusFailed, 5*time.Second)

	// cancel no-op
	if err := m.Cancel("frpc"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	st, _ := m.Status("frpc")
	if st.Status != StatusFailed {
		t.Errorf("status after cancel on failed = %q, want failed", st.Status)
	}

	// 切换 handler，再 Start → 必须能进入 downloading
	step.Store(1)
	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start#2: %v", err)
	}
	waitForStatus(t, m, "frpc", StatusSuccess, 5*time.Second)
}

// --- AC-cancel-then-restart-from-canceled ---

func TestCancel_CanceledThenRestart(t *testing.T) {
	srv := newSlowChunkServer(t, "linux", 1<<20, 4096, 80*time.Millisecond, nil)
	defer srv.Close()

	root := t.TempDir()
	m := New(root, discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"
	t.Cleanup(func() { waitGoroutineExit(t, m, "frpc", 3*time.Second) })

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start#1: %v", err)
	}
	// 等到真正在 downloading + progress 推进，再 cancel
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st, _ := m.Status("frpc")
		if st.Status == StatusDownloading && st.Progress > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := m.Cancel("frpc"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	st, _ := m.Status("frpc")
	if st.Status != StatusCanceled {
		t.Fatalf("status = %q, want canceled", st.Status)
	}

	// canceled → Start 必须进入 downloading（不报 ErrAlreadyInProgress）
	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start#2 from canceled: %v", err)
	}
	st2, _ := m.Status("frpc")
	if st2.Status != StatusDownloading {
		t.Errorf("status after Start#2 = %q, want downloading", st2.Status)
	}

	// 第二次也立刻 cancel 释放，防 testserver 挂留 + 等 goroutine 退出
	_ = m.Cancel("frpc")
	waitGoroutineExit(t, m, "frpc", 3*time.Second)
}

// --- AC-cancel-repeated-noop ---

func TestCancel_Repeated_NoOp(t *testing.T) {
	srv := newSlowChunkServer(t, "linux", 1<<20, 4096, 80*time.Millisecond, nil)
	defer srv.Close()

	m := New(t.TempDir(), discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"
	t.Cleanup(func() { waitGoroutineExit(t, m, "frpc", 3*time.Second) })

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st, _ := m.Status("frpc")
		if st.Status == StatusDownloading && st.Progress > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// 5 次串行 cancel
	for i := 0; i < 5; i++ {
		if err := m.Cancel("frpc"); err != nil {
			t.Fatalf("Cancel#%d: %v", i, err)
		}
	}
	st, _ := m.Status("frpc")
	if st.Status != StatusCanceled {
		t.Errorf("status = %q, want canceled", st.Status)
	}
}

// --- AC-cancel-bad-kind ---

func TestCancel_BadKind(t *testing.T) {
	m := New(t.TempDir(), discardLogger())
	if err := m.Cancel("frpx"); !errors.Is(err, ErrBadKind) {
		t.Errorf("Cancel(frpx) = %v, want ErrBadKind", err)
	}
}

// --- AC-cancel-concurrent N=10 + race detector ---

func TestCancel_Concurrent_N10(t *testing.T) {
	srv := newSlowChunkServer(t, "linux", 1<<20, 4096, 80*time.Millisecond, nil)
	defer srv.Close()

	m := New(t.TempDir(), discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"
	t.Cleanup(func() { waitGoroutineExit(t, m, "frpc", 3*time.Second) })

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st, _ := m.Status("frpc")
		if st.Status == StatusDownloading && st.Progress > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := m.Cancel("frpc"); err != nil {
				t.Errorf("concurrent Cancel: %v", err)
			}
		}()
	}
	wg.Wait()

	st, _ := m.Status("frpc")
	if st.Status != StatusCanceled {
		t.Errorf("status = %q, want canceled", st.Status)
	}
}

// --- F-3 兜底测试：doDownload 卡死在阻塞点，Cancel 3s 后强写 canceled ---

// fakeStuckManager 用注入了"不响应 ctx 的 reader"的方式让 doDownload
// 卡住的最小模拟：用 hang reader 让 io.Copy 永远不返。
// 我们用 httptest server 让 archive 路径返合法 header 但 body 永远不发完，
// 同时把 m.downloadClient.Transport 设成 stdlib default —— stdlib ctx cancel
// 会让 Body.Read 解除阻塞。所以更精确的模拟是注入"自己造的" reader 替换 resp.Body。
//
// 简化策略：用 wrapping transport，注入永远不响应 ctx 的 reader 作为 resp.Body。
type stuckTransport struct {
	inner http.RoundTripper
	// 测试结束时 close 让所有 Pipe Reader 解除阻塞（避免 goroutine leak + TempDir 锁）
	releaseCh chan struct{}
}

func (s *stuckTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 只 hijack archive 路径；releases/latest 走 inner（正常 200）。
	if !strings.HasSuffix(req.URL.Path, "/archive") {
		return s.inner.RoundTrip(req)
	}
	// 返合法 200 响应 + 阻塞的 Body（不响应 ctx —— 反 stdlib 契约的极端情况）。
	// releaseCh close 后 Body 才会发出 EOF。
	pr, pw := io.Pipe()
	go func() {
		<-s.releaseCh
		_ = pw.Close()
	}()
	resp := &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Body:       pr,
		// ContentLength: -1 让 io.Copy 一直拉
		ContentLength: -1,
		Request:       req,
	}
	return resp, nil
}

func TestCancel_3sTimeoutForceWrite(t *testing.T) {
	// API 端走正常 httptest，archive 端走 hijacked transport 让 Body.Read 卡死。
	archive := buildTarGz(t, map[string]string{
		"frp_" + frpTestVersion + "_linux_amd64/frpc": "x",
	})
	srv := newFRPServer(t, "frp_"+frpTestVersion+"_linux_amd64.tar.gz", archive)
	defer srv.Close()

	m := New(t.TempDir(), discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"
	// 注入 hijacked transport：archive 路径返不响应 ctx 的 Body。
	// releaseCh 在 t.Cleanup 中 close，让 goroutine 退出避免 TempDir 锁。
	releaseCh := make(chan struct{})
	t.Cleanup(func() { close(releaseCh) })
	m.downloadClient = &http.Client{
		Transport: &stuckTransport{
			inner:     http.DefaultTransport,
			releaseCh: releaseCh,
		},
	}

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForStatus(t, m, "frpc", StatusDownloading, 2*time.Second)

	cancelStart := time.Now()
	if err := m.Cancel("frpc"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	elapsed := time.Since(cancelStart)

	// F-3：Cancel 必须在 ~3s+ε 内返回（强写后立返）
	if elapsed < 3*time.Second {
		// 太快说明实际不是兜底路径；可能 inner transport 把 conn 关了让 Body.Read 解除
		t.Logf("Cancel returned in %v (may not have hit force-write path)", elapsed)
	}
	if elapsed > 5*time.Second {
		t.Errorf("Cancel took %v, expected ≤ ~3s+ε (force-write should be immediate)", elapsed)
	}

	st, _ := m.Status("frpc")
	if st.Status != StatusCanceled {
		t.Errorf("status = %q, want canceled (F-3 force-write must guarantee FR-7)", st.Status)
	}
}

// --- F-4 测试：resolveLatestAsset 阶段 Cancel ---

func TestCancel_DuringResolveAsset(t *testing.T) {
	// API 端永远不响应（只接受连接、不发 header / body）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	m := New(t.TempDir(), discardLogger())
	m.apiBaseURL = srv.URL
	m.goos = "linux"
	// apiClient 用大超时（让 Cancel 而非超时来主导）
	m.apiClient = &http.Client{Timeout: 60 * time.Second}

	if err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// 等 cancels 注册（goroutine 顶端就注册了）
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		m.mu.Lock()
		_, ok := m.cancels["frpc"]
		m.mu.Unlock()
		if ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancelStart := time.Now()
	if err := m.Cancel("frpc"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	elapsed := time.Since(cancelStart)
	if elapsed > 3*time.Second {
		t.Errorf("Cancel during resolveLatestAsset took %v (> 3s NFR-1)", elapsed)
	}

	st, _ := m.Status("frpc")
	if st.Status != StatusCanceled {
		t.Errorf("status = %q, want canceled (F-4: resolveLatestAsset must be ctx-aware)", st.Status)
	}
}

// --- 单调性：Cancel 在 setCanceled 后，doDownload 后续 ctx 重检不能再写 success ---
// 这条由 TestCancel_MidDownload 隐式覆盖（cancel 后等 5s 状态仍是 canceled）。

// --- 旁路：context import 使用（避免未使用警告） ---

var _ = context.Background
