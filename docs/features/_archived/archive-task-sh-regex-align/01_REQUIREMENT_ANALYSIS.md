# 01 需求分析 — T-054 archive-task-sh-regex-align

> Stage 1 / Requirement Analyst · mode: full

## 1. Goal（一句话问题陈述）

`scripts/archive-task.sh:50` 的 awk Insight 标题正则不容错任何前缀，导致当 `07_DELIVERY.md` 用带前缀的标题（如 `## §9 Insight`、`## 8. Insight`）时，bash 收割路径 harvest 0 命中——而 PowerShell 收割路径（`archive-task.ps1:48`）已在 T-028 修复为容错版；本任务把 bash 版对齐到 PowerShell 版，消除双实现不对称。

## 2. In-scope behaviors（编号、可测）

1. `scripts/archive-task.sh` 的 awk Insight 标题匹配模式，对**裸标题** `## Insight` 与 `## Insights`（含 `## ` 后任意空白数量）保持命中（不回归）。
2. 同一模式对**带单 token 前缀**的标题命中：`## §9 Insight`、`## 8. Insight`、`## §8 Insights`、`## 附录 Insight` 等"一个非空白 token + 空白 + Insight(s)"形态。
3. 命中标题后的收割行为不变：仅收割该段内以 `-`（前可有空白）开头的 bullet 行，遇到下一个 `## ` 标题即停止。
4. 在被修改行的**上方新增一行注释**，说明与 `scripts/archive-task.ps1` 对齐 + 引用 insight-index 中"双实现不对称"那条债。
5. 修改后的 bash 模式与 `scripts/archive-task.ps1:48` 的 PowerShell 正则在"前缀容错语义"上等价（两实现对账原则）。

## 3. Out-of-scope（本次明确不做）

- OOS-1：不改 `archive-task.sh` 的其它任何行（rotation、move、report 等逻辑均不动）。
- OOS-2：不改 `archive-task.ps1`（它已是容错版，是对齐基准，read-only 参照）。
- OOS-3：不改 `archive-task.sh` 收割 bullet 行的子正则（`/^[[:space:]]*-[[:space:]]/`）。
- OOS-4：不为该脚本新增 N=0 显式 warning（`.ps1:59-61` 有，`.sh` 无；本任务只对齐"标题前缀容错"这一处债，不顺手扩大到 warning 对齐——留待独立任务，避免改动面蔓延）。
- OOS-5：不新增自动化测试到 `verify_all` 套件（这是 verify_all 闸门相邻工具，反向证伪命令落进 06 即可，见任务约束 4）。
- OOS-6：不运行 `scripts/archive-task`、不 git commit/push（batch orchestrator 负责）。

## 4. Boundary conditions

- BC-1：裸 `## Insight`（最常见正常形态）必须仍命中——这是不回归的硬底线。
- BC-2：`## Insights`（复数）必须仍命中。
- BC-3：带前缀 `## §9 Insight` 必须新命中（修复目标）。
- BC-4：`## ` 后多空白（`## Insight  `、`##   Insight`）仍命中（POSIX `[[:space:]]+` 已覆盖）。
- BC-5：行尾尾随空白（`## Insight ` 含尾空格）仍命中（`[[:space:]]*$` 已覆盖）。
- BC-6：非 Insight 标题（如 `## Files changed`、`## Next steps`）**不得**误命中（防假阳性）。
- BC-7：双 token 前缀（如 `## §9 附录 Insight`，两个前缀 token）——容错版只容忍**单** token 前缀，故此形态不命中；与 `.ps1` 行为一致（`(?:[^\s\n]+\s+)?` 也只容忍单段），属可接受的对称行为，非缺陷。
- BC-8：fixture 验证不得污染真实 `.harness/insight-index.md`（用临时目录或 `--dry-run` + 临时 fixture + 手动 awk）。

## 5. Acceptance criteria（可验证）

- AC-1：修改后 `scripts/archive-task.sh:50`（或其新行号）的 awk 模式为 `/^##[[:space:]]+([^[:space:]]+[[:space:]]+)?Insights?[[:space:]]*$/`（容错版）。
- AC-2：被修改行上方存在一行注释，含"与 archive-task.ps1 对齐"语义 + insight 引用。
- AC-3：对临时 fixture（07_DELIVERY.md 含 `## §9 Insight` 标题 + 1 条 bullet），修复后的 awk 收割到该 bullet（≥1 行）；用旧模式收割 0 行（反向证伪修复前后差异）。
- AC-4：对临时 fixture（含裸 `## Insight` 标题 + bullet），修复后 awk 仍收割到该 bullet（不回归）。
- AC-5：对临时 fixture（含 `## Files changed` 段 + bullet，无 Insight 段），修复后 awk 收割 0 行（不误命中）。
- AC-6：`bash scripts/verify_all.sh` PASS，FAIL=0，测试数不低于 baseline（脚本改动不应触动测试计数）。
- AC-7：改动面 = 恰好 1 个文件（`scripts/archive-task.sh`），仅标题正则行 + 上方注释行。

## 6. Non-functional requirements

- NFR-1（兼容性）：模式须在 GNU awk 与 BSD/mawk 下均按 POSIX ERE 语义工作——`[[:space:]]` / `[^[:space:]]` / 分组 `(...)?` 均为 POSIX awk ERE 标准元素，无 GNU 扩展依赖。
- NFR-2（对账）：与 `.ps1` 正则在"前缀容错"语义维度等价（语法不同——awk POSIX ERE vs .NET regex——但容忍集合相同：0 或 1 个非空白 token 前缀）。

## 7. Related tasks

- **T-028** `archive-task-insight-regex-tolerance`（DELIVERED 2026-05-24，trivial）：仅修了 `.ps1` 的同款容错正则，遗留 `.sh` 未对齐——本任务即偿还该遗留债。见 `docs/tasks.md` L41。
- **T-044** `verify-gate-hardening`（DELIVERED 2026-05-30）：双实现逐桩对账原则的来源（`.harness/insight-index.md` L46）。
- **insight L18**（T-035 evidence）：明确记录 `.sh` regex 不容错前缀、建议下一个 trivial 任务同步——本任务即响应该建议。
- **insight L40**（T-041 evidence）：`## Adversarial tests` / `## Insight` 标题禁前缀，verify_all E.6 锚定裸标题——约束本任务 06 写法。

## 8. Open questions for user

无。技术上下文已由 orchestrator 完全核实（缺陷行号、当前模式、目标模式、等价 awk 写法均已给定），无歧义。

## 9. Verdict

`READY`
