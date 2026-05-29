# 07_DELIVERY — T-047 frontend-honest-states

> 状态：**DELIVERED**（pending archive）· 2026-05-30 · batch project-optimization-2026-05 · 由 dev-frontend 子 agent 实现，orchestrator 真跑 verify_all 闸门

## 需求

让前端"诚实"：消除"默认值/空列表伪装成真实状态"误导用户的 UX 缺陷（前端 UX 审计 A1/A2/A3 + 表单校验 B1）。

## 改动

- **A1**（Server.vue / Client.vue）：`loadConfig` 期间走 skeleton 加载态；失败显示 `n-result`（status=error）+ 重试按钮，**失败态不渲染表单**（杜绝默认值被误当真实配置覆盖）；成功才渲染表单。三分支 `v-if(loadError)/v-else-if(loading)/v-else` 天然互斥（insight L39 布尔代数式）。Settings.vue 经判断无初始数据拉取，无此风险，跳过。
- **A3**（stores/proxies.ts + Proxies.vue）：`fetchProxies` 加 try/catch + `error: string|null` state（`extractErrorMessage` 写入，失败保留旧列表不清空）；页面 `error!=null` 显示 `n-result` 错误态 + 重试，与空态 `#empty` 互斥 —— 不再把"加载失败"渲染成"暂无规则"。
- **A2**（Dashboard.vue）：`fetchMode` 失败不再静默，改 `message.warning(extractErrorMessage(...))` + `modeFetchFailed` 标记；失败时开关 disabled + tooltip"状态获取失败，请点击刷新" + "刷新状态"重试按钮 —— UI 不再撒谎。
- **B1**（Server.vue）：dashboard 三字段补 rules（端口 1-65535 整数；启用 Dashboard 时 user/pass required；validator 首行 `if (!dashboardEnabled) return true` 未启用不阻塞）。

## 验证

- `npx vitest run`：**327 passed**（297→327，+30；新增 Server.spec 13 / Client.spec 6 / Dashboard.spec 7 + Proxies.spec 扩 +4）。`vue-tsc --noEmit` 无错；`eslint .` 无错。
- orchestrator 真跑 `bash scripts/verify_all.sh`（完整含 e2e）：**PASS 32 / WARN 0 / FAIL 0**。baseline.json v20（frontend_tests 327 / test_count 614）。
- orchestrator 抽查生产改动 diff：proxies store / Dashboard 改动 idiomatic（extractErrorMessage + naive-ui 模式），无过度工程。

## Adversarial tests

- 每个改动带反向构造：「加载失败时绝不渲染成空列表/默认值」「三态互斥：error 时 loading 必为 false」「获取失败 UI 不撒谎（开关呈失败态/message.warning 被调用）」。
- 测试全程遵守 T-043 约定：`getExposed` 读 expose（非 `vm.__testing`）、`apiError` 构造失败（非 `new Error()`，确保 extractErrorMessage 透传断言成立）。
- 未删除/弱化任何现有测试（红线#3）。

## Insight

- "默认值表单"在加载失败时是 UX 反模式：用户会把空表单/默认值当成"当前真实配置"进而误操作覆盖。正解是**失败态根本不渲染表单**（而非渲染表单+弹 toast）。三态 `v-if/else-if/else` 写成互斥分支 + 断言"error 时 loading 必 false"锁死，避免 loading+error 同显。
- store 的 fetch 失败应**保留旧数据 + 暴露 error ref**，由页面据 error 区分"加载失败"与"空"，而非 `void fetchX()` 吞 promise 让失败静默退化成空态。
- 有状态控件（开关）获取失败必须显式呈现失败态（disabled+tooltip+重试），静默停在默认值 = UI 撒谎。
