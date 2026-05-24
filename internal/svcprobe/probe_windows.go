//go:build windows

package svcprobe

import (
	"context"
	"os/exec"
	"os/user"
	"regexp"

	"golang.org/x/sys/windows/svc"
)

// autoStartRE 匹配 sc.exe qc 输出中 START_TYPE 含 AUTO_START 的行。
// sc.exe 输出格式跨语言一致："START_TYPE         : 2   AUTO_START"。
var autoStartRE = regexp.MustCompile(`START_TYPE\s*:\s*2\s+AUTO_START`)

// probe 在 Windows 下：
//
//   - supervised = svc.IsWindowsService()（与 cmd/frp-easy/service_windows.go::isWindowsService 一致）。
//   - supervisor = "windows-service" / "none"。
//   - boot_autostart = `sc.exe qc frp-easy` 输出含 START_TYPE: 2 AUTO_START。
//   - run_as = supervised 时硬编码 "LocalSystem"（sc.exe 默认无 obj= 参数下的实际账户）；
//     非 supervised 时取 os/user.Current().Username。
func probe(ctx context.Context) Status {
	s := Status{Supervisor: "none"}

	inSvc, err := svc.IsWindowsService()
	if err == nil && inSvc {
		s.Supervised = true
		s.Supervisor = "windows-service"
		s.RunAs = "LocalSystem"
	} else {
		if u, uerr := user.Current(); uerr == nil {
			s.RunAs = u.Username
		}
	}

	if ctx.Err() != nil {
		s.ProbeError = "probe timeout"
		return s
	}
	out, err := exec.CommandContext(ctx, "sc.exe", "qc", "frp-easy").Output()
	if err == nil && autoStartRE.Match(out) {
		s.BootAutostart = true
	}
	return s
}
