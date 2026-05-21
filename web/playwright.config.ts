import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: false,
  workers: 1,
  reporter: 'list',
  use: {
    baseURL: 'http://localhost:7800',
    trace: 'off',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  // Windows 上用 pwsh 调用 .ps1（PowerShell 自带 `bash` 解析到 WSL shim 会失败）；
  // 其他平台沿用 bash 调用 .sh。两个脚本行为等价（详见 docs/features/_archived/polish-pass/02_SOLUTION_DESIGN.md §2）。
  webServer: {
    command: process.platform === 'win32'
      ? 'pwsh -NoProfile -ExecutionPolicy Bypass -File ../scripts/start-e2e-server.ps1'
      : 'bash ../scripts/start-e2e-server.sh',
    url: 'http://127.0.0.1:7800/api/v1/health',
    timeout: 120_000,
    reuseExistingServer: !process.env.CI,
    stdout: 'pipe',
    stderr: 'pipe',
  },
})
