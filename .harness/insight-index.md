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
- 2026-05-17 · Naive UI 凡使用 useMessage/useDialog 等 composable 的组件，App.vue 根组件必须包裹对应 Provider；缺失时 headless 浏览器中 setup() 抛异常，组件输出空节点 `<!-->`，表单不可见 · evidence: e2e-smoke-tests
- 2026-05-17 · go:embed 将 dist/ 静态快照嵌入二进制，前端重建后必须重新 go build；E2E 启动脚本用 find dist/ -newer $BIN 时间戳检查驱动重建，是最轻量的解决方案 · evidence: e2e-smoke-tests
- **2026-05-19** · vitest module resolution 在 .ts/.js 共存时优先加载 .js；historical `tsc` 残留的 .js/.d.ts 会让改 .ts 测试看似无效果且无报错。开发前清理 `find web/src -type f \( -name '*.js' -o -name '*.js.map' \) -delete` · evidence: hardening-pass-audit
- **2026-05-19** · modernc.org/sqlite 的 UNIQUE 约束错误文本格式为 `UNIQUE constraint failed: <table>.<column>`，区分大小写；用 strings.Contains 双关键字（"UNIQUE constraint failed" + "<table>.<column>"）能精确区分表内多个 UNIQUE 列的冲突 · evidence: hardening-pass-audit
- **2026-05-19** · Go AtomicWrite 双重 Chmod 模式（tmp + final）必须在 rename 前后两处都 chmod，仅 chmod tmp 时 rename 后 umask 可能让最终文件权限变宽 · evidence: hardening-pass-audit
