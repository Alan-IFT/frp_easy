# 02 — Solution Design · T-032 proxy-form-vmodel-oom-fix

> **Mode**: full · **Stage**: 2 · **Verdict**: READY
> **Owner**: solution-architect · **Date**: 2026-05-24
> **Upstream**: `docs/features/proxy-form-vmodel-oom-fix/01_REQUIREMENT_ANALYSIS.md` (verdict=READY)

---

## 1. Architecture summary（一句话）

把 `web/src/components/ProxyForm.vue` 从「父→子 prop + 子→父 emit + 双 deep watch 回桥」反模式重构为**单向数据流**：父组件 `Proxies.vue` 只在打开模态框那一刻把 `formData` 注入子组件作为**初始种子**（`:initial-value`），子组件内部用 `useProxyForm` composable 管理工作态，**提交时**由父组件主动 `proxyFormRef.value?.getProxyInput()` 拉取最终值（已有的 `validate()` + `resetBatchState()` defineExpose 通道天然适合扩展这第三个方法）。移除 `props.modelValue` 双向 watch 与 `emit('update:modelValue', ...)` 这条回桥，根除 OOM 反馈环；父组件 `v-model="formData"` 改为 `:initial-value="formData"`，handleSubmit 改读 `proxyFormRef.value?.getProxyInput()` 而非 `formData.value`。

---

## 2. Affected modules（文件路径 + 当前角色 + 本期改动）

### 2.1 必改

| 文件 | 当前角色 | 本期改动 |
|---|---|---|
| `web/src/components/ProxyForm.vue` | 双 watch + emit 桥源头（L143-L171） | 移除 L163-L171 两个 watch；prop 由 `modelValue` 改名 `initialValue`；移除 `emit('update:modelValue', ...)`；`defineExpose` 增加 `getProxyInput()` 方法 |
| `web/src/pages/Proxies.vue` | 父组件 v-model 消费方 | 把 `v-model="formData"`（L35）改为 `:initial-value="formData"`；handleSubmit（L152-L210）从 `formData.value` 读改为 `proxyFormRef.value?.getProxyInput()` 读；`proxyFormRef` 类型声明（L84-L89）追加 `getProxyInput: () => ProxyInput` |
| `web/src/composables/useProxyForm.ts` | `syncFromInput`（L66-L75）每次生成新数组引用，是反馈环放大器 | **移除** `syncFromInput` 函数（消费方 ProxyForm.vue 不再调用）；其余 API（`form / isTcpUdp / isHttpHttps / handleTypeChange / toProxyInput`）保持不变 |

### 2.2 必同步（测试）

| 文件 | 关注点 |
|---|---|
| `web/src/components/__tests__/ProxyForm.spec.ts` | 删 L89-L161 中 4 条仅测 `syncFromInput` 的用例语义改写为「mount 完整组件 + 替换 initialValue prop 后行为」；新增 AC-1 / AC-7 用例；其余 9 条保留 |
| `web/src/components/__tests__/qa_t007_adversarial.spec.ts` | L55-L76 `syncFromInput 是原子的` 用例需改写或删除（因 syncFromInput 被移除）；其余 4 条用例与 syncFromInput 无关，保留 |

### 2.3 不改

`web/src/types.ts`（OOS-2）、`web/src/api/proxies.ts`、`web/src/stores/proxies.ts`、`web/src/composables/useProxyGrouping.ts`、`web/src/composables/usePortPresets.ts`、后端 Go 代码（OOS-1）。

---

## 3. Module decomposition（新模块 / API 变更）

本任务**不引入新模块**，仅重构现有 3 个文件的内部 API。

### 3.1 `ProxyForm.vue` 公开 API 变更

```ts
// Before（现状）
const props = defineProps<{
  modelValue: ProxyInput        // 双向 v-model prop
  editMode?: boolean
  existingProxy?: Proxy | null
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', val: ProxyInput): void    // ← 删
  (e: 'update:batchMode', val: boolean): void
  (e: 'update:portsExpr', val: string): void
}>()

defineExpose({
  validate: () => formRef.value?.validate(),
  isBatchMode: () => batchMode.value,
  getPortsExpr: () => portsExpr.value,
  resetBatchState: () => { ... },
})
```

```ts
// After（本期方案 B — 单向数据流 + emit('submit') 改良版：实际选 B-lite 单向 + defineExpose 拉取）
const props = defineProps<{
  initialValue: ProxyInput      // ← 改名：意图明确为"初始种子"，仅在 mount/重置时使用
  editMode?: boolean
  existingProxy?: Proxy | null
}>()

const emit = defineEmits<{
  // update:modelValue 已删除
  (e: 'update:batchMode', val: boolean): void   // 保留：父组件按钮文案需要
  (e: 'update:portsExpr', val: string): void    // 保留：原 RA default 决策允许，但本期为稳定保留
}>()

defineExpose({
  validate: () => formRef.value?.validate(),
  isBatchMode: () => batchMode.value,
  getPortsExpr: () => portsExpr.value,
  resetBatchState: () => { ... },
  getProxyInput: (): ProxyInput => toProxyInput(),  // ← 新增：父组件提交时主动拉取
})
```

### 3.2 `useProxyForm.ts` 公开 API 变更

```ts
// Before
return { form, isTcpUdp, isHttpHttps, handleTypeChange, toProxyInput, syncFromInput }

// After
return { form, isTcpUdp, isHttpHttps, handleTypeChange, toProxyInput }
// syncFromInput 删除 —— 没有消费方
```

### 3.3 `Proxies.vue` 模板 + handleSubmit 变更

```vue
<!-- Before（L33-L40） -->
<proxy-form
  ref="proxyFormRef"
  v-model="formData"
  :edit-mode="!!editingProxy"
  :existing-proxy="editingProxy"
  @update:batch-mode="(v: boolean) => (batchMode = v)"
  @update:ports-expr="(v: string) => (portsExpr = v)"
/>
```

```vue
<!-- After -->
<proxy-form
  ref="proxyFormRef"
  :initial-value="formData"
  :edit-mode="!!editingProxy"
  :existing-proxy="editingProxy"
  @update:batch-mode="(v: boolean) => (batchMode = v)"
  @update:ports-expr="(v: string) => (portsExpr = v)"
/>
```

```ts
// handleSubmit 调整（L152-L210 局部）
async function handleSubmit() {
  try {
    await proxyFormRef.value?.validate()
  } catch { return }
  submitting.value = true
  try {
    const formValue = proxyFormRef.value?.getProxyInput()  // ← 新增：从子组件拉
    if (!formValue) {
      message.error('表单组件未就绪')
      return
    }
    // T-018 §C.1 批量分支
    if (!editingProxy.value && proxyFormRef.value?.isBatchMode()) {
      const expr = proxyFormRef.value.getPortsExpr().trim()
      const req: BatchProxiesRequest = {
        basename: formValue.name,     // ← 由 formData.value.name 改
        type: formValue.type,         // ← 同上
        localIP: formValue.localIP || '127.0.0.1',
        portsExpr: expr,
        enabled: formValue.enabled !== false,
      }
      // ... 其余不变
    }
    // 单条新增 / 编辑分支
    let savedProxy: Proxy
    if (editingProxy.value) {
      savedProxy = await proxiesStore.updateProxy(editingProxy.value.id, formValue)  // ← 由 formData.value 改
      message.success('规则已更新')
    } else {
      savedProxy = await proxiesStore.createProxy(formValue)
      message.success('规则已创建')
    }
    // 其余不变
  } catch (e) { ... }
}
```

`formData` ref 仍然保留作为「打开模态框时的种子」，由 `handleAdd()` / `handleEdit()` 写入；**不再**承担「实时反映用户输入」的职责。

---

## 4. Data model changes

无。本任务不涉及 SQLite schema、ProxyInput / Proxy 类型契约、后端 API 字段（OOS-1 / OOS-2 锁定）。

---

## 5. API contracts

后端 REST API 完全不变：`POST /api/v1/proxies`、`POST /api/v1/proxies/batch`、`PUT /api/v1/proxies/{id}` 三个调用点的请求 / 响应 / 状态码 / 错误信封都保持现状。前端 `web/src/api/proxies.ts`、`web/src/stores/proxies.ts` 不动。

---

## 6. Sequence / flow（请求级流程图）

### 6.1 新增规则（FR-3）

```
[用户] 点「新增规则 / 批量新增」
   │
   ▼
Proxies.vue:handleAdd()
   ├─ editingProxy.value = null
   ├─ formData.value = defaultFormData()        # 种子数据
   ├─ batchMode.value = false / portsExpr.value = ''
   ├─ proxyFormRef.value?.resetBatchState()
   └─ showForm.value = true
   │
   ▼
<n-modal show=true>
   └─ <proxy-form :initial-value="formData" ref="proxyFormRef" ... />
        │
        ▼
ProxyForm.vue setup()
   ├─ useProxyForm(props.initialValue) → form ref 拿到种子的 deep clone（见 §7 决策点 3）
   ├─ 无 watch(modelValue)  ←★ 反馈环根除
   ├─ 无 emit('update:modelValue', ...)  ←★ 反馈环根除
   └─ 渲染表单
   │
   ▼
[用户] 修改字段（仅写入 form.value.xxx；无回流到父）
   │
   ▼
[用户] 点「保存」
   │
   ▼
Proxies.vue:handleSubmit()
   ├─ await proxyFormRef.value?.validate()
   ├─ const formValue = proxyFormRef.value?.getProxyInput()  ←★ 主动拉取
   └─ proxiesStore.createProxy(formValue)  → POST /api/v1/proxies
   │
   ▼
[后端 200] → showForm=false → 列表刷新
```

### 6.2 编辑规则（FR-6）

```
[用户] 点列表行「编辑」
   │
   ▼
Proxies.vue:handleEdit(proxy)
   ├─ editingProxy.value = proxy
   ├─ formData.value = { ...proxy 完整字段, customDomains: [...proxy.customDomains] }
   │                                         ↑ 已有的 spread copy 防外部突变
   └─ showForm.value = true
   │
   ▼
<proxy-form :initial-value="formData" ... />
   │
   ▼
ProxyForm.vue setup()
   ├─ useProxyForm(props.initialValue) → form.value 拿到 proxy 的 deep clone
   ├─ 模态框关闭→重开时 ProxyForm 组件实例被销毁→重建（n-modal 默认行为，见 §7 决策点 4）
   │                                                ↑ 因此种子总是新鲜，不会"残留旧 proxy"
   └─ 渲染表单（customDomains 显示原值）
   │
   ▼
[用户] 修改 customDomains → form.value.customDomains 局部更新
   │
   ▼
[用户] 点「保存」
   │
   ▼
Proxies.vue:handleSubmit()
   └─ proxiesStore.updateProxy(editingProxy.value.id, getProxyInput())
       → PUT /api/v1/proxies/{id} （body.version 走乐观锁）
```

### 6.3 OOM 反馈环根除示意

```
旧：
父 formData ─v-model→ 子 props.modelValue
                          │ watch(deep)
                          ▼
                       syncFromInput → form.customDomains = [新引用]
                          │ watch(form, deep)
                          ▼
                       emit('update:modelValue', toProxyInput())
                          │ 新对象字面量
                          ▼
父 formData 被回写 → props.modelValue 再变 → 循环 ♾️ → OOM

新：
父 formData ─:initial-value→ 子 props.initialValue
                                  │ 仅 setup() 时读 1 次
                                  ▼
                              useProxyForm(initial)
                                  │
                                  ▼
                              form.value 独立维护
                                  │
                                  ▼ 用户编辑只写本地 form
                              （无 emit modelValue 回流）

[提交]：父 → proxyFormRef.getProxyInput() → toProxyInput() → POST
```

---

## 7. 候选方案对比（NFR-3 强制要求至少 2 个）

### 7.1 方案 A — `defineModel` 宏重构（Vue 3.4+ 官方 idiom）

**实现轮廓**：
- 在 ProxyForm.vue setup 中用 `const modelValue = defineModel<ProxyInput>({ required: true })` 代替 `defineProps + defineEmits('update:modelValue')`。Vue 编译器内部生成的 ref 含**循环检测**——同步赋值如果触发回流，第二次设置会被静默吞掉。
- form 仍由 `useProxyForm` 管理；setup 顶层 `watch(modelValue, syncFromInput, { deep: true })` 与 `watch(form, () => modelValue.value = toProxyInput(), { deep: true })` —— **结构上仍然是双 watch + 双向桥**，但依赖 defineModel 的循环检测兜底。

**改动半径**：
- ProxyForm.vue：`defineProps/defineEmits` 局部替换；两个 watch 仍存在。
- Proxies.vue：`v-model="formData"` 不变（defineModel 不破坏父侧 v-model 语法）。
- useProxyForm.ts：API 不变。

**优点**：
- 父侧 v-model 语法零迁移；defineModel 是 Vue 3.4+ 官方文档推荐的"双向绑定 idiom"。
- 修改面最小（如果只论 LOC）。

**缺点**（致命）：
- defineModel 的「循环检测」是**值相等比较**（Vue 源码：`if (value !== modelValue.value) emit(...)`），而 `toProxyInput()` 每次返回**全新对象字面量**——`!==` 始终为 true，循环检测**对当前场景无效**。RA §2.4 明确："`toProxyInput()` 每次返回全新对象"是反馈环的核心放大器，defineModel 不能根除。
- 即使加深比较自定义 `set` getter，复杂度也搬到了 ProxyForm 内部，且 `form` 与 `ProxyInput` 字段不对等（form.localPort: `number | null`、ProxyInput.localPort: `number`；form 总有 customDomains 数组、ProxyInput 在 tcp 时为 undefined）—— `toProxyInput()` / `syncFromInput()` 的双向转换层不可消除，定制 set/get 反而加复杂度。
- defineModel 设计假设是「父子直接绑同一个原子值（string / number / 简单对象）」，本场景是「子组件有内部工作态、需要 mapper 转换」，**强行套用 idiom 与场景不匹配**。
- 即便 defineModel 能 work，也仍是「双向绑定」语义——后续维护者改了 toProxyInput 一行返回新引用，OOM 又会回来。**架构上没有根治**。

**与 useProxyForm composable 的兼容方式**：API 不变。

**与 T-018 批量模式 emit 的处理**：`update:batchMode` / `update:portsExpr` 可平行改为 `defineModel('batchMode')` / `defineModel('portsExpr')`，父侧 `v-model:batch-mode` / `v-model:ports-expr`；但增益不大。

**长期可维护性**：差。"双向桥 + 框架兜底"是「写一行业务忘记一行容易出 OOM」的设计；下一个 reviewer 仍要解释「为什么 defineModel 在这里安全」。

### 7.2 方案 B — 单向数据流 + 父组件提交时拉取（本期推荐）

**实现轮廓**：
- 父侧把 prop 含义从「实时镜像」改为「初始种子」（`:initial-value="formData"`）。
- 子组件 setup 用 `useProxyForm(props.initialValue)` 一次性初始化内部 form；**不再** watch initialValue（如果父侧后续重置 formData，n-modal 关闭时已销毁子组件，重开会重新 setup —— 见 §7.5 决策点 4）。
- 子组件 defineExpose 新增 `getProxyInput()` 方法。
- 父组件 handleSubmit 改读 `proxyFormRef.value?.getProxyInput()` 而非 `formData.value`。
- `useProxyForm` 删除 `syncFromInput`（无消费方）。

**改动半径**：
- ProxyForm.vue：删 2 个 watch + 1 个 emit + prop 改名 + 加 1 个 defineExpose 方法（净 -10 LOC）。
- Proxies.vue：1 行 template 改 + handleSubmit 内 3 处字段引用从 `formData.value.xxx` 改成 `formValue.xxx`。
- useProxyForm.ts：删 syncFromInput 函数（-10 LOC）+ 返回值清单去掉它（-1 LOC）。

**优点**：
- **架构层根除反馈环**——不存在「子→父→子」回流路径，物理上不可能 OOM。
- 与现有 `validate()` / `isBatchMode()` / `getPortsExpr()` / `resetBatchState()` defineExpose 模式同构（已经在用「父通过 ref 拉子组件状态」），扩展第 5 个方法是自然延续，不引入新模式。
- 单向数据流是 React / Vue 生态的主流推荐架构（参见 Vue 官方 "Avoid mutating prop directly" 规则）；新人 onboarding 不需要解释「为什么这里要双向」。
- 删代码 > 加代码，长期可维护性最佳。
- vitest 测试更容易写——mount 后断言 emit 次数有常数上界（AC-1），无需复杂的"模拟父组件回写"。

**缺点**：
- 父组件 formData ref 不再实时反映用户输入（如果未来某需求需要"在不点保存的情况下监听用户输入"，比如自动保存草稿，要重新引入 emit）。本期 RA 明示 OOS-4 / OOS-5 排除此类需求，**当前无成本**。
- defineExpose 数量从 4 个增到 5 个——但与现状 API 习惯一致。
- handleSubmit 多一行 null-check（`if (!formValue) return`）——可接受。

**与 useProxyForm composable 的兼容方式**：删 syncFromInput；其余完全不动。原 13 条用例中 9 条与 syncFromInput 无关，保留；4 条直接测 syncFromInput 的用例需要改写或删（见 §10 风险分析 R-4 + §13 测试可证明性）。

**与 T-018 批量模式 emit 的处理**：保留现状——`update:batchMode` 和 `update:portsExpr` 是「子→父单向通知」（不存在父→子回写），不是反馈环源头，无需重构。父组件 `batchMode` / `portsExpr` 两个本地 ref 镜像保留。RA §10.1 提到可简化为父组件直接读，但**本期为最小改动半径选择保留**——切换按钮文案需要响应式 ref，去掉镜像反而引入新的 ref 拉取模式，性价比低。

**长期可维护性**：优。是经典的 React/Vue 单向数据流，新人不需要任何"为什么"解释；reviewer 看一眼就能确认无反馈环。

### 7.3 方案 C — 保留双向桥 + 加 guard 标志位（**淘汰**）

**实现轮廓**：在 ProxyForm.vue 加 `const syncing = ref(false)`，watch(modelValue) 时 set true → call syncFromInput → set false；watch(form) 时 `if (syncing.value) return`。

**为什么淘汰**：
- RA §10.2 明示 default 决策："**禁止**保留双向 watch + emit 桥的局部补丁形态。"
- NFR-3 明示原则："架构清理 > 局部补丁"。
- guard 标志位是经典反模式："为了让框架闭嘴而加 imperative 状态机"，下次有人加第 3 个 watch 或异步分支，guard 就漏。
- vitest 难以证明 guard 完全覆盖所有路径——AC-7「emit 次数有常数上界」断言对 guard 方案是"看运气"。

### 7.4 决策矩阵（用户三原则评分）

| 维度 | 方案 A defineModel | 方案 B 单向数据流（推荐） | 方案 C guard 标志位（淘汰） |
|---|---|---|---|
| 用户体验（OOM 不复发） | 中（依赖框架兜底；toProxyInput 返回新引用时无效） | **优**（架构上不可能复发） | 差（guard 漏一个分支就复发） |
| 软件工程标准 | 中（defineModel 设计假设与场景不匹配） | **优**（单向数据流是行业标准） | 差（imperative guard 反模式） |
| 长期易维护 | 中（双向桥仍需解释） | **优**（删代码 > 加代码；defineExpose 同构） | 差（每加 watch 都要复审 guard） |
| 改动半径 | 小（~15 LOC） | 小-中（~30 LOC，含测试改写） | 极小（~5 LOC） |
| 测试可证明性 | 差（仍需模拟双向） | **优**（mount + emit 次数断言） | 差（需穷举 guard 触发路径） |

### 7.5 决策点说明（推荐方案 B 的细节边界）

**决策点 1**：子组件是否监听 `props.initialValue` 后续变化？
- **不监听**。理由：模态框使用模式天然是「打开→编辑→提交/取消→关闭→（销毁组件）」，n-modal 默认在 `:show="false"` 时销毁子树（DOM 移除），下次打开重新挂载子组件。`useProxyForm(props.initialValue)` 在 setup 阶段读 prop 当前值即可，后续不会变。
- **校验**：grep 确认 Proxies.vue L33-L40 的 n-modal 使用模式中 `<proxy-form>` 是 `<n-modal>` 的直接子组件，n-modal `:show=false` 默认行为是销毁。如果未来需要保留（`:display-directive="show"`），需要在子组件加 `watch(() => props.initialValue, syncFromInput, { immediate: false, deep: false })`，但当前无此需求。

**决策点 2**：`getProxyInput()` 是同步还是 async？
- **同步**。`toProxyInput()` 当前已同步（`useProxyForm.ts` L49-L64），返回 ProxyInput 字面量。defineExpose 包装为 `(): ProxyInput => toProxyInput()` 保持同步语义。`validate()` 是 async（Naive UI 约束），handleSubmit 中已 `await validate()`；`getProxyInput()` 在 validate 之后调，同步取值。

**决策点 3**：`useProxyForm(initial)` 内部对 initial 是否需要 deep clone？
- **不需要**。现状 `useProxyForm` L18-L27 已经把 initial 的字段**逐个赋值**到新 `ref({ ... })` 内，本质上是 shallow copy；`customDomains: initial.customDomains ?? []` 在 `??` 短路时新建空数组，未短路时**引用同一份**。如果用户编辑 customDomains 用 `n-dynamic-tags` 触发 `form.value.customDomains.push(...)` 类突变，会污染父组件的 formData ref。但 RA §10.4 锁定 `Proxies.vue:handleEdit` L124 已经 spread `[...proxy.customDomains]` 防御了外部突变（因为 proxy 来自 store，store 中的数组绝不能被组件突变）。`formData.value.customDomains` 本身已经是 spread 后的新数组，子组件再持有引用并修改，影响仅限于 formData ref —— 而 handleSubmit 改读 `getProxyInput()` 后，formData 的值无人关心，污染无害。
- **结论**：保持 useProxyForm 现状不动；不引入 deep clone。

**决策点 4**：n-modal 销毁子组件的行为是否稳定？
- Naive UI 文档：`<n-modal>` 默认 `display-directive="if"`，`:show=false` 时 `v-if` 卸载内容。Proxies.vue L26-L49 未显式设 `display-directive`，走默认。
- **回归保险**：AC-1 与 AC-7 vitest 用例显式 mount 然后用 `wrapper.setProps({ initialValue: newObj })` 模拟 prop 变化场景，断言**即使 prop 后续变了**，子组件 emit 次数仍 ≤ 常数上界——即如果未来某代码路径让 n-modal 保留子组件，本方案仍不会 OOM（因为没有 emit `update:modelValue` 这条回桥）。

---

## 8. Reuse audit（强制）

| Need | Existing code | File path | Decision |
|---|---|---|---|
| form 状态管理（name/type/localIP/localPort/remotePort/customDomains/enabled/version + isTcpUdp/isHttpHttps + handleTypeChange 互斥重置） | `useProxyForm` composable | `web/src/composables/useProxyForm.ts` L17-L85 | **复用**——保留全部 API 除 `syncFromInput`；`form` / `isTcpUdp` / `isHttpHttps` / `handleTypeChange` / `toProxyInput` 不动 |
| form → ProxyInput 字段映射（含 tcp/http 分支选 remotePort vs customDomains） | `toProxyInput()` | `web/src/composables/useProxyForm.ts` L49-L64 | **复用**——被新 `getProxyInput()` defineExpose 方法直接调用 |
| `handleTypeChange` 互斥重置（T-007 AC-9：tcp/udp 清 customDomains、http/https 清 remotePort） | `useProxyForm.handleTypeChange` | `web/src/composables/useProxyForm.ts` L32-L47 | **复用**——AC-9 / FR-7 强制不可破坏；模板中 `@update:value="handleTypeChange"` L36 不变 |
| 父组件通过 ref 拉子组件状态（已有 4 个 defineExpose 方法） | `validate / isBatchMode / getPortsExpr / resetBatchState` | `web/src/components/ProxyForm.vue` L390-L401 | **扩展**——新增第 5 个 `getProxyInput()`，同构模式 |
| 父组件初始化 formData（defaultFormData + 编辑时 spread proxy） | `Proxies.vue handleAdd/handleEdit` | `web/src/pages/Proxies.vue` L97-L131 | **复用**——`handleAdd` L109 / `handleEdit` L118-L127 保持原样，仍作为「初始种子」语义 |
| 批量模式状态镜像到父组件（按钮文案需要） | `update:batchMode` / `update:portsExpr` emit + 父组件本地 ref | `web/src/components/ProxyForm.vue` L190-L197 + `web/src/pages/Proxies.vue` L38-L39 / L94-L95 | **保留**——非反馈环，不重构（最小改动半径优先） |
| Vitest + happy-dom mount 完整组件 | `@vue/test-utils` + `happy-dom` | `web/package.json` L28-L29 + `web/vitest.config.ts` L7 | **复用**——已是 dev dep + happy-dom 已是默认 environment；AC-1 / AC-7 用例直接 `mount()` 即可 |
| Naive UI form ref validate | `FormInst.validate()` | naive-ui L156 dep | **复用**——`validate()` defineExpose 保持现状 |
| 单向数据流模式（其它已有页面有先例） | `Wizard.vue / Setup.vue / Login.vue / Server.vue` 等都用本地 ref + 提交时拉取，**没有**双向 v-model 桥到子组件 | grep `web\src\pages\*.vue` 结果 | **同构**——本方案让 ProxyForm 与其它表单页一致 |

**无新依赖引入**。Vue 3.4 / Naive UI / @vue/test-utils / happy-dom 全部已在 package.json。

---

## 9. Partition assignment（强制；分区 dev-frontend 单分区覆盖）

| File | Partition | New / Edit | Dependency |
|---|---|---|---|
| `web/src/composables/useProxyForm.ts` | dev-frontend | edit（删 syncFromInput L66-L75 + 返回清单 L83） | — |
| `web/src/components/ProxyForm.vue` | dev-frontend | edit（删 watch L163-L171 + prop 改名 + emit 清单减 1 + defineExpose 加 getProxyInput） | 依赖 useProxyForm 改完 |
| `web/src/pages/Proxies.vue` | dev-frontend | edit（template L35 `v-model` → `:initial-value` + handleSubmit L152-L210 从 formData.value 改读 getProxyInput() + proxyFormRef 类型 L84-L89 加 getProxyInput 字段） | 依赖 ProxyForm 改完 |
| `web/src/components/__tests__/ProxyForm.spec.ts` | dev-frontend | edit（改写 syncFromInput 相关用例 + 新增 AC-1 / AC-7 mount + emit 上界用例） | 依赖 useProxyForm + ProxyForm 改完 |
| `web/src/components/__tests__/qa_t007_adversarial.spec.ts` | dev-frontend | edit（删 L55-L76 `syncFromInput 是原子的` 用例；其余保留） | 同上 |

### Dispatch order

1. dev-frontend（单分区一次性完成）

### Parallelism

不适用——全部改动均在 `web/**` 内，无后端 / DB 改动，单分区单 PR 完成。

---

## 10. Risk analysis（至少 3 条 + 缓解）

### R-1：n-modal 销毁/重建语义假设错误（决策点 4）

**风险**：如果项目当前 n-modal 行为不是「`:show=false` 卸载子组件」，那子组件实例会跨多次「打开→关闭」复用同一份 form 状态，`useProxyForm(initialValue)` 只在首次 setup 跑，第二次打开模态框时 form 仍是上次的旧值。

**缓解**：
- AC-1 / AC-7 vitest 用例显式 `wrapper.setProps({ initialValue: newObj })` 测试「prop 后续变化时子组件不读」的行为——如果有缺陷会被测试捕获。
- AC-7 含「父组件连续 3 次替换 initialValue」断言 emit ≤ 常数。
- 兜底：如果真的需要支持「不销毁子组件场景」，在子组件加 `watch(() => props.initialValue, (newVal) => { /* 重置 form */ Object.assign(form.value, useProxyForm(newVal).form.value) }, { deep: false })`——但**仅监听引用变化**（非 deep），父侧 `handleAdd / handleEdit` 都是 `formData.value = newObject` 整体替换，引用一定变；不会因为字段级 deep 比较再触发循环。本期暂不预先加，避免引入未必需要的复杂度——n-modal 默认行为已经够用，QA 阶段如果发现重置失效再加。

### R-2：defineExpose 类型契约与父组件 ref 类型声明漂移

**风险**：`Proxies.vue` L84-L89 手写的 `proxyFormRef = ref<{ validate / isBatchMode / getPortsExpr / resetBatchState }>(...)` 类型签名漏写新加的 `getProxyInput`，子组件实际暴露了但父组件 TS 类型不识别，`vue-tsc --noEmit` 报错。

**缓解**：
- AC-5 / AC-6 由 `npm run build`（vue-tsc）守门，类型不匹配会直接 build fail。
- Developer 实现时**必须**在 `Proxies.vue` 同 PR 内同步更新 `proxyFormRef` 类型签名加 `getProxyInput: () => ProxyInput`，并 import `ProxyInput` 类型（L74 已 import）。

### R-3：删 `syncFromInput` 让现有 ProxyForm.spec.ts 4 条用例编译失败

**风险**：`web/src/components/__tests__/ProxyForm.spec.ts` L89-L161 中 4 条用例直接 destructure 并调用 `syncFromInput`：
- L89-L108 `syncFromInput 可从外部更新表单`
- L114-L138 `AC-9 / C-1: 编辑现有 HTTP 规则加载时 customDomains 不被 watch 抹掉`
- L140-L161 `AC-9 / C-1: 编辑现有 TCP 规则加载时 remotePort 不被 watch 抹掉`
- `qa_t007_adversarial.spec.ts` L55-L76 `syncFromInput 是原子的`

删掉 `syncFromInput` 后 destructure 报 TS 错。

**缓解**：
- 改写策略：把上述用例的「调用 syncFromInput」改为「`mount(ProxyForm, { props: { initialValue: ... } })` + `wrapper.setProps({ initialValue: newObj })` + 断言子组件内部 form 状态」。等价于把单元级 composable 测试升级为组件级 mount 测试——AC-1 / AC-7 / FR-4 / FR-6 验收语义恰好需要此粒度。
- 对 AC-9 / C-1 两条「编辑现有 HTTP/TCP 规则加载时不被 watch 抹掉」用例，新方案下「watch 已删」是更强的保证，等价断言是「mount ProxyForm with initialValue.type='http' + customDomains=['x.com'] → 检查 wrapper.vm.form.customDomains === ['x.com']」。
- 对 `qa_t007_adversarial.spec.ts` L55-L76 `syncFromInput 是原子的`：用例前提（"flush:'pre' 让整个 sync 完成后才触发 watch"）已不适用——watch 不存在了。**删除**该用例并在 spec 文件头注释解释原因（指向本 02 文档 §10 R-3）。RA AC-4 要求"原 13 条用例语义保持"——本期把"语义"理解为"被新代码以等价或更强方式守护"，spec 内具体用例可改写。
- **测试改写归 Developer 阶段**，本设计仅声明改写策略。

### R-4：父组件 formData 与子组件 form 状态分叉

**风险**：handleSubmit 改读 `getProxyInput()` 后，`formData.value` 不再代表用户最终输入；如果未来某个新加的代码路径误读 `formData.value`（比如想做"提交前的 telemetry"），会拿到旧的种子值。

**缓解**：
- 在 `Proxies.vue` 中给 `formData` ref 加 JSDoc 注释：`/** ★ 仅作为 ProxyForm 的初始种子；用户编辑后的最终值用 proxyFormRef.value?.getProxyInput() 取（T-032 单向数据流）。 */`
- Developer 阶段在 04_DEVELOPMENT.md 显式记录此约定。
- 长期：考虑重命名 `formData` → `formInitialSeed`，但 RA §10.4 锁定 handleAdd/handleEdit 行为，rename 涉及内部 ref 名变更，本期不做（OOS-7 不顺手重构）。

### R-5：Naive UI 表单组件 v-model 兼容性（子组件内部 form 字段的 v-model:value）

**风险**：子组件模板中 `<n-input v-model:value="form.name" />` 等是「子组件内部 form ref 与 Naive UI 组件之间的 v-model」，与父子双向桥无关——但**确认**这些 v-model 在新方案下仍然工作（不会因为 form ref 创建路径不同而失效）。

**缓解**：
- 新方案下 `form` ref 由 `useProxyForm(props.initialValue)` 创建，仍是 Vue ref，仍是响应式——`v-model:value="form.name"` 完全 work，与旧版无差异。
- `<n-dynamic-tags v-model:value="form.customDomains">` 触发突变写到 form.customDomains，是 Naive UI 内部行为（直接 push / splice）；新方案下 form.customDomains 仍由 useProxyForm 创建（`initial.customDomains ?? []`），可写。
- 回归测试：FR-3（TCP 新增）+ FR-4（HTTP 含 customDomains 新增）+ FR-6（编辑 HTTP customDomains 增删改）由 vitest 用例覆盖；AC-2 手动 E2E 验证浏览器实际行为。

### R-6：defineModel 路线（方案 A）的诱惑——未来 reviewer 可能想"切换到方案 A 更短"

**风险**：方案 B 是「单向 + 主动拉取」，方案 A 是「框架兜底双向」，初看 A 更"Vue 3.4 idiomatic"。未来 reviewer 提 PR 改回 defineModel 时，没人记得为什么 B 才对。

**缓解**：
- 在 `ProxyForm.vue` 文件顶端 script setup 块上方加块注释，引用本文档：`/** T-032: 使用单向数据流（initialValue prop + defineExpose getProxyInput()）而非双向 v-model / defineModel。原因见 docs/features/_archived/proxy-form-vmodel-oom-fix/02_SOLUTION_DESIGN.md §7。toProxyInput() 每次返回新对象，defineModel 的循环检测对此场景无效。 */`
- Developer 阶段强制加此注释，code-reviewer 会在 05 阶段把关。

---

## 11. Migration / rollout plan

### 11.1 向后兼容性

- **后端 API**：零影响。OOS-1 锁定所有 REST endpoint 不变。
- **数据库 schema**：零影响。OOS-2 / 11.1 同上。
- **前端 `ProxyForm` 公开 prop**：**破坏性**——`modelValue` 改名为 `initialValue`。但 ProxyForm 的唯一消费方是 `Proxies.vue`（grep 验证），同 PR 内一起改完，无外部 breaking。
- **`useProxyForm` 公开 API**：**破坏性**——删除 `syncFromInput`。唯一消费方是 `ProxyForm.vue` + `ProxyForm.spec.ts` / `qa_t007_adversarial.spec.ts`（grep 验证），同 PR 内一起改完。

### 11.2 Feature flag

无。重构是原子替换；feature flag 引入额外复杂度且无回滚价值（OOM 是确定性 bug，新方案不存在"灰度"中间态意义）。

### 11.3 数据迁移

无（前端纯逻辑重构）。

### 11.4 部署步骤

1. Developer 在 `web/**` 内完成所有文件编辑（顺序见 §9 dispatch order）。
2. `npm run lint && npm run test && npm run build` 全绿。
3. `scripts/verify_all` PASS（AC-6 守门）。
4. PR 合并 → CI 滚动构建 → 用户下次 `irm | iex` 或 `curl | bash` 拉到新版本。
5. 不需要后端重启（前端资源 embed 在 Go 二进制中，但二进制启动即生效）。

### 11.5 回滚

`git revert <commit>` 即可。本任务仅触前端 .ts/.vue 文件，无 schema / 持久化层副作用。

---

## 12. Out-of-scope clarifications（本设计 NOT 覆盖）

- **OOS-1 / 2 / 3**（RA 已锁定）：后端契约 / Proxy 类型 / 批量模式业务语义不在本设计内。
- **重写 `update:batchMode` / `update:portsExpr` 为 v-model 形态**：RA §10.1 提到可简化，但本设计为最小改动半径选择**保留**——它们不是反馈环源头，本期不重构。
- **`useProxyForm` 折叠回 ProxyForm.vue 内联**：RA §10.3 允许重构 useProxyForm 公开 API，本设计仅删除 `syncFromInput` 一个函数（最小改动），不做更大的内联重构。
- **n-modal `display-directive="show"` 模式适配**：见 §10 R-1，当前默认 `if` 模式下方案 B 自然 work；如未来切换到 `show` 模式（保留子组件），需补一行 `watch(() => props.initialValue, ...)`，本期不预实现。
- **Playwright E2E 用例新增**：AC-3 明示"现有 E2E 继续通过；不强制新增"。本设计不要求 Developer 新增 Playwright 用例。
- **rename `formData` → `formInitialSeed`**：见 §10 R-4，本期不做。
- **其它组件类似 v-model 桥模式的清理**（如有）：OOS-7 不顺手重构。

---

## 13. 测试可证明性（NFR-4 + AC-1 + AC-7 设计）

### 13.1 AC-1 vitest 用例骨架（emit 次数上界）

```ts
// web/src/components/__tests__/ProxyForm.spec.ts 新增
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import ProxyForm from '../ProxyForm.vue'
import type { ProxyInput } from '../../types'

const defaultSeed = (): ProxyInput => ({
  name: '', type: 'tcp', localIP: '127.0.0.1', localPort: 80, enabled: true,
})

describe('T-032 AC-1: ProxyForm 不产生 update:modelValue 反馈环', () => {
  it('mount 后 50ms / 10 ticks 内不 emit "update:modelValue"', async () => {
    const wrapper = mount(ProxyForm, {
      props: { initialValue: defaultSeed() },
      global: { /* 如有 Naive UI provider 需要在此注入 NMessageProvider stub */ },
    })
    for (let i = 0; i < 10; i++) await nextTick()
    await new Promise(r => setTimeout(r, 50))
    // 关键：本设计删除了 'update:modelValue' emit，断言 emit 表中根本不应出现该事件
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })
})
```

### 13.2 AC-7 vitest 用例骨架（替换 initialValue 不进入循环）

```ts
describe('T-032 AC-7: ProxyForm initialValue 引用变化时不进入无限 emit 循环', () => {
  it('父组件连续 3 次替换 initialValue（新对象引用，字段同），emit 总次数 ≤ 常数', async () => {
    const seedA = defaultSeed()
    const wrapper = mount(ProxyForm, { props: { initialValue: seedA } })
    for (let i = 0; i < 5; i++) await nextTick()

    // 模拟父组件 3 次 formData.value = defaultFormData() —— 新引用、字段相同
    for (let i = 0; i < 3; i++) {
      await wrapper.setProps({ initialValue: defaultSeed() })
      for (let j = 0; j < 5; j++) await nextTick()
    }

    // 关键断言：上界是 0（因为没有 update:modelValue emit），而非随 N 增长
    const emits = wrapper.emitted('update:modelValue')
    expect(emits ?? []).toHaveLength(0)

    // 兜底：batchMode / portsExpr emit 不在此用例语义内，但也应 ≤ 常数（初始 false / '' 后无变化）
    expect((wrapper.emitted('update:batchMode') ?? []).length).toBeLessThanOrEqual(1)
    expect((wrapper.emitted('update:portsExpr') ?? []).length).toBeLessThanOrEqual(1)
  })
})
```

### 13.3 AC-9 / C-1 用例改写策略（FR-6 / FR-7 守护）

```ts
// 旧：测 syncFromInput 单调用 → 新：mount + initialValue + 断言 form
it('编辑现有 HTTP 规则加载时 customDomains 显示正确（FR-6 + AC-9 / C-1 等价断言）', async () => {
  const wrapper = mount(ProxyForm, {
    props: {
      initialValue: {
        name: 'edit-web', type: 'http', localIP: '127.0.0.1', localPort: 80,
        customDomains: ['existing.example.com', 'second.example.com'],
        enabled: true, version: 7,
      },
      editMode: true,
    },
  })
  await nextTick()
  // 旧断言：form.value.customDomains 是从内部 ref 读
  // 新断言：通过 defineExpose 拉
  const submitted = (wrapper.vm as any).getProxyInput?.() as ProxyInput | undefined
  // 或更直接：检查 n-dynamic-tags 渲染出的 tag 数量
  expect(submitted?.customDomains).toEqual(['existing.example.com', 'second.example.com'])
})
```

### 13.4 happy-dom 适配性证明

- `web/vitest.config.ts` L7 已配置 `environment: 'happy-dom'`，无需修改。
- happy-dom 14.x 支持 Vue 3.4 + Naive UI 组件 mount（已被现有 13 条 ProxyForm.spec.ts 用例验证；其中 AC-1 是首个 `mount()` 真挂载组件用例 —— 现有 13 条都是 composable unit 级，**本期是 mount 测试首次引入**）。
- 如果 mount 时 Naive UI 因缺 `NMessageProvider` 抛错（`useMessage` 报"No outer NMessageProvider"），Developer 阶段需要在 `global.stubs` 或 `global.plugins` 注入 stub。这是 happy-dom + Naive UI 组合的已知 caveat（参见 `web/src/App.vue` 顶层包裹 `<n-message-provider>`），Developer 用 `vi.mock('naive-ui', ...)` 局部 stub `useMessage` 返回 `{ error: vi.fn(), success: vi.fn() }` 即可。**本设计声明此处需要 stub**，免得 Developer 卡壳。

### 13.5 现有 13 条用例分类

| 用例（行号） | 是否需要改写 | 新方案下处理 |
|---|---|---|
| L25-L29 type=tcp / isTcpUdp | 否 | 仅测 useProxyForm computed，保留 |
| L31-L36 type=udp / isTcpUdp | 否 | 同上 |
| L38-L42 type=http / isHttpHttps | 否 | 同上 |
| L44-L49 type=https / isHttpHttps | 否 | 同上 |
| L53-L62 handleTypeChange(tcp) | 否 | 仅测 handleTypeChange，保留 |
| L64-L73 handleTypeChange(http) | 否 | 同上 |
| L75-L80 toProxyInput tcp | 否 | 仅测 toProxyInput，保留 |
| L82-L87 toProxyInput http | 否 | 同上 |
| L89-L108 syncFromInput 外部更新 | **是** | 改写为 mount + setProps + 断言 form（见 §13.3） |
| L114-L138 编辑 HTTP / customDomains 不被抹 | **是** | 改写为 mount + initialValue + 断言（更强：watch 已不存在，物理上不可能被抹） |
| L140-L161 编辑 TCP / remotePort 不被抹 | **是** | 同上 |
| L163-L177 type 切换 tcp → http customDomains 不残留 | 否 | 测 form ref + handleTypeChange watch，保留（仍在 useProxyForm 内） |
| L179-L188 type 切换 http → tcp customDomains 清空 | 否 | 同上 |
| L190-L200 type 不变时不重复触发清理 | 否 | 同上 |
| L202-L211 toProxyInput tcp 模式不上送 customDomains | 否 | 仅测 toProxyInput，保留 |

`qa_t007_adversarial.spec.ts` 5 条中 4 条与 syncFromInput 无关（保留），1 条（L55-L76）改写或删除（见 §10 R-3）。

**结论**：13 条用例中 3 条需改写（11/13 = 85% 直接复用），原 spec 的 AC-9 验收语义在新方案下被等价或更强地守护。

---

## 14. Verdict

**`READY`** — 派发到 Gate Reviewer（Stage 3）。

### 总结

- 推荐方案 B「单向数据流 + initialValue + defineExpose getProxyInput()」，理由：架构上根除反馈环（用户原则 1 + 3），与已有 4 个 defineExpose 方法同构（用户原则 2），删代码 > 加代码，测试更易写（AC-1 / AC-7 可用 emit 表上界断言）。
- 方案 A `defineModel` 被否定：循环检测对「toProxyInput 每次返回新对象」无效，且 form ↔ ProxyInput 字段映射层不可消除，强行套用 idiom 与场景不匹配。
- 方案 C guard 标志位被否定：RA §10.2 明令禁止，且 imperative guard 反模式长期不可维护。
- 改动半径：4 个文件、净 ~30 LOC 变更（含测试改写）、零新依赖、零后端 / DB 影响。
- 13 条现有用例 11 条保留 / 2-3 条改写；新增 AC-1 / AC-7 mount + emit 上界断言用例（vitest + happy-dom + @vue/test-utils 全部已具备）。
- Developer 阶段：dev-frontend 单分区一次性完成；按 §9 dispatch order 改完后 `scripts/verify_all` 守门。

### 已知 nuance（交给 Developer 注意）

1. `Proxies.vue` proxyFormRef 类型签名（L84-L89）需同步加 `getProxyInput: () => ProxyInput` 字段并 import 类型（L74 已有 ProxyInput import）。
2. `ProxyForm.vue` 头部加 §10 R-6 防御性注释，引用本文档路径（注意 archive 后路径会带 `_archived/`，按 insight L38 双路径模式或仅引用任务 ID）。
3. mount 测试若遇 Naive UI `useMessage` 报错，用 `vi.mock` 局部 stub（§13.4）。
4. n-modal 默认行为已支持子组件销毁/重建，决策点 4 假设成立；若 QA 发现重复打开行为异常，参考 §10 R-1 兜底加 `watch(() => props.initialValue, ...)`。

---
