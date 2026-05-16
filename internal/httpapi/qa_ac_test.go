package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/frp-easy/frp-easy/internal/auth"
	"github.com/frp-easy/frp-easy/internal/downloader"
	"github.com/frp-easy/frp-easy/internal/storage"
)

// --- 公共辅助函数 ---

// newTestServerFull 启动测试用服务器。
// configPaths 非 nil 时配置 TOML 写入路径。
func newTestServerFull(t *testing.T, configPaths map[string]string, logFiles map[string]string) (*httptest.Server, *storage.Store) {
	t.Helper()
	store, err := storage.Open(t.TempDir())
	if err != nil && !errors.Is(err, storage.ErrCorruptReset) {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	rl := auth.NewRateLimiter(store)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	lf := map[string]string{"frpc": "", "frps": ""}
	if logFiles != nil {
		lf = logFiles
	}
	deps := Dependencies{
		Store:       store,
		Locator:     &fakeLoc{},
		ProcMgr:     nil,
		RateLimiter: rl,
		LogFiles:    lf,
		ConfigPaths: configPaths,
		Ready:       func() bool { return true },
		Logger:      logger,
		Version:     "qa-test-0.1.0",
	}
	srv := httptest.NewServer(New(deps))
	t.Cleanup(srv.Close)
	return srv, store
}

// setupAndLogin 初始化管理员并返回 session cookie + CSRF token。
func setupAndLogin(t *testing.T, srv *httptest.Server) ([]*http.Cookie, string) {
	t.Helper()
	resp, _ := doJSON(t, srv, "POST", "/api/v1/setup",
		map[string]string{"username": "admin", "password": "VerySafePass123"}, nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("setup failed: %d", resp.StatusCode)
	}
	cookies := resp.Cookies()
	_, body := doJSON(t, srv, "GET", "/api/v1/auth/csrf", nil, cookies, "")
	var csrf CSRFResponse
	_ = json.Unmarshal(body, &csrf)
	return cookies, csrf.CSRFToken
}

// --- AC-5: TCP 规则 → frpc.toml 写入 ---

func TestAC5_RenderFrpc_TomlWritten(t *testing.T) {
	// 假设：设置 serverAddr 后 CREATE PROXY 应生成 frpc.toml。
	dir := t.TempDir()
	frpcToml := filepath.Join(dir, "frpc.toml")
	frpsToml := filepath.Join(dir, "frps.toml")

	srv, _ := newTestServerFull(t, map[string]string{"frpc": frpcToml, "frps": frpsToml}, nil)
	cookies, csrf := setupAndLogin(t, srv)

	// frpc サーバー接続先を設定 → serverAddr 必須
	resp, body := doJSON(t, srv, "PUT", "/api/v1/client",
		map[string]any{"serverAddr": "10.0.0.1", "serverPort": 7000},
		cookies, csrf)
	if resp.StatusCode != 200 {
		t.Fatalf("PUT /client status %d body=%s", resp.StatusCode, body)
	}

	// TCP proxy 作成 → applyConfigBestEffort が呼ばれる
	rp := 6022
	resp, body = doJSON(t, srv, "POST", "/api/v1/proxies",
		map[string]any{
			"name": "ac5-ssh", "type": "tcp",
			"localPort": 22, "remotePort": rp,
		}, cookies, csrf)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /proxies status %d body=%s", resp.StatusCode, body)
	}

	// frpc.toml が書き込まれているか確認
	data, err := os.ReadFile(frpcToml)
	if err != nil {
		t.Fatalf("frpc.toml not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "serverAddr") {
		t.Errorf("frpc.toml missing serverAddr\ncontent=%s", content)
	}
	if !strings.Contains(content, "ac5-ssh") {
		t.Errorf("frpc.toml missing proxy name 'ac5-ssh'\ncontent=%s", content)
	}
	if !strings.Contains(content, "6022") {
		t.Errorf("frpc.toml missing remotePort 6022\ncontent=%s", content)
	}
}

// --- AC-5 frps.toml: PUT /server → frps.toml 写入 ---

func TestAC5_RenderFrps_TomlWritten(t *testing.T) {
	dir := t.TempDir()
	frpsToml := filepath.Join(dir, "frps.toml")

	srv, _ := newTestServerFull(t, map[string]string{"frpc": filepath.Join(dir, "frpc.toml"), "frps": frpsToml}, nil)
	cookies, csrf := setupAndLogin(t, srv)

	resp, body := doJSON(t, srv, "PUT", "/api/v1/server",
		map[string]any{"bindPort": 7777}, cookies, csrf)
	if resp.StatusCode != 200 {
		t.Fatalf("PUT /server status %d body=%s", resp.StatusCode, body)
	}

	data, err := os.ReadFile(frpsToml)
	if err != nil {
		t.Fatalf("frps.toml not written: %v", err)
	}
	if !strings.Contains(string(data), "7777") {
		t.Errorf("frps.toml missing bindPort 7777\ncontent=%s", string(data))
	}
}

// --- AC-6: 删除后 frpc.toml 中代理消失 ---

func TestAC6_ProxyDeleted_RemovedFromToml(t *testing.T) {
	// 仮説：DELETE 後に frpc.toml が再生成されプロキシエントリが削除されるはず。
	dir := t.TempDir()
	frpcToml := filepath.Join(dir, "frpc.toml")

	srv, _ := newTestServerFull(t, map[string]string{"frpc": frpcToml, "frps": filepath.Join(dir, "frps.toml")}, nil)
	cookies, csrf := setupAndLogin(t, srv)

	// serverAddr 設定
	doJSON(t, srv, "PUT", "/api/v1/client",
		map[string]any{"serverAddr": "10.0.0.2", "serverPort": 7000}, cookies, csrf)

	// プロキシ作成
	rp := 6099
	resp, body := doJSON(t, srv, "POST", "/api/v1/proxies",
		map[string]any{"name": "ac6-del", "type": "tcp", "localPort": 80, "remotePort": rp},
		cookies, csrf)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST status %d body=%s", resp.StatusCode, body)
	}
	var created ProxyResponse
	_ = json.Unmarshal(body, &created)

	// TOML に ac6-del が含まれることを確認
	data, _ := os.ReadFile(frpcToml)
	if !strings.Contains(string(data), "ac6-del") {
		t.Fatalf("proxy not in toml before delete: %s", string(data))
	}

	// 削除
	resp, body = doJSON(t, srv, "DELETE",
		fmt.Sprintf("/api/v1/proxies/%d", created.ID), nil, cookies, csrf)
	if resp.StatusCode != 200 {
		t.Fatalf("DELETE status %d body=%s", resp.StatusCode, body)
	}

	// TOML から ac6-del が消えることを確認
	data, err := os.ReadFile(frpcToml)
	if err != nil {
		t.Fatalf("frpc.toml not found after delete: %v", err)
	}
	if strings.Contains(string(data), "ac6-del") {
		t.Errorf("deleted proxy still in frpc.toml\ncontent=%s", string(data))
	}
}

// --- AC-9: persistMode 写入 KV ---

func TestAC9_PersistMode_KVUpdated(t *testing.T) {
	// 仮説：PUT /mode で frpc=true を設定した後、KV に "true" が永続化されるはず。
	srv, store := newTestServer(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	resp, body := doJSON(t, srv, "PUT", "/api/v1/mode",
		map[string]bool{"frpc": true, "frps": false}, cookies, csrf)
	if resp.StatusCode != 200 {
		t.Fatalf("PUT /mode status %d body=%s", resp.StatusCode, body)
	}

	// 直接 KV を確認
	v, ok, err := store.KVGet(context.Background(), "mode.frpc.enabled")
	if err != nil || !ok {
		t.Fatalf("mode.frpc.enabled not in KV: ok=%v err=%v", ok, err)
	}
	if v != "true" {
		t.Errorf("mode.frpc.enabled = %q, want 'true'", v)
	}

	v2, ok2, err2 := store.KVGet(context.Background(), "mode.frps.enabled")
	if err2 != nil || !ok2 {
		t.Fatalf("mode.frps.enabled not in KV: ok=%v err=%v", ok2, err2)
	}
	if v2 != "false" {
		t.Errorf("mode.frps.enabled = %q, want 'false'", v2)
	}
}

// --- AC-9: フリップ（true→false）も永続化される ---

func TestAC9_PersistMode_ToggleFlip(t *testing.T) {
	srv, store := newTestServer(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	// 一度 true に設定
	doJSON(t, srv, "PUT", "/api/v1/mode",
		map[string]bool{"frpc": true, "frps": true}, cookies, csrf)

	// 次に false に変更
	resp, body := doJSON(t, srv, "PUT", "/api/v1/mode",
		map[string]bool{"frpc": false, "frps": false}, cookies, csrf)
	if resp.StatusCode != 200 {
		t.Fatalf("PUT /mode (flip) status %d body=%s", resp.StatusCode, body)
	}

	v, _, _ := store.KVGet(context.Background(), "mode.frpc.enabled")
	if v != "false" {
		t.Errorf("mode.frpc.enabled after flip = %q, want 'false'", v)
	}
}

// --- AC-12: 损坏 DB → initialized=false，备份文件存在 ---

func TestAC12_CorruptDB_NotInitialized(t *testing.T) {
	// 仮説：data.db が破損している場合 initialized=false が返されるはず。
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "data.db"),
		[]byte("garbage-not-a-real-sqlite-file"), 0600); err != nil {
		t.Fatal(err)
	}

	store, err := storage.Open(dir)
	if !errors.Is(err, storage.ErrCorruptReset) {
		t.Fatalf("expected ErrCorruptReset, got %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// 破損後の DB は initialized=false（admin がない）
	rl := auth.NewRateLimiter(store)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	deps := Dependencies{
		Store:       store,
		Locator:     &fakeLoc{},
		RateLimiter: rl,
		LogFiles:    map[string]string{"frpc": "", "frps": ""},
		Ready:       func() bool { return true },
		Logger:      logger,
		Version:     "test-corrupt",
	}
	testSrv := httptest.NewServer(New(deps))
	t.Cleanup(testSrv.Close)

	resp, body := doJSON(t, testSrv, "GET", "/api/v1/system/ready", nil, nil, "")
	if resp.StatusCode != 200 {
		t.Fatalf("status %d body=%s", resp.StatusCode, body)
	}
	var got SystemReady
	_ = json.Unmarshal(body, &got)
	if got.Initialized {
		t.Error("expected initialized=false after corrupt reset")
	}

	// 破損ファイルが改名されて存在するか確認
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var brokenFound bool
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "data.db.broken-") {
			brokenFound = true
			break
		}
	}
	if !brokenFound {
		t.Error("expected data.db.broken-* file to exist after corrupt reset")
	}
}

// --- AC-14: 默认 localIP=127.0.0.1 ---

func TestAC14_DefaultLocalIP(t *testing.T) {
	// 仮説：localIP を指定しない場合、デフォルトで 127.0.0.1 になるはず。
	srv, _ := newTestServer(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	rp := 6143
	resp, body := doJSON(t, srv, "POST", "/api/v1/proxies",
		map[string]any{"name": "ac14-default-ip", "type": "tcp", "localPort": 22, "remotePort": rp},
		cookies, csrf)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /proxies status %d body=%s", resp.StatusCode, body)
	}
	var got ProxyResponse
	_ = json.Unmarshal(body, &got)
	if got.LocalIP != "127.0.0.1" {
		t.Errorf("expected localIP=127.0.0.1, got %q", got.LocalIP)
	}
}

// --- AC-11: 日志 tail 500 行 + 增量读取 ---

func TestAC11_Logs_EmptyPathReturns404(t *testing.T) {
	// LogFiles には空文字列が入っている → 404 が返るはず
	srv, _ := newTestServer(t, nil, nil) // LogFiles={"frpc":"","frps":""}
	cookies, _ := setupAndLogin(t, srv)

	resp, body := doJSON(t, srv, "GET", "/api/v1/logs/frpc", nil, cookies, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for empty log path, got %d body=%s", resp.StatusCode, body)
	}
}

func TestAC11_Logs_TailLines500(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "frpc.log")

	// 600 行のログ生成
	var sb strings.Builder
	for i := 0; i < 600; i++ {
		sb.WriteString(fmt.Sprintf("log line %04d: test message frpc running\n", i+1))
	}
	if err := os.WriteFile(logPath, []byte(sb.String()), 0644); err != nil {
		t.Fatal(err)
	}

	srv, _ := newTestServerFull(t, nil, map[string]string{"frpc": logPath, "frps": ""})
	cookies, _ := setupAndLogin(t, srv)

	// デフォルト（?lines=500）
	resp, body := doJSON(t, srv, "GET", "/api/v1/logs/frpc", nil, cookies, "")
	if resp.StatusCode != 200 {
		t.Fatalf("status %d body=%s", resp.StatusCode, body)
	}
	var tailResp LogsTailResponse
	_ = json.Unmarshal(body, &tailResp)
	if len(tailResp.Lines) != 500 {
		t.Errorf("expected 500 lines, got %d", len(tailResp.Lines))
	}

	// インクリメンタル offset=0 → NextOffset > 0
	resp, body = doJSON(t, srv, "GET", "/api/v1/logs/frpc?offset=0", nil, cookies, "")
	if resp.StatusCode != 200 {
		t.Fatalf("incremental status %d body=%s", resp.StatusCode, body)
	}
	var incResp LogsIncrementalResponse
	_ = json.Unmarshal(body, &incResp)
	if incResp.NextOffset <= 0 {
		t.Errorf("expected NextOffset > 0, got %d", incResp.NextOffset)
	}
}

// ======================================================================
// 对抗测试
// ======================================================================

// --- 对抗1: SQL 注入字符串被 proxy name 校验拦截 ---

func TestAdversarial_SQLInjectionInProxyName(t *testing.T) {
	// 仮説：ValidateProxyName が正規表現 [A-Za-z0-9_-] で SQL 特殊文字を拒否するはず。
	// 拒否されない場合、データ破壊の可能性がある。
	srv, _ := newTestServer(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	malicious := []string{
		"'; DROP TABLE proxies; --",
		"name' OR '1'='1",
		`\"; SELECT * FROM admin --`,
		"<script>alert(1)</script>",
		"../../etc/passwd",
		"proxy\x00name", // NUL byte
	}

	for _, name := range malicious {
		rp := 9000
		resp, body := doJSON(t, srv, "POST", "/api/v1/proxies",
			map[string]any{"name": name, "type": "tcp", "localPort": 22, "remotePort": rp},
			cookies, csrf)
		if resp.StatusCode == http.StatusCreated {
			t.Errorf("SQL injection name %q was accepted (should be rejected), body=%s", name, body)
		}
		// 422 または 400 が期待される
		if resp.StatusCode != http.StatusUnprocessableEntity && resp.StatusCode != http.StatusBadRequest {
			t.Logf("name %q → status=%d (not 201, acceptable rejection)", name, resp.StatusCode)
		}
	}
}

// --- 対抗2: 超長入力 proxy name (> 64 chars) → 422 + field=name ---

func TestAdversarial_OverlongProxyName422(t *testing.T) {
	// 假设：超过 65 字符的名称应被 422 拒绝，返回 field=name。
	srv, _ := newTestServer(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	longName := strings.Repeat("a", 65)
	rp := 9001
	resp, body := doJSON(t, srv, "POST", "/api/v1/proxies",
		map[string]any{"name": longName, "type": "tcp", "localPort": 22, "remotePort": rp},
		cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", resp.StatusCode, body)
	}
	var e ErrorBody
	_ = json.Unmarshal(body, &e)
	if e.Error.Field != "name" {
		t.Errorf("expected field=name, got %q; body=%s", e.Error.Field, body)
	}
}

// --- 对抗3: 超长用户名 (> 32 chars) → 422 ---

func TestAdversarial_OverlongUsername422(t *testing.T) {
	// 仮説：33 文字以上のユーザー名は setup で 422 を返すはず。
	srv, _ := newTestServer(t, nil, nil)

	longUser := strings.Repeat("z", 64)
	resp, body := doJSON(t, srv, "POST", "/api/v1/setup",
		map[string]string{"username": longUser, "password": "VerySafePass123"}, nil, "")
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for overlong username, got %d body=%s", resp.StatusCode, body)
	}
}

// --- 对抗4: 密码过短 → 422 ---

func TestAdversarial_TooShortPassword422(t *testing.T) {
	// 仮説：11 文字以下のパスワードは 422 を返すはず。
	srv, _ := newTestServer(t, nil, nil)

	resp, body := doJSON(t, srv, "POST", "/api/v1/setup",
		map[string]string{"username": "admin", "password": "short1234"}, nil, "")
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for short password, got %d body=%s", resp.StatusCode, body)
	}
}

// --- 対抗5: 並行 proxy 作成（同名）→ 1 成功, rest 409/422 ---

func TestAdversarial_ConcurrentProxyCreation_OnlyOneSucceeds(t *testing.T) {
	// 假设：并发创建同名代理时，UNIQUE 约束应只允许 1 条写入。
	srv, store := newTestServer(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	const goroutines = 10
	var (
		success int32
		wg      sync.WaitGroup
	)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rp := 8080 + idx
			resp, _ := doJSON(t, srv, "POST", "/api/v1/proxies",
				map[string]any{
					"name": "concurrent-proxy", "type": "tcp",
					"localPort": 22, "remotePort": rp,
				}, cookies, csrf)
			if resp.StatusCode == http.StatusCreated {
				atomic.AddInt32(&success, 1)
			}
		}(i)
	}
	wg.Wait()

	if success != 1 {
		t.Errorf("expected exactly 1 concurrent create to succeed, got %d", success)
	}

	// DB に 1 件だけ存在することを確認
	proxies, err := store.ListProxies(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 {
		t.Errorf("expected 1 proxy in DB, got %d", len(proxies))
	}
}

// --- 对抗6: 非法 JSON 请求体 → 400 ---

func TestAdversarial_InvalidJSONBody400(t *testing.T) {
	// 仮説：JSON でないボディは 400 Bad Request を返すはず。
	srv, _ := newTestServer(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/proxies",
		strings.NewReader("this-is-not-json{{{{"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", csrf)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", resp.StatusCode)
	}
}

// --- 对抗7: 端口越界 → 422 ---

func TestAdversarial_PortOutOfRange422(t *testing.T) {
	// 仮説：remotePort=0 や remotePort=99999 は 422 を返すはず。
	srv, _ := newTestServer(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	for _, badPort := range []int{0, 65536, -1, 99999} {
		resp, body := doJSON(t, srv, "POST", "/api/v1/proxies",
			map[string]any{
				"name": fmt.Sprintf("badport-%d", badPort),
				"type": "tcp", "localPort": 22, "remotePort": badPort,
			}, cookies, csrf)
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("port %d: expected 422, got %d body=%s", badPort, resp.StatusCode, body)
		}
	}
}

// --- 对抗8: http 代理指定 remotePort → 422 ---

func TestAdversarial_HttpProxyWithRemotePort422(t *testing.T) {
	// 仮説：http/https プロキシに remotePort を指定すると 422 を返すはず。
	srv, _ := newTestServer(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	rp := 8000
	resp, body := doJSON(t, srv, "POST", "/api/v1/proxies",
		map[string]any{
			"name": "bad-http", "type": "http",
			"localPort": 80, "remotePort": rp,
			"customDomains": []string{"example.com"},
		}, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", resp.StatusCode, body)
	}
}

// --- 对抗9: 未认证访问 → 401 ---

func TestAdversarial_UnauthenticatedProxyCreate401(t *testing.T) {
	// 仮説：Cookie なしで保護ルートにアクセスすると 401 が返されるはず。
	srv, _ := newTestServer(t, nil, nil)
	setupAndLogin(t, srv) // setup のみ

	// Cookie なしで protected route にアクセス
	rp := 6500
	resp, body := doJSON(t, srv, "POST", "/api/v1/proxies",
		map[string]any{"name": "unauth", "type": "tcp", "localPort": 22, "remotePort": rp},
		nil, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", resp.StatusCode, body)
	}
}

// --- 对抗10: ConfigPaths 未设置时 renderAndApply 不崩溃 ---

func TestAdversarial_ConfigPaths_Nil_NocrashOnProxy(t *testing.T) {
	// 仮説：ConfigPaths が nil でも proxy 作成は正常完了するはず（TOML 書き込みはスキップ）。
	srv, _ := newTestServer(t, nil, nil) // ConfigPaths=nil
	cookies, csrf := setupAndLogin(t, srv)

	rp := 7777
	resp, body := doJSON(t, srv, "POST", "/api/v1/proxies",
		map[string]any{"name": "nil-paths", "type": "tcp", "localPort": 22, "remotePort": rp},
		cookies, csrf)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", resp.StatusCode, body)
	}
}

// --- 回归: GET /proc/status 在 ProcMgr=nil 时不崩溃 (AC-13) ---

func TestRegression_ProcStatus_NilProcMgr(t *testing.T) {
	// AC-13 の一部：バイナリ欠損環境でも /proc/status は 200 を返すはず。
	srv, _ := newTestServer(t, nil, nil)
	cookies, _ := setupAndLogin(t, srv)

	resp, body := doJSON(t, srv, "GET", "/api/v1/proc/status", nil, cookies, "")
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, body)
	}

	var status map[string]any
	_ = json.Unmarshal(body, &status)
	if _, ok := status["frpc"]; !ok {
		t.Errorf("proc/status missing 'frpc' key: %s", body)
	}
	if _, ok := status["frps"]; !ok {
		t.Errorf("proc/status missing 'frps' key: %s", body)
	}
}

// ======================================================================
// T-002 新端点测试（Code Review M-4 补充）
// ======================================================================

// newTestServerWithDownloader 创建带 Downloader 的测试服务器。
// 注意：不使用 t.TempDir() 作为下载器根目录，避免 Windows 上下载 goroutine
// 持有临时文件句柄时 t.TempDir() 清理失败导致测试报错。
func newTestServerWithDownloader(t *testing.T) (*httptest.Server, *storage.Store) {
	t.Helper()
	store, err := storage.Open(t.TempDir())
	if err != nil && !errors.Is(err, storage.ErrCorruptReset) {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	rl := auth.NewRateLimiter(store)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// 使用 os.MkdirTemp 而非 t.TempDir()，并以 _ 丢弃清理错误，
	// 避免下载 goroutine 在 Windows 上持有 archive 临时文件导致测试失败。
	dlRoot, dirErr := os.MkdirTemp("", "frp-dl-test-")
	if dirErr != nil {
		t.Fatalf("os.MkdirTemp: %v", dirErr)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dlRoot) })

	dl := downloader.New(dlRoot, logger)

	deps := Dependencies{
		Store:       store,
		Locator:     &fakeLoc{},
		ProcMgr:     nil,
		RateLimiter: rl,
		LogFiles:    map[string]string{"frpc": "", "frps": ""},
		Ready:       func() bool { return true },
		Logger:      logger,
		Version:     "qa-test-dl-0.1.0",
		Downloader:  dl,
	}
	srv := httptest.NewServer(New(deps))
	t.Cleanup(srv.Close)
	return srv, store
}

// --- GET /api/v1/wizard/status: 全新 DB → shouldShow=true ---

func TestWizardStatus_FreshDB_ShouldShow(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	cookies, _ := setupAndLogin(t, srv)

	resp, body := doJSON(t, srv, "GET", "/api/v1/wizard/status", nil, cookies, "")
	if resp.StatusCode != 200 {
		t.Fatalf("status %d body=%s", resp.StatusCode, body)
	}
	var got WizardStatus
	_ = json.Unmarshal(body, &got)
	if !got.ShouldShow {
		t.Errorf("fresh DB: expected shouldShow=true, got %+v", got)
	}
}

// --- GET /api/v1/wizard/status: 已有配置 → shouldShow=false ---

func TestWizardStatus_WithConfig_ShouldNotShow(t *testing.T) {
	srv, store := newTestServer(t, nil, nil)
	cookies, _ := setupAndLogin(t, srv)

	// 先写入一个配置 key 表示"用户已配置过"。
	if err := store.KVSet(context.Background(), "mode.frpc.enabled", "true"); err != nil {
		t.Fatalf("KVSet: %v", err)
	}

	resp, body := doJSON(t, srv, "GET", "/api/v1/wizard/status", nil, cookies, "")
	if resp.StatusCode != 200 {
		t.Fatalf("status %d body=%s", resp.StatusCode, body)
	}
	var got WizardStatus
	_ = json.Unmarshal(body, &got)
	if got.ShouldShow {
		t.Errorf("with config: expected shouldShow=false, got %+v", got)
	}
}

// --- POST /api/v1/wizard/complete → 200；再查 shouldShow=false ---

func TestWizardComplete_ThenShouldNotShow(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	cookies, csrf := setupAndLogin(t, srv)

	resp, body := doJSON(t, srv, "POST", "/api/v1/wizard/complete", map[string]string{}, cookies, csrf)
	if resp.StatusCode != 200 {
		t.Fatalf("POST wizard/complete status %d body=%s", resp.StatusCode, body)
	}

	// 再查 wizard/status → shouldShow 应为 false（已 handled）。
	resp, body = doJSON(t, srv, "GET", "/api/v1/wizard/status", nil, cookies, "")
	if resp.StatusCode != 200 {
		t.Fatalf("GET wizard/status status %d body=%s", resp.StatusCode, body)
	}
	var got WizardStatus
	_ = json.Unmarshal(body, &got)
	if got.ShouldShow {
		t.Errorf("after complete: expected shouldShow=false, got %+v", got)
	}
}

// --- POST /api/v1/system/download-bin: kind=frpc → 202 ---

func TestDownloadBin_ValidKind_202(t *testing.T) {
	srv, _ := newTestServerWithDownloader(t)
	cookies, csrf := setupAndLogin(t, srv)

	resp, body := doJSON(t, srv, "POST", "/api/v1/system/download-bin",
		map[string]string{"kind": "frpc"}, cookies, csrf)
	// 第一次触发 → 202 Accepted（异步下载已启动）。
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", resp.StatusCode, body)
	}
}

// --- POST /api/v1/system/download-bin: kind=invalid → 422 ---

func TestDownloadBin_InvalidKind_422(t *testing.T) {
	srv, _ := newTestServerWithDownloader(t)
	cookies, csrf := setupAndLogin(t, srv)

	resp, body := doJSON(t, srv, "POST", "/api/v1/system/download-bin",
		map[string]string{"kind": "invalid"}, cookies, csrf)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d body=%s", resp.StatusCode, body)
	}
}

// --- GET /api/v1/system/download-status/frpc → 200 + JSON ---

func TestDownloadStatus_KnownKind_200(t *testing.T) {
	srv, _ := newTestServerWithDownloader(t)
	cookies, _ := setupAndLogin(t, srv)

	resp, body := doJSON(t, srv, "GET", "/api/v1/system/download-status/frpc", nil, cookies, "")
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, body)
	}
	var st DownloadStatusResponse
	if err := json.Unmarshal(body, &st); err != nil {
		t.Fatalf("unmarshal DownloadStatusResponse: %v body=%s", err, body)
	}
	// 初始状态应为 idle。
	if st.Status == "" {
		t.Errorf("expected non-empty status field, got %+v", st)
	}
}

// --- GET /api/v1/system/download-status/unknownkind → 404 ---

func TestDownloadStatus_UnknownKind_404(t *testing.T) {
	srv, _ := newTestServerWithDownloader(t)
	cookies, _ := setupAndLogin(t, srv)

	resp, body := doJSON(t, srv, "GET", "/api/v1/system/download-status/unknownkind", nil, cookies, "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", resp.StatusCode, body)
	}
}

// --- GET /api/v1/system/public-ip → always 200 with ip or error ---

func TestPublicIP_Always200(t *testing.T) {
	srv, _ := newTestServer(t, nil, nil)
	cookies, _ := setupAndLogin(t, srv)

	resp, body := doJSON(t, srv, "GET", "/api/v1/system/public-ip", nil, cookies, "")
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, body)
	}
	var got PublicIPResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal PublicIPResponse: %v body=%s", err, body)
	}
	// 无论是否能访问网络，必须有 ip 或 error 字段之一。
	if got.IP == "" && got.Error == "" {
		t.Errorf("expected ip or error field in response, got empty; body=%s", body)
	}
}
