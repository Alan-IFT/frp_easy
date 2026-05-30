# 01 需求分析 — T-064 · menu-icons-and-a11y

> Harness stage 1 · Requirement Analyst · mode=**full** · 批次 ux-ui-uplift-2026-05 第 3 个 · 输出中文。

## 1. Goal（目标）

修复 frp_easy Web UI 三处可访问性缺陷：(a) 侧边栏菜单两个不同导航项共用同一齿轮字形 `⚙`，在折叠态（`:collapsed-width="64"`，仅显示图标）下视觉无法区分导致误点；(b) 全屏日志滚动容器纯键盘用户无法聚焦滚动；(c) 复制按钮的"已复制"瞬时反馈对屏幕阅读器不可感知。

## 2. In-scope behaviors（在范围内，可测）

### IS-1 消除菜单重复字形
- `web/src/components/AppLayout.vue` `menuOptions`（:130-171）中，"服务端配置"(key=`server`, :144) 与"设置"(key=`settings`, :169) 当前**均渲染字形 `⚙`**。修复后这两项必须渲染**互不相同**的图标。
- 全部 7 个 menu item（含子菜单项）渲染的图标字形/标识两两互不相同（不止修这一对，而是整组无重复——折叠态下任意两个顶层项都必须可凭图标区分）。

### IS-2 每个 menu item 配非空无障碍名
- `menuOptions` 中**每个顶层项**（`dashboard` / `proxies` / `server` / `server/monitor` / `client` / `logs` / `settings`）的渲染结果带一个**非空可访问名**（aria-label 或 title 或等价 ARIA 文本），使折叠态（图标无文字标签）下屏幕阅读器与悬停可区分。
- 无障碍名取值与该项 `label` 中文文案语义一致（如 `server` → 含"服务端配置"、`settings` → 含"设置"），且 `server` 与 `settings` 两项无障碍名**不相同**。

### IS-3 日志滚动容器键盘可聚焦
- `web/src/components/log/LogList.vue` 的滚动容器 `div.log-list-scroll`（:18-24）添加 `tabindex="0"` 使纯键盘用户可 Tab 聚焦后用方向键/PageUp/PageDown 滚动长日志。
- 该容器添加合适 ARIA role（`role="log"` 或 `role="region"`）+ 非空 `aria-label`，使屏幕阅读器识别其为日志区域。
- 范式对齐同文件 paused-banner（:30-41 已有 `role`/`tabindex`/键盘支持）。

### IS-4 复制反馈瞬时态 aria-live
- `web/src/components/FirewallHint.vue` 中显示"已复制 ✓"/"复制"切换文案的元素（单条命令复制按钮 :21，复制全部按钮 :30）所在的**可见反馈承载元素**添加 `aria-live="polite"` 或 `role="status"`，使复制成功/失败的文案变化被屏幕阅读器播报。
- `web/src/components/PublicIpDetector.vue` 显示"已复制 ✓"/"复制"切换的元素（:21）同样添加 `aria-live="polite"` 或 `role="status"`。
- **仅添加 ARIA 属性，不改 `copyToClipboard`（`web/src/utils/clipboard.ts`，T-061 已抽）逻辑、不改复制行为、不改文案内容**。

### IS-5 baseline + dev-map 同步
- 新增前端测试同步 bump `scripts/baseline.json` 的 `frontend_tests` / `test_count` / `version`。
- 若结构性说明变化，更新 `docs/dev-map.md` AppLayout / LogList 相关行。

## 3. Out-of-scope（明确不做）

- **OOS-1** 不引入任何新 npm 依赖（@vicons / 任何图标库）。零新依赖是硬 NFR（见 NFR-1）。
- **OOS-2** 不改菜单**结构**（项数、层级、children）、**路由 key**、**activeKey 计算逻辑**（`AppLayout.vue:122-128`，含 `/server/monitor` 精确匹配特判保持不变）。
- **OOS-3** 不改 `copyToClipboard` 逻辑 / 复制交互行为 / 复制文案内容（"复制"/"已复制 ✓"/"复制全部"等文字不变）。
- **OOS-4** 不破坏 LogViewer 子系统（T-036）既有行为：滚动状态机、followTail、search、空态/错误态/无命中分支均不改逻辑，仅在滚动容器上附加 a11y 属性。
- **OOS-5** 不重复造 modal 焦点陷阱：FullscreenLogModal 用 `n-modal preset=card`，Naive UI 自带焦点陷阱 + Esc，不动。
- **OOS-6** 不引入颜色/主题改动（图标 + a11y 属性为主）；主题色相关留 T-066。若图标实现确需颜色，遵守 NFR-3（禁硬编码白色文字色）。
- **OOS-7** 不改 paused-banner（:30-41 已合规，仅作范式参考）。

## 4. Boundary conditions（边界条件）

- **BC-1（折叠 vs 展开）** 折叠态（仅图标）和展开态（图标+文字）下，IS-2 的无障碍名都存在；展开态文字标签照常显示不受影响。
- **BC-2（有 children 的菜单项）** "日志"项（key=`logs`，:158-165）有子项 `frpc 日志`/`frps 日志`。IS-2 的"每个顶层项有非空可访问名"覆盖该项的顶层入口；子项是否加无障碍名由 SA 定夺（子项默认有文字 label，折叠态下展开为悬浮子菜单，优先级低于顶层）。
- **BC-3（LogList 状态分支）** 滚动容器 `div.log-list-scroll`（:18-24）在 `v-else` 分支（即非错误、非加载态）才渲染——错误态（:4）/加载态（:12）渲染的是 `.log-empty` 而非滚动容器。IS-3 的 tabindex/role 加在滚动容器上，故仅列表/空态/无命中三分支可聚焦；错误/加载态不可聚焦滚动容器（符合预期，无内容可滚）。
- **BC-4（复制反馈两态）** IS-4 的 aria-live 元素在"未复制"（显示"复制"）与"已复制"（显示"已复制 ✓"）两态下均存在于 DOM（文案切换而非元素增删），aria-live 区域内容变化触发播报；首次渲染（"复制"初始态）不应误触发播报（aria-live=polite 语义即仅在后续变化时播报，符合）。
- **BC-5（多复制按钮共存）** FirewallHint 可有多个端口 → 多个单条复制按钮 + 一个"复制全部"。每个反馈承载元素独立携带 aria-live（或承载元素粒度由 SA 定，但须保证每个可切换文案的反馈点被覆盖）。
- **BC-6（PublicIpDetector 无结果态）** `copied` 切换元素只在 `result.ip` 成功分支（:11-27）渲染；检测失败（:29-36 warning）无复制按钮，IS-4 不涉及该分支。

## 5. Acceptance criteria（验收，全部可反向证伪）

- **AC-1** `menuOptions` 渲染后，7 个顶层项图标字形/标识两两互不相同；特别地 `server`（服务端配置）与 `settings`（设置）的图标不相同（反向证伪：若两者仍相同则 FAIL）。
- **AC-2** 每个顶层 menu item 渲染结果有**非空**可访问名（aria-label/title/等价 ARIA 文本，trim 后长度 > 0）（反向证伪：任一项可访问名为空则 FAIL）。
- **AC-3** `server` 与 `settings` 两项可访问名不相同且各自语义匹配其 label（反向证伪：折叠态两项无障碍名相同则 FAIL）。
- **AC-4** `LogList` 列表分支渲染时，`div.log-list-scroll` 具有 `tabindex="0"`（反向证伪：缺失或非 0 则 FAIL）。
- **AC-5** `div.log-list-scroll` 具有非空 `role` 属性（值为 `log` 或 `region`）+ 非空 `aria-label`（反向证伪：缺 role 或 aria-label 则 FAIL）。
- **AC-6** `LogList` 错误态/加载态分支下不存在 `div.log-list-scroll`（验证 IS-3 仅作用于内容分支，且现有三态分支逻辑未被破坏）。
- **AC-7** `FirewallHint` 单条复制反馈承载元素与"复制全部"反馈承载元素均带 `aria-live="polite"` 或 `role="status"`（反向证伪：任一缺失则 FAIL）。
- **AC-8** `PublicIpDetector` 成功分支的复制反馈承载元素带 `aria-live="polite"` 或 `role="status"`（反向证伪：缺失则 FAIL）。
- **AC-9** 复制行为不变：点击复制仍调用 `copyToClipboard` 并保持"复制"↔"已复制 ✓"文案切换（反向证伪：行为或文案改变则 FAIL）。
- **AC-10** `verify_all` PASS；新增前端测试数与 `baseline.json` `frontend_tests` 新值一致（go_tests 不变 333）。
- **AC-11** e2e 不受影响：菜单 label 文案不变（仅换图标 + 加 ARIA），现有 e2e 若按菜单文本断言不受影响（须 grep 核实，insight L34）。

## 6. Non-functional requirements（非功能需求）

- **NFR-1（零新依赖，硬约束）** 不新增任何 npm 依赖。图标实现限于：(a) 互不相同的 Unicode 字形 + 无障碍名，或 (b) 内联 SVG（无依赖）。由 SA 权衡。若 SA 认为有压倒性理由引依赖须经 Gate Reviewer 显式批准。
- **NFR-2（结构稳定）** 不改菜单结构/路由/activeKey；不改 copyToClipboard 逻辑；LogViewer 子系统行为保真。
- **NFR-3（主题色，insight L16）** 若图标实现引入颜色，禁硬编码 `rgba(255,255,255,*)`；用 `useThemeVars`/语义色随主题自适应。本任务尽量零颜色改动。
- **NFR-4（测试范式，insight L45）** 测试断言全用 DOM 属性/文本查询 / getExposed；零 naive-ui 组件名查询。a11y 断言查 `aria-label`/`role`/`tabindex`/`aria-live` 等 DOM 属性。
- **NFR-5（跨字体一致性）** 若选 Unicode 字形方案，所选字形须在常见系统字体下可渲染（不出现豆腐块/缺字）——SA 选字形时考量；内联 SVG 方案天然无此风险。

## 7. Related tasks（相关历史，引用不重述）

- **T-036** log-ui-ux-polish（归档 `docs/features/_archived/log-ui-ux-polish/`）：LogViewer/LogList 子系统设计来源；paused-banner（`LogList.vue:30-41`）的 `role=button`/`tabindex=0`/`@keydown.enter`/`@keydown.space.prevent` 是 IS-3 的范式参考。team 已懂这套键盘范式。
- **T-002** zero-config-quickstart（归档）：AppLayout 菜单初始结构。
- **T-041** server-monitor-page-ui：新增"服务端监控"menu item（key=`server/monitor`, :148-151, 图标 `◉`）+ `AppLayout.vue:122-128` activeKey 对 `/server/monitor` 的精确匹配特判（OOS-2 保持不变）。
- **T-061** clipboard-util-extract：`web/src/utils/clipboard.ts::copyToClipboard` 已抽，三组件复制逻辑统一（IS-4 不动其逻辑，OOS-3）。
- **T-058 / T-062**：`FirewallHint.spec.ts` / `PublicIpDetector.spec.ts` 既有测试范式（DOM 文本 + getExposed + clipboard mock + `vi.mock('naive-ui')` message 单例 spy），新增 a11y 断言应复用。
- **注意**：AppLayout.vue 与 LogList.vue **当前均无独立 spec 文件**（PM 已 glob 核实：`web/src/components/__tests__/` 无 AppLayout/LogList spec）——dev-frontend 需新建 spec。

## 8. Open questions for user（用户决策项）

无阻塞性歧义。以下三项是设计细节，已由历史范式/约束隐含倾向，**交 SA 在 02 中定夺**，不阻塞（标 READY）：

- **Q-1（已可推导，交 SA）** 菜单无障碍名用 `aria-label` 还是 `title` 还是两者？候选：(a) `aria-label`（屏幕阅读器主路径）；(b) `title`（悬停 tooltip + 部分 AT 可读）；(c) 两者。倾向：Naive UI `MenuOption` 透传 ARIA 的可行方式由 SA 实测（icon render 函数返回的 span 可直接挂 `aria-label`/`title`，或在 option 上挂 `props`）。
- **Q-2（已可推导，交 SA）** 滚动容器 role 选 `log`（语义最贴日志流）还是 `region`+aria-label？候选：(a) `role="log"`（ARIA 专为日志/聊天流设计，AT 会自动跟读新增行）；(b) `role="region"`+aria-label（通用地标）。倾向 (a) `role="log"` 语义最精确，但需 SA 确认对既有滚动/followTail 无副作用。
- **Q-3（已可推导，交 SA）** 复制反馈用 `aria-live="polite"` 还是 `role="status"`？候选：(a) `aria-live="polite"`；(b) `role="status"`（隐含 `aria-live=polite` + `aria-atomic=true`）。二者等价偏好，SA 择一并全项目一致。

## 9. Verdict（结论）

**READY**

无阻塞用户的歧义。三项 open questions 均为设计细节，由 paused-banner 范式（T-036）+ ARIA 标准 + 零依赖约束可推导，交 Solution Architect 在 02_SOLUTION_DESIGN.md 定夺。范围清晰、验收可测可反向证伪、约束明确（零新依赖 / 不改结构路由 / 不改复制逻辑 / LogViewer 保真）。
