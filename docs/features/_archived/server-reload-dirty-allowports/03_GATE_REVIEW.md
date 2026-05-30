# 03 闸门评审 — T-060 server-reload-dirty-allowports

> 角色：Gate Reviewer · 模式：full · 语言：中文
> 职责：判定本任务是否可以进入开发。独立核实所有代码引用，不信任上游。

## 独立代码核实（已逐处真读）

| 设计声称 | 核实结果 | 文件/行 |
|---|---|---|
| `getAllowPortsInput()` 暴露于 AllowPortsEditor，返回 single 行 `{single}`、range 行 `{start,end}` | ✅ 真实存在 | `web/src/components/AllowPortsEditor.vue` L175-185（single 行 `{single: r.single ?? 0}`、range 行 `{start, end}`） |
| AllowPortsEditor seed 规则 single>0 认 single 否则 range | ✅ 真实 | 同文件 L104-109（`typeof r.single === 'number' && r.single > 0`） |
| `isDirty()` 当前只比 6 标量、注释 L156-158 标"已知局限" | ✅ 真实 | `Server.vue` L156-175 |
| `loadConfig()` L251 设 initialAllowPorts、L253 存 loadedSnapshot | ✅ 真实（行号 L251/L253） | `Server.vue` L243-253 |
| `allowPortsEditorRef` 已存在 + ConfirmDialog 弹窗机制已接 | ✅ 真实 | `Server.vue` L91/L144/L107-113/L161/L177-189 |
| `AllowPortRange = {start?, end?, single?}` | ✅ 真实（三键全可选，`typeof r.single==='number'` 判定可行） | `web/src/types.ts` L57-61 |
| `getExposed` 在 `test-utils/exposed.ts`（非 getExposed.ts） | ✅ 真实，Server.spec L12 已用 | `web/src/test-utils/exposed.ts` |
| `apiError` 测试工具存在 | ✅ 真实，Server.spec L13 已用 | `web/src/test-utils/apiError.ts` |
| e2e 不进 Server 配置页编辑流 | ✅ 核实：03-dashboard 仅 `getByText('frps（服务端）')`，01-setup/02-auth 无 Server | `web/tests/e2e/*.spec.ts` |

设计所有代码引用核实通过，无悬空引用。

## 1. 审计清单（8 维度）

| # | 维度 | 结论 | 理由 |
|---|---|---|---|
| 1 | 需求完整性 | PASS | 8 条范围内行为均可测（apiGet 调用计数 / isDirty 返回值 / normalize 字符串），无歧义词；AC-1~AC-10 逐条可验证。 |
| 2 | 设计完整性 | PASS | 设计覆盖全部范围内行为：normalize 函数 + loadedAllowPortsSnapshot + isDirty 扩展 + 注释更新 + expose 补充，流程图与不变量明确。 |
| 3 | 复用正确性 | PASS | Reuse audit 7 项全部经独立核实存在且可复用；不改 AllowPortsEditor 的决策正确（单向数据流保留，insight L13）。 |
| 4 | 风险覆盖 | PASS | R1 round-trip identity（真风险，AC-5 锁死）、R3 SFC 行数红线（真风险，要求 04 实测）、R5 e2e（已核实无路径）均为实质风险且配缓解。R4 边界值分歧分析准确。 |
| 5 | 迁移安全 | PASS | 无 DB/API/migration；行为变更纯增量（静默重载→弹确认），向后兼容，回滚 = 还原单文件。 |
| 6 | 边界处理 | PASS | §4 覆盖空两侧/single 未填/range 未填/round-trip/顺序/形态/错误态/ref 未挂载（`?.`+`?? []` 兜底），充分。 |
| 7 | 测试可行性 | PASS | 每条 AC 可测：normalize 是纯函数（直测）、isDirty/handleReloadClick 经 getExposed 句柄、apiGet 计数断言已是 Server.spec 既有范式。 |
| 8 | 范围外清晰 | PASS | OOS-1~5 明确（不改组件/Client/后端/不抽 util/不改 save），dev-frontend 不会过度构建。 |

## 2. Findings（WARN / FAIL）

无 FAIL。无阻塞性 WARN。

一条**开发期需履行的条件**（非阻塞，归 APPROVED WITH CONDITIONS）：

- **C1（R3 落实）**：dev-frontend 必须在 04 实测 `Server.vue` `<script setup>` 纯逻辑行数 < 200 行（红线）。当前 script 段（L117-330）含较多注释，新增约 12-15 行逻辑。若逼近 200，**优先精简注释**而非拆文件；若确实超限则标 `DESIGN DRIFT` 回退本设计重议（抽 util）。

## 3. 开发期高概率问题（预答）

1. **Q：normalize 用 `typeof r.single === 'number'` 还是镜像编辑器的 `r.single > 0`？**
   A：用 `typeof r.single === 'number'`（设计 §3 已定）。两侧同函数即可保证一致；single>0 的形态区分由编辑器 seed 负责，normalize 只需对"实际输出形态"稳定映射。无需在 normalize 里复刻 `>0` 判定。

2. **Q：`getAllowPortsInput()` 对未填 single 行返 `{single:0}`，normalize 产 `'s:0'`，会不会和加载快照对不上导致误判脏？**
   A：这是**正确**行为——用户加了一空行就是改动了端口策略，应判脏弹确认。加载快照（后端合法值）不含此行，比较不等 → 脏。符合需求 §4 边界条件。

3. **Q：loadedAllowPortsSnapshot 存什么——cfg.allowPorts 规范化，还是编辑器输出规范化？**
   A：存 `normalizeAllowPorts(cfg.allowPorts ?? [])`（设计 §6）。因 round-trip identity，加载时编辑器输出 normalize 应等于 cfg.allowPorts normalize；存 cfg 侧更早、不依赖编辑器挂载时序，更稳。AC-5 锁死二者相等。

4. **Q：测试如何驱动"只改端口策略"？**
   A：拿到编辑器 ref 调其 expose 的增删（或通过 DOM 点"添加单端口"按钮文本）；或直接断言 `normalizeAllowPorts` 单测 + 通过 expose 的 `loadedAllowPortsSnapshot` 配合 mock 编辑器输出。dev-frontend 选 DOM/getExposed 路径，禁 naive-ui 组件名查询（insight L45）。注意：Server.spec 现用真 AllowPortsEditor（未 mock），可通过 `allowPortsEditorRef` 句柄或 DOM 按钮文本"添加单端口"驱动。

5. **Q：baseline bump 多少？**
   A：等于本任务净新增前端测试数（当前 `frontend_tests=481`/`test_count=803`）。两字段同步加同一增量（B.4 守门，insight 见 baseline notes T-044）。

## 4. 裁决

`APPROVED WITH CONDITIONS`

- 条件 C1：04 实测 SFC script 逻辑 < 200 行（超限则 DESIGN DRIFT 回退）。
- 其余无阻塞。设计与需求一致、代码引用全部核实存在、测试可行、e2e 无影响、红线（单向数据流/不编辑生成文件/测试只升不降）均被设计尊重。

development 可以开始。PM 推进 Stage 4，派 dev-frontend。
