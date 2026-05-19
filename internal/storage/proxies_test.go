package storage

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestUpsertProxy_DuplicateNameReturnsSentinel 验证 AC-6.1：name 列 UNIQUE 冲突
// 必须返回 ErrDuplicateName sentinel（而不是包装错误），handler 才能据此映射 409。
func TestUpsertProxy_DuplicateNameReturnsSentinel(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	rp := 6000
	first := &Proxy{
		Name:       "dup-name",
		Type:       "tcp",
		LocalIP:    "127.0.0.1",
		LocalPort:  22,
		RemotePort: &rp,
		Enabled:    true,
	}
	if err := s.UpsertProxy(ctx, first); err != nil {
		t.Fatalf("first insert should succeed: %v", err)
	}

	// 再插一个同名规则（remotePort 故意换成 6001，避开 (type,remote_port) 冲突）。
	rp2 := 6001
	dup := &Proxy{
		Name:       "dup-name",
		Type:       "tcp",
		LocalIP:    "127.0.0.1",
		LocalPort:  23,
		RemotePort: &rp2,
		Enabled:    true,
	}
	err := s.UpsertProxy(ctx, dup)
	if err == nil {
		t.Fatal("duplicate name insert should fail")
	}
	if !errors.Is(err, ErrDuplicateName) {
		t.Errorf("expected ErrDuplicateName, got %v (type %T)", err, err)
	}
}

// TestUpsertProxy_DuplicateTypeRemotePortNotSentinel 验证 AC-6.2：
// (type, remote_port) 部分唯一索引冲突 **不** 返回 ErrDuplicateName，
// 上层 handler 据此继续走 422 兜底分支。
func TestUpsertProxy_DuplicateTypeRemotePortNotSentinel(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	rp := 6000
	first := &Proxy{
		Name:       "first",
		Type:       "tcp",
		LocalIP:    "127.0.0.1",
		LocalPort:  22,
		RemotePort: &rp,
		Enabled:    true,
	}
	if err := s.UpsertProxy(ctx, first); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// name 不同，但 (type=tcp, remotePort=6000) 与上一条冲突
	dup := &Proxy{
		Name:       "second",
		Type:       "tcp",
		LocalIP:    "127.0.0.1",
		LocalPort:  23,
		RemotePort: &rp,
		Enabled:    true,
	}
	err := s.UpsertProxy(ctx, dup)
	if err == nil {
		t.Fatal("duplicate (type,remotePort) insert should fail")
	}
	if errors.Is(err, ErrDuplicateName) {
		t.Errorf("expected NOT ErrDuplicateName for (type,remotePort) conflict, got %v", err)
	}
	// 错误文本仍应是 UNIQUE 冲突（仅是不同列），用于 handler 422 路径识别
	if !strings.Contains(strings.ToLower(err.Error()), "unique") {
		t.Errorf("expected UNIQUE-related error text, got: %v", err)
	}
}

// TestUpsertProxy_UpdateToDuplicateNameReturnsSentinel 验证 UPDATE 路径也走 sentinel。
func TestUpsertProxy_UpdateToDuplicateNameReturnsSentinel(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	rp1 := 7000
	a := &Proxy{
		Name: "alpha", Type: "tcp", LocalIP: "127.0.0.1",
		LocalPort: 22, RemotePort: &rp1, Enabled: true,
	}
	if err := s.UpsertProxy(ctx, a); err != nil {
		t.Fatalf("insert alpha: %v", err)
	}
	rp2 := 7001
	b := &Proxy{
		Name: "beta", Type: "tcp", LocalIP: "127.0.0.1",
		LocalPort: 23, RemotePort: &rp2, Enabled: true,
	}
	if err := s.UpsertProxy(ctx, b); err != nil {
		t.Fatalf("insert beta: %v", err)
	}

	// 把 beta 改名为 alpha → name UNIQUE 冲突
	b.Name = "alpha"
	err := s.UpsertProxy(ctx, b)
	if err == nil {
		t.Fatal("update to duplicate name should fail")
	}
	if !errors.Is(err, ErrDuplicateName) {
		t.Errorf("expected ErrDuplicateName on UPDATE, got %v", err)
	}
}

// TestIsDuplicateNameError_DirectChecks 单测 isDuplicateNameError 的判断逻辑，
// 防止驱动错误文本未来变更时悄无声息地破坏 sentinel 映射。
func TestIsDuplicateNameError_DirectChecks(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"name-conflict", errors.New("UNIQUE constraint failed: proxies.name"), true},
		{"name-conflict-wrapped",
			errors.New("storage.UpsertProxy insert: UNIQUE constraint failed: proxies.name (2067)"),
			true},
		{"type-remote-conflict",
			errors.New("UNIQUE constraint failed: proxies.type, proxies.remote_port"),
			false},
		{"unrelated", errors.New("some other error"), false},
		{"missing-prefix", errors.New("constraint failed proxies.name"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isDuplicateNameError(c.err); got != c.want {
				t.Errorf("isDuplicateNameError(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}
