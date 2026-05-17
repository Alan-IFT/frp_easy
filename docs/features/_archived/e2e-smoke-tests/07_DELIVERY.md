# 07 Delivery — T-006 e2e-smoke-tests

**交付日期**：2026-05-17  
**PM**：Claude (PM Orchestrator)

---

## 交付摘要

T-006 完成。verify_all 从 PASS:17/SKIP:2 变为 **PASS:18/FAIL:0/SKIP:0**，C.1 E2E smoke 由 SKIP 变为 PASS。

---

## 变更列表

| 文件 | 说明 |
|---|---|
| `web/playwright.config.ts` | 新增：Playwright 主配置（webServer、chromium、workers=1） |
| `web/tests/e2e/01-setup.spec.ts` | 新增：TC-01、TC-02 |
| `web/tests/e2e/02-auth.spec.ts` | 新增：TC-03 |
| `web/tests/e2e/03-dashboard.spec.ts` | 新增：TC-04、TC-05 |
| `web/tests/e2e/fixtures/auth.ts` | 新增：setupAccount / programmaticLogin / bypassWizard / programmaticLogout |
| `scripts/start-e2e-server.sh` | 新增：E2E 后端启动脚本，含时间戳重建判断 |
| `scripts/verify_all.sh` | 修改：C.1 节感知 playwright.config.ts，执行 playwright |
| `scripts/verify_all.ps1` | 修改：C.1 节同步（含 $LASTEXITCODE 检查） |
| `web/package.json` | 新增 @playwright/test ^1.44.0 devDependency |
| `web/vitest.config.ts` | 新增 exclude 规则排除 E2E 目录 |
| `web/src/App.vue` | **Bug 修复**：添加 NConfigProvider + NMessageProvider |
| `docs/dev-map.md` | 更新：新增 playwright、E2E tests、start-e2e-server 条目 |

---

## verify_all 最终输出

```
PASS: 18 | WARN: 0 | FAIL: 0 | SKIP: 0
```

---

## Insight

### 1. Naive UI NMessageProvider 必须在根组件

`useMessage()` 在 headless 浏览器（Playwright）和真实浏览器都需要 `<n-message-provider />` 祖先组件。缺少时：在真实浏览器中 Naive UI 打印 console.error 但不崩溃（可能降级）；在 Playwright 的严格模式下，setup() 抛出异常导致组件返回空节点 `<!---->`，表单无法渲染。**规则**：凡使用 Naive UI 任何 `use*()` composable 的项目，根 App.vue 必须包裹对应 Provider。

### 2. Go embed 二进制与 dist/ 的时间戳依赖

`go build` 将 `internal/assets/dist/` 的静态快照嵌入二进制。若前端重建后不重新 go build，二进制服务陈旧 HTML。`start-e2e-server.sh` 用 `find dist/ -newer $BIN` 检查时间戳来决定是否重建，是最轻量的解决方案。备选：在 CI build step 中始终按顺序 `npm run build` → `go build`。

### 3. reuseExistingServer 与测试数据污染

`reuseExistingServer: !process.env.CI` 在 CI 环境中（每次 E2E 运行都有干净数据库）是正确的。在本地开发中，若端口 8080 已有运行中的旧实例（数据库已有用户），TC-01 会失败（不跳转 /setup）。开发者应知晓：跑 E2E 前需关闭本地 frp-easy 进程，或使用专用测试端口。
