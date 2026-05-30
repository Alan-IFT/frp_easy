# 方案设计 — T-056 proc-stop-destructive-confirm

> Stage 2 · solution-architect · mode: full · 中文

## 1. Architecture summary

纯前端、单文件改动。在 `web/src/pages/Dashboard.vue` 引入一个 `ConfirmDialog` 实例 + 一个 reactive 的 `pendingAction { kind, type:'stop'|'restart' }` 状态机：把 frpc/frps 两卡片的"停止""重启"四个按钮的 `@click` 从直接调用 `handleStop/handleRestart` 改为调用新的 `requestStop(kind)` / `requestRestart(kind)`（仅记录 pendingAction 并打开对话框）；对话框的 `@confirm` 调 `confirmPending()`，根据 pendingAction 分派到原 `handleStop(kind)` / `handleRestart(kind)`；`@cancel`/`update:show=false` 清空 pendingAction。"启动"按钮链路与后端零改动。

## 2. Affected modules

| 文件 | 改动 |
|---|---|
| `web/src/pages/Dashboard.vue` | 模板：4 个破坏性按钮 `@click` 改指向 `requestStop/requestRestart`；新增 1 个 `<confirm-dialog>` 实例（绑定动态 title/content）。脚本：import `ConfirmDialog`；新增 `pendingAction` ref + `confirmTitle`/`confirmContent` computed + `requestStop`/`requestRestart`/`confirmPending`/`cancelPending` 函数；`defineExpose.__testing` 追加新句柄。 |
| `web/src/pages/__tests__/Dashboard.spec.ts` | 新增 confirm 流程用例（AC-1~AC-6 + adversarial）。 |
| `scripts/baseline.json` | bump `frontend_tests` + `test_count`（QA stage 落，dev 预留）。 |

`web/src/components/ConfirmDialog.vue`：**零改动**（既有 props/events 完全够用，见 §7 reuse audit）。

## 3. Module decomposition（新增脚本逻辑）

无新模块 / 文件。Dashboard.vue 内新增逻辑单元：

- **状态**：`const pendingAction = ref<{ kind: 'frpc' | 'frps'; type: 'stop' | 'restart' } | null>(null)`。`null` = 无待确认 / 对话框关闭。
- **对话框可见性**：用 computed `const showConfirm = computed(...)`？— 否。`ConfirmDialog` 用 `v-model:show`，需要可写。采用：单独 `const showConfirm = ref(false)`，`pendingAction` 与 `showConfirm` 同时置位/复位。`requestStop/requestRestart` 同步设 `pendingAction` + `showConfirm=true`；`cancelPending`（@cancel）与 `confirmPending`（@confirm 后）均复位 `showConfirm=false` + `pendingAction=null`。

  > 设计依据：ConfirmDialog 的 `handleConfirm` 内部先 `emit('confirm')` 再 `emit('update:show', false)`；`handleCancel` 先 `emit('cancel')` 再 `emit('update:show', false)`。父级 `v-model:show` 让 `update:show=false` 自动复位 `showConfirm`。为不依赖单一信号路径，`confirmPending`/`cancelPending` 显式复位 `pendingAction=null`（幂等）。

- **动态文案**：
  - `confirmTitle = computed(() => pendingAction.value ? '<停止|重启>' + labelOf(pendingAction.value.kind) + '？' : '')`
  - `confirmContent = computed(...)`：
    - type==='restart' → `'将短暂中断当前所有连接后重新建立。'`（与 kind 无关）
    - type==='stop' && kind==='frps' → `'将立即中断所有正在穿透的远程连接。'`
    - type==='stop' && kind==='frpc' → `'将断开本机所有正在转发的连接。'`

- **触发器**：
  - `function requestStop(kind: 'frpc' | 'frps') { pendingAction.value = { kind, type: 'stop' }; showConfirm.value = true }`
  - `function requestRestart(kind: 'frpc' | 'frps') { pendingAction.value = { kind, type: 'restart' }; showConfirm.value = true }`
  - `function confirmPending() { const p = pendingAction.value; showConfirm.value = false; pendingAction.value = null; if (!p) return; if (p.type === 'stop') void handleStop(p.kind); else void handleRestart(p.kind) }`
  - `function cancelPending() { showConfirm.value = false; pendingAction.value = null }`

  > 注：`confirmPending` 先快照 `p` 再复位 `pendingAction`，避免 computed 文案在分派前被清空导致竞态；分派用 `void handle...` 保持原 async fire-and-forget 语义（原模板 `@click="handleStop('frpc')"` 也是不 await 的）。

## 4. Data model changes

无。无 DB、无 API schema 变更。

## 5. API contracts

无新增 / 变更 REST 契约。确认后仍走既有 `procStore.stopProc(kind)` → `apiStopProc(kind)`（`PUT /api/proc/{kind}/stop` 等，零改动）。

## 6. Sequence / flow

```
用户点 frps 卡片"停止"按钮
  → @click="requestStop('frps')"
      → pendingAction = { kind:'frps', type:'stop' }
      → showConfirm = true
  → <confirm-dialog v-model:show=showConfirm
        :title=confirmTitle("停止服务端 frps？")
        :content=confirmContent("将立即中断所有正在穿透的远程连接。")
        @confirm=confirmPending @cancel=cancelPending />
  ── 分叉 ──
  A) 用户点"确认"
       ConfirmDialog: emit('confirm') → confirmPending()
         快照 p={frps,stop} → showConfirm=false → pendingAction=null
         → void handleStop('frps')        ← 此处才真正调 stopProc → apiStopProc
       ConfirmDialog: emit('update:show', false)  （幂等复位）
  B) 用户点"取消"/遮罩
       ConfirmDialog: emit('cancel') → cancelPending()
         showConfirm=false → pendingAction=null  ← 不调任何 API
       ConfirmDialog: emit('update:show', false)

用户点"启动"按钮
  → @click="handleStart('frpc')"   ← 不经 pendingAction，直接调 startProc（无确认）
```

## 7. Reuse audit

| Need | Existing code | File path | Decision |
|---|---|---|---|
| 二次确认对话框组件 | `ConfirmDialog`（props show/title/content；events update:show/confirm/cancel） | `web/src/components/ConfirmDialog.vue` | 原样复用，零改动 |
| 确认范式（v-model:show + :content + @confirm） | 删除代理规则确认 | `web/src/pages/Proxies.vue:64-68`（模板）/ `showDeleteConfirm`/`handleDeleteConfirm` | 镜像该范式到 Dashboard |
| 进程标签（客户端 frpc / 服务端 frps） | `labelOf(kind)` | `web/src/pages/Dashboard.vue:223-225` | 复用于动态 title |
| 停止/重启执行 + loading + 错误处理 | `handleStop` / `handleRestart`（含 loadingMap + try/catch + message） | `web/src/pages/Dashboard.vue:303-327` | 确认后调用，签名/行为不变 |
| 测试句柄读取 | `getExposed` | `web/src/test-utils/exposed.ts` | 测试沿用（insight L45） |
| 模拟 API 失败 | `apiError`（axios 形状） | `web/src/test-utils/apiError.ts` | 测试沿用（insight L45） |
| 测试 mock 骨架（messageSpies 单例 + proc/mode/system mock + Holder 包 MessageProvider） | 既有 Dashboard.spec | `web/src/pages/__tests__/Dashboard.spec.ts:17-152` | 复用骨架，新增 describe 块 |

reuse 充分：本任务**不新增任何文件 / 组件 / 依赖**。

## 8. Risk analysis（≥3，每条带缓解）

1. **风险：并发待确认串扰**（先点 frpc-停止再点 frps-重启，确认时执行错对象）。
   - 缓解：单一 `pendingAction` ref，后一次 `requestX` 直接覆盖前一次（last-wins）；`confirmPending` 以快照 `p` 分派，执行最后记录的操作。AC adversarial 用"先 requestStop('frpc') 再 requestRestart('frps') 后 confirm → 只 restart frps、stop frpc 零调用"反向证伪。
2. **风险：确认对话框破坏既有测试 / e2e 点击流程**。
   - 缓解：(a) 既有单测调的是 `handleStop/handleStart/handleRestart` 句柄（不点按钮，T-048 spec 直接 `t.handleStop('frps')`），这些句柄签名/行为零改动 → 零回归。(b) e2e `web/tests/e2e/03-dashboard.spec.ts` 仅断言元素可见 + 退出登录（TC-04/TC-05），**不点击停止/重启** → 确认框不影响 e2e。已核实，无需改 e2e。
3. **风险：computed 文案在 confirmPending 复位 pendingAction 后变空导致对话框闪烁错误文案**。
   - 缓解：`confirmPending` 先 `showConfirm=false`（对话框开始关闭）再清 `pendingAction`；且分派用快照 `p`，不读 computed。文案 computed 在 `pendingAction=null` 时返回 `''`，但对话框已不可见，用户不可感知。
4. **风险：SFC 纯逻辑行数逼近 200 红线**（insight L22）。
   - 缓解：新增逻辑约 4 个短函数 + 1 ref + 2 computed（约 25 纯逻辑行），加既有约 100 行远 < 200。dev self-check 用"script 段非 import 非注释非 testing hook 纯逻辑行数"口径核实。
5. **风险：`@click` 改向后"启动"被误加确认**。
   - 缓解：只改 4 个破坏性按钮（停止×2 / 重启×2）的 `@click`；"启动"按钮 `@click="handleStart(kind)"` 一字不动。AC-5 + 设计 §6 显式守门。

## 9. Migration / rollout plan

无数据迁移、无 feature flag。纯 UI 行为增强，向后兼容（API 调用时机后移到确认后，不改 API 形状）。回滚 = git revert 单文件。

## 10. Out-of-scope clarifications

- 不改 ConfirmDialog 组件契约。
- 不给"启动""自动启动开关"加确认。
- 不加"不再提示"持久化、不加输入框二次校验。
- 不改后端 / store / API。

## 11. Partition assignment（REQUIRED — `.harness/agents/dev-*.md` 存在：dev-db / dev-backend / dev-frontend）

| File | Partition | New / Edit | Dependency |
|---|---|---|---|
| `web/src/pages/Dashboard.vue` | dev-frontend | edit | — |
| `web/src/pages/__tests__/Dashboard.spec.ts` | dev-frontend | edit（QA 主责补对抗用例，dev 可先补 happy） | depends on Dashboard.vue |
| `scripts/baseline.json` | dev-frontend（QA 落数） | edit | depends on 测试新增完成 |

### Dispatch order

1. dev-frontend（唯一分区）

### Parallelism

None — 单分区、单文件。纯前端，无 db / backend 参与。

## 12. Verdict

**READY**
