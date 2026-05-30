# 07_DELIVERY — T-043 frontend-test-suite-repair

> 状态：**DELIVERED**（pending archive）· 2026-05-30 · batch project-optimization-2026-05

## 需求

修复主干 main 上前端 `vitest` 的 39 个失败（B.3 闸门红），恢复 verify_all B.3 绿。这是 batch 启动时发现的**红线违规**：最近批次 T-038~T-042 带红测试树交付（声明完成前未真跑 verify_all）。

## 根因（两类）

1. **38 个失败 = 测试 helper 脆弱性**。`getTesting()` 用 `wrapper.findComponent(C).vm.__testing` 读 `defineExpose` 暴露的句柄。该写法依赖 VTU `createVMProxy` 把 `vm.$.exposed` 透传到 `vm` 上，而透传条件是 `vm.$.exposed && vm.$.exposeProxy && key in vm.$.exposeProxy`（见 `node_modules/@vue/test-utils/dist/vue-test-utils.cjs.js:7470`）。`exposeProxy` 是否被创建取决于实例是否被父级 ref 访问过 —— 不可靠：同样的 `defineExpose({ __testing: {...} })`，LogViewer 能取到、ServerMonitor/Proxies 取到 `undefined`。实测 `vm.$.exposed` 两者都有 `__testing`，差别只在 `exposeProxy`。
2. **1 个失败 = 测试 mock 错误**。`useServerRuntime.spec` 及 4 个 ServerMonitor/qa_t041 用例用 `new Error('凭据失效')` reject，但实现用 `extractErrorMessage`（只透传**结构化 axios 错误**的 message，普通 Error 走友好 fallback）。普通 Error → `error.value` 变成通用 fallback → 断言"含具体关键词 / goServerHint=true"失败。

**关键判定**：生产路径是**正确**的 —— 后端 `handlers_server_runtime.go` 确实返回结构化错误且含精确关键词（"frps 进程不可达…"、"…凭据校验失败…"、"…dashboard 未启用…"），axios 包成 AxiosError → `extractErrorMessage` 透传 → `goServerHint` 匹配，功能在浏览器里是好的。是**测试 mock 没模拟真实后端响应**，不是组件/实现 bug。故不改 `extractErrorMessage`（"只透传结构化消息、普通错误走友好 fallback"是有意且正确的 UX 设计）。

## 方案与改动

- 新增 `web/src/test-utils/exposed.ts`：`getExposed<T>(wrapper, Component, key='__testing')` —— 健壮读取 `defineExpose` 句柄，先试 `vm[key]`（兼容旧路径）再回落规范的 `vm.$.exposed[key]`。全项目统一改用，根除该脆弱性。
- 新增 `web/src/test-utils/apiError.ts`：`apiError(message, status, code)` —— 构造与真实后端同形状的 axios 错误（`isAxiosError:true` + `response.data.error.message`），让测试正确模拟"后端返回带具体原因的错误"。
- 改 7 个 spec 文件的 `__testing` 访问点（ServerMonitor.spec / Proxies.spec / qa_t041 / qa_t042 / LogViewer.spec / qa_t036_perf；含 LogViewer 两个本已通过的文件，统一健壮化防未来回归）。
- 把 4 个断言具体后端消息/`goServerHint` 的用例改用 `apiError(...)`，让它们真正验证生产集成路径（extractErrorMessage 透传 + goServerHint 关键词匹配）。
- `useServerRuntime.spec` 那条普通 Error 用例的期望对齐正确契约（fallback 文案）。

## 验证

- `npx vitest run`：**297 passed / 28 files**（修复前 39 failed）。
- `npx vue-tsc --noEmit`：通过（修复初版 exposed.ts 触发 TS2339，已 cast `as VueWrapper` 修正）。
- `bash scripts/verify_all.sh --quick`：**PASS 30 / FAIL 1**（B.3 红→绿，B.1 typecheck 保持绿）。仅余 E.6（3 个旧归档报告缺对抗段）—— 属 batch 启动时的另一独立 pre-existing FAIL，由 T-044 修复，非本任务引入。

## Adversarial tests

- 转 `apiError` 后，ADV-1（frps 进程不可达 → 文案含关键词 + `goServerHint=false`）、ADV-2（凭据失败 → `goServerHint=true` + 按钮 + 导航 `/server`）真正走"结构化后端错误 → extractErrorMessage 透传 → goServerHint 匹配"链路，而非此前误打误撞靠 fallback。
- 反向证伪：ADV-2 反向用例（`apiError('一般网络错误')`，message 不含关键词）锁死 `goServerHint=false`，证明判定真的按关键词、非恒真。
- `getExposed` 的回落分支用真实组件双实例（LogViewer 走 `vm[key]`、ServerMonitor/Proxies 走 `vm.$.exposed[key]`）双向覆盖。

## Insight

- VTU `findComponent(C).vm.<exposedKey>` 读 `defineExpose` 是**脆弱反模式**：依赖 `vm.$.exposeProxy` 被 Vue 创建（取决于实例是否被父级 ref 访问），同款 `defineExpose({__testing})` 在不同组件下一个能取一个取 `undefined`，曾让整条前端测试基线变红半批次无人发现。规范做法读 `vm.$.exposed[key]`（defineExpose 后必然存在）。统一封装 `getExposed`（先 `vm[key]` 再回落 `vm.$.exposed[key]`）根治。任何 `defineExpose({__testing})` + spec 读 internals 的组件必须走这个 helper。
- 前端测试模拟 API 失败必须用 axios 形状错误（`isAxiosError:true` + `response.data.error.message`），不能用 `new Error()`：`extractErrorMessage` 只透传结构化错误的 message，普通 Error 走 fallback。用 `new Error()` 会让"断言 UI 显示具体后端原因/按错误关键词分流"的测试误判（fallback 不含关键词时恰好不报错 → 假绿/假红）。统一用 `apiError()` helper。
- 红线复发的根因是 verify_all 的 QA/verify 阶段被角色扮演而非真跑（insight L14 role-collapse 的延伸危害）。修复手段在 T-044：让 B.4 真计数 + 后续 batch orchestrator 真跑 verify_all。
