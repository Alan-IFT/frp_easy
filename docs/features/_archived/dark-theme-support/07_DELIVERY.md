# Delivery Summary — T-066 · dark-theme-support

- Task: dark-theme-support — 加生产暗色主题（跟随系统 useOsTheme + 手动 light/dark/auto 切换 + localStorage 持久化），激活 T-036/T-038 已就绪的 useThemeVars 主题感知基建。
- Mode: full（7-stage）
- 批次: ux-ui-uplift-2026-05（第 5 个，最大任务）
- 分区: dev-frontend 单分区（纯前端）

## Stages traversed（含时间戳）

| Stage | 角色 | 产出 | 裁决 | 时间 |
|---|---|---|---|---|
| 1 | Requirement Analyst | `01_REQUIREMENT_ANALYSIS.md` | READY（带默认假设：默认偏好=auto） | 2026-05-31 |
| 2 | Solution Architect | `02_SOLUTION_DESIGN.md` | READY | 2026-05-31 |
| 3 | Gate Reviewer | `03_GATE_REVIEW.md` | APPROVED WITH CONDITIONS（8 维全 PASS，C-1~C-5） | 2026-05-31 |
| 4 | Developer (dev-frontend) | `04_DEVELOPMENT.md` | READY FOR REVIEW（0 partition block / 0 drift） | 2026-05-31 |
| 5 | Code Reviewer | `05_CODE_REVIEW.md` | APPROVED（0 CRIT/0 MAJ/0 MIN/2 NIT） | 2026-05-31 |
| 6 | QA Tester | `06_TEST_REPORT.md` | APPROVED FOR DELIVERY（0 缺陷） | 2026-05-31 |
| 7 | PM | `07_DELIVERY.md` | DELIVERED | 2026-05-31 |

> 注：本任务在 role-collapsed PM 上下文执行（Task 工具不可用，insight L31 同源现象），PM 逐角色扮演各 stage 并按 agent 契约逐份落盘，PM_LOG.md 记录每次路由决策。

## Rollbacks

**0 次**。无任何 stage 回退。GR APPROVED WITH CONDITIONS 一次过、CR APPROVED 一次过、QA APPROVED FOR DELIVERY 0 缺陷。CR flag 的 ServiceStatusCard --warn class 回归风险经 PM 主动 grep 确证为伪命题（无该 spec、无 class 断言），未触发回退。

## Final verify_all result

**PENDING（预期 PASS）**。PM/dev/QA role-collapsed 上下文无 Bash/PS（insight L31）→ verify_all 全量真跑交 batch orchestrator Bash 会话作交付硬闸门。执行规格（确定性预测全绿）：
- frontend_tests: 552 → **576**（+24：dev 19 + QA 独立对抗 5）
- go_tests: 342（不变）
- test_count: 894 → **918**
- 预期 Pass=918 / Fail=0 / Warn=0
- 特别复核：LogViewer.spec 既有 darkTheme 用例零回归 / ServiceStatusCard 无独立 spec 删 scoped --warn 零回归 / AppLayout.spec 既有 7 图标 a11y +3 新用例 / useTheme·App·qa_adv 新 spec 可挂载 / e2e 03-dashboard 不受 n-select 影响。

## Baseline changes

- `scripts/baseline.json`：version 32→33 / test_count 894→918 / frontend_tests 552→576 / go_tests 342（不变）。

## Files changed（15）

**新建（4）**：
- `web/src/composables/useTheme.ts` — 主题状态层（模块级单例 composable）
- `web/src/composables/__tests__/useTheme.spec.ts` — 12 用例
- `web/src/__tests__/App.spec.ts` — 4 用例
- `web/src/composables/__tests__/qa_t066_adversarial.spec.ts` — QA 独立对抗 5 用例

**编辑（11）**：
- `web/src/App.vue` — `:theme=activeTheme` 绑定 + `<n-global-style/>`
- `web/src/components/AppLayout.vue` — n-select 三态切换控件（aria-label）+ 品牌绿→primaryColor
- `web/src/pages/Login.vue` / `Setup.vue` — 删整页 `background:#f5f5f5`
- `web/src/pages/Wizard.vue` — 删 `#f0f2f5` + 品牌/文字/图标色→themeVars
- `web/src/components/FirewallHint.vue` — 命令块背景→codeColor
- `web/src/components/PublicIpDetector.vue` — advisory 文字→textColor3
- `web/src/components/ServiceStatusCard.vue` — warn 边框 #f0a020→:style computed warningColor（删 scoped 块）
- `web/src/components/__tests__/AppLayout.spec.ts` — +3 用例
- `scripts/baseline.json` / `docs/dev-map.md` — 同步

**未碰**：后端 Go / store / 路由守卫 / API / DB / migration / e2e spec / LogViewer 日志子系统（OOS-2，自动跟随）。零新依赖（package.json 未改）。

## Outstanding risks

- 浅色不回归：默认 auto（OS 浅→浅）+ light 恒浅 + token 用语义值，确定性论证不回归；真跑核对作硬闸门。
- ServiceStatusCard warn 高亮由 :class 改 :style computed：语义保留（needsFix 时仍 border+inset），无 spec 断言旧 class 名（已 grep 确证）。
- 顶级路由页（/login /setup /wizard）无页内切换入口但跟随全局 NConfigProvider 主题——**可接受范围边界**（insight L30/L31，OOS-1 显式记录，QA-ADV-5 验证主题 context 穿透）。非缺陷。
- 真跑硬闸门未在本上下文执行（无 Bash）；交 batch orchestrator。

## Next steps for user

- batch orchestrator 在 Bash 会话跑 `scripts/verify_all` 确认 PASS + frontend_tests==576。
- 按批次约定：本任务**未 commit / 未 push / 未 archive**，由 batch orchestrator 统一处理。
- 可视核验建议：暗色下逐页扫一遍（Setup/Login/Dashboard/Proxies/Server/ServerMonitor/Client/Logs/Settings/Wizard），确认无白底黑字残留。

## Insight

- 2026-05-31 · Naive UI 全站暗色主题的状态层宜做**模块级单例 composable**（模块作用域 `pref` ref + `useTheme()` 返回它）而非 Pinia store——App.vue 与 AppLayout 等多组件需共享同一偏好且无需 devtools/SSR；但 `useOsTheme()` 内部用 onMounted/inject 必须在组件 setup 内同步调用，故 osThemeRef 须**惰性化**（首次调用在 App.vue setup 触发）且 dark/light/setPref 分支不读 OS（`osThemeRef?.value` 可选链 null 安全），使该单例在测试/非 setup 调用 setPref 也不抛错 · evidence: T-066 useTheme.ts:85-130 + 03 §3 Q2 + useTheme.spec/App.spec/qa_adv 受控 osThemeRef mock
- 2026-05-31 · 测试**模块级单例 composable** 的"不同初始持久化态"（默认/预置 dark/非法值/重载读回）必须用 `vi.resetModules()` + 动态 `import()` 拿全新模块实例——否则模块顶层 `const pref = ref(readPref(localStorage))` 只在首次 import 求值一次，后续用例改了 localStorage 也读不到（被首次加载值污染）；这是单例可共享性的代价。配套：跨用例共享单例的 spec（如 AppLayout.spec 一次性 import）须 `beforeEach` 显式 `setPref(DEFAULT)+localStorage.clear()` 复位防顺序敏感 · evidence: T-066 useTheme.spec/App.spec freshUseTheme() + AppLayout.spec beforeEach 复位 + GR C-1
- 2026-05-31 · "顶级路由页不嵌 AppLayout"（insight L30/L31）的反面同样成立且常被误判：这些页虽无 AppLayout 顶栏入口，但**仍被 App.vue 根 `<n-config-provider :theme>` 包裹整个 router-view**，故主题/message 等经 inject 的全局 context 照常穿透——"在 AppLayout 内"只决定**顶栏 UI 元素**可见性，不决定 provide/inject context 可达性。给全站加主题时顶级路由页"无切换入口但跟随主题"是正确范围边界而非遗漏，验证手法是把任意 Probe 组件挂 NConfigProvider{darkTheme} 下断言 useThemeVars 派生值变化（等价顶级路由页场景）· evidence: T-066 router.ts:8-26 + QA-ADV-5 主题 context 穿透 + OOS-1
- 2026-05-31 · scoped `<style>` 块的硬编码状态色（如 needsFix 高亮 #f0a020）无法读 `useThemeVars()`（scoped css 是静态），主题感知化的最小改法是把该样式从 scoped 类移到 `:style` computed 绑定（needsFix 时返样式对象、否则空对象）走 themeVars——同文件若已有 `:style` computed 范式（ServiceStatusCard 的 cmdBlockStyle 走 codeColor）则零新模式。删 scoped 类前必 grep 确认无测试断言该 class 名（本任务该组件根本无独立 spec，删 class 对基线零影响，再证 insight L42：评抽取/删除影响必实查测试断言什么）· evidence: T-066 ServiceStatusCard.vue:151-158 warnCardStyle + grep svc-status-card--warn 全 src 仅 1 行注释无测试断言
