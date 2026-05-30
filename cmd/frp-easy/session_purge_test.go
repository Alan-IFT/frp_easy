package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/frp-easy/frp-easy/internal/auth"
	"github.com/frp-easy/frp-easy/internal/storage"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestPurgeExpiredSessionsOnce 验证一次清理：过期 session 被处理、未过期 session 存活
// （不 over-delete）、且不报错 / 不 panic。
func TestPurgeExpiredSessionsOnce(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	// 1ns ttl → 调 purge 时（微秒级之后）必已过期。
	expired, err := store.CreateSession(ctx, time.Nanosecond)
	if err != nil {
		t.Fatalf("create expired: %v", err)
	}
	live, err := store.CreateSession(ctx, time.Hour)
	if err != nil {
		t.Fatalf("create live: %v", err)
	}

	purgeExpiredSessionsOnce(ctx, store, discardLogger())

	// 未过期 session 必须存活（证明没有 over-delete）。
	if _, err := store.GetSession(ctx, live.Token); err != nil {
		t.Errorf("live session should survive purge, got %v", err)
	}
	// 过期 session 不可达。
	if _, err := store.GetSession(ctx, expired.Token); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expired session should be gone (ErrNotFound), got %v", err)
	}
}

// TestPurgeSessionsLoop_ExitsOnCancel 验证 loop 在 ctx 取消后及时退出（无 goroutine 泄漏）。
func TestPurgeSessionsLoop_ExitsOnCancel(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()

	// 设长间隔，确保 loop 进入 select 后只能靠 ctx.Done() 退出（而非 ticker）。
	orig := sessionPurgeInterval
	sessionPurgeInterval = time.Hour
	defer func() { sessionPurgeInterval = orig }()

	rl := auth.NewRateLimiter(store)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		purgeLoop(ctx, store, rl, discardLogger())
		close(done)
	}()

	cancel()
	select {
	case <-done:
		// ok：loop 在 ctx 取消后返回。
	case <-time.After(2 * time.Second):
		t.Fatal("purgeLoop did not exit within 2s of ctx cancel (goroutine leak)")
	}
}

// TestPurgeExpiredLoginFailsOnce 验证 wiring 层一次 loginfail 清理（T-063 AC-4）：
// 过期 loginfail.<ip> 行被删、窗口内活行存活（不 over-delete）、非 loginfail 键不被碰，
// 且不报错 / 不 panic。对称复刻 TestPurgeExpiredSessionsOnce。
func TestPurgeExpiredLoginFailsOnce(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	rl := auth.NewRateLimiter(store)
	// 用真实时钟（rl.now 是 auth 包私有字段，跨包测试不可注入）：
	// 过期行 firstAt 取过去 2 小时（远超 60s 窗口，必过期）；
	// 活行 firstAt 取此刻（窗口内，剩余 ~60s，purge 时必未过期）。
	now := time.Now().UTC()
	writeLoginFail(t, store, "9.9.9.9", 5, now.Add(-2*time.Hour))
	writeLoginFail(t, store, "1.1.1.1", 5, now)
	// 非 loginfail 键（不可被碰）。
	if err := store.KVSet(ctx, "mode.frpc.enabled", "true"); err != nil {
		t.Fatalf("seed mode: %v", err)
	}

	purgeExpiredLoginFailsOnce(ctx, rl, discardLogger())

	// 过期行不可达。
	if _, found, _ := store.KVGet(ctx, "loginfail.9.9.9.9"); found {
		t.Error("expired loginfail row should be purged")
	}
	// 活行存活（证明没 over-delete；限流不失效）。
	if _, found, _ := store.KVGet(ctx, "loginfail.1.1.1.1"); !found {
		t.Error("live loginfail row should survive purge")
	}
	// 非 loginfail 键安然无恙。
	if v, found, _ := store.KVGet(ctx, "mode.frpc.enabled"); !found || v != "true" {
		t.Errorf("non-loginfail key must be untouched, found=%v v=%q", found, v)
	}
}

// TestPurgeLoop_RunsBothPurgesOnStart 验证泛化后的 purgeLoop 在启动即触发 session +
// loginfail 两个清理（同一 goroutine），并随 ctx 取消退出（T-063 AC-4 / IS-2 / IS-6）。
// 用 poll-until-condition + deadline（insight L10，禁固定 sleep）等启动清理生效。
func TestPurgeLoop_RunsBothPurgesOnStart(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	// 过期 session（1ns ttl）+ 过期 loginfail 行。
	expiredSess, err := store.CreateSession(ctx, time.Nanosecond)
	if err != nil {
		t.Fatalf("create expired session: %v", err)
	}
	rl := auth.NewRateLimiter(store)
	// 用真实时钟（默认 now），firstAt 取过去 2 小时确保已过期。
	writeLoginFail(t, store, "9.9.9.9", 5, time.Now().UTC().Add(-2*time.Hour))

	// 长间隔确保只靠"启动即清"那一次（非 ticker）。
	orig := sessionPurgeInterval
	sessionPurgeInterval = time.Hour
	defer func() { sessionPurgeInterval = orig }()

	loopCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		purgeLoop(loopCtx, store, rl, discardLogger())
		close(done)
	}()

	// poll：两清理都生效（过期 session 不可达 + 过期 loginfail 行被删）。
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, sessErr := store.GetSession(ctx, expiredSess.Token)
		_, loginfailFound, _ := store.KVGet(ctx, "loginfail.9.9.9.9")
		if errors.Is(sessErr, storage.ErrNotFound) && !loginfailFound {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("purgeLoop start purge did not clear both within 2s (sessErr=%v loginfailFound=%v)", sessErr, loginfailFound)
		}
		time.Sleep(10 * time.Millisecond) // poll 间隔（非同步点固定 sleep）
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("purgeLoop did not exit within 2s of ctx cancel (goroutine leak)")
	}
}

// writeLoginFail 直接写一条 loginfail.<ip> 计数行（绕过 RecordFailure 以精确控制 firstAt）。
// 值格式必须与 auth.failRecord 的 JSON 一致（count + firstAt）。
func writeLoginFail(t *testing.T, store *storage.Store, ip string, count int, firstAt time.Time) {
	t.Helper()
	b, err := json.Marshal(struct {
		Count   int       `json:"count"`
		FirstAt time.Time `json:"firstAt"`
	}{Count: count, FirstAt: firstAt})
	if err != nil {
		t.Fatalf("marshal loginfail: %v", err)
	}
	if err := store.KVSet(context.Background(), auth.LoginFailKeyPrefix+ip, string(b)); err != nil {
		t.Fatalf("seed loginfail KVSet: %v", err)
	}
}
