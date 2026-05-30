# T-064 · menu-icons-and-a11y — PM 任务输入

- **Mode**: full（完整 7-stage 流水线）
- **批次**: ux-ui-uplift-2026-05 第 3 个任务
- **分区**: 全前端 → dev-frontend 单分区
- **输出语言**: 中文（红线）

## 一句话目标

修侧边栏菜单图标的语义缺陷 + 补几处真实的键盘/屏幕阅读器可访问性。

## 范围（全部前端，dev-frontend 分区）

### 1. 侧边栏菜单图标（`web/src/components/AppLayout.vue:~130-171`）
当前菜单图标全是裸 Unicode 字形（`h('span',{class:'n-icon'},'⊙'/'⇌'/'⚙'/'◉'/'↗'/'≡'/'⚙')`）。
**核心缺陷**：`⚙`（齿轮）同时用于"服务端配置"(:144) 和"设置"(:169)——侧边栏 `:collapsed-width="64"` 折叠态只剩图标时两项视觉完全相同 → 误点。
必须：
- (a) **消除重复字形**（两项用不同图标）；
- (b) 给每个 menu item 配**无障碍名**（aria-label / title，让折叠态 + 屏幕阅读器能区分）。

### 2. 全屏日志滚动容器（`web/src/components/log/LogList.vue:~18-24` 的滚动容器）
加 `tabindex="0"` + 合适 `role`（如 `role="log"` 或 `region` + aria-label）让纯键盘用户能聚焦并滚动长日志。
**范式参考同文件 `:30-41` 的 paused-banner**（已有 `role=button`+`tabindex=0`+enter/space 完整键盘支持，team 懂这套，只是滚动容器漏了）。

### 3. 复制按钮瞬时态 aria-live（`web/src/components/FirewallHint.vue:~21,30` + `web/src/components/PublicIpDetector.vue:~21` 的"已复制 ✓"切换元素）
给该可见反馈元素加 `aria-live="polite"` / `role="status"`，让屏幕阅读器播报复制结果。
**只动可访问性属性，不动 copyToClipboard 逻辑**（T-061 已抽好 util，行为不变）。

## 设计约束（重要）

- **强偏好零新依赖**：不引入 @vicons / 任何图标库（增 bundle + 需过 build/verify）。可选实现：(a) 换成互不相同的字形 + 无障碍名；(b) 内联 SVG（无依赖、跨字体一致）。由 SA 权衡定夺，但**不得新增 npm 依赖**，除非有压倒性理由并经 Gate Reviewer 批准。
- 不改菜单**结构 / 路由 / activeKey 计算逻辑**（`AppLayout.vue:~122-128` 对 `/server/monitor` 的特判保持不变）——只换图标 + 加无障碍名。
- 不破坏 LogViewer 子系统既有行为（T-036）；FullscreenLogModal 用 `n-modal preset=card`，Naive UI 自带焦点陷阱 + Esc，**不要重复造**。

## 硬约束 / 红线

- 浅色/暗色主题下文字色禁硬编码 `rgba(255,255,255,*)`（insight L16）；本任务若动到颜色用 `useThemeVars`/语义色（但本任务主要是图标+a11y，尽量不引入颜色改动，留给 T-066）。
- 测试断言**全用 DOM 属性/文本查询 / getExposed，零 naive-ui 组件名查询**（insight L45）。a11y 断言可查 `aria-label`/`role`/`tabindex` 等 DOM 属性。
- **新增测试同步 bump `scripts/baseline.json`** 的 `frontend_tests`/`test_count`（+version）。
- e2e 不受影响须核实（菜单文本不变，03-dashboard 若按 menu 文本断言需确认无碍；insight L34）。
- 更新 `docs/dev-map.md` AppLayout / LogList 相关行（若有结构性说明变化）。

## 产出要求

- `docs/features/menu-icons-and-a11y/` 下 7 份阶段文档 + PM_LOG.md，全中文。
- `06_TEST_REPORT.md` 含**裸标题** `## Adversarial tests` 段（反向证伪：折叠态两个菜单项可由无障碍名区分 / 每个 menu item 有非空可访问名 / 日志容器可聚焦 / 复制反馈元素有 aria-live）。
- `07_DELIVERY.md` 含**裸标题** `## Insight` 段（如有）。

## 交付与验证边界（重要）

- PM 上下文**无 Bash/PS**，标 verify_all=**PENDING** + 执行规格（预期 PASS、frontend_tests 新计数）。真跑由 batch orchestrator 执行作硬闸门。
- **不要 commit / push / archive**。
- 当前 baseline：frontend_tests=534 / go_tests=333 / test_count=867 / version=29。

## 相关历史任务（PM 已预扫）

- **T-036** log-ui-ux-polish（已归档 `docs/features/_archived/log-ui-ux-polish/`）：LogViewer/LogList 子系统设计，paused-banner 键盘范式来源。
- **T-002** zero-config-quickstart（已归档）：AppLayout 菜单初始结构。
- **T-041** server-monitor-page-ui：新增"服务端监控"menu item（`◉`）+ activeKey `/server/monitor` 特判。
- **T-061** clipboard-util-extract：抽出 `web/src/utils/clipboard.ts::copyToClipboard`，三组件复制逻辑已统一（本任务不动其逻辑）。
- **T-058 / T-062**：FirewallHint / PublicIpDetector 既有测试范式（DOM 文本 + getExposed + clipboard mock）。

## 适用 insight（PM 已筛，下游须遵守）

- **L16**：浅色/暗色主题禁硬编码 `rgba(255,255,255,*)` 文字色 → 本任务尽量不引入颜色改动。
- **L34**：e2e 烟雾测试通常不点破坏性按钮 / 需 grep 核实是否按 menu 文本断言；菜单文本不变预期 e2e 零影响，仍须核实。
- **L42**：1:1 行为搬运无新行为（本任务 a11y 属性附加，不改交互逻辑）。
- **L45**：测试断言用 DOM 属性查询，零 naive-ui 组件名。
