# 04 开发 — T-058 frontend-interaction-polish

- **开发角色**: dev-frontend（单 Developer 模式，无 dev-* 分区文件）
- **日期**: 2026-05-30
- **依据**: 02_SOLUTION_DESIGN.md + 03_GATE_REVIEW.md conditions C-1~C-5

## 1. 实现清单

### (A) 剪贴板失败反馈
- `web/src/components/FirewallHint.vue`：
  - 引入 `useMessage`（`import { NAlert, NButton, useMessage } from 'naive-ui'` + `const message = useMessage()`）。
  - 新增组件内私有 `copyText(text): Promise<boolean>`，1:1 搬运 `LogViewer.vue:147-171` 范式。
  - `copyCmd`/`copyAll` 改为 `if (await copyText(...)) { 置短暂态 + setTimeout }`——"已复制 ✓"仅成功触发。
  - 删除两处空 `catch {}`。
- `web/src/components/PublicIpDetector.vue`：同款 `useMessage` + `copyText` + `copyIp` 改造。

### (B) 重置防误丢
- `web/src/pages/Server.vue`：
  - 引入 `ConfirmDialog`。
  - 新增 `loadedSnapshot`（标量字段快照）、`reloadConfirmShow`、`isDirty()`、`handleReloadClick()`、`confirmReload()`。
  - `loadConfig` 成功分支末尾 `loadedSnapshot.value = { ...form.value }`。
  - 模板按钮"重置"→"重新加载"，`@click="handleReloadClick"`；末尾加 `<confirm-dialog v-model:show ... @confirm="confirmReload" />`。
  - `defineExpose.__testing` 追加 5 个句柄。
- `web/src/pages/Client.vue`：同款（3 标量字段）。注意原 `@click="loadConfig()"` 改为 `@click="handleReloadClick"`。

### (C) 死分支清理
- `web/src/pages/Wizard.vue`：L85-90 两个相同文案 `<n-text>` 合并为单个无条件 `<n-text strong>frpc 客户端配置</n-text>`，加注释说明。

## 2. 测试清单（+27 前端测试）

| 文件 | 新增 | 内容 |
|---|---|---|
| `web/src/components/__tests__/FirewallHint.spec.ts`（新建） | 7 | copyCmd 成功/fallback 成功/fallback 失败 message.error + copyAll 成功/失败 + Adversarial（双重失败仍 message.error 不抛 / 临时 textarea 移除） |
| `web/src/components/__tests__/PublicIpDetector.spec.ts`（扩展） | 4 | copyIp 成功/fallback 成功/fallback 失败 + Adversarial 双重失败 |
| `web/src/pages/__tests__/Server.spec.ts`（扩展） | 7 | B 块 6（文案/不 dirty 直接重载/dirty 弹确认不调 apiGet/确认重载/取消不重载/快照刷新）+ Adversarial 1（dirty 不静默重载反向证伪） |
| `web/src/pages/__tests__/Client.spec.ts`（扩展） | 6 | 同款 5 + Adversarial 1 |
| `web/src/pages/__tests__/Wizard.spec.ts`（扩展） | 3 | frpc/both 各恰一次标题 + frps 不渲染 |

### C-1 自检（关键，T-057 刚踩坑）——断言查询方式逐条核对
- FirewallHint/PublicIpDetector：`wrapper.findAll('button').find(b => b.text().includes(...))` 按**可见文本**定位按钮；`wrapper.text()` 断言"已复制 ✓"；`document.querySelectorAll('textarea[aria-hidden]')` 按 DOM 属性。**零** `findComponent({name:...})`。
- Server/Client：`getExposed` 读句柄 + `wrapper.text()` 断言文案 + `mock.calls.length` 断言 apiGet 调用次数。
- Wizard：`wrapper.text()` 计数子串。
- **结论：全部 27 个新断言不依赖 naive-ui 组件名查询。**

### C-2 message spy 单例
- FirewallHint.spec / PublicIpDetector.spec：`vi.mock('naive-ui')` 工厂内定义 `messageSpies` 单例 + 导出 `__messageSpies`，测试取回断言。避免 `useMessage()` 每次返回新对象无法断言。

### C-5 已知局限
- dirty 检测仅覆盖 `form` 标量字段，不含 AllowPortsEditor 子组件内部行编辑状态（见 02 D2）。记入 06 已知局限段。

## 3. Conditions 消化（insight L17：好的 dev 在自然顺手时一并消化所有 C-N）

- **C-1**（断言不依赖组件名）→ 已落实 + 本文 §2 逐条自检 + 04 末尾再确认。✅
- **C-2**（message 单例 spy）→ 已落实。✅
- **C-3**（e2e 不断言文案）→ 已 grep 确认：`web/tests/e2e/{01-setup,02-auth,03-dashboard}.spec.ts` 无任何"重置"/"重新加载"/"frpc 客户端配置"/"复制"断言；03-dashboard 用 `bypassWizard` 绕过向导，不渲染 Wizard step2。本改动对 e2e 零影响。✅
- **C-4**（裸 `## Adversarial tests` + clipboard reject→fallback 失败→message.error 反向证伪）→ 06 落实。✅
- **C-5**（dirty 局限）→ 06 已知局限段。✅

## 4. Design drift / 决策落地

- D1（不抽 util）：未创建 `utils/clipboard.ts`，FirewallHint/PublicIpDetector 各自内联 `copyText`。两处逻辑结构相同但独立——抽取记 backlog。无 dev-map 结构变化（仅文件注释更新）。
- D2（dirty 不含 AllowPortsEditor）：实现按此，`isDirty()` 仅比 6/3 标量字段。

## 5. eslint / SFC 行数红线

- 无新增空 `catch`，无 `any` 滥用（测试中 `as unknown as` 取回 mock spy 是受控类型断言）。
- Server.vue 加 ~50 行（含注释），纯逻辑（非 import/template/expose 的 setup 行）仍 < 200。Client.vue 同。Wizard.vue 净减 3 行。
- 未触碰 `.claude/` / `CLAUDE.md` / `.github/`。

## 6. verify_all（由 orchestrator Bash 会话真跑作硬闸门）

- dev 上下文：本地未跑全量；静态预测全绿。
- baseline 已 bump：`frontend_tests` 454→481、`test_count` 772→799。
- 交 orchestrator 真跑 `bash scripts/verify_all.sh`（全量含 e2e）。verify 结果回填本节。

### verify_all 真跑结果（orchestrator 回填）

见下方 §7。
