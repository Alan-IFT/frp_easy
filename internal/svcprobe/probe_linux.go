//go:build linux

package svcprobe

import (
	"context"
	"os"
	"os/exec"
	"os/user"
)

// supervisedFromEnv 纯函数：仅据 INVOCATION_ID 是否非空判定 supervised + supervisor。
// systemd 232+ 文档化为所有 service 进程注入 INVOCATION_ID（systemd.exec(5)）。
// 抽出便于用 t.Setenv 在不真起 systemd 的前提下测两个分支（T-050 A-2）。
// 行为与重构前内联判定字节级等价。
func supervisedFromEnv(invocationID string) (supervised bool, supervisor string) {
	if invocationID != "" {
		return true, "systemd"
	}
	return false, "none"
}

// runAsFrom 纯函数：把 user.Current() 的结果与 $USER 兜底合并出 run_as。
// curName 为 user.Current().Username（curErr==nil 时有效），envUser 为 $USER 兜底。
// 抽出便于测"user.Current 失败 → 退回 $USER"分支（T-050 A-2）。
// 行为与重构前的 if/else 字节级等价。
func runAsFrom(curName string, curErr error, envUser string) string {
	if curErr == nil {
		return curName
	}
	return envUser
}

// probe 在 Linux 下用 systemd 约定探测：
//
//   - supervised = (INVOCATION_ID 环境变量非空) —— systemd 232+ 文档化注入给所有
//     service 进程（参 systemd.exec(5) "Environment Variables in Spawned Processes"）。
//   - supervisor = "systemd"（若 supervised）/ "none"。
//   - boot_autostart = `systemctl is-enabled frp-easy.service` stdout == "enabled"。
//   - run_as = os/user.Current() 的 Username（兜底 $USER）。
func probe(ctx context.Context) Status {
	s := Status{Supervisor: "none"}

	s.Supervised, s.Supervisor = supervisedFromEnv(os.Getenv("INVOCATION_ID"))

	u, uerr := user.Current()
	curName := ""
	if uerr == nil {
		curName = u.Username
	}
	s.RunAs = runAsFrom(curName, uerr, os.Getenv("USER"))

	// boot_autostart 通过 systemctl 命令探测；失败降级为 false。
	if ctx.Err() != nil {
		s.ProbeError = "probe timeout"
		return s
	}
	out, err := exec.CommandContext(ctx, "systemctl", "is-enabled", "frp-easy.service").Output()
	if err == nil && parseIsEnabled(out) {
		s.BootAutostart = true
	}
	return s
}
