# 02 方案设计 — T-066 · dark-theme-support

- 角色：Solution Architect（stage 2）。输入 `01_REQUIREMENT_ANALYSIS.md`（verdict=READY，采纳默认偏好=auto）。
- 模式：full。分区：全部前端 → dev-frontend 单分区。
- 已读代码：App.vue / AppLayout.vue / LogViewer.vue / ServiceStatusCard.vue / Login.vue / Setup.vue / Wizard.vue（前 45 行）/ FirewallHint.vue / PublicIpDetector.vue / main.ts / useLogPrefs.ts / test-utils/exposed.ts / e2e 03-dashboard / LogViewer.spec darkTheme 范式 / package.json（naive-ui ^2.38.0）。

## 1. Architecture summary（架构总览）

新增一个**模块级单例主题状态层** `web/src/composables/useTheme.ts`：持有 `'light'|'dark'|'auto'` 偏好（localStorage 持久化 + 内存降级，复用 `useLogPrefs.ts` 的 `createSafeStorage` 范式），并暴露一个 `computed` 的"当前生效主题对象"（`null`=浅色 / `darkTheme`=暗色）。`auto` 模式下生效主题由 Naive UI `useOsTheme()` 派生。`App.vue` 在其 setup 调 `useTheme()`，把生效主题绑到 `<n-config-provider :theme>`，并新增 `<n-global-style />` 让 `<body>` 背景/前景随主题切换。`AppLayout.vue` 顶栏新增三态切换控件（n-select 或 segmented），调同一 `useTheme()` 实例切换偏好。散落硬编码 hex（整页背景/文字灰/品牌绿/状态橙）改为靠 `<n-global-style>`（页面背景）或 `useThemeVars()` 语义 token（文字/品牌/状态色）。LogViewer 日志子系统已用 `useThemeVars()`，零改动自动跟随。纯前端、零新依赖、零后端/store/路由改动。

## 2. Affected modules（受影响模块，文件路径）

| 文件 | 改动 | 说明 |
|---|---|---|
| `web/src/composables/useTheme.ts` | **新建** | 主题状态层（单例 composable） |
| `web/src/App.vue` | 编辑 | `:theme` 绑定 + `<n-global-style />` |
| `web/src/components/AppLayout.vue` | 编辑 | 顶栏三态切换控件 + 品牌绿 `#18a058`→token |
| `web/src/pages/Login.vue` | 编辑 | 移除整页 `background:#f5f5f5` inline |
| `web/src/pages/Setup.vue` | 编辑 | 移除整页 `background:#f5f5f5` inline |
| `web/src/pages/Wizard.vue` | 编辑 | 整页 `#f0f2f5`→透明、品牌绿/文字灰/图标色→token |
| `web/src/components/FirewallHint.vue` | 编辑 | 命令块 `background:#f5f5f5`→themeVars |
| `web/src/components/PublicIpDetector.vue` | 编辑 | advisory 文字 `#888`→themeVars/depth |
| `web/src/components/ServiceStatusCard.vue` | 编辑 | scoped css `#f0a020`→CSS 变量（warningColor） |
| `web/src/composables/__tests__/useTheme.spec.ts` | **新建** | 状态层单测（核心 AC 落点） |
| `web/src/components/__tests__/AppLayout.spec.ts` | 编辑 | 补切换控件 a11y + 切换行为用例 |
| `web/src/App.vue` 测试（`web/src/__tests__/App.spec.ts`） | **新建** | App :theme 绑定 + NGlobalStyle 结构 |
| `scripts/baseline.json` | 编辑 | bump frontend_tests/test_count/version |
| `docs/dev-map.md` | 编辑 | App.vue 主题 + useTheme + 可复用工具表 |

## 3. Module decomposition（新模块）

### 3.1 `web/src/composables/useTheme.ts` — 主题状态层（单例 composable）

**职责**：单一真相源，持有主题偏好 + 派生生效主题 + 持久化 + OS 跟随。

**为何 composable 单例而非 Pinia store**：
- 偏好是纯 UI 局部状态，无需 Pinia devtools/SSR/跨页 hydration。
- 模块级单例 ref（在模块作用域 `const pref = ref(...)`，`useTheme()` 返回它）让 App.vue 与 AppLayout 共享同一状态，无需 provide/inject。这正是 `useLogPrefs` 的纯 composable 范式（虽 useLogPrefs 每次 new 一份，本层刻意做成模块单例因需跨组件共享）。
- `useThemeVars`/`useOsTheme` 是组合式 hook，store 内不便调用；composable 在组件 setup 内调用天然合规。

**公共 API（伪代码签名，dev 实现）**：
```ts
export type ThemePref = 'light' | 'dark' | 'auto'
export const THEME_STORAGE_KEY = 'frpEasy.themePref'   // 单一已知 key
export const DEFAULT_PREF: ThemePref = 'auto'           // AC-1 / BC-3 默认

export interface UseThemeReturn {
  pref: Ref<ThemePref>                       // 当前偏好（响应式）
  activeTheme: ComputedRef<GlobalTheme | null> // 生效主题：null=浅 / darkTheme=暗
  isDark: ComputedRef<boolean>               // 便于控件/测试读
  setPref: (p: ThemePref) => void            // 切换 + 持久化
}

export function useTheme(): UseThemeReturn
```

**内部结构**：
- 模块作用域单例：
  ```ts
  const storage = createSafeStorage()           // 复刻 useLogPrefs BC-13（本文件内私有副本，见 Reuse 决策）
  const pref = ref<ThemePref>(readPref(storage)) // 初始读 localStorage，非法/缺失→DEFAULT_PREF
  let osThemeRef: Ref<OsTheme> | null = null     // 惰性，首次在 setup 内 useOsTheme()
  ```
- `useTheme()` 体：首次调用时（必在某组件 setup，由 App.vue 保证）`if (!osThemeRef) osThemeRef = useOsTheme()`；之后复用。返回 `{ pref, activeTheme, isDark, setPref }`，其中：
  ```ts
  const activeTheme = computed<GlobalTheme | null>(() => {
    if (pref.value === 'dark') return darkTheme
    if (pref.value === 'light') return null
    // 'auto'：跟随 OS；useOsTheme 返回 'dark'|'light'|null
    return osThemeRef?.value === 'dark' ? darkTheme : null   // null OS → 浅色 BC-5
  })
  const isDark = computed(() => activeTheme.value === darkTheme)
  ```
- `setPref(p)`：`pref.value = p; storage.set(THEME_STORAGE_KEY, p)`（write-through，复刻 useLogPrefs setter 范式）。
- `readPref(storage)`：`const raw = storage.get(KEY); return (raw==='light'||raw==='dark'||raw==='auto') ? raw : DEFAULT_PREF`（BC-2 非法值降级 / BC-3 缺失降级）。

**关于 `useOsTheme()` 必须在 setup 调用的约束**：`useOsTheme()` 内部用 `onMounted`/`onBeforeUnmount`（注册 matchMedia listener）+ `inject`，故必须在某组件 setup 同步调用。设计令 **App.vue 在 setup 顶层调 `useTheme()`** → 触发首次 `useOsTheme()`。AppLayout 后续调 `useTheme()` 时 osThemeRef 已建立，复用。**约束**：`useTheme()` 首次调用方必须是组件 setup（App.vue 满足）。测试侧通过 mock `useOsTheme` 或在 setup 内调用规避。`activeTheme`/`setPref`/`pref` 不依赖 osThemeRef 存在（dark/light 分支不读 OS），仅 auto 分支读 `osThemeRef?.value`（null 安全）→ 即便测试未经 setup 调用 setPref('dark') 也安全。

### 3.2 AppLayout 顶栏切换控件

形态决策（行使 SA 收敛权，回应 01 Q2）：**采用 `n-select` 三态下拉**（light/dark/auto），理由：(1) 顶栏横向空间紧张（已有版本号/缺失横幅/用户名/退出），下拉占位最小；(2) 三态语义清晰；(3) `n-select` 可挂 `aria-label`。放在"退出登录"按钮**之前**（用户名右侧），不改"退出登录"按钮文本/位置（AC-13 e2e 保护）。
- 选项：`[{label:'跟随系统',value:'auto'},{label:'浅色',value:'light'},{label:'深色',value:'dark'}]`。
- 绑 `:value="themePref"` `@update:value="setThemePref"`，size=small，宽度约 110px，`aria-label="主题切换"`。
- AppLayout setup 调 `const { pref: themePref, setPref: setThemePref } = useTheme()`。

## 4. Data model changes

无。纯前端，无 schema/migration/API。localStorage key `frpEasy.themePref`（客户端本地，非服务端数据）。

## 5. API contracts

无新增/修改 REST API。

## 6. Sequence / flow（流程）

```
首次加载：
  main.ts createApp(App) → App.vue setup:
    useTheme() 首次调用 → readPref(localStorage)
       缺失/非法 → 'auto'
    useOsTheme() 注册 matchMedia listener（onMounted）
    activeTheme = computed(auto → OS dark? darkTheme : null)
  <n-config-provider :theme="activeTheme"> 渲染整个 router-view
  <n-global-style /> 给 body 上 themeVars 背景

用户在 AppLayout 顶栏切到"深色"：
  n-select @update:value('dark') → setThemePref('dark')
    → pref.value='dark' → storage.set(KEY,'dark')
    → activeTheme 重算 = darkTheme（响应式）
    → App.vue 的 n-config-provider :theme 变 darkTheme
    → 全站 + n-global-style 切暗（含 LogViewer var(--log-*) 自动跟随）

OS 主题变化（pref='auto'）：
  matchMedia 触发 → useOsTheme 的 ref 变 'dark'
    → activeTheme 重算 → 全站切暗，无需重载/操作（BC-4）

页面重载：
  App.vue setup → readPref → 'dark'（上次持久化）→ 立即生效暗色（AC-5）
```

## 7. Reuse audit（复用审计，强制）

| 需求 | 已有 | 文件路径 | 决策 |
|---|---|---|---|
| localStorage 持久化 + 内存降级 | `createSafeStorage`（私有于 useLogPrefs） | `web/src/composables/log/useLogPrefs.ts:41-84` | **复刻范式**（非直接 import：该函数当前未导出且耦合 log 前缀探针字符串 `__logPrefs_probe__`）。useTheme.ts 内置一份等价 `createSafeStorage`，探针字符串改 `__themePref_probe__`。决策理由见 Risk R-4。 |
| 暗色主题对象 | `darkTheme` | naive-ui 内置 | import 复用，零依赖 |
| OS 主题侦测 | `useOsTheme` | naive-ui 内置 | import 复用，零依赖 |
| 语义色 token | `useThemeVars()` | naive-ui 内置（LogViewer/ServiceStatusCard 已用） | 复用：`primaryColor`/`warningColor`/`textColor3`/`bodyColor` |
| body 背景随主题 | `NGlobalStyle` | naive-ui 内置 | 新挂 App.vue（当前全站无） |
| 测试 darkTheme mount | `mountInside(kind,'dark')` 范式 | `web/src/components/__tests__/LogViewer.spec.ts:37-46` | 测试参考：`NConfigProvider {theme: darkTheme}` |
| 读 defineExpose | `getExposed` | `web/src/test-utils/exposed.ts` | 复用（AppLayout/App spec） |
| 日志子系统主题感知 | `var(--log-*)` ← `useThemeVars` | `LogViewer.vue:124-135` + `log/*` | **不动**，自动跟随（OOS-2） |

## 8. Risk analysis（风险，每条带缓解）

- **R-1 浅色回归**：若 token 化改错（如把浅色背景误设暗）或默认偏好误为 dark → 浅色场景回归。**缓解**：默认 `auto`（OS 浅→浅）；AC-1/AC-3 单测锁默认浅 + light 恒浅；token 用语义值（bodyColor 浅色时本就是浅）；移除整页背景靠 NGlobalStyle 等价当前浅色观感。QA 反向证伪"默认浅色不回归"。
- **R-2 useOsTheme 在非 setup 调用报错**：composable 单例若被测试在 setup 外调用触发 `useOsTheme` 的 inject/onMounted 报错。**缓解**：osThemeRef 惰性化 + null 安全（dark/light/setPref 分支不读 OS）；测试 auto 跟随用例统一在组件 setup 内 mount（参考 LogViewer.spec），或 `vi.mock('naive-ui')` 提供受控 `useOsTheme`。设计 §3.1 已写明约束。
- **R-3 模块单例跨测试状态泄漏**：useTheme 模块级 `pref` ref 在多个测试间复用同一模块实例 → 一个测试 setPref('dark') 污染下一个。**缓解**：(a) useTheme.spec 每个用例 `beforeEach` 清 localStorage + setPref(DEFAULT) 复位，或 (b) 提供仅测试用的内部 reset（不推荐扩 API）。dev 优先 (a)：每用例显式 `setPref('auto')` + `localStorage.clear()`。**dev 必须处理此项**，否则测试顺序敏感（insight 风格：状态泄漏=脆弱测试）。
- **R-4 不直接复用 useLogPrefs.createSafeStorage**：该函数未导出且探针字符串/STORAGE_KEYS 耦合 log 域。**缓解**：useTheme 内置等价副本（~30 行）。这是受控重复（两处各自独立的本地降级，无共享状态），不触 insight L37/L42 关于"逐字重复跨组件"的抽取诉求（那是 UI 行为重复；此处是基础设施小工具，跨域抽取会反向耦合 log/theme）。**SA 判定：不为本任务抽 utils/safeStorage.ts**（避免改 useLogPrefs.ts 扩散 + 动其测试，参考 T-061 教训：抽取前必读既有测试断言什么；此处 useLogPrefs 测试断言其内存降级行为，抽取会迫使改 log 测试），记 backlog 候选（未来若第 3 处出现再抽）。
- **R-5 ServiceStatusCard scoped css 的 #f0a020 无法直接读 themeVars**：scoped `<style>` 块是静态 CSS 不能内联 themeVars.value。**缓解**：改用 CSS 变量——在组件 setup 把 `themeVars.value.warningColor` 投到一个 root inline CSS 变量（如 `--svc-warn`），scoped css 写 `var(--svc-warn)`（复刻 LogViewer rootCssVars 范式 LogViewer.vue:124）。或更简：把 warn 边框样式从 scoped css 移到 `:style` computed 绑定。dev 选其一，倾向后者（边框 box-shadow 走 `:style` computed，删 scoped 块）。
- **R-6 e2e 03-dashboard 受顶栏新控件影响**：新增 n-select 可能改变 dashboard 可见元素。**缓解**：已 grep e2e（insight L34）——TC-04 仅断言 `仪表盘`/`frpc（客户端）`/`frps（服务端）` 文本可见，TC-05 按 name `退出登录` 点击。新 n-select 不改这些文本、放退出按钮之前不遮挡 → e2e 零影响。dev 不改"退出登录"按钮。
- **R-7 范围过大失控**（本批次最大任务）：9+ 文件改动。**缓解**：范围已收敛——核心 = useTheme + App.vue + AppLayout 控件 + §2.6 核心 hex 集合（有限且已逐文件定位）。边缘像素微调列 OOS-7。GR 须确认可一次交付。

## 9. Migration / rollout plan

- 无数据迁移。无 feature flag（主题默认 auto 平滑生效；OS 浅色环境观感不变 = 后向兼容）。
- 回滚：还原 App.vue + 删 useTheme.ts + 还原 9 个组件的 hex 即回到纯浅色，无持久数据残留（localStorage key 残留无害，下次缺失/非法降级路径吞掉）。

## 10. Out-of-scope clarifications（设计边界）

- OOS-1：顶级路由页（/login /setup /wizard）无页内切换入口，但跟随 App.vue 全局 NConfigProvider 主题（insight L30/L31）。这三页**仍要 token 化**（§2.6 含 Login/Setup/Wizard）使其暗色可读，只是无切换控件。**交付文档显式记录此边界。**
- OOS-2：不动 LogViewer 子系统（`LogViewer.vue` + `log/*` + `LogToolbar.vue:204` 的 `var(--log-error,#d03050)` fallback）。
- OOS-3：不抽 `utils/safeStorage.ts`（R-4），useTheme 内置等价副本。
- OOS-4：不做主题色自定义/调色板/过渡动画。
- OOS-5：边缘组件像素级色彩微调若仅轻微瑕疵（非不可读）记同任务内可接受局限，不展开逐组件像素核验（回应 01 Q3 默认 (a)）。但 §2.6 核心集合必做。
- OOS-6：不改后端/store/路由守卫/API/DB/Go/e2e spec。

## 11. Partition assignment（分区指派，REQUIRED）

| File | Partition | New / Edit | Dependency |
|---|---|---|---|
| `web/src/composables/useTheme.ts` | dev-frontend | new | — |
| `web/src/App.vue` | dev-frontend | edit | depends on useTheme |
| `web/src/components/AppLayout.vue` | dev-frontend | edit | depends on useTheme |
| `web/src/pages/Login.vue` | dev-frontend | edit | — |
| `web/src/pages/Setup.vue` | dev-frontend | edit | — |
| `web/src/pages/Wizard.vue` | dev-frontend | edit | — |
| `web/src/components/FirewallHint.vue` | dev-frontend | edit | — |
| `web/src/components/PublicIpDetector.vue` | dev-frontend | edit | — |
| `web/src/components/ServiceStatusCard.vue` | dev-frontend | edit | — |
| `web/src/composables/__tests__/useTheme.spec.ts` | dev-frontend | new | depends on useTheme |
| `web/src/__tests__/App.spec.ts` | dev-frontend | new | depends on App.vue |
| `web/src/components/__tests__/AppLayout.spec.ts` | dev-frontend | edit | depends on AppLayout |
| `scripts/baseline.json` | dev-frontend | edit | — |
| `docs/dev-map.md` | dev-frontend | edit | — |

### Dispatch order
1. dev-frontend（单分区，全部）

### Parallelism
None — 单分区，内部按 useTheme.ts → App.vue/AppLayout → 其余 token 化 → 测试 顺序实现。

## 12. Verdict

**READY**。设计完整可交付，无需求缺口。范围已收敛（核心明确 + 边缘列 OOS-5），9 个生产文件改动均逐一定位且改法明确（junior dev 可照做）。零新依赖、零后端改动。提请 GR 重点确认：(a) 范围可一次交付（R-7）；(b) 默认偏好 auto 不致浅色回归（R-1/AC-1）；(c) ServiceStatusCard scoped css token 化方案（R-5）合理；(d) e2e 零影响（R-6/AC-13）。
