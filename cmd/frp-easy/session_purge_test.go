package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

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

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		purgeSessionsLoop(ctx, store, discardLogger())
		close(done)
	}()

	cancel()
	select {
	case <-done:
		// ok：loop 在 ctx 取消后返回。
	case <-time.After(2 * time.Second):
		t.Fatal("purgeSessionsLoop did not exit within 2s of ctx cancel (goroutine leak)")
	}
}
