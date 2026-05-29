import { defineConfig, devices } from '@playwright/test'

// E2E 用独立端口（默认 17800），刻意避开产品默认 7800：否则本机运行的 frp-easy 实例
// 占着 7800 会被 reuseExistingServer 复用成"脏后端"（DataDir 含 admin），让首个 setup
// 测试因 assertFreshBackend fail-fast —— 这是历史 C.1 假性失败的根因（insight L25）。
// start-e2e-server.{sh,ps1} 通过 webServer.env 收到同一个 E2E_PORT，绑定到该端口的
// 全新 tmpdir 数据目录，与用户的 7800 实例完全隔离。可用 E2E_PORT 环境变量覆盖。
const E2E_PORT = process.env.E2E_PORT || '17800'

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: false,
  workers: 1,
  reporter: 'list',
  use: {
    baseURL: `http://localhost:${E2E_PORT}`,
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
    url: `http://127.0.0.1:${E2E_PORT}/api/v1/health`,
    env: { E2E_PORT },
    timeout: 120_000,
    reuseExistingServer: !process.env.CI,
    stdout: 'pipe',
    stderr: 'pipe',
  },
})
