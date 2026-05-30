# 01 需求分析 — T-066 · dark-theme-support

- 任务：dark-theme-support（暗色主题支持）
- 模式：full（7-stage）
- 批次：ux-ui-uplift-2026-05（第 5 个，最大任务）
- 分区：全部前端 → dev-frontend
- 角色：Requirement Analyst（stage 1）。本文不做技术选型/模块划分（那是 Architect 的活）。

## 1. Goal（目标）

为 frp_easy Web UI 提供生产可用的暗色主题：跟随系统、可手动切换（light/dark/auto 三态）、偏好持久化，激活已就绪但全站未启用的 Naive UI 主题感知基建（T-036 日志子系统、T-038 ServiceStatusCard 已用 `useThemeVars()`，但 `App.vue` 仍裸 `<n-config-provider>` 只能浅色）。

## 2. In-scope behaviors（在范围内，编号、可测）

1. **全局主题偏好状态**：存在单一主题状态层，持有偏好值 ∈ `{'light','dark','auto'}`。导出"当前生效主题对象"：偏好为 `light` → 生效浅色（Naive UI `null` 主题）；偏好为 `dark` → 生效 `darkTheme`；偏好为 `auto` → 由系统 OS 主题决定（OS 暗 → `darkTheme`，OS 浅 → `null`）。
2. **持久化**：用户切换偏好后，偏好值写入 `localStorage`（单一已知 key）。页面重载后读回该值并恢复为生效主题。
3. **跟随系统**：偏好为 `auto` 时，生效主题随 `useOsTheme()`（Naive UI 内置 OS 主题侦测）的当前值变化而响应式变化（OS 主题改变 → 生效主题随之改变），无需重载页面。
4. **App.vue 接线**：`App.vue` 的 `<n-config-provider>` 绑定 `:theme` 到上述"当前生效主题对象"；并新增 `<n-global-style />`（当前全站无），使 `<body>` 背景/前景随主题切换。
5. **顶栏切换入口**：`AppLayout.vue` 顶栏（用户名/退出登录附近）存在主题切换控件，可在 light/dark/auto 三态间切换；切换即刻改变生效主题且触发持久化（行为 1+2）。控件带无障碍名（`aria-label`，延续 T-064 a11y 风格）。
6. **硬编码颜色 token 化（核心集合）**：以下散落硬编码 hex 改为主题感知值（语义 token 或 CSS 变量或 `n-text` depth/type），使其在暗色下不再产生"浅灰背景/浅色文字撞暗底"的不可读：
   - 整页背景 `background:#f5f5f5`（`Login.vue:2`、`Setup.vue:2`、`FirewallHint.vue:12`）与 `#f0f2f5`（`Wizard.vue:2`）——页面级背景优先靠 `<n-global-style>` + 容器透明化解决。
   - 文字色 `#888`（`PublicIpDetector.vue:27`、`Wizard.vue:26/32/38`）→ 语义 token（如 `textColor3`）或 `n-text depth`。
   - 品牌绿 `#18a058`（`AppLayout.vue:5`、`Wizard.vue:5`、`Wizard.vue:137` 图标）→ 主题 `primaryColor`。
   - 状态橙 `#f0a020`（`ServiceStatusCard.vue:185-186` 边框/box-shadow、`Wizard.vue:137` 图标）→ 主题 `warningColor`。
7. **暗色逐页可读性**：以下页面/组件在暗色主题下无白底黑字残留、无不可读灰字（可读性核验，QA 以"无残留硬编码浅色背景/文字 hex + 关键文字色走 token"为可观察判据）：Setup、Login、Dashboard（含 ServiceStatusCard）、Proxies、Server、ServerMonitor、Client、Logs、Settings、Wizard、AppLayout、FirewallHint、PublicIpDetector。
8. **默认偏好**：首次访问（localStorage 无该 key）时，默认偏好为 `auto`（见 Open Question 1 默认假设；若用户改选 `light` 见 §8）。`auto` 在 OS 为浅色时生效浅色，故"默认浅色不回归"在多数桌面环境成立。

## 3. Out-of-scope（明确不做）

1. **顶级路由页（/login /setup /wizard）无页内切换入口**：这三页是顶级路由，不嵌 `AppLayout`（insight L30/L31，已核 `router.ts` /wizard 与 / 平级），故顶栏切换控件在这些页不可见。**但因 `App.vue` 的 `<n-config-provider>` 包裹整个 `<router-view>`，它们仍跟随全局/持久化主题**。不为这三页单独造切换入口（属 scope creep）。此为可接受范围边界，须在交付文档显式记录。
2. **不动 LogViewer 日志子系统**（T-036）：`LogViewer.vue` + `log/*` 已用 `useThemeVars()` 投 `var(--log-*)` CSS 变量，会自动跟随全局主题，零改动。`LogToolbar.vue:204` 的 `var(--log-error, #d03050)` fallback 属该子系统，不动。
3. **零新依赖**：仅用 Naive UI 已提供的 `darkTheme` / `useOsTheme` / `useThemeVars` / `NGlobalStyle`。不引入任何新 npm 包。
4. **不改后端 / store 业务 / 路由守卫 / API / DB / Go 代码**：纯前端展示层。主题状态层若以 Pinia store 实现也只新增独立 store，不改既有 store。
5. **不做主题色自定义/调色板编辑器**：只在内置 light/dark 间切换，不支持用户自定义品牌色。
6. **不做主题切换过渡动画**（fade/transition）：超出本任务。
7. **边缘组件像素级色彩微调**：若某非核心组件在暗色下仅有轻微视觉瑕疵（非不可读），SA 可判定为同任务内 follow 或明确列 OOS，但 §2.6 核心集合必须 token 化。

## 4. Boundary conditions（边界条件）

- **BC-1 localStorage 不可用**（隐私模式 / Safari ITP / quota 满 / SSR 无 window）：主题状态层降级到内存（参考 `useLogPrefs.ts` 的 `createSafeStorage` BC-13 范式），不报错、不弹 message、UI 仍可切换（仅当次会话内有效，重载丢失）。
- **BC-2 非法/损坏持久化值**：localStorage 中该 key 的值不在 `{'light','dark','auto'}` 内（被外部篡改/旧版本残留）→ 降级到默认偏好（`auto`），不抛错。
- **BC-3 首次无持久化值**：localStorage 无该 key → 采用默认偏好 `auto`（§2.8）。
- **BC-4 auto 模式 OS 主题运行时切换**：偏好为 `auto` 时，OS 从浅切暗（或反向）→ 生效主题响应式跟随，无需重载、无需用户操作。
- **BC-5 useOsTheme 返回 null**（环境不支持 `matchMedia` / `prefers-color-scheme`）：`auto` 模式下视为浅色（生效 `null` 主题），不崩。
- **BC-6 偏好为 light/dark（非 auto）时**：生效主题恒定，不受 OS 主题变化影响（用户显式选择优先于系统）。

## 5. Acceptance criteria（验收标准，每条可验证）

- **AC-1**：主题状态层默认偏好为 `auto`，且在 `useOsTheme` mock 为浅色（或 null）时，生效主题为 `null`（浅色）——默认浅色不回归。（可观察：状态层导出值 + mock useOsTheme）
- **AC-2**：偏好设为 `dark` 后，生效主题为 `darkTheme`，且 `localStorage` 对应 key 值为 `'dark'`。（可观察：localStorage 值 + 状态导出）
- **AC-3**：偏好设为 `light` 后，生效主题为 `null`，localStorage 值为 `'light'`，且不受 useOsTheme（即便 mock 为暗）影响（BC-6）。
- **AC-4**：偏好为 `auto`，mock `useOsTheme` 为暗 → 生效 `darkTheme`；mock 为浅 → 生效 `null`（BC-4/AC-1）。
- **AC-5**：重载模拟（新建状态层实例，localStorage 预置 `'dark'`）→ 读回生效 `darkTheme`（持久化 round-trip，行为 2）。
- **AC-6**：localStorage 预置非法值（如 `'purple'`）→ 降级默认偏好 `auto`，不抛错（BC-2）。
- **AC-7**：localStorage 不可用（mock setItem throw / 无 window.localStorage）→ 状态层仍可构造、可切换、不抛错（BC-1，内存降级）。
- **AC-8**：`App.vue` 的 `<n-config-provider>` 的 `:theme` 绑定到状态层生效主题（结构性：组件渲染时 theme prop 反映状态）；`App.vue` 含 `<n-global-style />`。
- **AC-9**：`AppLayout.vue` 顶栏存在主题切换控件，带非空 `aria-label`；点击切换可改变状态层偏好并写 localStorage（可观察：DOM 属性 + 状态/localStorage）。
- **AC-10**：§2.6 核心硬编码 hex 集合在源码中被替换为主题感知值（静态：`Login.vue`/`Setup.vue`/`FirewallHint.vue` 不再含 `#f5f5f5` 整页背景；`Wizard.vue` 不再含 `#f0f2f5`/`#888`；品牌绿/状态橙改 token）。LogViewer 子系统的 `var(--log-*)` 保持不变。
- **AC-11**：既有 LogViewer / ServiceStatusCard / 全部既有前端 spec 零回归（包括 LogViewer.spec 的 darkTheme mount 用例）。
- **AC-12**：`scripts/verify_all` 预期 PASS；`baseline.json` 的 `frontend_tests`/`test_count`/`version` 同步 bump 到新计数。
- **AC-13**：e2e 03-dashboard TC-04（`仪表盘`/`frpc（客户端）`/`frps（服务端）` 可见）、TC-05（按 name `退出登录` 点击退出）不受顶栏新增切换控件影响——新控件不改这些文本、不遮挡这些元素。

## 6. Non-functional requirements（非功能需求）

- **NFR-1 零新依赖**（红线）：只用 Naive UI 内置能力。
- **NFR-2 浅色不回归**（红线）：默认场景（OS 浅色 + auto，或用户手选 light）视觉与当前一致。
- **NFR-3 可访问性**：切换控件有 `aria-label`（延续 T-064）。
- **NFR-4 测试可观察性**：断言优先可观察量（localStorage 值、状态导出、useOsTheme mock、DOM 属性），尽量少用 naive-ui 组件名查询（insight L45）；darkTheme mount 参考 `LogViewer.spec.ts` 范式（`NConfigProvider { theme: darkTheme }`）。复用 `web/src/test-utils/`。
- **NFR-5 不泄露内部细节 / 不破坏 SPA 状态**：切换主题不触发整页刷新（insight L17，主题切换本不涉导航，但控件实现不得用 `href`）。

## 7. Related tasks（相关历史，引用不重述）

- **T-036** log-ui-ux-polish（`docs/features/_archived/log-ui-ux-polish/`）：LogViewer 用 `useThemeVars()` 投 CSS 变量 + `useLogPrefs.ts` 的 localStorage BC-13 内存降级范式（本任务持久化层直接复用此范式）。`LogViewer.spec.ts` 的 darkTheme mount 范式是测试参考。
- **T-038** boot-autostart-hardening（`docs/features/_archived/boot-autostart-hardening/`）：`ServiceStatusCard.vue` 用 `useThemeVars().codeColor`；本任务需把其 scoped CSS 的 `#f0a020` 也 token 化。
- **T-048** frontend-consistency-cleanup（`docs/features/_archived/frontend-consistency-cleanup/`）：可读语义色范式（insight L16 的来源任务）。
- **T-062 / T-064**（本批次）：onboarding 引导 + 菜单图标 a11y；T-064 的 aria-label 风格是切换控件无障碍名参考。

## 8. Open questions for user（开放问题，附默认假设）

> 本任务为大任务但核心明确（暗色切换 + 持久化 + 跟随系统必交付）。以下问题均给默认假设，**若用户不答则采纳默认**，不阻塞。范围收敛权（token 化覆盖到哪些边缘组件、控件具体形态）属 SA，不列为 BLOCK。

1. **默认偏好是 `auto` 还是 `light`？**
   - (a) `auto`（跟随系统）—— **采纳默认**。理由：契合"加暗色主题"用户意图，且现代桌面/移动 OS 多支持 prefers-color-scheme；OS 浅色时仍生效浅色，满足"浅色不回归"。
   - (b) `light`（强制浅色，用户须手动开暗色）—— 更保守，但弱化"跟随系统"卖点。
2. **切换控件形态？**（实现细节，SA 决定，非 BLOCK）
   - (a) light/dark/auto 三态分段按钮（n-radio-button-group / n-button-group）—— 倾向，三态一目了然。
   - (b) 下拉选择（n-select / n-dropdown）—— 省横向空间。
   - (c) 单一明暗切换图标按钮（仅 light↔dark，省略 auto）—— 不满足 §2.1 三态需求，不采纳。
3. **token 化是否全量覆盖边缘组件？**（范围收敛，SA 决定，非 BLOCK）
   - (a) 核心集合（§2.6）必做，边缘瑕疵 SA 判 follow/OOS —— **采纳默认**（契合"大任务范围可控"纪律）。
   - (b) 全量逐组件像素级核验 —— 风险失控，不采纳。

## 9. Verdict（裁决）

**READY**（带默认假设）。

理由：核心需求无歧义（暗色切换 + 持久化 + 跟随系统），三个 open question 均有合理默认（Q1=auto / Q2 实现细节交 SA / Q3 核心优先交 SA 收敛），不存在无法靠默认解决的真实矛盾。INPUT.md 已明确核心必交付项与范围收敛权归属。PM 可推进 Stage 2（Solution Architect），由 SA 在 02 设计里行使范围收敛权并确认 token 化覆盖边界。
