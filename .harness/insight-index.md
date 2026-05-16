# Insight Index — frp_easy

> 项目踩坑学到的跨任务真相。≤30 行。
> 设计/实现任务开始时读；只在证据支持的意外之后写。
> 规则见 `.harness/rules/05-insight-index.md`。

<!-- 追加新 insight 写下面，一行一条。格式：
- YYYY-MM-DD · <一句话事实> · evidence: <任务 slug 或 commit sha>
-->
- 2026-05-16 · Windows os.Rename 不能覆盖已存在文件，需先 Remove 再 Rename；但 Remove 成功后 Rename 失败会丢失原文件，正确模式是先试 Rename 失败再 Remove+Rename · evidence: zero-config-quickstart
- 2026-05-16 · 向导页面必须是顶层路由（非 AppLayout 子路由），否则侧边栏导航干扰向导流程 · evidence: zero-config-quickstart
- 2026-05-16 · openapi.yaml 字段名应以 Go 常量为权威（直接读 .go），不以设计文档草稿为准；status 枚举值在设计阶段写错（done/error vs success/failed），Gate Review 捕获 · evidence: docs-and-api-schema
