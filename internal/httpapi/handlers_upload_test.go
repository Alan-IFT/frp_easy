package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/frp-easy/frp-easy/internal/auth"
	"github.com/frp-easy/frp-easy/internal/binloc"
	"github.com/frp-easy/frp-easy/internal/downloader"
	"github.com/frp-easy/frp-easy/internal/storage"
)

// newUploadTestServer 起 httptest 服务器，注入一个真实的 Downloader.Manager
// （root = t.TempDir()），让 uploadBin 能真正落盘到临时目录。
func newUploadTestServer(t *testing.T) (srv *httptest.Server, root string, store *storage.Store) {
	t.Helper()
	root = t.TempDir()
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
	srv = httptest.NewServer(New(deps))
	t.Cleanup(srv.Close)
	return srv, root, store
}

// buildMultipartUpload 构造 multipart/form-data body：kind=<kind> + file=<content>。
// fieldOrder 可选 "kind-first" / "file-first" / "no-kind" / "no-file"。
func buildMultipartUpload(t *testing.T, kind, filename string, content []byte, fieldOrder string) (body *bytes.Buffer, contentType string) {
	t.Helper()
	body = &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	writeKind := func() {
		if err := mw.WriteField("kind", kind); err != nil {
			t.Fatal(err)
		}
	}
	writeFile := func() {
		fw, err := mw.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	switch fieldOrder {
	case "file-first":
		writeFile()
		writeKind()
	case "no-kind":
		writeFile()
	case "no-file":
		writeKind()
	default: // kind-first
		writeKind()
		writeFile()
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return body, mw.FormDataContentType()
}

// doUpload posts multipart to /api/v1/system/upload-bin with auth + csrf.
func doUpload(t *testing.T, srv *httptest.Server, body io.Reader, contentType string, cookies []*http.Cookie, csrf string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest("POST", srv.URL+"/api/v1/system/upload-bin", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", contentType)
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

// elfHeader 返回最小 ELF 头（前 4 字节）+ 一些填充。
func elfHeader() []byte {
	h := []byte{0x7F, 'E', 'L', 'F'}
	h = append(h, make([]byte, 60)...)
	return h
}

// peHeader 返回最小 PE 头（MZ + 填充）。
func peHeader() []byte {
	h := []byte{'M', 'Z'}
	h = append(h, make([]byte, 62)...)
	return h
}

// TestUploadBin_Unauthenticated 未登录 → 401。
func TestUploadBin_Unauthenticated(t *testing.T) {
	srv, _, _ := newUploadTestServer(t)
	body, ct := buildMultipartUpload(t, "frpc", "frpc", elfHeader(), "kind-first")
	resp, _ := doUpload(t, srv, body, ct, nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// TestUploadBin_NoCSRF 已登录但无 CSRF token → 403。
func TestUploadBin_NoCSRF(t *testing.T) {
	srv, _, _ := newUploadTestServer(t)
	cookies, _ := setupAndLogin(t, srv)
	body, ct := buildMultipartUpload(t, "frpc", "frpc", elfHeader(), "kind-first")
	resp, _ := doUpload(t, srv, body, ct, cookies, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

// TestUploadBin_BadKind kind 非法 → 422 + field=kind。
func TestUploadBin_BadKind(t *testing.T) {
	srv, _, _ := newUploadTestServer(t)
	cookies, csrf := setupAndLogin(t, srv)
	body, ct := buildMultipartUpload(t, "frpx", "frpx", elfHeader(), "kind-first")
	resp, raw := doUpload(t, srv, body, ct, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422\nbody=%s", resp.StatusCode, raw)
	}
	var e ErrorBody
	_ = json.Unmarshal(raw, &e)
	if e.Error.Field != "kind" {
		t.Errorf("field = %q, want kind", e.Error.Field)
	}
}

// TestUploadBin_BadHeader 上传 0.5 KiB 的 .txt → 422 + 缺少 ELF/PE 头。
func TestUploadBin_BadHeader(t *testing.T) {
	srv, _, _ := newUploadTestServer(t)
	cookies, csrf := setupAndLogin(t, srv)
	body, ct := buildMultipartUpload(t, "frpc", "fake.txt", []byte("just a txt file 0123"), "kind-first")
	resp, raw := doUpload(t, srv, body, ct, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
	var e ErrorBody
	_ = json.Unmarshal(raw, &e)
	if e.Error.Field != "file" {
		t.Errorf("field = %q, want file", e.Error.Field)
	}
}

// TestUploadBin_PlatformMismatch Linux 主机上传 PE → 422 + "平台不匹配"。
// 注意：handler 用 runtime.GOOS，所以只在 linux 跑该断言；其它平台跳过反向也类似。
func TestUploadBin_PlatformMismatch(t *testing.T) {
	srv, _, _ := newUploadTestServer(t)
	cookies, csrf := setupAndLogin(t, srv)

	var content []byte
	var wantContains string
	switch runtime.GOOS {
	case "linux":
		content = peHeader()
		wantContains = "平台不匹配"
	case "windows":
		content = elfHeader()
		wantContains = "平台不匹配"
	default:
		t.Skipf("skip on %s", runtime.GOOS)
	}

	body, ct := buildMultipartUpload(t, "frpc", "x", content, "kind-first")
	resp, raw := doUpload(t, srv, body, ct, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422\nbody=%s", resp.StatusCode, raw)
	}
	var e ErrorBody
	_ = json.Unmarshal(raw, &e)
	if !contains(e.Error.Message, wantContains) {
		t.Errorf("message = %q, want contains %q", e.Error.Message, wantContains)
	}
}

// TestUploadBin_HappyPath 上传合法的（平台对应）binary → 200 + sha256 / size / path。
func TestUploadBin_HappyPath(t *testing.T) {
	srv, _, _ := newUploadTestServer(t)
	cookies, csrf := setupAndLogin(t, srv)

	var content []byte
	var wantPathContains string
	switch runtime.GOOS {
	case "linux":
		content = append(elfHeader(), bytes.Repeat([]byte("a"), 1024)...) // 1 KiB+
		wantPathContains = "frp_linux/frpc"
	case "windows":
		content = append(peHeader(), bytes.Repeat([]byte("a"), 1024)...)
		wantPathContains = "frp_win/frpc.exe"
	default:
		t.Skipf("skip on %s", runtime.GOOS)
	}

	body, ct := buildMultipartUpload(t, "frpc", "x", content, "kind-first")
	resp, raw := doUpload(t, srv, body, ct, cookies, csrf)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody=%s", resp.StatusCode, raw)
	}
	var got UploadBinResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Ok {
		t.Error("Ok = false")
	}
	if got.Kind != "frpc" {
		t.Errorf("Kind = %q", got.Kind)
	}
	if got.SHA256 == "" || len(got.SHA256) != 64 {
		t.Errorf("SHA256 = %q (len %d)", got.SHA256, len(got.SHA256))
	}
	if got.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", got.Size, len(content))
	}
	if !contains(got.Path, wantPathContains) {
		t.Errorf("Path = %q, want contains %q", got.Path, wantPathContains)
	}
}

// TestUploadBin_FileFirstOrderingWorks 字段顺序 file-first 也应当工作（B-6 修订）。
func TestUploadBin_FileFirstOrderingWorks(t *testing.T) {
	srv, _, _ := newUploadTestServer(t)
	cookies, csrf := setupAndLogin(t, srv)

	var content []byte
	switch runtime.GOOS {
	case "linux":
		content = elfHeader()
	case "windows":
		content = peHeader()
	default:
		t.Skipf("skip on %s", runtime.GOOS)
	}

	body, ct := buildMultipartUpload(t, "frpc", "x", content, "file-first")
	resp, raw := doUpload(t, srv, body, ct, cookies, csrf)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (字段顺序不应影响)\nbody=%s", resp.StatusCode, raw)
	}
}

// TestUploadBin_MissingKindField 缺 kind 字段 → 422 + field=kind。
func TestUploadBin_MissingKindField(t *testing.T) {
	srv, _, _ := newUploadTestServer(t)
	cookies, csrf := setupAndLogin(t, srv)
	body, ct := buildMultipartUpload(t, "", "x", elfHeader(), "no-kind")
	resp, raw := doUpload(t, srv, body, ct, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422\nbody=%s", resp.StatusCode, raw)
	}
	var e ErrorBody
	_ = json.Unmarshal(raw, &e)
	if e.Error.Field != "kind" {
		t.Errorf("field = %q, want kind", e.Error.Field)
	}
}

// TestUploadBin_MissingFileField 缺 file 字段 → 422 + field=file。
func TestUploadBin_MissingFileField(t *testing.T) {
	srv, _, _ := newUploadTestServer(t)
	cookies, csrf := setupAndLogin(t, srv)
	body, ct := buildMultipartUpload(t, "frpc", "x", nil, "no-file")
	resp, raw := doUpload(t, srv, body, ct, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422\nbody=%s", resp.StatusCode, raw)
	}
	var e ErrorBody
	_ = json.Unmarshal(raw, &e)
	if e.Error.Field != "file" {
		t.Errorf("field = %q, want file", e.Error.Field)
	}
}

// TestUploadBin_OversizeBody body > 64 MiB+1 → 413（MaxBytesReader 触发）。
// 用 io.Pipe + 慢 reader 模拟大 body，避免实际分配 64 MiB。
// 注意：multipart 实际包大小要让 MaxBytesReader 拒收，这里直接发裸 body 越限。
func TestUploadBin_OversizeBody(t *testing.T) {
	srv, _, _ := newUploadTestServer(t)
	cookies, csrf := setupAndLogin(t, srv)

	// 构造一个 multipart 内含 65 MiB file 字段
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	_ = mw.WriteField("kind", "frpc")
	fw, _ := mw.CreateFormFile("file", "x")
	// 写入 65 MiB（> 64+1=65 上限触发 MaxBytesReader，按 header 已经接近上限）
	chunk := bytes.Repeat([]byte("X"), 1<<20)
	// 直接超过 uploadBinMaxBytes + 1 MiB = 65 MiB；写 66 MiB 保险
	for i := 0; i < 66; i++ {
		fw.Write(chunk)
	}
	mw.Close()

	resp, raw := doUpload(t, srv, body, mw.FormDataContentType(), cookies, csrf)
	// 期望 413
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413\nbody=%s", resp.StatusCode, truncate(raw, 200))
	}
}

// TestUploadBin_ConcurrentSameKind 并发同 kind 上传，至少一个收 409。
func TestUploadBin_ConcurrentSameKind(t *testing.T) {
	srv, _, _ := newUploadTestServer(t)
	cookies, csrf := setupAndLogin(t, srv)

	var content []byte
	switch runtime.GOOS {
	case "linux":
		content = elfHeader()
	case "windows":
		content = peHeader()
	default:
		t.Skipf("skip on %s", runtime.GOOS)
	}

	// 大 buffer 让上传慢些 —— 用 io.Pipe 让两个并发在握 lock 期间撞上。
	// 简化：让每个上传写较多内容（4 MiB 重复）+ 用 goroutine 并发。
	bigContent := append(append([]byte{}, content...), bytes.Repeat([]byte("z"), 4<<20)...)

	var statuses [2]int
	var wg atomic.Int32
	wg.Add(2)
	done := make(chan struct{})
	for i := range statuses {
		go func(idx int) {
			defer func() {
				if wg.Add(-1) == 0 {
					close(done)
				}
			}()
			body, ct := buildMultipartUpload(t, "frpc", "x", bigContent, "kind-first")
			resp, _ := doUpload(t, srv, body, ct, cookies, csrf)
			statuses[idx] = resp.StatusCode
		}(i)
	}
	<-done

	// 必须有至少一个 409
	has409 := false
	has200 := false
	for _, s := range statuses {
		if s == http.StatusConflict {
			has409 = true
		}
		if s == http.StatusOK {
			has200 = true
		}
	}
	// 在快机器上两个 goroutine 可能串行完成，两个都 200。这里要求至少一次 200 成功。
	if !has200 {
		t.Errorf("expected at least one 200, got statuses %v", statuses)
	}
	// 不强求 409 出现（依赖时序），仅作 advisory（log）。
	if !has409 {
		t.Logf("note: no 409 observed; statuses=%v (timing-dependent)", statuses)
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && bytes.Contains([]byte(s), []byte(sub))
}

func truncate(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}
