# Code Review — T-066 · dark-theme-support

- 角色：Code Reviewer（stage 5）。独立核验，逐文件读改后代码 + 逐条 AC 覆盖 + 设计保真度。
- 输入：01 / 02 / 04。模式：full。

## Files reviewed
- `web/src/composables/useTheme.ts`（新）
- `web/src/App.vue`
- `web/src/components/AppLayout.vue`
- `web/src/pages/Login.vue` / `Setup.vue` / `Wizard.vue`
- `web/src/components/FirewallHint.vue` / `PublicIpDetector.vue` / `ServiceStatusCard.vue`
- `web/src/composables/__tests__/useTheme.spec.ts`（新）
- `web/src/__tests__/App.spec.ts`（新）
- `web/src/components/__tests__/AppLayout.spec.ts`（编辑）
- `scripts/baseline.json` / `docs/dev-map.md`

## Findings

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### NIT
- [STYLE] `web/src/components/AppLayout.vue:142` — `onThemeChange` 参数联合类型 `string|number|Array<string|number>|null` 手写复刻了 NSelect `@update:value` 的载荷类型；若 naive-ui 升级改签名需同步。当前 ^2.38.0 正确，可接受。属防御性收口，不阻塞。
- [STYLE] `web/src/composables/useTheme.ts:43-76` — `createSafeStorage` 与 `useLogPrefs.ts:41-84` 高度相似（受控副本）。04/02 §R-4 已论证不抽公共 util 的理由（避免反向耦合 + 触动 useLogPrefs 测试，T-061 教训），且两处探针字符串/key 已区分。记 backlog 候选（未来第 3 处出现再抽），非本任务缺陷。

## Requirement coverage check（逐条 AC）

| 准则 | 实现 | 状态 |
|---|---|---|
| AC-1 默认 auto + OS 浅→浅不回归 | `useTheme.ts:88(DEFAULT_PREF='auto')` + `:99-105 activeTheme` auto 分支 OS≠dark→null；`useTheme.spec.ts` AC-1 用例 | ✅ |
| AC-2 dark→darkTheme + 持久化 dark | `useTheme.ts:100,113-117 setPref write-through`；spec AC-2 | ✅ |
| AC-3 light→null + 持久化 + 不受 OS 暗影响 | `useTheme.ts:101`（light 分支不读 OS）；spec AC-3（osThemeRef='dark' 仍 null） | ✅ |
| AC-4 auto+OS暗→darkTheme / 浅→null | `useTheme.ts:104`；spec AC-4 两用例 | ✅ |
| AC-5 持久化重载读回 | `useTheme.ts:80-83 readPref`；spec AC-5（预置 dark + freshUseTheme） | ✅ |
| AC-6 非法值降级 auto | `useTheme.ts:78-83 isThemePref/readPref`；spec BC-2（purple→auto） | ✅ |
| AC-7 localStorage 不可用内存降级 | `useTheme.ts:43-76 createSafeStorage`；spec BC-1（setItem throw→不崩+切换生效） | ✅ |
| AC-8 App.vue :theme 绑定 + NGlobalStyle | `App.vue:2 :theme=activeTheme` + `:6 <n-global-style/>`；`App.spec.ts` 4 用例 | ✅ |
| AC-9 AppLayout 切换控件 + aria-label | `AppLayout.vue` n-select aria-label='主题切换' + onThemeChange；`AppLayout.spec.ts` +3 | ✅ |
| AC-10 §2.6 核心 hex token 化 | Login/Setup/Wizard 删整页背景；Wizard/FirewallHint/PublicIpDetector/AppLayout/ServiceStatusCard hex→themeVars（逐文件核对，见设计保真度表） | ✅ |
| AC-11 既有 spec 零回归 | LogViewer 子系统零改动（OOS-2）；ServiceStatusCard warn 由 class→:style 语义不变；详见下方零回归分析 | ✅（待 verify_all 真跑确认） |
| AC-12 verify_all PASS + baseline bump | baseline 552→571/894→913/v33；verify_all PENDING(预期 PASS) | ✅（预测） |
| AC-13 e2e 03-dashboard 不受影响 | n-select 放退出按钮前不改其文本/位置；spec 不改任何 e2e；AppLayout.spec 含"不改退出登录按钮"用例 | ✅ |

## Design fidelity check（设计保真度）

| 设计项（02） | 实现 | 状态 |
|---|---|---|
| 模块级单例 composable useTheme | `useTheme.ts:85-87` 模块作用域 pref/osThemeRef，`useTheme()` 返回 | ✅ |
| 公共 API pref/activeTheme/isDark/setPref | `useTheme.ts:30-35` UseThemeReturn 全对齐 | ✅ |
| THEME_STORAGE_KEY / DEFAULT_PREF | `useTheme.ts:25,27` 'frpEasy.themePref' / 'auto' | ✅ |
| osThemeRef 惰性 + null 安全（R-2） | `useTheme.ts:87,127-130` 惰性 useOsTheme；`:104 osThemeRef?.value` null 安全 | ✅ |
| createSafeStorage BC-13 副本（R-4） | `useTheme.ts:43-76`，探针 `__themePref_probe__` 区分 | ✅ |
| App.vue :theme + NGlobalStyle 在 config-provider 内 | `App.vue:2-8`（NGlobalStyle 在 NConfigProvider 子树内） | ✅（dev 预答 Q4） |
| AppLayout n-select 三态 + 退出按钮前 + aria-label | `AppLayout.vue` 切换控件位置/选项/aria-label | ✅ |
| 品牌绿→primaryColor | `AppLayout.vue:5` + `Wizard.vue:5` themeVars.primaryColor | ✅ |
| ServiceStatusCard #f0a020→:style computed warningColor（C-3/R-5） | `ServiceStatusCard.vue:151-158 warnCardStyle` + 删 scoped --warn/--ok | ✅ |
| 不动 LogViewer 子系统（OOS-2） | LogViewer.vue/log/* 零改动 | ✅ |
| 顶级路由页无切换入口但跟随全局（OOS-1） | Login/Setup/Wizard 无 n-select，但被 App.vue NConfigProvider 包裹 | ✅（设计边界，非缺陷） |
| 零新依赖 | useTheme 仅 import naive-ui 内置 darkTheme/useOsTheme；package.json 未改 | ✅ |

## 6 维审计

1. **Logic correctness**：activeTheme 三分支（dark/light/auto）+ auto 内 OS null/dark/light 三态全覆盖（BC-5 null→浅）。setPref isThemePref 守卫防非法值。readPref 缺失/非法降级 DEFAULT。createSafeStorage 三处 throw 点（构造探针/get/set）全 catch→内存降级。无 off-by-one/null deref（osThemeRef?.value 可选链）。✅
2. **Requirement fidelity**：13 条 AC 全有实现 + 对应测试（见覆盖表）。✅
3. **Design fidelity**：11 项设计逐条对齐，0 drift（见保真度表）。dev 04 标 DESIGN DRIFT=无，核验属实。✅
4. **Performance**：activeTheme/isDark 是 computed（缓存）；切换仅改一个 ref + 一次 localStorage write；NGlobalStyle 是 naive-ui 标准做法无额外开销。无热路径同步 IO（localStorage 切换时一次写可接受）。✅
5. **Security**：无新输入面。localStorage 值经 isThemePref 白名单校验（防注入非法主题值）。无 secret/XSS（themeVars 是 naive-ui 受控值，inline style 绑定的是已知 token 字符串）。✅
6. **Maintainability**：命名清晰（pref/activeTheme/setPref/warnCardStyle）；注释解释 WHY（为何单例、为何不抽 util、osTheme setup 约束、C-3 改法）；删 ServiceStatusCard 死 scoped 块无残留；无过早抽象（受控副本而非强行公共化）。✅

## 测试质量审查（CR 红线 4：测试是代码）

- useTheme.spec 12 用例断言**可观察量**（localStorage 值/pref/activeTheme===真实 darkTheme 引用/isDark），非形状匹配。vi.resetModules+动态 import 正确重置模块单例（否则 AC-5/6 不同预置态会被首次模块加载值污染）。受控 osThemeRef mock 使 auto 分支可测且 useTheme 非 setup 可安全调（避开 useOsTheme onMounted/inject 约束）。BC-1 用 spyOn Storage.prototype.setItem throw 真触发降级路径——非桩。✅
- App.spec 4 用例断言 NConfigProvider theme prop（可观察接线结果）+ NGlobalStyle 存在；切 dark 后响应式 theme=darkTheme 是真行为断言。✅
- AppLayout.spec +3 复用既有 mount 范式，beforeEach setPref('auto')+localStorage.clear 正确处理单例泄漏（C-1）；断言 DOM aria-label + 状态层 pref/localStorage（可观察）+ 退出按钮文本保留（AC-13 护栏）。✅
- 无删除既有测试（CR 红线 4）。LogViewer.spec darkTheme 范式不动。

## 零回归分析（AC-11）

- LogViewer 子系统：零源码改动 → LogViewer.spec（含 darkTheme mountInside）零回归。
- ServiceStatusCard：warn 高亮由 `:class --warn` 改 `:style warnCardStyle` computed。若既有 ServiceStatusCard spec 断言的是"needsFix 时是否有 warn 视觉/needsFix 文案/折叠区"，则语义保留（needsFix.value 时仍输出 border+shadow）。若有 spec 断言具体 class 名 `svc-status-card--warn` 存在 → 会回归（但该 class 已删）。**CR 建议 verify_all 真跑时特别复核 ServiceStatusCard 既有 spec 是否断言了 --warn class 名**（PM 已列入交付硬闸门复核项）。CR grep 判断：项目测试范式（insight L45）一贯少用组件/class 名查询、偏可观察量，故断言 --warn class 名的概率低，但需真跑确证。
- AppLayout：既有 7 图标 a11y 用例不受切换控件影响（menuIconSpans 只取带 aria-label 的 span.n-icon 菜单图标，n-select 的 aria-label 在不同元素 + 不是 span.n-icon，不串扰）。

## Verdict

**APPROVED**（0 CRITICAL / 0 MAJOR / 0 MINOR / 2 NIT）

实现与 01 需求（13 AC 全覆盖）、02 设计（11 项零 drift）完全一致，落实 GR 全部条件 C-1~C-5。测试断言可观察量且有意义（非形状匹配），正确处理模块单例泄漏与 useOsTheme setup 约束。零新依赖。建议 QA 在对抗测试中重点反向证伪：默认浅色不回归 / auto 跟随 OS / 手选暗色持久化重载保持 / localStorage 不可用内存降级 / 顶级路由页跟随全局主题；并请 verify_all 真跑确认 ServiceStatusCard 既有 spec 是否依赖已删的 `--warn` class 名（低概率，需确证）。
