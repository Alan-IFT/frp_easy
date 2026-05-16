# 任务看板 — frp_easy

> 由 **PM Orchestrator** 维护。每个任务开始时记录到这里，并随生命周期更新。
>
> 新任务开始前，应该查这个看板找相关历史工作。

## 进行中

| ID | Slug | 阶段 | mode | 开始 | 文档目录 |
|---|---|---|---|---|---|
| T-001 | web-ui-mvp | gate | full | 2026-05-16 | `docs/features/web-ui-mvp/` |

## 已完成

| ID | Slug | 结果 | 完成 | 文档目录 |
|---|---|---|---|---|
| _(暂无)_ | | | | |

## 约定

- **ID** 顺序编号：`T-001`、`T-002`、...
- **Slug** 小写连字符，≤40 字符（例：`csv-export-orders`）。
- **阶段** 之一：`req`（需求）、`design`（方案）、`gate`（闸门）、`dev`（开发）、`review`（评审）、`test`（测试）、`delivery`（交付）、`blocked`（阻塞）、`done`（完成）。
- **文档目录** 是 `docs/features/<slug>/` 下的相对路径。

## 任务怎么关联

新任务开始时，Requirement Analyst 会扫描这个看板找相关历史：

- 同一模块 → 先读之前的 `02_SOLUTION_DESIGN.md`。
- 同一 feature → 在之前的方案基础上扩展，**不要重新设计**。
- 决策冲突 → 标记给用户。
