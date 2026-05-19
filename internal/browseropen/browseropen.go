// Package browseropen 在交互式终端启动 frp-easy 时自动打开 UI URL。
//
// 设计动机（T-010）：DEPLOYMENT 路径 A 的目标用户是 Windows 普通用户（双击
// frp-easy.exe），他们不应该被迫去 stderr 里复制 URL。但 systemd / Windows
// Service 场景必须不开浏览器，否则要么报错（无 DISPLAY）要么污染 server。
//
// 决策模型：
//
//	┌────────────────────────────────────────────────────────────┐
//	│ ShouldOpen(noBrowserFlag) 决策树                             │
//	├────────────────────────────────────────────────────────────┤
//	│ if noBrowserFlag                            → false        │
//	│ if FRP_EASY_NO_BROWSER 环境变量 != ""        → false        │
//	│ if !IsTerminal(stdin)                       → false        │
//	│ if Linux && !exec.LookPath("xdg-open")      → false        │
//	│ otherwise                                    → true         │
//	└────────────────────────────────────────────────────────────┘
//
// Open(url) 仅做"启动平台命令"；失败由调用方记 WARN，不阻塞主流程。
package browseropen

import (
	"os"
	"os/exec"
	"runtime"

	"golang.org/x/term"
)

// commandFunc 是 exec.Command 的可注入版本，测试时替换以避免真的拉起浏览器。
var commandFunc = exec.Command

// lookPathFunc 是 exec.LookPath 的可注入版本，测试时替换。
var lookPathFunc = exec.LookPath

// stdinFd 是 os.Stdin.Fd() 的可注入版本，测试时返回非 TTY fd。
var stdinFd = func() uintptr { return os.Stdin.Fd() }

// isTerminalFunc 是 term.IsTerminal 的可注入版本。
var isTerminalFunc = term.IsTerminal

// goosFunc 返回当前平台名；可注入以让 TestOpen_CommandSelection 跑遍三平台
// 分支而不只是 runtime.GOOS。
var goosFunc = func() string { return runtime.GOOS }

// ShouldOpen 决定本次启动是否应自动打开浏览器。
//
// noBrowserFlag 由 cmd/frp-easy 的 --no-browser CLI flag 注入；
// 通过参数注入避免本包反向依赖 cmd 包或全局变量。
func ShouldOpen(noBrowserFlag bool) bool {
	if noBrowserFlag {
		return false
	}
	if os.Getenv("FRP_EASY_NO_BROWSER") != "" {
		return false
	}
	if !isTerminalFunc(int(stdinFd())) {
		return false
	}
	if runtime.GOOS == "linux" {
		if _, err := lookPathFunc("xdg-open"); err != nil {
			return false
		}
	}
	return true
}

// Open 调用平台默认浏览器打开 URL。
// 不等待子进程退出（仅 Start，不 Wait）；失败返回 err。
func Open(url string) error {
	var cmd *exec.Cmd
	switch goosFunc() {
	case "windows":
		// rundll32 url.dll,FileProtocolHandler <url> 是 Windows 推荐方式；
		// 比 `cmd /c start "" "<url>"` 在 URL 含 & 等字符时更稳。
		cmd = commandFunc("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = commandFunc("open", url)
	default: // linux / freebsd / openbsd / netbsd
		cmd = commandFunc("xdg-open", url)
	}
	return cmd.Start()
}
