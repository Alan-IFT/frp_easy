package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// helper —— 在 t.TempDir 创建一个 fresh Store；测试结束自动 Close。
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open fresh: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
	})
	return s
}

// ----- AC-12 fresh path -----

func TestOpen_Fresh(t *testing.T) {
	dir := t.TempDir()
	// dataDir 中显式删一遍以确保 Open 自己 mkdir 出来。
	innerDir := filepath.Join(dir, "fresh-sub")
	s, err := Open(innerDir)
	if err != nil {
		t.Fatalf("Open fresh: %v", err)
	}
	defer s.Close()

	if _, err := os.Stat(filepath.Join(innerDir, "data.db")); err != nil {
		t.Fatalf("data.db not created: %v", err)
	}

	// schema_migrations 已应用 version=1
	var version int64
	if err := s.db.QueryRow(`SELECT version FROM schema_migrations WHERE version = 1`).Scan(&version); err != nil {
		t.Fatalf("schema_migrations version=1 not present: %v", err)
	}
	if version != 1 {
		t.Fatalf("expected version=1, got %d", version)
	}

	// 5 张表都已建（admin / sessions / kv / proxies + schema_migrations）。
	wantTables := []string{"admin", "sessions", "kv", "proxies", "schema_migrations"}
	for _, tab := range wantTables {
		row := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tab)
		var name string
		if err := row.Scan(&name); err != nil {
			t.Fatalf("table %s not found: %v", tab, err)
		}
	}

	// 必要索引：idx_sessions_expires、idx_proxies_tcp_remote
	wantIdx := []string{"idx_sessions_expires", "idx_proxies_tcp_remote"}
	for _, idx := range wantIdx {
		row := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, idx)
		var name string
		if err := row.Scan(&name); err != nil {
			t.Fatalf("index %s not found: %v", idx, err)
		}
	}
}

// ----- AC-12 corrupt-reset path -----

func TestOpen_Corrupt(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")
	// 模拟损坏：写入垃圾数据。
	if err := os.WriteFile(dbPath, []byte("garbage-not-a-sqlite-file"), 0o644); err != nil {
		t.Fatalf("seed corrupt file: %v", err)
	}

	s, err := Open(dir)
	if !errors.Is(err, ErrCorruptReset) {
		t.Fatalf("expected ErrCorruptReset, got %v", err)
	}
	if s == nil {
		t.Fatalf("Open should return usable Store even on ErrCorruptReset")
	}
	defer s.Close()

	// 原 data.db 应被改名为 data.db.broken-<...>
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	var foundBroken, foundNewDB bool
	for _, e := range entries {
		name := e.Name()
		if name == "data.db" {
			foundNewDB = true
		}
		if strings.HasPrefix(name, "data.db.broken-") {
			foundBroken = true
		}
	}
	if !foundNewDB {
		t.Fatalf("new data.db not created")
	}
	if !foundBroken {
		t.Fatalf("corrupt file was not renamed to data.db.broken-<ts>; entries=%v", entries)
	}

	// 新库应可用：跑一条简单查询。
	ctx := context.Background()
	if _, found, err := s.KVGet(ctx, "no-such-key"); err != nil || found {
		t.Fatalf("KVGet on fresh corrupt-reset db: err=%v found=%v", err, found)
	}
}

// ----- admin -----

func TestAdmin_SetGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if got, err := s.GetAdmin(ctx); err != nil || got != nil {
		t.Fatalf("expected (nil, nil) on empty admin, got (%v, %v)", got, err)
	}

	if err := s.SetAdmin(ctx, "alice", "$argon2id$v=19$m=65536,t=3,p=2$AAAAA$BBBBB"); err != nil {
		t.Fatalf("SetAdmin: %v", err)
	}
	a, err := s.GetAdmin(ctx)
	if err != nil || a == nil {
		t.Fatalf("GetAdmin after Set: a=%v err=%v", a, err)
	}
	if a.Username != "alice" {
		t.Fatalf("username got %q want alice", a.Username)
	}
	if a.PasswordHash == "" {
		t.Fatalf("password hash empty")
	}
	if a.UpdatedAt.IsZero() {
		t.Fatalf("updated_at zero")
	}

	// 再次 Set 应 upsert 而非 INSERT 冲突（CHECK id=1）。
	if err := s.SetAdmin(ctx, "bob", "newhash"); err != nil {
		t.Fatalf("SetAdmin upsert: %v", err)
	}
	a2, err := s.GetAdmin(ctx)
	if err != nil || a2 == nil {
		t.Fatalf("GetAdmin second: %v %v", a2, err)
	}
	if a2.Username != "bob" || a2.PasswordHash != "newhash" {
		t.Fatalf("upsert failed: %+v", a2)
	}

	// 行数恒为 1
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM admin`).Scan(&count); err != nil {
		t.Fatalf("count admin: %v", err)
	}
	if count != 1 {
		t.Fatalf("admin row count = %d want 1", count)
	}

	// 空参数被拒
	if err := s.SetAdmin(ctx, "", "x"); err == nil {
		t.Fatalf("SetAdmin empty username should error")
	}
	if err := s.SetAdmin(ctx, "x", ""); err == nil {
		t.Fatalf("SetAdmin empty hash should error")
	}
}

// ----- sessions -----

func TestSession_Lifecycle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	sess, err := s.CreateSession(ctx, time.Hour)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.Token == "" || sess.CSRFToken == "" {
		t.Fatalf("empty token: %+v", sess)
	}
	if sess.ExpiresAt.Before(time.Now().Add(30 * time.Minute)) {
		t.Fatalf("expires_at too soon: %v", sess.ExpiresAt)
	}

	// Get round-trip
	got, err := s.GetSession(ctx, sess.Token)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Token != sess.Token || got.CSRFToken != sess.CSRFToken {
		t.Fatalf("round-trip mismatch: %+v vs %+v", got, sess)
	}

	// 不存在
	if _, err := s.GetSession(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unknown token, got %v", err)
	}

	// Delete
	if err := s.DeleteSession(ctx, sess.Token); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if _, err := s.GetSession(ctx, sess.Token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted session still findable: %v", err)
	}
	// 再 Delete 不应报错（idempotent）
	if err := s.DeleteSession(ctx, sess.Token); err != nil {
		t.Fatalf("idempotent delete: %v", err)
	}

	// PurgeExpired: 创建一条已过期的（直接写 DB），再 Purge。
	expSess, err := s.CreateSession(ctx, time.Hour)
	if err != nil {
		t.Fatalf("CreateSession #2: %v", err)
	}
	// 强行把它的 expires_at 改成过去。
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	if _, err := s.db.Exec(`UPDATE sessions SET expires_at = ? WHERE token = ?`, past, expSess.Token); err != nil {
		t.Fatalf("force expire: %v", err)
	}
	// GetSession 读到过期 → ErrNotFound
	if _, err := s.GetSession(ctx, expSess.Token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired session should ErrNotFound, got %v", err)
	}
	// 再创建一条未过期的会话，应当不被 purge
	live, err := s.CreateSession(ctx, time.Hour)
	if err != nil {
		t.Fatalf("CreateSession live: %v", err)
	}
	n, err := s.PurgeExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if n != 1 {
		t.Fatalf("Purge removed %d rows, want 1", n)
	}
	if _, err := s.GetSession(ctx, live.Token); err != nil {
		t.Fatalf("live session purged by mistake: %v", err)
	}

	// ttl<=0 拒绝
	if _, err := s.CreateSession(ctx, 0); err == nil {
		t.Fatalf("CreateSession ttl=0 should error")
	}
}

// ----- kv -----

func TestKV_SetGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// missing
	if _, found, err := s.KVGet(ctx, "missing"); err != nil || found {
		t.Fatalf("missing key: err=%v found=%v", err, found)
	}

	// set + get
	if err := s.KVSet(ctx, "mode.frpc.enabled", "true"); err != nil {
		t.Fatalf("KVSet: %v", err)
	}
	v, found, err := s.KVGet(ctx, "mode.frpc.enabled")
	if err != nil || !found {
		t.Fatalf("KVGet: err=%v found=%v", err, found)
	}
	if v != "true" {
		t.Fatalf("value got %q want true", v)
	}

	// upsert
	if err := s.KVSet(ctx, "mode.frpc.enabled", "false"); err != nil {
		t.Fatalf("KVSet upsert: %v", err)
	}
	v2, _, _ := s.KVGet(ctx, "mode.frpc.enabled")
	if v2 != "false" {
		t.Fatalf("upsert value got %q want false", v2)
	}

	// delete
	if err := s.KVDelete(ctx, "mode.frpc.enabled"); err != nil {
		t.Fatalf("KVDelete: %v", err)
	}
	if _, found, _ := s.KVGet(ctx, "mode.frpc.enabled"); found {
		t.Fatalf("KVDelete did not remove")
	}

	// 空 key 拒
	if err := s.KVSet(ctx, "", "x"); err == nil {
		t.Fatalf("KVSet empty key should error")
	}
}

// ----- proxies -----

func TestProxy_CRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	rp := 6000
	tcp := &Proxy{
		Name:       "ssh",
		Type:       "tcp",
		LocalIP:    "127.0.0.1",
		LocalPort:  22,
		RemotePort: &rp,
		Enabled:    true,
	}
	if err := s.UpsertProxy(ctx, tcp); err != nil {
		t.Fatalf("UpsertProxy tcp: %v", err)
	}
	if tcp.ID == 0 || tcp.Version != 1 {
		t.Fatalf("post-insert tcp = %+v", tcp)
	}

	http := &Proxy{
		Name:          "web",
		Type:          "http",
		LocalIP:       "127.0.0.1",
		LocalPort:     80,
		CustomDomains: []string{"www.example.com", "example.com"},
		Enabled:       true,
	}
	if err := s.UpsertProxy(ctx, http); err != nil {
		t.Fatalf("UpsertProxy http: %v", err)
	}
	if http.ID == 0 {
		t.Fatalf("http id zero")
	}

	// List should return both
	list, err := s.ListProxies(ctx)
	if err != nil {
		t.Fatalf("ListProxies: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list size = %d want 2", len(list))
	}
	// CustomDomains round-trip
	for _, p := range list {
		if p.Name == "web" {
			if len(p.CustomDomains) != 2 || p.CustomDomains[0] != "www.example.com" {
				t.Fatalf("customDomains round-trip = %v", p.CustomDomains)
			}
		}
		if p.Name == "ssh" {
			if p.RemotePort == nil || *p.RemotePort != 6000 {
				t.Fatalf("remotePort round-trip = %v", p.RemotePort)
			}
			if len(p.CustomDomains) != 0 {
				t.Fatalf("ssh customDomains should be empty: %v", p.CustomDomains)
			}
		}
	}

	// 唯一 name 冲突
	rp2 := 6001
	dup := &Proxy{
		Name: "ssh", Type: "tcp", LocalIP: "127.0.0.1",
		LocalPort: 22, RemotePort: &rp2, Enabled: true,
	}
	if err := s.UpsertProxy(ctx, dup); err == nil {
		t.Fatalf("duplicate name should fail UNIQUE")
	}

	// (type,remotePort) 冲突
	dup2 := &Proxy{
		Name: "ssh2", Type: "tcp", LocalIP: "127.0.0.1",
		LocalPort: 23, RemotePort: &rp, // 同 rp=6000
		Enabled: true,
	}
	if err := s.UpsertProxy(ctx, dup2); err == nil {
		t.Fatalf("duplicate (type,remotePort) should fail UNIQUE INDEX")
	}

	// 互斥规则：tcp 带 customDomains 应被 shape 校验拦下
	bad := &Proxy{
		Name: "bad-tcp", Type: "tcp", LocalIP: "127.0.0.1",
		LocalPort: 25, RemotePort: &rp2, CustomDomains: []string{"x.com"},
		Enabled: true,
	}
	if err := s.UpsertProxy(ctx, bad); err == nil {
		t.Fatalf("tcp + customDomains should be rejected")
	}
	// http 缺 customDomains
	badHTTP := &Proxy{
		Name: "bad-http", Type: "http", LocalIP: "127.0.0.1", LocalPort: 8080,
		Enabled: true,
	}
	if err := s.UpsertProxy(ctx, badHTTP); err == nil {
		t.Fatalf("http without customDomains should be rejected")
	}

	// 端口越界
	bp := 70000
	badPort := &Proxy{
		Name: "x", Type: "tcp", LocalIP: "127.0.0.1",
		LocalPort: 22, RemotePort: &bp, Enabled: true,
	}
	if err := s.UpsertProxy(ctx, badPort); err == nil {
		t.Fatalf("remotePort out of range should be rejected")
	}

	// GetProxy + DeleteProxy
	got, err := s.GetProxy(ctx, tcp.ID)
	if err != nil || got == nil {
		t.Fatalf("GetProxy: %v %v", got, err)
	}
	if err := s.DeleteProxy(ctx, tcp.ID); err != nil {
		t.Fatalf("DeleteProxy: %v", err)
	}
	if _, err := s.GetProxy(ctx, tcp.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("post-delete get: %v", err)
	}
	if err := s.DeleteProxy(ctx, 99999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete non-existent: %v", err)
	}

	// Update（拿剩余 http），version 校验
	live, err := s.GetProxy(ctx, http.ID)
	if err != nil {
		t.Fatalf("Get live: %v", err)
	}
	live.CustomDomains = []string{"only-one.com"}
	if err := s.UpsertProxy(ctx, live); err != nil {
		t.Fatalf("update live: %v", err)
	}
	if live.Version != 2 {
		t.Fatalf("post-update version = %d want 2", live.Version)
	}
	reloaded, _ := s.GetProxy(ctx, http.ID)
	if len(reloaded.CustomDomains) != 1 || reloaded.CustomDomains[0] != "only-one.com" {
		t.Fatalf("update did not persist: %v", reloaded.CustomDomains)
	}
}

func TestProxy_VersionConflict(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	rp := 7000
	p := &Proxy{
		Name: "vc", Type: "tcp", LocalIP: "127.0.0.1",
		LocalPort: 22, RemotePort: &rp, Enabled: true,
	}
	if err := s.UpsertProxy(ctx, p); err != nil {
		t.Fatalf("seed: %v", err)
	}
	id := p.ID

	// 两个调用者都读到 version=1
	a, _ := s.GetProxy(ctx, id)
	b, _ := s.GetProxy(ctx, id)
	if a.Version != 1 || b.Version != 1 {
		t.Fatalf("seed version: a=%d b=%d", a.Version, b.Version)
	}

	// a 先写：成功，DB version → 2
	a.LocalPort = 23
	if err := s.UpsertProxy(ctx, a); err != nil {
		t.Fatalf("a upsert: %v", err)
	}
	if a.Version != 2 {
		t.Fatalf("a post-version = %d want 2", a.Version)
	}

	// b 再写（仍带旧 version=1）：应 ErrVersionConflict
	b.LocalPort = 24
	if err := s.UpsertProxy(ctx, b); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}

	// 不存在的 id（用 1e9）—— 走 update 路径应 ErrNotFound
	rp2 := 7001
	missing := &Proxy{
		ID: 1_000_000_000, Name: "missing", Type: "tcp",
		LocalIP: "127.0.0.1", LocalPort: 22, RemotePort: &rp2,
		Enabled: true, Version: 1,
	}
	if err := s.UpsertProxy(ctx, missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update of missing id: %v", err)
	}

	// 真并发：N 个 goroutine 同时拿 version=2 改写，仅 1 个成功
	current, _ := s.GetProxy(ctx, id)
	if current.Version != 2 {
		t.Fatalf("setup for race: version=%d", current.Version)
	}
	const N = 8
	var wg sync.WaitGroup
	var successCount, conflictCount int
	var mu sync.Mutex
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			racer := *current
			racer.LocalPort = 1000 + i
			err := s.UpsertProxy(ctx, &racer)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				successCount++
			case errors.Is(err, ErrVersionConflict):
				conflictCount++
			default:
				t.Errorf("race goroutine %d got unexpected err: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	if successCount != 1 || conflictCount != N-1 {
		t.Fatalf("race: success=%d conflict=%d (want 1 / %d)", successCount, conflictCount, N-1)
	}
}

// ----- 额外小用例：DataDir / Close 幂等 / Proxy 互斥额外分支 -----

func TestStore_DataDirAndCloseIdempotent(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got := s.DataDir()
	want, _ := filepath.Abs(dir)
	if got != want {
		t.Fatalf("DataDir = %q want %q", got, want)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close 1: %v", err)
	}
	// 二次 Close 对 *sql.DB 而言会返回 sql.ErrConnDone 之外的错误也可接受；
	// 但我们的 Close 仅在 s == nil 时返回 nil。这里仅断言不 panic。
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("second Close panicked: %v", r)
		}
	}()
	_ = s.Close()

	// nil receiver 安全
	var nilStore *Store
	if err := nilStore.Close(); err != nil {
		t.Fatalf("nil Close: %v", err)
	}
}

func TestProxy_InvalidType(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	p := &Proxy{Name: "x", Type: "xtcp", LocalPort: 22, Enabled: true}
	if err := s.UpsertProxy(ctx, p); err == nil {
		t.Fatalf("invalid type should be rejected")
	}
	// 空 name
	rp := 10
	p2 := &Proxy{Name: "", Type: "tcp", LocalPort: 22, RemotePort: &rp, Enabled: true}
	if err := s.UpsertProxy(ctx, p2); err == nil {
		t.Fatalf("empty name should be rejected")
	}
	// nil proxy
	if err := s.UpsertProxy(ctx, nil); err == nil {
		t.Fatalf("nil proxy should be rejected")
	}
	// localPort 越界
	bad := &Proxy{Name: "y", Type: "tcp", LocalPort: 0, RemotePort: &rp, Enabled: true}
	if err := s.UpsertProxy(ctx, bad); err == nil {
		t.Fatalf("localPort 0 should be rejected")
	}
}

func TestKVDelete_Missing(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.KVDelete(ctx, "nope"); err != nil {
		t.Fatalf("KVDelete missing: %v", err)
	}
}

// 二次 Open（已应用迁移）必须 idempotent，不重复 INSERT schema_migrations。
func TestOpen_MigrationsIdempotent(t *testing.T) {
	dir := t.TempDir()
	s1, err := Open(dir)
	if err != nil {
		t.Fatalf("Open #1: %v", err)
	}
	_ = s1.Close()
	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("Open #2: %v", err)
	}
	defer s2.Close()
	var count int
	if err := s2.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("schema_migrations count = %d after re-Open, want 1", count)
	}
}

// down.sql 必须可执行，且执行后再 Open 能重跑 up.sql 把表建回来。
func TestMigration_DownRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// 读 down.sql 并执行（直接走 db；不在生产路径，只验证 SQL 可回滚）。
	wd, _ := os.Getwd()
	repoRoot := filepath.Dir(filepath.Dir(wd))
	downBytes, err := os.ReadFile(filepath.Join(repoRoot, "migrations", "0001_init.down.sql"))
	if err != nil {
		t.Fatalf("read down.sql: %v", err)
	}
	if _, err := s.db.Exec(string(downBytes)); err != nil {
		t.Fatalf("exec down.sql: %v", err)
	}
	// proxies 表应已不存在
	var name string
	err = s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='proxies'`).Scan(&name)
	if err == nil {
		t.Fatalf("proxies table still exists after down.sql")
	}
	// schema_migrations 该 version=1 已删
	var cnt int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version=1`).Scan(&cnt); err != nil {
		t.Fatalf("schema_migrations select: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("schema_migrations version=1 still present after down: cnt=%d", cnt)
	}
	_ = s.Close()

	// 再次 Open 应能重跑 up.sql（应用到全新空状态 + version=1 重新写入）
	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("Open after down: %v", err)
	}
	defer s2.Close()
	if err := s2.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='proxies'`).Scan(&name); err != nil {
		t.Fatalf("proxies not rebuilt after re-Open: %v", err)
	}
}

// PurgeExpired 在空表上应返回 0 且无错。
func TestPurgeExpired_Empty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	n, err := s.PurgeExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 purged, got %d", n)
	}
}

// ----- 防止权威 SQL 与嵌入副本 drift -----

func TestEmbeddedMigrations_MatchDisk(t *testing.T) {
	// 权威源在仓库根的 migrations/；包内副本在 internal/storage/sqlmigrations/。
	// 两者必须字节一致。
	files := []string{"0001_init.up.sql", "0001_init.down.sql"}
	// 找仓库根：本测试文件在 internal/storage/，往上两级。
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Dir(filepath.Dir(wd)) // .../frp_easy
	for _, f := range files {
		diskPath := filepath.Join(repoRoot, "migrations", f)
		diskBytes, err := os.ReadFile(diskPath)
		if err != nil {
			t.Fatalf("read disk %s: %v", diskPath, err)
		}
		embedPath := filepath.Join("sqlmigrations", f)
		embedBytes, err := os.ReadFile(embedPath)
		if err != nil {
			t.Fatalf("read embed %s: %v", embedPath, err)
		}
		if string(diskBytes) != string(embedBytes) {
			t.Fatalf("migration %s drift: disk vs internal/storage/sqlmigrations/ differ", f)
		}
	}
}
