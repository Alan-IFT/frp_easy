package procmgr

// T-050 A-1：procmgr 子进程生命周期"成功路径"覆盖。
//
// 此前所有 procmgr 测试都用 fakeLocator 让 Start 在 binary-missing 早返回，从不真
// spawn 子进程 —— supervise / waitUntilStable / ringBuffer / Restart / 状态机的
// "成功"转换零断言。
//
// 实现取舍（重要）：标准库的 "TestHelperProcess + os.Args[0]" 模式在这里**不可用**，
// 因为 procmgr.Start 硬编码 exec.Command(binPath, "-c", cfgPath)，把测试二进制当
// "frpc" 拉起时，Go testing 的 flag 解析器会拒绝未知 flag "-c" 并以 exit 2 退出，
// 无法到达 helper 逻辑（无法注入 -test.run / -- 分隔符）。故改用更稳健的跨平台做法：
// 测试时用 `go build` 编译一个独立的小 helper 程序到 t.TempDir()，helper 显式接受
// -c <path>（不会 choke）+ 读环境变量 GO_HELPER_MODE 决定"持续运行 N 秒 / 立即崩溃"。
// 把 locator 指向这个真二进制即可走完整 spawn 路径。
//
// 慢测试（依赖 waitUntilStable 的 3s 稳定窗口 + go build 编译耗时）用 testing.Short()
// 门控，`go test -short` 可跳过。
//
// 覆盖：
//   - Start → Running 转换（helper 持续运行，撑过 3s waitUntilStable）。
//   - 子进程立即崩溃 → Start 返回 err + State==error（waitUntilStable 提前判失败）。
//   - Stop 真停一个 running 子进程 → State==stopped、PID==0。
//   - Restart 重启（停旧 + 起新，PID 变化）。
//   - ringBuffer 满后丢老行（纯单元，不 spawn）。
//   - waitUntilStable 对 error/stopped/starting 的转换（纯单元，不 spawn）。
//
// 反向证伪点：
//   - 若 supervise 不把 starting 推到 running，Running 用例会卡在 starting 直至超时。
//   - 若崩溃进程没被 supervise 标 error，崩溃用例的 Start 会误判成功。
//   - 若 ringBuffer 不丢老行，满载后 JoinTail 会含被丢的行。

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// helperLocator 让 FRPCPath/FRPSPath 返回编译出的 helper 二进制路径。
type helperLocator struct{ path string }

func (h helperLocator) FRPCPath() (string, error) { return h.path, nil }
func (h helperLocator) FRPSPath() (string, error) { return h.path, nil }
func (h helperLocator) Missing() []string         { return nil }
func (h helperLocator) Root() string              { return "" }

// helperSrc 是被编译成独立 helper 程序的源码。它显式接受 -c flag（吞掉 procmgr 传入
// 的 -c <cfgPath>，避免 flag 解析报错），并按 GO_HELPER_MODE 决定行为：
//   - "crash"：立即以非零码退出（模拟子进程一启动就崩）。
//   - 其它（默认 "run"）：先吐一行日志，再持续运行 GO_HELPER_SECS 秒后退出。
const helperSrc = `package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	c := flag.String("c", "", "config path (ignored)")
	flag.Parse()
	_ = *c
	switch os.Getenv("GO_HELPER_MODE") {
	case "crash":
		fmt.Fprintln(os.Stderr, "helper: crashing immediately")
		os.Exit(3)
	default:
		secs := 5
		if v := os.Getenv("GO_HELPER_SECS"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				secs = n
			}
		}
		fmt.Fprintln(os.Stdout, "helper: running")
		time.Sleep(time.Duration(secs) * time.Second)
		os.Exit(0)
	}
}
`

// buildHelperBinary 用 go build 把 helperSrc 编译到 dir 下，返回可执行文件路径。
// 失败时 t.Skip（编译环境异常不应算业务测试失败）。
func buildHelperBinary(t *testing.T, dir string) string {
	t.Helper()
	srcDir := filepath.Join(dir, "helpersrc")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir helpersrc: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "main.go"), []byte(helperSrc), 0o644); err != nil {
		t.Fatalf("write helper src: %v", err)
	}
	// 独立的最小 go.mod，免受仓库主 module 影响。
	if err := os.WriteFile(filepath.Join(srcDir, "go.mod"), []byte("module helper\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("write helper go.mod: %v", err)
	}
	binName := "helper"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(dir, binName)
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = srcDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("无法编译 helper 二进制（跳过 spawn 测试）：%v\n%s", err, out)
	}
	return binPath
}

// newHelperManager 编译 helper 二进制并建一个会真 spawn 它的 Manager。
func newHelperManager(t *testing.T, mode string, secs int) *Manager {
	t.Helper()
	dir := t.TempDir()
	bin := buildHelperBinary(t, dir)
	cfg := filepath.Join(dir, "frpc.toml")
	if err := os.WriteFile(cfg, []byte("# helper\n"), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	m := New(Config{
		Locator:     helperLocator{path: bin},
		ConfigPaths: map[string]string{"frpc": cfg, "frps": cfg},
		LogFiles:    map[string]string{"frpc": filepath.Join(dir, "frpc.log")},
	})
	// procmgr.Start 内部用 exec.Command(...) 默认继承父进程 env；通过设置当前进程
	// 环境变量把 mode/secs 传给即将 spawn 的 helper 子进程。
	t.Setenv("GO_HELPER_MODE", mode)
	if secs > 0 {
		t.Setenv("GO_HELPER_SECS", strconv.Itoa(secs))
	} else {
		os.Unsetenv("GO_HELPER_SECS")
	}
	return m
}

// pollState 轮询直到 kind 的 State == want 或 deadline（poll-until-condition）。
func pollState(t *testing.T, m *Manager, kind string, want State, deadline time.Duration) {
	t.Helper()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if m.Status(kind).State == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s state 未在 %v 内到达 %q，当前 %q", kind, deadline, want, m.Status(kind).State)
}

// TestLifecycle_StartRunning：helper 持续运行 → Start 撑过 3s 稳定窗口 → State=running。
func TestLifecycle_StartRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过慢测试（waitUntilStable 3s 窗口）：-short")
	}
	m := newHelperManager(t, "run", 8)
	defer m.Shutdown()

	info, err := m.Start("frpc")
	if err != nil {
		t.Fatalf("Start 应成功（helper 持续运行）, got err=%v info=%+v", err, info)
	}
	if info.State != StateRunning {
		t.Errorf("Start 返回后 State=%q want running", info.State)
	}
	if info.PID == 0 {
		t.Errorf("running 子进程 PID 不应为 0")
	}
}

// TestLifecycle_CrashGoesError：helper 立即崩 → waitUntilStable 提前判失败 → State=error。
func TestLifecycle_CrashGoesError(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过慢测试：-short")
	}
	m := newHelperManager(t, "crash", 0)
	defer m.Shutdown()

	_, err := m.Start("frpc")
	if err == nil {
		t.Fatalf("Start 应返回错误（子进程立即崩溃）")
	}
	// supervise 会把崩溃标记成 error（非由 Stop 触发）。poll 等 supervise 收尾。
	pollState(t, m, "frpc", StateError, 3*time.Second)
	if le := m.Status("frpc").LastErr; le == "" {
		t.Errorf("崩溃后 LastErr 应非空（含 exit/tail 摘要）")
	}
}

// TestLifecycle_StartThenStop：起一个 running 子进程后 Stop → State=stopped、PID=0。
func TestLifecycle_StartThenStop(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过慢测试：-short")
	}
	m := newHelperManager(t, "run", 30)
	defer m.Shutdown()

	if _, err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	pollState(t, m, "frpc", StateRunning, 5*time.Second)

	info, err := m.Stop("frpc")
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if info.State != StateStopped {
		t.Errorf("Stop 后 State=%q want stopped", info.State)
	}
	if info.PID != 0 {
		t.Errorf("Stop 后 PID=%d want 0", info.PID)
	}
}

// TestLifecycle_Restart：Restart = Stop 旧 + Start 新，最终 running 且 PID 变化。
func TestLifecycle_Restart(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过慢测试：-short")
	}
	m := newHelperManager(t, "run", 30)
	defer m.Shutdown()

	if _, err := m.Start("frpc"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	pollState(t, m, "frpc", StateRunning, 5*time.Second)
	oldPID := m.Status("frpc").PID

	info, err := m.Restart("frpc")
	if err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if info.State != StateRunning {
		t.Errorf("Restart 后 State=%q want running", info.State)
	}
	if info.PID == 0 {
		t.Errorf("Restart 后 PID 不应为 0")
	}
	if info.PID == oldPID {
		t.Errorf("Restart 后 PID 应变化（旧=%d 新=%d）", oldPID, info.PID)
	}
}

// --- 纯单元（不 spawn 子进程，任何平台 + -short 都跑） ---

// TestRingBuffer_DropsOldest：ringBuffer 满载后丢最老的行，只留末 N 条。
func TestRingBuffer_DropsOldest(t *testing.T) {
	rb := newRingBuffer(3)
	for i := 1; i <= 5; i++ {
		rb.Push(fmt.Sprintf("line%d", i))
	}
	got := rb.JoinTail()
	want := "line3 | line4 | line5"
	if got != want {
		t.Errorf("ringBuffer 满后应留末 3 行 %q, got %q", want, got)
	}
	// 反向证伪：被丢弃的 line1/line2 不应出现。
	if strings.Contains(got, "line1") || strings.Contains(got, "line2") {
		t.Errorf("被丢弃的老行不应出现在 tail: %q", got)
	}
}

// TestRingBuffer_UnderCapacity：未满时按序保留全部。
func TestRingBuffer_UnderCapacity(t *testing.T) {
	rb := newRingBuffer(5)
	rb.Push("a")
	rb.Push("b")
	if got := rb.JoinTail(); got != "a | b" {
		t.Errorf("未满时应保留全部按序，got %q", got)
	}
}

// TestWaitUntilStable_ErrorEarlyReturn：state 已是 error 时 waitUntilStable 提前返回错误。
func TestWaitUntilStable_ErrorEarlyReturn(t *testing.T) {
	m := New(Config{Locator: &fakeLocator{}})
	m.mu.Lock()
	m.processes["frpc"].info.State = StateError
	m.mu.Unlock()
	if err := m.waitUntilStable("frpc", 2*time.Second); err == nil {
		t.Error("state=error 时 waitUntilStable 应返回错误")
	}
}

// TestWaitUntilStable_StoppedEarlyReturn：state 已是 stopped（进程消失）时提前返回错误。
func TestWaitUntilStable_StoppedEarlyReturn(t *testing.T) {
	m := New(Config{Locator: &fakeLocator{}})
	m.mu.Lock()
	m.processes["frpc"].info.State = StateStopped
	m.mu.Unlock()
	if err := m.waitUntilStable("frpc", 2*time.Second); err == nil {
		t.Error("state=stopped 时 waitUntilStable 应返回错误")
	}
}

// TestWaitUntilStable_StartingToRunning：state=starting 撑过窗口 → 推到 running，返回 nil。
func TestWaitUntilStable_StartingToRunning(t *testing.T) {
	m := New(Config{Locator: &fakeLocator{}})
	m.mu.Lock()
	m.processes["frpc"].info.State = StateStarting
	m.mu.Unlock()
	if err := m.waitUntilStable("frpc", 150*time.Millisecond); err != nil {
		t.Errorf("starting 撑过窗口应返回 nil, got %v", err)
	}
	if st := m.Status("frpc").State; st != StateRunning {
		t.Errorf("waitUntilStable 后 State=%q want running", st)
	}
}
