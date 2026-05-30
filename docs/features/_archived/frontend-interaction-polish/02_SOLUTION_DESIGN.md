# 02 方案设计 — T-058 frontend-interaction-polish

- **作者角色**: solution-architect
- **日期**: 2026-05-30
- **依据**: 01_REQUIREMENT_ANALYSIS.md + insight L14/L40/L45/L46 + T-036/T-047/T-056/T-057 范式

## 0. 关键决策（先答 01 的三个未知）

### D1（R3）：是否抽 `utils/clipboard.ts`？→ **不抽，两个新点位内联实现**
- **证据**：`web/src/components/__tests__/LogViewer.spec.ts:196-211` 直接测 `onCopy` 内联实现（`navigator.clipboard.writeText` 被 `Object.defineProperty` mock）。抽 util 后 LogViewer 改用它会：(a) 改 LogViewer.vue —— **扩散到 AC-X1 范围外文件**；(b) 可能动其 spec。
- **决策**：FirewallHint / PublicIpDetector 各自内联 LogViewer:145-172 的 fallback 范式（1:1 搬运逻辑结构，不抽公共函数）。抽取 `copyToClipboard` util 复用三处记为 **backlog**（见 §6）。符合 insight L42"抽 util 时先 1:1 搬运再加防御"——本任务先不抽，避免回归。

### D2（R1+B4）：dirty 检测方案？→ **快照 + 浅比较（不含 AllowPortsEditor 子组件内部状态）**
- Server.vue 的 `form` 是 6 个标量字段的 `ref<object>`；Client.vue 的 `form` 是 3 个标量字段。浅比较成本极低。
- **AllowPortsEditor 子组件**：其内部行编辑状态通过 `ref` 单向数据流（insight L13），父侧无法低成本读取"当前编辑值 vs 加载快照"。**决策**：dirty 检测**仅覆盖 `form` 标量字段**，不覆盖 allowPorts 编辑器。
  - 理由：(1) allowPorts 编辑器有独立的"添加/删除行"显式操作，用户对其改动有强感知，误丢风险远低于表单输入框；(2) 纳入会要求 AllowPortsEditor 暴露 dirty 句柄 —— 扩散到范围外文件。
  - 安全侧偏置：标量 dirty 已覆盖最常见的"填了一半 token / 端口"场景。**若仅 allowPorts dirty 而 form 不 dirty**，则直接 reload（不弹确认）——可接受的小代价，记入 06 已知局限。
- **不走退路（rename + 总是确认）**：因为标量浅比较成本低，"不 dirty 不打扰"体验更好（任务书优先此项）。

### D3：dirty 快照存储位置
- 在 `loadConfig` 成功写完 `form.value` 后，深拷贝一份到 `loadedSnapshot` ref（`{ ...form.value }` 浅拷贝即可，因字段全为标量）。
- `isDirty()` computed/函数：逐字段 `form.value.X !== loadedSnapshot.value.X` 任一不等即 dirty。初始 `loadedSnapshot = null` → 视为不 dirty（首挂载 onMounted 的 loadConfig 会立即填上）。

## 1. (A) 剪贴板失败反馈 — 设计

### FirewallHint.vue
- 引入 `import { useMessage } from 'naive-ui'`（已 import NAlert/NButton）；`const message = useMessage()`。
- App.vue 已挂 `NMessageProvider`（项目记忆 project_t006_insights），`useMessage()` 可用。
- 抽一个**组件内私有** async 函数 `copyText(text: string): Promise<boolean>`，1:1 搬运 LogViewer:147-171 逻辑：
  ```
  try { await navigator.clipboard.writeText(text); message.success('已复制到剪贴板'); return true }
  catch { 临时 textarea + execCommand('copy'); ok? message.success(...) : message.error('复制失败：请手动选择文本复制'); return ok }
  ```
- `copyCmd(cmd)`：`if (await copyText(cmd)) { copiedCmd.value = cmd; setTimeout(...2000) }` —— 保留"已复制 ✓"短暂态仅在成功时触发。
- `copyAll()`：同款，成功才置 `copiedAll = true`。

### PublicIpDetector.vue
- 引入 `useMessage`（已 import NButton/NAlert）。
- `copyIp()`：同款 `copyText` 私有函数；成功才置 `copied.value = true` + setTimeout。

### 失败反馈一致性
- 三处文案与 LogViewer 完全对齐：成功 `'已复制到剪贴板'`、失败 `'复制失败：请手动选择文本复制'`。

## 2. (B) 重置防误丢 — 设计

### Server.vue
- 引入 `import ConfirmDialog from '../components/ConfirmDialog.vue'`（T-056/Proxies 范式）。
- 新增 `ref` 状态：`loadedSnapshot = ref<ServerForm | null>(null)`、`reloadConfirmShow = ref(false)`。
- `loadConfig` 成功分支末尾：`loadedSnapshot.value = { ...form.value }`（在 6 字段赋值之后）。
- 新增 `isDirty(): boolean`：`loadedSnapshot.value == null ? false : 6 字段任一不等`。
- 新增 `handleReloadClick()`：`if (isDirty()) reloadConfirmShow.value = true; else void loadConfig()`。
- 新增 `confirmReload()`：`void loadConfig()`（ConfirmDialog confirm 事件触发；ConfirmDialog 自身 emit update:show false）。
- 模板：按钮文案"重置"→"重新加载"，`@click="handleReloadClick"`；末尾加
  `<confirm-dialog v-model:show="reloadConfirmShow" title="重新加载配置" content="将放弃当前未保存的修改并重新加载配置，确定？" @confirm="confirmReload" />`。
- `defineExpose.__testing` 追加 `loadedSnapshot, reloadConfirmShow, isDirty, handleReloadClick`（测试断言用）。

### Client.vue
- 同款，`form` 是 3 字段（serverAddr/serverPort/authToken）。
- 注意 Client.vue 现有 `@click="loadConfig()"`（带括号）→ 改 `@click="handleReloadClick"`。

### dirty 不覆盖 AllowPortsEditor（Server.vue 已知局限）
- 见 D2。06 记录："仅改 allowPorts 行、标量未动 → 直接 reload 不确认"的局限。

## 3. (C) Wizard 死分支 — 设计

`web/src/pages/Wizard.vue:85-90` 两个相同文案的 `<n-text>` 合并为单个无条件：
```
<n-text strong style="display: block; margin-bottom: 12px">
  frpc 客户端配置
</n-text>
```
外层 `<div v-if="selectedRole === 'frpc' || selectedRole === 'both'">`（L84）已控可见性，删除内层 `v-if/v-else` 不改变任何渲染结果。纯清理。

## 4. Partition assignment

- 单 Developer 模式（无 `.harness/agents/dev-*.md` 检测，按 PM 协议）→ 全部由 `dev-frontend` 角色实现。
- 涉及文件：`web/src/components/FirewallHint.vue`、`web/src/components/PublicIpDetector.vue`、`web/src/pages/Server.vue`、`web/src/pages/Client.vue`、`web/src/pages/Wizard.vue` + 各自 spec + `scripts/baseline.json`。
- partition: `dev-frontend`（仅前端，无后端/db 改动）。

## 5. 测试设计（dev 照做；断言禁按 naive-ui 组件名 — insight L45 / AC-X4）

### (A) FirewallHint.spec.ts（新建）
- mock `naive-ui` importOriginal + `useMessage: () => messageSpies` 单例 spy（T-057 Wizard.spec 范式）。
- mock `navigator.clipboard`（`Object.defineProperty(navigator,'clipboard',{value:{writeText: vi.fn()},configurable:true})`）+ `document.execCommand`（`vi.spyOn(document,'execCommand')`）。
- 用例：
  - writeText resolve → 点"复制" → `messageSpies.success` 调用 `'已复制到剪贴板'` + 按钮文案变"已复制 ✓"（用 `wrapper.text()` 断言，不查组件名）。
  - writeText reject + execCommand 返回 true → `messageSpies.success`。
  - writeText reject + execCommand 返回 false → `messageSpies.error` `'复制失败：请手动选择文本复制'`。
  - copyAll 同款一条。
  - 渲染断言用 `wrapper.find('button')` / `wrapper.text()` / DOM class，禁 `findComponent({name:...})`。

### (A) PublicIpDetector.spec.ts（扩展现有）
- 现有用 `withProvider` + 真 NMessageProvider。改为加 `useMessage` 单例 spy（或保留真 provider 但用 clipboard mock 断言 message 渲染文本）。**决策**：改用 `vi.mock('naive-ui')` 单例 spy 方式（与 Server.spec 一致），保持现有"检测失败"文本断言不变。
- 新增 copyIp 三态用例（成功 message.success / fallback 成功 / fallback 失败 message.error）。

### (B) Server.spec.ts + Client.spec.ts（扩展）
- 用 getExposed 读 `isDirty / handleReloadClick / reloadConfirmShow / loadedSnapshot / loadConfig`。
- 用例：
  - 加载后未改 → `isDirty()===false`；点"重新加载"（DOM 找文案为"重新加载"的 button 或调 handleReloadClick）→ 直接 loadConfig（断言 `apiGetServer/apiGetClient` mock 再被调用、`reloadConfirmShow` 仍 false）。
  - 改一个字段使 dirty → `isDirty()===true`；handleReloadClick → `reloadConfirmShow===true` 且此刻 apiGet **未**再被调用。
  - dirty + confirm → confirmReload → apiGet 再被调用。
  - dirty + cancel（不调 confirmReload）→ apiGet 未再被调用。
  - 文案断言：`wrapper.text()` 含"重新加载"、不含旧"重置"。

### (C) Wizard.spec.ts（扩展）
- selectedRole='both' → step2 含恰一次"frpc 客户端配置"标题（`wrapper.text().match(/frpc 客户端配置/g).length === 1`）。
- selectedRole='frpc' → 同样恰一次。
- 用 getExposed 设置 selectedRole + currentStep 或经 DOM 操作（参考现有 Wizard.spec 句柄）。

### baseline bump
- 统计本任务新增 vitest 用例数 N（dev 在 04 报告 `Tests N passed` 实测口径），`frontend_tests += N`、`test_count += N`。当前 frontend_tests=454 / test_count=772。

## 6. Backlog（不在本任务）

- 抽 `web/src/utils/clipboard.ts::copyToClipboard(text): Promise<boolean>` 复用 LogViewer/FirewallHint/PublicIpDetector，消除三处重复 + 让 LogViewer 改用。需同步迁移 LogViewer.spec 的 clipboard 断言。低优先，独立任务。

## 7. 风险闭环

- **R1** → D2 决策（dirty 不含 AllowPortsEditor，记局限）。
- **R2** → 04 确认 e2e 不断言 Server/Client/Wizard 文案。
- **R3** → D1 决策（不抽 util）。
