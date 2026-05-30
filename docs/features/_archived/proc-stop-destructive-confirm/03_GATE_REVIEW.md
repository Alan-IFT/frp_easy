# 闸门评审 — T-056 proc-stop-destructive-confirm

> Stage 3 · gate-reviewer · mode: full · 中文
> 独立验证：所有"复用既有代码"声明已 grep / read 实文件核对，非纸面信任。

## 验证动作（reviewer 实读）

- `web/src/components/ConfirmDialog.vue`：实读确认 props = `show/title/content`，events = `update:show/confirm/cancel`；`handleConfirm` = `emit('confirm')` 后 `emit('update:show', false)`；`handleCancel` = `emit('cancel')` 后 `emit('update:show', false)`。✅ 与设计 §3/§6 描述一致。
- `web/src/pages/Dashboard.vue`：实读确认 4 个破坏性按钮 `@click`（停止 frpc L91 / 重启 frpc L98 / 停止 frps L177 / 重启 frps L185）；`handleStop`(L303) / `handleRestart`(L316) / `handleStart`(L290) / `labelOf`(L223) / `loadingMap`(L227) / `canStop`(L272) 均存在且签名与设计假设一致；`defineExpose({__testing})`(L339) 已暴露 `handleStart/Stop/Restart` + `labelOf`。✅
- `web/src/pages/Proxies.vue:64-68`：实读确认删除确认范式 `<confirm-dialog v-model:show + :content + @confirm>`，设计镜像准确。✅
- `web/src/pages/__tests__/Dashboard.spec.ts`：实读确认既有测试调的是 `t.handleStop('frps')` 句柄（不点按钮），故句柄行为不变 → 既有 14 用例零回归（设计 §8 风险 2a 准确）。messageSpies 单例 + proc/mode/system mock 骨架可复用。✅
- `web/tests/e2e/03-dashboard.spec.ts`：实读确认 TC-04 仅断言文案可见、TC-05 仅退出登录，**无任何停止/重启按钮点击** → 新确认框不影响 e2e（设计 §8 风险 2b 准确）。✅
- `.harness/insight-index.md`：L40/L52（标题禁前缀）、L45（getExposed/apiError）、L46（baseline bump）、L22（SFC 纯逻辑行口径）均与设计 / 测试计划一致，无矛盾。✅

## 1. Audit checklist（8 维度）

| # | 维度 | 判定 | 理由 |
|---|---|---|---|
| 1 | Requirement completeness | PASS | 9 个 in-scope 行为全部可测且无歧义词；4 个破坏性操作 + 启动豁免 + 6 条文案逐字定义；无 open question。 |
| 2 | Design completeness | PASS | 每个 in-scope 行为在 §3/§6 有对应实现单元（pendingAction 状态机 + 4 函数 + 2 computed）；启动豁免显式保留。 |
| 3 | Reuse correctness | PASS | reuse audit 7 行全部 read 核对：ConfirmDialog 零改动可行、Proxies 范式存在、Dashboard 句柄存在。无遗漏既有代码。 |
| 4 | Risk coverage | PASS | 5 条风险覆盖并发串扰 / 既有测试回归 / e2e 回归 / computed 竞态 / 启动误加确认；e2e 风险已实读证伪为"不影响"。 |
| 5 | Migration safety | PASS | 无数据 / API 迁移；纯 UI，回滚 = revert 单文件。N/A 项明确标注无。 |
| 6 | Boundary handling | PASS | §4 覆盖并发待确认（last-wins）、确认后 API 失败（原 try/catch 不吞）、禁用态（DOM 已挡）、两条 update:show=false 路径、重复确认。 |
| 7 | Test feasibility | PASS | AC-1~AC-6 均可经 `getExposed` 读句柄 + spy `stopProc/restartProc/startProc` 验证调用次数；AC-7/8 经 verify_all + eslint。每条 AC 可测。 |
| 8 | Out-of-scope clarity | PASS | §3 + §10 双处明确：不改启动 / 不改后端 / 不改 ConfirmDialog 契约 / 不加"不再提示" / 不加输入框校验。developer 不会过度构建。 |

无 WARN、无 FAIL。

## 2. Findings

无 BLOCKED 级 finding。以下为给 developer 的**强约束条件**（APPROVED WITH CONDITIONS，不阻塞 stage 4 启动）：

- **C-1（测试句柄暴露）**：`defineExpose({__testing})` 必须追加 `pendingAction` / `showConfirm` / `confirmTitle` / `confirmContent` / `requestStop` / `requestRestart` / `confirmPending` / `cancelPending`，否则 AC-1~AC-6 无法用 `getExposed` 断言。
- **C-2（spy 调用次数）**：测试断言点确认前 `stopProc/restartProc` 调用次数为 0，确认后恰好 1 次——必须用 `vi.mocked(procApi.apiStopProc)` 或 store action spy，且 mock 默认 resolved（避免 reject 走错误分支干扰断言）。当前 spec `beforeEach` 已 reset 四个 proc mock（L140-143），但**未给 stop/restart 设默认 resolved**——developer 补用例时需在用例内 `mockResolvedValue` 或 beforeEach 补默认，否则确认后调用会 reject 触发 message.error（不影响"是否调用"断言，但影响"成功文案"类断言）。
- **C-3（标题禁前缀）**：QA 写 06 的 `## Adversarial tests` 必须裸标题无任何前缀（insight L40/L52，verify_all E.6）。
- **C-4（baseline bump）**：新增前端测试后同步 bump `scripts/baseline.json` 的 `frontend_tests` + `test_count`（insight L46，verify_all B.4）。
- **C-5（SFC 行数）**：改后核 Dashboard.vue script 段纯逻辑行 < 200（insight L22 口径）。

## 3. High-probability questions during development（预答）

1. **Q：用一个 ConfirmDialog 实例还是两个？**
   A：一个实例 + `pendingAction` 驱动动态 title/content（设计 §3 已定，最清晰、行数最省）。
2. **Q：`showConfirm` 用 ref 还是 computed(pendingAction !== null)？**
   A：用 ref（设计 §3）。ConfirmDialog 的 `v-model:show` 需要可写双向绑定；若用 computed(pendingAction!==null) 则 `update:show` 回写需额外 setter，反而绕。ref + 与 pendingAction 同步置位/复位更直白。
3. **Q：确认后用 await 还是 void fire-and-forget？**
   A：`void handleStop/handleRestart`（设计 §3 注）。原模板 `@click="handleStop('frpc')"` 本就不 await；handler 内部自管 loading + catch。保持一致。
4. **Q：取消是否要恢复焦点 / 处理遮罩点击？**
   A：ConfirmDialog（n-modal preset dialog）的遮罩点击 / Esc 走 `update:show=false`，父级 `v-model:show` 自动复位 `showConfirm`；`cancelPending`（@cancel）再幂等清 `pendingAction`。无需额外焦点处理（与删除确认一致）。
5. **Q：既有 Dashboard 测试会不会因为多了一个 modal 而 mount 报错？**
   A：不会。ConfirmDialog 内部 n-modal 在 `show=false` 时不渲染内容；既有用例不触发 requestX，`showConfirm` 恒 false。已实读 spec 用 Holder 包 NMessageProvider，n-modal 可正常 mount。

## 4. Verdict

**APPROVED WITH CONDITIONS**（C-1~C-5，均为 developer/QA 落实项，不阻塞 stage 4 启动）
