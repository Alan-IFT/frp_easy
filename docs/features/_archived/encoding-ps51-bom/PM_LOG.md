# PM Log · T-021 encoding-ps51-bom

> PM Orchestrator 派发记录 + 阶段切换 + 已知陷阱预警。

## Stage 0 · 任务受理

**时间**：2026-05-23
**模式**：full（7-stage）
**触发**：用户审视项目 follow-up，PM 决策本任务 = MAJOR UX 修复，价值最高。
**先验前置**：T-019 / T-020 / T-020-followup 已 push 到 origin/main，工作树干净，verify_all PASS:19 基线稳定。

## Stage 1 · Requirement Analyst

**状态**：DONE · Verdict = READY
**产物**：`01_REQUIREMENT_ANALYSIS.md`（18 条 AC + 5 项 I-1~I-5 + 8 项给 SA 的决议清单）。

## Stage 2 · Solution Architect（轮次 1）

**状态**：DONE · Verdict = READY-FOR-GATE-REVIEW
**产物**：`02_SOLUTION_DESIGN.md` 轮次 1（含 §1~§10）。

## Stage 3 · Gate Reviewer

**状态**：DONE · Verdict = CHANGES REQUIRED
**产物**：`03_GATE_REVIEW.md`（PM 接管落盘，insight L42 已知）。
**4 项必须修改**：
- C-1: `working-tree-encoding=UTF-8-BOM` 不是 git iconv 合法值
- C-2: `ReadAllText` 需 `throwOnInvalidBytes=$true`
- C-3: AC-10 `iex` BOM 吞咽 mock 在 PS5.1 不可靠
- C-4: PS 伪码 `$root` startsWith guard

## Stage 2 · Solution Architect（轮次 2 差异修订）

**状态**：DONE · Verdict = REVISED, ready for differential gate review
**产物**：`02_SOLUTION_DESIGN.md` 轮次 2（§11 修订历史段落记录 7 处 before/after）。
**PM 差异审查**：grep `02` 7 处修订全部到位（throwOnInvalidBytes、StartsWith、.editorconfig、UTF8Encoding($true)、AC-10 合并 AC-9）。APPROVED（PM 自行做差异审查，节省重派 reviewer 成本）。

## Stage 4 · Developer（dispatched）

派发要点：
- 输出文件：`04_DEVELOPMENT.md`
- 修改 11 个 .ps1 加 BOM、新增 `scripts/.editorconfig`、verify_all 加 E.7（双脚本同步）、baseline.json 升 version + notes、dev-map 追加。
- 必读 02 §3 完整步骤 + 03 §6 高概率开发问题预答。
