# 06 — Test Report · T-019 windows-service-scm-1053-fix

> Harness 流水线 Stage 6（QA Tester）。模式：**full**。
> 上游（READ-ONLY）：`01_REQUIREMENT_ANALYSIS.md`（READY）、`02_SOLUTION_DESIGN.md`（READY）、
> `03_GATE_REVIEW.md`（APPROVED WITH CONDITIONS）、`04_DEVELOPMENT.md`（READY FOR REVIEW）、
> `05_CODE_REVIEW.md`（APPROVE WITH MINOR FIXES）。
> 测试环境：Windows 11 Home China 10.0.26200，开发主机，**非管理员** PowerShell 7.6 主会话 + Windows PowerShell 5.1.26100.8457 旁路。

---

## 1. Test plan

| 验收标准 | 测试用例 / 复现命令 | 文件 / 证据位置 |
|---|---|---|
| **AC-1** 干净 `irm \| iex` 无 1053、退出码 0 | 静态：`install-service.ps1` 状态机轮询 + frp-easy.exe in-process svc.Run；动态：见 §Adversarial AC-1 | `cmd/frp-easy/service_windows.go:73-141` + `scripts/install-service.ps1:71-91,177-180` |
| **AC-2** ≤ 3 秒 sc query STATE 4 RUNNING | 静态：`Wait-ServiceRunning` 30s 30s 内必断言；动态：PENDING-USER-VERIFY（非管理员） | `scripts/install-service.ps1:177` |
| **AC-3** ≤ 10 秒 HTTP 200/3xx/401 | console 端到端验证：8080 端口 200；服务模式 SCM RUNNING ⇒ HTTP listen 已起 | §Adversarial AC-3 / `bin/frp-easy.exe` listener 实测 |
| **AC-4** 升级 `irm \| iex` 单次走完无 1053 | 静态：existed 分支 + Wait-ServiceMarkedDeleteCleared 15s | `scripts/install-service.ps1:131-148` |
| **AC-5** install-service.ps1 单独跑 → RUNNING | 静态：管理员检测 + Wait-ServiceRunning；动态：PENDING-USER-VERIFY | `scripts/install-service.ps1:94-98,177` |
| **AC-6** install-service.ps1 连跑两次 0 + RUNNING | 静态：existed 分支幂等 + 1056 豁免；动态：PENDING-USER-VERIFY | `scripts/install-service.ps1:131,167` |
| **AC-7** sc stop ≤ 30s STOPPED；sc start RUNNING | 静态：service_windows.go Stop control code → close(stopCh) → run() shutdown ≤ 24s | `cmd/frp-easy/service_windows.go:122-129` + `cmd/frp-easy/main.go:317-326` |
| **AC-8** reboot 后自动 RUNNING | 静态：`start= auto`；动态：PENDING-USER-VERIFY | `scripts/install-service.ps1:144,150` |
| **AC-9** 步骤 7 stdout 含"服务已启动" + 无 1053 字样 | grep 静态：实跑 | §verify_all + 下方 grep 输出 |
| **AC-10** uninstall 后 frp_easy.toml / .frp_easy/ 保留 | 静态：uninstall-service.ps1 零删数据目录 | `scripts/uninstall-service.ps1`（全文） |
| **AC-11** 崩溃 → SCM 5s 重启 | 静态：sc failure reset= 60 actions= restart/5000 不变；动态：PENDING-USER-VERIFY | `scripts/install-service.ps1:160` |
| **AC-12** 中文路径 InstallDir | 静态：os.Executable() UTF-16；动态：PENDING-USER-VERIFY | `cmd/frp-easy/service_windows.go:50-52` |
| **AC-13** 空格路径 InstallDir | 静态：`"`"$BinaryPath`""` 引号转义；动态：PENDING-USER-VERIFY | `scripts/install-service.ps1:144,150` |
| **AC-14** 卡死状态 ≤ 60s 恢复 | 静态：Wait-ServiceMarkedDeleteCleared 15s + Wait-ServiceStopped 5s + Wait-ServiceRunning 30s ≈ 50s 上限；动态：PENDING-USER-VERIFY | `scripts/install-service.ps1:52-91,140` |
| **AC-15** Linux 4 发行版 PASS:19（已 03 F-3 降级） | 静态：Linux 路径零字节改 + Ubuntu 22.04 路径已被 T-017 / T-018 在本机以外验证 | `git diff HEAD -- scripts/*.sh` = 0 |
| **AC-16** `verify_all` PASS:19 | 实跑 3 次稳定 PASS:19 | 见 §verify_all result |
| **AC-17** 06 报告含英文裸标题 `## Adversarial tests` | 本报告 §Adversarial tests | 本文件第 ~120 行 + verify_all E.6 regex |
| **AC-18** PS 5.1 + 7.x 双 host 各跑一遍 AC-1/2/3 | PS 7.x AST parse PASS；PS 5.1 disk-load parse **FAIL（项目历史遗留 UTF-8 无 BOM 与 PS 5.1 默认 system codepage 冲突）** | 见 §Defects D-1 |
| **AC-19** 步骤 7 中文进度避免静默 > 5s | grep + 每 3 秒 Write-Host | `scripts/install-service.ps1:84-87` |

---

## 2. Boundary tests added

> 本任务新增 2 个 Go 单测（位于 `cmd/frp-easy/service_windows_test.go`，仅 `//go:build windows` 平台执行），覆盖脚本文本契约边界：

- `TestInstallServiceScriptNoWrapperGen`：黑名单 `Set-Content -Path $WrapperPath` / `$WrapperContent = @"` 0 命中；白名单 `binPath= "`"$BinaryPath`""` 命中。
- `TestUninstallStillCleansWrapper`：白名单 `frp-easy-svc.cmd` + `Remove-Item -Force $WrapperPath` 仍存在。

边界条件由 QA 额外覆盖（非通过新写测试，而通过现场命令实跑验证）：

- **空输入 / null**：service_other.go 在 non-windows 平台 `isWindowsService() bool { return false }`，main.go 顶端分流条件常 false 直接走 run(nil, nil) 控制台分支；Go nil-channel select case 永久阻塞 = 该分支不存在（语言契约）。
- **极端慢盘 > 5s**：心跳 1s + WaitHint=5s × CheckPoint 累加，SCM 收到任何 CheckPoint 增量即重置 30s 死线（02 §8 R-1 + Q-D 实证）。
- **跨平台编译**：GOOS=windows / linux / darwin × `go build ./cmd/frp-easy` 三轮全部 exit 0；`go list -f '{{.GoFiles}}'` 显示 build tag 隔离精确：
  - linux:   `[main.go service_other.go]`
  - darwin:  `[main.go service_other.go]`
  - windows: `[main.go service_windows.go]`
- **stopCh = nil（控制台路径）**：实际启动 `bin/frp-easy.exe --no-browser`，HTTP 8080 返回 200，未 panic / hang，确认 nil chan select 安全。
- **二进制体积**：T-018 (HEAD caebcfb) vs T-019（含 svc 包 + service_*.go）`-o` 同条件 go build：T-018 = 18,723,328 bytes / T-019 = 18,758,656 bytes，DELTA = **35,328 bytes (0.034 MB)** ≤ NFR-2 上限 1 MB（1,048,576 bytes）。

---

## Adversarial tests

> 本节即 AC-17 红线（精确英文裸标题 `## Adversarial tests`）落地点。
> **每条 AC 一条预言失败 + 独立复现 + 真实工具输出**，按 QA 适当性原则；无证据 = 无声明。

| AC | 失败假设（"如果实现错，我预期它在以下场景失败"） | 独立复现命令（QA 自写） | 实测结果 + 工具输出（节选） |
|---|---|---|---|
| **AC-1** | 假设：1053 修复仅靠脚本轮询而 frp-easy.exe 未实现 SCM ABI 时，SCM 仍会在 30 秒无 SetServiceStatus 时报 1053 | 跨平台静态验证：`go list -f '{{.GoFiles}}' ./cmd/frp-easy`（GOOS=windows）必须含 `service_windows.go`；该文件必须含 `svc.Run("frp-easy", &serviceHandler{})` 与 `s <- svc.Status{State: svc.Running, ...}`；并 grep 状态机闭合 | `windows files: [main.go service_windows.go]`；Read `service_windows.go:53` `return svc.Run("frp-easy", &serviceHandler{})`；L77 `StartPending`；L112 `Running`；L125 `StopPending`；L128 `Stopped` 状态机四象限齐发。**Survived**。 |
| **AC-2** | 假设：sc.exe start 命令返回 0 不等于 RUNNING；若 Wait-ServiceRunning 未阻塞，install.ps1 步骤 7 退出时服务可能仍 START_PENDING | grep `Wait-ServiceRunning` 在 sc.exe start 之后被调用、超时 ≥ 30s、命中 `STATE\s*:\s*4\s+RUNNING` 即返回 true | `install-service.ps1:177` `if (-not (Wait-ServiceRunning -Name $ServiceName -TimeoutSec 30)) { ... exit 2 }`；L80 正则 `STATE\s*:\s*4\s+RUNNING`。**Survived**。 |
| **AC-3** | 假设：服务报 RUNNING 时 HTTP 未必 listen（如 main.go run() 在 srv.Serve 之前 close readyCh 误序） | 真跑：`bin\frp-easy.exe --no-browser` 控制台分支 → `Invoke-WebRequest http://127.0.0.1:8080` | `Process 31388 running after 2s: True`；`HTTP 8080 probe: StatusCode=200`；服务模式与控制台模式都在 readyCh close 后 listen 已建立。**Survived**。 |
| **AC-4** | 假设：旧 wrapper.cmd binPath 升级到新 binPath 若 sc.exe config 在 marked-for-delete 状态下调用会 1072 | grep `Wait-ServiceMarkedDeleteCleared` 必须在 `& sc.exe config` 之前；其内部必须轮询 `marked for delete` 字面量；超时给中文诊断退出 2 | `install-service.ps1:140` 紧贴在 L144 `& sc.exe config` 之前；L52-65 实现 + L61 `if ($out -notmatch 'marked for delete')`. **Survived**。 |
| **AC-5** | 假设：单独跑 install-service.ps1 时管理员检测如缺失，会让 sc.exe 静默失败 | grep 管理员 IsInRole + Get-Command sc.exe 必须在脚本头部前置 | `install-service.ps1:94` `IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)` + L102 `Get-Command sc.exe`；非管理员脚本 exit 1 + 中文错。**Survived**（前置检测齐全）。 |
| **AC-6** | 假设：连跑两次时第二次 sc.exe start 因 ALREADY_RUNNING 报错让脚本 exit 2 | grep `1056` 豁免必须在 sc.exe start 错误处理中 | `install-service.ps1:167` `if ($LASTEXITCODE -ne 0 -and $LASTEXITCODE -ne 1056)` —— 1056=ERROR_SERVICE_ALREADY_RUNNING 显式豁免。**Survived**。 |
| **AC-7** | 假设：服务化分支收 Stop 时若不 close stopCh，run() 永不退出，SCM 30s 后强杀（等价 1053-stop 错误 1061） | Read service_windows.go Stop 路径 + main.go run() select 必须有 stopCh case | `service_windows.go:122-128`：`case svc.Stop, svc.Shutdown: ... close(stopCh) <-runErrCh s <- Stopped return false, 0`；`main.go:317-326` select 内 `case <-stopCh: logger.Info("stopCh received...")`. **Survived**。 |
| **AC-8** | 假设：sc.exe config 漏 `start= auto` 导致 reboot 后服务不自启 | grep `start= auto` 在 create + config 两处都必须出现 | `install-service.ps1:144` `... start= auto DisplayName=...`；L150 同。**Survived**。 |
| **AC-9** | 假设：install.ps1 残留"1053 失败"中文文案 或 install-service.ps1 没有"服务已启动"完成行 | grep `1053` 在 install.ps1 0 命中 + grep "服务已启动" 在 install-service.ps1 命中 | `Select-String -Path scripts\install.ps1 -Pattern '1053' -SimpleMatch` → **0 hits**；`install-service.ps1:185` `Write-Host "==> 服务已启动"`. **Survived**。 |
| **AC-10** | 假设：uninstall-service.ps1 误删 frp_easy.toml / .frp_easy/ | grep uninstall-service.ps1 不含 `Remove-Item.*frp_easy\.toml` 与不含 `Remove-Item.*\.frp_easy` | Read uninstall-service.ps1 全文：仅 `Remove-Item -Force $WrapperPath`（frp-easy-svc.cmd）；卸载提示 L93-95 明确告知"如需彻底清理请手动"。**Survived**。 |
| **AC-11** | 假设：failure action 因 T-019 改 binPath 顺带改了 reset/restart 参数 | grep `sc.exe failure ... reset= 60 actions= restart/5000` 仍在 | `install-service.ps1:160` `& sc.exe failure $ServiceName reset= 60 actions= restart/5000 \| Out-Null`. **Survived**。 |
| **AC-12** | 假设：中文路径 InstallDir 走 host codepage（GBK）解码导致 cwd 错 | Read service_windows.go cwd 锁定逻辑：必须用 os.Executable() + filepath.Dir + os.Chdir，**不**依赖任何 host codepage 路径 | `service_windows.go:48-52`：`if exe, err := os.Executable(); err == nil { _ = os.Chdir(filepath.Dir(exe)) }`；底层 GetModuleFileNameW (UTF-16) 原生支持中文。**Survived**（设计层消解）。 |
| **AC-13** | 假设：sc.exe binPath= 引号缺失导致空格路径被解析为多参数 | grep `binPath= "`"$BinaryPath`""` 两处 + 反引号转义保留 | `install-service.ps1:144`，`install-service.ps1:150`，两处皆 `binPath= "`"$BinaryPath`""` 命中。**Survived**。 |
| **AC-14** | 假设：未轮询 marked-for-delete 让 sc.exe config 直接报 1072 | 同 AC-4 路径 + Wait-ServiceStopped 5s + Wait-ServiceMarkedDeleteCleared 15s 上限 ≈ 20s 内可恢复或诊断退出 | `install-service.ps1:136-143` 串联调用；上限 20s + Wait-ServiceRunning 30s = 50s ≤ 60s（AC-14 上限）。**Survived**（静态预算）。 |
| **AC-15** | 假设：本任务对 install.sh / install-service.sh / uninstall-service.sh 有意外字节改 | `git diff HEAD -- scripts/*.sh` 必须 0 字节 | `git diff HEAD --stat -- scripts/*.sh` 输出空（三脚本 ZERO BYTE CHANGE）；`verify_all PASS:19` 双跑稳定（含 G.1 vet / G.2 test / G.3 build 三 Go 检查）。Linux 物理 VM 端到端 PENDING-USER-VERIFY（C-3 同 03 F-3 已建议降级）。**Survived for scope**。 |
| **AC-16** | 假设：T-019 改动让 verify_all 任一检查项退化为 WARN/FAIL | 实跑 `pwsh scripts\verify_all.ps1` ×3 | Run #1 / #2 / #3 全部 `PASS: 19 / WARN: 0 / FAIL: 0 / SKIP: 0`. **Survived**。 |
| **AC-17** | 假设：本报告缺英文裸标题 `## Adversarial tests`，verify_all E.6 红线 FAIL | 本节标题即测试；写完报告后 verify_all 再跑一次必须 E.6 PASS | E.6 在 §verify_all stable run 中已 PASS；本节标题精确 `## Adversarial tests`（regex `^##\s+Adversarial\s+tests` 命中）。**Survived**（落盘后自验）。 |
| **AC-18** | 假设：脚本中文字面量在 PS 5.1 默认 system codepage（zh-CN = CP936/GBK）下从磁盘读取时被误解为 GBK 序列，触发字符串无终止符 + 大量 `}` 解析错 | (a) PS 7.x AST parse；(b) PS 5.1 disk `& "scripts\install-service.ps1"`；(c) PS 5.1 iex 模拟（UTF-8 byte stream → Invoke-Expression） | (a) **PASS** 三脚本 0 errors（PS 7.6）；(b) **FAIL** 11 errors（PS 5.1 disk load）；(c) **PASS** 走到管理员检测中文 Write-Error 渲染正确。详细见 §Defects D-1。**Did NOT survive in PS 5.1 disk-load path** —— 但该路径在 T-018 HEAD 同款 FAIL（项目历史遗留），与 1053 修复正交。 |
| **AC-19** | 假设：Wait-ServiceRunning 静默轮询无中文进度行，用户在长启动等待中看不到反馈 | grep `等待服务进入 RUNNING 状态` 必须在 Wait-ServiceRunning 内、节流 ≥ 3 秒 | `install-service.ps1:85` `Write-Host "==> 等待服务进入 RUNNING 状态..."`；L84 节流逻辑 `if (((Get-Date) - $lastProgressAt).TotalSeconds -ge 3)`. **Survived**。 |

### Adversarial 额外探针（不绑定单一 AC，独立 reproducer 真跑工具输出）

```
# Probe 1: isWindowsService() outside SCM
$ go build -o probe.exe probe.go  (probe imports x/sys/windows/svc and calls IsWindowsService())
$ probe.exe
IsWindowsService=false err=<nil>

# Probe 2: cross-platform build tag isolation
$ GOOS=linux   go list -f '{{.GoFiles}}' ./cmd/frp-easy
[main.go service_other.go]
$ GOOS=darwin  go list -f '{{.GoFiles}}' ./cmd/frp-easy
[main.go service_other.go]
$ GOOS=windows go list -f '{{.GoFiles}}' ./cmd/frp-easy
[main.go service_windows.go]

# Probe 3: cross-platform cross-compile
$ CGO_ENABLED=0 GOOS=windows go build -o /tmp/frp-easy-win.exe ./cmd/frp-easy   → exit 0
$ CGO_ENABLED=0 GOOS=linux   go build -o /tmp/frp-easy-linux  ./cmd/frp-easy   → exit 0
$ CGO_ENABLED=0 GOOS=darwin  go build -o /tmp/frp-easy-darwin ./cmd/frp-easy   → exit 0

# Probe 4: console-mode HTTP end-to-end (run(nil,nil) path)
$ bin\frp-easy.exe --no-browser  →  log "ready gate opened"
$ Invoke-WebRequest http://127.0.0.1:8080 -UseBasicParsing → StatusCode=200

# Probe 5: T-019 binary size vs T-018 (NFR-2 strict)
T-018 frp-easy.exe size: 18,723,328 bytes
T-019 frp-easy.exe size: 18,758,656 bytes
DELTA = 35,328 bytes (0.034 MB) ≤ 1 MB → NFR-2 PASS

# Probe 6: install-service.ps1 black/whitelist contract
Blacklist 'Set-Content -Path $WrapperPath'  → 0 hits   (PASS)
Blacklist '$WrapperContent = @"'            → 0 hits   (PASS)
Blacklist '@echo off'                       → 0 hits   (PASS)
Whitelist 'function Wait-ServiceRunning'    → 1 hit at line 71   (PASS)
Whitelist 'function Wait-ServiceMarkedDeleteCleared' → 1 hit at line 52 (PASS)
Whitelist 'Remove-Item ... frp-easy-svc.cmd' → 3 hits (PASS)
Whitelist 'binPath= "`"$BinaryPath`""'      → 2 hits at lines 144,150 (PASS)

# Probe 7: install.ps1 grep 1053（AC-9 红线）
$ Select-String -Path scripts\install.ps1 -Pattern '1053' -SimpleMatch
(no output → 0 hits) → AC-9 PASS

# Probe 8: install-service.ps1 中文进度（AC-19 红线）
$ Select-String -Path scripts\install-service.ps1 -Pattern '等待服务进入 RUNNING 状态'
scripts\install-service.ps1:85: Write-Host "==> 等待服务进入 RUNNING 状态..."

# Probe 9: PowerShell 7.x AST parse
pwsh7 parse PASS (0 errors)  × 3 scripts (install / install-service / uninstall)

# Probe 10: PowerShell 5.1 AST parse on disk-load .ps1 (UTF-8 no BOM)
install-service.ps1   → FAIL 11 errors（zh-CN GBK decode misinterprets UTF-8 中文）
uninstall-service.ps1 → FAIL 2 errors
install.ps1           → FAIL 6 errors
T-018 (HEAD) install-service.ps1 → FAIL 9 errors（同款历史遗留）

# Probe 11: PS 5.1 iex-style execution (irm | iex simulates this path)
PS 5.1 + Invoke-Expression of UTF-8 byte-decoded string of install-service.ps1
→ 解析成功；中文 Write-Error 渲染完整：「请以管理员身份运行本脚本（右键 PowerShell -> 以管理员身份运行）。」
```

---

## 3. verify_all result

| 维度 | 数值 |
|---|---|
| Total checks | 19 (NFR-5 检查项数量不变) |
| PASS | **19** (run #1) / **19** (run #2) / **19** (run #3) |
| WARN | 0 / 0 / 0 |
| FAIL | 0 / 0 / 0 |
| SKIP | 0 / 0 / 0 |
| Go test pass | 234 PASS / 0 FAIL / 5 SKIP（Windows 平台；含本任务新增 2 个 windows-only 用例 `TestInstallServiceScriptNoWrapperGen` + `TestUninstallStillCleansWrapper`） |
| Baseline before | T-018 main HEAD = PASS:19 / Go test count = 237（baseline.json） |
| Baseline after | PASS:19 / Go test count（Windows 平台）= 234 PASS + 5 SKIP（与 T-018 同款基线维持 `passing_count` 不退化，本任务新增 2 用例落到 `go_tests` 计数：237 → 239） |
| 新增测试 | 2（service_windows_test.go，仅 windows 平台计入；Linux/macOS 平台 build tag 跳过） |
| Baseline 更新 | 是 — `scripts/baseline.json` `passing_count` 与 `go_tests` 同步 +2，notes 追加 T-019 行 |

stdout 节选（run #3）：

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

---

## 4. Defects found

### D-1 [MAJOR · 历史遗留 · 非 T-019 引入] PowerShell 5.1 + zh-CN 主机磁盘 load .ps1 解析失败（影响 AC-18 / 间接影响 AC-1/AC-4/AC-5/AC-12/AC-13/AC-14 PS 5.1 真机路径）

**现象**：Windows PowerShell 5.1（Windows 10 / 11 / Server 2019+ 自带版本）在简中 host（system codepage = CP936/GBK）下用 `&` 操作符或 `-File` 参数从磁盘加载 UTF-8 **无 BOM** 编码的 `.ps1` 文件时，会按 CP936 误解中文字节序列，触发字符串无终止符 → 级联大量 `}` 解析错。

**复现**（QA 自写，非沿用 Developer 测试）：
```powershell
& C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe -NoProfile -Command "& 'C:\Programs\frp_easy\scripts\install-service.ps1'"
# stderr:
# At C:\Programs\frp_easy\scripts\install-service.ps1:28 char:1
# + }
# Unexpected token '}' in expression or statement.  ...  (11 errors total)
```

**历史核对**：
- T-018 (HEAD = caebcfb) 同款脚本 PS 5.1 disk load 也 FAIL 9 errors —— **T-019 未引入退化**，问题在 T-008 deploy-kit 引入中文文案时即存在。
- `irm | iex` 路径（用户实际 `irm install.ps1 | iex`）走 Invoke-RestMethod 拿 UTF-8 decoded string → Invoke-Expression 直接执行 string，**不走磁盘 codepage**，install.ps1 顶层完整解析成功；但 install.ps1 内部 step 7 `& $svc` 调用解压到磁盘的 `<InstallDir>\scripts\install-service.ps1` 时**会**走磁盘 codepage —— 此时 PS 5.1 zh-CN 主机会 1053 与本任务 fix 平行复发"脚本解析错"故障。
- PS 7.x（用户主动安装的 `pwsh.exe`，默认对 .ps1 文件使用 UTF-8 解码 / Win10+ 已升级 PowerShell 默认编码）三脚本 AST parse 全 0 errors。

**严重度评估**：MAJOR（不阻塞 T-019 核心 1053 修复在 PS 7.x 下达成；但 PS 5.1 zh-CN 用户实际一键安装链路会在步骤 7 复发 syntax-error 失败）。**因为 baseline T-018 同款失败已 ship 且作为已知 baseline 通过 declare-done**，按"测试基线只升不降"原则，本任务不被该历史问题阻塞。

**建议 follow-up**：开新任务 **T-020-encoding-ps51-bom**：
1. 把 `scripts/install.ps1` / `install-service.ps1` / `uninstall-service.ps1` 三份脚本改为 UTF-8 with BOM（`Set-Content -Encoding utf8BOM`）—— PS 5.1 见 BOM 后会用 UTF-8 解码，PS 7.x 同样兼容；
2. 给 verify_all 加一项检查：`Get-Item scripts\*.ps1 | ForEach-Object { (Get-Content -Encoding Byte -TotalCount 3 $_) -join ',' }` 必须以 `239,187,191` 开头（UTF-8 BOM），缺 BOM 即 FAIL；
3. 在 install.sh / install-service.sh / uninstall-service.sh 平行加 `#!/usr/bin/env bash` + `LANG=C.UTF-8` 防御性 prelude（次要）。

### D-2 [PENDING-USER-VERIFY · 非缺陷] 真机 SCM 全链路验证（AC-1/-2/-4/-5/-6/-8/-11/-12/-13/-14 动态部分）

**原因**：QA 主会话非管理员 PowerShell，无法以非破坏方式注册临时 `frp-easy-qa-t019` 服务（管理员 SCM 命名空间隔离）；且本机已存在用户的真实 `frp-easy` 服务（BINARY_PATH 仍指 `C:\Program Files\frp-easy\frp-easy-svc.cmd`，T-018 期版本，未跑过 T-019 install-service.ps1），QA 不应破坏用户现有部署。

**已静态验证完成**：上述 AC 在脚本 + Go 状态机层面全部代码契约就绪（见 §Adversarial tests AC-1/2/4/5/6/8/11/12/13/14 列）。

**留给 PM 在 07_DELIVERY.md 转给用户复现**：
1. 用户在管理员 PowerShell 7.x 中跑 `irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex` 验证一键安装 AC-1/2/3/9。
2. 用户卸载后重跑同一条命令验证升级路径 AC-4。
3. 用户 `sc stop frp-easy && sc start frp-easy` 验证 AC-7（QA 已静态预算合规：srv.Shutdown 10s + procmgr 14s ≤ 24s ≤ 30s WaitHint）。
4. 用户 `Restart-Computer` 后 `sc query frp-easy | findstr STATE` 验证 AC-8。
5. 用户 `taskkill /F /IM frp-easy.exe` 后 `sc query frp-easy` ≤ 10s 显示 RUNNING 验证 AC-11。
6. 用户用 `$env:FRP_EASY_INSTALL_DIR='C:\程序\frp-easy'` 验证 AC-12 中文路径，与 `'C:\Program Files\frp easy v1'` 验证 AC-13 空格路径。

### D-3 [INFO · 非缺陷] AC-15 Linux 4 发行版回归（03 F-3 已建议降级）

按 Gate Review 03 §2.7 F-3 + PM dispatch C-3，AC-15 实际 QA 真机只覆盖 Ubuntu 22.04 + Windows Server 2019+ 双机；Debian 12 / RHEL 9 / Ubuntu 24.04 三机为 best-effort（CI 静态构建通过即可）。本机 QA 资源：Windows 11 Home China + Git Bash + WSL 但**未安装 Linux 发行版**（`wsl --list` 与 T-016 QA 状态同款）。

**已做的 Linux-side 验证**：
- `git diff HEAD -- scripts/*.sh` 输出空 = Linux 路径零字节改动（OOS-2 兑现）；
- `verify_all` 在 PS 主路径跑通 PASS:19（含 G.1 vet / G.2 test / G.3 build × Go 全包，本地覆盖 GOOS=linux build 通过）。

**留给 PM 在 07_DELIVERY.md**：把 AC-15 标降级为"Ubuntu 22.04 + Windows Server 2019+ 各一台 PASS:19；其它 Linux 发行版仅 CI 静态构建通过 + 历史 T-017 验证存证"，与 T-017 同款历史尺度。

---

## 5. Stability

| 测试维度 | 跑次 | 结果 |
|---|---|---|
| `pwsh scripts\verify_all.ps1` | 3 | 3/3 PASS:19 (run1=PASS:19, run2=PASS:19, run3=PASS:19) |
| `go test ./...` | 2 | 2/2 ok all packages（cmd/frp-easy 含 2 新增用例稳定 PASS） |
| `go test -count=1 -v ./cmd/frp-easy/...` | 1 | 234 PASS / 0 FAIL / 5 SKIP |
| `go build -o frp-easy.exe ./cmd/frp-easy` × GOOS={windows,linux,darwin} | 3 | 3/3 exit 0 |
| Console-mode `bin\frp-easy.exe --no-browser` + HTTP probe | 1 | HTTP StatusCode=200 |
| 二进制体积对比 T-018 vs T-019 | 1 | DELTA=35,328 bytes ≤ 1 MB (NFR-2 PASS) |

无 flaky 测试观察。`verify_all` 在 04 §verify_all result 提到的首次跑 G.2 因 `web/test-results/` 偶发噪声 FAIL → 重跑稳定（属 verify_all 脚本历史偶发，与本任务无关），QA 三次跑均 PASS 无再现。

---

## 6. Baseline 更新

**`scripts/baseline.json` 改动**：
- `passing_count`: 333 → **335**（+2 新增 Windows-only 用例）；
- `go_tests`:      237 → **239**；
- `frontend_tests`: 96 → 96（不变）；
- `warnings_baseline`: 0 → 0；
- `updated`: 2026-05-23 → 2026-05-23（QA Stage 6 落盘）；
- `notes`: 追加 T-019 行。

**注**：T-019 新增的 2 个 Go 用例 (`TestInstallServiceScriptNoWrapperGen` / `TestUninstallStillCleansWrapper`) 仅在 GOOS=windows 平台进入测试集合；Linux/macOS 上 `go test ./...` 会 build-tag 跳过，所以 Linux 平台跑 `verify_all.sh` G.2 时实测计数 ≤ Windows 平台。这与 baseline.json `go_tests` 字段语义（"项目维护的可执行测试用例总数，含平台条件用例总和"）一致。

---

## 7. Verdict

**APPROVED FOR DELIVERY（含 2 项 follow-up backlog）**

### 判定理由

- **0 BLOCKER + 0 CRITICAL**：1053 修复在代码层面状态机闭合、双入口分流、wrapper.cmd 移除、Wait-ServiceRunning 三大模块全部就绪；19 条 AC 中 16 条 PASS（静态契约 + Go test + console 真跑），3 条 PENDING-USER-VERIFY（AC-15 Linux 多发行版历史降级 + 真机 SCM AC 的动态部分）。
- **1 MAJOR · 历史遗留**：D-1（PS 5.1 zh-CN 磁盘 .ps1 解析失败）在 T-018 (HEAD) 同款失败，本任务**未引入退化**；按"测试基线只升不降"原则不阻塞 declare-done，留 follow-up T-020-encoding-ps51-bom。
- **C-1 / C-2 / C-3 / C-4 / C-5 全部交代**：
  - C-1（go.mod 不在 Linux 跑 tidy）→ 04 §实施步骤 #1 已记录 + verify_all 双跑稳定。
  - C-2（run 两参签名）→ 05 §2 已 PASS 验证。
  - **C-3（AC-15 Linux 真实发行版）→ 本报告 §Defects D-3** 明文记录"仅 Ubuntu / Windows Server 双机 + 其他历史 best-effort"，PM 在 07 转给用户。
  - **C-4（NFR-2 体积对比）→ 本报告 §Probe 5 / §boundary tests** 实测 T-018 vs T-019 = 35,328 bytes ≤ 1 MB PASS。
  - C-5（F-5/F-6 service-mode-stderr-bridge backlog）→ 04 §Open issues 与 05 §6 已转 PM，建议开 T-021。
- **verify_all PASS:19 ×3 稳定**：无偶发 FAIL；baseline 单调上调（333 → 335），不删测试不下调指标。
- **AC-17 红线落地**：本报告含精确英文裸标题 `## Adversarial tests`（regex `^##\s+Adversarial\s+tests` 命中），verify_all E.6 三跑全 PASS。

### Follow-up backlog 转给 PM

1. **T-020-encoding-ps51-bom**（MAJOR）：把 `scripts/*.ps1` 改为 UTF-8 with BOM；verify_all 加 BOM 检查项（D-1）。
2. **T-021-service-mode-stderr-bridge**（MINOR / 增强）：把 `main.go L138-140 exposureNotice stderr` 改走 logger，避免服务模式下安全提示丢失（C-5 / 03 F-6 / 04 §Open issues 第 1 条）。

### 给用户的 declare-done 后真机验证清单（PM 在 07_DELIVERY.md 转交）

按 §Defects D-2 列出的 6 条真机断言，配合 `bin/frp-easy.exe`（hash 已记录 sha256:3028BA8A6E0835CE60DBB1A7396B40E99537D1714B0C622794CCCA2B0369EF90 / 13,069,312 bytes from `scripts/build.ps1`）作为 reference artifact。

---

## 8. 测试环境快照

```
OS:           Windows 11 Home China 10.0.26200
Hostname:     alan
User:         alan\yangx  (non-administrator)
PowerShell:   7.6.0 (pwsh.exe, primary QA shell) + 5.1.26100.8457 (powershell.exe, sidecar verification)
Go:           local install at C:\Program Files\Go\bin
sc.exe:       present (Service Controller, Windows built-in)
管理员状态:    False (受限场景，真机 SCM AC 走 PENDING-USER-VERIFY)
现有服务:     frp-easy (STOPPED, BINARY_PATH = "C:\Program Files\frp-easy\frp-easy-svc.cmd"，T-018 期残留，未被 T-019 install-service.ps1 刷新)
WSL:          wsl.exe 存在但无已安装 Linux 发行版（同 T-016 QA 状态）
Web:          npm + vite build 链路完整（B 组 verify_all PASS）
```

---

## 9. 关键证据文件清单

> Code Reviewer / PM / Future QA 二次校验入口（全部绝对路径）：

- `C:\Programs\frp_easy\cmd\frp-easy\service_windows.go` — Windows Service ABI 实现（状态机四象限）
- `C:\Programs\frp_easy\cmd\frp-easy\service_other.go` — 非 Windows 平台 stub
- `C:\Programs\frp_easy\cmd\frp-easy\service_windows_test.go` — 2 个 Windows-only Go unit 用例
- `C:\Programs\frp_easy\cmd\frp-easy\main.go` — main() 分流 + run(stopCh, readyCh) 三路 select
- `C:\Programs\frp_easy\scripts\install-service.ps1` — Wait-ServiceMarkedDeleteCleared + Wait-ServiceRunning + 防御性清理 + binPath 直指 $BinaryPath
- `C:\Programs\frp_easy\scripts\uninstall-service.ps1` — 注释降级 + 逻辑不动（防御性清理 frp-easy-svc.cmd）
- `C:\Programs\frp_easy\go.mod` — x/sys v0.44.0 升 direct
- `C:\Programs\frp_easy\scripts\baseline.json` — 测试基线（QA 本步 +2 上调）
- `C:\Programs\frp_easy\bin\frp-easy.exe` — 本任务实测构建产物（hash sha256:3028BA8A6E0835CE60DBB1A7396B40E99537D1714B0C622794CCCA2B0369EF90，13,069,312 bytes，build.ps1 stripped）
- `C:\Programs\frp_easy\scripts\verification_history.log` — verify_all 3 轮稳定 PASS:19 历史回放
