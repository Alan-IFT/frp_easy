# Code Review — T-056 proc-stop-destructive-confirm

> Stage 5 · code-reviewer · mode: full · 中文
> 独立视角逐条走 01 AC + 02 设计，实读改动文件。

## Files reviewed

- `web/src/pages/Dashboard.vue`（模板 4 按钮改向 + 1 ConfirmDialog 实例；脚本 pendingAction 状态机）
- `web/src/pages/__tests__/Dashboard.spec.ts`（TestingHandle 扩展 + beforeEach 默认桩 + T-056 happy-path describe 块）
- `web/src/components/ConfirmDialog.vue`（确认零改动，复核契约）
- `docs/dev-map.md`（Dashboard 行 T-056 注）

## Findings

### CRITICAL
无。

### MAJOR
无。

### MINOR
- [MAINT] `Dashboard.vue:342` — `type ProcKind = 'frpc' | 'frps'` 局部定义。项目 `types.ts` 未导出统一的 ProcKind；此处局部联合类型限定了 `requestStop/requestRestart` 入参，与模板字面 `'frpc'|'frps'` 一致，未来若新增第三种进程才需上提。当前作用域内合理，不阻塞。

### NIT
- [STYLE] `Dashboard.vue:357-359` — `confirmContent` 的 frps/frpc 三元表达式可读；若未来后果文案增多可抽 map，当前两分支三元最简洁。纯偏好，不动。

## Requirement coverage check

| Criterion | Implementation | Status |
|---|---|---|
| AC-1 点 frpc 停止弹框且 stop API 0 调用 | `Dashboard.vue:362-365` requestStop 只设 pendingAction+showConfirm；spec `Dashboard.spec.ts:306-315` 断言 showConfirm=true + apiStopProc 未调 | ✅ |
| AC-2 确认后 stopProc(frpc) 恰 1 次、不串 restart/start | `Dashboard.vue:373-380` confirmPending 按 p.type 分派 handleStop；spec L317-332 断言 1 次 + restart/start 零调 | ✅ |
| AC-3 frps 重启确认后 restartProc(frps) 1 次 | `Dashboard.vue:367-380`；spec L334-345 | ✅ |
| AC-4 取消 → API 零调用 + 对话框关闭 | `Dashboard.vue:382-385` cancelPending 不调 handler；spec L347-359 | ✅ |
| AC-5 启动不弹确认、startProc 直接 1 次 | `Dashboard.vue:83/169` "启动"按钮 `@click="handleStart"` 未改向；spec L361-372 断言 pendingAction=null | ✅ |
| AC-6 动态文案随 pendingAction 正确切换 | `Dashboard.vue:346-360` confirmTitle/confirmContent computed；spec L374-398 四组合全断言 | ✅ |
| AC-7 verify_all 全量 PASS + baseline bump | 交 QA stage 6 落 baseline + orchestrator 全量真跑 | ⏳（QA/PM 阶段验证） |
| AC-8 eslint 0 error + SFC 纯逻辑 < 200 | 04 §SFC 自检纯逻辑 ~110 行；新增标准 Vue 语法无新违规 | ✅（QA verify 复核 eslint） |

## Design fidelity check

| Design item | Implementation | Status |
|---|---|---|
| 单 ConfirmDialog 实例 + pendingAction 驱动（02 §3） | `Dashboard.vue:196-202` 单 `<confirm-dialog>` + `:title=confirmTitle :content=confirmContent` | ✅ |
| `showConfirm` 用 ref（非 computed，02 §3 注） | `Dashboard.vue:344` `const showConfirm = ref(false)`，与 `v-model:show` 可写绑定匹配 | ✅ |
| confirmPending 先快照 p 再复位（02 §3 竞态防御） | `Dashboard.vue:373-379` 先 `const p = pendingAction.value` → 清 pendingAction → 用 p 分派 | ✅ |
| `void handle...` fire-and-forget（02 §3） | `Dashboard.vue:378-379` `void handleStop(p.kind)` / `void handleRestart(p.kind)` | ✅ |
| ConfirmDialog 零改动（02 §7） | `ConfirmDialog.vue` 字节未变；props show/title/content + events 复用 | ✅ |
| 4 破坏性按钮改向、启动不改（02 §6） | `Dashboard.vue:91/98/177/185` requestX；`:83/:169` handleStart 不动 | ✅ |
| loading/disabled 行为不变（01 §2.9） | 4 按钮 `:loading=loadingMap[...]` / `:disabled` 表达式逐字保留 | ✅ |

## 专项审查

- **逻辑正确性（边界）**：并发待确认走 last-wins（单 `pendingAction` ref，后一次 requestX 覆盖）—— `confirmPending` 用快照 `p` 分派，执行最后记录的操作，无串扰。确认后立即 `showConfirm=false` + `pendingAction=null`，无法对同一 pending 连点两次触发两次 API（幂等）。确认后 API 失败仍走 `handleStop/handleRestart` 内既有 try/catch + message.error（未被吞）。✅
- **设计漂移**：无。04 §Design drift 标"无"，实读核对一致。✅
- **性能**：纯 UI，无新轮询 / 无 N+1 / 无大分配。computed 仅在 pendingAction 变化时重算，开销可忽略。✅
- **安全**：无新输入、无 v-html、无注入面。确认文案为硬编码中文常量，无用户输入拼接。✅
- **可维护性**：新增逻辑紧凑（1 type + 2 ref + 2 computed + 4 短函数），命名清晰（requestX / confirmPending / cancelPending 语义自解释），注释只写 WHY（竞态快照、破坏性语义、启动豁免）。无死代码、无过度抽象。✅
- **测试质量（非 shape-matching）**：AC-2 不仅断言 stopProc 调用，还断言 restart/start **零调用**（防串扰）；AC-1/AC-3 断言确认前 API 零调用（真正验证"延迟到确认后"语义，非仅"最终调到了"）；AC-6 四组合逐字断言文案（防文案张冠李戴）。这些断言能真正捕获回归，非仅过形状。✅
- **既有测试回归**：既有 ~21 个 Dashboard 用例调的是 `t.handleStop/handleStart/handleRestart` 句柄（句柄签名 / 行为零改动）→ 零回归。beforeEach 新增的 stop/restart/start 默认桩是 additive（既有用例多用 `mockResolvedValueOnce` 覆盖，不冲突）。✅

## Verdict

**APPROVED**（0 CRITICAL / 0 MAJOR；1 MINOR + 1 NIT 仅记录不阻塞）
