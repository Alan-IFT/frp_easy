# 03 闸门评审 — T-067 · responsive-layout

> Harness Stage 3 · Gate Reviewer · 模式 full · 全中文 · 独立核验，不盲信上游

## 核验方法

- 已读 01（READY）+ 02（READY）+ AI-GUIDE + insight-index（L16/L34/L37/L45 + T-066 范式）。
- 已逐条核验 02 引用的真实代码（红线 2/3）：
  - `web/src/composables/useTheme.ts`——确认模块单例 + 惰性 hook init + 单例守卫范式真实存在（02 §3 复刻依据成立）。
  - `web/src/components/AppLayout.vue:84-93,132`——确认 `collapsed=ref(false)` + `@collapse/@expand` + `show-trigger` 真实存在（FR-2 共存复用点成立）。
  - `web/src/components/__tests__/AppLayout.spec.ts`——确认既有 8 例（T-064 菜单 5 + T-066 主题 3）mount 范式（真 Pinia + vi.mock vue-router/naive-ui useOsTheme/useMessage）；**spec 未注入 matchMedia**（grep 0 命中）→ R-3 缓解成立：既有用例下 useViewport 走 happy-dom 默认 matchMedia（innerWidth 默认 → max-width:767.98 matches=false）= 展开态 = 既有行为，零回归。
  - `web/src/pages/__tests__/Server.spec.ts` / `Client.spec.ts`——grep `width|max-width|matchMedia` **0 命中** → R-4 缓解成立：既有 spec 断言 DOM 文本/按钮/getExposed/apiGet，**不**断言输入控件 width style，故 max-width 改动对既有 spec 透明（与 insight L37 "评估抽取是否破坏测试须实读断言"同源）。
  - `web/playwright.config.ts:21-22`——确认 `devices['Desktop Chrome']`（视口 1280×720）。
  - `web/tests/e2e/03-dashboard.spec.ts`——TC-04 断言 `getByText('仪表盘').first()` + `frpc（客户端）` + `frps（服务端）`（后两者是 Dashboard 页正文卡片标题，与侧栏折叠态无关恒可见；'仪表盘' `.first()` 匹配侧栏菜单或页头标题任一，鲁棒）；TC-05 `getByRole('button',{name:'退出登录'})`。**1280 ≥ 768 阈值 → e2e 默认视口侧栏保持展开，零回归**（R-1/BC-2 成立）。
  - `web/vitest.config.ts:7`——确认 `environment: 'happy-dom'`，`package.json` happy-dom ^14（R-2 测试须 vi.stubGlobal matchMedia 受控注入的依据成立）。

## 1. 审计清单（8 维）

| # | 维度 | 结论 | 一句话理由 |
|---|---|---|---|
| 1 | 需求完整性 | PASS | FR-1~FR-8 全可测、无歧义；阈值方向（<768 折叠 / >=768 展开，BC-1）与 e2e 视口边界（BC-2）钉死，无悬置 open question。 |
| 2 | 设计完整性 | PASS | 每条 in-scope 行为有对应设计：FR-1/2/3→useViewport+watch、FR-4/5→n-space wrap+版本号 v-if、FR-6/7→max-width、FR-8→padding 响应式；§6 流程图覆盖窄/宽/手动展开/跨阈值四态。 |
| 3 | 复用正确性 | PASS | Reuse audit 7 行经核验全部真实（useTheme 范式 / AppLayout collapsed-trigger / AppLayout spec mount / Settings max-width 写法 / 不用 naive-ui useBreakpoint 有充分理由）。 |
| 4 | 风险覆盖 | PASS | R-1（e2e 误触）/R-2（happy-dom matchMedia）/R-3（既有 8 例）/R-4（Server/Client spec）/R-5（顶栏 wrap 影响退出按钮）五大真实风险全列且有缓解；GR 已独立核验 R-1/R-3/R-4 缓解成立。 |
| 5 | 迁移安全 | PASS | 无 DB/API/store 迁移；rollback = 还原 4 文件 + 删 useViewport.ts，无残留状态（无新 localStorage key）。 |
| 6 | 边界处理 | PASS | BC-1（阈值边界 767.98 让 768 判展开）/BC-2（1280 e2e）/BC-3（横幅+窄屏 wrap）/BC-4（手动展开不锁死，§6 精确论证 watch 仅跨阈值触发）/BC-5（单例 initialized 守卫无重复监听）/BC-6（safeMatchMedia null 降级）/BC-7（折叠态 T-064 a11y）全设计。 |
| 7 | 测试可行性 | PASS | AC-2~AC-5 全可观察量验证（collapsed bool / 手动 expand 后状态 / 容器 style 含 max-width / 顶栏 wrap + 关键入口 DOM 存在）；AC-6 静态核实 e2e 视口；测试 mock matchMedia 路径明确（vi.stubGlobal + 受控 MediaQueryList + resetModules 拿新单例）。 |
| 8 | Out-of-scope 清晰度 | PASS | OOS-1~OOS-6 明确边界（不动 Proxies/Login/Setup 表单、不引依赖、不改菜单结构/路由、不引颜色、不做 drawer、App.vue 不改）；开发者不会过度建造。 |

**8 维全 PASS，0 WARN，0 FAIL。**

## 2. Findings（WARN/FAIL）

无 WARN，无 FAIL。设计与需求一致、可测、边界完备、零回归路径已核验。

## 3. 开发期高概率问题（预答）

- **Q1：useViewport 单例的 matchMedia 监听不在 onUnmounted 清理，会泄漏吗？**
  预答：不会。模块单例存活整个 app 生命周期（与 useTheme osThemeRef 同），`initialized` 守卫保证监听只注册一次，监听数恒为 1（NFR-4）。不要在组件 onUnmounted remove——那会让其它消费方失去响应（单例语义）。

- **Q2：AppLayout `collapsed` 初值怎么设？**
  预答：`const collapsed = ref(isNarrow.value)`（在 useViewport() 之后），让初次渲染即反映当前断点默认态。然后 `watch(isNarrow, n => collapsed.value = n)`。**不要**用 `watch(..., {immediate:true})` 之外再重复设初值导致双重赋值——初值 + watch（非 immediate）即足够（初值定基线，watch 管跨阈值）。

- **Q3：测试里 happy-dom 默认 matchMedia 会让既有 AppLayout 8 例折叠吗？**
  预答：不会。happy-dom 默认 innerWidth（约 1024）→ `(max-width:767.98px)` matches=false → isNarrow=false → collapsed 初值 false = 展开。既有 8 例零回归（GR 已核 spec 无 matchMedia 注入）。**dev 须在新增窄屏用例里用 `vi.stubGlobal('matchMedia', ...)` 显式注入 matches=true 的受控 MediaQueryList，且因 useViewport 是模块单例须 `vi.resetModules()` + 动态 import 拿全新单例**（复刻 useTheme.spec C-1：模块顶层 ref/initialized 只首次求值，跨用例泄漏须 resetModules）。

- **Q4：顶栏 n-space 加 wrap 会不会让 1280 视口也换行影响 e2e？**
  预答：不会。wrap 只在容器宽度不足时换行；1280 视口顶栏元素总宽 < 1280 不触发换行，退出按钮文本/role/可点击性不变（R-5 缓解）。dev 须确认 wrap 属性不改退出按钮可访问名。

- **Q5：表单 `width:100%; max-width:Npx` 在 n-form-item 内会撑满整行吗？影响观感？**
  预答：n-input/n-input-number 在 form-item 内本就块级布局；`width:100%` 让其填满 form-item 内容区，`max-width:Npx` 封顶——宽屏下 form-item 内容区通常 > Npx 故视觉 = 原 Npx 宽（NFR-1 桌面不回归），窄屏 form-item 收窄则随之收窄（FR-6/7）。鉴权 Token 行后跟"查看明文"按钮——dev 须确认 n-input width:100% 不把按钮挤掉（建议 max-width 作用在 n-input 上、按钮在 n-space/同级保持，观察现有布局：input 与 button 平级在 form-item 内，input width:100% 会占满推开 button → **dev 须保留 input 的 max-width 上限使其不占满整行，让 button 仍并排**；这正是 max-width 而非纯 width:100% 的价值）。

## 4. Verdict

**APPROVED FOR DEVELOPMENT**

设计可编码：8 维全 PASS、零新依赖、阈值边界（767.98 < 1280）确保 e2e 零回归经 GR 独立 grep 核实、FR-2 不锁死有 watch 仅跨阈值触发的精确论证、既有 AppLayout/Server/Client spec 零回归经 GR 实读断言核实。

**开发期条件（非阻塞，dev 须落实）**：
- **C-1**：useViewport 窄屏测试须 `vi.stubGlobal('matchMedia', 受控 MediaQueryList)` + `vi.resetModules()` + 动态 import 拿全新单例（模块单例跨用例泄漏防护，复刻 useTheme.spec 范式）。
- **C-2**：表单 max-width 用 `width:100%; max-width:Npx`，确保鉴权 Token 行的 input 不占满整行把"查看明文"按钮挤掉（max-width 封顶保留并排，Q5）。dev 须复核改后 Server/Client 行内 input+button 仍并排。
- **C-3**：AppLayout collapsed 用 `ref(isNarrow.value)` 初值 + `watch(isNarrow)`（非 immediate），不双重赋值（Q2）。
- **C-4**：新增测试同步 bump baseline.json（frontend_tests/test_count/version），并修复 baseline.json 现有结构（PM 已注意到第 11-12 行间有冗余尾巴文本块，dev 写 notes 时一并清理使其为合法单一 JSON 对象）。
- **C-5**：dev 改顶栏 wrap/版本号 v-if 后须确认 e2e 03-dashboard 不受影响（1280 视口不折叠、退出按钮可点击）——静态核实即可（grep 已由 GR 完成，dev 复核 wrap 不改退出按钮可访问名）。
