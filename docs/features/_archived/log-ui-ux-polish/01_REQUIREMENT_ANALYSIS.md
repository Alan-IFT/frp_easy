# 01 — Requirement Analysis · T-036 / log-ui-ux-polish

> 任务模式：**full**
> Stage：1（Requirement Analyst）
> 上游输入：`docs/features/log-ui-ux-polish/INPUT.md`（用户原始请求 + PM 全权授权 + 关键上下文）
> 范围声明：本任务仅涉及 `web/src/components/LogViewer.vue` 与 `web/src/pages/Logs.vue` 的样式与用户体验改造，**纯前端**。后端 API、路由、procmgr、logrotate、日志文件落盘策略不在范围。

---

## 1. Goal

把 frpc / frps 日志页从"单一深色 `n-code` 块 + 自动刷新开关 + 刷新按钮"的极简形态，升级为可读、可控、可观测、主题感知的日志查看器，让用户在排障与日常巡检场景下用纯前端样式与交互优化得到工程级体验。

---

## 2. In-scope behaviors（编号 / 可测）

> 每条均可由 DOM 断言、Vitest 组件测试或用户可见行为验证。下列"等级解析"统一指：用 regex 从每行文本前缀匹配 frp 标准格式 `YYYY-MM-DD HH:MM:SS [级别] ...`，提取 `ERROR / WARN / INFO / DEBUG / TRACE` 五级；无法解析的行归为 `PLAIN` 等级，按普通文本渲染。

### 2.1 日志可读性

1. **等宽字体**：日志正文使用 CSS `font-family` 等宽字体栈（含 `ui-monospace`、`Consolas`、`Menlo`、`Monaco`、`monospace` 兜底），不允许 inline style。
2. **字号 / 行距**：正文默认 13 px，`line-height: 1.55`；最小可读字号 12 px、最大 16 px。
3. **等级着色**：解析得到的 `ERROR` 行整行前景色用 Naive UI 主题 `errorColor`；`WARN` 用 `warningColor`；`INFO` 用主题默认前景（不额外着色）；`DEBUG` / `TRACE` 用 `textColor3`（更弱）；`PLAIN` 用默认前景。
4. **行号列**：左侧渲染当前缓冲内的本地行号（从 1 起，缓冲滚动时行号同步前移），行号列宽固定字符，前景用 `textColor3`，且 `user-select: none`（复制日志正文不带行号）。
5. **行间分隔**：行之间用 1 px `dividerColor` 极淡分隔或仅依靠 `padding-y: 2px` 视觉分组，二选一并固定（**PM 决策**：用 padding-y 分组，不画分隔线，避免视觉噪声）。
6. **行内分块视觉**：`timestamp`、`level`、`message` 三段在解析成功时以 CSS class 区分，便于将来扩展；解析失败行整行作为 `message`。

### 2.2 查看效率

7. **关键字搜索**：顶部工具条提供 `n-input` 搜索框，输入即时过滤（**PM 决策**：默认大小写不敏感、纯子串匹配；不支持 regex；输入框旁附"Aa"切换开关切大小写敏感，**不**附 regex 开关）。
8. **搜索高亮**：命中行整行保留，匹配子串用 `<mark>` 包裹，背景色用主题 `primaryColorSuppl` 的 alpha 变体；未命中行隐藏（不是淡出）。
9. **等级筛选**：顶部工具条提供等级多选 `n-select`（选项：`ERROR / WARN / INFO / DEBUG / TRACE / PLAIN`，默认全选）；取消勾选某等级时该等级所有行被隐藏。
10. **"跟随尾部" 自动滚动**：单独开关 `n-switch`，默认 **开**（**PM 决策**：与下方 2.5 状态反馈联动，开时新增日志自动滚到底；关时滚动位置由用户控制）。
11. **滚动暂停语义**：当 `跟随尾部` 开且用户主动向上滚（鼠标滚轮 / 拖滚动条 / PageUp / 触屏滑动）使视口距底 > 32 px 时，自动**临时**关闭"跟随尾部"并显示提示条 "已暂停跟随；点击此处回到底部"；用户点击提示条 → 滚到底 + 重新开启"跟随尾部"。
12. **滚到底快捷按钮**：工具条始终显示 `↓ 底部` 小按钮，无论是否暂停，点击即滚到底（不修改"跟随尾部"开关状态）。

### 2.3 操作便利

13. **复制全部**：工具条 `复制` 按钮，点击复制当前**可见**（过滤后）日志全文到剪贴板；`navigator.clipboard.writeText` 失败时降级 `document.execCommand('copy')`；成功 / 失败均用 `useMessage` 反馈。
14. **清屏**：工具条 `清屏` 按钮，点击清空当前缓冲区 `lines.value = []` 并复位 `currentOffset.value = 0`，**不**调后端；后续 `loadIncremental` 从新偏移继续追加。
15. **折行切换**：`n-switch` 控制 `white-space: pre` vs `pre-wrap`，默认 `pre-wrap`（**PM 决策**：默认折行，开关持久化到 `localStorage` key `logViewer.wrap`）。
16. **高度调节**：日志容器最大高度可调节（**PM 决策**：用 Naive UI `n-slider` 在工具条折叠区给出 300 / 500 / 800 / "全屏"四档；默认 500 px；持久化到 `localStorage` key `logViewer.height`）。
17. **全屏切换**：工具条 `全屏` 按钮（**PM 决策**：用 Naive UI `n-modal` `preset="card"` `style="width: 95vw; height: 90vh"` 模拟全屏，不调 Fullscreen API；理由：Naive UI 主题感知 / 暗色一致 / 不与浏览器 ESC 行为冲突 / 包体零增量）；Modal 内复用同一 `LogViewer` 渲染，关 Modal 不丢缓冲。
18. **导出按钮**：**PM 决策**：本期**不做**导出文件按钮。理由：浏览器 `复制` + 用户自己粘贴到文本编辑器已覆盖 95% 场景，导出需要追加 `Blob + URL.createObjectURL`、文件名命名约定、字符集声明等设计成本；后续如有需要可作单独 trivial 任务追加。`Open questions` Q-f 已就地拍板"否"。

### 2.4 状态反馈

19. **加载中**：首次 `loadTail` 进行中时容器内显示 `n-spin` + 文案 "正在加载日志…"；不阻塞页头与工具条。
20. **空态**：缓冲为空且非加载中时，容器内居中显示中性图标 + 文案 "暂无日志输出"。等级筛选或搜索导致**无命中**时，文案改为 "无匹配日志（已应用筛选 / 搜索）+ '清空筛选' 链接"。
21. **错误反馈**：`loadTail` / `loadIncremental` 抛错时，**首次** `loadTail` 失败显示容器内红字 + `重试` 按钮（不再静默 swallow）；自动刷新轮询失败时仅在工具条右侧显示小红点 + 悬停 tooltip 显示最近一次错误消息 + 失败计数（**PM 决策**：连续 3 次轮询失败后自动停止 polling 并 `useMessage.error('自动刷新已停止：连续 3 次拉取失败')`，避免在无网络时无限轰炸）。
22. **自动刷新心跳**：工具条显示 "上次更新：HH:MM:SS"，每次 `loadIncremental` 成功后更新；首次 `loadTail` 后亦更新。
23. **当前行数指示**：工具条右侧显示 `<当前缓冲行数> / <上限>`，例如 `342 / 500`，让用户感知缓冲填充程度。

### 2.5 主题响应

24. **暗色 / 浅色双适配**：所有颜色（背景、前景、等级色、行号色、搜索高亮、Modal 背景）必须读 Naive UI 主题 token（`useThemeVars()`），**不允许** hardcode `#1a1a1a` 等具体十六进制（**PM 决策**：沿用 Naive UI 默认 token，不自定义 token；理由：长期可维护性 > 一次性视觉特异性）。
25. **主题切换实时跟随**：在 `n-config-provider` 切换 light / dark 时，日志组件颜色立即跟随，不需刷新页面。

### 2.6 切换 frpc / frps

26. **切 kind 时**：复用 `LogViewer.vue` 现有 watch；切换时停 polling、重置 `autoRefresh = false`、清缓冲、重置 offset、重新 `loadTail`；搜索关键字、等级筛选、折行、高度、跟随尾部状态在跨 kind 切换时**保留**（用户偏好持久化层面），但当前缓冲清空（数据不混）。

---

## 3. Out-of-scope（本期明确不做）

1. 后端 `/api/v1/logs/{kind}` 路由、参数、返回结构改造（数据契约不变）。
2. 日志落盘策略、轮转、留存周期、压缩、外部转储（procmgr / logrotate 相关）。
3. 多 kind 并排 / 分屏对比视图（如同屏看 frpc + frps）。
4. 日志告警 / 阈值通知 / WebSocket 推流（仍走 2s 轮询）。
5. 引入 xterm.js / monaco / react-window / @tanstack/virtual 等重型库做虚拟滚动（500 行内手算 DOM 节点数 ≤ 500 × ~6 子节点 ≈ 3000，浏览器无压力）。
6. 导出为文件按钮（Q-f 决策 = 否）。
7. 列宽 / 字段顺序 / 字段显隐自定义（如隐藏 timestamp 列）—— 解析后 3 段视觉分块固定。
8. 时区切换 / timestamp 本地化重格式化（保持后端原文）。
9. 服务端推送 SSE / WebSocket 协议升级。
10. 多用户协作（其他用户的视图状态同步）。

---

## 4. Boundary conditions

| 编号 | 条件 | 期望行为 |
|---|---|---|
| BC-1 | 日志为空（`lines = []`，非加载中） | 渲染 2.4 §20 空态文案 "暂无日志输出"；不渲染行号列；不渲染空白容器留 500 px 空洞 |
| BC-2 | 单行超长（> 1000 字符，例如 panic stack 单行 5 KB） | 在 `pre-wrap` 模式下正常折行不溢出容器宽度；在 `pre` 模式下水平滚动条出现；行号列宽不变；CSS `word-break: break-all` 保留以兜底无空格的二进制噪声 |
| BC-3 | 500 行满载 + 新增 incremental 数据 | `lines.value = [...lines.value, ...newLines].slice(-500)` 保持上限；行号显示**当前缓冲内**编号（即新增行不会让"行 1" 变 "行 -100"，行号始终 1..N where N = lines.length） |
| BC-4 | 用户切 kind（frpc → frps） | 停 polling + 关 autoRefresh + 清缓冲 + 重置 offset + loadTail；搜索 / 筛选 / 折行 / 高度 / 跟随尾部 UI 状态保留 |
| BC-5 | 自动刷新中切 kind | 同 BC-4；当前正在 in-flight 的 `loadIncremental` Promise 即便后到也不污染新 kind 的缓冲（用 `props.kind` 闭包比对 + 丢弃过期响应；具体实现由 Architect 决） |
| BC-6 | 网络错误持续轮询 | 见 2.4 §21：连续 3 次失败后停 polling + message.error + 工具条小红点保留直到用户手动重试 |
| BC-7 | 用户滚动到中间 vs 末尾 | 见 2.2 §11：用户主动向上滚使视口距底 > 32 px → 临时关跟随 + 提示条；用户回到底部（距底 ≤ 32 px）→ **不**自动重开跟随（避免抖动），需用户点提示条 / `↓ 底部` 按钮 / 切换开关显式重开 |
| BC-8 | 搜索无命中 | 见 2.4 §20 "无匹配日志（已应用筛选 / 搜索）+ '清空筛选' 链接"；行号列仍渲染但仅为空容器宽度占位 |
| BC-9 | 等级筛选全部取消勾选 | 视为"无命中"，渲染 BC-8 空态并提示 "请至少选择一个日志等级" |
| BC-10 | 全屏 Modal 打开期间，父 `Logs.vue` 卸载 | Modal 内 LogViewer 跟随卸载、停 polling、清 timer；不泄漏 `setInterval` |
| BC-11 | 浏览器选项卡后台（`document.visibilityState = 'hidden'`） | 不在范围（不做后台节流优化）—— 但 polling 自然继续，OOS-11 |
| BC-12 | 缓冲上限 500 用户自定 | **PM 决策**：本期不可自定，固定 500（Q-c）|
| BC-13 | `localStorage` 不可用（无痕模式 / Safari ITP 拦截） | 偏好持久化降级为"内存内 session 期保留"；不报错；不弹 message |

---

## 5. Acceptance criteria

> 每条由 Vitest 组件测试或 Playwright e2e（视 stage 6 决策）覆盖。`E2E fixture 类 helper 不用 Vitest mock`（insight L37），但本任务的核心 composable / 组件渲染必须有 Vitest 单测（insight L36）。

| AC | 描述 | 验证手段 |
|---|---|---|
| AC-1 | 渲染含 `ERROR` 行的固定 fixture，DOM 中该行容器 class 含 `level-error`，computed style 前景色与 `useThemeVars().errorColor` 一致 | Vitest mount + `vi.mock('naive-ui', ...)` 全套（按 insight L37） |
| AC-2 | 输入搜索关键字 `connection refused`（小写）后，3 行包含该子串（任意大小写）的行可见，其余行隐藏；大小写敏感开关打开后，仅小写命中行可见 | Vitest mount + setProps + 断言 visible 行数 |
| AC-3 | 等级多选去掉 `INFO`，渲染只剩 ERROR / WARN / DEBUG / TRACE / PLAIN 行 | Vitest mount + 断言 visible 行数 |
| AC-4 | 跟随尾部开启 + push 新行 → 容器 `scrollTop` 在 nextTick 内等于 `scrollHeight - clientHeight`（误差 ≤ 1 px） | Vitest mount + `await nextTick()` + 模拟 DOM |
| AC-5 | 跟随尾部开启 + 模拟用户向上滚（设 `scrollTop = 0`）→ 提示条出现 + 内部状态 `autoFollow` = false；新 push 不再滚到底 | Vitest mount + DOM event dispatch |
| AC-6 | 点击 `复制` 按钮，`navigator.clipboard.writeText` 收到当前可见日志拼接字符串（行间 `\n`，不含行号、不含 HTML tag） | Vitest mount + `vi.spyOn(navigator.clipboard, 'writeText')` |
| AC-7 | 点击 `清屏` → `lines.value = []`、`currentOffset.value = 0`、空态文案出现；后端 API **零调用**（断言 mock fetch call count 不变） | Vitest mount + mock API |
| AC-8 | 折行开关切换 → 容器 class 在 `wrap` / `nowrap` 之间切换；`localStorage.getItem('logViewer.wrap')` 与开关一致 | Vitest mount + localStorage mock |
| AC-9 | 高度档位切到 800 → 容器 `max-height` style 计算值 = 800 px | Vitest mount + getComputedStyle |
| AC-10 | 点击全屏 → `n-modal` 出现且 visible；点击 Modal 关闭 → Modal 消失，主容器缓冲数据未变 | Vitest mount + 断言 Modal show prop |
| AC-11 | 切 kind `frpc → frps` → 缓冲清空 + autoRefresh 关 + polling timer 清；搜索 / 折行 / 高度 / 跟随尾部状态保留 | Vitest mount + setProps |
| AC-12 | 连续 3 次模拟 `apiGetLogsIncremental` reject → polling timer 被 clear、`useMessage.error` 被调用一次、工具条小红点 class 出现 | Vitest mount + mock API reject |
| AC-13 | 暗色主题下背景色与浅色主题不同（断言 computed background-color 在两次 mount 下不同） | Vitest mount + n-config-provider theme 切换 |
| AC-14 | 单行超长（fixture 含 1 行 2000 字符）+ `pre-wrap` → 容器 `scrollWidth ≤ clientWidth + 行号列宽`（不出现水平滚动） | Vitest mount + DOM rect 测量 |
| AC-15 | 空缓冲 → 空态文案 "暂无日志输出" 可见；行号列不渲染（断言 querySelector('.line-number-col') === null） | Vitest mount |
| AC-16 | 首次 loadTail 失败 → 容器内 `重试` 按钮可见，点击后再次调 `apiGetLogsTail`（断言 mock call count = 2） | Vitest mount + mock API reject once |
| AC-17 | 包体增量 < 50 KB gzip 后（对照基线 `web/dist/assets/index-*.js`） | `scripts/verify_all` 中 frontend build size diff 或手工 du 对比 |

---

## 6. Non-functional requirements

| NFR | 描述 | 验证 |
|---|---|---|
| NFR-1 | 首次渲染 500 行 fixture < 200 ms（Chromium / 中端笔记本基准） | Stage 6 QA 用 `performance.now()` 实测，写 `06_TEST_REPORT.md` |
| NFR-2 | 500 行 `lines.join('\n')` 拼接 + render 不阻塞主线程超过 50 ms（不引入 long task） | Stage 6 QA Chrome DevTools Performance 录制 |
| NFR-3 | bundle 体积增量 ≤ 50 KB（gzip 前；不引入 xterm.js / monaco / react-window / @tanstack/virtual / fuse.js 等重型库） | `scripts/verify_all` build 产物 du 对比 |
| NFR-4 | 不引入 inline style（合规 `.harness/rules/50-fullstack.md` "布局不用 inline style"） | Stage 5 Code Reviewer 静态 grep `style="`（仅允许零星动态绑定且必须 justify） |
| NFR-5 | 不引入新 npm 依赖（除非有不可替代理由，并在 02_SOLUTION_DESIGN 写明） | `package.json` diff 审查 |
| NFR-6 | 所有等级颜色 / 文案为中文 UI 时使用中文（按 `.harness/rules/00-core.md` 输出语言总规） | Stage 5 grep 英文残留 |
| NFR-7 | 安全：搜索高亮 `<mark>` 包裹时必须做 HTML escape，避免日志正文 XSS（如日志包含 `<script>`） | Stage 5 静态审查 + AC-2 隐式覆盖 |
| NFR-8 | 可访问性：搜索框 / 开关 / 按钮均有 `aria-label` 或 Naive UI 默认语义 | Stage 5 静态审查 |
| NFR-9 | `localStorage` key 命名空间 `logViewer.*` 不与其他模块冲突；缺失时优雅降级 | BC-13 + Vitest 单测 |

---

## 7. Related tasks

| 任务 | 关联性 | 复用 |
|---|---|---|
| **T-001 web-ui-mvp**（2026-05-16，已归档） | LogViewer.vue 与 Logs.vue 在 T-001 首次落地，自此**未做样式优化** | 阅读 `docs/features/_archived/web-ui-mvp/02_SOLUTION_DESIGN.md` 了解原始数据契约假设 |
| **T-022 service-mode-stderr-bridge**（2026-05-23，trivial） | 修了服务模式 ui.log 来源（exposureNotice 走 logger），属后端落盘改造；与本任务**不冲突**，但意味着 `logs/frpc.log` / `logs/frps.log` 文件路径与格式与 T-022 后状态对齐 | 仅参考，不重复设计 |
| **T-032 proxy-form-vmodel-oom-fix**（2026-05-24，已归档） | 引入 `vi.mock('naive-ui', importOriginal + 6 方法 stub)` mount-level 测试范式（insight L36） | **必须复用**该 mock 模式，避免 mount 时 `useMessage` undefined |
| **T-033 e2e-setup-spec-flake-fix**（2026-05-24，已归档） | 明确"e2e fixture 不用 Vitest mock"对称镜像约定（insight L37） | 本任务的组件单测走 Vitest mount；e2e（如有）不 mock API |

`docs/tasks.md` 中**无**其他 log UI 类历史任务。

---

## 8. Open questions for user（PM 全权决策，逐条就地拍板）

| Q | 问题 | 候选答案 | PM 决策 | 理由 |
|---|---|---|---|---|
| Q-a | 日志等级解析策略 | (1) regex 解析 timestamp + level 前缀，按级别着色 / 分块；(2) 纯文本不解析，无等级着色；(3) 简化：仅靠子串匹配 `ERROR` / `WARN` 关键字粗着色 | **PM 决策**: (1) regex 解析 | frp 标准日志格式稳定（`YYYY-MM-DD HH:MM:SS [LEVEL] ...`），regex 解析失败时降级 PLAIN 不破坏；着色让排障 ERROR 行肉眼可定位；选项 3 误命中风险大（如日志 message 体内含 "ERROR" 一词） |
| Q-b | 搜索是否区分大小写 / 是否支持 regex | (1) 默认大小写不敏感、纯子串、提供 Aa 切换；(2) 默认敏感、纯子串；(3) 支持 regex 切换 | **PM 决策**: (1) | 排障场景关键字大小写记忆模糊（IP / 端口 / 路径 / 错误关键字大小写混杂），不敏感是更友好默认；regex 对普通用户心智负担大且容易写出灾难性回溯正则 |
| Q-c | 缓冲行上限是否从 500 调整 | (1) 保持 500；(2) 提高到 1000；(3) 用户可在 UI 自调 | **PM 决策**: (1) 保持 500 | 500 行覆盖典型排障窗口（最近 ~5 分钟）；提高到 1000 会让"500 行 join+render < 50 ms" NFR-2 边际变紧；用户可调引入 UI / 持久化 / 边界测试成本，本期 OOS |
| Q-d | 主题色板 | (1) 沿用 Naive UI 默认 token（`errorColor` / `warningColor` / `textColor3` / `dividerColor` 等）；(2) 自定义 token 色板（emerald / amber / rose 等） | **PM 决策**: (1) | 长期可维护性优先：主题切换、A11y 对比度、暗色一致性全由 Naive UI 主体接管；自定义 token 增加 ADR 体量、增加 stage 5 颜色审查面 |
| Q-e | 全屏走 Naive UI Modal 还是浏览器 Fullscreen API | (1) `n-modal` `preset="card"` `style="width:95vw;height:90vh"`；(2) `document.documentElement.requestFullscreen()`；(3) 同时支持 | **PM 决策**: (1) `n-modal` | 主题感知 + 暗色一致 + 不与浏览器 ESC 冲突 + 无 Safari iOS 兼容性坑（iOS Safari 不支持 Fullscreen API）+ 包体零增量；用户认知与 frps / frpc 形态、配置编辑等其他 Modal 操作一致 |
| Q-f | 是否需要"导出为文件"按钮 | (1) 加 `导出 .log` 按钮（Blob + URL.createObjectURL）；(2) 不加（用户用复制 + 粘贴到文本编辑器）；(3) 后续作 trivial 任务再加 | **PM 决策**: (2) 本期不加 | 复制覆盖 95% 场景；导出需追加文件名约定、字符集声明（UTF-8 BOM？参考 insight L18 BOM 锁定故事）、移动端浏览器下载行为差异，设计成本高于本期收益；OOS-6 记录；如未来明确需求作 trivial 任务追加 |
| Q-g | 是否需要等级筛选下拉 | (1) 加 `n-select` 多选等级；(2) 不加，仅靠搜索；(3) 加 checkbox 形态 | **PM 决策**: (1) `n-select` 多选 | 等级筛选是日志查看器经典 affordance（kibana / grafana / docker logs UI 通用），覆盖"只看 ERROR" 高频需求；多选 select 比 checkbox 占用工具条空间更省；与 Q-a 解析配套 |
| Q-h | 默认是否开启"跟随尾部"自动滚动 | (1) 默认开（与自动刷新独立）；(2) 默认关，与自动刷新联动；(3) 与自动刷新同步 | **PM 决策**: (1) 默认开 | 跟随尾部与自动刷新是**两个语义维度**：前者控制"渲染到底"，后者控制"是否拉新数据"；用户打开页面期望"看到最新"即视觉到底，跟随默认开符合直觉；用户手动上滚自动暂停（§11）已防止干扰；自动刷新本身保持默认关（沿用 T-001 现状，避免无意识耗后端 polling） |

**所有 Open questions 已就地决策。**

---

## 9. Verdict

**READY**

- 用户已授权 PM 全权决策（INPUT.md L11-L17），不允许 BLOCKED ON USER（PM 派发指示）。
- 所有 8 个 Open questions 已逐条 PM 决策并写入文档。
- 26 条 in-scope 行为、13 条 boundary conditions、17 条 acceptance criteria、9 条 NFR 全部可验证。
- 范围与红线：纯前端、禁 inline style（NFR-4）、不引入重型库（NFR-3 / NFR-5）、主题 token 化（NFR-1 系列）、Vitest 测试范式复用 T-032 mock 模式（Related §T-032）。
- 下游：Solution Architect 在 02 中给出组件拆分（建议拆 LogToolbar / LogList / LogLine / FullscreenLogModal 子组件；最终拆法 SA 拍板）、composable 抽取（建议 `useLogBuffer` / `useLogFilter` / `useFollowTail`）、CSS scoped vs CSS Modules 取舍、localStorage 偏好持久化层封装。

---

_由 Requirement Analyst 写于 2026-05-24，PM 全权授权下就地决策。_
