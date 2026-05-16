# 05 — 跨任务 Insight Index

## 这是什么

`.harness/insight-index.md` 是一个**≤30 行 append-only 文件**，记录项目踩坑学到的真相 —— 那些如果不记下来，每次新任务都会被重新发现一遍的事实。

好的 insight 例子：
- "遗留的 `users.created_at` 字段是 `DATETIME` 不是 `TIMESTAMPTZ` —— 朴素的 UTC 转换会在 CI 里静默地把日期偏 8 小时。"
- "WSL 下 build 如果同时打开超过 4 个 watcher 会报 `ENOSPC`；dev 模式用 `chokidar.useFsEvents: false`。"
- "Vendor SDK v2.7.1 对非法 key 返回 `null` 而不是抛异常 —— 每次调用都要显式 null 检查。"

这些**不是规则**（没有"必须如何"），**不是 bug 报告**（那在任务里），而是**项目特有的、来之不易的事实**，不能被忘、不该被重推导。

## 什么时候读

**任何涉及设计或实现决策的任务开始时。** 先扫这个文件再读其他规则片段 —— 如果有命中条目，就少一个错误假设。

跳过：纯 typo 修正、注释整理、单纯的依赖版本升级。

## 什么时候写

完成任务时，如果工作过程中暴露了非显然的真相，下一个人（或 AI）也会踩，追加一行：

```markdown
- YYYY-MM-DD · <一句话事实> · evidence: <任务 slug 或 commit sha>
```

**写入规则**：
- 总长 ≤30 行。要加新行而文件满了，**先归档最老的**（移到 `docs/features/_archived/insight-history.md`）再追加。
- 一行一个事实。需要段落的就不够清晰。
- 必须带 evidence（任务 slug 或 commit SHA），未来读的人能追溯。
- **对抗性检验**：写之前问"一个有经验的人，新看代码库，能在 10 分钟内推导出这个吗？"如果能，**别写** —— 那不是 insight，是普通文档。

## 什么时候**不**写

- Bug 报告（那在任务里）
- 规则/约定（那在 `00-core.md` 或新规则片段里）
- "最佳实践"主张（代码或 `.harness/rules/` 是它的位置）
- 任务总结（那在 `docs/features/<task-slug>/` 里）

Insight 是**用证据击败某人先验的发现**，不是"我们决定 X"，也不是"X 已记录"。

## 归档

PM Orchestrator 在每个任务结束时跑 `scripts/archive-task`，它会：
1. 如果该任务的 07_DELIVERY.md 有 `## Insight` 段，把行追加到 `.harness/insight-index.md`。
2. 把任务的 7 个阶段文档移到 `docs/features/_archived/<task-slug>/summary.md`（压成一个文件）+ 旁边保留原始 7 个文件。
3. 如果 `.harness/insight-index.md` 超过 30 行，把最老的轮转到 `docs/features/_archived/insight-history.md`。

归档脚本**永远不删** —— 只移动和压缩。原任务文档随时能从 `_archived/` 找回。
