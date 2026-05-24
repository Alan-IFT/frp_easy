import type { Page } from '@playwright/test'

const E2E_USERNAME = 'e2eadmin'
const E2E_PASSWORD = 'E2eTestPass1!'

/**
 * 前置条件守门：验证后端处于"未初始化"状态，可让 TC-01 的"自动跳 /setup"语义成立。
 *
 * 背景（T-033）：playwright.config.ts 的 `reuseExistingServer: !process.env.CI` 让本地
 * 非 CI 跑测试时复用已有 7800 端口的 frp-easy 进程。若上一轮 TC-02 已经把那个进程的
 * DataDir 写入了 admin，则本轮 TC-01 的"未初始化跳 /setup"前提被悄悄破坏，spec 报
 * "URL 不在 /setup" 但根因无从读出。本函数让根因显式化。
 *
 * 调用位置：01-setup.spec.ts 的 TC-01 / TC-02 第一行。
 * 调用代价：1 个 GET /api/v1/system/ready 请求 + JSON 解析，<50ms。
 * 鉴权要求：无（endpoint 在中间件链中位于 SessionAuth 之前，公开可达；与
 * web/src/router.ts L36-40 路由守卫匿名调用同款路径）。
 *
 * 失败时抛 Error 包含修复指引；Playwright list reporter 会原样打印多行 \n 换行。
 */
export async function assertFreshBackend(page: Page): Promise<void> {
  const resp = await page.request.get('/api/v1/system/ready')
  if (!resp.ok()) {
    throw new Error(
      `前置条件检测失败：GET /api/v1/system/ready 返回 HTTP ${resp.status()}。` +
      `请检查后端是否正常启动（参考 scripts/start-e2e-server.{ps1,sh}）。`,
    )
  }
  const body = await resp.json() as { initialized: boolean; binMissing: string[]; version: string }
  if (body.initialized) {
    throw new Error(
      '前置条件违反：后端已初始化（initialized=true），无法验证"未初始化时自动跳转 /setup"语义。\n' +
      '根因：Playwright reuseExistingServer 复用了一个 DataDir 含 admin 的 frp-easy 进程（典型于本地非 CI 多轮跑测试，且上一轮残留进程仍占着 127.0.0.1:7800）。\n' +
      '修复指引：\n' +
      '  1. 关闭所有占用 127.0.0.1:7800 的本地 frp-easy 实例：\n' +
      '     - Windows (普通进程): Get-Process | Where-Object { $_.Path -like "*frp-easy*" } | Stop-Process -Force\n' +
      '     - Windows (服务模式 / Session 0 进程拒绝访问时): **以管理员身份打开 PowerShell**（Win+X → Terminal (Admin)），然后 Stop-Service frp-easy；测完 Start-Service frp-easy 恢复。普通用户 / 非 elevated session 会报 "Cannot open frp-easy service on computer" 拒绝访问\n' +
      '     - Linux/Mac: lsof -ti :7800 | xargs kill  # systemd 装为服务时改用：sudo systemctl stop frp-easy\n' +
      '  2. 重跑 `cd web && npx playwright test --project=chromium`\n' +
      '  3. 或显式设置 CI=true 强制 Playwright 启全新 webServer + 全新 tmpdir：\n' +
      '     - PowerShell: $env:CI = "true"; cd web; npx playwright test --project=chromium\n' +
      '     - bash:       CI=true npx playwright test --project=chromium\n' +
      '     注：CI=true 让 Playwright 不复用既有 server，但端口仍是 7800 —— 必须先做步骤 1 才能让 webServer 起来',
    )
  }
}

/**
 * 读后端 ready 状态并返回结构化结果，调用方决定如何处理。
 * 与 assertFreshBackend 互补：assertFreshBackend 是"判定即 throw"硬守门；
 * 本函数是"取数据让调用方决策"软查询。当前 spec 未直接使用，留给未来 02-auth /
 * 03-dashboard 类似前提条件检查复用。
 */
export async function getBackendReadyStatus(
  page: Page,
): Promise<{ initialized: boolean; binMissing: string[]; version: string }> {
  const resp = await page.request.get('/api/v1/system/ready')
  if (!resp.ok()) {
    throw new Error(`getBackendReadyStatus failed: HTTP ${resp.status()}`)
  }
  return resp.json() as Promise<{ initialized: boolean; binMissing: string[]; version: string }>
}

/**
 * 通过后端 API 直接完成账号初始化（不走 UI）。
 * 若账号已存在（后端返回 409），静默忽略，保证幂等。
 * POST /api/v1/setup 是公开接口，无需认证。
 */
export async function setupAccount(
  page: Page,
  opts: { username?: string; password?: string } = {},
): Promise<void> {
  const username = opts.username ?? E2E_USERNAME
  const password = opts.password ?? E2E_PASSWORD

  const resp = await page.request.post('/api/v1/setup', {
    data: { username, password },
  })
  // 200 = 创建成功；409 = 已存在（幂等，不报错）
  if (!resp.ok() && resp.status() !== 409) {
    throw new Error(`setupAccount failed: HTTP ${resp.status()}`)
  }
}

/**
 * 通过后端 API 直接登录（不走 UI），获取 session cookie。
 * page.request 与 page 共享 cookie 上下文，API 设置的 frp_easy_sid 立即对浏览器有效。
 */
export async function programmaticLogin(
  page: Page,
  opts: { username?: string; password?: string } = {},
): Promise<void> {
  const username = opts.username ?? E2E_USERNAME
  const password = opts.password ?? E2E_PASSWORD

  const resp = await page.request.post('/api/v1/auth/login', {
    data: { username, password },
  })
  if (!resp.ok()) {
    throw new Error(`programmaticLogin failed: HTTP ${resp.status()}`)
  }
  // resp 的 Set-Cookie 头已写入 page 的 browser context，后续 goto 自动携带
}

/**
 * 调用 wizard/complete API 将 wizard.handled 置为 "true"，
 * 消除路由守卫对 /dashboard 的重定向（TC-04、TC-05 前置操作）。
 * 前提：page 已处于已登录状态（session cookie 已设置）。
 * 若 wizard 已经 complete，再次调用是幂等的（KVSet 覆盖写）。
 *
 * csrfToken 字段名：json:"csrfToken"（GET /api/v1/auth/csrf 响应体）
 */
export async function bypassWizard(page: Page): Promise<void> {
  // GET /api/v1/auth/csrf — 此端点为 GET，不受 CSRF 检查；返回 { csrfToken: "..." }
  const csrfResp = await page.request.get('/api/v1/auth/csrf')
  if (!csrfResp.ok()) {
    throw new Error(`bypassWizard: csrf fetch failed: HTTP ${csrfResp.status()}`)
  }
  const body = await csrfResp.json() as { csrfToken: string }
  const csrfToken = body.csrfToken

  // POST /api/v1/wizard/complete — 需要 X-CSRF-Token header（middleware.go CSRF()）
  const completeResp = await page.request.post('/api/v1/wizard/complete', {
    headers: { 'X-CSRF-Token': csrfToken },
    data: {},
  })
  if (!completeResp.ok()) {
    throw new Error(`bypassWizard: wizard/complete failed: HTTP ${completeResp.status()}`)
  }
}

/**
 * 通过后端 API 退出登录（不走 UI）。
 * 若需要在不经过 UI 点击的情况下清除 session，可调用此函数。
 */
export async function programmaticLogout(page: Page): Promise<void> {
  // 先获取 CSRF token
  const csrfResp = await page.request.get('/api/v1/auth/csrf')
  if (!csrfResp.ok()) {
    throw new Error(`programmaticLogout: csrf fetch failed: HTTP ${csrfResp.status()}`)
  }
  const body = await csrfResp.json() as { csrfToken: string }
  const csrfToken = body.csrfToken

  const resp = await page.request.post('/api/v1/auth/logout', {
    headers: { 'X-CSRF-Token': csrfToken },
    data: {},
  })
  if (!resp.ok()) {
    throw new Error(`programmaticLogout failed: HTTP ${resp.status()}`)
  }
}
