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
	"github.com/frp-easy/frp-easy/internal/storage"
)

// --- 共通ヘルパー ---

// newTestServerFull はテスト用サーバーを起動する。
// configPaths が nil 以外の場合は TOML 書き込みパスが設定される。
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

// setupAndLogin は管理者をセットアップしてセッション cookie + CSRF トークンを返す。
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

// --- AC-5: TCP ルール → frpc.toml 書き込み ---

func TestAC5_RenderFrpc_TomlWritten(t *testing.T) {
	// 仮説：serverAddr が設定されていれば CREATE PROXY 後に frpc.toml が生成されるはず。
	// これが失敗するなら renderAndApplyFrpc が機能していないことになる。
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

// --- AC-5 frps.toml: PUT /server → frps.toml 書き込み ---

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

// --- AC-6: 削除後 frpc.toml からプロキシが消える ---

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

// --- AC-9: persistMode が KV に保存される ---

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

// --- AC-12: 損坏 DB → initialized=false, broken ファイル存在 ---

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

// --- AC-14: デフォルト localIP=127.0.0.1 ---

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

// --- AC-11: ログ tail 500行 + インクリメンタル ---

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
// 対抗テスト（Adversarial tests）
// ======================================================================

// --- 対抗1: SQL インジェクション文字列が proxy name 検証でブロックされる ---

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
	// 仮説：65 文字以上の名前が 422 で拒否され、field=name が返されるはず。
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

// --- 対抗3: 超長ユーザー名 (> 32 chars) → 422 ---

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

// --- 対抗4: 短すぎるパスワード → 422 ---

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
	// 仮説：同名プロキシを並行作成しても UNIQUE 制約で 1 件しか作成されないはず。
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

// --- 対抗6: 不正 JSON ボディ → 400 ---

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

// --- 対抗7: ポート範囲外 → 422 ---

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

// --- 対抗8: http proxy に remotePort を指定 → 422 ---

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

// --- 対抗9: 未認証アクセス → 401 ---

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

// --- 対抗10: ConfigPaths 未設定でも renderAndApply はクラッシュしない ---

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

// --- 回帰: GET /proc/status が ProcMgr=nil でもクラッシュしない (AC-13) ---

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
