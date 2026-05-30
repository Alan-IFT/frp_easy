# Task Input — T-054 archive-task-sh-regex-align

- **slug**: `archive-task-sh-regex-align`
- **mode**: full（7-stage）
- **批次**: project-optimization-2026-05
- **一句话目标**: 修复 `scripts/archive-task.sh:50` awk 的 Insight 标题正则不容错前缀的缺陷，与 `scripts/archive-task.ps1:48` 容错版对齐，偿还 insight（T-035 那条，当前 `.harness/insight-index.md` L18）记录的"双实现不对称"债。

## 精确技术上下文（orchestrator 已核实）

- 缺陷位置：`scripts/archive-task.sh:50`，当前 awk 模式 `/^##[[:space:]]+Insights?[[:space:]]*$/` —— 只认裸 `## Insight` / `## Insights`，不容错任何前缀。
- 目标对齐：`scripts/archive-task.ps1:48` 正则 `^##\s+(?:[^\s\n]+\s+)?Insights?\s*$` —— 容忍单 token 前缀（如 `## §8 Insight`、`## 8. Insight`）。
- bash awk 等价容错写法：`/^##[[:space:]]+([^[:space:]]+[[:space:]]+)?Insights?[[:space:]]*$/`。
- 背景债：insight 明确记录"建议下一个 trivial 任务把 .ps1 同款容错 regex 同步到 .sh"。双实现对账原则（insight L46 = T-044）；verify_all/harvest 工具改动必须反向证伪（insight L46/L30 等）。

## 硬约束

1. 只改 `scripts/archive-task.sh:50` 一处（+ 上方一行注释引用 .ps1 对齐 + insight）。不扩大改动面。
2. 06_TEST_REPORT.md 的 `## Adversarial tests` 段（裸标题）记录反向证伪：含 `## §9 Insight` 前缀的临时 fixture 证明修复后 awk 能 harvest、修复前不能；裸 `## Insight` 不回归。不污染真实 `.harness/insight-index.md`。
3. 不运行 git commit/push；不运行 `scripts/archive-task`（batch 收尾统一做）。
4. 红线：不编辑 `.claude/` / `CLAUDE.md` / `.github/copilot-instructions.md`；测试数只升不降。
