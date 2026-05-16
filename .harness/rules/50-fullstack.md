---

## 全栈项目专属规则

### 分区 Developer（v0.4+）

partitioned 模式下，这个项目用**分区 Developer agents**：

- `dev-frontend` 负责 UI / 页面 / 组件 / 客户端状态
- `dev-backend` 负责 API 路由 / services / 服务端逻辑
- `dev-db` 负责 schema / migrations / ORM 模型

Solution Architect 必须在每份 `02_SOLUTION_DESIGN.md` 里写 `## Partition assignment`（分区分配）章节。PM Orchestrator 按依赖顺序派分区（通常 dev-db → dev-backend → dev-frontend）。Generic `developer` 作为模糊任务的 fallback 保留。每个分区的 owned paths 和契约见 `.harness/agents/dev-*.md`。

如果用 single Developer 模式初始化，本章节仅供参考，所有代码改动由 generic `developer` agent 处理。

### API 契约
- 每个后端 endpoint 必须有 typed schema（OpenAPI / tRPC / GraphQL）。**不允许无 schema 的路由**。
- 前端绝不重复定义后端的类型；要么从 OpenAPI 生成，要么通过共享 package 导入。
- 破坏性 API 改动 → 在 `docs/features/<task>/MIGRATION.md` 写迁移说明。

### 数据库
- **绝不直接编辑生产 DB**。所有改动通过 migration。
- Migration 必须可回滚。不可回滚的（比如带数据 DROP COLUMN）需要显式用户确认。
- Migration 文件追加为主 — **绝不**修改已合并的 migration。

### 前端
- 布局不用 inline style；用项目的 styling system。
- 组件 > 200 行必须拆分。
- 表单必须**前端验证（UX）+ 后端验证（安全）双重**。

### 跨层
- Loading 状态和 error 状态是"完成"的必要部分，**不是可选**。
- 鉴权路由必须在**前端和后端**都验证。
- 环境变量：写到 `.env.example`，**绝不**提交实际的 `.env`。
