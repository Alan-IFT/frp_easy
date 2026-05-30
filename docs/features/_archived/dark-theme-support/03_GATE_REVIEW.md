# 03 闸门评审 — T-066 · dark-theme-support

- 角色：Gate Reviewer（stage 3）。模式：full。
- 输入：`01_REQUIREMENT_ANALYSIS.md`（READY）、`02_SOLUTION_DESIGN.md`（READY）。
- 独立核验（不信上游，亲读代码）：
  - ✅ `useLogPrefs.ts:41-84` 确有 `createSafeStorage`，且**未 export**（仅模块内用）→ SA 复用决策（复刻而非 import）依据成立。探针字符串 `__logPrefs_probe__` 耦合 log 域，证实 R-4 不直接复用的理由。
  - ✅ `LogViewer.spec.ts:37-46` 确有 `mountInside(kind, theme)` → `NConfigProvider {theme: theme==='dark'?darkTheme:null}`，darkTheme 测试范式存在可参考。
  - ✅ `e2e/03-dashboard.spec.ts`：TC-04 断言 `仪表盘`/`frpc（客户端）`/`frps（服务端）` 文本可见；TC-05 `getByRole('button',{name:'退出登录'})` 点击。无主题断言、无 select 断言 → 顶栏新增 n-select（不改退出按钮文本/位置）零影响成立。
  - ✅ `ServiceStatusCard.vue:139-145` 已用 `useThemeVars().codeColor` 走 `:style` computed（cmdBlockStyle）；:185-186 scoped css `#f0a020` 确为静态 css 不能内联 themeVars → R-5 改 `:style` computed 走 warningColor 方案有同文件先例（cmdBlockStyle）。
  - ✅ `App.vue` 确为裸 `<n-config-provider><n-message-provider><router-view/>` 三层，无 :theme 无 NGlobalStyle → 接线点明确。
  - ✅ 硬编码 hex 清单逐条核对源码命中（AppLayout:5 / PublicIpDetector:27 / FirewallHint:12 / Login:2 / Setup:2 / Wizard:2,5,26,32,38,137 / ServiceStatusCard:185-186）。
  - ✅ naive-ui ^2.38.0（package.json）提供 darkTheme/useOsTheme/useThemeVars/NGlobalStyle，零新依赖成立。
  - ✅ insight 核对：L16（禁硬编码文字色，本任务正是修它）、L30/L31（顶级路由页边界，01 OOS-1/02 OOS-1 已显式记录）、L34（e2e 已 grep 核实）、L45（断言可观察量，设计 NFR-4 已承诺）、L37/L42（抽取诉求，R-4 已论证不抽合理）——无 insight 与设计假设矛盾。

## 1. Audit checklist（8 维审计）

| # | 维度 | 结果 | 理由 |
|---|---|---|---|
| 1 | 需求完整性 | **PASS** | 8 条 in-scope 均可测（localStorage 值/状态导出/useOsTheme mock/DOM 属性/静态 hex 检查），AC-1~AC-13 每条有可观察判据。默认偏好歧义已由 RA Q1 给默认（auto）且 PM 采纳。 |
| 2 | 设计完整性 | **PASS** | 设计覆盖全部 8 条 in-scope：状态层（§3.1）/持久化+降级（§3.1 createSafeStorage 副本）/auto 跟随（activeTheme computed 读 osThemeRef）/App.vue 接线（§2+§6）/顶栏控件（§3.2）/§2.6 hex token 化逐文件/逐页可读（§2 表 + OOS-5 边界）。 |
| 3 | 复用正确性 | **PASS** | Reuse audit 8 行均经独立核验（见上）。createSafeStorage 未导出→复刻合理；darkTheme/useOsTheme/useThemeVars/NGlobalStyle import 复用；getExposed/LogViewer.spec 范式复用。未漏既有可复用代码。 |
| 4 | 风险覆盖 | **PASS** | 7 条风险覆盖真实风险（浅色回归/useOsTheme setup 约束/模块单例测试泄漏/不抽 util/scoped css token/e2e/范围）。R-3 模块单例跨测试泄漏是本设计最易踩坑点，已点名 dev 处理（见 C-1）。无明显遗漏风险。 |
| 5 | 迁移安全 | **PASS** | 无数据迁移/无 API 变更。回滚路径明确（删 useTheme + 还原 hex）。localStorage 残留无害（降级路径吞非法值）。 |
| 6 | 边界处理 | **PASS** | BC-1 内存降级（复刻 useLogPrefs）/BC-2 非法值降级 DEFAULT/BC-3 缺失默认 auto/BC-4 OS 运行时切换响应式/BC-5 useOsTheme null→浅/BC-6 light·dark 不受 OS 影响——6 条边界设计齐全，activeTheme computed 分支逐条对应。 |
| 7 | 测试可行性 | **PASS** | 每条 AC 可测：状态层 AC-1~7 直接构造 useTheme + mock useOsTheme + localStorage 操作；AC-8 App :theme 绑定经 mount 后查 NConfigProvider theme prop（或经 getExposed/computed）；AC-9 AppLayout select aria-label + 切换 DOM 查询；AC-10 静态源码 grep；AC-11 既有 spec 跑过；AC-13 e2e。darkTheme mount 有 LogViewer.spec 范式。 |
| 8 | 范围清晰 | **PASS** | OOS 1-6 显式（顶级路由页边界/不动 LogViewer/不抽 util/无调色板·过渡/边缘微调可接受/不改后端）。dev 不会过度构建。范围虽大但有限且逐文件定位（见 C-2）。 |

## 2. Findings（WARN/FAIL 明细）

无 FAIL。无 WARN。8 维全 PASS。

以下为 **APPROVED WITH CONDITIONS** 的开发期条件（非缺陷，是必须遵守的实现约束，dev 须落实，CR/QA 须复核）：

- **C-1（落实 R-3，强条件）**：useTheme 是**模块级单例**，`pref` ref 跨测试用例共享同一模块实例。dev 在 `useTheme.spec.ts` 必须每个用例 `beforeEach` 复位（`localStorage.clear()` + `setPref('auto')` 或等价），否则测试顺序敏感会偶发红。**不得**为复位扩展生产 API（不加 reset 导出）；用既有 setPref + localStorage 操作复位即可。
- **C-2（落实 R-7，范围闸门）**：本任务为本批次最大任务（9 生产文件 + 3~4 测试文件）。GR 判定**范围可控、可一次交付**——因核心明确（useTheme + App.vue + AppLayout 控件）且 hex 集合有限（已逐文件逐行定位，§2.6 共 9 处命中点）。dev **不得**扩散到 §2.6 以外的组件做"顺手"色彩微调（OOS-5：边缘瑕疵记可接受局限，不展开）。若实现中发现某核心文件改动意外牵连第 10+ 文件，停下报 PM 而非自行扩范围。
- **C-3（落实 R-5）**：ServiceStatusCard scoped css 的 `#f0a020` → `:style` computed 绑定走 `themeVars.value.warningColor`（复刻同文件 cmdBlockStyle:139-145 范式），删对应 scoped 块或改 CSS 变量。两方案 dev 任选，但 warn 视觉语义须保留（边框+inset box-shadow 仍提示 needsFix）。
- **C-4（落实 R-1/AC-13）**：(a) 默认偏好恒 `auto`，DEFAULT_PREF 不得误设 dark；(b) 移除 Login/Setup/Wizard 整页 inline background 后，浅色观感须等价当前（靠 NGlobalStyle 给 body 上 themeVars 浅色背景）；(c) AppLayout 顶栏 n-select 放"退出登录"按钮**之前**，不改其文本/role，e2e 03-dashboard 零影响。
- **C-5（baseline）**：新增测试同步 bump `scripts/baseline.json` 的 `frontend_tests`/`test_count`/`version`。

## 3. High-probability developer questions（预测 dev 提问，预答）

1. **Q：useTheme 做成模块单例 ref 还是 `useTheme()` 内每次 new？**
   A：模块级单例（模块作用域 `const pref = ref(...)`，`useTheme()` 返回它）。理由 02 §3.1：App.vue 与 AppLayout 须共享同一偏好。注意 C-1 测试复位。
2. **Q：useOsTheme() 在 useTheme.spec 单测里直接调会报错吗？**
   A：会（它用 onMounted/inject 需 setup 上下文）。故 (a) 测 auto 跟随用例统一在组件 setup 内 mount（或 `vi.mock('naive-ui')` 提供受控 useOsTheme ref），(b) 测 light/dark/setPref/persist 用例不触发 osThemeRef（这些分支不读 OS，osThemeRef 惰性保持 null 安全）。02 §3.1 已写明惰性 + null 安全。
3. **Q：activeTheme 在 auto + useOsTheme 返回 null（环境不支持 matchMedia）时？**
   A：BC-5 → 视为浅色（返回 null 主题）。`osThemeRef?.value === 'dark' ? darkTheme : null` 天然满足。
4. **Q：App.vue 加 NGlobalStyle 放哪一层？**
   A：放 `<n-config-provider>` 内（NGlobalStyle 需在 config-provider 子树才能读到主题 themeVars）。即 `<n-config-provider :theme><n-global-style/><n-message-provider><router-view/></n-message-provider></n-config-provider>`。
5. **Q：测试断言 App.vue 的 :theme 绑定怎么查（少用组件名查询，insight L45）？**
   A：优先经 useTheme 状态导出 + 切换后断言 isDark/activeTheme（可观察量）；App :theme 结构性断言可 mount App + 经 getExposed 暴露 activeTheme 或查 NConfigProvider 的 theme prop（结构性断言此处可接受少量组件查询，但状态层逻辑断言走 useTheme 直测为主）。
6. **Q：Wizard.vue:137 图标色 `binWarning>0?'#f0a020':'#18a058'` 怎么 token 化？**
   A：改 `:color="binWarning.length>0 ? themeVars.warningColor : themeVars.primaryColor"`（Wizard setup 引入 useThemeVars）。

## 4. Verdict

**APPROVED WITH CONDITIONS**

设计与需求 8 维全 PASS，无 FAIL/无 WARN。范围虽为本批次最大但 GR 独立核验判定可控、可一次交付（C-2）。条件 C-1~C-5 为开发期必须遵守的实现约束（非缺陷回退）。开发可推进至 Stage 4（dev-frontend）。
