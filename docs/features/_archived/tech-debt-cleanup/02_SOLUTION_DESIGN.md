# 方案设计 — T-004 tech-debt-cleanup

**任务 ID**：T-004  
**日期**：2026-05-16

---

## 1. 分区划分

| 分区 | 文件 | 说明 |
|---|---|---|
| scripts | `scripts/verify_all.sh`, `scripts/verify_all.ps1`, `scripts/build.sh`, `scripts/build.ps1` | OPT-1 + OPT-5 |
| frontend | `web/src/router.ts` | OPT-2 |
| backend | `cmd/frp-easy/main.go`, `internal/httpapi/router.go`, `internal/httpapi/handlers_system.go`, `internal/httpapi/qa_ac_test.go` | OPT-4 + OPT-6 + OPT-7 + OPT-8 |

---

## 2. 详细设计

### 2.1 verify_all 前端路径修复（OPT-1）

**verify_all.sh**：B.1-B.4 块将 `if [[ ! -f package.json ]]` 改为 `if [[ ! -f web/package.json ]]`，并在块内首行 `pushd web`，块尾 `popd`。pkgmgr() 调用不变（在 web/ 内执行时自动找到 package.json）。

**verify_all.ps1**：同理，`if (-not (Test-Path "package.json"))` 改为 `if (-not (Test-Path "web/package.json"))`，B.1-B.4 块内 `Push-Location (Join-Path $root "web")` + 块尾 `Pop-Location`。

### 2.2 向导路由守卫补全（OPT-2）

在 `router.ts` 的 `beforeEach` 中，在现有 wizard 检查块（`if auth.user !== null && to.path === '/dashboard'`）之后，增加一个独立检查：

```typescript
// 向导已完成时，直接访问 /wizard 重定向到 /dashboard（TD-1 修复）
if (auth.user !== null && to.path === '/wizard') {
  const wizard = useWizardStore()
  if (!wizard.checked) {
    await wizard.checkWizard()
  }
  if (!wizard.shouldShow) {
    return '/dashboard'
  }
}
```

注意：`shouldShow` 为 false 时才重定向（包含 handled 且无需显示的情况）。

### 2.3 slog 双写（OPT-4）

`cmd/frp-easy/main.go` 的 `newLogger`：

```go
import "io"

func newLogger(logFile *os.File) *slog.Logger {
  if logFile != nil {
    w := io.MultiWriter(logFile, os.Stderr)
    return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}))
  }
  return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
```

### 2.4 版本号注入（OPT-5）

**build.sh**：将 `VERSION="0.1.0"` 替换为：
```bash
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
```

**build.ps1**：将 `$version = "0.1.0"` 替换为：
```powershell
$version = try { git describe --tags --always --dirty 2>$null } catch { "dev" }
if (-not $version) { $version = "dev" }
```

### 2.5 ParseIPFromJSON 统一（OPT-6）

`internal/httpapi/handlers_system.go` 的 `fetchIPFromURL`：

当前：使用内联 `var body struct { IP string "json:\"ip\"" }` + `json.NewDecoder`

修改后：
```go
import (
  "io"
  "github.com/frp-easy/frp-easy/internal/downloader"
)

func fetchIPFromURL(ctx context.Context, url string) (string, error) {
  req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
  resp, err := http.DefaultClient.Do(req)
  if err != nil { return "", fmt.Errorf("do request: %w", err) }
  defer resp.Body.Close()
  if resp.StatusCode != http.StatusOK { return "", fmt.Errorf("HTTP %d", resp.StatusCode) }
  data, err := io.ReadAll(resp.Body)
  if err != nil { return "", fmt.Errorf("read body: %w", err) }
  return downloader.ParseIPFromJSON(data)
}
```

删除：`"encoding/json"` import（若无其他用途，否则保留）。注意：`downloadBin` handler 仍用 `json.NewDecoder`，所以 import 保留。

### 2.6 /api/v1/health 端点（OPT-7）

**router.go**：在 `/api/v1` 路由组的公开 endpoint 区域（ReadyGate 之外）添加：

实际上 health 端点应完全绕过 ReadyGate，需在 ReadyGate 中间件之前注册，或添加豁免。最简方案：在 `r.Use(ReadyGate(...))` 之前注册 `/api/v1/health` 路由。

```go
// 健康检查（ReadyGate 之前，始终可用）
r.Get("/api/v1/health", h.health)
```

**新增 handler**（在 handlers_system.go 末尾添加）：
```go
func (h *handlers) health(w http.ResponseWriter, r *http.Request) {
  writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": h.deps.Version})
}
```

**测试**：在 `qa_ac_test.go` 中添加 `TestHealth_ReturnsOK`。

### 2.7 auto-restore TOML 预检（OPT-8）

`cmd/frp-easy/main.go` 的 `autoRestoreProcs`，在 `pm.Start(kind)` 之前添加：

```go
tomlPath, hasCfg := pm_config_paths[kind]  // 实际上 configPaths 需传入
if hasCfg {
  if _, err := os.Stat(tomlPath); os.IsNotExist(err) {
    logger.Warn("auto-restore skipped: config file missing", "kind", kind, "path", tomlPath)
    continue
  }
}
```

`autoRestoreProcs` 函数签名需增加 `configPaths map[string]string` 参数（来自 `deps.ConfigPaths`，在 main.go 中已有 `frpcTOML`、`frpsTOML` 变量）。

---

## 3. 测试策略

- verify_all 修复：跑 `bash scripts/verify_all.sh` 确认 B.1/B.3 从 SKIP 变为 PASS
- 向导守卫：在 `web/src/stores/__tests__/` 添加路由守卫测试（或通过 Vitest 测试 wizard store 逻辑）
- slog 双写：代码审查（不写单测）
- 版本注入：代码审查
- ParseIPFromJSON：go test ./internal/httpapi/... 通过（现有 IP 测试涵盖）
- health 端点：go test ./internal/httpapi/... 新增测试
- TOML 预检：代码审查 + 现有 Go 测试不降

---

## 4. 风险

- verify_all.sh pushd/popd 在某些 CI 环境需要注意，但 bash 标准支持
- health 端点绕过 ReadyGate 需要在 chi router 中 ReadyGate 之前注册（设计已说明）
- ParseIPFromJSON 统一：两者行为完全相同，无兼容性风险
