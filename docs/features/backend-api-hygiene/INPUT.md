# INPUT — T-055 backend-api-hygiene

- **slug**: `backend-api-hygiene`
- **mode**: full（7-stage）
- **一句话目标**: 两项后端 API 卫生改进——(A) 堵 frps 运行态代理端点的 path 注入缺口；(B) 停止 handler 向前端透传内部错误细节。

## A. Path 注入缺口（健壮性/安全 + 输入校验）
- `internal/httpapi/handlers_server_runtime.go:194-207` `serverRuntimeProxyDetail`：`pt := chi.URLParam(r,"type")` 未校验直接传 `c.ProxyDetail(...)`。
- `internal/httpapi/handlers_server_runtime.go:210-222` `serverRuntimeTraffic`：`name := chi.URLParam(r,"name")` 同样未校验。
- 下游 `internal/frpsadmin/client.go:190-191` `doGet` 用 `c.baseURL+path` 纯字符串拼接；`Traffic`/`ProxyDetail`/`Proxies` 把 `{type}`/`{name}` 直接拼进 path 未经 `url.PathEscape`。
- 修复方向：
  1. `serverRuntimeProxyDetail` 用已存在 `frpsProxyTypes` 校验 `pt`，不命中返 422（`CodeValidationFailed` + 友好中文）。
  2. `internal/frpsadmin/client.go` 构造 path 的所有位置（`Proxies`/`ProxyDetail`/`Traffic`），对 `type`/`name` segment 用 `url.PathEscape(...)` 编码后再拼。这是防御根位置。

## B. handler 错误细节透传（一致性 + 不泄露内部）
- `internal/httpapi/handlers_proc.go:41`（procStop）：`writeError(..., err.Error(), "")` → 改固定文案。
- `internal/httpapi/handlers_proxies.go:260`（mapProxyWriteError 兜底 500）：`"保存失败: "+msg` → 改固定文案"保存失败"。保留 L241/L248-255/L256-259 不动。
- `internal/httpapi/handlers_system.go:140`（downloadBin default 500）：`err.Error()` → 改固定文案。
- 务必保留 `internal/httpapi/handlers_system.go:586` uploadBin errno 透传（B-A.12 / R-7 有意决策）。
- 错误细节去向：用现成 `h.deps.Logger`（handlers_proxies.go:147-148 范式）记原始 error，前端 message 不含内部细节。

## 约束
- 改动面严格限定，禁扩散。(type,remote_port) sentinel 化记为 backlog 不做。
- 补测试，baseline.json bump go_tests + test_count。
- 06 含裸 `## Adversarial tests` 段。
- 不 git commit/push，不跑 archive-task。
- 07 含裸 `## Insight` 段。
- 红线：不编辑 `.claude/`/`CLAUDE.md`/`.github/`；不在 internal 子包外引用 internal；handler 不写裸 SQL。
