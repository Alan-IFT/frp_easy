//go:build linux

package svcprobe

// T-050 A-2：Linux 平台分支覆盖。
//
// 此前 probe_linux.go 的 INVOCATION_ID / $USER 兜底分支零断言。本文件用 t.Setenv
// 注入真实环境，覆盖：
//   - supervisedFromEnv：INVOCATION_ID 非空 → (true,"systemd")；空 → (false,"none")。
//   - runAsFrom：user.Current 成功 → 用其 Username；失败 → 退回 $USER。
//   - probe() 端到端：t.Setenv("INVOCATION_ID","x") 后 Supervised==true 且 Supervisor=="systemd"；
//     清空后 Supervised==false 且 Supervisor=="none"。
//
// 反向证伪点：
//   - 若 supervised 判定写反，setenv 用例的 Supervised 会与预期相反。
//   - 若 runAsFrom 在 curErr!=nil 时仍返回 curName，envUser 兜底用例会失败。
//
// 仅 linux build tag —— 与 probe_linux.go 同平台。

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSupervisedFromEnv(t *testing.T) {
	cases := []struct {
		name        string
		invocation  string
		wantSup     bool
		wantSupName string
	}{
		{"systemd-injected", "abc123def456", true, "systemd"},
		{"empty", "", false, "none"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sup, name := supervisedFromEnv(c.invocation)
			if sup != c.wantSup || name != c.wantSupName {
				t.Errorf("supervisedFromEnv(%q)=(%v,%q) want (%v,%q)",
					c.invocation, sup, name, c.wantSup, c.wantSupName)
			}
		})
	}
}

func TestRunAsFrom(t *testing.T) {
	if got := runAsFrom("alice", nil, "bob"); got != "alice" {
		t.Errorf("user.Current 成功时应返回 curName=alice, got %q", got)
	}
	if got := runAsFrom("alice", errors.New("no current user"), "bob"); got != "bob" {
		t.Errorf("user.Current 失败时应退回 $USER=bob, got %q", got)
	}
	if got := runAsFrom("", errors.New("fail"), ""); got != "" {
		t.Errorf("两端皆空时应为空串, got %q", got)
	}
}

// TestProbe_Linux_InvocationIDBranch 用真实 env 注入覆盖 probe() 的 supervised 分支。
func TestProbe_Linux_InvocationIDBranch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t.Setenv("INVOCATION_ID", "test-invocation-id")
	s := Probe(ctx)
	if !s.Supervised || s.Supervisor != "systemd" {
		t.Errorf("INVOCATION_ID 非空时应 Supervised=true Supervisor=systemd, got %+v", s)
	}

	t.Setenv("INVOCATION_ID", "")
	s = Probe(ctx)
	if s.Supervised || s.Supervisor != "none" {
		t.Errorf("INVOCATION_ID 空时应 Supervised=false Supervisor=none, got %+v", s)
	}
}
