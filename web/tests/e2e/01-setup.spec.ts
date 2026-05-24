import { test, expect } from '@playwright/test'
import { assertFreshBackend } from './fixtures/auth'

test.describe('Setup', () => {
  test('TC-01 未初始化时访问 / 自动跳转 /setup', async ({ page }) => {
    // T-033：守门检测后端 initialized；若被复用 server 污染则给出明确根因 + 修复指引
    await assertFreshBackend(page)
    await page.goto('/')
    await expect(page).toHaveURL(/\/setup/)
  })

  test('TC-02 setup 表单提交成功后离开 /setup', async ({ page }) => {
    // T-033：与 TC-01 同款守门；TC-02 的 setup 提交语义只有在 fresh server 下才合法
    await assertFreshBackend(page)
    await page.goto('/setup')
    // form.username 初始值为 'admin'，fill 会先清空再填入
    await page.getByPlaceholder('admin').fill('e2eadmin')
    await page.getByPlaceholder('至少12位，含字母和数字').fill('E2eTestPass1!')
    await page.getByPlaceholder('再次输入密码').fill('E2eTestPass1!')
    await page.getByRole('button', { name: '完成初始化' }).click()
    // 允许跳转到 /dashboard 或 /wizard（路由守卫决定）
    await expect(page).not.toHaveURL(/\/setup/, { timeout: 10_000 })
    await expect(page.locator('body')).not.toContainText('初始化失败')
  })
})
