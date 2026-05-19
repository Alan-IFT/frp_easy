# 任务看板 — frp_easy

> 由 **PM Orchestrator** 维护。每个任务开始时记录到这里，并随生命周期更新。
>
> 新任务开始前，应该查这个看板找相关历史工作。

## 进行中

（无）

## 已完成

| ID | Slug | 结果 | 完成 | 文档目录 |
|---|---|---|---|---|
| T-010 | deploy-polish-and-ci | DELIVERED | 2026-05-19 | `docs/features/_archived/deploy-polish-and-ci/` |
| T-009 | polish-pass | DELIVERED | 2026-05-19 | `docs/features/_archived/polish-pass/` |
| T-008 | deploy-kit | DELIVERED | 2026-05-19 | `docs/features/_archived/deploy-kit/` |
| T-007 | hardening-pass-audit | DELIVERED | 2026-05-19 | `docs/features/_archived/hardening-pass-audit/` |
| T-006 | e2e-smoke-tests | DELIVERED | 2026-05-17 | `docs/features/_archived/e2e-smoke-tests/` |
| T-005 | docs-and-api-schema | DELIVERED | 2026-05-16 | `docs/features/_archived/docs-and-api-schema/` |
| T-004 | tech-debt-cleanup | DELIVERED | 2026-05-16 | `docs/features/_archived/tech-debt-cleanup/` |
| T-003 | readme-and-health-report | DELIVERED | 2026-05-16 | `docs/features/_archived/readme-and-health-report/` |
| T-002 | zero-config-quickstart | DELIVERED | 2026-05-16 | `docs/features/_archived/zero-config-quickstart/` |
| T-001 | web-ui-mvp | DELIVERED | 2026-05-16 | `docs/features/_archived/web-ui-mvp/` |

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
