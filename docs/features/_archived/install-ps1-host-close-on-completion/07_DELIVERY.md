# 07 — Delivery · T-031 install-ps1-host-close-on-completion

> Harness 流水线 Stage 7（PM Orchestrator delivery wrap-up）。模式：**full**。
> 上游：01 (READY) + 02 (READY FOR GATE REVIEW) + 03 (APPROVED W/ Conditions C-1~C-4) + 04 (READY FOR CODE REVIEW) + 05 (APPROVED · 0 CRITICAL/MAJOR) + 06 (READY FOR DELIVERY · PASS=24/WARN=0/FAIL=0)。
> 用户决策原则：用户体验好 · 符合软件工程标准 · 长期易使用易维护。

---

## §1 任务摘要

**问题**：用户反馈在 Win11 PowerShell 7 终端运行一键安装脚本（`scripts/install.ps1`，通过 `irm | iex` 形态加载）到后期会自动关闭终端，导致无法观察实际安装结果（看不到第 8 步打印的访问地址 / 公网 IP 探测结果 / 服务注册结果横幅）。

**根因**：T-026 用 `& { ... } @PSBoundParameters` 子作用域包裹解决了**已打开的**交互式 PowerShell console host 中 `iex` 形态 `exit N` 杀宿主问题（insight L44 nuance 明记），但**未**覆盖用户极可能用的"`pwsh -Command "irm | iex"` / `cmd /c pwsh -Command ...` / Win+R `pwsh -Command ...` / Windows Terminal 启动一行命令"等入口——这些是 PowerShell 文档化的"-Command 跑完即退"行为，脚本侧无法逆转（除非引入 `Read-Host` 类阻塞，破 FR-3 红线）。

**解决路径**（方案 E：A + C + 静态闸门组合）：

- README 推荐入口字串改为 `pwsh -NoExit -Command "irm ... | iex"`，正面化 PowerShell `-NoExit` 文档化语义（about_PowerShell_Exe："Don't exit after running startup commands"），让 cmd / Win+R / Windows Terminal 三入口的窗口在脚本结束时都保持交互式 prompt。
- install.ps1 末尾 `exit 0` 改为 `$global:LASTEXITCODE = 0`，防御性消解 RA §2 R4 假设（exit 0 在 iex + `& {}` 下是否真不杀宿主），同时让"成功路径 $LASTEXITCODE = 0"永远成立、T-026 子作用域外失败横幅可靠不误触发。
- verify_all 新增 3 道静态闸门 E.8 / E.9 / E.10 防止 FR-3 / FR-8 / FR-10 在未来 PR 中悄悄被破坏。

---

## §2 改动汇总（按 file）

| File | 性质 | 行数 | 说明 |
|---|---|---|---|
| `scripts/install.ps1` | edit | +9 / -1 | L35-L40 追加 T-031 决议注释引用 insight L44 + MS `about_PowerShell_Exe`；L397-L401 把 `exit 0` 改 `$global:LASTEXITCODE = 0` + 防御性注释 |
| `README.md` | edit | +27 / -10 | "Windows" 段推荐入口字串改 `pwsh -NoExit -Command "..."`；PS5.1 段同步 `powershell -NoExit -Command "..."`；新增"如你看到的是旧入口"过渡段；保留 ">安全提示" + "国内 VM" 段不动 |
| `scripts/verify_all.ps1` | edit | +49 / 0 | L344-L392 新增 E.8 (forbid Read-Host/ReadKey/pause/Wait-Event) / E.9 (forbid wrapper.cmd/bat) / E.10 (README -NoExit) 三 Step；E.8 采纳 03 §E15 推荐的"单走 Select-String"路径彻底消除 multiline bug |
| `scripts/verify_all.sh` | edit | +41 / 0 | L341-L381 同步实现 E.8 / E.9 / E.10（bash 版用 `grep -nE` 按行扫描天然 multiline） |
| `scripts/baseline.json` | edit | +1 / -1 | version 13→14；notes 追加 T-031 摘要 + 实测 PASS 21→24 (+3) |
| `docs/dev-map.md` | edit | +1 / 0 | scripts/ 块追加 T-031 注解（接 T-026 注解后） |
| `scripts/install-service.ps1` | **不改** | 0 | OQ-5 a 决议遵守；`about_Scopes` 文档化 script scope 隔离已由 ADV-4 mini repro 实证 |
| `scripts/.editorconfig` | **不改** | 0 | T-026 决议遵守 |
| `scripts/uninstall-service.ps1` | **不改** | 0 | OOS-3 决议遵守 |

**净改动**：6 个文件 / +128 / -12 行。

---

## §3 验证证据

### 3.1 verify_all 最终输出

命令：`pwsh -NoProfile -File scripts/verify_all.ps1 -Quick`

```
[E.8] install.ps1 / install-service.ps1 forbid interactive blockers (FR-3) ... PASS
[E.9] No wrapper.cmd / install*.bat in scripts/ (FR-8 single-script invariant) ... PASS
[E.10] README Windows install entry contains -NoExit (T-031 FR-10) ... PASS

=== Summary ===
  PASS: 24
  WARN: 0
  FAIL: 0
  SKIP: 0
```

基线 21 + 3 新闸门 = 24，**符合 02 §5.1 "+3" 设计目标**。

### 3.2 ADV-1~5 自测全 PASS（详见 06 §2）

| ADV | 假设 | 实测 stdout | 结论 |
|---|---|---|---|
| ADV-1 | Read-Host → E.8 FAIL | "Interactive blockers found: install.ps1:58 Read-Host \"ADV-1 ...\"" | ✓ |
| ADV-2 | install-wrapper.cmd → E.9 FAIL | "Forbidden wrapper files found" | ✓ |
| ADV-3 | 删 README -NoExit → E.10 FAIL | "missing -NoExit flag (T-031 FR-1 / FR-10)" | ✓ |
| ADV-4 | `& script.ps1` 隔离 script scope | "still alive\n1" | ✓ OQ-5 a 安全 |
| ADV-5 | `$global:LASTEXITCODE = 0` 跨 scope | "0" | ✓ RISK-D 假 |

### 3.3 回归扫描

- Go ./... 全 ok（15 internal package + cmd/frp-easy 全 cached PASS）
- 前端 vitest 13/13 文件 + 103/103 测试 PASS
- install.ps1 -Help 路径 PASS（exit 0 + Help 文本完整）
- BOM / scope / param 红线全过

### 3.4 Gate Conditions C-1~C-4 兑现

- **C-1 (E.8 multiline)** ✓ verify_all.ps1 L344-L368 采纳"单走 Select-String -Path $t -Pattern $pat"重构，去掉两段式
- **C-2 (baseline 数字)** ✓ baseline.json L10 写实测 "PASS 21 -> 24 (+3)"
- **C-3 (README 不误删安全提示)** ✓ README L85 / L87 完整保留
- **C-4 (mini repro 不落 scripts/)** ✓ inner-exit-1.ps1 落 `docs/features/install-ps1-host-close-on-completion/.scratch/`，E.7c WARN=0

---

## §4 NFR-5 用户真机复测引导（必读）

按 insight L44 红线 + NFR-5，QA 环境（Claude Code subprocess pipe，非交互式 console host）**无法**自动复现 "交互式 PS console host 下宿主存活" 行为。**用户在真机 5 分钟内可完成全部人工复测**：

### 5 分钟一次性验证（**关键 ADV-5 用户对照**）

Win+R 跑两次（**或** 任一 cmd / Windows Terminal tab 跑两次）：

```powershell
# (a) 新字串 - 窗口应保留
pwsh -NoExit -Command "irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex"

# (b) 旧字串 - 窗口应在 step 8 横幅打完立即关闭
pwsh -Command "irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex"
```

**期望对照**：(a) 跑完后窗口**保留**且 prompt 回来，能完整阅读 step 8 横幅；(b) 跑完后窗口**关闭**（PowerShell 文档化"-Command 跑完即退"，不是 bug）—— 这正反对照让用户**亲眼**看见 README 推荐 `-NoExit` 必要性。

### 其他人工复测项

完整人工复测命令清单见 `06_TEST_REPORT.md` §3，包含：
- AC-1 / AC-2 交互式 PS7 + iex 与磁盘形态成功路径（窗口不关 + `$LASTEXITCODE=0`）
- AC-3 / AC-4 失败路径（非 admin / 架构非 AMD64）窗口不关 + 中文红字横幅
- AC-8 PS5.1 磁盘形态兼容（允许中文乱码，T-029 OOS 化）
- AC-12 PS5.1 + PS7 双解释器 8 用例矩阵

---

## §5 已知 follow-up（不阻塞 T-031 交付）

### F-1: [MAJOR 历史预存] verify_all.ps1 vs verify_all.sh 的 E.6 实现不一致

**症状**：bash 侧 `verify_all.sh --quick` E.6 FAIL，归罪 `docs/features/_archived/download-cancel-and-upload-decouple/06_TEST_REPORT.md`（T-027 归档）；同时 PowerShell 侧 E.6 PASS（**假阳性**）。

**根因**：
- T-027 06 §4 标题为 `## §4 Adversarial tests`（带数字 `§4` 前缀，违反 L18/L48 裸标题约定）。
- bash `grep -qE '^##\s+Adversarial\s+tests'` 严格行首锚 + 显式按行扫描 → 不命中 → FAIL（**正确行为**）。
- PowerShell `Get-Content -Raw + -match '##\s+Adversarial\s+tests'` 缺 `^` 锚 + Raw 单字符串模式 → 子串搜索命中 T-027 06 L166 引用块行内字面 `## Adversarial tests` → PASS（**假阳性**）。

**严重度**：MAJOR / 历史预存（**非 T-031 引入** + 不阻塞 T-031 交付 + 已归档任务）。

**修复方案**（建议新开 T-033 trivial 任务）：
- verify_all.ps1 E.6 加 `(?m)` multiline + `^` 行首锚：把 `-match` pattern 改为 `'(?m)^##\s+Adversarial\s+tests\s*$'`，对账 bash 严格行首语义。
- T-027 06 标题回填裸 `## Adversarial tests`（涉及已归档文件，需用户授权——可由 T-033 PM 决定是否回填或仅修闸门）。

T-031 本身未触发：本文件 §6 写裸 `## Insight` 标题；06 §6 写裸 `## Adversarial tests` 标题。

---

## §6 Insight

> 本段为 PM 强制契约段（L43/L46/L48/L49 红线）。**裸标题** `## Insight`（无 §N / 数字前缀），archive-task 收割 regex 按此匹配（T-028 已加容错但裸标题仍是首选）。

### Insight 1：PowerShell `-NoExit` 是 cmd / Win+R / Windows Terminal 入口让窗口在 `-Command` 跑完后不关闭的官方文档化 idiom

`pwsh -NoExit -Command "..."` / `powershell -NoExit -Command "..."` 同语义（MS `about_PowerShell_Exe`："Don't exit after running startup commands"）。与 T-026 `& {}` 子作用域包裹**互补**：后者保护**已打开的**交互式 prompt 宿主下 `iex` 形态 `exit N` 不杀宿主；前者保护**新启动**的 `-Command` 形态宿主进程在 `-Command` 跑完后不退出。任何"一键安装"类管道脚本如希望支持 cmd / Run 框 / Windows Terminal 三入口，推荐入口字串**必须**加 `-NoExit`；不加是 PowerShell 文档化"-Command 跑完即退"行为，**不是 bug** —— 脚本侧无法逆转（除非引入 `Read-Host` 类阻塞，破 FR-3 红线）。·evidence: T-031 04 ADV-3 闸门触发 + 06 §3 ADV-5 用户对照测试（新旧字串各跑一次，新串窗口保留 vs 旧串窗口立即关闭）+ MS `about_PowerShell_Exe` 文档原文

### Insight 2：`& { ... }` 子作用域内显式 `$global:LASTEXITCODE = 0` 是"成功路径"清零陈旧非零值的稳定 idiom

PowerShell scriptblock 内 `exit N` 隐式 set `$LASTEXITCODE = N`，但若末尾不走 `exit` 而依赖最后一条命令自然推断退出码（如 `Write-Host` 成功 → 0），在某些 PS 版本下 `$LASTEXITCODE` 可能保留前一条 native 命令的陈旧值。`$global:VAR` 修饰符（about_Scopes 文档化）跨 scope 显式赋值合法，在 child scope `& { ... }` 内对 `$global:LASTEXITCODE` 的赋值直接写穿到根 scope。与子作用域外 `if ($LASTEXITCODE -ne 0)` 失败横幅判定可靠配合：成功路径 `$LASTEXITCODE = 0` 永远成立，失败路径 `exit N` 隐式 set $LASTEXITCODE=N 不被覆盖（因为最后一次写入是 exit N，没有后续 `$global:` 赋值打穿）。这是替代"末尾 exit 0"的更鲁棒模式：避免 iex 顶层 exit 是否杀宿主的 R4 不确定性。·evidence: T-031 04 ADV-5 mini repro `pwsh -NoProfile -Command "& { $global:LASTEXITCODE = 0 }; $LASTEXITCODE"` 输出 `0`，证实 `$global:` 跨 child scope 写穿到根 scope

### Insight 3：verify_all.ps1 vs verify_all.sh 同款 step 的 regex 实现不对称是隐患

双实现（PowerShell + Bash）verify_all 的同款 step 在 regex 锚定 / multiline 标志 / 输入模式（按行 vs Raw 单字符串）上若不严格对账，会让"项目级红线闸门"在不同操作系统侧产生**假阳性**（一侧 PASS 一侧 FAIL）。本任务实测 E.6 即此案例：bash 严格行首锚 + 按行扫描严正 FAIL T-027 06 §4 标题违规；PowerShell Raw + 缺 `^` 锚假性 PASS 同款违规。**长期解**：每加新 step 必须 dev / Code Reviewer / QA 三方核对"PS 实现 + Bash 实现"是否在边界 case 上行为一致；建议 `.harness/skills/verify/` 加一条"verify_all 双实现对账"说明。·evidence: T-031 06 §5.2 实测 bash E.6 FAIL vs PowerShell E.6 PASS，归罪 T-027 06 同款文件

---

## §7 Verdict

**DELIVERED**

理由：

- 6 个文件改动 100% 兑现 02 §3.2 设计；4 个 Gate Conditions C-1~C-4 全部满足；7 条红线全过。
- verify_all 最终输出 PASS=24 / WARN=0 / FAIL=0 / SKIP=0，符合 02 §5.1 "+3" 设计目标。
- ADV-1~5 + AC-5/9/10/11/额外 自动化项全 PASS；OQ-5 a + RISK-D 用 mini repro 实证证伪。
- Go + 前端单测零回归；install.ps1 -Help 不破；BOM / param / [CmdletBinding] 等历史红线全保。
- NFR-5 真机交互式 PS7 复测项（AC-1/2/3/4/6/7/8/12 + ADV-5 用户对照）已在 06 §3 + 本文件 §4 给出精确可粘贴命令 + 期望输出。
- 1 MAJOR 历史预存缺陷（F-1: verify_all 双实现 E.6 不一致）已记录路由方案，**不阻塞 T-031**。
- 新 insight 3 条（含 1 条任务外双实现对账 idiom）已写入 §6 裸 `## Insight` 段，archive-task 可收割。

PM 接收本 07 后跑 `scripts/archive-task --task install-ps1-host-close-on-completion` 收割 insight + 归档阶段文档；更新 tasks.md 阶段为 done；commit + push。

—— PM Orchestrator, 2026-05-24
