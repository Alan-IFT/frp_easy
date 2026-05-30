# 需求分析 — T-056 proc-stop-destructive-confirm

> Stage 1 · requirement-analyst · mode: full · 中文

## 1. Goal（一句话问题陈述）

Dashboard 上 frpc / frps 的"停止"与"重启"按钮当前点击即立刻调用后端中断进程，缺少二次确认，误点会瞬间中断所有正在穿透 / 转发的连接；本任务为这四个破坏性操作（frpc-停止、frpc-重启、frps-停止、frps-重启）加二次确认，与站内"删除代理规则"的确认标准对齐。

## 2. In-scope behaviors（编号、可测、无"可能/应该"）

1. 点击 frpc 卡片的"停止"按钮时，弹出确认对话框，且在用户确认前**不**调用停止 API（`procStore.stopProc` / 底层 `apiStopProc`）。
2. 点击 frpc 卡片的"重启"按钮时，弹出确认对话框，且在用户确认前**不**调用重启 API（`procStore.restartProc` / 底层 `apiRestartProc`）。
3. 点击 frps 卡片的"停止"按钮时，弹出确认对话框，且在用户确认前**不**调用停止 API。
4. 点击 frps 卡片的"重启"按钮时，弹出确认对话框，且在用户确认前**不**调用重启 API。
5. 在确认对话框点击"确认"后，调用与待执行操作（kind + stop/restart）严格对应的原 handler（`handleStop(kind)` 或 `handleRestart(kind)`），不串扰其它 kind / 其它操作。
6. 在确认对话框点击"取消"后，不调用任何停止 / 重启 API，对话框关闭，进程状态不变。
7. "启动"按钮行为不变：点击直接调用 `handleStart(kind)`，**不**弹确认（启动是非破坏性操作）。
8. 确认文案按进程 + 操作定制（复用既有 `labelOf(kind)`：客户端 frpc / 服务端 frps）：
   - 停止 · 标题 `停止{label}？`；内容：frps 为"将立即中断所有正在穿透的远程连接。"，frpc 为"将断开本机所有正在转发的连接。"
   - 重启 · 标题 `重启{label}？`；内容固定为"将短暂中断当前所有连接后重新建立。"
9. 确认后按钮的 `:loading`（`loadingMap['<kind>-stop' | '<kind>-restart']`）与 `:disabled`（`canStop(kind)` / `state==='stopped'`）行为与改动前一致：loading 由原 handler 在 API 调用期间置位，确认对话框本身不引入新的 loading 语义。

## 3. Out-of-scope（本迭代明确不做）

- 不改"启动"按钮的任何行为（无确认）。
- 不改后端：`procStore` / `apiStopProc` / `apiRestartProc` / Go 进程管理逻辑零改动。
- 不改自动启动开关（`handleModeToggle`）的确认逻辑——开关已是声明式状态，非"瞬时破坏性命令"，不在本任务范围。
- 不改 `ConfirmDialog.vue` 的 props / events 契约（既有 `show/title/content` + `update:show/confirm/cancel` 已够用；尽量零改动）。
- 不引入"不再提示"记忆勾选、不引入危险操作输入框二次校验（站内删除确认也没有，保持一致）。
- 不改 e2e 测试集的断言目标（e2e TC-04/TC-05 不点击停止/重启，见 §7）。

## 4. Boundary conditions（null / empty / max / 并发 / 错误路径）

- **并发待确认**：用户先点 frpc-停止（对话框打开），未确认又点 frps-重启——必须保证最终确认时执行的是**最后一次记录的**待执行操作（kind + type），不能串到第一次的操作；或保证后点的操作覆盖前一个待确认操作。设计需明确单一 `pendingAction` 语义。
- **确认后 API 失败**：原 handler 已有 try/catch + `message.error(extractErrorMessage(...))`，确认链路不得吞掉该错误路径。
- **按钮禁用态**：当 `canStop(kind)` 为 false（进程已 stopped）或重启在 stopped 态禁用时，按钮本身 disabled，点击不可达——确认逻辑不需额外防御该路径（DOM 层已挡）。
- **对话框 update:show=false 的两条路径**：点击"确认"（ConfirmDialog 内部 `handleConfirm` 先 emit confirm 再 emit update:show=false）与点击"取消"/遮罩关闭（emit cancel + update:show=false）必须语义可分：仅"确认"触发实际 API。
- **重复确认**：确认对话框点确认后立即关闭（`show=false`），无法对同一 pendingAction 连点两次触发两次 API。

## 5. Acceptance criteria（可验证）

- **AC-1**：单测——点 frpc"停止"按钮 → 对话框 `show=true` 且 `apiStopProc` / `stopProc` 调用次数为 0。
- **AC-2**：单测——AC-1 后点"确认" → `stopProc('frpc')` 被调用恰好 1 次，且未调用 `restartProc` / `startProc`。
- **AC-3**：单测——点 frps"重启"按钮后点"确认" → `restartProc('frps')` 被调用恰好 1 次。
- **AC-4**：单测——点任一停止/重启按钮后点"取消" → 对应 stop/restart API 调用次数为 0，对话框关闭。
- **AC-5**：单测——点"启动"按钮 → 不弹确认对话框（无 pending），`startProc(kind)` 直接调用 1 次。
- **AC-6**：单测——确认对话框 title / content 随 pendingAction 正确切换：frps-停止 含"将立即中断所有正在穿透的远程连接"；frpc-停止 含"将断开本机所有正在转发的连接"；重启含"将短暂中断当前所有连接后重新建立"，标题含 `停止客户端 frpc？` / `重启服务端 frps？` 等正确组合。
- **AC-7**：`scripts/verify_all` 全量（含 e2e）PASS，前端测试数 ≥ 基线 + 新增数，`scripts/baseline.json` 的 `frontend_tests` / `test_count` 同步 bump（B.4 闸门）。
- **AC-8**：`web/src/pages/Dashboard.vue` 改动后 eslint 0 error；script 段纯逻辑行（去 import / 注释 / testing hook）< 200（insight L22/L31 口径）。

## 6. Non-functional requirements

- **一致性（UX）**：确认范式（组件、按钮文案"确认/取消"、warning 类型）与 `Proxies.vue:64-68` 删除确认逐项对齐，避免站内出现两套确认交互。
- **回归安全**：现有 Dashboard 测试（T-047 A2 / T-048 E1 / D4 共 ~14 用例）零回归——确认改动不得改 `handleStop/handleRestart/handleStart` 的对外可观察行为（成功/失败文案、loading）。
- **可测性**：新增确认相关状态 / 触发器需经 `defineExpose({__testing})` 暴露，供 `getExposed` 读（insight L45）。

## 7. Related tasks（关联历史，引用不复述）

- **T-048 frontend-consistency-cleanup**（`docs/features/frontend-consistency-cleanup/`）：在 Dashboard 引入 `labelOf` / `actionResultMsg` / `handleStart/Stop/Restart` 并经 `defineExpose({__testing})` 暴露——本任务直接复用这些句柄与 `labelOf`。
- **T-047 frontend-honest-states**（`docs/features/frontend-honest-states/`）：Dashboard 开关失败不静默，确立 `messageSpies` 单例 + mock 范式（本任务测试沿用 `web/src/pages/__tests__/Dashboard.spec.ts` 的 mock 骨架）。
- **T-042 proxy-runtime-status-merge** / **T-037**（`Proxies.vue`）：删除确认复用 `ConfirmDialog.vue` 的范式（`v-model:show` + `:content` + `@confirm`）——本任务镜像该范式到 Dashboard。
- **T-043 frontend-test-suite-repair**：`getExposed` / `apiError` test-utils（insight L45）——本任务测试沿用。

## 8. Open questions for user

无。orchestrator 已在 INPUT.md 锁定全部决策（哪些操作加确认、对哪些进程、复用哪个组件、确认文案逐字）。无残留歧义。

## 9. Verdict

**READY**
