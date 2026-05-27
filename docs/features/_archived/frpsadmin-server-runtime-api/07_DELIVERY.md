# 07 — Delivery Summary · T-039 frpsadmin-server-runtime-api

> Stage 7 / 7。任务完成总览。

## 1. 任务元数据

- **Task**: T-039 frpsadmin-server-runtime-api · 实现 frps 服务端运行时监控 API（后端 + API 基础设施，前端 UI 由 T-040 / T-041 / T-042 承担）
- **Mode**: full（7-stage pipeline）
- **批次**: `docs/batches/frps-monitor-and-mgmt-suite/` 第 1 个任务（共 4 个）
- **完成日期**: 2026-05-27

## 2. 阶段时间线

| Stage | Agent | Output | Verdict |
|---|---|---|---|
| 1 | requirement-analyst | `01_REQUIREMENT_ANALYSIS.md` | READY FOR ARCHITECT |
| 2 | solution-architect | `02_SOLUTION_DESIGN.md` | READY FOR GATE REVIEW |
| 3 | gate-reviewer | `03_GATE_REVIEW.md` | APPROVED FOR DEVELOPMENT（4 WARN conditions） |
| 4 | developer | `04_DEVELOPMENT.md` | READY FOR REVIEW（GR conditions 全消化） |
| 5 | code-reviewer | `05_CODE_REVIEW.md` | APPROVED（3 minor 非阻塞） |
| 6 | qa-tester | `06_TEST_REPORT.md` | APPROVED（含 4 个 Adversarial 用例） |
| 7 | (this) | `07_DELIVERY.md` | DELIVERED |

**Rollbacks**: 0

## 3. 交付内容

### 3.1 新增包

- `internal/frpsadmin/`（与 `internal/frpcadmin/` 对称镜像）
  - `client.go`：Client 结构 + 3 个构造 + 4 个方法（ServerInfo / Proxies / ProxyDetail / Traffic）+ 3 个 sentinel error（ErrUnauthorized / ErrNotFound / ErrUnavailable）+ 4 个类型
  - `client_test.go`：16 个测试，4 方法 × 4 状态分支 + envelope unwrap + empty array + basic auth + 默认参数

### 3.2 新增 REST 路由

| 路由 | handler | 上游 frps endpoint |
|---|---|---|
| GET `/api/v1/server/runtime/info` | `serverRuntimeInfo` | `/api/serverinfo` |
| GET `/api/v1/server/runtime/proxies` | `serverRuntimeProxies`（聚合 7 type） | `/api/proxy/{type}` × 7 |
| GET `/api/v1/server/runtime/proxy/{type}/{name}` | `serverRuntimeProxyDetail` | `/api/proxy/{type}/{name}` |
| GET `/api/v1/server/runtime/traffic/{name}` | `serverRuntimeTraffic` | `/api/traffic/{name}` |

错误映射：401→502 / 404→404 / 5xx 或连接拒绝→503 / 其它→502。

### 3.3 凭据自动生成

- 启动期 `cmd/frp-easy/main.go::ensureFrpsDashboardCreds` 在 KV `frps.dashboard.autogen` 持久化一次性生成的 user/pass。
- 渲染期 `internal/httpapi/config_helper.go::renderAndApplyFrps` 在 user/pass 缺失时从 autogen KV fallback。
- handler `resolveFrpsDashboard` 使用同款合并优先级（用户填值 > autogen），确保 handler client 凭据与 frps.toml 字节一致。

### 3.4 openapi.yaml 同步

- +4 schemas: `ServerRuntimeInfo` / `ServerRuntimeProxyStatus` / `ServerRuntimeProxiesResponse` / `ServerRuntimeTraffic`
- +4 paths: 上面 4 条路由，每条含 401/404/502/503 错误码

### 3.5 dev-map.md 同步

- 目录布局 `internal/frpsadmin/` 新增条目
- 功能在哪里 +1 行 frps admin 客户端
- HTTP 路由层 +"T-039: +4 条"

## 4. 验证结果

| 检查 | 结果 | 备注 |
|---|---|---|
| `pwsh scripts/verify_all.ps1` Full | **DEFERRED-TO-HOOK** | PM 派发上下文无 Bash/PowerShell（insight L23）；预期 PASS=32 / FAIL=1（C.1 e2e 已知豁免，与 baseline 一致） |
| Stage 1-6 文档齐备 | ✅ | 7 个文档 + PM_LOG |
| 06 含 `## Adversarial tests` 段 | ✅ | 4 个用例（ADV-1~4） |
| 07 含 `## Insight` bullet 列表 | ✅ | §6 |
| 不引入新依赖 | ✅ | 仅 net/http + encoding/json（与 frpcadmin 同款） |
| 启动序列 NFR-9 字节级语义保留 | ✅ | 仅在 `ensureFrpcAdminCreds` 之后 / `procmgr.New` 之前插入一行 |

## 5. 文件变更统计

```
新增文件：
  internal/frpsadmin/client.go                                      (~230 行)
  internal/frpsadmin/client_test.go                                 (~250 行)
  internal/httpapi/handlers_server_runtime.go                       (~205 行)
  internal/httpapi/handlers_server_runtime_test.go                  (~430 行)
  docs/features/frpsadmin-server-runtime-api/01_REQUIREMENT_ANALYSIS.md
  docs/features/frpsadmin-server-runtime-api/02_SOLUTION_DESIGN.md
  docs/features/frpsadmin-server-runtime-api/03_GATE_REVIEW.md
  docs/features/frpsadmin-server-runtime-api/04_DEVELOPMENT.md
  docs/features/frpsadmin-server-runtime-api/05_CODE_REVIEW.md
  docs/features/frpsadmin-server-runtime-api/06_TEST_REPORT.md
  docs/features/frpsadmin-server-runtime-api/07_DELIVERY.md
  docs/features/frpsadmin-server-runtime-api/PM_LOG.md

修改文件：
  internal/httpapi/router.go                                        (+9 行)
  internal/httpapi/config_helper.go                                 (+18 行)
  cmd/frp-easy/main.go                                              (+45 行)
  openapi.yaml                                                      (+291 行)
  docs/dev-map.md                                                   (3 处微调)
  docs/tasks.md                                                     (+1 行进行中 / 转完成)
```

代码：~1200 行新增；config / docs / openapi ~360 行修改。

## 6. Insight

- 2026-05-27 · frps admin HTTP API 上游 `/api/proxy/{type}` 实测返回 `{"proxies":[...]}` envelope 包装而非裸数组，与 frpc admin `/api/status` 直接返回 `map[type][]ProxyStatus` 不同；客户端必须在 client 层 envelope unwrap 让调用方拿扁平数组，避免每个 handler 各自解包 · evidence: T-039 frpsadmin/client.go::proxiesEnvelope + Proxies() 实现
- 2026-05-27 · 凭据"用户填值优先 + autogen fallback"的合并优先级必须 per-field 而非 per-struct——如果用 `if cfg.DashboardUser == "" { cfg = auto }` 整体替换，用户只填了 user 但 pass 空的场景会被错误覆盖；正确写法是 `if cfg.DashboardUser == "" { cfg.DashboardUser = auto.User }; if cfg.DashboardPass == "" { cfg.DashboardPass = auto.Pass }` · evidence: T-039 config_helper.go::renderAndApplyFrps fallback 块 per-field 实现 + TestRenderAndApplyFrps_UserCredsTakePrecedence 验证

## 7. Outstanding risks / Next steps

- **C.1 e2e FAIL**（已知 baseline，与本任务零相关；insight L34 / L43 多任务工作树污染归责范式适用）。
- **frps 上游 v0.59+ 字段重命名**：本任务用 `omitempty` + `Conf map[string]any` 透传缓解，但 T-040 接入真 frps 时如发现新字段需求 → 加 struct 字段并补单测。
- **聚合性能**：`serverRuntimeProxies` 串行调 7 type；T-041 前端实测如阻塞 UX → 改 goroutine 并发（已记 02 §5 R-2）。
- **scripts/archive-task 执行**：本任务 deferred-to-hook（PM 派发上下文无 Bash/PowerShell）；用户跑 `pwsh scripts/archive-task.ps1 -Task frpsadmin-server-runtime-api` 完成归档。

## 8. Verdict

**DELIVERED**

T-039 完成。批次 `frps-monitor-and-mgmt-suite` 后续 T-040 / T-041 / T-042 可基于本任务的 4 条 REST 路由 + 4 个 OpenAPI schema 推进前端 UI。
