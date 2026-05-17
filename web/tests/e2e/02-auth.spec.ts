import { test, expect } from '@playwright/test'

test.describe('Auth', () => {
  test('TC-03 login 表单提交成功后离开 /login', async ({ page }) => {
    // 前置条件：管理员账号已由 TC-02 创建（共享同一后端实例）
    await page.goto('/login')
    await page.getByPlaceholder('admin').fill('e2eadmin')
    await page.getByPlaceholder('密码').fill('E2eTestPass1!')
    await page.getByRole('button', { name: '登录' }).click()
    // 允许跳转到 /dashboard 或 /wizard（路由守卫决定）
    await expect(page).toHaveURL(/\/(dashboard|wizard)/, { timeout: 10_000 })
    await expect(page.locator('body')).not.toContainText('用户名或密码错误')
  })
})
