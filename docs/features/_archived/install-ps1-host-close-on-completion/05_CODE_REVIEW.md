# 05 — Code Review · T-031 install-ps1-host-close-on-completion

> Harness 流水线 Stage 5（Code Reviewer）。模式：**full**。
> 上游：04_DEVELOPMENT.md（Verdict=READY FOR CODE REVIEW）+ 03_GATE_REVIEW.md（4 Conditions C-1~C-4）+ 02_SOLUTION_DESIGN.md。
> 输出契约：实际代码 diff 审计（非设计文档审计）+ APPROVED / CHANGES REQUIRED / REJECTED。

## Files reviewed
- `scripts/install.ps1`
- `scripts/verify_all.ps1`
- `scripts/verify_all.sh`
- `scripts/baseline.json`
- `README.md`
- `docs/dev-map.md`

## A. 设计兑现度

- **A1 PASS** — install.ps1 L397-L401：`$global:LASTEXITCODE = 0` 写在 `"@ | Write-Host` 之后、`} @PSBoundParameters` 之前（L402），3 行 T-031 注释完整引用"防前置命令陈旧非零值" + `$global:` scope 修饰符 about_Scopes 文档化。完全与 02 §3.2.1 一致。
- **A2 PASS** — install.ps1 L35-L40：T-024/T-026 注释块后追加 6 行 T-031 注释，引用 insight L44（"& {} 子作用域仅在交互式 console host 下保护"）+ MS `about_PowerShell_Exe` 原文 `-NoExit: "Don't exit after running startup commands"`。
- **A3 PASS** — README L67 `pwsh -NoExit -Command "..."`；L75-L78 PS5.1 段 `powershell -NoExit -Command "..."`；L83 "**如你看到的是旧入口**" 段完整存在。
- **A4 PASS** — verify_all.ps1 L344-L392：E.8/E.9/E.10 三 Step 完整新增，插在 E.7c L336 与 `# --- Summary ---` L394 之间。
- **A5 PASS** — verify_all.sh L341-L381：E.8/E.9/E.10 三 step 同步实现，位置在 E.7c 之后、Summary 之前。
- **A6 PASS** — baseline.json L2 `"version": 14`；L10 notes 追加 T-031 全摘要 + 实测数字 "PASS 21 -> 24 (+3)"。
- **A7 PASS** — docs/dev-map.md L30 scripts/ 块追加 T-031 注解（接 T-026 注解后），含完整改动摘要 + PASS 21→24 实测数字。
- **A8 PASS** — install-service.ps1 / .editorconfig / uninstall-service.ps1 无任何 T-031 改动痕迹（grep 无命中）。OQ-5 a + T-026 决议守住。

## B. Gate Conditions 兑现

- **B1 PASS (C-1)** — verify_all.ps1 L344-L368 E.8 采纳"**单走 Select-String -Path $t -Pattern $pat**"重构（L352）；外层无 `if ($content -match $pat)` 两段式，直接按行过滤注释 + 元描述词。L342-L343 注释明示"避免 Get-Content -Raw 无 (?m) 标志下 `^\s*pause\s*$` 漏报，03 §E15"。**multiline bug 彻底消除**。
- **B2 PASS (C-2)** — baseline.json L10 notes 写 "PASS 21 -> 24 (+3)"（实测），非设计 22→25。
- **B3 PASS (C-3)** — README L85 ">安全提示" 行 + L87 "#### 国内 VM 公网 IP 探测兜底" 段完整保留。
- **B4 PASS (C-4)** — Step 11 已删 .snapshot/；ADV-4 mini repro 落 `.scratch/inner-exit-1.ps1`（任务目录），未污染 scripts/；E.7c WARN=0。

## C. 代码质量

- **C1 PASS** — E.8 注释跳过策略：L353 `$trimmed.StartsWith('#')` 跳整行 # 注释；L357 `_.Line -match '禁|red\.?line|forbidden|FR-3|破\s*FR-3'` 跳元描述词。**潜在 MINOR**：行内尾部注释如 `Write-Host "x" # Read-Host 禁` 不会被第一道规则跳过（trimmed 首字非 #），但**会**被第二道"禁"元描述词命中而跳过——当前生效；如未来注释里不含元描述词的行内尾部注释会假命中。实务上 install.ps1/install-service.ps1 当前无此风险，可接受。
- **C2 PASS** — E.9 verify_all.ps1 L373 `Get-ChildItem -Path "scripts" -File` 默认不递归（无 -Recurse）；模式 `^install.*\.(cmd|bat)$` 含 `^` 锚定；verify_all.sh L362 `find scripts -maxdepth 1` 显式 maxdepth。
- **C3 PASS** — E.10 正则在 README L67-L71 真实结构下命中 Windows 段第一个 powershell 块（Dev 实测 ADV-3 触发 FAIL 验证生效）。
- **C4 PASS** — install.ps1 L40 引用 MS `about_PowerShell_Exe`：`-NoExit "Don't exit after running startup commands"` —— 官方原文准确。
- **C5 PASS** — README "如你看到的是旧入口"段（L83）中文清晰，给出两条过渡指引（改用新字串 / 先开 PS7 prompt 再粘贴旧字串）。
- **C6 PASS** — install.ps1 L1 = `# install.ps1 ...`，首字节非 BOM（dev 实测 `23 20 69` = `# i`）。

## D. 红线复审

- **D1 PASS** — install.ps1 / install-service.ps1 grep `Read-Host|pause|[Console]::ReadKey|Wait-Event` 仅命中 install.ps1 注释行（L37 "除非引入 Read-Host 类阻塞"）—— E.8 元描述词跳过规则消化。无任何实际阻塞调用。
- **D2 PASS** — install.ps1 grep `CmdletBinding` 仅命中 L24/L33/L47 三处禁用说明注释，无任何 `[CmdletBinding()]` 实际声明。
- **D3 PASS** — install.ps1 首字节非 BOM（C6 验）；其他 .ps1 BOM 状态 verify_all E.7a PASS 验证（Step 12 PASS=24）。
- **D4 PASS** — `Glob scripts\install*.{cmd,bat}` 无命中。
- **D5 PASS** — install.ps1 L41-L42 顶层 `param([switch]$Help)` 与 L53-L55 内层 `param([switch]$Help)` 同名同类型（未加新参数，保持 T-026 状态）。

## E. 测试

- **E1 PASS** — ADV-1/2/3 三闸门自测 FAIL 触发正确；ADV-4 stdout "still alive\n1" + ADV-5 stdout "0"，与 02 §5.2 / RISK-D 期望完全吻合。
- **E2 PASS** — Step 12 PASS=24 / WARN=0 / FAIL=0 = 基线 21 + 3，符合 02 §5.1 "+3" 目标（设计 22→25 是估算，03 §B7 已纠正以实测为准）。
- **E3 PASS** — "still alive\n1" 证明 `& script.ps1` call operator 隔离 script scope 让 `exit 1` 不杀外层；`$LASTEXITCODE` 正确透传 = 1。OQ-5 a 安全。
- **E4 PASS** — "0" 证明 `$global:` 修饰符在 `& {}` child scope 内对 `$global:LASTEXITCODE` 的赋值穿透到根 scope。§3.2.1 防御性置零方案安全。

## AC 覆盖矩阵

| AC | 实现位置 | 状态 |
|---|---|---|
| AC-1/2/6/8/12 | T-026 子作用域 + L401 `$global:LASTEXITCODE = 0` 巩固 | 留 QA 真机复测 |
| AC-3/4/7 | install.ps1 L408-L412 失败横幅（T-026 既有，本任务零破坏） | 留 QA 真机复测 |
| AC-5 | verify_all.ps1 E.8 L344 / verify_all.sh L341 | PASS（ADV-1 自测） |
| AC-9 | T-026 既有 -Help 路径 | 未破 |
| AC-10 | verify_all.ps1 E.9 L371 / verify_all.sh L362 | PASS（ADV-2 自测） |
| AC-11 | ADV-4 mini repro 真实跑通 | PASS |
| AC-额外 | verify_all.ps1 E.10 L381 / verify_all.sh L370 | PASS（ADV-3 自测） |

## Findings

### CRITICAL
（无）

### MAJOR
（无）

### MINOR
- **[MAINT]** `verify_all.ps1:357` — E.8 元描述词跳过 regex 依赖未来注释作者继续使用相同元描述词模式（如 `禁` / `forbidden` / `FR-3` / `red.?line`）；如有人写 `Read-Host should not be used`（不含任一元描述词），会误命中 E.8。当前 install.ps1/install-service.ps1 无此风险。NIT 级别，不阻塞。

### NIT
- **[STYLE]** `verify_all.sh:352` — `echo -e $e8_hits` 在路径名含 `\` 时可能扭曲诊断输出（03 §E16），但与项目既有 E.5/E.6/E.7 同款 idiom 对齐，已知 NIT 不改。

## 设计漂移检查
无漂移。所有 02 §3.2 子条目均按设计落地；C-1 推荐的"单走 Select-String"改造是 03 Gate Reviewer 明示推荐路径，非漂移。

## Verdict
**APPROVED**

理由：6 个文件改动 100% 兑现 02 §3.2 设计；4 个 Gate Conditions C-1~C-4 全部满足（其中 C-1 采纳推荐的"单走 Select-String"重构彻底消除 multiline bug）；7 条红线全过；ADV-1~5 + 最终 verify_all PASS=24/WARN=0/FAIL=0 实测对账；OQ-5 a + T-026 + T-029 BOM 决议全保。无 CRITICAL / MAJOR finding；MINOR 1 条为 maintainability 提醒，不阻塞合入。NFR-5 真机 PS7 交互式入口复测留 QA 06 ADV-5 强制段。

—— Code Reviewer, 2026-05-24
