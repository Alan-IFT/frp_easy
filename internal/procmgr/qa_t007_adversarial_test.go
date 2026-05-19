package procmgr

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func mkAdvManager() *Manager {
	loc := &fakeLocator{frpcErr: errors.New("missing"), frpsErr: errors.New("missing")}
	return New(Config{
		Locator:     loc,
		ConfigPaths: map[string]string{"frpc": "/tmp/x.toml", "frps": "/tmp/y.toml"},
	})
}

// AC-5 对抗：连续并发调用 Start，验证 defer-unlock 重构后无死锁、无 panic、idempotent 维持。
// 如果原有 Unlock 漏一处，并发调用会立即死锁。
func TestAdversarial_AC5_ConcurrentStartNoDeadlock(t *testing.T) {
	m := mkAdvManager()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = m.Start("frpc")
		}()
	}
	finishedCh := make(chan struct{})
	go func() { wg.Wait(); close(finishedCh) }()
	select {
	case <-finishedCh:
		// good
	case <-time.After(5 * time.Second):
		t.Fatalf("ADVERSARIAL FAIL: 20 并发 Start 在 5s 内未完成 → 疑似死锁")
	}
}

// AC-5 对抗：Start + Stop 交替并发 — 验证锁内 state 转换无 race
func TestAdversarial_AC5_StartStopRaceNoDeadlock(t *testing.T) {
	m := mkAdvManager()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = m.Start("frps") }()
		go func() { defer wg.Done(); _, _ = m.Stop("frps") }()
	}
	finishedCh := make(chan struct{})
	go func() { wg.Wait(); close(finishedCh) }()
	select {
	case <-finishedCh:
	case <-time.After(5 * time.Second):
		t.Fatalf("ADVERSARIAL FAIL: 并发 Start/Stop 死锁")
	}
}

// AC-5 对抗：早返回路径 — invalid kind 不进锁，应立即返回
func TestAdversarial_AC5_InvalidKindNoLock(t *testing.T) {
	m := mkAdvManager()
	done := make(chan struct{})
	go func() {
		_, err := m.Start("garbage")
		if err == nil {
			t.Error("ADVERSARIAL: invalid kind should error")
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatalf("ADVERSARIAL: invalid kind 返回耗时 > 1s")
	}
}

// AC-5 对抗：连续 Start 同一 kind 多次（idempotent 路径）— 验证不会因 idempotent 路径
// 早返回时锁状态不一致导致后续调用死锁
func TestAdversarial_AC5_RepeatedStartIdempotent(t *testing.T) {
	m := mkAdvManager()
	for i := 0; i < 50; i++ {
		_, _ = m.Start("frpc")
	}
	// 如果有锁泄露，到这里早就死锁了
}
