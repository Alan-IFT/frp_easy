package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// TestUpsertProxiesTx_HappyPath：5 条全成功，返回 5 个 ID。
func TestUpsertProxiesTx_HappyPath(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	ps := make([]*Proxy, 0, 5)
	for i := 0; i < 5; i++ {
		rp := 6000 + i
		ps = append(ps, &Proxy{
			Name:       fmt.Sprintf("web-%d", rp),
			Type:       "tcp",
			LocalIP:    "127.0.0.1",
			LocalPort:  rp,
			RemotePort: &rp,
			Enabled:    true,
		})
	}

	ids, err := s.UpsertProxiesTx(ctx, ps)
	if err != nil {
		t.Fatalf("UpsertProxiesTx: %v", err)
	}
	if len(ids) != 5 {
		t.Fatalf("ids len = %d, want 5", len(ids))
	}
	for i, id := range ids {
		if id <= 0 {
			t.Errorf("ids[%d] = %d, want > 0", i, id)
		}
		if ps[i].ID != id {
			t.Errorf("ps[%d].ID = %d not back-filled", i, ps[i].ID)
		}
		if ps[i].Version != 1 {
			t.Errorf("ps[%d].Version = %d, want 1", i, ps[i].Version)
		}
		if ps[i].UpdatedAt.IsZero() {
			t.Errorf("ps[%d].UpdatedAt not set", i)
		}
	}

	// DB 应有 5 行
	all, _ := s.ListProxies(ctx)
	if len(all) != 5 {
		t.Errorf("DB rows = %d, want 5", len(all))
	}
}

// TestUpsertProxiesTx_RollbackOnNameDup：第 3 条与 DB 已有 name 冲突 → ErrDuplicateName，
// DB 行数不变。
func TestUpsertProxiesTx_RollbackOnNameDup(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// 预置一条 name=web-6002 的规则
	rp := 5000
	pre := &Proxy{
		Name: "web-6002", Type: "tcp", LocalIP: "127.0.0.1",
		LocalPort: rp, RemotePort: &rp, Enabled: true,
	}
	if err := s.UpsertProxy(ctx, pre); err != nil {
		t.Fatalf("pre insert: %v", err)
	}

	// 批量 5 条，第 3 条命名冲突
	ps := make([]*Proxy, 0, 5)
	for i := 0; i < 5; i++ {
		port := 6000 + i
		ps = append(ps, &Proxy{
			Name: fmt.Sprintf("web-%d", port),
			Type: "tcp", LocalIP: "127.0.0.1",
			LocalPort: port, RemotePort: &port, Enabled: true,
		})
	}

	_, err := s.UpsertProxiesTx(ctx, ps)
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("err = %v, want ErrDuplicateName", err)
	}

	all, _ := s.ListProxies(ctx)
	if len(all) != 1 {
		t.Errorf("DB rows = %d, want 1 (rollback should keep only pre-existing)", len(all))
	}
}

// TestUpsertProxiesTx_RollbackOnTcpRemoteDup：第 3 条与 DB 已有 (tcp, 6003) 冲突 →
// ErrDuplicateTcpRemote，DB 行数不变。
func TestUpsertProxiesTx_RollbackOnTcpRemoteDup(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// 预置一条 (tcp, remote_port=6003)
	rp := 6003
	pre := &Proxy{
		Name: "blocker", Type: "tcp", LocalIP: "127.0.0.1",
		LocalPort: 9999, RemotePort: &rp, Enabled: true,
	}
	if err := s.UpsertProxy(ctx, pre); err != nil {
		t.Fatalf("pre insert: %v", err)
	}

	// 批量 5 条，第 4 条触发冲突（remotePort=6003）
	ps := make([]*Proxy, 0, 5)
	for i := 0; i < 5; i++ {
		port := 6000 + i // 6000,6001,6002,6003,6004
		ps = append(ps, &Proxy{
			Name: fmt.Sprintf("new-%d", port),
			Type: "tcp", LocalIP: "127.0.0.1",
			LocalPort: port, RemotePort: &port, Enabled: true,
		})
	}

	_, err := s.UpsertProxiesTx(ctx, ps)
	if !errors.Is(err, ErrDuplicateTcpRemote) {
		t.Fatalf("err = %v, want ErrDuplicateTcpRemote", err)
	}

	all, _ := s.ListProxies(ctx)
	if len(all) != 1 {
		t.Errorf("DB rows = %d, want 1 (rollback)", len(all))
	}
}

// TestUpsertProxiesTx_RollbackOnInternalDup：批量本身就含两条同 name → 第二条 INSERT 失败
// → ErrDuplicateName，整批回滚。
func TestUpsertProxiesTx_RollbackOnInternalDup(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	rp1 := 6000
	rp2 := 6001
	ps := []*Proxy{
		{Name: "same", Type: "tcp", LocalIP: "127.0.0.1", LocalPort: rp1, RemotePort: &rp1, Enabled: true},
		{Name: "same", Type: "tcp", LocalIP: "127.0.0.1", LocalPort: rp2, RemotePort: &rp2, Enabled: true},
	}
	_, err := s.UpsertProxiesTx(ctx, ps)
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("err = %v, want ErrDuplicateName", err)
	}
	all, _ := s.ListProxies(ctx)
	if len(all) != 0 {
		t.Errorf("DB rows = %d, want 0 (rollback)", len(all))
	}
}

// TestIsDuplicateTcpRemoteError_FromRealDriver（T-018 B-8 实证）：直接通过 store 的底层
// db 句柄发一条 raw INSERT 触发 (type, remote_port) 复合 UNIQUE，捕获 modernc.org/sqlite
// 真实 err.Error()，断言 isDuplicateTcpRemoteError 关键字组合命中。
// **未来驱动升级若改文本，此用例立即捕获回归**，避免静默退化为 internal 500。
func TestIsDuplicateTcpRemoteError_FromRealDriver(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// 第一条 (tcp, 6000) 由 UpsertProxy 写入
	rp := 6000
	if err := s.UpsertProxy(ctx, &Proxy{
		Name: "first", Type: "tcp", LocalIP: "127.0.0.1",
		LocalPort: 22, RemotePort: &rp, Enabled: true,
	}); err != nil {
		t.Fatalf("pre: %v", err)
	}

	// 直接对底层 db 发一条 raw INSERT 触发 (type, remote_port) 冲突 —— 拿到 modernc
	// 原始 err 不经任何 sentinel 包装。
	s.mu.Lock()
	_, rawErr := s.db.ExecContext(ctx, `
		INSERT INTO proxies (name, type, local_ip, local_port, remote_port,
		                     custom_domains, enabled, version, updated_at)
		VALUES (?, 'tcp', '127.0.0.1', ?, ?, NULL, 1, 1, datetime('now'))`,
		"second", 23, 6000)
	s.mu.Unlock()

	if rawErr == nil {
		t.Fatal("expected raw conflict error")
	}
	t.Logf("modernc.org/sqlite composite UNIQUE raw err.Error(): %v", rawErr)

	if !strings.Contains(rawErr.Error(), "UNIQUE constraint failed") {
		t.Errorf("raw err missing 'UNIQUE constraint failed': %v", rawErr)
	}
	if !isDuplicateTcpRemoteError(rawErr) {
		t.Errorf("isDuplicateTcpRemoteError(%v) = false, want true", rawErr)
	}
	// 反向：单列 name 冲突不应被 tcp_remote 误判
	if isDuplicateTcpRemoteError(errors.New("UNIQUE constraint failed: proxies.name")) {
		t.Error("isDuplicateTcpRemoteError must not match proxies.name conflict")
	}
}

// TestUpsertProxiesTx_ConcurrentWithUpsertProxy（T-018 B-5）：goroutine A 跑 UpsertProxy、
// goroutine B 跑 UpsertProxiesTx，端口 / name 互不冲突；断言均 nil error 且
// 不出现 `database is locked`。固化 s.mu 串行化契约 —— 未来若移除锁会立即捕获。
func TestUpsertProxiesTx_ConcurrentWithUpsertProxy(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	const numA = 5 // 5 个并发 UpsertProxy
	const numB = 5 // 5 个并发 UpsertProxiesTx（每个 3 条）

	var wg sync.WaitGroup
	errs := make(chan error, numA+numB)

	// A: 5 goroutines 各起一条 UpsertProxy（name = `a-N`, remotePort = 7000+N）
	for i := 0; i < numA; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			rp := 7000 + n
			err := s.UpsertProxy(ctx, &Proxy{
				Name: fmt.Sprintf("a-%d", n),
				Type: "tcp", LocalIP: "127.0.0.1",
				LocalPort: rp, RemotePort: &rp, Enabled: true,
			})
			if err != nil {
				errs <- fmt.Errorf("A[%d]: %w", n, err)
			}
		}(i)
	}

	// B: 5 goroutines 各起一个 UpsertProxiesTx 写 3 条（name = `b-M-N`, remotePort=8000 + 30*M + N）
	for i := 0; i < numB; i++ {
		wg.Add(1)
		go func(m int) {
			defer wg.Done()
			ps := make([]*Proxy, 0, 3)
			for j := 0; j < 3; j++ {
				rp := 8000 + 30*m + j
				ps = append(ps, &Proxy{
					Name: fmt.Sprintf("b-%d-%d", m, j),
					Type: "tcp", LocalIP: "127.0.0.1",
					LocalPort: rp, RemotePort: &rp, Enabled: true,
				})
			}
			if _, err := s.UpsertProxiesTx(ctx, ps); err != nil {
				errs <- fmt.Errorf("B[%d]: %w", m, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		// 关键断言：不允许出现 database is locked
		msg := err.Error()
		if strings.Contains(msg, "database is locked") {
			t.Fatalf("CONCURRENCY FAIL (database is locked): %v", err)
		}
		t.Errorf("unexpected concurrent error: %v", err)
	}

	// 验证最终行数 = numA + numB*3
	all, _ := s.ListProxies(ctx)
	wantCount := numA + numB*3
	if len(all) != wantCount {
		t.Errorf("final row count = %d, want %d", len(all), wantCount)
	}
}

// TestUpsertProxiesTx_RejectsNonZeroID：仅支持新建。
func TestUpsertProxiesTx_RejectsNonZeroID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	rp := 6000
	ps := []*Proxy{
		{ID: 99, Name: "x", Type: "tcp", LocalIP: "127.0.0.1",
			LocalPort: rp, RemotePort: &rp, Enabled: true},
	}
	_, err := s.UpsertProxiesTx(ctx, ps)
	if err == nil {
		t.Fatal("expected error on non-zero ID")
	}
	if !strings.Contains(err.Error(), "non-zero ID") {
		t.Errorf("err = %v, want 'non-zero ID' message", err)
	}
}

// TestUpsertProxiesTx_EmptySlice：空切片 → nil/nil（不开事务）。
func TestUpsertProxiesTx_EmptySlice(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	ids, err := s.UpsertProxiesTx(ctx, nil)
	if err != nil {
		t.Errorf("nil slice should not error, got %v", err)
	}
	if ids != nil {
		t.Errorf("ids = %v, want nil", ids)
	}
}
