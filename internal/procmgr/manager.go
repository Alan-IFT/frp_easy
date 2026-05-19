// Package procmgr 管理 frpc / frps 子进程的生命周期。
//
// 设计契约见 02 §3.5、§6.4：
//   - 每个 kind（frpc / frps）一个 supervisor goroutine：fork → tee
//     stdout/stderr 到日志文件 → 监听退出 → 广播状态事件。
//   - 启动 = 写 TOML（调用方负责）→ fork → 等 3s 确认未退出 → state=running。
//   - 配置变更 frpc = frpcadmin.Reload(5s) → 失败 → Restart。
//   - 配置变更 frps = 始终 Restart。
//   - Stop：Linux SIGTERM→3s→SIGKILL；Windows cmd.Process.Kill()。
//
// 进程日志路径：构造时由 logFiles 指定（main.go 把 frpc.log / frps.log 路径
// 注入）；子进程 stdout/stderr 同时 tee 到该文件以兜底（FRP 在 toml 里
// 配 log.to 已经会写，但子进程在配置载入前可能先吐到 stderr，故双写）。
package procmgr

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// State 是 02 §3.5 的状态枚举。
type State string

const (
	StateStopped  State = "stopped"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateStopping State = "stopping"
	StateError    State = "error"
)

// ProcessInfo 是单个 kind 的当前快照。
type ProcessInfo struct {
	Kind      string    `json:"kind"`
	State     State     `json:"state"`
	PID       int       `json:"pid"`
	LastErr   string    `json:"lastErr,omitempty"`
	ChangedAt time.Time `json:"changedAt"`
}

// StatusEvent 是状态广播事件。
type StatusEvent struct {
	Kind string
	Info ProcessInfo
}

// Locator 抽象出二进制定位（避免对 binloc 包硬依赖）。
// binloc.Locator 实现了同名方法。
type Locator interface {
	FRPCPath() (string, error)
	FRPSPath() (string, error)
	Missing() []string
}

// FrpcReloader 抽象出 frpc admin reload 调用（避免对 frpcadmin 硬依赖）。
type FrpcReloader interface {
	Reload(ctx context.Context, strict bool) error
}

// Manager 是 procmgr 的入口；每个 kind 内部有一个 supervisor。
type Manager struct {
	loc         Locator
	configPaths map[string]string // kind → frpc.toml / frps.toml 路径
	logFiles    map[string]string // kind → 子进程合并日志文件
	reloader    FrpcReloader      // 可 nil；ApplyConfigChange("frpc") 时为 nil 则直接 Restart

	mu        sync.Mutex
	processes map[string]*processState

	subMu       sync.Mutex
	subscribers []chan StatusEvent
}

type processState struct {
	info   ProcessInfo
	cmd    *exec.Cmd
	cancel context.CancelFunc // 关闭 supervisor goroutine
	doneCh chan struct{}      // supervisor 关闭后 close
}

// Config 构造 Manager 所需的全部依赖。
type Config struct {
	Locator     Locator
	ConfigPaths map[string]string // kind → toml 路径
	LogFiles    map[string]string // kind → 子进程合并日志路径
	Reloader    FrpcReloader      // 可 nil
}

// New 建一个 Manager。
func New(c Config) *Manager {
	m := &Manager{
		loc:         c.Locator,
		configPaths: c.ConfigPaths,
		logFiles:    c.LogFiles,
		reloader:    c.Reloader,
		processes:   map[string]*processState{},
	}
	for _, kind := range []string{"frpc", "frps"} {
		m.processes[kind] = &processState{
			info: ProcessInfo{Kind: kind, State: StateStopped, ChangedAt: time.Now().UTC()},
		}
	}
	return m
}

// Subscribe 返回一个事件通道（cap=16，慢消费者旧事件被丢弃）。
func (m *Manager) Subscribe() <-chan StatusEvent {
	ch := make(chan StatusEvent, 16)
	m.subMu.Lock()
	m.subscribers = append(m.subscribers, ch)
	m.subMu.Unlock()
	return ch
}

func (m *Manager) emit(ev StatusEvent) {
	m.subMu.Lock()
	subs := append([]chan StatusEvent(nil), m.subscribers...)
	m.subMu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
			// 慢消费者：丢老事件腾一个槽再塞。
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- ev:
			default:
			}
		}
	}
}

// Status 返回当前 kind 的 ProcessInfo 快照。
func (m *Manager) Status(kind string) ProcessInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ps, ok := m.processes[kind]; ok {
		return ps.info
	}
	return ProcessInfo{Kind: kind, State: StateStopped}
}

// StatusAll 返回 frpc / frps 各一份快照。
func (m *Manager) StatusAll() map[string]ProcessInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]ProcessInfo{}
	for k, ps := range m.processes {
		out[k] = ps.info
	}
	return out
}

// Start 启动 kind 子进程。
//
// 若当前 State ∈ {starting, running} 直接返回当前 ProcessInfo + 不报错（idempotent）。
// 若二进制缺失（locator 报错）返回错误 + state 不变。
// 启动后等待 3 秒确认未退出 → state=running。
//
// 【T-007 AC-5】实现采用单一 defer-unlock 模式：所有早返回不再手动 unlock；
// 持锁段封装在 IIFE 内，IIFE 退出时 defer 释放锁；解锁后才 emit / supervise /
// waitUntilStable（保留"不在持锁期间 emit"的不变量，避免与慢消费者死锁）。
//
// 关键不变量（外部可观察行为）：
//   - idempotent Start 时返回的 ProcessInfo、错误值类型不变。
//   - emit 次数 / 顺序：cmd.Start 失败 → emit error 一次；成功 → emit starting
//     一次，supervise 内部再 emit running / stopped / error。
//   - waitUntilStable 仅在 cmd.Start 成功时调用，且必在 unlock 之后。
func (m *Manager) Start(kind string) (ProcessInfo, error) {
	if err := validateKind(kind); err != nil {
		return ProcessInfo{}, err
	}

	// 第一段（IIFE，持锁）输出到这些局部变量；第二段（解锁后）据此决策。
	var (
		infoSnapshot ProcessInfo
		shouldEmit   bool // cmd.Start 成功 / cmd.Start 失败 均 emit；早返回（idempotent / 校验失败）不 emit
		startErr     error
		successPath  bool // 仅 cmd.Start 成功路径走 supervise + waitUntilStable
		startCmd     *exec.Cmd
		startCtx     context.Context
		startDone    chan struct{}
		stdoutPipe   io.ReadCloser
		stderrPipe   io.ReadCloser
		logPath      string
	)

	func() {
		m.mu.Lock()
		defer m.mu.Unlock()

		ps := m.processes[kind]
		switch ps.info.State {
		case StateStarting, StateRunning:
			// idempotent 早返回（不 emit、不报错）
			infoSnapshot = ps.info
			return
		case StateStopping:
			infoSnapshot = ps.info
			startErr = fmt.Errorf("procmgr.Start(%s): currently stopping", kind)
			return
		}

		binPath, err := m.binPathFor(kind)
		if err != nil {
			infoSnapshot = ps.info
			startErr = err
			return
		}
		cfgPath, ok := m.configPaths[kind]
		if !ok || cfgPath == "" {
			infoSnapshot = ps.info
			startErr = fmt.Errorf("procmgr.Start(%s): no config path configured", kind)
			return
		}
		logPath = m.logFiles[kind]
		if logPath != "" {
			if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
				infoSnapshot = ps.info
				startErr = fmt.Errorf("procmgr.Start(%s) mkdir log: %w", kind, err)
				return
			}
		}

		cmd := exec.Command(binPath, "-c", cfgPath)
		cmd.Dir = filepath.Dir(binPath)
		applyPlatformAttrs(cmd)
		stdoutPipe, _ = cmd.StdoutPipe()
		stderrPipe, _ = cmd.StderrPipe()

		if err := cmd.Start(); err != nil {
			ps.info.State = StateError
			ps.info.LastErr = err.Error()
			ps.info.ChangedAt = time.Now().UTC()
			infoSnapshot = ps.info
			shouldEmit = true
			startErr = fmt.Errorf("procmgr.Start(%s): %w", kind, err)
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		doneCh := make(chan struct{})
		ps.cmd = cmd
		ps.cancel = cancel
		ps.doneCh = doneCh
		ps.info.State = StateStarting
		ps.info.PID = cmd.Process.Pid
		ps.info.LastErr = ""
		ps.info.ChangedAt = time.Now().UTC()
		infoSnapshot = ps.info
		shouldEmit = true
		successPath = true
		startCmd = cmd
		startCtx = ctx
		startDone = doneCh
	}()

	// 第二段（已解锁）：emit + supervise + waitUntilStable
	if shouldEmit {
		m.emit(StatusEvent{Kind: kind, Info: infoSnapshot})
	}
	if startErr != nil {
		return infoSnapshot, startErr
	}
	if !successPath {
		// idempotent 早返回（StateStarting / StateRunning）
		return infoSnapshot, nil
	}

	// supervisor goroutine：负责 tee 日志 + 监听退出。
	go m.supervise(startCtx, kind, startCmd, stdoutPipe, stderrPipe, logPath, startDone)

	// 等 3 秒确认未自我退出（02 §6.4）。
	if waitErr := m.waitUntilStable(kind, 3*time.Second); waitErr != nil {
		return m.Status(kind), waitErr
	}
	return m.Status(kind), nil
}

// Stop 停止 kind 子进程。idempotent。
func (m *Manager) Stop(kind string) (ProcessInfo, error) {
	if err := validateKind(kind); err != nil {
		return ProcessInfo{}, err
	}
	m.mu.Lock()
	ps := m.processes[kind]
	if ps.info.State == StateStopped || ps.cmd == nil || ps.cmd.Process == nil {
		info := ps.info
		m.mu.Unlock()
		return info, nil
	}
	ps.info.State = StateStopping
	ps.info.ChangedAt = time.Now().UTC()
	info := ps.info
	cmd := ps.cmd
	doneCh := ps.doneCh
	cancel := ps.cancel
	m.mu.Unlock()
	m.emit(StatusEvent{Kind: kind, Info: info})

	platformKill(cmd)

	// 等子进程退出（最多 5 秒）。
	select {
	case <-doneCh:
	case <-time.After(5 * time.Second):
		// 兜底强杀
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-doneCh:
		case <-time.After(2 * time.Second):
		}
	}
	if cancel != nil {
		cancel()
	}

	m.mu.Lock()
	ps.info.State = StateStopped
	ps.info.PID = 0
	ps.info.ChangedAt = time.Now().UTC()
	ps.cmd = nil
	ps.cancel = nil
	ps.doneCh = nil
	info = ps.info
	m.mu.Unlock()
	m.emit(StatusEvent{Kind: kind, Info: info})
	return info, nil
}

// Restart 等于 Stop + Start。
func (m *Manager) Restart(kind string) (ProcessInfo, error) {
	if _, err := m.Stop(kind); err != nil {
		return m.Status(kind), err
	}
	return m.Start(kind)
}

// ApplyConfigChange 在配置文件已重写后通知子进程重新加载。
//
// 调用方负责先把新 TOML 写到 m.configPaths[kind]。
//
// 对 frpc：调 reloader.Reload（5s 超时）；失败则 Restart。整段 5s 预算。
// 对 frps：始终 Restart（上游无 reload）。
func (m *Manager) ApplyConfigChange(kind string) error {
	if err := validateKind(kind); err != nil {
		return err
	}
	// 未运行 → 不需要 reload，直接返回（配置在下次 Start 时生效）。
	if m.Status(kind).State != StateRunning {
		return nil
	}
	switch kind {
	case "frpc":
		if m.reloader != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := m.reloader.Reload(ctx, true)
			cancel()
			if err == nil {
				return nil
			}
			// reload 失败 → 降级为 Restart。
			if _, rErr := m.Restart("frpc"); rErr != nil {
				return fmt.Errorf("procmgr.ApplyConfigChange(frpc): reload failed (%v); restart also failed (%w)", err, rErr)
			}
			return nil
		}
		_, err := m.Restart("frpc")
		return err
	case "frps":
		_, err := m.Restart("frps")
		return err
	}
	return nil
}

// Shutdown 优雅关闭：停止全部子进程。
func (m *Manager) Shutdown() {
	for _, kind := range []string{"frpc", "frps"} {
		_, _ = m.Stop(kind)
	}
}

// --- 内部 ---

func validateKind(kind string) error {
	if kind != "frpc" && kind != "frps" {
		return fmt.Errorf("procmgr: invalid kind %q (want frpc|frps)", kind)
	}
	return nil
}

func (m *Manager) binPathFor(kind string) (string, error) {
	switch kind {
	case "frpc":
		return m.loc.FRPCPath()
	case "frps":
		return m.loc.FRPSPath()
	}
	return "", fmt.Errorf("procmgr: invalid kind %q", kind)
}

// supervise 在 goroutine 内运行：tee 日志，监听 cmd.Wait() 结果，更新状态。
func (m *Manager) supervise(ctx context.Context, kind string, cmd *exec.Cmd,
	stdout, stderr io.ReadCloser, logPath string, doneCh chan struct{}) {

	defer close(doneCh)

	// 【T-007 AC-2 / C-5 归责边界】
	// 本处打开的是 UI 进程 tee 子进程 stdout/stderr 用的日志文件：UI 进程对该
	// 文件的"首次创建权限"负责。OpenFile 的 mode 仅在新建文件时生效（O_CREATE
	// + 文件不存在）；老版本升级遗留的 0o644 文件可借 Chmod 幂等收紧到 0o600。
	//
	// 范围边界：FRP 子进程通过 toml 的 log.to 自行打开同路径文件（O_APPEND）时，
	// 上游 FRP 内部 OpenFile 的 mode 由 FRP 自己决定，**不在本任务修补范围内**。
	// 升级路径下 UI 进程的 Chmod 仍能把已存在文件的权限收紧；FRP 后续以 append
	// 模式打开（无 O_CREATE 或文件已存在）不会重置 mode 位。
	var logFile *os.File
	if logPath != "" {
		var err error
		logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err == nil {
			_ = os.Chmod(logPath, 0o600)
			defer logFile.Close()
		}
	}

	// tee stdout / stderr
	var teeWG sync.WaitGroup
	teeWG.Add(2)
	tail := newRingBuffer(20) // 末 20 行用于 error 上报
	go func() {
		defer teeWG.Done()
		teePipe(stdout, logFile, tail)
	}()
	go func() {
		defer teeWG.Done()
		teePipe(stderr, logFile, tail)
	}()

	// 子进程退出后 cmd.Wait 返回；这是阻塞点。
	waitErr := cmd.Wait()
	teeWG.Wait()

	exitState := StateStopped
	lastErr := ""
	if waitErr != nil {
		// 非零退出 / 信号 → 区分"被我们 Stop"与"自我退出"。
		// 看当前 state：若是 Stopping → 由 Stop 推进，不覆盖。
		m.mu.Lock()
		current := m.processes[kind].info.State
		m.mu.Unlock()
		if current == StateStopping {
			// Stop 流程会负责后续状态。
			_ = ctx // suppress unused
			return
		}
		exitState = StateError
		lastErr = fmt.Sprintf("exit: %v | tail: %s", waitErr, tail.JoinTail())
	}
	// 正常 / 异常退出（非由 Stop 触发）→ 更新状态。
	m.mu.Lock()
	ps := m.processes[kind]
	ps.info.State = exitState
	ps.info.PID = 0
	ps.info.LastErr = lastErr
	ps.info.ChangedAt = time.Now().UTC()
	info := ps.info
	ps.cmd = nil
	ps.cancel = nil
	ps.doneCh = nil
	m.mu.Unlock()
	m.emit(StatusEvent{Kind: kind, Info: info})
}

// waitUntilStable 等 d 时间后看 state：仍 starting → 推到 running 并返回 nil；
// 若已变成 error / stopped → 返回错误。
func (m *Manager) waitUntilStable(kind string, d time.Duration) error {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		st := m.Status(kind).State
		if st == StateError {
			return fmt.Errorf("procmgr.Start(%s): process exited within %v", kind, d)
		}
		if st == StateStopped {
			return fmt.Errorf("procmgr.Start(%s): process disappeared", kind)
		}
		time.Sleep(100 * time.Millisecond)
	}
	// 时间到，state 应仍是 starting → 推到 running。
	m.mu.Lock()
	ps := m.processes[kind]
	if ps.info.State == StateStarting {
		ps.info.State = StateRunning
		ps.info.ChangedAt = time.Now().UTC()
		info := ps.info
		m.mu.Unlock()
		m.emit(StatusEvent{Kind: kind, Info: info})
		return nil
	}
	st := ps.info.State
	m.mu.Unlock()
	if st == StateError {
		return fmt.Errorf("procmgr.Start(%s): error state", kind)
	}
	return nil
}

// teePipe 把 r 同时复制到 logFile 与 tail ring buffer。
func teePipe(r io.ReadCloser, logFile *os.File, tail *ringBuffer) {
	if r == nil {
		return
	}
	defer r.Close()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if logFile != nil {
			_, _ = logFile.WriteString(line + "\n")
		}
		tail.Push(line)
	}
}

// ringBuffer 留末 N 条字符串，supervise 子进程异常退出时用于附加日志摘要。
type ringBuffer struct {
	mu    sync.Mutex
	buf   []string
	max   int
}

func newRingBuffer(n int) *ringBuffer {
	return &ringBuffer{max: n}
}

func (r *ringBuffer) Push(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, s)
	if len(r.buf) > r.max {
		r.buf = r.buf[len(r.buf)-r.max:]
	}
}

func (r *ringBuffer) JoinTail() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.buf, " | ")
}

// --- 跨平台占位（真正实现见 manager_windows.go / manager_unix.go） ---

// applyPlatformAttrs 由 OS 特化文件实现。
// platformKill 同上。

// 让导入 runtime 在两个平台文件之外也保留，便于未来扩展。
var _ = runtime.GOOS

// 兜底导出：让 errors 不被误删（reserve for future error wrapping）。
var _ = errors.New
