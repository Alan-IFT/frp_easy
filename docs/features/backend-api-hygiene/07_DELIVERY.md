# Delivery Summary — T-055 backend-api-hygiene

- **Task**: `backend-api-hygiene` — 堵 frps 运行态代理端点 path 注入缺口 + 停止 3 处 500 兜底向前端透传内部错误细节。
- **Mode**: full（1→2→3→4→5→6→7）
- **Partition**: dev-backend（单分区，纯后端 Go）
- **Batch**: project-optimization-2026-05（commit / archive-task 由 batch orchestrator 负责）

## Stages traversed

| Stage | 角色 | 产出 | Verdict |
|---|---|---|---|
| 1 | Requirement Analyst | `01_REQUIREMENT_ANALYSIS.md` | READY |
| 2 | Solution Architect | `02_SOLUTION_DESIGN.md` | READY |
| 3 | Gate Reviewer | `03_GATE_REVIEW.md` | APPROVED WITH CONDITIONS（C-1~C-4） |
| 4 | Developer (dev-backend) | `04_DEVELOPMENT.md` | READY FOR REVIEW |
| 5 | Code Reviewer | `05_CODE_REVIEW.md` | APPROVED（0 CRITICAL / 0 MAJOR） |
| 6 | QA Tester | `06_TEST_REPORT.md` | APPROVED FOR DELIVERY（pending orchestrator verify_all） |
| 7 | PM | 本文件 | DELIVERED |

## Rollbacks

**0 次**。全流程一次过；Gate 的 C-1（B 必须测）/ C-2（type 校验前移）/ C-3（06 裸标题）/ C-4（07 裸标题）均在 development 期主动消化（insight L17 范式），stage 5 处于"几乎无需改"状态。stage 5 提出的 `r.URL.EscapedPath()` 不确定性由 PM 决策改用 `r.RequestURI`（非 rollback，是测试断言加固）。

## Files changed

生产代码（5 文件）：
- `internal/frpsadmin/client.go` — import `net/url`；`Proxies`/`ProxyDetail`/`Traffic` 三处 path 拼接加 `url.PathEscape`（A-2 纵深防御根位置）。
- `internal/httpapi/handlers_server_runtime.go` — 新增 `isFrpsProxyType`；`serverRuntimeProxyDetail` type 白名单校验前移到构造 client 之前（A-1 + C-2）。
- `internal/httpapi/handlers_proc.go` — 新增 `(h *handlers) writeInternalError` helper；`procStop` 500 兜底改固定文案"停止进程失败"（B-1）。
- `internal/httpapi/handlers_proxies.go` — `mapProxyWriteError` 改 `*handlers` 方法 + 2 调用点；兜底改固定文案"保存失败"（B-2）；保留 ErrDuplicateName 409 / unique-constraint 422 / validation 422 透传。
- `internal/httpapi/handlers_system.go` — `downloadBin` default 500 改固定文案"启动下载失败"（B-3）；保留 uploadBin errno 透传（O-3）。

测试（2 edit + 1 new）：
- `internal/frpsadmin/client_test.go` — +3 顶层 Test（escape 防注入，`r.RequestURI` 断言）。
- `internal/httpapi/handlers_server_runtime_test.go` — +2 顶层 Test（type 白名单 422 + 未触上游 / 7 type 全放行）。
- `internal/httpapi/handlers_hygiene_test.go`（new）— +5 顶层 Test（writeInternalError / mapProxyWriteError 兜底 + 保留分支）。

baseline：
- `scripts/baseline.json` — `go_tests` 308→318、`test_count` 734→744、`passing_count` 734→744（+10 顶层 Test*；insight L46）。

## Final verify_all result

**DEFERRED 到 orchestrator 权威执行**。本任务全程在 PM/role-collapse 上下文产出（insight L14），无 Bash/PowerShell，未自跑也未伪造 verify_all（insight L46 红线）。orchestrator 独立真跑 `bash scripts/verify_all.sh`（全量含 e2e，因可能影响后端契约）作硬闸门。预期：Go tests 308→318 全 PASS、Fail=0、E.6 因 06 裸 `## Adversarial tests` 标题保持 PASS。

## Baseline changes

- go_tests +10（308→318），test_count +10（734→744），frontend_tests 不变（426，纯后端任务）。

## Outstanding risks

- R-A：escape 测试预期串依赖 `url.PathEscape` 的 `encodePathSegment` 行为（`/`→`%2F`、`?`→`%3F`、`#`→`%23`、空格→`%20`、`%`→`%25`、`=`/`.` 不编码）；本机 Go 版本若有差异由 verify_all 暴露。已用 `r.RequestURI`（请求行原文）规避 `EscapedPath()` 规范化不确定性。
- 无 schema / 无数据迁移 / 无前端契约变更（仅 3 处 500 兜底 message 文案 + 1 个新 422 分支）。

## Backlog（本任务有意不做）

- O-2：`(type,remote_port)` 冲突 422 分支（handlers_proxies.go:248-255）的 sentinel 化——记为 backlog，避免 scope drift。

## Next steps for user

- orchestrator 真跑 verify_all 确认全绿后，本任务并入 batch project-optimization-2026-05 一并 commit + archive-task。

## Insight

- 2026-05-30 · frps 运行态代理 handler 的 `{type}`/`{name}` path 参数注入防御正确分层是"client 层 `url.PathEscape` 作根防御（即使 handler 漏校验也安全）+ handler 层白名单校验前移到构造 client 之前（非法输入零成本早返、不触上游）"双层；测试断言上游收到的 path 必须用 `r.RequestURI`（请求行原文逐字节保留）而非 `r.URL.EscapedPath()`（后者在 RawPath 与 Path 默认转义不同如含 `%2F` 时虽返 RawPath 但语义上是"可被规范化重算"的，不如 RequestURI 确定）· evidence: T-055 frpsadmin/client.go Proxies/ProxyDetail/Traffic + client_test.go::TestProxyDetail_PathEscape 表驱动 6 子例 + handlers_server_runtime.go::serverRuntimeProxyDetail 校验前移
- 2026-05-30 · 后端 500 兜底"固定面向用户文案 + 原始 error 进 logger"的统一 helper（`writeInternalError`）解决了一个隐性可测性问题：当 500 兜底由具体类型（`*procmgr.Manager` / `*downloader.Manager`，其方法仅返 sentinel 且 handler 已前置校验）守护时，该分支在黑盒 HTTP 测试中事实上不可达——抽纯 helper 直测（`httptest.NewRecorder` + 捕获型 `slog` buffer 断言"响应不含 leak 子串 + cause 进日志"）是零生产行为变更、零依赖、零扩散的最小可测路径，优于为测试引入 mock 接口（会扩散到 5+ 调用点）· evidence: T-055 handlers_proc.go::writeInternalError + handlers_hygiene_test.go::TestWriteInternalError_FixedMessage_NoLeak / TestMapProxyWriteError_Fallback_NoLeak
