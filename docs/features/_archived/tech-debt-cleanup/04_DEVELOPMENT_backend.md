# T-004 后端开发记录

## 修改摘要

### OPT-4：slog 双写（`cmd/frp-easy/main.go`）

`newLogger` 函数原本在 `logFile != nil` 时只写文件，stderr 仅有启动 banner。
改用 `io.MultiWriter(logFile, os.Stderr)` 后，结构化 JSON 日志同时写入 `ui.log` 和
stderr，便于 Docker / systemd 日志采集与文件持久化并存。

- 新增 import：`"io"`
- 删除旧 `var w = os.Stderr` / `_ = w` 死代码

### OPT-8：auto-restore TOML 预检（`cmd/frp-easy/main.go`）

`autoRestoreProcs` 函数添加 `configPaths map[string]string` 参数。在调用
`pm.Start(kind)` 之前，通过 `os.Stat` 检查对应 TOML 文件是否存在。若不存在则
记 Warn 并 `continue`，避免子进程因缺少配置文件而立即进入 error 状态。

- 调用点改为：`autoRestoreProcs(store, pm, loc, logger, map[string]string{"frpc": frpcTOML, "frps": frpsTOML})`
- 预检仅在 `configPaths[kind]` 存在时生效，保持向后兼容

### OPT-6：ParseIPFromJSON 统一（`internal/httpapi/handlers_system.go`）

`fetchIPFromURL` 原先自行用 `json.NewDecoder` 解析 `{"ip":"..."}` 响应体。
现改为 `io.ReadAll` + `downloader.ParseIPFromJSON(data)`，复用 downloader
包中已有逻辑，消除重复实现。

- 新增 import：`"io"`（`downloader` 已存在于 imports）
- `"encoding/json"` 保留（`downloadBin` handler 仍使用 `json.NewDecoder`）

### OPT-7：`/api/v1/health` 端点（router.go + handlers_system.go + qa_ac_test.go）

**router.go** 重构：取消顶层 `r.Use()` 全局调用，改为：
1. 顶层直接注册 `GET /api/v1/health` — 无任何中间件
2. `r.Group(fn)` 内使用原有五个中间件（ReadyGate → Recover → RequestID → Logger → CORS）
3. NotFound / MethodNotAllowed / 根路径保留在顶层（静态资源无需 API 中间件）

**handlers_system.go** 新增 `health` 方法，返回 `{"status":"ok","version":"..."}` JSON。

**qa_ac_test.go** 新增两个测试：
- `TestHealth_ReturnsOK`：ready=true 时 GET /api/v1/health → 200 + status=ok
- `TestHealth_BypassesReadyGate`：ready=false 时 GET /api/v1/health 依然 → 200

## 测试结果

```
go build ./...                   # 无错误
go test ./...                    # 全部通过

cmd/frp-easy                     [no test files]
internal/appconf                 ok
internal/assets                  ok
internal/auth                    ok
internal/binloc                  ok
internal/downloader              ok
internal/frpcadmin               ok
internal/frpconf                 ok
internal/httpapi                 ok  (4.1s)
internal/logtail                 ok
internal/procmgr                 ok
internal/storage                 ok
```

## 变更文件

- `cmd/frp-easy/main.go`
- `internal/httpapi/router.go`
- `internal/httpapi/handlers_system.go`
- `internal/httpapi/qa_ac_test.go`
