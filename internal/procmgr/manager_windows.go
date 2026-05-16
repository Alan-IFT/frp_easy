//go:build windows

package procmgr

import (
	"os/exec"
	"syscall"
)

// applyPlatformAttrs 在 Windows 上给子进程一个独立 process group，
// 让我们能用 GenerateConsoleCtrlEvent / Kill 不波及父进程（02 §3.5）。
func applyPlatformAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000200, // CREATE_NEW_PROCESS_GROUP
	}
}

// platformKill 在 Windows 上直接硬 kill（02 §6.4 注解：FRP 在 Win 下对
// Ctrl+Break 支持需要 detached console，复杂度不值；MVP 阶段配置已经持久化，
// 子进程崩了无状态丢失）。
func platformKill(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
