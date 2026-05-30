# Code Review — T-054 archive-task-sh-regex-align

> Stage 5 / Code Reviewer · mode: full

## Files reviewed

- `scripts/archive-task.sh`（L47-53：注释块 + 改后 awk 行）
- `scripts/archive-task.ps1`（L48 对齐基准，read-only 参照）
- `scripts/verify_all.sh`（L281-296 E.6 闸门，确认 06 标题约束）

## Findings

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### NIT
- [STYLE] `scripts/archive-task.sh:48-50` — 注释用了内容锚（T-035 / 双实现不对称）而非 insight 行号锚。这是 developer 主动选择（GR O-2：行号随 rotation 漂移），评审认同此为更稳的引用方式，不构成问题。仅记录，不阻塞。

## Requirement coverage check

| Criterion | Implementation | Status |
|---|---|---|
| AC-1 awk 模式为容错版 | `scripts/archive-task.sh:53` `/^##[[:space:]]+([^[:space:]]+[[:space:]]+)?Insights?[[:space:]]*$/` | ✅ |
| AC-2 上方注释含 .ps1 对齐 + insight 引用 | `scripts/archive-task.sh:47-50` | ✅ |
| AC-3 §9 前缀 fixture 修复后命中 / 修复前 0 命中 | 06 §Adversarial tests AT-1（命令 + 预期落盘，orchestrator 实跑） | ✅（验证委托 06/orchestrator） |
| AC-4 裸 `## Insight` 不回归 | 06 §Adversarial tests AT-2 | ✅（同上） |
| AC-5 `## Files changed` 不误命中 | 06 §Adversarial tests AT-3 | ✅（同上） |
| AC-6 verify_all PASS FAIL=0 测试数不降 | 04 §verify_all + 06 §verify_all（orchestrator 硬闸门） | ⏳（委托 orchestrator 真跑） |
| AC-7 改动面 = 1 文件、仅正则行 + 注释 | git diff 仅 `scripts/archive-task.sh`；L51-53 awk 行尾部字节未变 | ✅ |

## Design fidelity check

| Design item | Implementation | Status |
|---|---|---|
| 02 §6.1 awk 模式逐片段等价 .ps1 | `archive-task.sh:53` 与对账表逐字节一致 | ✅ |
| 02 §6 同行 awk 后半（flag 终止 + bullet 子正则）保持不变 | L53 `{flag=1; next} /^##[[:space:]]/{flag=0} flag && /^[[:space:]]*-[[:space:]]/` 字节未变 | ✅ |
| 02 §6.2 不用贪婪 `.*` 多 token 写法 | 实现用 `([^[:space:]]+[[:space:]]+)?` 单 token，符合 | ✅ |
| 02 §10 不加 N=0 warning / 不动子正则 / 不扩 verify_all | 仅改标题正则 + 注释，OOS 全守 | ✅ |
| 02 §11 不误派 dev-* 分区 | 04 复述"通用 Developer 承担"分区判定 | ✅ |

## 6 维评审

1. **逻辑正确性**：新正则是旧正则的真超集（旧能命中的新必能命中——把可选组取 0 次即退化为旧式），无回归方向；防假阳性靠 `Insights?` 锚 + 行尾 `[[:space:]]*$`，`## Files changed` 不会命中（`changed` 不匹配 `Insights?$`）。✅
2. **需求保真**：见上覆盖表，AC-1/2/7 已源码层落实，AC-3/4/5/6 委托 06 + orchestrator 实跑。✅
3. **设计保真**：见设计保真表，与 02 字节级一致，零 drift。✅
4. **性能**：awk 单遍逐行，正则放宽不改复杂度（仍 O(n) 行数）。✅
5. **安全**：无外部输入路径变化；正则仅作用于受控 07_DELIVERY.md，无注入面。✅
6. **可维护性**：注释块逐片段对账 awk↔.NET，未来改任一实现可对照另一实现，正是 insight L46 双实现对账原则的源码层固化。✅

## Verdict

`APPROVED`

无 CRITICAL / MAJOR / MINOR。1 条 NIT 为认同性记录。AC-6（verify_all 硬闸门）按项目约定委托 batch orchestrator 独立真跑——这是 role-collapsed 上下文（无 Bash）下的既定路径，非评审遗漏。
