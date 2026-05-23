# 07 — 交付报告 · T-019 windows-service-scm-1053-fix

> Harness 流水线 Stage 7（PM Orchestrator）。模式：**full**。
> 上游 6 份阶段产物全部完成：01 (READY) / 02 (READY) / 03 (APPROVED WITH CONDITIONS) / 04 (READY FOR REVIEW) / 05 (APPROVE WITH MINOR FIXES) / 06 (APPROVED FOR DELIVERY)。

---

## 1. 任务摘要

修复 Windows 一键安装链路 `irm ... install.ps1 | iex` 在管理员终端注册并启动 `frp-easy` 服务时 `sc.exe start` 返回错误码 **1053**（"服务没有及时响应启动或控制请求"）的根因故障。

根因：T-008 deploy-kit 注册的 Windows Service `binPath=` 指向 `frp-easy-svc.cmd` 包装脚本 → 拉起 `frp-easy.exe` 普通控制台进程；该进程**不实现 Windows SCM 协议**（无 `StartServiceCtrlDispatcher` / `SetServiceStatus` 握手），SCM 默认 30s 等不到 RUNNING 上报即报 1053 强杀。

修复：把 `frp-easy.exe` 改造成**双入口单二进制**——`main()` 顶端 `svc.IsWindowsService()` 自动分流，被 SCM 拉起时走 `svc.Run("frp-easy", &serviceHandler{})` 服务化分支（完整 START_PENDING(CheckPoint 1s 心跳, WaitHint 5s) → RUNNING → Stop control code → STOP_PENDING(30s) → 优雅关停 procmgr/HTTP/storage → STOPPED 状态机），控制台 / 双击 .exe 启动时走原 `run()` 不变。`wrapper.cmd` 中间壳整条消除：`sc.exe binPath=` 直接指向 `frp-easy.exe`，`os.Chdir(filepath.Dir(os.Executable()))` 在服务化分支首步锁 cwd 替代旧 `cd /d` 语义；`install-service.ps1` 增 `Wait-ServiceMarkedDeleteCleared 15s` + `Wait-ServiceRunning 30s + 3s 中文进度`，让 install.ps1 步骤 7 退出即满足 STATE 4 RUNNING（AC-2 / AC-19 / Q8 / Q9）。

---

## 2. 改动清单

### 2.1 新增文件

| 文件 | 用途 |
|---|---|
| `cmd/frp-easy/service_windows.go` | Windows Service ABI 实现（`//go:build windows`）：`isWindowsService()` / `runService()` / `serviceHandler.Execute` 完整状态机 + 1s CheckPoint 心跳 + Stop control code → close(stopCh) → 优雅关停 |
| `cmd/frp-easy/service_other.go` | 非 Windows 平台空 stub（`//go:build !windows`），用 `errors.New` 避免引入新依赖 |
| `cmd/frp-easy/service_windows_test.go` | Windows 单测（`//go:build windows`）：两个文本契约 grep 用例（`TestInstallServiceScriptNoWrapperGen` + `TestUninstallStillCleansWrapper`） |

### 2.2 编辑文件

| 文件 | 改动摘要 |
|---|---|
| `cmd/frp-easy/main.go` | `main()` 顶端调 `isWindowsService()` 分流到 `runService()`；`run()` 签名改为 `run(stopCh <-chan struct{}, readyCh chan<- struct{}) error`，nil-channel 安全；新增 `if readyCh != nil { close(readyCh) }` 与 `case <-stopCh:` 优雅关停 case。NFR-9 启动序列零字节改。 |
| `scripts/install-service.ps1` | 删除 wrapper.cmd 生成块；`sc.exe binPath=` 直接指向 `$BinaryPath`；新增 `Wait-ServiceMarkedDeleteCleared` 15s 轮询（Q8）+ `Wait-ServiceRunning` 30s 轮询 + 每 3s 中文进度（Q9 / AC-19）；安装前防御性 `Remove-Item -Force frp-easy-svc.cmd`（R-5） |
| `scripts/uninstall-service.ps1` | 注释升级为"防御性清理"语义，逻辑保留（Remove-Item -Force 旧 wrapper） |
| `go.mod` | `golang.org/x/sys v0.44.0` 从 indirect 块移到 direct require 块（手编、不跑 `go mod tidy`，规避 Linux 上 module graph pruning 退回 indirect）|
| `docs/dev-map.md` | 新增 Windows Service ABI 模块条目 + main.go 双入口分流说明 |

### 2.3 零字节改动

- `cmd/frp-easy/main.go` 的 `run()` 内部启动序列（appconf / storage / logrotate / binloc / procmgr / auth / HTTP / autoRestoreProcs）顺序保留；
- `internal/procmgr/` 全部（Q7 复用 graceful shutdown）；
- `internal/appconf/`、`internal/storage/`、`internal/httpapi/`、`internal/logrotate/`、`internal/binloc/`；
- `scripts/install.ps1` / `scripts/install.sh` / `scripts/install-service.sh` / `scripts/uninstall-service.sh`（OOS-2 Linux 路径）；
- `scripts/verify_all.{ps1,sh}`（NFR-5 检查项数不变）；
- 前端 `web/` 全部。

---

## 3. verify_all 结果（declare-done 闸门）

PM 在 Stage 7 入口又跑了一次（继 QA Stage 6 三跑稳定 PASS 之后）。**`pwsh scripts\verify_all.ps1` 真实输出尾段**：

```
[A.1] No hardcoded secrets ... PASS
[A.2] No .env files committed ... PASS
[A.3] TODO / FIXME budget (warn only) ... PASS
[G.1] go vet ... PASS
[G.2] go test ./... ... PASS
[G.3] go build ./cmd/frp-easy ... PASS
[B.1] Install / typecheck ... PASS
[B.2] Lint ... PASS
[B.3] Unit tests pass ... PASS
[B.4] Test count >= baseline ... PASS
[B.5] No tsc residue in web/src/ ... PASS
[C.1] E2E smoke (playwright) ... PASS
[D.1] OpenAPI / tRPC schema present ... PASS
[E.1] CLAUDE.md present ... PASS
[E.2] workflow.md present ... PASS
[E.3] All 7 agent definitions present in .harness/agents/ ... PASS
[E.4] Binding in sync (.harness/ -> .claude/) ... PASS
[E.5] AI-GUIDE.md indexes every .harness/rules/*.md (and vice versa) ... PASS
[E.6] Adversarial tests section present in completed task reports ... PASS

=== Summary ===
  PASS: 19
  WARN: 0
  FAIL: 0
  SKIP: 0
```

**E.6 PASS** 确认本任务 `06_TEST_REPORT.md` 含精确英文裸标题 `## Adversarial tests`（insight L29 + L40 红线）。

---

## 4. AC 覆盖总览

| AC | 状态 | 备注 |
|---|---|---|
| AC-1 / AC-2 / AC-3 / AC-4 / AC-5 / AC-6 / AC-7 / AC-8 / AC-12 / AC-13 / AC-14 | PENDING-USER-VERIFY | QA 主机非管理员，真机 SCM 动态部分由用户在 Win11 管理员 PowerShell 复现（见 §6 用户真机验证清单） |
| AC-9 | PASS | install-service.ps1 / install.ps1 grep `1053` 0 命中；stdout 含 "==> 服务已启动" |
| AC-10 | PASS | uninstall-service.ps1 静态分析：零删数据目录逻辑，仅清理 wrapper |
| AC-11 | PASS | install-service.ps1 `sc failure reset= 60 actions= restart/5000` 不变 |
| AC-15 | DEGRADED（按 03 §2.7 F-3 降级） | 仅 Windows Server 2019+ + Ubuntu 22.04 当 declare-done 硬卡点；其他 Linux 发行版 best-effort。本地 QA 无 4 发行版资源（与 T-016/T-017 历史一致），降级写入 §5 |
| AC-16 | PASS | verify_all PASS:19 三跑稳定 |
| AC-17 | PASS | 06 含精确英文裸标题 `## Adversarial tests`（verify_all E.6 regex 命中） |
| AC-18 | PASS（静态） | install-service.ps1 / install.ps1 不依赖 PS7.x 专属语法；动态真机由用户在 PS5.1 + PS7.x 各跑一次 |
| AC-19 | PASS | install-service.ps1 L84-87 每 3s Write-Host 进度行 |

NFR-1 ~ NFR-9 全部满足（详见 05 §2 检查矩阵）。

---

## 5. Gate Review 条件最终对账

| 条件 | 类型 | 落地证据 |
|---|---|---|
| **C-1** Developer 04 显式记录 "不在 Linux 跑 go mod tidy" | 必须 | `04_DEVELOPMENT.md` §实施步骤 #1 + Summary 末段 + §Design drift C-1 行 |
| **C-2** runErrCh <- run(stopCh, readyCh) 两参签名 | 必须 | `service_windows.go:85` + `main.go:120,101,292-294` 三处一致 |
| **C-3** QA 报告如实记录 AC-15 实际跑的 Linux 发行版 | 建议 | `06_TEST_REPORT.md` §Defects D-3 明文记录"仅 Ubuntu / Windows Server 双机当硬卡，其他历史 best-effort"；PM 在本节 §4 AC-15 行采纳此降级 |
| **C-4** QA 报告追加体积对比 AC（≤ 1 MB） | 建议 | `06_TEST_REPORT.md` §boundary tests 实测 T-018 vs T-019 二进制增长 35,328 bytes（约 35 KB，远小于 1 MB 上限）PASS |
| **C-5** F-5/F-6 service-mode-stderr-bridge backlog | 提示，无需阻塞 | 已转给 PM，开 follow-up（见 §7） |

---

## 6. 给用户的真机验证清单（PM 转交）

> 因 QA 本地主机为非管理员（W11 Home 10.0.26200，user `yangx`），真机 SCM 动态 AC 需要用户在管理员 PowerShell 复现。期待用户用 PS7.x（或 PS5.1）执行下列断言。

**前提**：用户从最新 main HEAD（即 T-019 合并后）拿 release zip 或本地 `git pull && pwsh scripts\build.ps1 && pwsh scripts\package.ps1` 重新构建发布包。

**步骤**（管理员 PowerShell）：

```powershell
# 一键升级测试
irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex
# 期望：8 步全过、最终 "==> 服务已启动"、退出码 0

# AC-1 / AC-2：服务 RUNNING
sc.exe query frp-easy | Select-String 'STATE'
# 期望：STATE              : 4  RUNNING

# AC-3：UI 可访问
Invoke-WebRequest http://127.0.0.1:7800 -UseBasicParsing | Select-Object -Property StatusCode
# 期望：StatusCode 200 / 301 / 302 / 401 任一

# AC-7：停服 → 再启
sc.exe stop frp-easy
Start-Sleep -Seconds 2
sc.exe query frp-easy | Select-String 'STATE'   # 期望 STATE 1 STOPPED
sc.exe start frp-easy
Start-Sleep -Seconds 3
sc.exe query frp-easy | Select-String 'STATE'   # 期望 STATE 4 RUNNING

# AC-8：reboot 自启
Restart-Computer   # （用户自决；重启后再次 sc query 期望 STATE 4 RUNNING）
```

**若 AC-1 ~ AC-3 任一失败 → 报回 PM 并开 BLOCKER（本任务 declare-done 退回 Stage 4）**。
**若 AC-7 / AC-8 失败 → 报回 PM 评估是否 follow-up（NFR-7 / sc.exe failure action 范围）**。

可选环境测试：
- AC-12（中文路径）：`$env:FRP_EASY_INSTALL_DIR = "C:\程序\frp-easy"; irm ... | iex`；
- AC-13（空格路径）：`$env:FRP_EASY_INSTALL_DIR = "C:\Program Files\frp easy v1"; irm ... | iex`；
- AC-18（PS5.1）：管理员 `powershell.exe`（**注意 D-1 历史遗留**：T-018 起 `.ps1` 文件在 PS5.1 + zh-CN 主机磁盘加载时可能 BOM 问题导致 syntax error，已知 baseline，不阻塞 T-019）。

---

## 7. Follow-up backlog（转给 PM 立项）

> T-019 不直接处理，但需要新建任务跟踪。QA / Reviewer 建议的编号 T-020 / T-021 与并行已归档的 **T-020 claude-settings-context7-fix** 冲突，本 07 把它们重编为 **T-021 / T-022**，等用户开新任务时复用此 slug：

1. **T-021 (建议) — encoding-ps51-bom · MAJOR · 历史遗留（非 T-019 引入）**
   - 触发：QA 06 §Defects D-1 实测 `powershell.exe` (Windows PowerShell 5.1) + zh-CN 主机加载磁盘上的 `.ps1` 文件时遇 UTF-8 无 BOM 解析失败（中文字符被错解为 ANSI/GBK，syntax error）。
   - 影响：所有 `scripts/*.ps1` 在 PS5.1 + zh-CN 直接执行（非 `irm | iex` 管道形态）会失败；T-018 同款，未被 declare-done 拦截。
   - 修复方向：把所有 `scripts/*.ps1` 改为 UTF-8 with BOM；新增 verify_all 检查项确保 BOM 持久。

2. **T-022 (建议) — service-mode-stderr-bridge · MINOR / 增强**
   - 触发：05 §6 C-5 + 03 §F-6 + 04 §Open issues 第 1 条。
   - 问题：`main.go` L138-140 在 `UIBindAddr == "0.0.0.0"` 时 `fmt.Fprint(os.Stderr, exposureNotice(...))` —— 服务模式下 stderr 被 SCM 丢弃，安全提示不进 ui.log。
   - 修复方向：把 `exposureNotice` 改走 logger（slog → lumberjack → ui.log）让两种宿主下提示都不丢。

3. **代码层增强（非新任务，建议下次 polish-pass 顺手做）**
   - 05 §P2-1：`service_windows.go:127` `<-runErrCh` 加 `time.After(28*time.Second)` 兜底超时；
   - 05 §P2-4：`service_windows.go` Execute default case 改为 `s <- c.CurrentStatus` echo 当前状态；
   - 05 §P2-6：`install-service.ps1:202` 末尾说明补"否则 SCM 找不到 frp-easy.exe 会启动失败"原因。

---

## 8. Insight

> 本任务从 04 / 05 / 06 中筛选出的"非琐碎、跨任务可复用"的项目真相，下面三条会被 `scripts/archive-task.ps1` 自动追加到 `.harness/insight-index.md`。

- **2026-05-23** · Go `os.Executable()` + `os.Chdir(filepath.Dir(exe))` 在 Windows 服务模式锁 cwd 是 wrapper.cmd 的零成本替代品：Go stdlib 底层调 `GetModuleFileNameW`（UTF-16）天然兼容中文 / 空格 / UNC 路径，不依赖 host codepage / GB18030 —— 反向消解了 insight L17（`Set-Content -Encoding Default` 写 wrapper.cmd 中文路径乱码）的全部根因。后续任何 Go 服务化需要锁 cwd 的场景应优先选此 idiom 而非 `.cmd` 中间壳 · evidence: `cmd/frp-easy/service_windows.go` L46-49 + T-019 02 §8 R-4
- **2026-05-23** · Windows Service 注册时 `sc.exe binPath=` 指向**普通控制台 .exe 或 .cmd 包装**会触发 SCM 错误 1053（30s 等不到 SERVICE_RUNNING 上报强杀）：必须让 .exe 自身实现 SCM 协议——Go 用 `golang.org/x/sys/windows/svc.IsWindowsService()` 自动分流 + `svc.Run(name, handler)` + Execute 内 SetServiceStatus 状态机（START_PENDING + 1s CheckPoint + WaitHint 5s → RUNNING → STOP_PENDING 30s → STOPPED）；wrapper.cmd 完全可移除，binPath 直接指 .exe · evidence: T-019 02 §1 + service_windows.go Execute
- **2026-05-23** · Go module graph pruning 在 build tag 隔离场景下会让 windows-only 引用的 direct require（如 `golang.org/x/sys`）在 Linux 跑 `go mod tidy` 时被退回 indirect；保持 direct 块稳定必须**手编 go.mod 或仅在 `GOOS=windows go mod tidy`** 下执行，不在 Linux 主开发机跑 `tidy` · evidence: T-019 03 §F-7 + 04 §实施步骤 #1
- **2026-05-23** · Harness sub-agent 子集（gate-reviewer / code-reviewer）frontmatter 默认 `tools: Read, Glob, Grep`（无 Write/Edit），按 insight L41 已知问题：reviewer 在派发时即使 prompt 显式"必须直接写到文件"也无法落盘，PM 必须接管"代为落盘"动作；建议未来在 `.harness/agents/*.md` frontmatter 加上 Write 工具（与 developer / qa-tester 对齐）让 reviewer 也能自落盘 · evidence: T-019 Stage 3 + Stage 5 两次同款 fallback

---

## 9. Verdict

**DELIVERED**

理由：
- verify_all `PASS:19, WARN:0, FAIL:0, SKIP:0` 三次稳定（QA Stage 6 三跑 + PM Stage 7 一跑）；
- 19 条 AC 中 11 条 PENDING-USER-VERIFY（管理员权限 / 真机 SCM 动态部分，按 §6 转交）、1 条 DEGRADED（AC-15 按 Gate Review F-3 + QA D-3 双重确认降级）、其余 7 条 PASS；无 BLOCKER / CRITICAL；1 MAJOR D-1 历史遗留（T-018 同款，非 T-019 引入）不阻塞；
- Gate Review 条件 C-1 / C-2 必须项已在代码 + 文档双重落地；C-3 / C-4 建议项已由 QA 在 06 妥处；C-5 转入 follow-up T-022；
- 代码层面（02 §3.1 设计 + 04 实施 + 05 §5 design fidelity check）无 design drift，状态机闭合性、跨平台编译纪律、wrapper.cmd 移除的双重防御均经 reviewer 真读验证；
- `## Insight` 段 4 条已就位，`scripts/archive-task.ps1` 将其追加到 `.harness/insight-index.md` 后本任务文档迁移到 `docs/features/_archived/windows-service-scm-1053-fix/`。
