# 01 需求分析 — T-067 · responsive-layout

> Harness Stage 1 · Requirement Analyst · 模式 full · 批次 ux-ui-uplift-2026-05 第 6/最后一个 · 全中文

## 1. 目标（一句话）

让 FRP Easy 应用外壳（侧边栏 / 顶栏 / Server·Client 表单）在窄屏/移动端可用：侧栏窄屏自动折叠、顶栏不溢出、表单不横向溢出——支撑"人在外面用手机查看进程状态 / 重启穿透"的真实运维场景。当前内容栅格已响应式（Dashboard `n-grid`），但布局骨架完全无窄屏处理。

## 2. In-scope 行为（编号、可测、无歧义）

- **FR-1（侧栏窄屏自动折叠）**：当视口宽度 < 768px 时，`AppLayout.vue` 的 `n-layout-sider` 默认进入 `collapsed` 态（仅图标 64px）；当视口宽度 ≥ 768px 时，默认为展开态（200px）。"默认态"指：在用户未对该断点区间做过手动展开/收起操作时的态。
- **FR-2（自动折叠与手动 trigger 共存）**：现有 `show-trigger` 手动展开/收起按钮保留并可用。窄屏（< 768px）下用户手动点击展开后，侧栏保持展开，不被自动逻辑在同一断点区间内立即强制收回（不抖动、不锁死）。
- **FR-3（断点变化时重置默认态）**：当视口宽度跨越 768px 阈值（窄→宽 或 宽→窄）时，侧栏 `collapsed` 重置为该新区间的默认态（窄→折叠，宽→展开）。这是"自动折叠"的触发时机；用户的手动操作仅在同一区间内被尊重（FR-2），跨区间由新默认态接管。
- **FR-4（顶栏窄屏不溢出）**：`AppLayout.vue` 顶栏在窄屏下不产生横向溢出（无水平滚动条 / 元素不被裁切）。手段二选一或组合：(a) 顶栏容器 `flex-wrap` 允许优雅换行；(b) 非关键元素（版本号 `v{{version}}`）窄屏隐藏。**关键入口（品牌名、主题切换控件、退出登录按钮）窄屏下仍可见可达**。
- **FR-5（二进制缺失横幅窄屏不破版）**：当 `appStore.binMissing.length > 0` 时，顶栏内的二进制缺失横幅（`n-alert` 含下载/上传/进度）在窄屏下不溢出（随顶栏换行 / 内部收缩），不破坏布局。
- **FR-6（Server 表单 max-width 化）**：`web/src/pages/Server.vue` 表单内固定像素宽输入控件（监听端口 200px、鉴权 Token 360px、Dashboard 端口 200px、Dashboard 用户名 240px、Dashboard 密码 240px）改为 `max-width: <原值>px` + `width: 100%`，使窄屏自适应不横向溢出、宽屏维持原宽度上限观感。
- **FR-7（Client 表单 max-width 化）**：`web/src/pages/Client.vue` 表单内固定像素宽输入控件（服务端地址 300px、服务端端口 200px、鉴权 Token 360px）同 FR-6 改为 `max-width + width:100%`。
- **FR-8（内容区 padding 窄屏可减小，可选）**：`n-layout-content` 桌面 `padding: 24px`，窄屏（< 768px）可减小到 12-16px 以增加可用宽度。此项为可选优化，不做也不构成验收失败。

## 3. Out-of-scope（本轮明确不做）

- **OOS-1**：Proxies.vue（`width: 640px`）、Login.vue（360px）、Setup.vue（400px）、Settings.vue（已 `max-width:480px`）、Wizard.vue 内表单宽度——本轮聚焦 Server/Client（PM INPUT 范围）。Proxies/Login/Setup 表单宽窄屏行为列为已知局限，未来候选。
- **OOS-2**：不引入任何新依赖（断点机制用 Naive UI 内置 `useBreakpoint`/`useBreakpoints` 或原生 `matchMedia`，SA 选定）。
- **OOS-3**：不改菜单结构 / 路由 key / activeKey 计算 / T-064 menuIcon 无障碍名 / T-066 主题切换控件的功能与位置。仅可调整顶栏元素的布局/换行/窄屏隐藏样式。
- **OOS-4**：不改后端 / store / API / 路由守卫 / DB / e2e spec 文件。
- **OOS-5**：不引入新颜色（本任务以布局为主）。若布局确需任何颜色（预期不需要），一律用 `useThemeVars` / `n-text` 语义色（insight L16），不硬编码。
- **OOS-6**：不做"窄屏隐藏整个侧栏改用抽屉/drawer"这类重交互重构——仅做"自动折叠到图标态"，保留现有 n-layout-sider 结构。

## 4. 边界条件

- **BC-1（阈值边界）**：视口宽度恰好 = 768px 时按 **≥ 768px 展开** 判定（即 `< 768` 折叠，`>= 768` 展开），消除歧义。SA 须确保所选断点机制的边界语义与此一致（若 Naive UI `useBreakpoint` 的 's'/'m' 分界点不是 768，SA 据其实际分界点取一个 < 1280 的明确阈值并在设计中固定边界判定方向）。
- **BC-2（e2e 视口）**：`web/playwright.config.ts` 用 `devices['Desktop Chrome']` = 视口 **1280×720（宽 1280px）**。1280 ≥ 阈值（768），故 e2e 默认视口下侧栏保持展开态、菜单文本可见，03-dashboard 零回归。这是硬边界：**阈值必须 < 1280px**。
- **BC-3（横幅 + 窄屏并存）**：`binMissing.length > 0` 且视口 < 768px：横幅随顶栏换行 / 内部收缩，不溢出（FR-5）。
- **BC-4（手动展开 + 仍窄屏）**：用户窄屏手动展开后视口仍 < 768px：保持展开不被强制收回（FR-2）。
- **BC-5（阈值附近抖动）**：视口在 768px 附近反复变化时，matchMedia/breakpoint listener 不泄漏（组件卸载时清理监听）、不产生无限循环更新。
- **BC-6（断点机制不可用降级）**：环境无 `matchMedia`（happy-dom 默认可能无）或 `useBreakpoint` 返回初始/空值时，侧栏退化为现有行为（展开态，由用户手动控制），不抛错、不白屏。
- **BC-7（无障碍）**：自动折叠后键盘用户仍可通过 `show-trigger` 展开侧栏（不陷入无法展开的状态）；T-064 menuIcon 的 aria-label/title/role 在折叠态依然有效。

## 5. 验收标准（可验证）

- **AC-1**：`scripts/verify_all` PASS（编译 + 全量测试 + 双实现计数闸门），由 batch orchestrator Bash 会话真跑。
- **AC-2（FR-1/FR-3）**：测试 mock 窄屏断点（视口 < 768px）→ 断言 AppLayout 侧栏 `collapsed` 可观察量为 true（折叠态）；mock 宽屏（≥ 768px / 默认）→ 断言 `collapsed` 为 false（展开态，桌面不回归）。
- **AC-3（FR-2 不锁死）**：mock 窄屏初始折叠 → 模拟手动展开（触发 `@expand`）→ 断言侧栏变展开态（窄屏仍可手动展开，不锁死）。
- **AC-4（FR-6/FR-7）**：mount Server.vue / Client.vue loaded 态 → 断言目标输入控件容器 style 含 `max-width`（值 = 原像素上限）+ `width: 100%`，不再是裸 `width: <px>`。
- **AC-5（FR-4 不溢出）**：断言顶栏容器具备窄屏换行能力（`flex-wrap: wrap` 或等价）/ 或版本号在窄屏被隐藏的可观察标记；关键入口（aria-label="主题切换" 控件、"退出登录" 文本）在 DOM 中始终存在。
- **AC-6（BC-2 e2e）**：静态核实 `playwright.config.ts` 视口宽（1280）≥ 阈值（768），e2e 03-dashboard 在默认视口保持展开、菜单文本可见，零回归（PM/QA grep e2e spec 确认）。
- **AC-7（既有零回归）**：AppLayout 既有 spec（T-064 菜单图标 5 例 + T-066 主题控件 3 例）、Server/Client 既有 spec（T-047/T-058/T-060/T-062）全部仍 PASS。
- **AC-8（baseline）**：`scripts/baseline.json` 的 `frontend_tests` / `test_count` / `version` 同步 bump，反映新增前端测试数。

## 6. 非功能需求

- **NFR-1（兼容性/桌面不回归）**：宽屏（≥ 768px，含 e2e 1280px 与常规桌面）布局与当前完全一致：侧栏展开 200px、表单原宽上限、顶栏横排。自动折叠/换行仅在窄屏激活。
- **NFR-2（零新依赖）**：断点机制必须用 Naive UI 已有能力或原生 `matchMedia`，不新增 npm 包（OOS-2）。
- **NFR-3（a11y）**：键盘/屏幕阅读器用户不因自动折叠丧失任何导航能力（BC-7）。
- **NFR-4（资源清理）**：任何 matchMedia / resize / breakpoint 监听在组件卸载时清理，无泄漏（BC-5）。

## 7. 相关历史任务

- **T-066 dark-theme-support**（`docs/features/dark-theme-support/`）：AppLayout 顶栏现有结构基线——主题切换 `n-select`（aria-label="主题切换"，退出按钮前）、品牌色 `themeVars.primaryColor`、`useTheme()` 模块单例 composable 范式（本任务断点 composable 若新建可复刻其模块单例 + setup 内调 hook + 测试 vi.resetModules 范式）。
- **T-064 menu-icons-and-a11y**（`docs/features/menu-icons-and-a11y/`）：AppLayout `menuOptions` 的 `menuIcon(glyph,name)` helper + 7 项 a11y 无障碍名；折叠态仅图标的撞车修复——本任务自动折叠会让折叠态成为窄屏常态，T-064 的折叠态 a11y 因此更重要，须保护。
- **T-048 frontend-consistency-cleanup**：Dashboard `n-grid` 响应式起步、router.push 范式。
- **T-041 server-monitor-page-ui**：`/server/monitor` 入口（AppLayout 菜单第 4 项），不可破坏。
- **Dashboard.vue**：`n-grid cols="1 m:2" responsive="screen"` 是项目已认可的响应式范本（content 已响应式，本任务补骨架）。

## 8. 给用户的 open questions

无阻塞性 open question。PM INPUT 已明确折叠阈值方向（< 1280px，建议 768px）、共存语义（自动折叠 + 手动 trigger 共存不锁死）、范围聚焦（Server/Client）。RA 已将这些消化为带默认值的 in-scope 条款（FR-1 阈值 768、FR-2/FR-3 共存语义、OOS-1 范围）。SA 在设计阶段可据 Naive UI `useBreakpoint` 实际分界点微调阈值具体值（仍须 < 1280，BC-1/BC-2），不构成需求歧义。

## 9. Verdict

**READY** — 无悬置 open question，进入 Stage 2 Solution Architect。
