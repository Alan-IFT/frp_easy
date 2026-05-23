# 04 — Development Record · T-019 windows-service-scm-1053-fix

> Harness 流水线 Stage 4（Developer）。模式：**full**。
> 上游（READ-ONLY）：`01_REQUIREMENT_ANALYSIS.md`（READY）、`02_SOLUTION_DESIGN.md`（READY）、`03_GATE_REVIEW.md`（APPROVED WITH CONDITIONS）。
> 本文档**只描述实施记录**——不重写设计、不评审。Code Reviewer / QA Tester 看本文找"动了哪几行 / 为什么 / 验证证据"。

---

## Summary

按 02 §3 + §11 dispatch order 全部 9 步实施完毕，frp-easy.exe 升级为"双入口单二进制"：被 Windows SCM 拉起时 `main()` 顶端 `isWindowsService()` 检测成功后调 `runService()`，进入 `svc.Run` + `serviceHandler.Execute` 实现完整 `SetServiceStatus` 状态机（START_PENDING + 1s CheckPoint 心跳 → close(readyCh) 切 RUNNING → SCM Stop 控制码触发 close(stopCh) 走优雅关停链路 → STOPPED），从根因层消解 sc.exe start 1053 报错；wrapper.cmd（`frp-easy-svc.cmd`）整条中间壳从 `install-service.ps1` 移除（cwd 由 frp-easy.exe 内 `os.Chdir(filepath.Dir(os.Executable()))` 锁定），并在 install / uninstall 两端追加防御性清理；install-service.ps1 在 sc.exe start 后新增 `Wait-ServiceRunning` 30s 轮询（每 3 秒中文进度行）+ Stop 后 `Wait-ServiceMarkedDeleteCleared` 15s 防卡死；`go.mod` 把 `golang.org/x/sys v0.44.0` 从 indirect 块手工搬到 direct 块（**不**跑 go mod tidy，详见下方 §实施步骤 C-1 注）。verify_all PASS:19。

---

## Files changed

### 新增（3 个）

- `cmd/frp-easy/service_windows.go`（新增，`//go:build windows`）——
  - `isWindowsService() bool`：包装 `svc.IsWindowsService()`，error 时安全降级为 false。
  - `runService() error`：`os.Executable()` → `os.Chdir(filepath.Dir(exe))` → `svc.Run("frp-easy", &serviceHandler{})`。
  - `serviceHandler.Execute`：完整 SCM 状态机（详见 02 §3.1 + §6 序列图）。RUNNING 状态按 F-2 显式写 CheckPoint=0 / WaitHint=0（不是依赖 Go 零值）。
- `cmd/frp-easy/service_other.go`（新增，`//go:build !windows`）——
  - `isWindowsService() bool { return false }`
  - `runService() error { return errors.New("service mode not supported on this platform") }`（按 03 Q-D5 直接用 errors.New 而非自定义 stringError type，micro-nit 决策）。
- `cmd/frp-easy/service_windows_test.go`（新增，`//go:build windows`）——两个文本契约单测：
  - `TestInstallServiceScriptNoWrapperGen`：grep `install-service.ps1` 不含 `Set-Content -Path $WrapperPath` 也不含 `$WrapperContent = @"`，并且 sc.exe binPath 已改指 `$BinaryPath`。
  - `TestUninstallStillCleansWrapper`：grep `uninstall-service.ps1` 仍含 `frp-easy-svc.cmd` + `Remove-Item -Force $WrapperPath` 防御性清理。

### 编辑（5 个）

- `cmd/frp-easy/main.go`
  - `main()` 顶端：追加 `if isWindowsService() { if err := runService(); err != nil { ...; os.Exit(1) }; return }` 分流。
  - `run()` 签名由 `func run() error` 改为 `func run(stopCh <-chan struct{}, readyCh chan<- struct{}) error`（按 C-2 两参签名，忽略 02 §3.1 sample 中 `run(stopCh)` 一参笔误）。
  - `ready.Store(true)` 之后追加 `if readyCh != nil { close(readyCh) }`。
  - signal select 追加 `case <-stopCh: logger.Info("stopCh received, shutting down")`（与现有 `case s := <-sigCh:` / `case e := <-serveErr:` 三路并存；stopCh==nil 时 Go select 对 nil chan 的 case 永久阻塞 = 该分支不存在，控制台分支行为零变化）。
  - `main()` 的 `run()` 调用改为 `run(nil, nil)`。
- `go.mod`
  - `golang.org/x/sys v0.44.0` 从 `require ( ... // indirect )` 块手工搬到上方 direct `require` 块（按字母序插在 `golang.org/x/crypto` 与 `golang.org/x/term` 之间）。
  - 版本号不变；`go.sum` 不动。
  - **未跑 `go mod tidy`**（详见 §实施步骤 C-1）。
- `scripts/install-service.ps1`
  - 头注释从"为锁定 cwd，安装时同时生成 frp-easy-svc.cmd"改为"T-019 起 sc.exe binPath= 直接指向 frp-easy.exe，进程内 Windows Service ABI 锁定 cwd"，并标注 BC-3/BC-4 由 os.Executable() UTF-16 保证正确。
  - 删除旧 L68-82 整段 wrapper.cmd 生成块（`$WrapperPath` 定义 + here-string + `Set-Content -Encoding Default`）。
  - 在 `$BinaryPath` 解析后追加防御性清理：若 `frp-easy-svc.cmd` 残留则 `Remove-Item -Force ... -ErrorAction SilentlyContinue` + 删除成功打印"==> 已清理旧版包装脚本残留"。
  - 新增 `Wait-ServiceMarkedDeleteCleared -Name -TimeoutSec` 函数（15s 默认）：轮询 sc.exe query，sc 返回非 0（服务消失）或输出不含 "marked for delete" 即返回 true，超时返回 false。在 `Wait-ServiceStopped` 之后立刻调用，超时给中文诊断退出 2（Q8）。
  - 新增 `Wait-ServiceRunning -Name -TimeoutSec` 函数（30s 默认）：每 500ms 轮询 sc.exe query，命中 `STATE : 4 RUNNING` 即返回 true；超时返回 false；每 3 秒打印一行 "==> 等待服务进入 RUNNING 状态..." 中文进度（AC-19）。在 sc.exe start 后立刻调用，超时给中文诊断 + 排障路径退出 2（Q9）。
  - `sc.exe create` / `sc.exe config` 的 `binPath=` 改为 `"`"$BinaryPath`""`（双反引号 + 内层双引号 PowerShell 转义），与旧 `$WrapperPath` 同款引号语义保 BC-4 空格路径兼容。
  - 末尾说明从"服务 binPath 与包装脚本均使用绝对路径"改为"服务 binPath 直接指向 $BinaryPath"。
- `scripts/uninstall-service.ps1`
  - 头注释段落从"清理 install-service.ps1 生成的 frp-easy-svc.cmd 包装脚本"降级为"对旧版（< T-019）install-service.ps1 曾生成的 frp-easy-svc.cmd 包装脚本做防御性清理"，并说明 T-019 起 install-service.ps1 不再生成 wrapper.cmd（cwd 由 frp-easy.exe 内 os.Chdir 锁定）。
  - 末段清理代码块注释从"清理由 install-service.ps1 生成的 cwd 包装脚本"改为"防御性清理：旧版（< T-019）install-service.ps1 曾生成 frp-easy-svc.cmd 包装脚本……";Write-Host 文案从"已删除服务包装脚本"改为"已清理旧版包装脚本残留"。
  - **逻辑不动**（Remove-Item -Force $WrapperPath -ErrorAction SilentlyContinue 这一行字符级不变，单测 `TestUninstallStillCleansWrapper` grep 模式保持命中）。
- `docs/dev-map.md`
  - cmd/frp-easy/ 行追加 T-019 三个新文件说明（service_windows.go / service_other.go / service_windows_test.go）。
  - "功能在哪里"表格"程序入口"行追加 T-019 main() 分流 + run() 双参签名说明。
  - 新增一行"Windows Service ABI | cmd/frp-easy/service_windows.go / service_other.go | ...（状态机摘要）"。

### 未改文件（重要锚定，与 02 §2.3 对齐）

- `internal/procmgr/manager.go` —— graceful Shutdown 链路复用（Q7 决议），零字节改动。
- `internal/appconf/`、`internal/storage/`、`internal/httpapi/`、`internal/logrotate/`、`internal/browseropen/` —— 全部零字节改动。
- `scripts/install.sh`、`install-service.sh`、`uninstall-service.sh` —— Linux 路径零字节改动（OOS-2）。
- `scripts/install.ps1` —— 现状已不含 "1053" 字样（grep 确认 0 命中），步骤 7 透传链路（`$LASTEXITCODE = 0; & $svc; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }`）保持原样依赖 install-service.ps1 行为变更间接生效。**零字节改动**。
- `scripts/verify_all.{ps1,sh}` —— NFR-5 检查项数量不变，零字节改动。

---

## 实施步骤（含 Gate Review 强制条件落地）

按 02 §11 dispatch order 顺序执行：

1. **go.mod 升级 x/sys 为 direct（C-1 强制落地）**：
   - 用 Edit 手工把 `golang.org/x/sys v0.44.0 // indirect` 从下半 indirect 块删除，并按字母序插入到上方 direct require 块（在 `golang.org/x/crypto v0.51.0` 与 `golang.org/x/term v0.43.0` 之间）。
   - **C-1 关键纪律**：**未跑 `go mod tidy`**。原因：tidy 在 Linux / 当前 OS（GOOS=windows）若不同将基于 build tag 后的 import graph 做 module graph pruning，Linux 跑 tidy 会因 service_other.go 不引用 x/sys/windows/svc 而把 x/sys 退回 indirect，回退本步改动。正确做法：① 改完 go.mod 后**不要**在 Linux 跑 `go mod tidy`；② 若必须 tidy，用 `GOOS=windows go mod tidy` 保持 x/sys 在 direct；③ 用 `go build ./cmd/frp-easy` 双平台编译验证而非 tidy。本任务采用手工 Edit + 双平台编译验证路径，全程无 tidy 调用。

2. **service_other.go（先建非 Windows stub）**：写 `isWindowsService() bool { return false }` + `runService() error { return errors.New("service mode not supported on this platform") }`。按 03 §3 Q-D5 用 errors.New 而非 stringError 自定义 type（micro-nit，errors 已在 main.go import）。

3. **service_windows.go（C-2 强制落地）**：
   - `isWindowsService()`：包装 `svc.IsWindowsService()`（实测签名 `(bool, error)`），error 时返回 false 安全降级。
   - `runService()`：`os.Executable()` → `os.Chdir(filepath.Dir(exe))` → `svc.Run("frp-easy", &serviceHandler{})`。`os.Executable()` 在 Windows 上底层调 GetModuleFileNameW（UTF-16），对 BC-3（中文路径）/ BC-4（空格路径）天然正确，不再依赖 wrapper.cmd 的 host codepage（insight L17 在新设计层面被消解）。
   - `serviceHandler.Execute`：
     - 立刻 `s <- svc.Status{State: svc.StartPending, CheckPoint: 0, WaitHint: 5000}`。
     - `go func() { runErrCh <- run(stopCh, readyCh) }()` —— **C-2 落地**：按 02 §3.1 注末 + §3.3 的两参签名调用 `run(stopCh, readyCh)`，忽略 02 §3.1 sample 中 `run(stopCh)` 一参笔误。
     - 1s ticker 累加 CheckPoint 心跳，直到 readyCh close 跳出 HEARTBEAT 循环；ready 之前 runErrCh 返回 = 启动失败，报 STOPPED + errno=1。
     - 切 RUNNING：**F-2 落地**——显式写 `svc.Status{State: svc.Running, Accepts: accepted, CheckPoint: 0, WaitHint: 0}`，不依赖 Go 零值。
     - 主 select 循环：Interrogate 回显 / Stop / Shutdown → 报 STOP_PENDING (WaitHint=30000) → close(stopCh) → 等 `<-runErrCh` → 报 STOPPED + errno=0。
     - run() 在 RUNNING 之后自发退出（HTTP fatal 等）也走 STOPPED；err != nil 时 errno=1。

4. **main.go 编辑**：
   - `main()` 顶端追加 isWindowsService() 分流 → runService()（错误时 fmt.Fprintln(os.Stderr, err) + os.Exit(1)）→ return。
   - `run()` 签名改为 `func run(stopCh <-chan struct{}, readyCh chan<- struct{}) error`，函数注释完整描述两参语义与控制台分支退化逻辑。
   - `ready.Store(true)` + `logger.Info("ready gate opened")` 之后追加 `if readyCh != nil { close(readyCh) }`。
   - signal select 追加 `case <-stopCh: logger.Info("stopCh received, shutting down")` 第三路（Q-D1 nil chan 永久阻塞 = 该分支不存在）。
   - `main()` 的 `run()` 调用改为 `run(nil, nil)`。

5. **service_windows_test.go**：写两个单测，readRepoFile helper 用 `filepath.Join("..", "..", ...)` 从 cmd/frp-easy/ 包目录拼 repo 根。覆盖 install-service.ps1 黑名单 2 项（Set-Content / here-string）+ 白名单 1 项（binPath 改指 BinaryPath）+ uninstall-service.ps1 白名单 2 项（文件名提及 + Remove-Item 动作）。

6. **install-service.ps1 重写**：详见上方"Files changed"段。

7. **uninstall-service.ps1 注释降级**：详见上方。

8. **install.ps1 检查**：`Grep 1053 scripts/install.ps1` 0 命中，确认现状已不含 1053 字样，关键实施要点 #7 表述确认，零字节改动。

9. **docs/dev-map.md 更新**：cmd/frp-easy/ 行 + "功能在哪里"两行。

10. **F-5 自检**：Read `internal/browseropen/browseropen.go` L50-66 `ShouldOpen`：第 57 行 `if !isTerminalFunc(int(stdinFd())) { return false }` —— 服务模式下 Stdin 不绑定到 TTY → IsTerminal 返回 false → ShouldOpen 返回 false → 不打开浏览器。F-5 确认无需在 service_windows.go 显式禁用 browseropen，无 DESIGN DRIFT。

11. **双平台编译验证**：
    - `go build ./cmd/frp-easy`（GOOS=windows，默认）→ exit 0。
    - `GOOS=linux go build -o NUL ./cmd/frp-easy` → exit 0（验证 service_other.go 让 Linux 编译不引用 svc 包；R-3 缓解成立）。

12. **`go test ./cmd/frp-easy`**：两个新增单测均 PASS。

13. **verify_all.ps1 完整跑**：PASS:19 / WARN:0 / FAIL:0 / SKIP:0。

---

## verify_all result

- **Baseline**（实施前；同一仓库快照，T-019 改动未起步前跑 `pwsh scripts\verify_all.ps1`）：
  - 第一次跑：PASS:18 / WARN:0 / FAIL:1 / SKIP:0，G.2 因上次 playwright 残留的 `web/test-results/` 目录让 `go list ./...` 输出 stderr warning（与本任务无关的偶发噪声）。
  - 第二次跑（同次会话内）：**PASS:19 / WARN:0 / FAIL:0 / SKIP:0**（与 T-018 main HEAD 一致；首次 FAIL 为 stderr 噪声偶发）。
- **After changes**（实施完毕、verify_all 完整跑）：**PASS:19 / WARN:0 / FAIL:0 / SKIP:0**。
- **Delta**：0 new failures（无新引入失败）；新增 2 个 go test 用例（`TestInstallServiceScriptNoWrapperGen` + `TestUninstallStillCleansWrapper`），测试基线只升不降（红线 #3 满足）。
- **跑的版本**：在主开发环境 Windows 11 / PowerShell 7 跑的 `scripts\verify_all.ps1`（非 bash 版）。

---

## Design drift (if any)

**无 DESIGN DRIFT**。

实施全程按 02 + 03 决议落地：
- C-1（go.mod 操作纪律）：go mod tidy 全程未调用，手工 Edit + 双平台编译验证。
- C-2（run 两参签名）：按 02 §3.1 注末 + §3.3 落地，忽略 §3.1 sample `run(stopCh)` 一参笔误。
- F-1（同 C-2）：已落地。
- F-2（RUNNING Status 显式 CheckPoint=0/WaitHint=0）：已落地，service_windows.go 第 121 行 `svc.Status{State: svc.Running, Accepts: accepted, CheckPoint: 0, WaitHint: 0}`。
- F-5（browseropen 服务模式 isatty 自然返回 false）：Read `internal/browseropen/browseropen.go` 第 57 行确认 `ShouldOpen` 依赖 `isTerminalFunc(int(stdinFd()))`，服务模式无 TTY → 返回 false → 不开浏览器。无需在 service_windows.go 显式禁用，无 DESIGN DRIFT。
- F-7（go.mod 操作纪律）：同 C-1。

---

## Open issues for review

以下为本任务实施过程中观察到但**不在本任务 in-scope** 的项，仅供 Code Reviewer / QA Tester / 后续任务参考：

1. **F-6（exposureNotice stderr 服务模式丢失）**：main.go L138-140 在 `UIBindAddr == "0.0.0.0"` 时走 `fmt.Fprint(os.Stderr, exposureNotice(...))`，服务模式下 stderr 被 SCM 丢弃且 lumberjack 不重定向标准流——安全提示在 Windows 服务模式下**实际会丢失**。01 §7 T-011 行描述"日志（lumberjack 写入 .frp_easy\logs\ui.log）保留该行"不准确。本任务**不修复**（03 §4 C-5 已建议留作 follow-up T-020 service-mode-stderr-bridge）。
2. **F-4 体积断言**：QA 报告里建议追加体积对比 AC（`frp-easy.exe` 增长 ≤ 1 MB）。本任务编译产物大小不在本步实测；可由 QA 在 06 跑 `ls -l` 对比 T-018 release。
3. **AC-15 4 发行版回归**：03 §2.7 F-3 已指出 T-017 QA 实际仅 Ubuntu 单台 VM 跑过，4 发行版同款是 RA 误认。本任务对 Linux 路径零字节改动（OOS-2），AC-15 由 PM 在 06 / 07 决议降级。
4. **verify_all G.2 偶发噪声**：基线第一次跑因 `web/test-results/` playwright 残留让 PowerShell 的 `go list ./...` 输出 stderr warning 行触发 G.2 FAIL；重跑稳定。属 verify_all 脚本偶发噪声而非本任务引入。

---

## Dev-map updates

新增 / 修改的行（详见 `docs/dev-map.md` 实际 diff）：

- 目录布局 `cmd/frp-easy/` 行追加 3 行 T-019 说明（service_windows.go / service_other.go / service_windows_test.go）。
- "功能在哪里"表格"程序入口"行追加 T-019 main() 分流 + run() 双参签名说明。
- "功能在哪里"表格新增一行 "Windows Service ABI | cmd/frp-easy/service_windows.go / service_other.go | (Execute 状态机摘要)"。

---

## Insight to surface

- T-019 实施验证 · `os.Executable()` + `os.Chdir(filepath.Dir(exe))` 在 Windows 服务模式锁 cwd 是 wrapper.cmd 的零成本替代品：Go stdlib 底层调 GetModuleFileNameW（UTF-16）天然兼容中文 / 空格 / UNC 路径，不依赖 host codepage / GB18030 —— 反向消解了 insight L17（Set-Content -Encoding Default 写 wrapper.cmd 中文路径乱码）的全部根因。后续任何在 Go 服务化中需要锁 cwd 的场景应优先选此 idiom 而非 .cmd 中间壳 · evidence: `cmd/frp-easy/service_windows.go` L46-49 + 02 §8 R-4。

---

## Verdict

**READY FOR REVIEW**
