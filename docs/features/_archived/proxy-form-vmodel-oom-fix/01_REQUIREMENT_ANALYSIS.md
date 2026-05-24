# 01 — Requirement Analysis · T-032 proxy-form-vmodel-oom-fix

> **Mode**: full · **Stage**: 1 · **Verdict**: READY
> **Owner**: requirement-analyst · **Date**: 2026-05-24

---

## 1. Goal（一句话）

消除 `ProxyForm.vue` ↔ `Proxies.vue` 之间的 v-model 双向 watch 反馈环路，让"新增代理规则"对话框打开后不再因 Out of Memory 卡死。

---

## 2. 复现路径与症状（独立复核根因）

### 2.1 复现条件

| 项 | 值 |
|---|---|
| 入口 | WebUI 登录后 → 左侧导航「代理规则」（`/proxies`） |
| 触发动作 | 点击右上角「新增规则 / 批量新增」按钮 → 模态框 `<n-modal>` 打开 → 内部 `<proxy-form>` 渲染 |
| 现象触发时刻 | 模态框打开后立刻（无需用户输入任何字段） |
| 浏览器表现 | 标签页卡死 → 内存占用陡升 → 浏览器（或单标签页）报 "Out of Memory / Aw, Snap!" |

### 2.2 用户实际看到

1. 表单点开瞬间界面冻结。
2. 一段时间后浏览器弹出 "Out of Memory" 错误（用户原文）。
3. 无法新增任何代理规则；无法编辑（编辑入口走同一组件路径，同样受影响）。

### 2.3 用户期望

1. 模态框打开后表单可正常输入与提交，CPU/内存占用平稳。
2. 单条新增、批量新增、编辑现有规则三条路径全部可用。
3. 修复后回归测试覆盖该 OOM 路径，未来若再引入双向反馈环要被自动捕获。

### 2.4 根因（RA 独立复核，与 PM 静态分析一致）

文件 `web/src/components/ProxyForm.vue` L164-L171：

```ts
// 通知父组件表单变更
watch(form, () => {
  emit('update:modelValue', toProxyInput())
}, { deep: true })

// 响应父组件的变更
watch(() => props.modelValue, (val) => {
  syncFromInput(val)
}, { deep: true })
```

`web/src/composables/useProxyForm.ts` L66-L75 `syncFromInput`：

```ts
function syncFromInput(val: ProxyInput) {
  form.value.name = val.name
  form.value.type = val.type
  form.value.localIP = val.localIP ?? '127.0.0.1'
  form.value.localPort = val.localPort || null
  form.value.remotePort = val.remotePort ?? null
  form.value.customDomains = val.customDomains ?? []  // ← 永远生成新数组引用
  form.value.enabled = val.enabled !== false
  form.value.version = val.version ?? 0
}
```

环路链：
1. 父组件 `Proxies.vue` `handleAdd()` 把 `formData.value = defaultFormData()` 赋新对象 → `props.modelValue` 变化。
2. 子组件 watch `props.modelValue`（deep=true）触发 → `syncFromInput(val)` 写 `form.value.customDomains = val.customDomains ?? []`。当父传入 `defaultFormData()` 不含 `customDomains` 字段时，此处 `??` 短路产出**全新空数组引用** `[]`，写入响应式 form。
3. 子组件 watch `form`（deep=true）触发 → `emit('update:modelValue', toProxyInput())`，`toProxyInput()` 每次返回**全新对象字面量**。
4. 父组件 v-model 把新对象写回 `formData.value` → `props.modelValue` 再变化 → 回到第 2 步。

每次循环新分配对象 + 触发 deep 比较 → Vue 调度队列无穷累积 → 主线程死循环 + 堆内存爆炸 → 浏览器 OOM。

### 2.5 现存单测为何未捕获

`web/src/components/__tests__/ProxyForm.spec.ts` 全部用例都**只调用 `useProxyForm()` 这一 composable**（unit 级），不挂载完整 `ProxyForm.vue` 组件，因此**双 watch + emit + 父组件 v-model 回写**这条整链路没有被覆盖。Vitest 现有 `AC-9 / C-1: 编辑现有 HTTP 规则加载时 customDomains 不被 watch 抹掉` 测试仅断言 `syncFromInput` 单次调用后的状态，不模拟父子双向桥。

---

## 3. In-scope behaviors（可测试）

**FR-1**：打开「新增规则 / 批量新增」模态框后，`window.performance.memory.usedJSHeapSize` 在打开后 5s 内增量 < 5 MiB；不出现浏览器卡死或 OOM。

**FR-2**：打开「编辑规则」模态框（编辑现有 TCP / UDP / HTTP / HTTPS 任一规则），同 FR-1 的内存增量约束成立。

**FR-3**：单条新增 TCP 规则（name、type=tcp、localIP、localPort、remotePort、enabled）→ 点击「保存」→ 调用 `POST /api/v1/proxies` → 列表出现新行。

**FR-4**：单条新增 HTTP 规则（含 customDomains 至少 1 项）→ 点击「保存」→ 调用 `POST /api/v1/proxies` → 列表出现新行。

**FR-5**：批量新增（batchMode=true，type=tcp，portsExpr="6000-6002"）→ 点击「批量创建」→ 调用 `POST /api/v1/proxies/batch` → 列表出现 3 行。

**FR-6**：编辑现有 HTTP 规则的 customDomains（增/删/改其中一项）→ 点击「保存」→ 调用 `PUT /api/v1/proxies/{id}` → 列表对应行更新。

**FR-7**：类型切换 tcp ↔ http 时 form 字段互斥重置语义保持现状（与 ProxyForm.spec.ts AC-9 全部断言一致）：tcp/udp 切到时清 customDomains；http/https 切到时清 remotePort。

**FR-8**：模态框打开后**关闭**（点「取消」或外部关闭），再次打开（无论 add 还是 edit），FR-1 / FR-2 仍成立（即修复不依赖一次性初始化副作用）。

**FR-9**：父组件 `Proxies.vue` 现有的 `proxyFormRef.value?.resetBatchState()` 调用（L112、L129）继续工作 —— 即子组件仍向父暴露 `validate / isBatchMode / getPortsExpr / resetBatchState` 四个方法（公开 API 形态可改但语义需保留）。

**FR-10**：所有现有 `web/src/components/__tests__/ProxyForm.spec.ts` 测试用例（13 条）全部 PASS。

---

## 4. Out-of-scope（本期不做）

**OOS-1**：不改 `POST /api/v1/proxies` / `POST /api/v1/proxies/batch` / `PUT /api/v1/proxies/{id}` 任何后端契约。

**OOS-2**：不改 `web/src/types.ts` 中 `ProxyInput` / `Proxy` 字段定义。

**OOS-3**：不改批量模式、端口表达式、端口探测、预设 Tag 这些 T-018 功能的业务语义。

**OOS-4**：不改 `Proxies.vue` 列表渲染、折叠分组（`useProxyGrouping`）、防火墙提示等周边逻辑。

**OOS-5**：不引入新 Pinia store 或全局状态。

**OOS-6**：不升级 Vue 主版本（保持 ^3.4.0），仅在 3.4+ 已具备的 idiom 范围内重构。

**OOS-7**：不改其它无相关的组件（即使它们也有类似 v-model 桥模式，本期不顺手重构）。

---

## 5. Boundary conditions

| 场景 | 期望行为 |
|---|---|
| `props.modelValue` 中 `customDomains` 为 undefined | 表单内部按"空数组"语义处理；不触发反馈环。 |
| `props.modelValue` 中 `customDomains` 为 `[]`（空数组） | 同上；不与 undefined 区分对待。 |
| `props.modelValue` 中 `customDomains` 为 `['a.com', 'b.com']` | 表单显示这两项；编辑可增删；保存时按用户最终编辑值上送。 |
| `props.modelValue` 中 `remotePort` 为 undefined | 单条 tcp/udp 模式下显示空输入框；用户必填。 |
| `props.modelValue` 中 `localPort` 为 0 或 falsy | 表单内 `form.localPort` 为 null（与现有 `useProxyForm` 初始化语义一致）。 |
| 父组件连续 1s 内调用 `handleAdd()` 5 次 | 每次都正确重置为 defaultFormData；无累积副作用；无 OOM。 |
| 父组件在模态框打开期间外部强制重置 `formData.value`（例：未来某新代码路径） | 表单内部同步显示新值；不进入循环。 |
| 编辑模态框打开后用户改一个字段（例 localPort 22→2222），再点保存 | `formData.value` 反映用户输入；提交 PUT 的 body.localPort=2222。 |
| `type` 在 tcp ↔ http 之间快速切换 10 次 | 互斥字段按 FR-7 清理；无残留；无 OOM。 |
| 批量模式下连续切换 batchMode 开关 5 次 | 父组件 `batchMode` ref 与子组件保持同步；按钮文案"保存"↔"批量创建"正确切换。 |
| 多个浏览器标签同时打开 /proxies 模态框 | 每个标签内部独立；本任务不涉及跨标签同步。 |

---

## 6. Acceptance criteria（验收准则）

**AC-1**（Verifies FR-1, FR-2, FR-8）：在 happy-dom（或 jsdom）vitest 环境下挂载完整 `<ProxyForm>` 组件（用 `@vue/test-utils mount`），传入 `defaultFormData()` 作为 modelValue，等待 50ms / 10 次 nextTick，断言：
- 子组件 emit 'update:modelValue' 的次数 ≤ 2 次（初次初始化 + 单次稳定），不发散。
- 父组件 wrapper 把 emit 值回写到 modelValue prop 后，再等 50ms，emit 总次数仍 ≤ 3 次。

**AC-2**（Verifies FR-1）：手动 E2E（Playwright 或人工）—— 打开 /proxies，点「新增规则 / 批量新增」，5s 内浏览器无卡顿、无 OOM、Chrome DevTools Performance Monitor 中 JS heap size 增量 < 5 MiB。

**AC-3**（Verifies FR-3, FR-4, FR-5, FR-6）：现有 Playwright E2E（如有覆盖 /proxies 的）继续通过；若现有 E2E 无相关覆盖，**不强制**新增 E2E（vitest 组件级 AC-1 已守门 OOM），但 vitest 用例必须覆盖：
- 单条 TCP 新增提交后 emit 的最终 modelValue 含 remotePort、不含 customDomains。
- 单条 HTTP 新增提交后 emit 的最终 modelValue 含 customDomains、不含 remotePort。
- 编辑现有 HTTP 规则加载后 customDomains 显示正确，**不被反馈环抹掉**。

**AC-4**（Verifies FR-7, FR-10）：`web/src/components/__tests__/ProxyForm.spec.ts` 原 13 条用例全部 PASS（不删不改原断言；可加 it 但不可改 expect）。

**AC-5**（Verifies FR-9）：父组件 `Proxies.vue` 中 `proxyFormRef.value?.resetBatchState()` / `validate()` / `isBatchMode()` / `getPortsExpr()` 四个调用点保持工作，类型检查通过（`npm run build` 内 `vue-tsc --noEmit` PASS）。

**AC-6**（守门）：`scripts/verify_all` PASS（含前端 `npm run lint` / `npm run test` / `npm run build`）。

**AC-7**（回归保险）：新增 1 条 vitest 用例 `ProxyForm.vue does not enter an infinite emit loop when modelValue reference changes`，模拟父组件连续 3 次替换 modelValue（每次新对象引用，字段值相同），断言子组件 emit 次数有上界（具体上界由 Architect 决定，但**必须**是常数而非随 N 增长）。

---

## 7. Non-functional requirements

**NFR-1（性能）**：模态框打开到表单可交互的耗时 < 200ms（人感无延迟）。

**NFR-2（兼容性）**：方案必须在 Vue ^3.4.0（package.json 当前版本）下工作；不要求升级 Vue 主版本。

**NFR-3（可维护性）**：修复方案应消除"双向 watch + emit"反模式，而不是给 watch 加 guard 标志位的局部补丁 —— 用户明示原则"架构清理 > 局部补丁"。Architect 应优先评估 Vue 3.4+ 官方 `defineModel` 宏方案，并在 02 文档中给出至少 2 个候选方案的对比结论。

**NFR-4（可测试性）**：修复后的 ProxyForm 应能在 vitest + happy-dom 下挂载完整组件并断言 emit 行为（即不依赖只能 E2E 才能验证的运行时副作用）。

**NFR-5（安全 / 隐私）**：本任务不涉及敏感数据流；无新增 NFR。

---

## 8. 受影响范围

### 8.1 必改（核心改动面）

| 文件 | 当前角色 | 预期影响 |
|---|---|---|
| `web/src/components/ProxyForm.vue` | 双 watch + emit 桥源头 | 重构 v-model 形态 |
| `web/src/composables/useProxyForm.ts` | `syncFromInput` 持续生成新数组引用，是反馈环放大器 | 视方案需调整 / 可能并入组件 |
| `web/src/pages/Proxies.vue` | v-model 消费方；父组件 | 视方案需调整 v-model 绑定语法 |

### 8.2 必同步（消费方）

| 文件 | 关注点 |
|---|---|
| `web/src/components/__tests__/ProxyForm.spec.ts` | 现有 13 条用例必须 PASS；新增 AC-1 / AC-7 用例 |
| `web/src/components/__tests__/qa_t007_adversarial.spec.ts` | 含 ProxyForm 引用，回归确保 PASS |

### 8.3 不应改

| 文件 | 原因 |
|---|---|
| `web/src/types.ts` | 后端契约字段（OOS-2） |
| `web/src/api/proxies.ts` | HTTP API 封装无关 |
| `web/src/stores/proxies.ts` | Pinia store 无关 |
| `web/src/composables/useProxyGrouping.ts` / `usePortPresets.ts` | 列表/预设辅助无关 |
| 后端 Go 代码（`internal/httpapi`、`internal/storage` 等） | 后端契约不变 |

---

## 9. 必读上下文文件清单（给 Architect）

**源代码**：
1. `web/src/components/ProxyForm.vue` —— 双 watch + emit 桥 + 现有 defineExpose 公开 API。
2. `web/src/composables/useProxyForm.ts` —— form 状态 + syncFromInput 放大器。
3. `web/src/pages/Proxies.vue` —— 父组件 v-model 消费方 + handleAdd/handleEdit 调用 resetBatchState 的路径。
4. `web/src/components/__tests__/ProxyForm.spec.ts` —— 现有 13 条断言，必须保持 PASS。
5. `web/src/types.ts` —— ProxyInput / Proxy 类型契约（OOS-2 锁定）。

**Vue 3.4+ 文档**（context7 已查）：
- `defineModel` 宏：自动产出 props + emits，无需手写 watch + emit 桥。
- 多 v-model 命名参数：`v-model:foo="..."` 配 `defineModel('foo')`。
- `defineModel` 内部对"父→子→父"回流已做循环检测，是官方推荐的双向绑定 idiom。

**项目历史决策**：
- `useProxyForm` 由 T-007 / T-018 演化而来，其 `handleTypeChange` 互斥重置语义（AC-9）是产品决策，**不可破坏**。
- 批量模式（`batchMode` / `portsExpr`）由 T-018 引入，通过 `update:batch-mode` / `update:ports-expr` 两个额外 emit 与父组件通信 —— Architect 必须处理这两个 emit 的归宿（继续 emit / 改 defineModel / 父组件改读 defineExpose 都需评估）。

**Harness insight 相关**：
- `insight-index.md` L20 / L41 / L44 / L48 / L50 —— reviewer 不落盘陷阱（T-030 已加 Write 工具修复），本任务 03 / 05 应能正常落盘。
- 无其它前端相关 insight。

---

## 10. RA default 决策（用户已授权"你来决策"前提下）

PM 明示用户委托决策原则：用户体验好 / 软件工程标准 / 长期易维护，且"以官方文档为准"。下列"是否允许破坏现有公开 API"的边界，RA 给出 default 决策，Architect 在 02 文档中若有更优方案可推翻并附理由。

### 10.1 公开 API 形态（defineExpose 暴露的方法）

**Default 决策**：允许调整 defineExpose 形态，但**必须保留语义等价的入口**让父组件 `Proxies.vue` 调用。具体允许的变动：
- `isBatchMode()` / `getPortsExpr()` 可改为通过 v-model 由子→父反向暴露（`v-model:batch-mode` / `v-model:ports-expr` 已存在 emit，父组件已镜像到本地 ref `batchMode` / `portsExpr` L94-L95），父组件可直接读本地 ref 而非通过 ref 调方法。
- `validate()` **必须保留**为 defineExpose 方法 —— Naive UI form ref 模式约束，无等价 v-model 替代。
- `resetBatchState()` **必须保留**为 defineExpose 方法 —— 父组件 handleAdd/handleEdit 显式调用。

**假设依据**：父组件已通过 `@update:batch-mode` / `@update:ports-expr` 两个 emit 把状态镜像到本地（Proxies.vue L38-L39、L94-L95），handleSubmit L162-L163 仍走 `proxyFormRef.value?.isBatchMode() / getPortsExpr()` —— 是冗余而非必需的双通道。可简化为父组件直接用本地 ref，让 defineExpose 缩到最小 (`validate` + `resetBatchState`)。

### 10.2 是否引入 defineModel 宏

**Default 决策**：**优先尝试 `defineModel`**（Vue 3.4+ 官方推荐 idiom），其内置循环检测正是为本类双向桥反模式设计；若 Architect 评估发现 defineModel 与现有 `useProxyForm` composable 抽象有不可调和冲突（如 form 内部字段需要类型变换 → ProxyInput 的转换，不能直接绑），允许退回到"单向数据流 + emit 提交"方案，**禁止**保留双向 watch + emit 桥的局部补丁形态。

**假设依据**：`useProxyForm` 内 `form` 与外部 `ProxyInput` 不是 1:1 同构（form.localPort 是 `number | null`、ProxyInput.localPort 是 `number`；form 总有 customDomains 数组、ProxyInput 在 tcp 模式下为 undefined），存在 `toProxyInput()` / `syncFromInput()` 双向转换层。defineModel 直接绑外部 ProxyInput 类型时，需评估如何在子组件内"映射到工作类型 → 提交时还原"。Architect 应明确该映射归属（保留 composable / 折叠进组件 setup）。

### 10.3 是否允许改 `useProxyForm` 公开 API

**Default 决策**：允许重构 `useProxyForm` 的返回签名（`form / isTcpUdp / isHttpHttps / handleTypeChange / toProxyInput / syncFromInput`），只要：
- 现有 13 条单测的**语义**断言可以等价表达（用例本身可以小幅改写以适配新签名）。
- `handleTypeChange` 的互斥重置语义保留（AC-9）。

**假设依据**：`useProxyForm` 仅在 `ProxyForm.vue` 一处消费（grep 验证），改公开 API 半径可控。

### 10.4 父组件 `Proxies.vue` 改动半径

**Default 决策**：允许改 `<proxy-form>` 的 v-model 语法（如从 `v-model="formData"` 改为 `v-model:value="formData"` 或多 v-model 拆分），但**禁止**改 `handleAdd / handleEdit / handleSubmit / handleDeleteConfirm` 四个方法的对外行为（提交时的 HTTP 调用、message.success 文案、列表刷新顺序）。

---

## 11. Related tasks（历史关联）

| Task | 关联点 |
|---|---|
| T-001 web-ui-mvp | ProxyForm.vue 初始创建，原始 v-model + watch + emit 桥模式 |
| T-007 hardening-pass-audit | AC-9 引入 `handleTypeChange` 互斥重置语义；现有 ProxyForm.spec.ts AC-9 / C-1 用例由本任务建立 |
| T-018 upload-bin-multiport-ip-probe | §C.1 / §C.2 / §C.3 引入批量模式、端口预设、端口探测；扩展了 defineExpose 与 emit 数量 |

注：以上历史任务文档均已归档在 `docs/features/_archived/<task>/`。

---

## 12. Open questions for user

**无**。用户已通过 PM 明示委托决策原则（"以用户体验好、符合软件工程标准、长期易使用易维护为原则来决策"），并要求"根据 context7 官方文档进行修复"——Vue 3.4+ 官方 idiom 唯一明确的双向绑定方案即 `defineModel`。

第 10 节 RA default 决策已覆盖所有边界。Architect 阶段可在 02 文档中给出最终方案，若与 RA default 决策有偏差需附理由，由 Gate Reviewer 在 03 把关。

---

## 13. Verdict

`READY` — 派发到 Solution Architect。
