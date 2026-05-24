//go:build !linux && !windows

package svcprobe

import (
	"context"
	"os/user"
)

// probe 兜底：darwin / freebsd / 等当前不支持服务化的平台一律返回 supervised=false。
func probe(_ context.Context) Status {
	s := Status{Supervisor: "none"}
	if u, err := user.Current(); err == nil {
		s.RunAs = u.Username
	}
	return s
}
