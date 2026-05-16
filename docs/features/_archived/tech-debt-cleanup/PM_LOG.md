# PM 日志 — T-004 tech-debt-cleanup

**任务 ID**：T-004  
**Slug**：tech-debt-cleanup  
**创建**：2026-05-16  
**模式**：full（7-stage）

## 范围决策（PM 制定）

实施以下 7 项，跳过 TD-8 / OPT-9：

| # | 项目 | 分区 |
|---|---|---|
| OPT-1 | verify_all 前端路径修复 | scripts |
| OPT-2 | 向导路由守卫补全 | frontend |
| OPT-4 | slog 双写（tee 到 stderr） | backend |
| OPT-5 | 版本号从 git describe 注入 | scripts |
| OPT-6 | ParseIPFromJSON 统一 | backend |
| OPT-7 | 添加 /api/v1/health 端点 | backend |
| OPT-8 | auto-restore TOML 预检 | backend |

**跳过**：TD-8（SQLite 单连接是 SQLite 正确设计），OPT-9（OpenAPI 范围过大）

## 阶段记录

| 阶段 | 状态 | 时间 |
|---|---|---|
| 01_REQUIREMENT_ANALYSIS | 完成 | 2026-05-16 |
| 02_SOLUTION_DESIGN | 完成 | 2026-05-16 |
| 03_GATE_REVIEW | 完成 | 2026-05-16 |
| 04_DEVELOPMENT | 进行中 | 2026-05-16 |
