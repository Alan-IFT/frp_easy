# 04 开发 — T-055 backend-api-hygiene

> Stage 4 / Developer（partition: dev-backend）· mode: full · 输出语言：中文

## 1. 实现概述

按 02 设计与 03 conditions 落地两组改动，零新第三方依赖（仅新增标准库 `net/url`），零 schema 变更。5 处生产代码 edit + 3 个测试文件（2 edit + 1 new）+ baseline.json bump。

## 2. 生产代码改动清单

### A. Path 注入防御

1. `internal/frpsadmin/client.go`
   - import 新增 `net/url`。
   - `Proxies`：`"/api/proxy/"+url.PathEscape(proxyType)`。
   - `ProxyDetail`：`"/api/proxy/"+url.PathEscape(proxyType)+"/"+url.PathEscape(name)`。
   - `Traffic`：`"/api/traffic/"+url.PathEscape(name)`。
   - 这是纵深防御的**根位置**：即使上游 handler 漏校验，client 层也安全。

2. `internal/httpapi/handlers_server_runtime.go`
   - 新增包内 helper `isFrpsProxyType(pt string) bool`（线性扫描 `frpsProxyTypes`）。
   - `serverRuntimeProxyDetail`：**消化 C-2**——type 白名单校验**前移到 `buildFrpsAdminClient` 之前**。非白名单 type → 422 `VALIDATION_FAILED` field=`type` 文案"不支持的代理类型"，且不构造 client、不触上游。

### B. 内部错误细节不外泄

3. `internal/httpapi/handlers_proc.go`
   - 新增 helper `(h *handlers) writeInternalError(w, userMsg, cause)`：固定文案进 `writeError`（500 INTERNAL），原始 cause 进 `h.deps.Logger.Error`（nil 守卫，与 proxies.go:147 范式一致，级别用 Error 因 500 是真错误）。
   - `procStop` 500 兜底（旧 `err.Error()`）→ `h.writeInternalError(w, "停止进程失败", err)`。

4. `internal/httpapi/handlers_proxies.go`
   - `mapProxyWriteError` 由自由函数改为 `(h *handlers)` 方法（以复用 `writeInternalError`）；两个调用点（createProxy / updateProxy）改 `h.mapProxyWriteError(w, err)`。
   - 兜底 500（旧 `"保存失败: "+msg`）→ `h.writeInternalError(w, "保存失败", err)`。
   - **保留不动**：L241 `ErrDuplicateName` 409、L248-255 `(type,remote_port)` 冲突 422、L256-259 validation 透传。

5. `internal/httpapi/handlers_system.go`
   - `downloadBin` default 500（旧 `err.Error()`）→ `h.writeInternalError(w, "启动下载失败", err)`。
   - **保留不动**：uploadBin L586 errno 透传（O-3 / B-A.12 / R-7）。

## 3. 测试改动清单

| 文件 | 测试函数 | 覆盖 |
|---|---|---|
| `internal/frpsadmin/client_test.go` | `TestProxyDetail_PathEscape`（表驱动 6 子例） | AC-2/AC-3：正常 path 无回归 + `/`/`?`/`#`/空格/`%` 编码 |
| | `TestTraffic_PathEscape` | AC-2：Traffic name segment escape |
| | `TestProxies_PathEscape` | AC-2：Proxies type segment escape |
| `internal/httpapi/handlers_server_runtime_test.go` | `TestServerRuntimeProxyDetail_BadType_422` | AC-1：非白名单 type→422 + field=type + 未触上游 |
| | `TestServerRuntimeProxyDetail_AllValidTypes` | A-1 回归：7 个白名单 type 全放行 |
| `internal/httpapi/handlers_hygiene_test.go`（new） | `TestWriteInternalError_FixedMessage_NoLeak` | AC-3/B-1/B-3：固定文案 + 不含内部子串 + cause 进日志 |
| | `TestWriteInternalError_NilLogger` | BC-5：nil logger 不 panic |
| | `TestMapProxyWriteError_Fallback_NoLeak` | AC-5/B-2：兜底严格"保存失败"无 SQL 后缀 + 进日志 |
| | `TestMapProxyWriteError_DuplicateName_Preserved` | B-2 保留：409 语义化分支 |
| | `TestMapProxyWriteError_Validation_Preserved` | B-2 保留：422 validation 透传 |

新增 Go 顶层 `Test*` 函数 = **10**（`go test -list '.*' ./...` 口径；表驱动子例不计入顶层计数）。

## 4. baseline.json bump（insight L46）

- `go_tests`: 308 → **318**
- `test_count`: 734 → **744**
- `passing_count`: 734 → **744**
- `frontend_tests`: 426（不变，本任务纯后端）
- notes 追加 T-055 段。

## 5. 测试 seam 决策记录（消化 C-1）

B-1/B-3 的 500 兜底在黑盒 HTTP 测试中不可达（`*procmgr.Manager.Stop` / `*downloader.Manager.Start` 仅返 sentinel，且 handler 已前置 `validProcKind` / sentinel 分类）。按 02 §6 论证，抽 `writeInternalError` 纯 helper 并直测——这是零生产行为变更、零依赖、零扩散的最小可测路径。`mapProxyWriteError` 改方法后可直接构造 `*handlers` + `httptest.NewRecorder()` 调用，覆盖兜底 + 保留分支。

## 6. 消化 conditions 一览

- **C-1**（B 必须有测试，数只升不降）→ 落地：5 个 B 测试 + baseline bump 318/744。
- **C-2**（type 校验前移）→ 落地：校验在 `buildFrpsAdminClient` 之前，AC-1 断言 `upstreamHit==false` 更稳。
- **C-3**（06 裸 `## Adversarial tests`）→ 交 stage 6。
- **C-4**（07 裸 `## Insight`）→ 交 stage 7。

## 7. `url.PathEscape` 行为锁定（消化 Q1/R-2）

实测语义（Go stdlib）：`PathEscape("tcp")="tcp"`（unreserved 不编码，无回归）；`PathEscape("a/b")="a%2Fb"`；`PathEscape("x y#z")="x%20y%23z"`；`PathEscape("a%2f")="a%252f"`（已编码字符的 `%` 被再编码，幂等安全）。AC-2 表驱动逐一断言上游 `r.URL.EscapedPath()`。

## 8. verify_all 自验

dev 上下文无 Bash 工具，无法自跑 `go build` / `go test` / `scripts/verify_all`。代码改动已逐处复核编译正确性（import、方法签名、调用点一致性）。**orchestrator 独立真跑 `bash scripts/verify_all.sh`（全量含 e2e）作硬闸门**。

## 9. Design drift

无。完全按 02 设计落地；C-2 的"前移"是 03 预答的优化建议，已采纳并在 §2.2 标注。

## 10. 状态

**READY FOR REVIEW**（dev-backend partition 完成）
