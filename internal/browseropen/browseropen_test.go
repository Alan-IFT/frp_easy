package browseropen

import (
	"errors"
	"os/exec"
	"runtime"
	"testing"
)

// stubTerm 让 isTerminalFunc 在测试中可控。
func stubTerm(t *testing.T, result bool) {
	t.Helper()
	orig := isTerminalFunc
	isTerminalFunc = func(_ int) bool { return result }
	t.Cleanup(func() { isTerminalFunc = orig })
}

// stubLookPath 让 lookPathFunc 在测试中可控。
func stubLookPath(t *testing.T, found bool) {
	t.Helper()
	orig := lookPathFunc
	lookPathFunc = func(_ string) (string, error) {
		if found {
			return "/usr/bin/xdg-open", nil
		}
		return "", errors.New("not found")
	}
	t.Cleanup(func() { lookPathFunc = orig })
}

// stubCommand 捕获 commandFunc 收到的参数，不真的执行。
func stubCommand(t *testing.T) *[]string {
	t.Helper()
	captured := &[]string{}
	orig := commandFunc
	commandFunc = func(name string, args ...string) *exec.Cmd {
		*captured = append(*captured, name)
		*captured = append(*captured, args...)
		// 返回一个永远 Start fail 的 cmd（执行 ":"/no-op 命令），不让测试真的拉起浏览器。
		// 用一个不存在的命令；Start() 会返回 err，但我们的测试只断言 captured。
		return exec.Command("___frp_easy_test_nonexistent_cmd___")
	}
	t.Cleanup(func() { commandFunc = orig })
	return captured
}

func TestShouldOpen_NoBrowserFlag(t *testing.T) {
	stubTerm(t, true)
	if ShouldOpen(true) {
		t.Errorf("--no-browser flag should suppress open, got true")
	}
}

func TestShouldOpen_EnvVar(t *testing.T) {
	stubTerm(t, true)
	t.Setenv("FRP_EASY_NO_BROWSER", "1")
	if ShouldOpen(false) {
		t.Errorf("FRP_EASY_NO_BROWSER=1 should suppress open, got true")
	}
}

func TestShouldOpen_NonInteractive(t *testing.T) {
	stubTerm(t, false)
	t.Setenv("FRP_EASY_NO_BROWSER", "")
	if ShouldOpen(false) {
		t.Errorf("non-TTY should suppress open (service mode), got true")
	}
}

func TestShouldOpen_InteractiveDefault(t *testing.T) {
	stubTerm(t, true)
	t.Setenv("FRP_EASY_NO_BROWSER", "")
	if runtime.GOOS == "linux" {
		stubLookPath(t, true) // 模拟 xdg-open 可用
	}
	if !ShouldOpen(false) {
		t.Errorf("interactive TTY default should open, got false")
	}
}

func TestShouldOpen_LinuxNoXdgOpen(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only path")
	}
	stubTerm(t, true)
	t.Setenv("FRP_EASY_NO_BROWSER", "")
	stubLookPath(t, false)
	if ShouldOpen(false) {
		t.Errorf("Linux without xdg-open should not open, got true")
	}
}

// stubGOOS 让 goosFunc 在测试中返回指定平台名，覆盖 runtime.GOOS。
func stubGOOS(t *testing.T, name string) {
	t.Helper()
	orig := goosFunc
	goosFunc = func() string { return name }
	t.Cleanup(func() { goosFunc = orig })
}

// TestOpen_CommandSelection 跑遍三平台分支：linux / windows / darwin。
// 不依赖 runtime.GOOS，通过 stubGOOS 注入；commandFunc 也 stub 让 Open 不真的拉起浏览器。
func TestOpen_CommandSelection(t *testing.T) {
	cases := []struct {
		goos    string
		wantCmd string
	}{
		{"windows", "rundll32"},
		{"darwin", "open"},
		{"linux", "xdg-open"},
	}
	for _, c := range cases {
		t.Run(c.goos, func(t *testing.T) {
			stubGOOS(t, c.goos)
			captured := stubCommand(t)
			_ = Open("http://127.0.0.1:8080")
			if len(*captured) < 1 {
				t.Fatalf("commandFunc not called")
			}
			got := (*captured)[0]
			if got != c.wantCmd {
				t.Errorf("command = %q, want %q (stubbed GOOS=%s)", got, c.wantCmd, c.goos)
			}
			// 最后一个参数必须是 URL（windows 上是 url.dll,FileProtocolHandler 后面那个，
			// 即 args 中的最后一个；非 windows 上是 args[0]）
			last := (*captured)[len(*captured)-1]
			if last != "http://127.0.0.1:8080" {
				t.Errorf("last arg = %q, want URL", last)
			}
		})
	}
}
