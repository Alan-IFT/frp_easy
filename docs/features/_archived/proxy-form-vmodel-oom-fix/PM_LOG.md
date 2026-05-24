# PM_LOG — T-032 proxy-form-vmodel-oom-fix

## 任务摘要
用户报告：WebUI「新增代理规则」时卡住，最后报错"Out of Memory"。
要求：基于官方 Vue 文档（context7 已查证）修复，以用户体验、软件工程标准、长期易维护为决策原则。

## PM 初始定位（移交给 RA 验证）
代码静态阅读发现：`web/src/components/ProxyForm.vue` 与父组件 `web/src/pages/Proxies.vue` 之间存在双向 `watch` + `emit` 反馈环路：

1. 子组件：`watch(form, () => emit('update:modelValue', toProxyInput()), { deep: true })`
2. 子组件：`watch(() => props.modelValue, (val) => syncFromInput(val), { deep: true })`
3. `toProxyInput()` 每次返回新对象（无 customDomains 键，因为 tcp 分支）
4. `syncFromInput(val)` 把 `val.customDomains ?? []` 写回 form —— **永远产生新 `[]` 引用**，触发 deep watch
5. → 互相触发，无限循环 → Out of Memory

context7 官方文档关键证据：
- Vue 3.4+ 推荐 `defineModel` 宏，无需手写 watch/emit 桥
- `watch` 在 deep 模式下对 reactive 对象的任何嵌套 mutation 触发，包括新数组引用赋值

## 决策原则（用户委托）
- 用户体验好 → 修复 OOM、表单交互流畅
- 软件工程标准 → 遵循 Vue 3.4+ 官方推荐的 `defineModel` 模式 或 单一数据源
- 长期易使用易维护 → 优先消除"双向桥"反模式（架构清理 vs 补丁）

## 阶段进度
- [x] Stage 1: Requirement Analyst — verdict READY (2026-05-24)
- [x] Stage 2: Solution Architect — verdict READY (推荐方案 B 单向数据流)
- [x] Stage 3: Gate Reviewer — verdict APPROVED WITH CONDITIONS (P0=0, P1=3, P2=2)
- [ ] Stage 4: Developer (dev-frontend) — 准备派发
- [ ] Stage 5: Code Reviewer
- [ ] Stage 6: QA Tester
- [ ] Stage 7: Delivery

## Stage 3 备注 — reviewer 落盘陷阱第 5 次复现
T-030 已为 gate-reviewer.md frontmatter 加 `tools: Read, Write, Glob, Grep` 但本会话 gate-reviewer 仍把内容塞消息体让 PM 代为落盘。PM 已替 reviewer 完成 03 落盘并在文件首加 PM 说明。Insight 候选收割到 07_DELIVERY.md。

## 相关 insight（已查 `.harness/insight-index.md`）
- L29 ↔ 本任务：前端 TS 与后端 Go 字段名漂移 → 不直接相关
- L41 / L44 / L48 / L50：**reviewer 不落盘** —— 派发时显式要求 reviewer 自落盘（T-030 已把 Write 工具加进 frontmatter，本任务应可正常落盘）
- L43 / L46 / L49 / L51：**07 标题禁数字前缀** —— PM 写 07 时必须裸标题 `## Insight`
- L19：**verify_all E.6 标题禁数字前缀** —— QA 06 标题必须裸 `## Adversarial tests`
