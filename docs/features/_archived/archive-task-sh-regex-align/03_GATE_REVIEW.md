# 03 闸门评审 — T-054 archive-task-sh-regex-align

> Stage 3 / Gate Reviewer · mode: full

## 代码核实（grep + read 实证）

- 已读 `scripts/archive-task.sh:50`：现模式 `awk '/^##[[:space:]]+Insights?[[:space:]]*$/{flag=1; next} /^##[[:space:]]/{flag=0} flag && /^[[:space:]]*-[[:space:]]/' "$delivery_file"` —— 确认不容错前缀，与 01/02 描述一致。
- 已读 `scripts/archive-task.ps1:48`：`if ($content -match '(?ms)^##\s+(?:[^\s\n]+\s+)?Insights?\s*$(.*?)(?=^##\s|\z)')` —— 确认是容错版对齐基准，与 02 §6.1 对账表一致。
- 已读 `scripts/verify_all.sh:287` E.6 闸门：`grep -qE '^##\s+Adversarial\s+tests'` —— 确认锚定裸标题，06 必须用裸 `## Adversarial tests`（01 AC、insight L40 已约束）。
- 已确认 `scripts/verify_all.sh` 无针对 07 `## Insight` 标题的独立静态闸门——故本任务 07 用裸 `## Insight` 即可，无额外锚定风险。

## 8 维审计

| # | 维度 | 判定 | 理由 |
|---|---|---|---|
| 1 | 需求完整性 | PASS | 7 条 AC 全可测；BC-1..8 覆盖裸/前缀/多空白/尾空白/非命中/双 token/污染六类边界，无歧义词。 |
| 2 | 设计完整性 | PASS | 02 §6.1 对账表逐片段映射 awk↔.NET，覆盖全部 in-scope 行为 1-5；改动点在流程图中精确定位。 |
| 3 | 复用正确性 | PASS | 复用审计指向 `.ps1:48` 实证正则 + 同行 awk 后半保留，已核对两文件实际内容，无虚构符号。 |
| 4 | 风险覆盖 | PASS | R1（awk 方言）/R2（误伤同行）/R3（污染 index）/R4（行号漂移）四风险均贴合本改动真实失败面，缓解可执行。 |
| 5 | 迁移安全 | PASS | 新模式是旧模式超集（命中集合单调扩大），无回归方向；单行 git revert 可回滚。 |
| 6 | 边界处理 | PASS | BC-6（防假阳性 `## Files changed`）+ BC-7（双 token 不命中，与 .ps1 对称）已设计；02 §6.2 论证为何不用贪婪 `.*`。 |
| 7 | 测试可行性 | PASS | AC-3/4/5 均可用临时 fixture + 独立 awk 命令验证；E.6 锚定裸标题的约束已传导到 06 写法。 |
| 8 | 越界清晰度 | PASS | OOS-1..6 明确（不动 .ps1、不动子正则、不加 N=0 warning、不扩 verify_all、不 commit/archive）；02 §10 再次划界，over-build 风险低。 |

无 WARN，无 FAIL。

## Findings

无 WARN / FAIL 级 finding。

补充观察（非阻塞，建议 developer 顺手消化）：
- O-1：02 §11 已明确"不派 `dev-*` 分区，由通用 Developer / PM-role 承担"——developer 落地时在 04 复述该分区判定即可，避免归档审查二次怀疑分区是否漏派。
- O-2：insight 实际行号是 L18（T-035 那条债）、L46（T-044 双实现对账），任务描述中的"L23/L26/L46"是逻辑引用；04 注释引用 insight 时建议写"insight-index 双实现不对称债（T-035 那条）"这种内容锚而非行号锚，因行号会随 index rotation 漂移。

## High-probability questions during development（预答）

1. **Q：注释该写什么、放哪？** A：放被改 awk 行**正上方**一行，内容含"与 archive-task.ps1:48 同款容错正则对齐"+ insight 内容锚（双实现不对称债 / T-035）。不写易漂移的行号。
2. **Q：Edit 该替换整行还是子串？** A：替换标题正则子串 `/^##[[:space:]]+Insights?[[:space:]]*$/` → `/^##[[:space:]]+([^[:space:]]+[[:space:]]+)?Insights?[[:space:]]*$/`，保留同行 awk 后半（`{flag=1; next} ... flag && /^[[:space:]]*-...`）不动（R2 缓解）。
3. **Q：如何验证不污染真实 index？** A：在临时目录建 fixture 07_DELIVERY.md，直接对 fixture 跑独立 `awk '<新模式>...' <fixture>` 拿 stdout，不调用 `archive-task.sh` 主流程的 append/move；旧模式同法跑一次拿 0 命中做对照（AC-3）。
4. **Q：要不要 bump baseline.json？** A：不需要——本任务不增减任何测试，go_tests/frontend_tests/test_count 不变；verify_all B.4 计数闸门应保持原值通过。
5. **Q：06 标题能不能写 `## 反向证伪 / Adversarial tests`？** A：不能。verify_all E.6 `grep -qE '^##\s+Adversarial\s+tests'` 锚定裸标题，必须恰好 `## Adversarial tests`（insight L40 已记 T-041 因 `## 3. Adversarial tests` 触发 FAIL）。

## Verdict

`APPROVED`

development 可启动。无阻塞条件；O-1 / O-2 为建议性，developer 顺手消化即可（与 T-035 节奏一致，让 stage 5 处于"几乎无需改"状态）。
