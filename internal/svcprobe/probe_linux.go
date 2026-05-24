//go:build linux

package svcprobe

import (
	"context"
	"os"
	"os/exec"
	"os/user"
	"strings"
)

// probe 在 Linux 下用 systemd 约定探测：
//
//   - supervised = (INVOCATION_ID 环境变量非空) —— systemd 232+ 文档化注入给所有
//     service 进程（参 systemd.exec(5) "Environment Variables in Spawned Processes"）。
//   - supervisor = "systemd"（若 supervised）/ "none"。
//   - boot_autostart = `systemctl is-enabled frp-easy.service` stdout == "enabled"。
//   - run_as = os/user.Current() 的 Username（兜底 $USER）。
func probe(ctx context.Context) Status {
	s := Status{Supervisor: "none"}

	if os.Getenv("INVOCATION_ID") != "" {
		s.Supervised = true
		s.Supervisor = "systemd"
	}

	if u, err := user.Current(); err == nil {
		s.RunAs = u.Username
	} else {
		s.RunAs = os.Getenv("USER")
	}

	// boot_autostart 通过 systemctl 命令探测；失败降级为 false。
	if ctx.Err() != nil {
		s.ProbeError = "probe timeout"
		return s
	}
	out, err := exec.CommandContext(ctx, "systemctl", "is-enabled", "frp-easy.service").Output()
	if err == nil && strings.TrimSpace(string(out)) == "enabled" {
		s.BootAutostart = true
	}
	return s
}
