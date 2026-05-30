# T-067 · responsive-layout — 任务输入（PM → 流水线）

- **模式**：full（完整 7-stage）
- **批次**：ux-ui-uplift-2026-05 第 6 个（最后一个）
- **分区**：纯前端 → dev-frontend 单分区
- **输出语言**：中文（红线）

## 一句话目标

让应用外壳在窄屏/移动端可用——FRP 管理面板"人在外面用手机查看进程状态 / 重启穿透"是真实场景。当前内容栅格已响应式（Dashboard），但布局骨架（侧边栏 / 顶栏 / 表单宽）完全没有窄屏处理。

## 证据 / 现状（行号按内容定位，T-062/064/066 已漂移）

- `web/src/components/AppLayout.vue`：
  - `n-layout-sider` 固定 `:width="200"`，`:collapsed="collapsed"` 仅由用户手点 `show-trigger` 控制（`collapsed = ref(false)`，@collapse/@expand 改值）。**无基于视口宽度的自动折叠**（无 useBreakpoint / matchMedia）。
  - 顶栏 `n-space align="center" style="width:100%"` 横排：品牌名 "FRP Easy"（themeVars.primaryColor）+ 版本 v{{version}}（depth=3）+ flex:1 占位 + （binMissing 时）二进制缺失横幅 n-alert（含下载/上传/进度）+ authStore.user 用户名 + 主题切换 n-select（T-066，width:110px，aria-label='主题切换'）+ "退出登录" n-button。窄屏会挤压换行甚至溢出。
- 表单固定像素宽：
  - `web/src/pages/Server.vue`：bindPort 200px、authToken 360px、dashboardPort 200px、dashboardUser 240px、dashboardPass 240px。
  - `web/src/pages/Client.vue`：serverAddr 300px、serverPort 200px、authToken 360px。
  - （Proxies.vue 640px、Login 360px、Setup 400px、Wizard 内若干——范围聚焦 Server/Client，其余非必须）。
- 正面信号：`web/src/pages/Dashboard.vue` 已用 `n-grid cols="1 m:2" responsive="screen"`——team 认可响应式，只是没做骨架。

## 范围与设计方向（全部前端 dev-frontend）

1. **侧边栏窄屏自动折叠**：监听断点（Naive UI `useBreakpoint()` 或 `matchMedia` 轻量 composable），视口窄于阈值（如 < 768px）时 `n-layout-sider` 自动 `collapsed`。**必须与手动 `show-trigger` 共存**（用户在窄屏仍可手动展开/收起，自动折叠只是默认态）；不要做成"窄屏锁死无法展开"。
2. **顶栏窄屏不溢出**：让顶栏在窄屏 `flex-wrap` 优雅换行 / 或非关键元素（版本号等）窄屏隐藏，确保品牌 + 主题切换 + 退出登录等关键入口窄屏仍可达不溢出。二进制缺失横幅窄屏也要不破版。
3. **表单 max-width 化**：Server/Client 等表单的固定像素宽改为 `max-width`（配 `width:100%`），窄屏自适应不横向溢出，宽屏维持原观感上限。
4. 内容区 `padding`（桌面 24px）窄屏可酌情减小（如 12-16px），但非必须。

## 硬约束 / 红线

- **零新依赖**（Naive UI 已有 `useBreakpoint`；或纯 CSS 媒体查询）。
- **桌面布局不得回归**：自动折叠只在窄屏触发；宽屏维持现状（侧栏展开 200px、表单原宽上限）。
- 新增任何颜色一律用 T-066 的主题 token / `useThemeVars`（insight L16），不硬编码；本任务以布局为主，尽量不引入颜色。
- 不破坏 T-064（菜单图标 + a11y）/ T-066（主题切换控件）在 AppLayout 顶栏的既有结构；自动折叠不能让键盘用户陷入无法展开的状态（a11y）。
- 测试断言**优先可观察量**（断点 mock 后 collapsed 状态 / 表单容器 style 含 max-width / 顶栏 flex-wrap），**尽量少用 naive-ui 组件名查询**（insight L45）。useBreakpoint 测试需 mock `matchMedia`（happy-dom 默认可能无，须显式装）。复用 `web/src/test-utils/`。
- **新增测试同步 bump `scripts/baseline.json`** 的 `frontend_tests`/`test_count`（+version）。
- e2e 不受影响须核实：**playwright.config.ts 用 `devices['Desktop Chrome']` = 视口 1280x720（宽 1280px）**。03-dashboard 在默认视口跑、断言菜单/退出文本。**折叠阈值须 < 1280px（如 768px），确保 e2e 默认视口保持展开态**，否则自动折叠会隐藏菜单文本让 03-dashboard FAIL（insight L34）。
- 更新 `docs/dev-map.md`（AppLayout 响应式行 + 新断点 composable 若有）。

## 产出要求

- `docs/features/responsive-layout/` 下 7 份阶段文档 + PM_LOG.md，全中文。
- `06_TEST_REPORT.md` 含**裸标题** `## Adversarial tests` 段（反向证伪：窄屏 mock 断点→侧栏 collapsed；宽屏→展开不回归；窄屏侧栏仍可手动展开不锁死；表单窄屏不溢出；e2e 默认视口宽度 > 阈值保持展开）。
- `07_DELIVERY.md` 含**裸标题** `## Insight` 段。

## 交付与验证边界

- PM 上下文无 Bash/PS，标 verify_all=PENDING + 执行规格（预期 PASS、frontend_tests 新计数、确认 e2e 视口 > 折叠阈值）。真跑由 batch orchestrator 执行作硬闸门，特别复核 e2e（C.1）+ AppLayout 既有 spec 零回归。
- **不要 commit / push / archive**。
- 当前 baseline：frontend_tests=576 / go_tests=342 / test_count=918。

## 相关历史任务

- T-048 frontend-consistency-cleanup（响应式起步、router.push、可读语义色）
- T-064 menu-icons-and-a11y（AppLayout menuOptions 图标 + a11y）
- T-066 dark-theme-support（AppLayout 顶栏主题切换 n-select + useThemeVars + token 化）
- T-041 server-monitor-page-ui（/server/monitor 入口）

## 适用 insight（PM 已筛，下游须遵循）

- L16：浅色主题严禁硬编码 rgba 白色文字色；用 useThemeVars/n-text 语义色。
- L34：e2e 回归风险先 grep e2e spec 确认实际断言；多数烟雾测试只断言文案可见。
- L45：测试断言优先可观察量，少用 naive-ui 组件名查询。
- T-066 insight：模块单例 composable + osTheme 须 setup 内调；测模块单例不同态须 vi.resetModules + 动态 import。
