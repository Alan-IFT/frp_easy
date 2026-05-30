# Development Record — Frontend partition · T-066 dark-theme-support

## Partition
dev-frontend — owns: `web/**`（+ 任务授权的 `scripts/baseline.json` / `docs/dev-map.md` 元数据 bump）。
全部改动在 owned paths 内，零越界后端/DB（符合 dev-frontend 契约）。

## 实现概要

按 02 §11 内部顺序：useTheme.ts → App.vue → AppLayout → token 化 6 组件 → 测试 3 份 → baseline → dev-map。落实 GR 全部条件 C-1~C-5。

### 1. 新建主题状态层 `web/src/composables/useTheme.ts`
- 模块级单例：模块作用域 `pref` ref（初值 `readPref(localStorage)`）+ `activeTheme` computed + `isDark` + `setPref` write-through 持久化。
- `THEME_STORAGE_KEY='frpEasy.themePref'`，`DEFAULT_PREF='auto'`（AC-1/BC-3）。
- `activeTheme`：`dark→darkTheme` / `light→null` / `auto→osThemeRef?.value==='dark'?darkTheme:null`（BC-5 null OS→浅）。
- 内置 `createSafeStorage`（探针 `__themePref_probe__`）复刻 useLogPrefs BC-13 内存降级（R-4：不抽公共 util，受控副本，避免反向耦合 log/theme 两域 + 触动 useLogPrefs 既有测试，T-061 教训）。
- `osThemeRef` 惰性：`useTheme()` 首次调用时 `useOsTheme()`（须 setup 内，App.vue 保证）；dark/light/setPref 分支不读 OS，`osThemeRef?.value` null 安全（R-2）。
- `setPref` 自带 `isThemePref` 守卫，非法值不改偏好（防御）。

### 2. `web/src/App.vue`
- `<n-config-provider :theme="activeTheme">`（绑 useTheme），内层加 `<n-global-style />`（放 config-provider 子树内，dev 预答 Q4），保留 NMessageProvider/router-view。
- setup 调 `const { activeTheme } = useTheme()` —— useOsTheme 首次（合规）调用点。

### 3. `web/src/components/AppLayout.vue`
- 顶栏"退出登录"按钮**之前**加 `<n-select :value=themePref :options=themeOptions size=small width=110px aria-label="主题切换" @update:value=onThemeChange>`（C-4c：不改退出按钮文本/位置，e2e 保护）。
- `themeOptions`=[跟随系统/auto, 浅色/light, 深色/dark]；`onThemeChange` 把 NSelect 联合类型收口到 ThemePref 再 setPref。
- 品牌绿 `#18a058`→`themeVars.primaryColor`（引入 `useThemeVars` + `NSelect` + `SelectOption` type + `useTheme`）。

### 4. Token 化（§2.6 核心集合，C-3）
- `Login.vue` / `Setup.vue`：删整页 inline `background:#f5f5f5`（靠 n-global-style 给 body 上浅色 themeVars 背景，浅色观感等价 C-4b；暗色自适应）。
- `Wizard.vue`：删整页 `background:#f0f2f5`；品牌 `#18a058`→`themeVars.primaryColor`；三处角色描述 `#888`→`themeVars.textColor3`；step3 图标 `#f0a020/#18a058`→`themeVars.warningColor/primaryColor`（引入 useThemeVars）。
- `FirewallHint.vue`：命令块 `background:#f5f5f5`→`themeVars.codeColor`（引入 useThemeVars）。
- `PublicIpDetector.vue`：advisory 文字 `#888`→`themeVars.textColor3`（引入 useThemeVars）。
- `ServiceStatusCard.vue`（C-3）：scoped css `.svc-status-card--warn` 的 `#f0a020`（border-color + inset box-shadow）→ `:style="warnCardStyle"` computed 走 `themeVars.value.warningColor`（needsFix 时给 border+shadow，否则空对象 = 默认边框等价原 --ok）。删 `--warn/--ok` scoped 块。复刻同文件 cmdBlockStyle:139-145 范式。
- **不动 LogViewer 子系统**（OOS-2）：`LogViewer.vue` + `log/*` 的 `var(--log-*)` ← useThemeVars 已就绪，自动跟随，零改动。

### 5. 测试（+19；C-1 单例复位）
- `web/src/composables/__tests__/useTheme.spec.ts`（新，12 用例）：AC-1/2/3/5（默认 auto+OS浅→浅不回归 / dark→darkTheme+持久化 / light 不受 OS 暗影响 BC-6 / 预置 dark 重载读回）+ AC-4/BC-4/BC-5（auto+OS暗→dark / auto+OS浅→null / OS 运行时切换响应式跟随 / useOsTheme null→浅不崩）+ BC-1/2/3+防御（非法值降级 / 缺失默认 / setItem 抛错内存降级不崩 / setPref 拒绝非法值）。
- `web/src/__tests__/App.spec.ts`（新，4 用例）：含 NGlobalStyle / 默认 auto+OS浅→NConfigProvider theme null 不回归 / 切 dark→theme=darkTheme 响应式 / auto+OS暗→darkTheme。
- `web/src/components/__tests__/AppLayout.spec.ts`（编辑，+3）：aria-label='主题切换' 控件存在 / setPref(dark)→pref+localStorage 持久化 / 不改退出登录按钮（AC-13）。
- **测试策略**：useOsTheme 统一 `vi.mock('naive-ui')` 受控 `osThemeRef`（使 useTheme 非 setup 也安全 + mock OS dark/light/null）；useTheme/App.spec 用 `vi.resetModules()`+动态 import 拿全新模块单例规避 pref 跨用例泄漏；AppLayout.spec `beforeEach` `localStorage.clear()`+`setPref('auto')`+`osThemeRef='light'` 复位单例（C-1）。darkTheme 取 `importActual` 真实对象引用相等断言。断言全用可观察量（localStorage 值/pref/activeTheme===darkTheme/isDark/DOM aria-label），少用 naive-ui 组件名查询（insight L45）；App.spec 对 NConfigProvider/NGlobalStyle 结构断言是 AC-8 本质不可避免（dev 预答 Q5）。

### 6. baseline / dev-map（C-5）
- `scripts/baseline.json`：frontend_tests 552→571 / test_count 894→913 / version 32→33 / go_tests 不变 342。notes 追加 T-066 全文。
- `docs/dev-map.md`：App.vue 主题接线备注 + composables 段 useTheme.ts 行 + 可复用工具表 +1 行（全局暗色主题状态）。

## Files changed (this partition only)
- `web/src/composables/useTheme.ts` — **新建**：主题状态层（模块单例）
- `web/src/App.vue` — :theme 绑定 + NGlobalStyle
- `web/src/components/AppLayout.vue` — n-select 切换控件 + 品牌色 token
- `web/src/pages/Login.vue` — 删整页背景 hex
- `web/src/pages/Setup.vue` — 删整页背景 hex
- `web/src/pages/Wizard.vue` — 背景/品牌/文字/图标色 token
- `web/src/components/FirewallHint.vue` — 命令块背景 token
- `web/src/components/PublicIpDetector.vue` — advisory 文字 token
- `web/src/components/ServiceStatusCard.vue` — warn 边框 token（:style computed，删 scoped 块）
- `web/src/composables/__tests__/useTheme.spec.ts` — **新建**（12）
- `web/src/__tests__/App.spec.ts` — **新建**（4）
- `web/src/components/__tests__/AppLayout.spec.ts` — 编辑（+3）
- `scripts/baseline.json` — bump
- `docs/dev-map.md` — 同步

## Out-of-partition coordination
无。纯前端单分区，未触后端/DB/migration/Go/e2e spec。无 BLOCKED ON PARTITION。

## DESIGN DRIFT
无。完全按 02 设计实现（含 n-select 三态、模块单例 composable、NGlobalStyle 放 config-provider 内、ServiceStatusCard :style computed token 化）。

## 静态自检（dev 在 role-collapsed 上下文的确定性论证）
- TS 类型：`onThemeChange(v: string|number|Array<...>|null)` 收口 ThemePref 后调 setPref（自带守卫）；`SelectOption`/`MenuOption` 从 naive-ui type import；`useTheme` 导出类型 UseThemeReturn 对齐消费点；`warnCardStyle` 用 `needsFix.value`（ComputedRef<boolean>，已读 useServiceStatus.ts:32 确认）。
- 无 imported-not-used：AppLayout 新增 NSelect/SelectOption/useThemeVars/useTheme 均被使用（themeVars 模板品牌色、themePref 模板 :value、setThemePref/onThemeChange）；Wizard/FirewallHint/PublicIpDetector 的 useThemeVars→themeVars 模板用；App.vue NGlobalStyle 模板用、useTheme→activeTheme 模板用。
- 无 imported-but-removed-usage：删除的 hex 字面量不被任何 JS 引用。ServiceStatusCard 删 scoped --warn/--ok 类后 `:class` 已改纯 'svc-status-card'，无悬空类引用。
- 测试可挂载：mock 范式 1:1 复刻既有 LogViewer.spec（importOriginal + 6 方法 useMessage stub）+ AppLayout.spec（setActivePinia + vue-router mock + router-view stub）。useTheme.spec/App.spec 的 vi.resetModules+动态 import 是 vitest 标准重置模块单例手法。
- 零回归论证：LogViewer.spec 的 darkTheme mountInside 不变（LogViewer 子系统零改动）；ServiceStatusCard 既有 spec（若有）只断言可见文案/needsFix 行为，warn 高亮由 class 改 :style 对"是否高亮"可观察语义不变（needsFix 时仍有 border+shadow），cmdBlockStyle 不动。

## verify_all result
**PENDING**（PM/dev role-collapsed 上下文无 Bash/PS，insight L31）。执行规格（交 batch orchestrator Bash 会话真跑作硬闸门）：
- 预期 **PASS**。
- `frontend_tests == 571`（552 + 19）。
- `go_tests == 342`（不变）。
- `test_count == 913`（894 + 19）。
- 特别复核：LogViewer.spec / ServiceStatusCard 既有 spec 零回归；新建 useTheme.spec/App.spec 可挂载；AppLayout.spec +3 通过。
- e2e 不受影响（03-dashboard TC-04/TC-05 断言文本/退出按钮 name 不变；n-select 放退出按钮前不遮挡）。

## Verdict
**READY FOR REVIEW**（frontend partition complete）
