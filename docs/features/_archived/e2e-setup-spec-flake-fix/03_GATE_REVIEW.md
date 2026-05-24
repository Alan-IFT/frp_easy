# 03 — Gate Review · T-033 e2e-setup-spec-flake-fix

> Stage 3 输出。Mode: full. Author: gate-reviewer（PM 派发上下文角色化执行；等价 Mode A 自落盘 —— PM 上下文有 Write 工具，直接 author + write 同步）.
> 上游：01 RA verdict=READY；02 Architect verdict=READY.

## Dispatch context audit

- PM 派发上下文工具集：Read / Write / Edit / Glob / Grep（无 Bash / PowerShell / Task / TodoWrite）
- Reviewer 在该上下文角色化执行 —— **Write 可用 → 等价 Mode A 自落盘**
- 未触发 PM_FALLBACK_WRITE sentinel 路径，但 T-034 协议在 PM_LOG.md 中显式记录了 dispatch prompt 内已包含 Two-mode preamble（见 PM_LOG Stage 3 段）

## 实证读盘（reviewer 必读）

| 文件 | 状态 | 引用说明 |
|---|---|---|
| `web/tests/e2e/01-setup.spec.ts` (现状) | 已读 | 20 行，TC-01 line 4-7 / TC-02 line 9-19，与 02 §12 "修后骨架" 改动量精确对齐 |
| `web/tests/e2e/fixtures/auth.ts` (现状) | 已读 | 已有 `setupAccount` / `programmaticLogin` / `bypassWizard` / `programmaticLogout` 四个 export；追加 `assertFreshBackend` / `getBackendReadyStatus` 无冲突 |
| `web/playwright.config.ts` | 已读 | `reuseExistingServer: !process.env.CI`（L26）确认存在；webServer.url=/api/v1/health（L24）确认；fullyParallel=false / workers=1（L4-5）确认 |
| `scripts/start-e2e-server.ps1` | 已读 | tmpdir 用 `[Guid]::NewGuid()`（L48），确认 02 §1 "已正确" 评估 |
| `scripts/start-e2e-server.sh` | 已读 | tmpdir 用 `mktemp -d`（L48），确认平行结构 |
| `internal/httpapi/handlers_setup.go` | 已读 | L23-31 admin != nil → 409，证伪 RA §8 候选根因 #3 |
| `internal/httpapi/handlers_system.go` | 已读（L1-80） | `systemReady` handler L32-48 存在；response shape `{initialized, binMissing, version}` 与 02 §5 设计一致 |
| `web/src/router.ts` L33-85 | 已读 | beforeEach 守卫 L42-43 `!app.initialized && to.path !== '/setup'` → '/setup'，与 TC-01 期望语义一致 |
| `web/src/pages/Setup.vue` L103-122 | 已读 | `handleSubmit` 成功路径调 `router.push('/dashboard')`，失败分支仅 `message.error(...)` 不跳转 —— 与 02 §6 TC-02 流程描述一致 |

## 8-dimension audit

### #1 — Requirement completeness — **PASS**

R-1..R-6 全部可测；AC-1..AC-8 全部可观测验证。OOS-1..OOS-6 明确划定边界。Open questions §10 = 无。RA §8 已实证收敛根因到候选 #4。

### #2 — Design completeness — **PASS**

02 §3 给出新函数完整 signature + 调用位置；§6 顺序图覆盖 TC-01 / TC-02；§12 给出完整伪代码（dev 拷贝即可）；零模糊点。

### #3 — Reuse correctness — **PASS**

02 §7 表格 5 条 reuse audit：
- `app.fetchReady()` 在 `web/src/router.ts` L36-40 确认存在（守卫间接调用）✓
- `systemReady` handler 在 `internal/httpapi/handlers_system.go` L32-48 确认存在 ✓
- `setupAccount` 等 fixture 在 `web/tests/e2e/fixtures/auth.ts` 确认存在（4 export）✓
- `fullyParallel: false` + `workers: 1` 在 `web/playwright.config.ts` L4-6 确认 ✓
没有 reinventing the wheel。

### #4 — Risk coverage — **PASS**

02 §8 列 R-1..R-6 六条风险全部带 mitigation 或显式接受。**额外补充一条 reviewer 视角风险（不阻塞）**：

- **GR-R-A**（reviewer 加注）：`assertFreshBackend` 抛 Error 的字符串里使用 emoji-free 纯文本 + 多行换行（`\n`），需在 Playwright reporter 的 'list' mode 下能正常显示。这是 happy-path（playwright reporter list 默认全文打印 stack），不阻塞，但 dev 实现时确认换行字符是 `\n`（JS string literal）而非 raw `\\n`。

### #5 — Migration safety — **PASS**

02 §9 说明零 schema、零迁移、零 feature flag、零契约破坏 —— migration safety 不适用即等价 PASS（所有要求自动满足）。

### #6 — Boundary handling — **PASS**

01 §4 / 02 §8 R-3 共同覆盖：
- `/api/v1/system/ready` HTTP 5xx（如 ErrCorruptReset） → 02 §12 伪代码独立分支抛 "无法读取后端状态：HTTP <code>"
- `body.initialized=true` → 抛 "前置条件违反" + 修复指引
- `body.initialized=false` → 静默通过（happy path）
- response 非合法 JSON：02 §12 伪代码用 `resp.json() as ...` 会自然 throw（reviewer 注：JS 侧 `resp.json()` 失败会 reject promise，被 await 上抛 → Playwright 报 FAIL）

### #7 — Test feasibility — **PASS**

AC-1..AC-8 全部可测。**AC-5 是最关键的 R-4 实证 acceptance**：QA stage 6 必须做"故意 setup 后跑 TC-01 + 看错误信息"的反向实证，确认错误信息包含"前置条件违反"字面。这一条 reviewer 明确传达给 QA。

### #8 — Out-of-scope clarity — **PASS**

01 §3 OOS-1..OOS-6 + 02 §10 OOS-D1..OOS-D4 两份 OOS 列表互补无冲突；范围边界清晰：不碰后端、不碰路由守卫、不碰 verify_all、不碰 reuseExistingServer 默认值、不引入 reset endpoint。Dev 越界风险低。

## Findings summary

| # | 维度 | 等级 | 说明 |
|---|---|---|---|
| 1 | 需求完整性 | PASS | — |
| 2 | 设计完整性 | PASS | — |
| 3 | Reuse 正确性 | PASS | — |
| 4 | 风险覆盖 | PASS | 补充 GR-R-A 一条 non-blocking 注 |
| 5 | 迁移安全 | PASS | N/A (零迁移) |
| 6 | 边界处理 | PASS | — |
| 7 | 测试可行性 | PASS | AC-5 实证 onus 在 QA stage 6 |
| 8 | OOS 清晰度 | PASS | — |

零 FAIL，零 WARN（GR-R-A 为 reviewer 加注，不计入 WARN 等级）。

## High-probability developer questions（pre-answered）

### Q1 — `page.request.get('/api/v1/system/ready')` 是否需要鉴权？

**答**：不需要。该 endpoint 在 `internal/httpapi/middleware.go` 的中间件链中位于 SessionAuth 之前的公开路径白名单（与 `/api/v1/health`、`/api/v1/setup` 同类）。**证据**：`web/src/router.ts` L36-40 在用户未登录甚至未初始化时即调用 `app.fetchReady()` → 走同款 axios → 同款 endpoint → 一直工作。

### Q2 — 用 `await page.request.get(...)` 还是 `await fetch(...)`？

**答**：用 `page.request.get(...)`。证据：现有 `fixtures/auth.ts` L18 `setupAccount` 的实现就用 `page.request.post('/api/v1/setup', ...)` —— 沿用同款 API 保持一致性。`page.request` 与 `page` 共享 cookie 上下文（fixtures/auth.ts L31 注释明确）。

### Q3 — Error 抛出后 Playwright 报告是否能完整显示多行 stack？

**答**：能。`web/playwright.config.ts` L7 `reporter: 'list'` —— list reporter 在 FAIL 时打印 Error.message 全文（包括 `\n` 换行）。dev 实现确认 `\n` 是 JS string literal 即可。

### Q4 — 是否需要给 `assertFreshBackend` 加 Vitest 单测？

**答**：**不需要**。这是 e2e fixture（运行时依赖真后端 + Playwright runtime），用 Vitest mock 测试价值低；它的"测试"就是 spec 自身在 AC-5 反向实证场景中触发 / 不触发的行为，由 QA 在 stage 6 直接覆盖。dev 不需写额外单测。

### Q5 — 现有 `fixtures/auth.ts` 4 个 export 与新增 2 个是否会有命名冲突？

**答**：不会。grep `web/tests/e2e/fixtures/auth.ts` 现有 export 名：`setupAccount`、`programmaticLogin`、`bypassWizard`、`programmaticLogout` —— 新增 `assertFreshBackend`、`getBackendReadyStatus` 与之无重名。

## 跨 stage 提醒（给 QA stage 6）

QA 必须做的实证：

1. **AC-1 / AC-2 / AC-3** —— 在干净工作树跑 verify_all 至少 1 次（理想 3-5 次连跑），C.1 全部 PASS
2. **AC-4** —— `$env:CI = 'true'` 模拟 CI 模式跑一次，C.1 PASS
3. **AC-5（最关键，R-4 反向实证）** —— 手工构造"已 setup 的后端"场景：
   - 步骤：(a) 跑一次 verify_all 让 TC-02 把后端 setup 了，**保留**那个 frp-easy 进程 (b) 立即再跑 `cd web && npx playwright test 01-setup --project=chromium` → TC-01 必须 FAIL，且错误信息必须包含 "前置条件违反" + "修复指引"字面
   - 这是验证 fix 的根因诊断是否正确的唯一实证手段
4. **AC-6** —— 同机器跑 `scripts/verify_all.ps1` 和 `scripts/verify_all.sh` 双实现，C.1 结论一致
5. **AC-7 / AC-8** —— `git diff` 静态确认 scripts/verify_all.* 字节零变 + spec 不含 `test.skip`/`test.fixme`/`retry`

## Verdict

**APPROVED**

设计可立即进入 Stage 4 development。dev-frontend 单分区接手 `web/tests/e2e/fixtures/auth.ts` + `web/tests/e2e/01-setup.spec.ts` 两文件改动。无 condition、无 blocked。
