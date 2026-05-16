//go:build !windows

package procmgr

import (
	"os/exec"
	"syscall"
	"time"
)

// applyPlatformAttrs：Unix 上为子进程设置独立 process group，便于整组发信号。
func applyPlatformAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// platformKill：先 SIGTERM，最多等 3 秒；超时则 SIGKILL（02 §3.5 / §6.4）。
func platformKill(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	// 给整个 process group 发信号。
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	// 等待 3 秒由调用方的 select<-doneCh 兜底；这里仅在 3s 后补一刀。
	go func() {
		time.Sleep(3 * time.Second)
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}()
}
