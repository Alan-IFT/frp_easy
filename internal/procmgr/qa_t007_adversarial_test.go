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

// T-050 C-3：合法终态集合。
//
// 在 fakeLocator 让 Start 因 binary missing 早返回 error 的对抗场景下，并发收敛后
// 单个 kind 的 State 只可能停在以下三态之一：
//   - StateStopped：从未成功推进过（初始态 / Stop 后）。
//   - StateError：Start 因 binary missing 把 state 置 error。
//   - StateStopping：Start/Stop 交替时被 Stop 推进到 stopping（mkAdvManager 下无真
//     子进程能 cmd!=nil，Stop 实际多走 idempotent 早返回，但保留此态防御并发窗口）。
//
// 反向证伪点：若并发后出现 StateStarting / StateRunning（不可能，因为从无真 spawn），
// 或 StatusAll 长度 != 2（New 固定建 frpc+frps 两条），断言会失败。
var advTerminalStates = map[State]bool{
	StateStopped:  true,
	StateError:    true,
	StateStopping: true,
}

// assertAdvConverged 在并发收敛后断言 kind 落在合法终态集合，且 StatusAll 仍恒为 2 条。
func assertAdvConverged(t *testing.T, m *Manager, kind string) {
	t.Helper()
	st := m.Status(kind).State
	if !advTerminalStates[st] {
		t.Errorf("ADVERSARIAL FAIL: %s 并发收敛后 State=%q 不在合法终态集合 {stopped,error,stopping}", kind, st)
	}
	if all := m.StatusAll(); len(all) != 2 {
		t.Errorf("ADVERSARIAL FAIL: StatusAll len=%d，应恒为 2（frpc+frps）", len(all))
	}
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
	// T-050 C-3：并发收敛后断言最终态合法（binary missing → error），StatusAll 恒 2。
	assertAdvConverged(t, m, "frpc")
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
	// T-050 C-3：Start/Stop 交替收敛后 frps 最终态仍须合法，StatusAll 恒 2。
	assertAdvConverged(t, m, "frps")
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
	// T-050 C-3：50 次重复 Start 后最终态仍合法（binary missing → error），StatusAll 恒 2。
	assertAdvConverged(t, m, "frpc")
}
