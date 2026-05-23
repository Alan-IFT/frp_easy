//go:build windows

// service_windows.go — Windows Service ABI 集成（T-019 windows-service-scm-1053-fix）。
//
// 用途：把 frp-easy.exe 在被 Windows SCM 拉起时切到服务化分支，
//       实现完整 SetServiceStatus 状态机（START_PENDING + 1s CheckPoint
//       心跳 + WaitHint=5s → RUNNING → Stop control code → STOP_PENDING
//       → 优雅关停 procmgr / HTTP / storage → STOPPED），从根因层解决
//       sc.exe start frp-easy 报错 1053（"服务没有及时响应启动或控制请求"）。
//
// 设计依据：
//   - docs/features/windows-service-scm-1053-fix/02_SOLUTION_DESIGN.md §3.1 / §3.3
//   - 03_GATE_REVIEW.md F-2（RUNNING 状态显式 CheckPoint=0/WaitHint=0 让代码意图清晰）
//   - 03 §4 Verdict C-2（run 两参签名）
//
// 与 wrapper.cmd 的对比：以前 sc.exe binPath 指向 frp-easy-svc.cmd，由 cmd 锁 cwd；
// 现在 sc.exe binPath 直接指向 frp-easy.exe，cwd 由本文件 runService() 起手
// `os.Chdir(filepath.Dir(exe))` 锁定 —— os.Executable() 原生 UTF-16，
// 对中文 / 空格 / UNC 路径（BC-3 / BC-4）天然正确，不再经 host codepage。

package main

import (
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc"
)

// isWindowsService 由 main.go 顶端调用；true 表示进程是被 SCM 拉起。
// IsWindowsService 实际签名为 (bool, error)；错误时安全降级到非服务模式
// （视为控制台运行），避免在 dev / 双击场景下走错分支。
func isWindowsService() bool {
	inService, err := svc.IsWindowsService()
	if err != nil {
		return false
	}
	return inService
}

// runService 进入 Windows Service 主循环；阻塞直到 SCM Stop 完成后返回。
//
// 内部：① 把 cwd 锁定到 frp-easy.exe 所在目录（替代旧 wrapper.cmd 的 cd /d，
// 让 main.go run() 内 frp_easy.toml 相对路径解析正确）；② 调 svc.Run 进入
// SCM 控制循环；svc.Run 内部会调度 serviceHandler.Execute。
func runService() error {
	// 锁 cwd 到 exe 所在目录。os.Executable() 在 Windows 上底层调用
	// GetModuleFileNameW（UTF-16），对中文 / 空格路径天然正确，不依赖 host codepage。
	if exe, err := os.Executable(); err == nil {
		_ = os.Chdir(filepath.Dir(exe))
	}
	return svc.Run("frp-easy", &serviceHandler{})
}

// serviceHandler 实现 golang.org/x/sys/windows/svc.Handler。
type serviceHandler struct{}

// Execute 是 SCM 主循环的入口。
//
// 状态机（02 §6 序列图）：
//   1. 立刻报 START_PENDING（CheckPoint=0, WaitHint=5s）。
//   2. 启 goroutine 跑 run(stopCh, readyCh)；同时启 1s ticker 累加 CheckPoint 心跳。
//   3. run() 在 HTTP server 启动 + autoRestoreProcs + ready.Store(true) 后
//      `close(readyCh)` 通知 Execute 切 RUNNING；Execute 报 RUNNING
//      （CheckPoint=0, WaitHint=0，符合 MSDN 语义）。
//   4. 主循环 select：收到 Stop / Shutdown 控制码 → 报 STOP_PENDING
//      （WaitHint=30s，符合 NFR-7 30s 优雅关停预算）→ close(stopCh)
//      触发 run() 内 select 命中 stopCh case → run() 走 pm.Shutdown() +
//      srv.Shutdown() 关停 → Execute 阻塞 <-runErrCh 等真正返回 → 报 STOPPED。
//   5. 任何阶段 run() 自发退出（HTTP fatal / 启动失败）也走 STOPPED 路径，
//      并按是否在 ready 之前/之后映射 Win32ExitCode（1 = 一般错误 / 0 = 正常）。
func (h *serviceHandler) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (svcSpecificEC bool, exitCode uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown

	// 1. 立刻报 START_PENDING + CheckPoint=0 + WaitHint=5s（Q5 决议）。
	s <- svc.Status{State: svc.StartPending, CheckPoint: 0, WaitHint: 5000}

	// 2. 启动 run() goroutine + 心跳 ticker。
	stopCh := make(chan struct{})
	readyCh := make(chan struct{})
	runErrCh := make(chan error, 1)

	go func() {
		runErrCh <- run(stopCh, readyCh)
	}()

	cp := uint32(0)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// 3. 心跳循环：直到 readyCh 关闭 或 run() 在 ready 前退出。
HEARTBEAT:
	for {
		select {
		case <-readyCh:
			break HEARTBEAT
		case err := <-runErrCh:
			// run() 在 ready 之前就返回 → 启动失败。
			s <- svc.Status{State: svc.Stopped}
			if err != nil {
				return false, 1
			}
			return false, 0
		case <-ticker.C:
			cp++
			s <- svc.Status{State: svc.StartPending, CheckPoint: cp, WaitHint: 5000}
		}
	}

	// 4. 报 RUNNING（F-2：显式 CheckPoint=0/WaitHint=0 让代码意图清晰）。
	s <- svc.Status{State: svc.Running, Accepts: accepted, CheckPoint: 0, WaitHint: 0}

	// 5. 主循环：等 SCM 控制信号 / run() 退出。
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				// SCM 主动询问当前状态；直接回显。
				s <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				// NFR-7：30s 预算包含 srv.Shutdown(10s) + procmgr 最坏 24s 中的一部分；
				// 实际 stopCh 一发出 run() 即开始关停，30s WaitHint 让 SCM 等待。
				s <- svc.Status{State: svc.StopPending, WaitHint: 30000}
				close(stopCh)
				<-runErrCh // 等 run() 真正退出（pm.Shutdown + srv.Shutdown 完成）。
				s <- svc.Status{State: svc.Stopped}
				return false, 0
			default:
				// 未知控制码忽略（svc 包对 0 / 其它值的处理）。
			}
		case err := <-runErrCh:
			// run() 在 RUNNING 之后自发退出（如 srv.Serve 返回 ErrServerClosed 之外的 fatal）。
			s <- svc.Status{State: svc.Stopped}
			if err != nil {
				return false, 1
			}
			return false, 0
		}
	}
}
