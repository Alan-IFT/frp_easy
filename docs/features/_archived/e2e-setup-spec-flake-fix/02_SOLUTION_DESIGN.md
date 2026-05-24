# 02 — Solution Design · T-033 e2e-setup-spec-flake-fix

> Stage 2 输出。Mode: full. Author: solution-architect（PM 派发上下文内角色化执行）.
> 上游：01_REQUIREMENT_ANALYSIS.md verdict = READY.

## 1. Architecture summary

**根本策略 = 让 `01-setup.spec.ts` 对"前置条件"显式负责**：测试运行时主动检测后端 `/api/v1/system/ready` 的 `initialized` 字段；若已 initialized，立即用明确的"前置条件违反"FAIL 信息暴露真因（满足 R-4），并附**修复指引**让维护者立即可执行。**不**改 `reuseExistingServer` 默认值（保持本地开发体验），**不**改后端 setup endpoint 语义，**不**改前端路由守卫。

辅助策略 = 在 `web/tests/e2e/fixtures/auth.ts` 新增 `assertFreshBackend(page)` 工具函数，把"前置条件检测 + 显式 FAIL 信息"封装为可复用 helper，未来 02-auth / 03-dashboard 类似前提条件检查可同款复用。

## 2. Affected modules（文件路径，存在 / 新增标注）

| 文件 | 存在/新增 | 改动概述 |
|---|---|---|
| `web/tests/e2e/01-setup.spec.ts` | 存在（改） | TC-01 之前 + TC-02 之前调用 `assertFreshBackend(page)`；并改善 TC-02 失败信息 |
| `web/tests/e2e/fixtures/auth.ts` | 存在（扩展） | 新增 `assertFreshBackend` + `resetBackendIfNeeded` 函数 |
| `web/playwright.config.ts` | 存在（不改） | **保留** `reuseExistingServer: !process.env.CI`；不动 |
| `scripts/start-e2e-server.{ps1,sh}` | 存在（不改） | 已经用独立 tmpdir，无需调整 |

## 3. Module decomposition（新增 / 扩展函数）

### `web/tests/e2e/fixtures/auth.ts` 新增 API

```typescript
/**
 * 前置条件守门：验证后端处于"未初始化"状态，可让 TC-01 的"自动跳 /setup"语义成立。
 * 若后端 initialized=true（典型场景：本地开发时 reuseExistingServer 复用了已有 server
 * 而该 server 的 DataDir 上一轮跑测试时已被 setup），抛 Playwright 友好的 FAIL，错误
 * 信息直接指明根因 + 修复指引（FR-R-4）。
 *
 * 调用位置：TC-01 / TC-02 第一行。
 * 调用代价：1 个 GET /api/v1/system/ready 请求 + JSON 解析，<50ms。
 */
export async function assertFreshBackend(page: Page): Promise<void>

/**
 * （可选 helper，目前不在 spec 直接调用 —— 留给未来 02-auth / 03-dashboard 复用扩展）
 * 检查后端 ready 状态并返回结构化结果，调用方决定如何处理。
 */
export async function getBackendReadyStatus(page: Page): Promise<{ initialized: boolean; binMissing: string[]; version: string }>
```

### `assertFreshBackend` 失败时的信息（R-4 实证）

```
Error: 前置条件违反：后端已初始化（initialized=true），无法验证"未初始化时自动跳转 /setup"语义。
根因：Playwright reuseExistingServer 复用了一个 DataDir 含 admin 的 frp-easy 进程（典型于本地非 CI 多轮跑测试）。
修复指引：
  1. 关闭所有占用 127.0.0.1:7800 的本地进程（Get-NetTCPConnection -LocalPort 7800 / lsof -i :7800）
  2. 重跑 `cd web && npx playwright test --project=chromium`
  3. 或显式设置 $env:CI = 'true' 强制 Playwright 启全新 webServer + 全新 tmpdir
```

## 4. Data model changes

**无**。本任务零 schema 改动、零 DB 影响。

## 5. API contracts

**无新 API**。复用现有 `GET /api/v1/system/ready`：

```
GET /api/v1/system/ready
Response 200:
{
  "initialized": boolean,
  "binMissing": string[],
  "version": string
}
```

参考实现 `internal/httpapi/handlers_system.go` L32-48（`systemReady` handler）。该 endpoint **不需要鉴权**（ReadyGate / SessionAuth 中间件链中 `/api/v1/system/ready` 通常前置；本任务前置假设其公开可达 —— 已被 TC-01 的 `page.goto('/')` 间接验证：路由守卫 `app.fetchReady()` 用同款 axios 调用同 endpoint，匿名可达）。

## 6. Sequence / flow

### TC-01 修复后流程

```
1. Playwright 启动 webServer（或复用已有；视 reuseExistingServer + 7800 占用）
2. webServer.url=/api/v1/health 200 OK
3. test('TC-01...') 进入
4. assertFreshBackend(page) 调用：
   a. page.request.get('/api/v1/system/ready')
   b. body.initialized 必须 = false
   c. 若 true → throw Error('前置条件违反: ...') → Playwright 报告 FAIL，错误信息直接告知根因
5. page.goto('/')
6. expect(page).toHaveURL(/\/setup/) （原有断言保留）
```

### TC-02 修复后流程

```
1. assertFreshBackend(page) 调用（同上；若复用 server + 已 setup → 立即 FAIL 出明确信息）
2. page.goto('/setup')
3. 表单填值 + 提交（原有逻辑保留）
4. expect(page).not.toHaveURL(/\/setup/, { timeout: 10_000 })
5. expect(...).not.toContainText('初始化失败')
```

注：TC-02 的 setup 接口提交在 `assertFreshBackend` PASS 后才执行 → 一定是 fresh server → 一定成功 → 后续 URL 跳转生效（满足 R-2）。

## 7. Reuse audit

| Need | Existing code | File path | Decision |
|---|---|---|---|
| Backend ready check | 路由守卫已用 | `web/src/router.ts` L36-40 `app.fetchReady()` | 测试侧 fixture 同款 axios.GET 调 endpoint（不通过 store，直接 page.request） |
| `/api/v1/system/ready` handler | `systemReady` | `internal/httpapi/handlers_system.go` L32-48 | 调用，无需扩展 |
| Setup endpoint 409 语义 | `setup` | `internal/httpapi/handlers_setup.go` L23-31 | 仅引用为根因 #3 证伪证据 |
| Test fixture 模板 | `setupAccount` / `programmaticLogin` / `bypassWizard` | `web/tests/e2e/fixtures/auth.ts` | 在同文件追加 `assertFreshBackend` |
| Playwright test 顺序保证 | `fullyParallel: false` + `workers: 1` | `web/playwright.config.ts` L4-6 | 复用 — TC 内顺序固定确保 TC-01 跑在 TC-02 setup 前 |

## 8. Risk analysis（至少 3 条 + mitigation）

| 风险 | 后果 | Mitigation |
|---|---|---|
| **R-1**：测试 fixture 新调一次 `page.request.get('/api/v1/system/ready')` 会增加 ~50ms × 2 TC = 100ms 单轮开销 | 性能 NFR-1（≤+20%）受微损 | 实测开销 << 当前 baseline（~30s），影响 < 1%。无需 mitigation。 |
| **R-2**：`assertFreshBackend` 的 FAIL 字符串硬编码中文 → 未来项目本地化变更可能漂移 | 错误信息维护成本 | 接受：项目当前所有 user-facing 文案均中文（zh-CN），与 frontend 既有约定一致。 |
| **R-3**：`/api/v1/system/ready` 在某些边缘情况下 5xx（DB 损坏 `ErrCorruptReset`）会让 `assertFreshBackend` 反向报"前置条件违反"误导 | 误诊（罕见路径） | `assertFreshBackend` 实现内对非-200 响应单独分支：throw "无法读取后端状态：HTTP <code>"，与"前置条件违反"区分开。 |
| **R-4**：TC-02 现在依赖 `assertFreshBackend` PASS 前提，但 TC-02 的真实任务是"提交 setup 表单成功"—— 如果上游 RA 后期希望 TC-02 也能跑在"已 setup"的 server 上（语义改"幂等"），本设计将不再覆盖 | RA 后续可能扩展 | 本任务严格按 RA `01 §2 R-1/R-2` 落地，扩展按新任务处理。 |
| **R-5**：（候选根因 #4 反向）如果用户报告的 flake 其实是另一个未被 PM 识别的根因（不在 RA 5 个候选内）→ 本 fix 不能根治 | 假性根治 | QA stage 6 必须实证：手工构造"已 setup 的 server" + 跑修后 spec → 必须 FAIL 出明确信息（AC-5）。如果该实证不能 FAIL，则根因诊断错。 |
| **R-6**：双实现 verify_all 中 C.1 不被本任务触及 | 与 insight L67 风险无关 | 本任务不改 verify_all。✓ |

## 9. Migration / rollout plan

- **回滚**：本任务零数据库、零后端、零生产 path 改动；测试代码改动可单文件 git revert。无回滚障碍。
- **兼容性**：spec 改动仅扩展前置守门，原有断言完全保留；任何"已经能 PASS 的环境"在改动后仍 PASS。CI 路径（reuseExistingServer=false → 永远 fresh server）改动后毫不受影响。
- **feature flag**：无必要。
- **MIGRATION.md**：无（无契约破坏）。

## 10. Out-of-scope clarifications

- **OOS-D1**：不改 `playwright.config.ts` 的 `reuseExistingServer` 字段。理由：改成 `false` 会让本地非 CI 每次 verify_all 启全新 webServer + go build（~5-10s），损害本地 dev 体验；fix 优先选"显式守门"而非"环境屏蔽"。
- **OOS-D2**：不动 `start-e2e-server.{ps1,sh}` 的 tmpdir 机制（已正确）。
- **OOS-D3**：不在测试侧主动调用 reset endpoint（后端**没有** reset endpoint；新增 reset endpoint 属于安全敏感改动 → 应单独任务评估，不在本任务 scope）。
- **OOS-D4**：不引入 Playwright `globalSetup` 钩子做集中守门。理由：现行 spec 数量少（3 个文件），逐 spec 显式 `assertFreshBackend` 可读性 > 隐式 globalSetup。未来 spec 增多时再迁移。

## 11. Partition assignment（必填，因有 dev-*.md）

| 文件 | Partition | New / Edit | Dependency |
|---|---|---|---|
| `web/tests/e2e/fixtures/auth.ts` | dev-frontend | edit（追加 `assertFreshBackend` + `getBackendReadyStatus`） | — |
| `web/tests/e2e/01-setup.spec.ts` | dev-frontend | edit（TC-01/TC-02 起首调 `assertFreshBackend`） | depends on `fixtures/auth.ts` |

### Dispatch order

1. dev-frontend（单分区一次完成）

### Parallelism

不适用（单分区）。

## 12. 实现层细节（让 dev 不用再做决策）

### `assertFreshBackend` 完整伪代码

```typescript
export async function assertFreshBackend(page: Page): Promise<void> {
  const resp = await page.request.get('/api/v1/system/ready')
  if (!resp.ok()) {
    throw new Error(
      `前置条件检测失败：GET /api/v1/system/ready 返回 HTTP ${resp.status()}。` +
      `请检查后端是否正常启动。`
    )
  }
  const body = await resp.json() as { initialized: boolean; binMissing: string[]; version: string }
  if (body.initialized) {
    throw new Error(
      '前置条件违反：后端已初始化（initialized=true），无法验证"未初始化时自动跳转 /setup"语义。\n' +
      '根因：Playwright reuseExistingServer 复用了一个 DataDir 含 admin 的 frp-easy 进程' +
      '（典型于本地非 CI 多轮跑测试，且上一轮残留进程仍占着 127.0.0.1:7800）。\n' +
      '修复指引：\n' +
      '  1. 关闭所有占用 127.0.0.1:7800 的本地进程：\n' +
      '     - Windows: Get-Process | Where-Object { $_.Path -like "*frp-easy*" } | Stop-Process -Force\n' +
      '     - Linux/Mac: lsof -ti :7800 | xargs kill\n' +
      '  2. 重跑 `cd web && npx playwright test --project=chromium`\n' +
      '  3. 或显式 `$env:CI = "true"` (PS) / `CI=true` (bash) 强制 Playwright 启全新 webServer'
    )
  }
}

export async function getBackendReadyStatus(
  page: Page,
): Promise<{ initialized: boolean; binMissing: string[]; version: string }> {
  const resp = await page.request.get('/api/v1/system/ready')
  if (!resp.ok()) {
    throw new Error(`getBackendReadyStatus failed: HTTP ${resp.status()}`)
  }
  return resp.json() as Promise<{ initialized: boolean; binMissing: string[]; version: string }>
}
```

### `01-setup.spec.ts` 修后骨架

```typescript
import { test, expect } from '@playwright/test'
import { assertFreshBackend } from './fixtures/auth'

test.describe('Setup', () => {
  test('TC-01 未初始化时访问 / 自动跳转 /setup', async ({ page }) => {
    await assertFreshBackend(page)  // 新增：守门 + 明确 FAIL 信息
    await page.goto('/')
    await expect(page).toHaveURL(/\/setup/)
  })

  test('TC-02 setup 表单提交成功后离开 /setup', async ({ page }) => {
    await assertFreshBackend(page)  // 新增：守门
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

## 13. 候选方案取舍（RA 建议至少 2 个，本设计已涵盖）

### 方案 A（选定）：测试侧显式守门 + 友好错误信息

- **优点**：零侵入生产代码；零后端改动；本地开发体验不受影响；错误信息直接告知根因 + 修复指引（满足 R-4）；fixture 可复用
- **缺点**：未根治"用户复用 server 时仍需手工 kill 进程"的体验摩擦 —— 但这是测试基础设施层面的真实约束，不应隐性消化
- **取舍理由**：用户原则 "用户体验好" 的本意是"出问题时立即知道为什么 + 怎么修"，而非"隐式吃掉问题"。错误信息显式化即 = 用户体验好。

### 方案 B（备选未选）：`playwright.config.ts` 改 `reuseExistingServer: false`

- **优点**：根因绝对隔离（永远新 server + 新 tmpdir）；CI 与本地行为一致
- **缺点**：本地每次 verify_all 强启 webServer = 重建 + 启动开销 ~5-10s；如果用户已开 vite dev 在 7800 端口 → webServer 启动**失败**（端口占用）让所有 e2e 直接 FAIL（更糟）
- **不选理由**：负面副作用 > 正面收益。`reuseExistingServer=true` 是 Playwright 官方推荐的本地开发模式，强行关闭违反工具最佳实践。

### 方案 C（备选未选）：后端加 `/api/v1/test/reset` endpoint + 测试 beforeAll 调它

- **优点**：测试可自治、不需要用户手工干预
- **缺点**：在生产二进制内引入 "wipe admin" 类安全敏感 endpoint（即使只在 dev / test build 下编译）→ 需 build tag 防御 + 单元测试覆盖 + 文档 + 安全评审 → scope 大幅膨胀，违反 "scope 隔离不扩大" 红线
- **不选理由**：成本远大于收益。RA `01 §3 OOS-4` 已显式排除。

### 方案 D（备选未选）：Playwright `globalSetup` hook

- **优点**：集中守门，无需每个 spec 调
- **缺点**：隐式控制流，新维护者读 spec 时看不到守门动作 → 调试成本增加；当前 spec 数量少（3 个 file）显式 > 隐式
- **不选理由**：可读性输给方案 A。未来 spec >= 10 个可重新评估。

## 14. Verdict

**READY**

- 单分区（dev-frontend）单文件级改动
- 候选方案 4 个，决策 + 排除理由完整
- AC-1..AC-8 全部对应到本设计的具体动作
- 风险 R-1..R-6 全部有 mitigation 或显式接受
- 零后端 / 零 DB / 零迁移影响
- 可直接进 Stage 3 Gate Review
