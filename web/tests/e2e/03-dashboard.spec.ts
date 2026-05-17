import { test, expect } from '@playwright/test'
import { programmaticLogin, bypassWizard } from './fixtures/auth'

test.describe('Dashboard', () => {
  test('TC-04 dashboard 关键元素可见', async ({ page }) => {
    // 通过 API 登录，session cookie 写入 BrowserContext
    await programmaticLogin(page)
    // 调用 wizard/complete，消除路由守卫重定向
    await bypassWizard(page)
    await page.goto('/dashboard')
    await expect(page.getByText('仪表盘').first()).toBeVisible()
    await expect(page.getByText('frpc（客户端）')).toBeVisible()
    await expect(page.getByText('frps（服务端）')).toBeVisible()
  })

  test('TC-05 退出登录跳转 /login，session 清除', async ({ page }) => {
    // 每个 test 获得新 page（fresh cookies），重建已登录状态
    await programmaticLogin(page)
    await bypassWizard(page)
    await page.goto('/dashboard')
    await page.getByRole('button', { name: '退出登录' }).click()
    // 验证跳转到 /login
    await expect(page).toHaveURL(/\/login/, { timeout: 10_000 })
    // 验证 session 已清除：再次访问 /dashboard 应重定向回 /login
    await page.goto('/dashboard')
    await expect(page).toHaveURL(/\/login/)
  })
})
