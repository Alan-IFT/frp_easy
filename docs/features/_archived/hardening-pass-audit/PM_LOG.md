# PM_LOG — T-007 hardening-pass-audit

## 任务定义

由 PM 在 2026-05-19 创建。用户通过 `/harness-kit:harness` 发起综合审计后修复任务。

**决策原则**（用户给定）：用户体验好、符合软件工程标准、长期易使用易维护。PM 自主决策，用户只看结果。

## 审计输入

启动前并行运行三个 Explore agent 做：安全审计、代码质量审计、UX 审计。详细发现见三份 agent 输出（已在 PM 主上下文中综合）。

## 本任务取舍（PM 决策）

**纳入**（高价值、低风险、聚焦）：

后端安全：
1. `internal/frpconf/render.go` 临时文件 → 0o600 权限（P1）
2. `cmd/frp-easy/main.go` 日志文件 0o644 → 0o600（P3）
3. 新增 SecurityHeaders 中间件：X-Content-Type-Options/X-Frame-Options/Referrer-Policy（P3）
4. `internal/httpapi/handlers_logs.go` 日志增量读取大小上限（P2 DoS）

后端质量：
5. `internal/procmgr/manager.go` `Start()` defer-unlock 重构（H 维护性）
6. `internal/storage/proxies.go` UNIQUE 冲突 sentinel `ErrDuplicateName`（M 用户反馈）

前端 UX：
7. `Dashboard.vue` 进程错误信息完整可见（H）
8. `Proxies.vue` 删除后清理 firewallPorts、添加空状态占位（H/L）
9. `ProxyForm.vue` 类型切换清理无关字段（M）

**延后**（独立任务理由）：
- Version int64→string（API 契约变更，独立任务）
- 文案集中化 i18n（大型重构，单独 ROI 评估）
- 日志虚拟滚动（前端 feat，独立设计）
- Wizard 步骤进度反馈（产品决策，独立设计）
- frpc admin 凭据加密存储（深度安全工程，单独评估）
- 自动 Secure cookie 检测（部署模式相关，独立 ADR）

## 阶段流转

- 2026-05-19 创建 → req
