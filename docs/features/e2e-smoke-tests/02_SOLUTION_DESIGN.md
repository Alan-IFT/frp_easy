# T-006 技术方案：Playwright E2E 烟雾测试

**任务 ID**：T-006  
**Slug**：e2e-smoke-tests  
**日期**：2026-05-16  
**阶段**：design  
**上游**：`01_REQUIREMENT_ANALYSIS.md` — READY

---

## 1. 架构总览

本任务不修改任何 Go 业务代码或 Vue 组件，纯粹在 `web/` 目录下增加 Playwright 测试基础设施，并修改 `scripts/verify_all.sh` / `verify_all.ps1` 的 C.1 节使其感知新配置位置。核心变化：新增 `web/playwright.config.ts` 配置 Playwright 框架；新增 `scripts/start-e2e-server.sh` 脚本负责构建 Go 二进制并以独立临时数据目录启动后端；新增 3 个 spec 文件（带数字前缀保证执行顺序）加 1 个 fixture 辅助文件；`verify_all.sh` / `verify_all.ps1` 的 C.1 节扩展文件检测条件，并在找到 `web/playwright.config.ts` 时 pushd 到 `web/` 目录后执行 playwright 命令。整条链路：`verify_all.sh C.1` → `cd web && npm exec playwright test --project=chromium` → Playwright webServer 拉起 `start-e2e-server.sh` → Go 二进制以临时 DataDir 启动 → 5 个 TC 顺序执行 → 结果上报。

---

## 2. 受影响模块

| 文件 | 状态 | 说明 |
|---|---|---|
| `web/playwright.config.ts` | 新建 | Playwright 主配置 |
| `web/package.json` | 编辑 | devDependencies 添加 `@playwright/test ^1.44.0` |
| `web/package-lock.json` | 编辑 | npm install 自动生成 |
| `web/tests/e2e/01-setup.spec.ts` | 新建 | TC-01、TC-02 |
| `web/tests/e2e/02-auth.spec.ts` | 新建 | TC-03 |
| `web/tests/e2e/03-dashboard.spec.ts` | 新建 | TC-04、TC-05 |
| `web/tests/e2e/fixtures/auth.ts` | 新建 | `programmaticLogin` / `bypassWizard` 辅助函数 |
| `scripts/start-e2e-server.sh` | 新建 | 构建 Go 二进制并以临时数据目录启动后端 |
| `scripts/verify_all.sh` | 编辑 | C.1 节：扩展文件检测 + pushd 到 web/ |
| `scripts/verify_all.ps1` | 编辑 | C.1 节同步修改（PowerShell 版本） |

---

## 3. 模块分解

### 3.1 `web/playwright.config.ts`

**职责**：定义 Playwright 运行参数、webServer 配置、浏览器矩阵。

**关键字段设计**：

```typescript
import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './tests/e2e',
  // 无显式 testMatch → 靠数字前缀文件名保证字母序执行顺序
  fullyParallel: false,
  workers: 1,
  reporter: 'list',
  use: {
    baseURL: 'http://localhost:8080',
    trace: 'off',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: 'bash ../scripts/start-e2e-server.sh',
    url: 'http://localhost:8080/api/v1/health',
    timeout: 60_000,
    reuseExistingServer: !process.env.CI,
    stdout: 'pipe',
    stderr: 'pipe',
  },
})
```

**设计决策**：
- `workers: 1` — 单后端实例，顺序执行，避免并发污染共享 DB。
- `reuseExistingServer: !process.env.CI` — 本地可复用已运行的服务器；CI 每次强制启动新实例（requirement §2.3 item 11）。
- `webServer.url` 指向 `/api/v1/health` — 此端点绕过 ReadyGate 中间件，服务启动中即可访问（`router.go` 第 65 行 `r.Get("/api/v1/health", h.health)`），是可靠的就绪探测点。
- `timeout: 60_000` — 留足 Go 构建（如需）+ 进程启动时间（requirement §2.3 item 12）。
- `command` 路径 `../scripts/start-e2e-server.sh` — Playwright 从 `playwright.config.ts` 所在目录（`web/`）启动命令，`../` 指向项目根目录。

### 3.2 `scripts/start-e2e-server.sh`

**职责**：按需构建 Go 二进制；以独立临时数据目录启动 frp-easy 服务器进程（通过 `exec` 替换 shell 进程，确保 Playwright 能正确管理生命周期）。

**完整设计**：

```bash
#!/usr/bin/env bash
# start-e2e-server.sh — 为 Playwright E2E 测试启动 frp-easy 后端。
# 使用独立临时数据目录（FRP_EASY_CONFIG 环境变量注入），保证测试数据隔离。
# 通过 exec 替换 shell 进程，使 Playwright webServer 可直接管理 frp-easy 的生命周期。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN="$ROOT/bin/frp-easy"

# 1. 构建后端（bin/frp-easy 不存在时构建；已存在则跳过以加速重复运行）
if [[ ! -f "$BIN" ]]; then
  echo "[e2e-server] bin/frp-easy not found, building..." >&2
  cd "$ROOT"
  CGO_ENABLED=0 go build -o "$BIN" ./cmd/frp-easy
  echo "[e2e-server] build done" >&2
fi

# 2. 创建临时数据目录和配置文件
TMPDIR=$(mktemp -d)
echo "[e2e-server] using tmpdir: $TMPDIR" >&2

cat > "$TMPDIR/frp_easy.toml" <<EOF
UIBindAddr = "127.0.0.1"
UIPort     = 8080
DataDir    = "$TMPDIR/data"
LogDir     = "$TMPDIR/logs"
EOF

# 3. exec 替换 shell 进程，Playwright 通过 SIGTERM/SIGKILL 管理 frp-easy 生命周期
export FRP_EASY_CONFIG="$TMPDIR/frp_easy.toml"
exec "$BIN"
```

**关键机制说明**：
- `FRP_EASY_CONFIG` — main.go 第 59 行 `envOr("FRP_EASY_CONFIG", "frp_easy.toml")` 读取此变量。临时 TOML 将 DataDir / LogDir 指向 `$TMPDIR/data` 和 `$TMPDIR/logs`，彻底与项目开发数据隔离。
- `exec "$BIN"` — shell 被 frp-easy 进程替换，Playwright 的 `webServer.command` 子进程即为 frp-easy 本身，SIGTERM 能直接送达，优雅关停有效（`main.go` 第 183 行 `signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)`）。
- `internal/assets/dist/` 不存在时，`go build ./cmd/frp-easy` 因 `//go:embed all:dist`（assets 包）编译失败，脚本以非零退出码退出，Playwright webServer 超时，C.1 = FAIL（满足 requirement §2.3 item 10）。
- `bin/` 目录已在 `.gitignore` 中，构建产物不进版本库（dev-map.md `bin/` 节）。
- 临时目录由 OS 负责最终清理。在 CI 中每次测试使用新 `$TMPDIR`（`reuseExistingServer: false`），隔离有保证。

### 3.3 `web/tests/e2e/fixtures/auth.ts`

**职责**：封装跨 spec 复用的 API 辅助逻辑，不依赖 UI 交互。

**公共 API**：

```typescript
import type { Page } from '@playwright/test'

/** 通过后端 API 直接登录（不走 UI），获取 session cookie。
 *  page.request 与 page 共享 cookie 上下文，API 设置的 frp_easy_sid 立即对浏览器有效。
 */
export async function programmaticLogin(page: Page): Promise<void>

/** 调用 wizard/complete API 将 wizard.handled 置为 "true"，
 *  消除路由守卫对 /dashboard 的重定向（TC-04、TC-05 前置操作）。
 *  前提：page 已处于已登录状态（session cookie 已设置）。
 *  若 wizard 已经 complete，再次调用是幂等的（KVSet 覆盖写）。
 */
export async function bypassWizard(page: Page): Promise<void>
```

**实现伪代码**：

```typescript
export async function programmaticLogin(page: Page): Promise<void> {
  const resp = await page.request.post('/api/v1/auth/login', {
    data: { username: 'e2eadmin', password: 'E2eTestPass1!' },
  })
  // 非 200 时抛错（让测试明确失败，不静默忽略）
  if (!resp.ok()) {
    throw new Error(`programmaticLogin failed: HTTP ${resp.status()}`)
  }
  // resp 的 Set-Cookie 头已写入 page 的 browser context，后续 goto 自动携带
}

export async function bypassWizard(page: Page): Promise<void> {
  // GET /api/v1/auth/csrf — 此端点为 GET，不受 CSRF 检查；返回 { csrfToken: "..." }
  const csrfResp = await page.request.get('/api/v1/auth/csrf')
  if (!csrfResp.ok()) {
    throw new Error(`bypassWizard: csrf fetch failed: HTTP ${csrfResp.status()}`)
  }
  const { csrfToken } = await csrfResp.json() as { csrfToken: string }

  // POST /api/v1/wizard/complete — 需要 X-CSRF-Token header（middleware.go CSRF()）
  const completeResp = await page.request.post('/api/v1/wizard/complete', {
    headers: { 'X-CSRF-Token': csrfToken },
    data: {},
  })
  if (!completeResp.ok()) {
    throw new Error(`bypassWizard: wizard/complete failed: HTTP ${completeResp.status()}`)
  }
}
```

**为何 `page.request` 能携带 session cookie**：Playwright 的 `page.request` 是与该 page 的 BrowserContext 共享的 APIRequestContext，API 响应的 Set-Cookie 头写入 BrowserContext 的 cookie 存储，后续 `page.goto()` 导航自动包含这些 cookie。这是 Playwright 官方推荐的"直接 API 登录"方式。

---

## 4. 数据模型变更

无。不新增表或迁移。E2E 测试通过 `start-e2e-server.sh` 使用独立的临时数据目录，与主开发数据库完全隔离。

---

## 5. API 合约（测试使用的端点，均为现有 API）

| 端点 | 方法 | 鉴权 | CSRF | 用途 |
|---|---|---|---|---|
| `/api/v1/health` | GET | 无 | 无 | webServer 就绪探测；返回 `{"status":"ok","version":"..."}` |
| `/api/v1/setup` | POST | 无 | 无 | TC-02 建立管理员账号；已初始化返回 409 |
| `/api/v1/auth/login` | POST | 无 | 无 | programmaticLogin；成功返回 Set-Cookie: frp_easy_sid |
| `/api/v1/auth/csrf` | GET | SessionAuth | 无（GET 不检查） | bypassWizard 第一步；返回 `{"csrfToken":"..."}` |
| `/api/v1/wizard/complete` | POST | SessionAuth | 必需 X-CSRF-Token | bypassWizard 第二步；幂等写 kv wizard.handled=true |
| `/api/v1/auth/logout` | POST | SessionAuth | 必需（由前端 axios 拦截器处理） | TC-05 通过点击 UI 按钮触发，无需测试代码直接调用 |

以上端点均在 `internal/httpapi/router.go`（行 65–117）定义，无需修改。

---

## 6. 请求流程

```
verify_all.sh C.1
  └─ pushd web/
       └─ npm exec playwright test --project=chromium
            │
            ├─ [webServer startup]
            │    └─ bash ../scripts/start-e2e-server.sh
            │         ├─ go build -o bin/frp-easy ./cmd/frp-easy   (如果不存在)
            │         ├─ mktemp -d → 写 frp_easy.toml (DataDir=tmpdir)
            │         └─ exec bin/frp-easy  (FRP_EASY_CONFIG=tmpdir/frp_easy.toml)
            │              └─ HTTP 监听 :8080，/api/v1/health 立即可用
            │
            ├─ [Playwright polls http://localhost:8080/api/v1/health until 200]
            │
            ├─ 01-setup.spec.ts (chromium)
            │    ├─ TC-01: goto('/') → 期望 URL=/setup
            │    └─ TC-02: 填 setup 表单 → 提交 → 期望 URL≠/setup，无"初始化失败"
            │
            ├─ 02-auth.spec.ts (chromium)
            │    └─ TC-03: goto('/login') → 填登录表单 → 期望 URL=/dashboard|wizard，无"用户名或密码错误"
            │
            ├─ 03-dashboard.spec.ts (chromium)
            │    ├─ TC-04: programmaticLogin → bypassWizard → goto('/dashboard')
            │    │         → 期望 page 含"仪表盘"、"frpc（客户端）"、"frps（服务端）"
            │    └─ TC-05: programmaticLogin → bypassWizard → goto('/dashboard')
            │              → 点击"退出登录" → 期望 URL=/login
            │              → goto('/dashboard') → 期望 URL=/login (session 已清除)
            │
            └─ [Playwright sends SIGTERM to frp-easy process]
                 └─ frp-easy 优雅关停（main.go §7）
```

---

## 7. 选择器确认（从源文件实际读取，不猜测）

读取 `web/src/pages/Setup.vue`、`web/src/pages/Login.vue`、`web/src/pages/Dashboard.vue`、`web/src/components/AppLayout.vue` 后确认：

### Setup.vue 选择器

| 元素 | Playwright 选择器 | 来源 |
|---|---|---|
| 用户名输入框 | `page.getByPlaceholder('admin')` | `<n-input placeholder="admin" />`（Setup.vue 第 11 行） |
| 密码输入框 | `page.getByPlaceholder('至少12位，含字母和数字')` | `<n-input placeholder="至少12位，含字母和数字" />`（第 18 行） |
| 确认密码输入框 | `page.getByPlaceholder('再次输入密码')` | `<n-input placeholder="再次输入密码" />`（第 24 行） |
| 提交按钮 | `page.getByRole('button', { name: '完成初始化' })` | `<n-button @click="handleSubmit">完成初始化</n-button>`（第 32–36 行） |
| 失败文本（负向断言） | `page.locator('body')` + `not.toContainText('初始化失败')` | `extractErrorMessage(e, '初始化失败')`（第 117 行） |

**重要**：`form.username` 在 Setup.vue 中初始化为 `'admin'`（第 62 行），即页面加载时用户名框已有值 `admin`。Playwright 的 `fill()` 会先清空再填入，`'e2eadmin'` 可正确写入。

### Login.vue 选择器

| 元素 | Playwright 选择器 | 来源 |
|---|---|---|
| 用户名输入框 | `page.getByPlaceholder('admin')` | `<n-input placeholder="admin" />`（Login.vue 第 12 行） |
| 密码输入框 | `page.getByPlaceholder('密码')` | `<n-input placeholder="密码" />`（第 18 行） |
| 登录按钮 | `page.getByRole('button', { name: '登录' })` | `<n-button @click="handleLogin">登录</n-button>`（第 38 行） |
| 错误文本（负向断言） | `page.locator('body')` + `not.toContainText('用户名或密码错误')` | `e.response?.data?.error?.message ?? '用户名或密码错误'`（第 104 行） |

### Dashboard.vue 可见文本

| 元素 | Playwright 断言 | 来源 |
|---|---|---|
| 页面标题 | `expect(page.getByText('仪表盘')).toBeVisible()` | `<n-page-header title="仪表盘" />`（Dashboard.vue 第 3 行） |
| frpc 卡片标题 | `expect(page.getByText('frpc（客户端）')).toBeVisible()` | `<n-card title="frpc（客户端）">`（第 19 行） |
| frps 卡片标题 | `expect(page.getByText('frps（服务端）')).toBeVisible()` | `<n-card title="frps（服务端）">`（第 90 行） |

### AppLayout.vue 退出按钮

| 元素 | Playwright 选择器 | 来源 |
|---|---|---|
| 退出登录按钮 | `page.getByRole('button', { name: '退出登录' })` | `<n-button size="small" @click="handleLogout">退出登录</n-button>`（AppLayout.vue 第 44 行） |

**Naive UI 渲染特性**：`<n-input>` 渲染为标准 `<input>` 元素（placeholder 属性保留），`<n-button>` 渲染为标准 `<button>` 元素。Playwright 的 `getByPlaceholder()` 和 `getByRole('button', ...)` 均可直接使用，无需特殊处理。

---

## 8. 测试文件详细设计

### 8.1 `web/tests/e2e/01-setup.spec.ts`（TC-01、TC-02）

```typescript
import { test, expect } from '@playwright/test'

test.describe('Setup', () => {
  test('TC-01 未初始化时访问 / 自动跳转 /setup', async ({ page }) => {
    await page.goto('/')
    await expect(page).toHaveURL(/\/setup/)
  })

  test('TC-02 setup 表单提交成功后离开 /setup', async ({ page }) => {
    await page.goto('/setup')
    await page.getByPlaceholder('admin').fill('e2eadmin')
    await page.getByPlaceholder('至少12位，含字母和数字').fill('E2eTestPass1!')
    await page.getByPlaceholder('再次输入密码').fill('E2eTestPass1!')
    await page.getByRole('button', { name: '完成初始化' }).click()
    await expect(page).not.toHaveURL(/\/setup/, { timeout: 10_000 })
    await expect(page.locator('body')).not.toContainText('初始化失败')
  })
})
```

**注意**：TC-01 和 TC-02 共用同一个后端实例。TC-01 在 TC-02 之前运行（同文件顺序），此时数据库中无管理员，路由守卫跳转到 /setup 满足 TC-01 的断言。TC-02 运行后管理员账号创建完成，供后续文件使用。

### 8.2 `web/tests/e2e/02-auth.spec.ts`（TC-03）

```typescript
import { test, expect } from '@playwright/test'

test.describe('Auth', () => {
  test('TC-03 login 表单提交成功后离开 /login', async ({ page }) => {
    await page.goto('/login')
    await page.getByPlaceholder('admin').fill('e2eadmin')
    await page.getByPlaceholder('密码').fill('E2eTestPass1!')
    await page.getByRole('button', { name: '登录' }).click()
    await expect(page).toHaveURL(/\/(dashboard|wizard)/, { timeout: 10_000 })
    await expect(page.locator('body')).not.toContainText('用户名或密码错误')
  })
})
```

**注意**：此 spec 文件获得全新的 BrowserContext（无上一个文件的 session），但后端数据库中管理员账号（由 TC-02 创建）已存在。TC-03 通过 UI 重新登录，是对登录流程的独立验证。

### 8.3 `web/tests/e2e/03-dashboard.spec.ts`（TC-04、TC-05）

```typescript
import { test, expect } from '@playwright/test'
import { programmaticLogin, bypassWizard } from './fixtures/auth'

test.describe('Dashboard', () => {
  test('TC-04 dashboard 关键元素可见', async ({ page }) => {
    await programmaticLogin(page)  // 通过 API 登录，session cookie 写入 BrowserContext
    await bypassWizard(page)       // 调用 wizard/complete，消除路由守卫重定向
    await page.goto('/dashboard')
    await expect(page.getByText('仪表盘')).toBeVisible()
    await expect(page.getByText('frpc（客户端）')).toBeVisible()
    await expect(page.getByText('frps（服务端）')).toBeVisible()
  })

  test('TC-05 退出登录跳转 /login，session 清除', async ({ page }) => {
    await programmaticLogin(page)
    await bypassWizard(page)
    await page.goto('/dashboard')
    await page.getByRole('button', { name: '退出登录' }).click()
    await expect(page).toHaveURL(/\/login/, { timeout: 10_000 })
    // 验证 session 已清除：访问 /dashboard 应重定向回 /login
    await page.goto('/dashboard')
    await expect(page).toHaveURL(/\/login/)
  })
})
```

**TC-04/TC-05 均在同文件内各自独立**：每个 `test(...)` 获得新 page（fresh cookies），通过 `programmaticLogin` + `bypassWizard` 重建已登录状态。这确保 TC-04 和 TC-05 互不干扰。

**为何需要 `bypassWizard`**：新管理员账号且无任何 FRP 配置时，`wizard.shouldShow = true`（`handlers_wizard.go` `wizardStatus`），路由守卫跳转 /wizard。调用 `wizard/complete` 将 `wizard.handled = "true"` 写入 KV（`handlers_wizard.go` 第 68–74 行），路由守卫不再跳转。

---

## 9. verify_all.sh C.1 修改方案

**修改位置**：`scripts/verify_all.sh` 第 171–182 行

**当前代码**：

```bash
# --- C. E2E (require playwright config) ---
if [[ "$QUICK" == "true" ]]; then
    :  # skipped via flag, do not even emit step
elif [[ ! -f playwright.config.ts && ! -f playwright.config.js ]]; then
    step "C.1" "E2E smoke (playwright)" "SKIP"
else
    PM=$(pkgmgr)
    if $PM exec playwright test --project=chromium &>/dev/null; then
        step "C.1" "E2E smoke (playwright)" "PASS"
    else
        step "C.1" "E2E smoke (playwright)" "FAIL"
    fi
fi
```

**替换为**：

```bash
# --- C. E2E (require playwright config) ---
if [[ "$QUICK" == "true" ]]; then
    :  # skipped via flag, do not even emit step
elif [[ ! -f playwright.config.ts && ! -f playwright.config.js && \
        ! -f web/playwright.config.ts && ! -f web/playwright.config.js ]]; then
    step "C.1" "E2E smoke (playwright)" "SKIP"
else
    # frp_easy 约定：playwright config 位于 web/ 子目录
    if [[ -f web/playwright.config.ts || -f web/playwright.config.js ]]; then
        PLAYWRIGHT_DIR="$ROOT/web"
    else
        PLAYWRIGHT_DIR="$ROOT"
    fi
    pushd "$PLAYWRIGHT_DIR" >/dev/null
    PM=$(pkgmgr)   # 在 playwright 目录内调用，检测 web/ 下的 lockfile（web/ 用 npm）
    if $PM exec playwright test --project=chromium &>/dev/null; then
        step "C.1" "E2E smoke (playwright)" "PASS"
    else
        step "C.1" "E2E smoke (playwright)" "FAIL"
    fi
    popd >/dev/null
fi
```

**设计说明**：
- `pkgmgr` 函数（verify_all.sh 第 44–49 行）检测 `pnpm-lock.yaml` / `yarn.lock`（相对路径）。在 `pushd "$PLAYWRIGHT_DIR"` 后调用，会检测 `web/` 目录下的 lockfile。`web/` 目录无 pnpm-lock.yaml / yarn.lock，`pkgmgr` 返回 `npm`，与 `web/package.json` 一致。**不需要修改 `pkgmgr` 函数**。
- `PLAYWRIGHT_DIR` 变量作用域局部于该 `if-else` 块，不影响其他检查节。
- `--quick` 分支不变：`QUICK=true` 时跳过整个 C.1，输出中不出现 C.1 行（requirement §2.2 item 7）。

**Diff 精确行范围**：替换 `scripts/verify_all.sh` 第 173–182 行（`elif [[ ! -f playwright.config.ts...` 到末尾的 `fi`）。

---

## 10. verify_all.ps1 C.1 修改方案

**修改位置**：`scripts/verify_all.ps1` 第 157–163 行

**当前代码**：

```powershell
# --- C. End-to-end (require playwright config) ---
if (-not $Quick) {
    Step "C.1" "E2E smoke (playwright)" {
        if (-not ((Test-Path "playwright.config.ts") -or (Test-Path "playwright.config.js"))) { return "SKIP" }
        $pkgMgr = if (Test-Path "pnpm-lock.yaml") { "pnpm" } elseif (Test-Path "yarn.lock") { "yarn" } else { "npm" }
        & $pkgMgr exec playwright test --project=chromium 2>&1 | Out-Null
    }
}
```

**替换为**：

```powershell
# --- C. End-to-end (require playwright config) ---
if (-not $Quick) {
    Step "C.1" "E2E smoke (playwright)" {
        $hasConfig = (Test-Path "playwright.config.ts") -or (Test-Path "playwright.config.js") `
                  -or (Test-Path "web/playwright.config.ts") -or (Test-Path "web/playwright.config.js")
        if (-not $hasConfig) { return "SKIP" }
        # frp_easy 约定：playwright config 位于 web/ 子目录
        if ((Test-Path "web/playwright.config.ts") -or (Test-Path "web/playwright.config.js")) {
            $playwrightDir = Join-Path $root "web"
        } else {
            $playwrightDir = $root
        }
        Push-Location $playwrightDir
        try {
            $pkgMgr = if (Test-Path "pnpm-lock.yaml") { "pnpm" } elseif (Test-Path "yarn.lock") { "yarn" } else { "npm" }
            & $pkgMgr exec playwright test --project=chromium 2>&1 | Out-Null
        } finally {
            Pop-Location
        }
    }
}
```

**设计说明**：`Push-Location` / `Pop-Location` 在 `try/finally` 中执行，确保即使 playwright 命令失败也能恢复工作目录（Step 函数的 catch 块捕获异常后脚本继续）。

---

## 11. Reuse Audit

| 需求 | 现有代码 | 文件路径 | 决策 |
|---|---|---|---|
| 后端 API 端点（health, setup, login, csrf, wizard/complete） | `handlers_setup.go`, `handlers_auth.go`, `handlers_wizard.go`, `router.go` | `internal/httpapi/` | 原样复用，不修改 |
| Go 二进制构建入口 | `main.go` | `cmd/frp-easy/main.go` | 原样复用；start-e2e-server.sh 调用 `go build ./cmd/frp-easy` |
| 应用配置加载（FRP_EASY_CONFIG 环境变量） | `appconf.Load()` | `internal/appconf/config.go` 第 59 行 `envOr("FRP_EASY_CONFIG", ...)` | 直接利用现有机制，无需修改 |
| verify_all.sh 包管理器检测函数 | `pkgmgr()` 函数 | `scripts/verify_all.sh` 第 44–49 行 | 复用原函数，在 pushd 后调用即可感知正确目录 |
| verify_all.sh step 输出函数 | `step()` 函数 | `scripts/verify_all.sh` 第 33–42 行 | 原样复用 |
| frp-easy 优雅关停 | SIGTERM handler | `cmd/frp-easy/main.go` 第 182–193 行 | exec 方式启动后，Playwright kill 直达 frp-easy 进程 |
| 健康检查端点 | `h.health` handler | `internal/httpapi/handlers_system.go` 第 224–230 行 | 直接用作 webServer.url 探测点 |
| CSRF 机制 | `CSRF()` middleware + `h.csrf` handler | `internal/httpapi/middleware.go` 第 224–245 行，`handlers_auth.go` 第 125–133 行 | fixture bypassWizard 遵循此机制 |

---

## 12. 风险分析

| 风险 | 概率 | 影响 | 缓解措施 |
|---|---|---|---|
| **R-1：测试文件执行顺序不可靠** — 若 Playwright 不遵循字母序（在某些版本或平台），文件顺序错乱会导致 TC-03 在 TC-02 之前运行（管理员不存在），TC-04 在 setup 之前运行。 | 低 | 高 | 使用数字前缀（`01-setup`、`02-auth`、`03-dashboard`）确保字母序即为执行序。Playwright with `workers: 1` 的字母序发现是文档化行为（v1.44+）。 |
| **R-2：端口 8080 冲突** — 开发者本地运行时，另一个 frp-easy 进程或其他服务占用 8080 端口，`start-e2e-server.sh` 启动失败。 | 中 | 中 | `reuseExistingServer: !process.env.CI` 在本地允许复用已有服务器（若已有 frp-easy 在 8080 监听）。CI 使用独立环境，无冲突风险。文档中说明开发者应确保 8080 空闲或关闭 `reuseExistingServer`。 |
| **R-3：Naive UI 组件渲染不透明** — 若某 Naive UI 版本的 `<n-input>` 使用 Shadow DOM 或 `<div contenteditable>` 而非标准 `<input>`，`getByPlaceholder()` 失效。 | 低 | 高 | 已通过实际阅读 `web/package.json`（naive-ui ^2.38.0）确认该版本不使用 Shadow DOM，渲染为标准 `<input>` 和 `<button>` 元素。若升级 naive-ui 大版本后失效，需检查选择器。 |
| **R-4：bypassWizard 在 TC-04 运行前 wizard 已完成** — TC-03 的路由守卫跳转使向导页面加载，Vue wizard store 可能在 TC-03 测试中触发 wizard/complete API（如用户手动操作）。 | 低 | 低 | `bypassWizard` 是幂等的（`KVSet(ctx, "wizard.handled", "true")` 覆盖写，多次调用无副作用）。即使 wizard 已完成，再次调用返回 200，测试不中断。 |
| **R-5：go build 耗时超出 60 秒超时** — CI 环境较慢，首次构建 Go 二进制超过 60s timeout，webServer 超时，C.1 = FAIL。 | 低 | 中 | CI 流程应在 E2E 之前已执行 G.3（`go build ./cmd/frp-easy`），`bin/frp-easy` 已存在，`start-e2e-server.sh` 跳过构建。`verify_all.sh` 的顺序是 G → B → C，G.3 先于 C.1 执行。 |
| **R-6：internal/assets/dist/ 不存在时的 FAIL vs SKIP 行为** — 若 dist/ 不存在，`go build` 编译时因 `//go:embed all:dist` 失败，start-e2e-server.sh 以非零退出，playwright webServer 超时，C.1 = FAIL。 | 可控 | 低 | 这是预期行为（requirement §2.3 item 10）。文档中明确告知开发者需先执行 `npm run build`（在 web/ 目录）再运行 E2E。 |

---

## 13. 迁移/上线计划

1. **向后兼容性**：不修改任何现有 API、Vue 组件、Go 代码，不修改数据库 schema。风险为零。
2. **package.json 变更**：新增 `@playwright/test` devDependency。`web/package-lock.json` 需要随之更新。开发者执行 `cd web && npm install` 即可同步，不影响生产构建（devDependency 不打包入 dist）。
3. **CI 流程影响**：如果 CI 使用 `npm ci`（frozen-lockfile），需要确保 `package-lock.json` 已提交（requirement §2.1 item 4 验收标准 AC-8）。
4. **Playwright 浏览器安装**：首次使用前需执行 `cd web && npx playwright install chromium`（仅安装 chromium，约 200 MB）。CI 需在测试步骤前添加此命令，或使用预装了 Playwright 的 Docker 镜像。
5. **rollback**：只需删除新增文件（`web/playwright.config.ts`、`web/tests/`、`scripts/start-e2e-server.sh`）并还原 `scripts/verify_all.sh` C.1 节和 `scripts/verify_all.ps1` C.1 节，verify_all.sh C.1 自动回到 SKIP 状态。

---

## 14. 超出范围的说明

- **Windows 本地 start-e2e-server.sh**：脚本使用 `bash`，Windows 开发者需通过 WSL 或 Git Bash 运行。不提供 `.ps1` 版本（本任务范围外）。
- **Firefox / WebKit 测试**：仅 chromium（requirement §3）。
- **截图、视频、trace**：本任务全部关闭（requirement §3）。
- **data-testid 属性**：不向 Vue 组件添加（requirement §3）。
- **向导流程内部测试**：不测试 /wizard 的具体步骤（requirement §3）。
- **E2E 失败时的详细报告**：使用默认 list reporter，不配置 HTML reporter（requirement §3）。
- **Playwright 浏览器安装 CI 步骤**：CI 配置（GitHub Actions 等）的修改不在本任务范围内，开发者需自行处理。

---

## 15. 分区分配

### 文件分区表

| 文件 | 分区 | 新建/编辑 | 依赖 |
|---|---|---|---|
| `scripts/start-e2e-server.sh` | dev-backend | 新建（调用 go build + exec frp-easy） | — |
| `scripts/verify_all.sh` | dev-frontend | 编辑（C.1 节 9 行替换） | 依赖 start-e2e-server.sh 语义理解 |
| `scripts/verify_all.ps1` | dev-frontend | 编辑（C.1 节 7 行替换） | 同上 |
| `web/package.json` | dev-frontend | 编辑（添加 @playwright/test） | — |
| `web/package-lock.json` | dev-frontend | 编辑（npm install 自动生成） | 依赖 package.json |
| `web/playwright.config.ts` | dev-frontend | 新建 | 依赖 start-e2e-server.sh |
| `web/tests/e2e/fixtures/auth.ts` | dev-frontend | 新建 | 依赖 API 端点设计 §5 |
| `web/tests/e2e/01-setup.spec.ts` | dev-frontend | 新建 | 依赖 fixtures/auth.ts、选择器 §7 |
| `web/tests/e2e/02-auth.spec.ts` | dev-frontend | 新建 | 同上 |
| `web/tests/e2e/03-dashboard.spec.ts` | dev-frontend | 新建 | 同上 |

### 调度顺序

1. **dev-backend** — 创建 `scripts/start-e2e-server.sh`
2. **dev-frontend** — 其余所有文件（依赖 start-e2e-server.sh 已就位）

### 并行性

dev-backend 先完成 start-e2e-server.sh，dev-frontend 才能完整验证 playwright.config.ts 的 webServer 配置。严格顺序。

---

## 16. 结论

**READY**

需求分析文件 `01_REQUIREMENT_ANALYSIS.md` 结论为 READY（第 173 行）。本方案覆盖了所有 8 条验收标准（AC-1 至 AC-8），所有关键选择器已从源文件实际确认，API 流程经 `router.go` / `handlers_auth.go` / `handlers_wizard.go` 交叉验证，verify_all.sh 修改方案精确到行号级别。无未解决的阻塞项。
