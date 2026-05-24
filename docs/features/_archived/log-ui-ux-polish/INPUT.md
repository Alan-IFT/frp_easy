# INPUT — T-036 / log-ui-ux-polish

## 用户原始请求

> 优化 frpc 和 frps 日志的 UI 样式和用户使用体验。

## 模式

`full`（完整 7-stage pipeline）

## 用户授权

- "以用户体验好，符合软件工程标准，长期易使用易维护为原则来决策。"
- "你来决策就可以了，我只看结果是否符合需求。"
- "所有 commit 和 push 都由你来操作。"

PM 在 7-stage 流水线内对一切非红线决策（颜色方案、可选功能取舍、组件 API、依赖引入与否、ADR 类技术选型等）拥有终审权——无需就候选答案回头问用户。每个候选答案由 PM 直接拍板并记入 01_REQUIREMENT_ANALYSIS.md 的 "Open questions for user" 段；该段最终为空或全部就地拍板，**不允许 BLOCKED ON USER 阻塞**。

## 关键上下文

- **当前实现**：`web/src/components/LogViewer.vue`（~94 行）+ `web/src/pages/Logs.vue` 包裹。仅一个 `n-code` 块、硬编码深色背景、`#1a1a1a`、12px monospace、500 px 高度、`max-height + overflow-y: auto`、`white-space: pre-wrap`、`word-break: break-all`。
- **当前 UX**：1) "自动刷新" 开关默认关闭；2) "刷新" 按钮重读 500 行 tail；3) `lines.value` slice(-500) 内存上限；4) 用户切 kind 时 stopPolling + autoRefresh = false + loadTail。
- **后端 API**（不在改造范围）：`GET /api/v1/logs/{kind}?lines=500` (tail) / `?offset=N` (incremental)，返回 `{ lines: string[] }` 或 `{ data: string, nextOffset: number }`。
- **数据规模**：单条日志典型 ~100-200 字节；500 行 ≈ 50-100 KB；frps 高负载 1s 数百行不属一般家用场景。
- **技术栈约束**：Vue 3 SFC + Naive UI v2 + Pinia + Vite + Vitest + Playwright；本项目 frontend 用 Naive UI 是硬基线（见 dev-map）。
- **构建约束**：单二进制嵌入 web/dist，包体增加 < 50 KB 为软约束（重型日志库如 xterm.js 不宜引入）。
