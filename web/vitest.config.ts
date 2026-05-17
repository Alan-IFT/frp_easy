import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  test: {
    environment: 'happy-dom',
    // E2E テストは Playwright で実行するため、vitest の探索から除外
    exclude: ['**/node_modules/**', '**/tests/e2e/**'],
  },
})
