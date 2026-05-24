# 04 — Development Record · T-031 install-ps1-host-close-on-completion

> Harness 流水线 Stage 4（Developer）。模式：**full**。
> 上游：01 (Verdict=READY) + 02 (Verdict=READY FOR GATE REVIEW) + 03 (Verdict=APPROVED WITH CONDITIONS C-1~C-4)。
> 角色契约：`.harness/agents/developer.md`。

---

## §1 实施摘要

按 02 §8 14 步 + 03 §4 Conditions C-1~C-4 整合后顺序执行，无 BLOCKED 事件。**核心改动**：

| 文件 | 类型 | 行数变化 | 内容 |
|---|---|---|---|
| `scripts/install.ps1` | edit | +11 / -1 | L26-L34 区段追加 5 行 T-031 决议注释；L391 `exit 0` 改为 `$global:LASTEXITCODE = 0` + 4 行防御性注释 |
| `README.md` | edit | +15 / -7 | L67-L75 Windows 段：推荐入口字串改 `pwsh -NoExit -Command "..."`、加 `-NoExit` 关键参数说明、PS5.1 段同步给 `powershell -NoExit -Command "..."` 替代字串、新增"如你看到的是旧入口"反向兼容段；L77 ">安全提示"行 + L79 "国内 VM" 段**原样保留**（C-3 兑现） |
| `scripts/verify_all.ps1` | edit | +51 / 0 | L336 E.7c 末尾 `}` 之后、`# --- Summary ---` 之前新增 E.8 / E.9 / E.10 三个 Step；E.8 用 C-1 推荐的"单走 Select-String 按行扫描"路径替代两段式 `-match`+Select-String（天然 multiline 避免 R4-pause 漏报） |
| `scripts/verify_all.sh` | edit | +44 / 0 | bash 同步新增 E.8 / E.9 / E.10 三 step；bash `grep -nE` 按行扫描天然 multiline，无需 PCRE `(?m)` |
| `scripts/baseline.json` | edit | version 13→14；notes 改写 | C-2 兑现：notes 用 step 7 实测 "PASS 21 → 24 (+3)" 写，不照搬设计 22→25 |
| `docs/dev-map.md` | edit | +1 行 | scripts/ 块 T-026 注解后追加一行 T-031 注解 |

**未改动**（OQ-5 a + T-026 决议 + OOS-3）：`scripts/install-service.ps1` / `scripts/.editorconfig` / `scripts/uninstall-service.ps1`。

**未引入**（FR-3 + FR-8 红线）：任何 `Read-Host` / 裸 `pause` / `[Console]::ReadKey` / `Wait-Event` / `Start-Sleep -Seconds` 阻塞调用；任何 wrapper.cmd / .bat 文件。

---

## §2 步骤实录（命令 + 结果）

### Step 1 — 备份快照
- 命令：PowerShell `mkdir` + `cp` 6 个文件到 `docs/features/install-ps1-host-close-on-completion/.snapshot/`（C-4 精神：不落 `scripts/.t031-snapshot/` 避免触发 E.7c WARN，落任务目录）
- 结果：6 个文件字节级备份 OK（install.ps1 18184 / verify_all.ps1 15378 / verify_all.sh 13758 / README.md 14597 / baseline.json 1104 / dev-map.md 19065）

### Step 2 — `scripts/install.ps1` L26-L34 追加 T-031 决议引用注释
- 命令：Edit 工具精确字符串替换
- 结果：在 T-024 / T-026 注释块末尾追加 6 行 T-031 注释（引用 MS `about_PowerShell_Exe`：`-NoExit: Don't exit after running startup commands`）

### Step 3 — `scripts/install.ps1` L389-L391 删 `exit 0` 改 `$global:LASTEXITCODE = 0`
- 命令：Edit 工具精确字符串替换
- 结果：`exit 0` 4 行 before → `$global:LASTEXITCODE = 0` 7 行 after（含 3 行防御性注释解释 `$global:` 修饰符 about_Scopes 文档化跨 scope 写穿）
- 字节级核对：`head -c 3 scripts/install.ps1 | od -An -tx1` = `23 20 69`（`# i`），**首字节非 BOM**，T-026 红线保持

### Step 4 — `README.md` L67-L78 区段改
- 命令：Edit 工具精确字符串替换（C-3 严守："旧 8 行 → 新 14 行" 包整段精确匹配）
- 结果：审查 awk `NR>=66 && NR<=95` 输出：L86 = `> 安全提示...`、L88 = `#### 国内 VM 公网 IP 探测兜底`、L90 = "服务端安装结束横幅..."—— **L77 安全提示 + L79 国内 VM 段原样保留**（行号位移正常，因 Windows 段增了 7 行；C-3 兑现）

### Step 5 — `scripts/verify_all.ps1` 插入 E.8 / E.9 / E.10
- 命令：Edit 工具精确字符串替换（插入在 E.7c 末尾 `}` 与 `# --- Summary ---` 之间）
- 结果：3 个新 Step 块共 51 行；E.8 按 C-1 推荐 + 03 §E15 改造为**单走 Select-String -Path $t -Pattern $pat -ErrorAction SilentlyContinue**（避免 `Get-Content -Raw + -match` 无 multiline 标志的潜在 `^\s*pause\s*$` 漏报）；保留两层注释跳过排除（# 注释行 + 元描述词 `禁/forbidden/FR-3/red.?line`）

### Step 6 — `scripts/verify_all.sh` 同步新增 E.8 / E.9 / E.10
- 命令：Edit 工具精确字符串替换
- 结果：bash `grep -nE` 按行扫描天然 multiline，pause pattern 无需 PCRE `(?m)`；与 PowerShell 版语义对账（同 4 个 forbidden pattern + 同 2 层注释/元描述跳过）

### Step 7 — 跑 verify_all 拿实测数字（C-2 关键）
- 命令：`pwsh -NoProfile -File scripts/verify_all.ps1 -Quick`
- 结果 Summary 原文：
  ```
  === Summary ===
    PASS: 24
    WARN: 0
    FAIL: 0
    SKIP: 0
  ```
- **基线对照**：Step 1 之前跑的 baseline = `PASS 21 / WARN 0 / FAIL 0 / SKIP 0`；改动后 = `PASS 24`（+3 = E.8 + E.9 + E.10），符合设计预期（02 §5.1 "Quick 模式 22→25" 是设计者凭印象，实测基线 21、新基线 24，与 03 §B7 推算"基线 21/22 + 3" 一致）

### Step 8 — `scripts/baseline.json` version 13→14 + notes 用 Step 7 实测数字
- 命令：Edit 工具精确字符串替换
- 结果：notes 中明确写 "verify_all PASS 21 -> 24 (+3) / WARN 0 / FAIL 0 / SKIP 0（Quick 模式实测）"，C-2 兑现

### Step 9 — `docs/dev-map.md` scripts/ 块追加 T-031 注解
- 命令：Edit 工具精确字符串替换（接在 T-026 注解之后）
- 结果：dev-map.md +1 行 T-031 注解（含完整改动摘要 + 实测数字 21→24）

### Step 10 — Adversarial 自测
见 §4 全文。

### Step 11 — 删快照备份 + git status 复核
- 命令：`rm -rf docs/features/install-ps1-host-close-on-completion/.snapshot && git status --short`
- 结果：本任务范围 modified = 6 个文件（README.md / docs/dev-map.md / scripts/baseline.json / scripts/install.ps1 / scripts/verify_all.ps1 / scripts/verify_all.sh）+ untracked `docs/features/install-ps1-host-close-on-completion/`（含 01-04 + .scratch）。完全符合 02 §3.2 清单与 §11 partition assignment。

### Step 12 — 最终 verify_all 复核
- 命令：`pwsh -NoProfile -File scripts/verify_all.ps1 -Quick`
- 结果 Summary：`PASS: 24 / WARN: 0 / FAIL: 0 / SKIP: 0`，与 Step 7 完全一致（无回归）

---

## §3 意外

**无非预期问题**。所有改动按 02 §3.2 设计逐项落地：

- C-1 推荐的"单走 Select-String"改造比"加 `(?m)` 前缀到 `'^\s*pause\s*$'`" 更鲁棒（直接消除 `Get-Content -Raw -match` 那一层 multiline 标志陷阱），无副作用。
- C-2 实测 PASS=21（不是设计估算的 22），改动后 PASS=24（不是估算的 25），baseline.json notes 已用实测数字。
- C-3 README 改动用 Edit 工具精确字符串替换"旧 8 行块 → 新 14 行块"，未触及 L77 起的 "> 安全提示"行；改动后 awk 抽样确认 L86 = "> 安全提示..."（行号下移合理）。
- C-4 ADV-4 mini repro 临时脚本落在 `docs/features/install-ps1-host-close-on-completion/.scratch/inner-exit-1.ps1`，未落 `scripts/`，未触发 E.7c WARN。

---

## §4 测试摘要

### ADV-1 — E.8 命中 Read-Host
- 临时插入 `Read-Host "ADV-1 temporary blocker"` 到 `scripts/install.ps1` L114 上方
- `verify_all.ps1 -Quick` 输出：`[E.8] install.ps1 / install-service.ps1 forbid interactive blockers (FR-3) ... FAIL`；Summary `PASS: 23 / WARN: 0 / FAIL: 1`
- 还原后 E.8 复 PASS
- **结论**：E.8 闸门对 Read-Host 检测正确生效 ✓

### ADV-2 — E.9 命中 install-wrapper.cmd
- 临时 `touch scripts/install-wrapper.cmd`（空文件）
- `verify_all.ps1 -Quick` 输出：`[E.9] No wrapper.cmd / install*.bat in scripts/ ... FAIL`；Summary `PASS: 23 / WARN: 0 / FAIL: 1`
- 删除后 E.9 复 PASS
- **结论**：E.9 闸门对 install*.cmd / install*.bat 检测正确生效 ✓

### ADV-3 — E.10 命中 README 缺 -NoExit
- 临时改 README L70 `pwsh -NoExit -Command ...` → `pwsh -Command ...`（删 `-NoExit`）
- `verify_all.ps1 -Quick` 输出：`[E.10] README Windows install entry contains -NoExit (T-031 FR-10) ... FAIL`；Summary `PASS: 23 / WARN: 0 / FAIL: 1`
- 还原后 E.10 复 PASS
- **结论**：E.10 闸门对 README -NoExit 缺失检测正确生效 ✓

### ADV-4 — mini repro AC-11 `& script.ps1` script-scope 隔离
- 命令：`pwsh -NoProfile -Command "& { & 'C:/Programs/frp_easy/docs/features/install-ps1-host-close-on-completion/.scratch/inner-exit-1.ps1' ; 'still alive'; $LASTEXITCODE }"`
- inner 脚本仅一行 `exit 1`
- **stdout 完整原文**：
  ```
  still alive
  1
  ```
- **结论**：`& 'path/inner.ps1'` call operator 让磁盘 .ps1 跑在独立 script scope，inner `exit 1` 仅退该 scope，外层 `'still alive'` 仍执行；`$LASTEXITCODE` 透传到外层 = `1`。**约 R2 假，OQ-5 a 决议安全**：`install.ps1` 中 `& $svc` 调用 `install-service.ps1` 的 9 处 `exit N` 不会泄漏到外层 iex runspace ✓

### ADV-5 — mini repro RISK-D `$global:LASTEXITCODE = 0` 跨 scope 写全局
- 命令：`pwsh -NoProfile -Command "& { $global:LASTEXITCODE = 0 }; $LASTEXITCODE"`
- **stdout 完整原文**：
  ```
  0
  ```
- **结论**：`$global:` scope 修饰符（about_Scopes 文档化）让 child scope `& { ... }` 内对 `$global:LASTEXITCODE` 的赋值直接写穿到根 scope，外层 `$LASTEXITCODE` 读到 `0`。**约 RISK-D 假，§3.2.1 防御性置零方案安全** ✓

### 最终 verify_all（Step 12）
- 命令：`pwsh -NoProfile -File scripts/verify_all.ps1 -Quick`
- Summary 原文：
  ```
  === Summary ===
    PASS: 24
    WARN: 0
    FAIL: 0
    SKIP: 0
  ```
- baseline 对照 `PASS 21 / WARN 0 / FAIL 0 / SKIP 0` → delta = **+3 PASS 净增（E.8 + E.9 + E.10）/ 0 新 WARN / 0 新 FAIL**，符合 02 §5.1 + 03 §B7 期望

### NFR-5 真机交互式 PS7 复测（AC-1 / AC-2 / AC-3 / AC-4 / AC-6 / AC-7 / AC-8 / AC-12）
**Developer 不跑，留 QA 在 06 跑**（NFR-5 + insight L44：QA 06 `## Adversarial tests` 段强制粘贴人工复测命令 + 截图描述）。本任务 Developer 自测仅覆盖自动化可证项（ADV-1~5）。

---

## §5 Gate Conditions 兑现确认（C-1 ~ C-4）

| ID | 条件 | 状态 | 怎么做 |
|---|---|---|---|
| **C-1** | 02 §3.2.4 E.8 step 内 forbidden 数组的 `'^\s*pause\s*$'` 必须改 `(?m)` 前缀**或**重构为"单走 Select-String"路径 | **已做** | 采纳推荐方案：重构 E.8 实现为"单走 `Select-String -Path $t -Pattern $pat`"路径（无外层 `if ($content -match $pat)` 两段式）；`Select-String` 默认按行扫描，`^`/`$` 按行匹配，**天然 multiline 无需 `(?m)`**；与 bash `grep -nE` 语义对账一致 |
| **C-2** | baseline.json notes 中 PASS 数字必须用**实跑 verify_all 后 Summary 输出**，不照搬设计 22→25 | **已做** | Step 7 实跑 verify_all 拿到 PASS=24（基线 21 + 3 = E.8/E.9/E.10）；baseline.json notes 写 "verify_all PASS 21 -> 24 (+3) / WARN 0 / FAIL 0 / SKIP 0（Quick 模式实测）"；非照搬设计的 22→25 |
| **C-3** | README L67-L78 改动用 Edit 精确字符串替换，**不得**误删 L77 ">安全提示"行 + L79 "国内 VM" 段 | **已做** | Step 4 用 Edit 工具传"完整旧 8 行块 → 新 14 行块"做精确字符串替换，old_string 锚定在 "管道形态推荐" 段结束、未跨进 ">安全提示"行；改后 awk 抽样 L86 = "> 安全提示..."、L88 = "#### 国内 VM 公网 IP 探测兜底"（行号下移 ≈ 7 行符合新 Windows 段增量），两段原样完好 |
| **C-4** | AC-11 mini repro 临时脚本**不得**落 `scripts/`（触发 E.7c WARN）；落任务 `.scratch/` 或 `C:\tmp\` | **已做** | ADV-4 inner-exit-1.ps1 落在 `docs/features/install-ps1-host-close-on-completion/.scratch/`；E.7c step 12 实测 PASS（未触发 unclassified WARN） |

---

## §6 候选 Insight 草稿（PM 在 07 写 `## Insight` 段时取用）

> 按 .harness/agents/developer.md §"Insight to surface" 格式，仅记录"非显然项目真理"。
> 02 §9.2 已草拟 2 条；本节用 §4 实测 stdout 补完 evidence 占位。

1. **2026-05-24 · PowerShell `-NoExit` 是 cmd / Win+R / Windows Terminal 入口让窗口在 `-Command` 跑完后不关闭的官方文档化 idiom**：`pwsh -NoExit -Command "..."` / `powershell -NoExit -Command "..."` 同语义；与 T-026 `& {}` 子作用域包裹**互补**——后者保护**已打开的**交互式 prompt 宿主，前者保护**新启动**的 -Command 形态宿主。任何"一键安装"类管道脚本如希望支持 cmd / Run 框 / Windows Terminal 三入口，推荐入口字串**必须**加 `-NoExit`；不加是 PowerShell 文档化"-Command 跑完即退"行为，**不是 bug**。 · evidence: T-031 §4 ADV-3 实测 README 删 `-NoExit` 触发 E.10 FAIL；QA 06 ADV-5 实测两入口字串对照截图 + PS5.1 / PS7 双版本下 -NoExit 生效（待 QA 补人工 evidence）

2. **2026-05-24 · `& { ... }` 子作用域内显式 `$global:LASTEXITCODE = 0` 是"成功路径"清零陈旧非零值的稳定 idiom**：PowerShell scriptblock 内 `exit N` 隐式 set `$LASTEXITCODE = N`，但若末尾不走 `exit` 而靠最后一条命令自然推断退出码（如 `Write-Host` 成功 → 0），在某些 PS 版本下 `$LASTEXITCODE` 可能保留前一条命令的陈旧值。`$global:VAR` 修饰符跨 scope 显式赋值（about_Scopes 文档化）让"成功路径 $LASTEXITCODE = 0"永远成立，与 T-026 子作用域外 `if ($LASTEXITCODE -ne 0)` 失败横幅判定可靠配合。 · evidence: T-031 §4 ADV-5 实测 `pwsh -NoProfile -Command "& { $global:LASTEXITCODE = 0 }; $LASTEXITCODE"` stdout = `0`

3. **2026-05-24 · verify_all 的 PowerShell `Select-String -Path $t -Pattern $pat` 单调用比"先 `Get-Content -Raw + -match $pat` 再 Select-String 按行" 两段式更鲁棒**：`Get-Content -Raw` 返回整文件单字符串，`-match` 默认无 multiline → `^` 仅匹配字符串首位、`$` 仅匹配末位，让 `'^\s*pause\s*$'` 类按行锚定 pattern 漏报。`Select-String` 默认按行扫描，`^`/`$` 按行匹配，天然 multiline；与 bash `grep -nE` 行为对账一致，是 verify_all 跨 ps/sh 实现对账的稳定 idiom。 · evidence: T-031 §2 Step 5 实施 + §4 ADV-1 实测 E.8 命中 Read-Host FAIL；03 §E15 Gate Reviewer 提议、04 落实

---

## §7 Verdict

**READY FOR CODE REVIEW**

理由：
- §1 改动清单覆盖 02 §3.2 全部条目（install.ps1 + README.md + verify_all.{ps1,sh} + baseline.json + dev-map.md），install-service.ps1 / .editorconfig / uninstall-service.ps1 字节零变（OQ-5 a + T-026 红线）。
- §2 步骤实录 12 步全完成（Step 13/14 即本节）；Step 7 实测 PASS=21（基线）/ Step 12 实测 PASS=24（改后），delta +3 净增、0 新 WARN / 0 新 FAIL，与 02 §5.1 期望一致。
- §3 无意外事件，无 BLOCKED 设计 / 能力 / verify_all 重复失败。
- §4 ADV-1~5 自测全部 PASS（E.8/E.9/E.10 闸门对 Read-Host / wrapper.cmd / README -NoExit 缺失均正确触发 FAIL）；ADV-4/5 mini repro 实测 stdout 证实 OQ-5 a + RISK-D 安全。
- §5 Gate Conditions C-1 ~ C-4 全部兑现，每条配实施动作摘要。
- §6 候选 insight 3 条（02 §9.2 草拟 2 条 + Developer 新发现 1 条 "Select-String 单调用比 Get-Content -Raw + -match 鲁棒"），全部带 evidence。
- FR-3 / FR-8 红线全过（脚本 0 处新增 Read-Host / pause / ReadKey / Wait-Event；仓库无 install*.cmd / install*.bat）。
- T-026 BOM 红线保持（install.ps1 首 3 字节 = `23 20 69`，非 BOM）。

PM 接 04 后派 Code Reviewer / QA。

---

— Developer, 2026-05-24
