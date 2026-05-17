import type { Page } from '@playwright/test'

const E2E_USERNAME = 'e2eadmin'
const E2E_PASSWORD = 'E2eTestPass1!'

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
