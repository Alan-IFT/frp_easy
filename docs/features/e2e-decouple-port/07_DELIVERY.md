# 07_DELIVERY — T-052 e2e-decouple-port

> 状态：**DELIVERED**（pending archive）· 2026-05-30 · batch project-optimization-2026-05

## 需求

根治 C.1 e2e 的长期假性失败（insight L25）：本机运行的 frp-easy 实例占着产品默认端口 7800 → Playwright `reuseExistingServer` 复用了这个"脏后端"（DataDir 含 admin）→ 首个 setup 测试 assertFreshBackend fail-fast。让全量 verify_all（含 e2e）能在任何装了 frp-easy 的开发机 / 用户机上可靠跑绿，而不必先关掉用户的 frp-easy。

## 方案与改动

核心：**e2e 用独立端口（默认 17800，env `E2E_PORT` 可覆盖），刻意避开产品默认 7800**，与用户本机实例结构性隔离。每轮仍用全新 tmpdir 数据目录（既有逻辑）。

- `web/playwright.config.ts`：新增 `const E2E_PORT = process.env.E2E_PORT || '17800'`，用于 `baseURL` / `webServer.url`，并通过 `webServer.env: { E2E_PORT }` 把解析后的端口显式传给启动脚本（保证 config 与脚本同值）。
- `scripts/start-e2e-server.sh`：`E2E_PORT="${E2E_PORT:-17800}"`，写入 `frp_easy.toml` 的 `UIPort`。
- `scripts/start-e2e-server.ps1`：`$e2ePort = if ($env:E2E_PORT) {...} else {"17800"}`，写入 `UIPort`（双实现对称，insight L26）。
- `web/tests/e2e/fixtures/auth.ts`：更新 assertFreshBackend 的背景注释与失败指引，说明 T-052 起 e2e 用独立端口、用户实例不再是诱因、本守门现在主要兜底"残留 e2e server 未 teardown"的少见情形；端口引用 7800 → 17800。

## 验证

- `cd web && npx playwright test --project=chromium`：**5 passed (4.6s)** —— 关键：**用户的 frp-easy 仍在 7800 运行**，e2e 在 17800 起全新后端、全过（修复前此场景必 fail-fast）。
- `bash scripts/verify_all.sh`（**完整，含 C.1 e2e**）：**PASS 32 / WARN 0 / FAIL 0**。C.1 从 insight L25 记录的"长期假性 FAIL / 被 --quick 豁免"恢复为真实 PASS。这是含 e2e 的全套验证首次在本机全绿，解锁后续任务的全量 verify_all 闸门。

## Adversarial tests

- 正向证伪：在用户 frp-easy 占用 7800 的真实环境下跑 e2e，5/5 通过（修复前是确定性 fail-fast）—— 证明端口隔离真的避开了产品实例。
- 配置一致性：`webServer.env: { E2E_PORT }` 把 config 解析的端口显式注入子进程，杜绝"config 用 17800、脚本默认值漂移到别的值"的隐患；脚本各自的 `:-17800` 仅作脱离 playwright 单跑时的兜底。

## Insight

- e2e 测试**绝不应复用产品默认端口**：任何会被用户/开发者实际安装运行的服务，其 e2e 必须用独立端口（这里 17800），否则 `reuseExistingServer` 会复用真实运行实例造成脏后端。这比 assertFreshBackend 那种"事后检测 + 报错指引"更治本——后者只能告诉你坏了，端口隔离让它根本不发生。
- Playwright `webServer.env` 是让 config 与启动脚本共享运行参数的正道：config 解析一次 `E2E_PORT` 后通过 env 显式下发，避免两处各自读环境变量 + 默认值漂移。
- 即使 e2e webServer 在 win32 下经 `pwsh` 子进程启动后端，作为 `npx playwright test`（bash 发起）的嵌套子进程也能正常运行（本会话 PowerShell 直接调用被 deny，但嵌套 spawn 不受影响）。
