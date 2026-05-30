# 任务输入 — T-066 · dark-theme-support

- **Mode**: full（完整 7-stage）
- **批次**: ux-ui-uplift-2026-05（第 5 个，最大的一个）
- **分区**: 全部前端 → dev-frontend 单分区
- **输出语言**: 中文（红线）

## 一句话目标

加生产暗色主题（跟随系统 + 手动切换 + 持久化），激活已投入一半的主题感知基建——日志子系统（T-036）已用 `useThemeVars()` 投射 CSS 变量且 `LogViewer.spec` 已有 `darkTheme` 测试、ServiceStatusCard（T-038）也用了 `useThemeVars`，但全站 App.vue 仍是裸 `<n-config-provider>` 只能浅色，这半套工程用户拿不到收益。

## 证据 / 现状

- `web/src/App.vue`：根组件是裸 `<n-config-provider>`（无 `:theme`、无 `useOsTheme`、无切换入口）→ 全站只能浅色。已读确认实为：
  ```
  <n-config-provider>
    <n-message-provider>
      <router-view />
    </n-message-provider>
  </n-config-provider>
  ```
- 对照：`web/src/components/LogViewer.vue`（用 `useThemeVars()` + 注释"切主题 0 额外代码即跟随 AC-13"）、`LogViewer.spec.ts`（已用 `darkTheme` mount）、`web/src/components/ServiceStatusCard.vue`（`useThemeVars`）——基建就绪，缺全局开关。
- 散落硬编码 hex（行号因前序任务可能漂移，按内容定位）：`#888` 文字色（Wizard.vue 多处、PublicIpDetector.vue）；`background:#f5f5f5`（Login.vue、Setup.vue、FirewallHint.vue）；`#f0f2f5`（Wizard.vue 页面背景）；品牌绿 `#18a058`（AppLayout.vue、Wizard.vue）；橙 `#f0a020`（ServiceStatusCard.vue）。

## 范围与设计方向（全部前端，dev-frontend 分区）

1. **全局主题状态**：新建一个轻量主题状态层（建议 `web/src/composables/useTheme.ts` 或 `stores/theme.ts`），持有 `'light' | 'dark' | 'auto'` 偏好，localStorage 持久化，`auto` 用 `useOsTheme()`（Naive UI 内置）跟随系统。导出当前生效 theme（`null`=浅色 / `darkTheme`=暗色）供 App.vue 消费。降级：localStorage 不可用时内存降级（参考 useLogPrefs 的 BC-13 范式）。
2. **App.vue**：`<n-config-provider :theme="...">` 绑定上述状态。加 `<n-global-style />`（当前全站无）让 body 背景随主题自动切换——修 Login/Setup/Wizard 整页 `background:#f5f5f5/#f0f2f5` 硬编码的最干净办法。
3. **顶栏切换入口**：在 `AppLayout.vue` 顶栏（用户名/退出附近）加主题切换控件（light/dark/auto 三态按钮或下拉），给无障碍名（aria-label，延续 T-064 a11y 风格）。注意：Login/Setup/Wizard 是顶级路由不在 AppLayout 内（insight L30/L31）→ 这些页无页内切换入口，但因 App.vue 的 NConfigProvider 包裹整个 router-view，它们仍会跟随全局/持久化主题。这是可接受的范围边界，显式记录（不要为这几页单独造切换入口，属 scope creep）。
4. **硬编码颜色 token 化**：把上述散落 hex 改成主题感知值——`useThemeVars()` 的语义 token（`textColor3`/`bodyColor`/`primaryColor`/`warningColor` 等）或 CSS 变量。页面背景优先靠 `<n-global-style>` 解决，剩余文字/品牌/状态色用 themeVars。不要动 LogViewer 子系统（已就绪，会自动跟随）。
5. **暗色逐页可读性核验**：所有页面（Setup/Login/Dashboard/Proxies/Server/ServerMonitor/Client/Logs/Settings/Wizard）+ 关键组件在暗色下可读，无白底黑字残留、无不可读灰字。

## 硬约束 / 红线

- 零新依赖（Naive UI 已提供 `darkTheme`/`useOsTheme`/`useThemeVars`/`NGlobalStyle`）。
- 浅色主题视觉不得回归（默认仍是浅色，除非系统暗色 + auto 或用户手选暗色）。
- insight L16：禁硬编码 `rgba(255,255,255,*)` 文字色；改用语义 token。
- 测试参考 `LogViewer.spec` 的 `darkTheme` 范式；断言优先可观察量（localStorage 持久化值、主题状态、`useOsTheme` 默认、切换后状态）经 DOM/store 查询，尽量少用 naive-ui 组件名查询（insight L45）。复用 `web/src/test-utils/`。
- 新增测试同步 bump `scripts/baseline.json` 的 `frontend_tests`/`test_count`（+version）。
- e2e 不受影响须核实（01-setup/02-auth/03-dashboard 不断言主题；若新增顶栏切换控件改变 03-dashboard 可见元素，需确认断言无碍；insight L34）。
- 更新 `docs/dev-map.md`（App.vue 主题 + 新 useTheme/theme store + 可复用工具表）。

## 产出要求

- `docs/features/dark-theme-support/` 下 7 份阶段文档 + PM_LOG.md，全中文。
- `06_TEST_REPORT.md` 含裸标题 `## Adversarial tests` 段（反向证伪：默认浅色不回归 / auto 跟随 OS（mock useOsTheme）/ 手选暗色持久化且重载后保持 / localStorage 不可用时不崩内存降级 / 顶级路由页仍跟随全局主题）。
- `07_DELIVERY.md` 含裸标题 `## Insight` 段。

## 交付与验证边界

- PM 上下文无 Bash/PS，标 verify_all=PENDING + 执行规格（预期 PASS、frontend_tests 新计数）。真跑由 batch orchestrator 执行作硬闸门，特别复核 LogViewer/ServiceStatusCard 既有 spec 零回归。
- 不要 commit / push / archive。
- 当前 baseline：frontend_tests=552 / go_tests=342 / test_count=894 / version=32。

## 范围纪律提醒

这是本批次最大任务，Gate Reviewer 须确认范围可控、可一次交付；若 SA 判定范围过大有失控风险，可在 02 设计里收敛（如先做"主题切换 + 背景 + 文字色"核心，把个别边缘组件色微调记为同任务内 follow 或明确 OOS），但核心暗色切换 + 持久化 + 跟随系统必须交付。遇同 stage 3 次回退立即返回 FAILED。

## RA 起手

RA 先扫 docs/tasks.md 找 T-036（日志主题感知）/ T-038（ServiceStatusCard themeVars）/ T-048（可读语义色）历史，复用其 themeVars 范式。
