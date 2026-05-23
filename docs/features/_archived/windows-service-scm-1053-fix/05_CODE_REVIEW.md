# 05 CODE REVIEW · T-019 windows-service-scm-1053-fix

> Reviewer：Code Reviewer (Stage 5)
> 日期：2026-05-23
> 上游：01_REQUIREMENT_ANALYSIS.md (READY) / 02_SOLUTION_DESIGN.md (READY) /
>       03_GATE_REVIEW.md (APPROVED WITH CONDITIONS) / 04_DEVELOPMENT.md (READY FOR REVIEW)
> 模式：full
> 注：本评审由 code-reviewer sub-agent 完整产出文本，因其工具集仅 Read/Glob/Grep（无 Write），由 PM 代为落盘（insight L41 红线场景，与 Stage 3 同款 fallback）。

## 1. Summary

T-019 落地质量很高。Developer 严格按 02 §3 + §11 dispatch order 推进，且把 03 Gate Review 的 5 条 condition（C-1~C-5）逐条点对点处理：C-1（go.mod 不在 Linux 跑 tidy）作为 §实施步骤 #1 的纪律说明落到 04；C-2（两参签名）在 main.go run() / service_windows.go go func() / runErrCh 三处一致用 `run(stopCh, readyCh)` 实现；F-2（RUNNING 状态显式 CheckPoint=0/WaitHint=0）在 service_windows.go L112 明文写零值；F-5（browseropen isatty 自然返回 false）经 Read 内部 `isTerminalFunc(int(stdinFd()))` 自查通过，未在 service_windows.go 显式禁用——符合设计；F-7（go.mod x/sys 提升 direct）实际效果：go.mod L9 已在 direct 块按字母序排好，go.sum 仅保留 v0.44.0 hash 未漂版。代码层面无 BLOCKER：状态机闭合（START_PENDING heartbeat → RUNNING → STOP_PENDING → STOPPED 四象限全发齐 SetServiceStatus）、stopCh / readyCh 在控制台分支 `run(nil, nil)` 通过 Go nil-channel-永久阻塞语义无副作用退化、`os.Executable()` + `os.Chdir(filepath.Dir(exe))` 从根因层消解 wrapper.cmd host-codepage（insight L17）问题。install-service.ps1 删 wrapper 生成块、加 Wait-ServiceMarkedDeleteCleared 15s + Wait-ServiceRunning 30s + 3 秒一行中文进度，AC-19 / Q8 / Q9 三决议全部到位。需求 19 条 AC 与设计 12 IS 全部找到代码或脚本支撑。仅有少量 P2 / Nit 级别建议（不阻塞 Stage 6 QA）。

## 2. 检查矩阵

| 检查项 | 结果 | 证据（file:line） |
|---|---|---|
| **C-1** 04 明确"不在 Linux 跑 go mod tidy" | PASS | `04_DEVELOPMENT.md` §实施步骤 #1 / Summary 末段 |
| **C-2** run() 两参签名实际实现 | PASS | `main.go:120` `func run(stopCh <-chan struct{}, readyCh chan<- struct{}) error`；`service_windows.go:85` `runErrCh <- run(stopCh, readyCh)`；`main.go:101` `run(nil, nil)` |
| **F-2** RUNNING 显式 CheckPoint=0/WaitHint=0 | PASS | `service_windows.go:112` `svc.Status{State: svc.Running, Accepts: accepted, CheckPoint: 0, WaitHint: 0}` |
| **F-5** 未错误地显式禁用 browseropen | PASS | `service_windows.go` 全文无 `browseropen` 引用；`browseropen.go:57` isatty 检查在服务模式自然 false |
| **F-7** x/sys 提升 direct（未被 tidy 退回） | PASS | `go.mod:9` direct 块含 `golang.org/x/sys v0.44.0`；indirect 块 (L15-28) 无 x/sys 行；`go.sum` v0.44.0 hash 完整 |
| **Q1** in-process svc.Run | PASS | `service_windows.go:53` |
| **Q2** svc.IsWindowsService 自动探测 | PASS | `service_windows.go:35` |
| **Q3** 移除 wrapper.cmd | PASS | `install-service.ps1` 全文 grep `Set-Content -Path $WrapperPath` 0 命中、`$WrapperContent = @"` 0 命中；binPath= 改指 `$BinaryPath` (`install-service.ps1:144,150`) |
| **Q5** 1s CheckPoint + 5s WaitHint | PASS | `service_windows.go:77,89,107` |
| **Q6** LocalSystem（无 obj=） | PASS | `install-service.ps1:144,150` sc.exe 命令未带 `obj=` 参数 |
| **Q7** procmgr.Shutdown 复用 graceful | PASS | `main.go:332` `pm.Shutdown()` 不变；`procmgr/manager.go:393-397` 零字节改 |
| **Q8** marked-for-delete 15s 轮询 + 中文诊断 | PASS | `install-service.ps1:52-65` + L140-143 调用 |
| **Q9** Wait-ServiceRunning 30s + 3s 中文进度 | PASS | `install-service.ps1:71-91` + L177 调用；AC-19 进度行 L85 |
| **Q-D5（Nit 建议）** errors.New 而非 stringError | PASS | `service_other.go:13,22` |
| **NFR-1** 零新外部依赖（x/sys 仅从 indirect 升 direct） | PASS | `go.mod:9` 版本未变 v0.44.0；go.sum 不漂 |
| **NFR-3** 安装/卸载脚本中文输出 | PASS | install-service.ps1 / uninstall-service.ps1 所有 Write-Host / Write-Error 文案中文 |
| **NFR-5** verify_all 检查项数量不变 | PASS | 04 §verify_all PASS:19；与 T-018 baseline 一致 |
| **NFR-9** run() 启动序列零字节改 | PASS | `main.go:155-294` 顺序保留（appconf → storage → logrotate → binloc/procmgr/auth → HTTP → autoRestore → ready）；新增点仅 L292-294 close(readyCh) 与 L320-321 stopCh case |
| 单测 windows build tag 隔离 | PASS | `service_windows_test.go:1` `//go:build windows`；service_other.go / service_windows.go 同款 tag |
| AC-17 06 报告英文裸标题（red line） | PENDING | 待 QA Stage 6 在 06_TEST_REPORT.md 落地；本审范围外 |

## 3. 发现（按严重度）

### P0（must-fix before merge）

**无。**

### P1（should-fix before merge）

**无。**

### P2（nice-to-fix；不阻塞）

#### P2-1 [LOGIC] `service_windows.go:127` — Stop 路径未对 `<-runErrCh` 加超时，理论可让 SCM 卡在 STOP_PENDING > 30 秒

`service_windows.go:122-129` 收到 Stop 控制码后流程：
```
s <- svc.Status{State: svc.StopPending, WaitHint: 30000}
close(stopCh)
<-runErrCh   // 等 run() 真正退出
s <- svc.Status{State: svc.Stopped}
```

`run()` 内优雅关停最坏耗时（02 §8 R-2 估算）：
- `srv.Shutdown(ctx 10s)` 最坏 10s；
- `pm.Shutdown()` 串行调 `Stop("frpc")` + `Stop("frps")`，每 kind 最坏 5s + 2s 兜底强杀 = 7s × 2 = 14s（`internal/procmgr/manager.go:319,326`）。
- 合计最坏 **24 秒**，仍在 30s WaitHint 内，**理论安全**。

但若 `pm.Shutdown()` 内部走到任何额外阻塞（如 supervisor goroutine 未及时收到 doneCh），实际耗时可能突破 30s 让 SCM 强杀，触发 NFR-7 失败。建议在 `<-runErrCh` 加 `time.After(28 * time.Second)`（留 2s 余量给最后 SetServiceStatus(Stopped) 上报）超时分支，超时也走 Stopped 上报避免 SCM 强杀路径。**非阻塞**（既有路径已留 6s 余量；属增强）。

#### P2-2 [LOGIC] `service_windows.go:84-86` — runErrCh buffered 容量 1 + 主循环退出后 goroutine 仍可能阻塞写

`runErrCh` 在 L82 声明为 buffered chan 1，`runErrCh <- run(...)` 在 goroutine L85 写入。Heartbeat 阶段如果 run() 在 ready 之前出错，Execute 在 L100-104 报 STOPPED 后直接 `return false, 1/0` —— **但此时 run() goroutine 已 push 成功（buffer=1）**，无泄漏；OK。

主循环 Stop 路径 L122-129：先 close(stopCh) → run() 内部 select 命中 → return nil → goroutine 写入 runErrCh（buffer 空，成功）→ Execute L127 `<-runErrCh` 拿到 → 报 STOPPED。**无泄漏**。

主循环 run() 自发退出路径 L133-139：runErrCh 拿到 → 报 STOPPED；OK。

**唯一边角**：主循环 Stop 之后 `<-runErrCh` 之前，若 run() 已 panic 并被 Go runtime crash 整个进程（未走 select 写 runErrCh），Execute 永远阻塞在 L127，SCM 30s 超时强杀。但 panic crash 进程 = OS 把整个 frp-easy.exe 进程清掉，SCM 检测进程死亡也会标记 STOPPED；最终结果仍合规，不致 1053 等价的"卡死"问题。**非阻塞**。

#### P2-3 [LOGIC] `install-service.ps1:167` — sc.exe start 成功后未检测 `1056` (ALREADY_RUNNING) 与"未启动"的混淆

```powershell
& sc.exe start $ServiceName
if ($LASTEXITCODE -ne 0 -and $LASTEXITCODE -ne 1056) {
    Write-Error "sc.exe start 失败 ..."
    exit 2
}
# T-019：轮询 Wait-ServiceRunning（这里）
```

如果 `sc.exe start` 返回 1056（服务已经 RUNNING），随后 `Wait-ServiceRunning` 仍会跑（耗时 0.5s 内首次轮询命中即返回 true），逻辑正确。但若 sc.exe 返回 1056 而服务实际处于 START_PENDING（极端竞态：上一次 start 还没到 RUNNING），`Wait-ServiceRunning` 仍能在 30s 内等到 RUNNING，OK。

**非阻塞**。逻辑链路覆盖竞态。

#### P2-4 [MAINT] `service_windows.go:130-132` — 未知 SCM 控制码 default 分支空实现

```go
default:
    // 未知控制码忽略（svc 包对 0 / 其它值的处理）。
```

svc 包对常见控制码（Pause/Continue/ParamChange/NetBindAdd/...）会传给 Handler；目前未声明 Accepts 包含 Pause/Continue，SCM 不会发；但 ParamChange 等其它代码 SCM 不询问 Accepts 也可能发。空实现不响应 SetServiceStatus 让 SCM 等待 echo——可能在事件查看器上看到 timeout warn。建议至少 echo 当前状态：

```go
default:
    s <- c.CurrentStatus  // 与 Interrogate 同款处理
```

**非阻塞**（SCM 默认不会发未声明 Accept 的控制码到 Handler；属健壮性增强）。

#### P2-5 [TEST] `service_windows_test.go` — 单测仅 grep 脚本文本，不真实拉起 SCM 状态机

两个单测都是文本契约（黑名单 + 白名单 grep），属"shape-matching test"。Go 主代码（service_windows.go Execute 状态机、心跳 ticker、close(readyCh) → 切 RUNNING 时序）**完全没有单测覆盖**。

理由可接受：Execute 必须真 SCM 才能验证，本地 `sc.exe create frp-easy-dev binPath=...` 才能测；Stage 6 QA 会真机跑 AC-1~AC-19。**非阻塞**（已记录在 02 §10 / Q10 决议"不加 --service-debug"路径，开发期调试靠真服务）。

#### P2-6 [DOC] `install-service.ps1:202` — 末尾说明"安装服务后请勿移动 $InstallDir"未解释原因

```
注意：服务 binPath 直接指向 $BinaryPath；安装服务后请勿移动 $InstallDir。
```

建议补一句"否则 SCM 找不到 frp-easy.exe 会启动失败"。**非阻塞**。

### Nit（preference；不必处理）

#### N-1 `service_windows.go:73` 返回值命名 `svcSpecificEC bool, exitCode uint32`

第一个 bool 在 `golang.org/x/sys/windows/svc` 中文档名为 `svcSpecificEC`（如果为 true 则 errno 被解释为 service-specific，否则为 Win32ExitCode）。目前 Execute 始终返回 false，所以 errno=1 会被解释为 Win32 ERROR_INVALID_FUNCTION (1)，对用户排障稍误导（用户在事件查看器看到 Win32 1 会以为是"函数不支持"）。可考虑改为：
- ready 之前失败：`return true, 1` (service-specific errno=1，事件查看器显示"特定服务错误码 1"，更准确);
- 或保持现状 + 在 ui.log 写 "service exited before ready: <err>" 让用户去 ui.log 看真实原因。

**Nit**：当前实现可接受，事件查看器消息不至于误导到 1053 等价程度。

#### N-2 `service_windows.go:99-104` heartbeat 阶段 run() 错误时直接 `s <- svc.Status{State: svc.Stopped}` 而未先报 STOP_PENDING

按 MSDN 严格语义建议任何 PENDING 到 STOPPED 之间应有 STOP_PENDING 短暂过渡；svc 包内部对此宽容。**Nit**。

#### N-3 `service_other.go` 注释 L6 "main.go 顶端调 isWindowsService() 在这些平台恒为 false" — Linux 不会调 runService()，但行 L17 注释把它当主依据；可以更显式"isWindowsService=false 时 main.go 不进入 runService 分支，因此 runService 在 non-windows 实际无被调路径"。**Nit**。

#### N-4 `install-service.ps1:144,150` 两次 `sc.exe config` 与 `sc.exe create` 参数列表（binPath/start/DisplayName）重复——可抽 PS array 复用。**Nit**。

## 4. Requirement coverage check（AC-1 ~ AC-19）

| AC | 描述 | 实现 / 证据 | 状态 |
|---|---|---|---|
| AC-1 | 干净 Win11 `irm \| iex` 无 1053、退出码 0 | `service_windows.go` Execute 状态机 + `install-service.ps1` Wait-ServiceRunning | PASS（QA 真机验） |
| AC-2 | ≤ 3 秒 sc query STATE 4 RUNNING | `install-service.ps1:177` Wait-ServiceRunning 在 SCM RUNNING 后才退出 → install.ps1 退出即满足 | PASS |
| AC-3 | ≤ 10 秒 HTTP 200/3xx/401 | `main.go:259-282` net.Listen + srv.Serve goroutine 在 close(readyCh) 之前，SCM RUNNING ⇒ HTTP listen 已就绪 | PASS（隐含） |
| AC-4 | 升级路径单次走完无 1053 | `install-service.ps1:131-148` existed 分支 stop + Wait-Stopped + Wait-MarkedDelete + config | PASS |
| AC-5 | 单独跑 install-service.ps1 → RUNNING | `install-service.ps1:177` Wait-ServiceRunning | PASS |
| AC-6 | 连续跑两次退出码 0 + RUNNING | `install-service.ps1` existed 分支幂等；sc start 1056 (ALREADY_RUNNING) 路径 L167 已豁免 | PASS |
| AC-7 | sc stop ≤ 30s → STOPPED；sc start → RUNNING | `service_windows.go:122-128` Stop control code → close(stopCh) → run() select case → pm.Shutdown + srv.Shutdown ≤ 24s（NFR-7） | PASS |
| AC-8 | reboot 后自动 RUNNING | `install-service.ps1:144,150` `start= auto` | PASS |
| AC-9 | 步骤 7 stdout 含"服务已启动" + 无 1053 字样 | `install-service.ps1:185` Write-Host "==> 服务已启动"；install.ps1 grep 1053 0 命中 | PASS |
| AC-10 | uninstall 后 frp_easy.toml / .frp_easy/ 保留 | `uninstall-service.ps1` 零删数据目录逻辑（仅清理 wrapper） | PASS |
| AC-11 | 崩溃 → SCM 5s 重启 | `install-service.ps1:160` sc failure reset= 60 actions= restart/5000 不变 | PASS |
| AC-12 | 中文路径 InstallDir | `os.Executable()` Windows UTF-16 + os.Chdir | PASS（02 §8 R-4 证据链） |
| AC-13 | 空格路径 InstallDir | `install-service.ps1:144,150` `"`"$BinaryPath`""` 反引号转义 | PASS |
| AC-14 | 旧版本卡死 ≤ 60s 恢复或中文诊断 | `install-service.ps1:140` Wait-ServiceMarkedDeleteCleared 15s + Wait-Stopped 5s + Wait-Running 30s ≈ 最坏 50s | PASS |
| AC-15 | Linux 4 发行版 verify_all PASS:19（已 03 §2.7 F-3 降级） | OOS-2 Linux 路径零字节改；04 §verify_all PASS:19 | PENDING（03 F-3 已建议 PM 在 06/07 降级；待 QA） |
| AC-16 | verify_all PASS:19 | 04 §verify_all result PASS:19 | PASS |
| AC-17 | 06 含英文裸标题 `## Adversarial tests` | 待 QA Stage 6 | PENDING |
| AC-18 | PowerShell 5.1 + 7.x 双 host | 脚本不依赖 7.x 专属语法；管理员检测、sc.exe 调用、Wait-ServiceRunning 函数均 5.1 兼容 | PASS（QA 双 host 真机验） |
| AC-19 | 步骤 7 中文进度避免静默 > 5s | `install-service.ps1:84-87` 每 3 秒 Write-Host | PASS |

## 5. Design fidelity check（02 §3 / §6 / §7）

| 设计项 | 实现 | 状态 |
|---|---|---|
| 双入口分流 `isWindowsService()` 顶端 | `main.go:93-100` | PASS |
| `runService()` 内 `os.Chdir(filepath.Dir(exe))` | `service_windows.go:50-52` | PASS |
| `svc.Run("frp-easy", &serviceHandler{})` | `service_windows.go:53` | PASS |
| Execute 状态机：START_PENDING(CP=0, Wait=5s) | `service_windows.go:77` | PASS |
| 心跳 ticker 1s 累加 CheckPoint | `service_windows.go:89,105-107` | PASS |
| `go run(stopCh, readyCh)` | `service_windows.go:84-86` | PASS（C-2 落地） |
| close(readyCh) 由 run() 内部触发 | `main.go:292-294` | PASS |
| RUNNING 状态显式 CP=0/Wait=0 | `service_windows.go:112` | PASS（F-2 落地） |
| Stop control code → STOP_PENDING(Wait=30s) → close(stopCh) → run() 优雅关停 → STOPPED | `service_windows.go:122-129` + `main.go:317-326` + L329-333 | PASS |
| Interrogate echo CurrentStatus | `service_windows.go:119-121` | PASS |
| run() 自发退出（ready 后）走 STOPPED + errno=1/0 | `service_windows.go:133-139` | PASS |
| 移除 wrapper.cmd 生成 | `install-service.ps1` grep `Set-Content -Path $WrapperPath` 0 命中 | PASS |
| sc.exe binPath= 直接指向 frp-easy.exe | `install-service.ps1:144,150` | PASS |
| install-service.ps1 顶端清理 legacy wrapper | `install-service.ps1:119-125` | PASS |
| Wait-ServiceMarkedDeleteCleared 15s（Q8） | `install-service.ps1:52-65` | PASS |
| Wait-ServiceRunning 30s + 每 3s 中文进度（Q9 / AC-19） | `install-service.ps1:71-91` | PASS |
| uninstall-service.ps1 注释降级 + 逻辑不动 | `uninstall-service.ps1:74-83` `Remove-Item -Force $WrapperPath` 仍在 | PASS |
| go.mod x/sys 提升 direct | `go.mod:9` direct 块 | PASS |
| 非 Windows 平台 stub | `service_other.go` `errors.New` (Q-D5) | PASS |
| run() 启动序列零字节改（NFR-9） | `main.go:155-289` 顺序保留 | PASS |
| service_windows_test.go //go:build windows tag | `service_windows_test.go:1` | PASS |
| 不写 Event Log（Q4 / OOS-9） | service_windows.go 全文无 `eventlog` 引用 | PASS |
| 不引第三方 service helper（NFR-1） | go.mod 无 NSSM/WinSW 等 | PASS |
| 服务账户保持 LocalSystem（Q6 / OOS-7） | install-service.ps1 sc.exe 无 `obj=` | PASS |
| procmgr.Shutdown 复用（Q7 / NFR-9） | `main.go:332` + procmgr 零字节改 | PASS |
| browseropen isatty 服务模式自然 false（F-5） | `browseropen.go:57` `isTerminalFunc(int(stdinFd()))` | PASS |

**无 design drift。**

## 6. Gate Review 条件对账（03 §4）

| 条件 | 类型 | 落地 |
|---|---|---|
| **C-1** Developer 04 显式记录 F-7 提示（不在 Linux 跑 go mod tidy） | 必须 | PASS · `04_DEVELOPMENT.md` §实施步骤 #1 + Summary 末段 + Design drift 段 C-1 行均明文记录 |
| **C-2** runErrCh <- run(stopCh, readyCh) 两参签名 | 必须 | PASS · `service_windows.go:85` + `main.go:120,101,292-294` |
| **C-3** QA 报告如实记录 AC-15 实际跑的 Linux 发行版 | 建议（QA 接受时 PM 裁决） | PENDING · 留 Stage 6 |
| **C-4** QA 报告追加体积对比 AC（≤ 1 MB） | 建议 | PENDING · 留 Stage 6（04 §Open issues 已转交） |
| **C-5** F-5 / F-6 backlog（建议 T-020 service-mode-stderr-bridge） | 提示，无需阻塞 | PASS · 04 §Open issues for review 第 1 条已转交 PM |

## 7. Positive observations

1. **状态机闭合性**：`service_windows.go` Execute 在四条退出路径（ready 前 run 错 / ready 前 run 正常返回 / Stop 控制码 / ready 后 run 自发退出）全部发齐 `SetServiceStatus(Stopped)` 才 return；任何分支都不会让 SCM 等不到 STOPPED 而走强杀路径——这是 1053-stop-side 等价错误 1061 的根因防御。
2. **跨平台编译纪律**：service_windows.go / service_other.go / service_windows_test.go 三件套 build tag 严格隔离，让 Linux/macOS `go build` 完全不接触 `golang.org/x/sys/windows/svc` import，R-3 缓解结构清晰。配合 04 §实施步骤 #1 的 "不在 Linux 跑 go mod tidy" 纪律，go.mod 的 x/sys direct 状态稳定。
3. **wrapper.cmd 移除的双重防御**：install-service.ps1 顶端（L119-125）清旧、uninstall-service.ps1 末段（L74-83）也清旧，任一升级链路或卸载链路走过都能消除残留——R-5 的"用户磁盘上不留不知何用的 .cmd 文件"目标双保险达成。
4. **Wait-ServiceRunning 中文进度策略**：每 3 秒一行的 cadence 既能让用户感知"在工作"，又不像 1 秒一行那样污染终端；超时退出码 2 配合中文诊断（含 ui.log 路径）让用户在最坏情况下也能自助排障——AC-19 与 NFR-3 双满足。
5. **设计文档对照实现的诚实度**：04 §Design drift 段直接列出 C-1/C-2/F-1/F-2/F-5/F-7 六条 condition 的落地证据（含具体行号），没有自我表扬式回避——非常便于 reviewer 与 QA 二次验证。
6. **insight L17 反向消解**：把"wrapper.cmd 中文路径 host codepage 乱码"问题从"靠 -Encoding Default 兜底"升级为"从根因层不再生成 wrapper.cmd"——这是设计层面的代际改进，比修补漏洞更彻底。

## 8. 与 04 的偏差

无。04 Files changed / 实施步骤 / Design drift 全部与代码实际状态对齐：
- service_windows.go / service_other.go / service_windows_test.go 三个新文件均存在且内容匹配 04 描述；
- main.go 的 main() 分流 / run() 双参签名 / readyCh 关闭点 / stopCh case 全部一致；
- install-service.ps1 / uninstall-service.ps1 / go.mod 改动幅度与 04 描述吻合；
- install.ps1 / verify_all.{ps1,sh} / internal/** 确实零字节改动（grep 验证）。

## 9. Verdict

**APPROVE WITH MINOR FIXES**

### 评定理由

- **0 P0 + 0 P1**：无任何 BLOCKER；状态机闭合、双入口分流、wrapper.cmd 移除、Wait-ServiceRunning 三大功能模块代码层面正确。
- **6 P2 + 4 Nit**：P2 全部属"增强 / 健壮性建议"，不影响 AC-1 ~ AC-19 任一条断言；Nit 属个人风格偏好，Developer 自由选择。
- **C-1 / C-2 / F-2 / F-5 / F-7 全部落地**：03 Gate Review 的必须条件 C-1 / C-2 在代码 + 04 文档双重落地；F-2 / F-5 / F-7 三条设计层 finding 也已在实现中正确处理。
- **AC 覆盖**：19 条 AC 中 16 条代码已就绪、3 条待 QA 真机验证（AC-1/-2/-3 端到端、AC-15 Linux 回归、AC-17 报告标题）——这些非代码层断言不卡 Stage 5。
- **设计 drift**：无。Reuse audit 8 项、Risk 缓解 8 条、Q-D1 ~ Q-D5 五个开发预答案，代码全部按 02 设计实施。

### 给 Developer 的可选清单（不阻塞 Stage 6，建议在后续清扫迭代中处理）

1. **P2-1**（service_windows.go:127）`<-runErrCh` 加 `time.After(28*time.Second)` 超时分支，给最坏路径留 2s 上报 Stopped 余量。
2. **P2-4**（service_windows.go:130-132）未知 SCM 控制码 default 分支改为 `s <- c.CurrentStatus` echo 当前状态，避免事件查看器 timeout warn。
3. **P2-6**（install-service.ps1:202）末尾"请勿移动 InstallDir"补一句原因。
4. **N-1** ready 前 run() 出错的 errno 语义可选用 `return true, 1` 切到 service-specific 错误码，事件查看器更准确。

### 转给 PM 的 backlog 项（C-5 范围）

- **T-021（建议）service-mode-stderr-bridge**：把 main.go L163-165 的 `exposureNotice(...)` stderr 改走 logger，避免服务模式下安全提示丢失（04 §Open issues 第 1 条 / 03 F-6）。
- **C-3 / C-4**：留 Stage 6 QA 在 06_TEST_REPORT.md 落地（AC-15 实际跑的发行版如实记录 + frp-easy.exe 体积对比）。

### 进入 Stage 6 QA 的前提

代码层面通过，可直接进入 QA Tester（Stage 6）。Stage 6 主任务：
- 真机跑 AC-1 / AC-2 / AC-3 / AC-4 / AC-7 / AC-12 / AC-13 / AC-14（Win11 + Server 2019/2022）；
- PowerShell 5.1 + 7.x 双 host 各一次（AC-18）；
- 体积对比（C-4 / F-4）；
- 06_TEST_REPORT.md 含英文裸标题 `## Adversarial tests`（AC-17 / insight L29+L40 红线）。
