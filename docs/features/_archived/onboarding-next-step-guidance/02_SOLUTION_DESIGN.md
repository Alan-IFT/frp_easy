# 02 方案设计 — T-062 · onboarding-next-step-guidance

> Stage 2 / Solution Architect · 模式：full · 分区：dev-frontend · 输出语言：中文
> 上游：01_REQUIREMENT_ANALYSIS.md（裁决 READY）

## 1. 架构摘要

本任务是**纯前端 UI 增量**：在 4 个页面 + 1 个组件内追加"正向下一步"引导文案、SPA 内导航入口（统一 `router.push`），以及 Wizard `both` 模式 token 不一致的非阻断 warning（一处 `computed` 条件比较）。无新模块、无新依赖、无后端/store/API/路由守卫/数据流改动。系统层面无结构变化——仅在既有 SFC 的 template/script setup 内增量。所有导航目标（`/proxies` `/dashboard` `/server/monitor`）均为 router.ts 现有路由。

## 2. 受影响模块（现有文件，全部 `web/**`）

| 文件 | 改动性质 | 对应 IS |
|---|---|---|
| `web/src/pages/Wizard.vue` | edit（step3 加规则引导 + step2 token 不一致 warning + defineExpose 暴露新 computed/handler） | IS-1, IS-7 |
| `web/src/pages/Client.vue` | edit（handleSave 成功后置 showNextStep 标志 + template 引导块 + useRouter） | IS-2 |
| `web/src/pages/Proxies.vue` | edit（handleSubmit 成功后置 showPostSaveHint 标志 + template 引导块 + #empty 补连通入口 + useRouter） | IS-3, IS-4 |
| `web/src/pages/Server.vue` | edit（loaded 态 #action 区加"查看运行态 →"按钮 + useRouter） | IS-5 |
| `web/src/components/ProxyForm.vue` | edit（remotePort n-form-item 加 placeholder/help 纯文案） | IS-6 |
| `web/src/pages/__tests__/Wizard.spec.ts` | edit（+token 不一致 warning 用例 + 加规则引导用例） | AC-1~3, AC-9~11 |
| `web/src/pages/__tests__/Client.spec.ts` | edit（+保存成功引导用例 + push 断言；需新增 vue-router mock） | AC-4, AC-12 |
| `web/src/pages/__tests__/Proxies.spec.ts` | edit（+保存成功引导 + 空态连通入口用例；需新增 vue-router mock） | AC-5, AC-6, AC-12 |
| `web/src/pages/__tests__/Server.spec.ts` | edit（+查看运行态链接用例；需新增 vue-router mock） | AC-7, AC-12 |
| `web/src/components/__tests__/ProxyForm.spec.ts` | edit 或 new（+远程端口文案断言用例） | AC-8 |
| `scripts/baseline.json` | edit（bump frontend_tests / test_count / version） | AC-13 |
| `docs/dev-map.md` | edit（pages 行追加 T-062 导航引导职责说明） | — |

## 3. 模块分解（无新模块）

本任务不新增模块/组件/composable/util。所有改动是既有 SFC 内增量。理由（见 §7 复用审计）：5 项引导各自高度页面特化（文案不同、触发条件不同、放置不同），抽公共组件收益低于内联成本，且与项目既有范式（T-048 各页内联 router.push、T-057 Wizard 内联 binWarning）一致。

## 4. 数据模型变更

无。不碰 DB、migration、storage、store 字段、API 契约、types.ts。

## 5. API 契约

无 API 改动。所有改动是前端内导航（vue-router）+ 静态文案。涉及的 vue-router API：
- `const router = useRouter()` —— 在 Client.vue / Proxies.vue / Server.vue 的 `<script setup>` 顶层调用（与 Wizard.vue 既有 `const router = useRouter()` 一致，L199）。
- `router.push('/proxies' | '/dashboard' | '/server/monitor')` —— 仅字符串路径，目标均为 router.ts 现有路由（已核：proxies L17 / dashboard L16 / server/monitor L20 / Wizard 自身 push('/dashboard') L332/L351 已在用）。

## 6. 实现流程（逐文件设计，开发者可直接实现）

### 6.1 Wizard.vue — IS-1 加规则引导（step3）+ IS-7 token 不一致 warning（step2）

**IS-7（token 不一致预警，step2）**：
- 新增 `computed` `tokenMismatch`：
  ```
  const tokenMismatch = computed(() => {
    if (selectedRole.value !== 'both') return false
    const fs = frpsForm.value.authToken.trim()
    const fc = frpcForm.value.authToken.trim()
    return fs !== '' && fc !== '' && fs !== fc
  })
  ```
  （BC-3：两端均非空才触发；BC-4：trim 后判空 + trim 后比较相等性，与提交时 `authToken || undefined` 语义对齐——纯空白视为空不触发误报）。
- template：在 step2 的 `<n-alert v-if="configError" ...>`（L124-126）**之前或之后**加一个非阻断 warning alert：
  ```
  <n-alert v-if="tokenMismatch" type="warning" style="margin-top: 12px">
    两端 token 不一致，frpc 将无法连接 frps（如非有意配置，请改为一致）
  </n-alert>
  ```
- **非阻断保证**：`tokenMismatch` 只用于 template 展示，**不**进入 `handleNext()` 的校验逻辑、不写 `configError`、不 return。`handleNext()` step2 分支保持原样（AC-9）。

**IS-1（加规则引导，step3 全就绪分支）**：
- 在 step3 既有"全就绪自动跳转"分支（`<template v-if="binWarning.length === 0">`，L141-146）内，"已保存配置并启用对应模式，现在跳转到仪表盘"文案下方，**条件性**加引导按钮：
  ```
  <n-button
    v-if="selectedRole === 'frpc' || selectedRole === 'both'"
    text type="primary" style="margin-top: 12px"
    @click="goToProxies"
  >
    下一步：前往「代理规则」添加要转发的端口 →
  </n-button>
  ```
  - 仅 frpc/both 显示（BC-1：frps 不显示，AC-2）。
  - 仅在 `binWarning.length === 0` 分支内（即就绪态），缺失分支 `<template v-else>`（L149-158）**不加**（BC-2，AC-3）。
- 新增 handler：`function goToProxies(): void { void router.push('/proxies') }`。
- **不破坏 T-057**：全就绪分支仍保留原 `<n-text>` 文案 + `<n-spin v-if="completing">`，仍在 handleNext 中 `message.success('配置已保存，正在跳转...')` + `void router.push('/dashboard')`（L329-332 不动）。引导按钮是**附加**元素（AC-11）。注意：全就绪分支会自动 `router.push('/dashboard')`，引导按钮在自动跳转发生前的瞬间可见——但 step3 渲染与 push 在同一 tick，测试可在 push 前断言按钮存在（见 §测试设计）。该按钮主要价值在于**手动 push 路径**（用户若停留可点）。设计选择：引导按钮存在性 + 可点击 push('/proxies') 是验收点，不要求阻止自动跳转。
- defineExpose 追加：`tokenMismatch`, `goToProxies`（供测试 getExposed 读取/调用）。

### 6.2 Client.vue — IS-2 保存成功后加规则引导

- 引入 `import { useRouter } from 'vue-router'` + `const router = useRouter()`（script setup 顶层）。
- 新增 `const showNextStepHint = ref(false)`。
- `handleSave()` 成功分支（`message.success('客户端配置已保存（重启 frpc 后生效）')` 之后，L192）追加：`showNextStepHint.value = true`。
- catch 分支不置（BC-7：失败不显示）。
- template：在 loaded 态 `<n-card>` 内 `#action` 之后、或 card 外（建议 card 外、ConfirmDialog 之前），加：
  ```
  <n-alert v-if="showNextStepHint" type="success" style="margin-top: 16px">
    配置已保存。
    <n-button text type="primary" @click="goToProxies">
      下一步：前往「代理规则」添加要转发的端口 →
    </n-button>
  </n-alert>
  ```
- handler：`function goToProxies(): void { void router.push('/proxies') }`。
- defineExpose 追加：`showNextStepHint`, `goToProxies`。

### 6.3 Proxies.vue — IS-3 保存成功引导 + IS-4 空态连通入口

- 引入 `import { useRouter } from 'vue-router'` + `const router = useRouter()`。
- **IS-3**：新增 `const showPostSaveHint = ref(false)`。`handleSubmit()` 成功分支（`showForm.value = false` 之后、firewall 逻辑附近，L215 区域）追加 `showPostSaveHint.value = true`（新增成功与编辑成功两路径都经此，BC-5）。`handleDeleteConfirm` **不**置（BC-6 删除不触发）。catch 不置。
- template IS-3 引导块（放在 `<firewall-hint>` 附近，L36-37 区域）：
  ```
  <n-alert v-if="showPostSaveHint" type="success" style="margin-top: 16px" title="规则已保存">
    <n-space>
      <n-button text type="primary" @click="goToDashboard">去仪表盘启动 frpc →</n-button>
      <n-button text type="primary" @click="goToMonitor">去服务端监控查看运行态 →</n-button>
    </n-space>
  </n-alert>
  ```
- **IS-4**：改 `#empty` 模板（L31-33）—— 保留现有 `n-empty` 描述，在其下追加连通入口：
  ```
  <template #empty>
    <n-empty description="暂无代理规则，点击右上角「新增规则」开始配置">
      <template #extra>
        <n-button text type="primary" size="small" @click="goToMonitor">
          去服务端监控查看运行态 →
        </n-button>
      </template>
    </n-empty>
  </template>
  ```
  （空态用"去服务端监控"——无规则时去仪表盘启动 frpc 意义不大；监控页能看 frps 是否在跑。仅 1 个入口避免空态拥挤。）
- handlers：`function goToDashboard(): void { void router.push('/dashboard') }`、`function goToMonitor(): void { void router.push('/server/monitor') }`。
- defineExpose 追加：`showPostSaveHint`, `goToDashboard`, `goToMonitor`。

### 6.4 Server.vue — IS-5 查看运行态链接（loaded 态）

- 引入 `import { useRouter } from 'vue-router'` + `const router = useRouter()`。
- template：在 loaded 态 `<n-card v-else>` 的 `#action` `<n-space>`（L94-100）内，"重新加载"按钮之后追加：
  ```
  <n-button text type="primary" @click="goToMonitor">查看运行态 →</n-button>
  ```
  （仅在 `v-else` loaded 态 card 内，故加载失败 `v-if=loadError` / 加载中 `v-else-if=loading` 两 card 不含此按钮，满足 BC-8 / AC-7。）
- handler：`function goToMonitor(): void { void router.push('/server/monitor') }`。
- defineExpose 追加：`goToMonitor`。

### 6.5 ProxyForm.vue — IS-6 远程端口纯文案

- remotePort `n-form-item`（L56-68）加 help 文案。两种实现二选一（开发者择简）：
  - **方案 A（推荐，可测且明显）**：`n-form-item` 加 `feedback`/在 item 内 n-input-number 下加一行说明文本：
    ```
    <n-form-item v-if="isTcpUdp" label="远程端口" path="remotePort">
      <n-space vertical :size="2" style="width: 100%">
        <n-input-number v-model:value="form.remotePort" :min="1" :max="65535"
          placeholder="1-65535，需在服务端「端口策略」允许范围内" style="width: 100%" />
        <n-text depth="3" style="font-size: 12px">需在服务端「端口策略」允许范围内</n-text>
      </n-space>
    </n-form-item>
    ```
  - **方案 B（最小）**：仅改 placeholder 为 `"1-65535，需在服务端「端口策略」允许范围内"`。
  - **决策：用方案 A**（placeholder 在用户输入后消失，help 文本常驻更稳，且 DOM 可稳定断言；引入 NText 到 ProxyForm 既有 import）。
- 纯文案，不读 allowPorts 数据、不改 `rules.remotePort` 校验、不改 useProxyForm（OOS-1）。

## 7. 复用审计

| 需求 | 既有代码 | 文件路径 | 决策 |
|---|---|---|---|
| SPA 内导航 | `useRouter().push` 范式 | Wizard.vue:199/332/351、ServerMonitor.vue:218-220、T-048 各页 | 复用：Client/Proxies/Server 引入 useRouter（与既有一致，insight L17 强制 router.push） |
| 引导/提示视觉 | `n-alert` / `n-text` / `n-button text` | Wizard.vue step3、Dashboard.vue、Proxies.vue n-empty | 复用既有 naive-ui 组件，零新依赖 |
| 空态 | `n-empty` + `#extra` slot | Proxies.vue:31-33 | 扩展：在现有 n-empty 上加 #extra slot 入口（不重写文案，IS-4） |
| Wizard 完成流分支 | binWarning/missingForRole/completing | Wizard.vue:141-158/322-347 | 复用不改：新引导作为第 3 路并存（T-057 护栏 AC-11） |
| token 字段 | frpsForm.authToken / frpcForm.authToken | Wizard.vue:71-79/112-120 | 复用：tokenMismatch computed 读这两个 ref，不改字段 |
| 测试 router mock | `vi.mock('vue-router', () => ({ useRouter: () => ({ push: pushSpy }) }))` | Wizard.spec.ts:58-59 | 复用范式：Client/Proxies/Server.spec 新增同款 vue-router mock |
| 测试句柄 | getExposed / apiError | web/src/test-utils/ | 复用：所有新断言用 getExposed + DOM 文本，零组件名查询（insight L45） |
| message 断言 | `vi.mock('naive-ui')` 单例 spy | Wizard.spec/Server.spec/Client.spec | 复用既有 mock（IS-2/IS-3 仍调 message.success，断言已有，无需新增 message 断言核心是 hint 可见性 + push） |

**结论**：无新模块、无新依赖正当性需求（零新依赖）。全部复用既有范式。

## 8. 风险分析（含缓解）

- **R-1：IS-1 破坏 T-057 全就绪自动跳转。** 全就绪分支会同 tick 自动 `router.push('/dashboard')`，引导按钮的"手动 push('/proxies')"实际用户极难点到（自动跳转先行）。
  - 缓解：设计上引导按钮**仅附加不阻断**，验收点是"按钮存在 + 可点击触发 push('/proxies')"（DOM + push mock），不要求阻止自动跳转。测试在 step3 渲染瞬间（push('/dashboard') 调用后但 DOM 仍在 Holder 中）断言按钮存在并手动调用 goToProxies。AC-11 显式护栏锁死自动跳转行为不变。**接受局限**：全就绪态该按钮 UX 价值有限（用户会先到 dashboard），但语义正确且零破坏；真正高价值的是 frpc/both 缺失态——但缺失态按 BC-2 不加（聚焦补二进制）。设计如实记录此张力，留 GR 评估是否需调整放置（如改在缺失分支也加，或移除自动跳转——后者会破坏 T-057，不采纳）。
- **R-2：Client/Proxies/Server.spec 引入 vue-router mock 破坏既有用例。** 这些 spec 此前无 vue-router mock（组件原本不 import useRouter）。新增 `vi.mock('vue-router')` 是模块级 mock，可能影响既有用例。
  - 缓解：Wizard.spec 已验证该 mock 范式与 getExposed/naive-ui mock 共存无碍（Wizard.spec:58-59）。Client/Proxies/Server 既有用例不依赖 router，新增 mock 只提供 push spy，不改既有行为。开发者须**实读**这些 spec 的 mount/Holder 范式（insight L37 教训：评估 mock 影响要实读断言），确保新 mock 不与既有 stub 冲突。
- **R-3：Proxies #empty #extra slot 与 n-data-table 渲染交互。** n-empty 在 #extra slot 加按钮需确认 n-empty 支持 #extra（naive-ui n-empty 文档支持 `extra` slot）。
  - 缓解：naive-ui NEmpty 原生支持 `#extra` slot（常用模式）。若开发者验证不可用，退化为在 #empty 模板内 n-empty 后并列一个 n-button（仍在 #empty slot 作用域内）。
- **R-4：测试数 bump 与 baseline 漂移。** 漏 bump baseline.json → verify_all B.4 FAIL。
  - 缓解：开发者实现完成后按实际新增用例数 bump frontend_tests/test_count/version（红线 + AC-13）。04_DEVELOPMENT 记录精确增量。
- **R-5：e2e 回归。** 新文案进入 03-dashboard 等路径。
  - 缓解：PM 已 grep 核实 e2e 不进 Proxies/Server/Client 编辑流、用 bypassWizard 绕过向导、无新文案断言（insight L34）。零 e2e 改动。

## 9. 迁移 / 上线计划

- 向后兼容：纯增量 UI，无破坏性变更。无 feature flag 需求。
- 无数据迁移。
- 回滚：git revert 单批前端改动即可，无状态残留（无 store/DB 改动）。

## 10. 范围外澄清（设计边界）

- 不实现 OOS-1（ProxyForm 跨页端口策略实时校验联动）—— IS-6 仅纯文案。
- 不实现 OOS-2（token 强制阻断）—— IS-7 仅非阻断 warning。
- 不改 T-057/T-058/T-060 既有 Wizard 完成流 / Server-Client 重新加载+dirty / 端口策略 dirty 逻辑。
- 不改 router.ts、不加路由、不碰路由守卫。
- 不抽公共引导组件（§3 理由：页面特化，内联与既有范式一致）。

## 11. Partition assignment（必填 — dev-*.md 存在）

| File | Partition | New / Edit | Dependency |
|---|---|---|---|
| `web/src/pages/Wizard.vue` | dev-frontend | edit | — |
| `web/src/pages/Client.vue` | dev-frontend | edit | — |
| `web/src/pages/Proxies.vue` | dev-frontend | edit | — |
| `web/src/pages/Server.vue` | dev-frontend | edit | — |
| `web/src/components/ProxyForm.vue` | dev-frontend | edit | — |
| `web/src/pages/__tests__/Wizard.spec.ts` | dev-frontend | edit | — |
| `web/src/pages/__tests__/Client.spec.ts` | dev-frontend | edit | — |
| `web/src/pages/__tests__/Proxies.spec.ts` | dev-frontend | edit | — |
| `web/src/pages/__tests__/Server.spec.ts` | dev-frontend | edit | — |
| `web/src/components/__tests__/ProxyForm.spec.ts` | dev-frontend | new/edit | — |
| `scripts/baseline.json` | dev-frontend（测试数 bump，惯例由实现分区改） | edit | — |
| `docs/dev-map.md` | dev-frontend | edit | — |

### Dispatch order
1. dev-frontend（唯一分区）

### Parallelism
None — 单分区。所有改动在 `web/**`（+ baseline.json / dev-map.md 文档惯例）。无跨分区依赖。

## 12. 裁决

**READY** —— 设计完整，开发者可无需进一步设计决策即实现。唯一需 GR 关注点是 R-1（IS-1 全就绪态引导按钮的 UX 张力与放置），已如实记录并给出"不破坏 T-057 优先、接受局限"的明确决策，GR 可据此评估是否需求层调整放置（若需调整，回退 RA）。
