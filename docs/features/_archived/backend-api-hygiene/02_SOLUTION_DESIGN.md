# 02 方案设计 — T-055 backend-api-hygiene

> Stage 2 / Solution Architect · mode: full · 输出语言：中文

## 1. 架构摘要

两项内聚的后端卫生改动，零新依赖、零 schema 变更、零 API 形状变更（仅错误 message 文案 + 一个新增 422 验证分支）。A 在 `frpsadmin.Client` 的 path 拼接处加 `url.PathEscape`（纵深防御根位置）+ handler 层 `serverRuntimeProxyDetail` 加 type 白名单前置校验；B 把 3 处 500 兜底分支的 `err.Error()` 透传改为固定中文文案 + 原始 error 进 `h.deps.Logger`。

## 2. 受影响模块

| 文件 | 改动性质 |
|---|---|
| `internal/frpsadmin/client.go` | edit：import `net/url`；`Proxies`/`ProxyDetail`/`Traffic` 三处 path 拼接用 `url.PathEscape` |
| `internal/httpapi/handlers_server_runtime.go` | edit：`serverRuntimeProxyDetail` 加 type 白名单校验（A-1） |
| `internal/httpapi/handlers_proc.go` | edit：`procStop` 500 兜底改固定文案 + logger（B-1） |
| `internal/httpapi/handlers_proxies.go` | edit：`mapProxyWriteError` 兜底 500 改固定文案 + logger（B-2） |
| `internal/httpapi/handlers_system.go` | edit：`downloadBin` default 500 改固定文案 + logger（B-3） |
| `internal/frpsadmin/client_test.go` | edit：补 escape 验证测试（AC-2） |
| `internal/httpapi/handlers_server_runtime_test.go` | edit：补 type 白名单 422 测试（AC-1） |
| `internal/httpapi/handlers_proc.go`（新建对应 test） | new：`handlers_hygiene_test.go` 承载 B-1/B-2/B-3 单测 |
| `scripts/baseline.json` | edit：bump `go_tests` + `test_count` |

## 3. 模块分解（无新模块）

无新模块。一个新的**包内私有 helper** 用于 B 的可测性（见 §6 测试 seam 论证）：

```go
// handlers_proc.go（或就近）新增：
// writeInternalError 把 500 兜底统一化：固定面向用户文案 + 原始 error 进日志。
// 不暴露内部细节给前端，保留可诊断性（RequestID 关联）。
func (h *handlers) writeInternalError(w http.ResponseWriter, userMsg string, cause error) {
    if h.deps.Logger != nil && cause != nil {
        h.deps.Logger.Error("internal error", "userMsg", userMsg, "cause", cause)
    }
    writeError(w, http.StatusInternalServerError, CodeInternal, userMsg, "")
}
```

3 处兜底分支改为调用 `h.writeInternalError(w, "<固定文案>", err)`。该 helper 是纯函数式行为（无状态），可在同包测试中 `h := &handlers{deps: Dependencies{Logger: capturingLogger}}` 直接调用断言。

## 4. 数据模型变更

无。

## 5. API 契约

### A-1 新增校验分支
- `GET /api/v1/server/runtime/proxy/{type}/{name}`，`{type}` ∉ `frpsProxyTypes`：
  - status `422`，body `{"error":{"code":"VALIDATION_FAILED","message":"不支持的代理类型","field":"type"}}`
  - 不调用上游 frps。

### B 文案变更（status code / envelope 形状不变）
| 端点 | 旧 message | 新 message |
|---|---|---|
| `POST /api/v1/proc/{kind}/stop` 500 | `<err.Error()>` | `停止进程失败` |
| `POST /api/v1/proxies` / `PUT` 兜底 500 | `保存失败: <SQL>` | `保存失败` |
| `POST /api/v1/system/download-bin` default 500 | `<err.Error()>` | `启动下载失败` |

错误码均保持 `INTERNAL`，status 保持 500。

## 6. 序列 / 流程

### A 请求流（escape 根防御）
```
前端 GET /api/v1/server/runtime/proxy/{type}/{name}
  → serverRuntimeProxyDetail
      ├─ pt = URLParam("type"); name = URLParam("name")
      ├─ [新增] pt ∉ frpsProxyTypes → 422 VALIDATION_FAILED field=type，return（不触上游）
      └─ c.ProxyDetail(ctx, pt, name)
          → doGet(ctx, "/api/proxy/"+url.PathEscape(pt)+"/"+url.PathEscape(name), ...)
              → baseURL + escapedPath（特殊字符已编码，不改变 path/query 边界）
```

### B 错误流（固定文案 + 日志）
```
handler 兜底 500 分支
  → h.writeInternalError(w, "<固定文案>", originalErr)
      ├─ Logger != nil → Logger.Error("internal error", "cause", originalErr)  // 内部细节进日志
      └─ writeError(w, 500, INTERNAL, "<固定文案>", "")                        // 前端只见固定文案
```

### 测试 seam 论证（为何用 helper）
`procStop` 的 500 兜底实际由 `*procmgr.Manager.Stop()` 触发，但 `Stop()` 仅在 `validateKind(kind)` 失败时返错——而 `procStop` 已用 `validProcKind(kind)` 前置校验（handlers_proc.go:35），故黑盒 HTTP 测试**无法**经真实 `*procmgr.Manager` 到达 L41 兜底（同理 `downloadBin` 的 `*downloader.Manager.Start()` 只返 sentinel）。`ProcMgr`/`Downloader` 在 `Dependencies` 中是**具体类型**（router.go:28/39），无接口 seam，引入 mock 接口会扩散到 5+ 调用点（违反"禁扩散"）。

**决策**：抽 `writeInternalError` 纯 helper，B-1/B-2/B-3 的"固定文案 + 不含内部细节 + 进日志"行为在该 helper 单测中以 `httptest.NewRecorder()` + 捕获型 `slog.Logger`（`slog.New` 接 `bytes.Buffer`）直接验证。这是零生产行为变更、零依赖、零扩散的最小可测路径。AC-1/AC-2（A 部分）仍走既有 `frpsAdminFactory` seam + httptest mock 的黑盒路径（无需 helper）。

## 7. 复用审计

| 需要 | 既有代码 | 文件路径 | 决策 |
|---|---|---|---|
| type 白名单 | `frpsProxyTypes` `[]string{"tcp",...}` | `internal/httpapi/handlers_server_runtime.go:46` | 复用——A-1 直接用线性扫描判 membership |
| logger nil 守卫 + 记录范式 | `if h.deps.Logger != nil { h.deps.Logger.Warn(...) }` | `internal/httpapi/handlers_proxies.go:147-148` | 复用范式——`writeInternalError` 沿用 |
| 统一错误响应 | `writeError(w, status, code, msg, field)` | `internal/httpapi/errors.go:50` | 复用——所有分支照常走它 |
| frpsadmin 测试 seam | `frpsAdminFactory` 可替换 + `NewWithBaseURL` | `handlers_server_runtime.go:105` / `frpsadmin/client.go:79` | 复用——AC-1 黑盒测试注入 mock |
| httptest path 断言范式 | `r.URL.Path` 断言（已有 TestProxyDetail_Success） | `frpsadmin/client_test.go:157` | 扩展——AC-2 改用 `r.URL.EscapedPath()` 验证编码 |
| path escape | `net/url.PathEscape`（标准库） | stdlib | 新 import（标准库，零第三方依赖；理由：单 segment 编码是 Go 推荐的 path 注入防御） |
| 测试用捕获 logger | `slog.New(slog.NewTextHandler(buf, nil))` | 项目测试已大量使用（io.Discard 形态） | 复用——B 测试改接 `bytes.Buffer` 以断言记录内容（可选）或仅断言响应文案 |

## 8. 风险分析

- R-1.（escape 回归）`url.PathEscape("tcp")` 必须等于 `"tcp"`，否则破坏所有正常 path。`url.PathEscape` 对 `[A-Za-z0-9-._~]` 等 unreserved 字符不编码，`tcp`/`ssh` 类输出字节一致。**缓解**：AC-2 显式断言正常 path 无回归（`/api/proxy/tcp/ssh` 不变）+ 既有 9 个 frpsadmin client 测试（含 TestProxyDetail_Success path 断言）回归网兜底。
- R-2.（PathEscape vs PathSegment 语义）`url.PathEscape` 不编码 `/`? —— 实测 `url.PathEscape("a/b")` = `"a%2Fb"`（PathEscape 会编码 `/`，因它面向单 path segment）。**缓解**：AC-2 用 `a/b` 断言编码为 `a%2Fb`。
- R-3.（B 改动误删语义化分支）handlers_proxies.go 的 L241/L248-255/L256-259 是有意的语义化分类，误删会让 409/422 退化成 500。**缓解**：只改 L260 单行，前置分支 1:1 保留；AC-5 仅断言兜底文案，既有 TestProxy_DuplicateName422 等回归网守门前置分支不变。
- R-4.（日志泄露到前端旁路）若 logger handler 误配成响应流会反向泄露。**缓解**：测试用 `bytes.Buffer`/`io.Discard`，生产 logger 是独立 ui.log；helper 只写 logger 不写 w（除 writeError）。
- R-5.（e2e 契约影响）本任务改后端错误 message，前端 e2e 若硬断言旧 message 文本会断。**缓解**：orchestrator 真跑全量 verify_all（含 e2e）作硬闸门；改的 3 处均是 500 兜底（e2e 正常路径不触发），且 A-1 是新增 422 分支不影响既有正常路径。

## 9. 迁移 / 上线计划

- 无 schema / 无数据迁移。
- 向后兼容：A-1 是新增校验（更严格），对合法输入零影响；B 仅改 message 文案，status/envelope 不变。
- 回滚：纯代码改动，`git revert` 即可。

## 10. 范围外澄清

- 不做 `serverRuntimeTraffic` 的 name 业务白名单（O-1）；client 层 escape 已是根防御。
- 不做 `(type,remote_port)` 冲突 422 的 sentinel 化（O-2，backlog）。
- 不碰 uploadBin errno 透传（O-3）。

## 11. 分区分配（partition 模式 — `.harness/agents/dev-*.md` 存在）

| 文件 | 分区 | New / Edit | 依赖 |
|---|---|---|---|
| `internal/frpsadmin/client.go` | dev-backend | edit | — |
| `internal/httpapi/handlers_server_runtime.go` | dev-backend | edit | — |
| `internal/httpapi/handlers_proc.go` | dev-backend | edit（含 writeInternalError helper） | — |
| `internal/httpapi/handlers_proxies.go` | dev-backend | edit | 依赖 writeInternalError |
| `internal/httpapi/handlers_system.go` | dev-backend | edit | 依赖 writeInternalError |
| `internal/frpsadmin/client_test.go` | dev-backend | edit（测试） | — |
| `internal/httpapi/handlers_server_runtime_test.go` | dev-backend | edit（测试） | — |
| `internal/httpapi/handlers_hygiene_test.go` | dev-backend | new（测试） | 依赖 writeInternalError |
| `scripts/baseline.json` | dev-backend | edit | 全部测试落地后 |

### Dispatch order
1. dev-backend（单分区，纯后端 Go）

### Parallelism
无——纯后端单分区，顺序落地。

## 12. 裁决

**READY**
