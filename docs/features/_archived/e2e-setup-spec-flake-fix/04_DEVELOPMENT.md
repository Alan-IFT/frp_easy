# 04 — Development Record · T-033 e2e-setup-spec-flake-fix

> Stage 4 输出。Mode: full. Partition: **dev-frontend** (owns `web/**`).
> Author: dev-frontend（PM 派发上下文角色化执行）.
> 上游：03 GR verdict=APPROVED.

## Partition

**dev-frontend** — owns: `web/**`

零越界。两文件改动全部位于 `web/tests/e2e/` 子树下。

## Files changed (this partition only)

| 文件 | 操作 | LoC delta（约） |
|---|---|---|
| `web/tests/e2e/fixtures/auth.ts` | edit（追加 2 个 export：`assertFreshBackend` + `getBackendReadyStatus`） | +55 |
| `web/tests/e2e/01-setup.spec.ts` | edit（import 新 fixture + TC-01/TC-02 起首调 `assertFreshBackend` + 2 行中文注释） | +4（净，原 20 行 → 26 行） |

## 实施步骤

### 1. 扩展 `web/tests/e2e/fixtures/auth.ts`

按 02 §12 完整伪代码字节级落盘 `assertFreshBackend` 和 `getBackendReadyStatus` 两个 export。位置：紧接在 `const E2E_USERNAME / E2E_PASSWORD` 常量之后、原 `setupAccount` 之前 —— 让前置守门类 helper 在文件顶部，与后续 setup / login / wizard 类业务 helper 形成层次。

JSDoc 完整覆盖：
- 函数职责与背景（T-033 引用 + 触发场景）
- 调用位置与代价（< 50ms）
- 鉴权要求（无，引用 router.ts L36-40 间接证据）
- 失败行为（Error.message 多行换行被 Playwright list reporter 原样打印）

`assertFreshBackend` 三分支：
- `!resp.ok()` → throw "前置条件检测失败：HTTP <code>" + 引导查 start-e2e-server.{ps1,sh}
- `body.initialized=true` → throw "前置条件违反" + 3 步修复指引（Windows / Linux / CI mode）
- 其余（initialized=false 200） → 静默返回

`getBackendReadyStatus`（无副作用查询版）保留备用。

### 2. 改 `web/tests/e2e/01-setup.spec.ts`

按 02 §12 修后骨架字节级落盘：
- import 段追加 `import { assertFreshBackend } from './fixtures/auth'`
- TC-01 第一行 `await assertFreshBackend(page)` + 中文注释 `// T-033：守门...`
- TC-02 第一行 `await assertFreshBackend(page)` + 中文注释 `// T-033：与 TC-01 同款守门...`
- 原断言（`page.goto('/')` + `expect(page).toHaveURL(/\/setup/)` 等）零改动

### 3. 无其他改动

零后端 / 零 DB / 零 verify_all / 零 playwright.config / 零 router / 零 store / 零 component 改动 —— 严格符合 02 §11 partition assignment 表。

## verify_all baseline 对照

PM 派发上下文工具集（Read/Write/Edit/Glob/Grep）不含 Bash/PowerShell，**dev 角色无法在本上下文跑 `scripts/verify_all`**。这是 T-034 已确证项目级事实。verify_all 调用 deferred 至 PM_LOG.md 的"DECLARE_DONE checklist"由 batch caller / stop-hook 跑。

预测的 baseline 对照（基于静态分析）：

| Step | 修前 | 修后预测 | 理由 |
|---|---|---|---|
| A.* (build / typecheck) | PASS | PASS | TS 改动严格符合 `Promise<void>` / `Promise<{...}>` 类型签名；现有 `@playwright/test` Page 类型已 import |
| B.* (vitest unit) | PASS | PASS | 零 src 改动，零 unit 测试影响 |
| **C.1 (playwright e2e)** | **FAIL** | **PASS**（fresh 工作树）/ **FAIL 但带明确根因信息**（污染工作树） | fix 的核心目标。完整实证由 QA stage 6 做 |
| D.* (schema / contract) | PASS | PASS | 零 openapi 改动 |
| E.* (lint / style / encoding) | PASS（除 E.6 已知）| PASS（除 E.6） | spec 文件无 BOM 限制；fixture 文件无新增编码差异 |
| **E.6 (archived doc title)** | **FAIL** | **FAIL（不变，OOS-1）** | 本任务显式不修 |
| F.* | PASS | PASS | — |
| G.1 / G.2 (T-034 reviewer protocol gates) | PASS | PASS | 本任务不动 reviewer agent / pm-orchestrator agent |

**baseline 净变化预测：FAIL 数 2 → 1（C.1 由 FAIL 转 PASS；E.6 不变）**

## Out-of-partition coordination

无。本任务单分区单 stage 完成；无跨分区依赖。

## DESIGN DRIFT log

无漂移。所有改动严格按 02 §12 伪代码 + §11 partition 表执行。

## 实施过程中的非平凡发现

- 无意外。02 §12 给出完整伪代码 + §6 顺序图 + §8 R-3 已枚举 5xx 分支处理 → dev 在落码时零设计决策。
- 唯一微调：JSDoc 中"修复指引"步骤的 PowerShell 命令写为 `Get-Process | Where-Object { $_.Path -like "*frp-easy*" } | Stop-Process -Force` —— 比 02 §12 伪代码的 `Stop-Process` 更具体且与 frp-easy 二进制名匹配，**这是 JSDoc 文本完善而非设计漂移**，落地结果对 spec 行为零影响。

## Adversarial tests（dev 自检）

- **ADV-1**：读已落盘文件 `web/tests/e2e/fixtures/auth.ts` L1-60，确认 `assertFreshBackend` export 存在且 signature `(page: Page) => Promise<void>` 正确 → ✓
- **ADV-2**：读已落盘文件 `web/tests/e2e/01-setup.spec.ts` L2，确认 `import { assertFreshBackend } from './fixtures/auth'` 路径相对正确（`./fixtures/auth` 同目录） → ✓
- **ADV-3**：grep `test.skip|test.fixme|retry:` 在 `web/tests/e2e/01-setup.spec.ts` 应为 0 命中（OOS-3 / AC-8） → 由 grep 在 Stage 5 / Stage 6 验证；本 dev 阶段静态目读未引入这些字面
- **ADV-4**：grep 现有 `fixtures/auth.ts` 4 个 export 名（`setupAccount`/`programmaticLogin`/`bypassWizard`/`programmaticLogout`）与新增 2 个（`assertFreshBackend`/`getBackendReadyStatus`）零冲突 → 已 desk-confirm
- **ADV-5**：TC-01 是否仍验证原核心 invariant `await expect(page).toHaveURL(/\/setup/)`？读修后 01-setup.spec.ts L9 → ✓ 原断言保留

## Verdict

**READY FOR REVIEW** (frontend partition complete)

dev 落码完成。两文件改动落地。verify_all 实证 deferred 到 QA stage 6（PM 派发上下文工具裁剪不可达）。等待 Stage 5 code review。
