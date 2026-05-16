# 开发导航 — frp_easy

> 项目结构和约定导航。**新增 / 移动 / 删除模块时，请同步更新这里**。
>
> Developer agent 写代码前会读这个文件，避免重复造已经存在的轮子。

## 目录布局

（随项目增长持续更新。）

```
frp_easy/
├── (项目根目录)
├── .claude/        ← AI 配置（不要把 secret 提交到这里）
├── docs/           ← SPEC、feature 文档、本导航、任务看板
├── scripts/        ← verify_all、baseline、sync 辅助
└── (源码目录在这里)
```

## 功能在哪里

| 功能区域 | 文件 | 约定 |
|---|---|---|
| (空 — 边做边填) | | |

## 可复用工具

| 需求 | 已有 | 文件 | 备注 |
|---|---|---|---|
| (空 — 边做边填) | | | |

## 要遵循的模式

（边发现边补充示例：）

- "所有 HTTP 错误用 `src/errors.ts` 的 `AppError` class。"
- "DB migration 放 `migrations/`，通过 `scripts/migrate.sh` 应用。"
- "`src/X.ts` 的测试在 `tests/X.test.ts`。"

## 要避免的模式

（边发现边补充示例：）

- "不要从 client 代码 import server 代码。"
- "不要在路由 handler 里写原始 SQL；用 `OrdersRepo` class。"
