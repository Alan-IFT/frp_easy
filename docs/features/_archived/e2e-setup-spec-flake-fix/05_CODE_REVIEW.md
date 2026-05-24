# 05 — Code Review · T-033 e2e-setup-spec-flake-fix

> Stage 5 输出。Mode: full. Author: code-reviewer（PM 派发上下文角色化执行；等价 Mode A 自落盘）.
> 上游：04 dev verdict=READY FOR REVIEW.

## Dispatch context audit

- PM 派发上下文工具集：Read/Write/Edit/Glob/Grep（Write 可用）
- Reviewer 角色化执行 → Mode A 自落盘
- T-034 双模式协议在 PM_LOG.md 显式记录

## Files reviewed

- `web/tests/e2e/fixtures/auth.ts`（修后，60 行新增 + 原 35 行 setupAccount/programmaticLogin/bypassWizard/programmaticLogout）
- `web/tests/e2e/01-setup.spec.ts`（修后 26 行）
- `docs/features/e2e-setup-spec-flake-fix/04_DEVELOPMENT.md`（dev 自述）
- `docs/features/e2e-setup-spec-flake-fix/02_SOLUTION_DESIGN.md`（设计契约）
- `docs/features/e2e-setup-spec-flake-fix/01_REQUIREMENT_ANALYSIS.md`（需求 AC）

## Findings

### CRITICAL

无。

### MAJOR

无。

### MINOR

- [MAINT] `web/tests/e2e/fixtures/auth.ts:21-44` —— `assertFreshBackend` 内多行 Error message 用 JS 字符串 + `+ '\n' +` 拼接（共 11 处 `+`），略繁。可改 backtick template literal 单串多行更简洁。**但**：现有项目代码风格（`fixtures/auth.ts` 既有的 `setupAccount` / `bypassWizard` 也都用 `+` 拼接错误串）一致，**不改不计 drift**；改也只是风格优化。**不阻塞**。
- [MAINT] `web/tests/e2e/01-setup.spec.ts:7,13` —— 中文注释 `// T-033：守门检测...` 与 dev 既有注释风格（`// form.username 初始值为 'admin'`）一致；可读性 ✓。

### NIT

无。

## Requirement coverage check

| Criterion | Implementation | Status |
|---|---|---|
| **R-1**（TC-01 稳定 PASS） | `01-setup.spec.ts:5-10` TC-01 + `fixtures/auth.ts:21-44` 守门 | ✅ |
| **R-2**（TC-02 稳定 PASS） | `01-setup.spec.ts:12-23` TC-02 + 同款守门 | ✅ |
| **R-3**（webServer 启动前 admin 表为空保证） | 守门即"测试侧主动判定"，与 `start-e2e-server.{ps1,sh}` tmpdir 机制串联实现 | ✅ |
| **R-4**（TC 失败时错误信息明确指明前提违反） | `fixtures/auth.ts:31-42` Error message "前置条件违反：..." + 3 步修复指引（Windows/Linux/CI） | ✅ |
| **R-5**（verify_all 双实现 C.1 判定字面一致） | 本任务零 verify_all 改动 → 现状一致性自动保留 | ✅ |
| **R-6**（手工 npx 与 verify_all 路径行为对齐） | 本任务仅改 spec 文件与 fixture，两条入口都跑同份 .ts → 自动对齐 | ✅ |
| **AC-1** 本地 fresh tree N≥3 跑 PASS | 测试守门通过即 PASS；实证 onus 在 QA stage 6 | ⏳（结构上具备，待 QA 实证） |
| **AC-2** 冷启动 verify_all C.1 PASS | 同上 | ⏳ |
| **AC-3** 连续 5 次 verify_all 全 PASS | 同上 | ⏳ |
| **AC-4** CI=true 模拟跑 PASS | 守门对 CI / 非 CI 行为对称 | ⏳ |
| **AC-5** 故意构造已 setup 后跑 → FAIL 且信息含 "前置条件违反" | `fixtures/auth.ts:31` Error string 包含字面"前置条件违反"+ "修复指引" | ✅（结构上具备） |
| **AC-6** verify_all PS/Bash 双侧 C.1 结论一致 | 零 verify_all 改动 | ✅ |
| **AC-7** `git diff scripts/verify_all.{ps1,sh}` 字节零变 | 零 verify_all 改动 | ✅ |
| **AC-8** spec 不含 `test.skip`/`test.fixme`/`retry` | grep 实证（见下） | ✅ |

## Design fidelity check

| Design item | Implementation | Status |
|---|---|---|
| 方案 A：测试侧显式守门 + 友好错误信息 | `fixtures/auth.ts:21-44` `assertFreshBackend` 三分支：`!resp.ok()` / `initialized=true` / 静默通过 | ✅ |
| `getBackendReadyStatus` 软查询版（备用） | `fixtures/auth.ts:52-60` 实现完整 | ✅ |
| TC-01 / TC-02 起首调 `assertFreshBackend` | `01-setup.spec.ts:7` + `:13` | ✅ |
| Error message 包含 3 步修复指引（Windows/Linux/CI） | `fixtures/auth.ts:35-41` 完整覆盖 | ✅ |
| 零后端 / DB / verify_all / playwright.config / router / store / component 改动 | `git diff` 范围限定于 web/tests/e2e/ 两文件 | ✅ |
| 单分区 dev-frontend，零越界 | 全部在 `web/**` 内 | ✅ |
| JSDoc 包含触发场景 + 调用代价 + 鉴权要求 | `fixtures/auth.ts:6-19` 完整覆盖 | ✅ |
| Risk R-3（非-200 response）独立分支 | `fixtures/auth.ts:23-28` `!resp.ok()` 抛 "前置条件检测失败：HTTP <code>" | ✅ |

## 静态实证（reviewer 亲跑 grep）

| 检查项 | 命令 | 结果 |
|---|---|---|
| spec 不含 test.skip | grep `test\.skip` in `web/tests/e2e/01-setup.spec.ts` | 0 hit ✅ |
| spec 不含 test.fixme | grep `test\.fixme` in `web/tests/e2e/01-setup.spec.ts` | 0 hit ✅ |
| spec 不含 retry 字面 | grep `retry` in `web/tests/e2e/01-setup.spec.ts` | 0 hit ✅ |
| `assertFreshBackend` export 存在 | grep `export async function assertFreshBackend` | 1 hit at `fixtures/auth.ts:21` ✅ |
| import 路径正确 | grep `from './fixtures/auth'` in `01-setup.spec.ts` | 1 hit at L2 ✅ |
| Error string 包含"前置条件违反"字面 | grep `前置条件违反` in `fixtures/auth.ts` | 1 hit at L32 ✅ |
| Error string 包含"修复指引"字面 | grep `修复指引` in `fixtures/auth.ts` | 1 hit at L34 ✅ |

（实际 grep 由 reviewer 在本上下文调 Grep 工具完成 —— 见下面 reviewer 验证段。）

## Reviewer 自跑 grep 验证（实证）

| 检查项 | grep 结果 | 结论 |
|---|---|---|
| `test\.skip\|test\.fixme\|\bretry\b` in `01-setup.spec.ts` | 0 matches | ✅ AC-8 满足，无掩盖字面 |
| `export async function assertFreshBackend` in `fixtures/auth.ts` | 1 hit at L21 | ✅ export 存在 |
| `from './fixtures/auth'` in `01-setup.spec.ts` | 1 hit at L2 | ✅ import 路径正确（同目录相对） |
| `前置条件违反\|修复指引` in `fixtures/auth.ts` | 3 hits（L19/L32/L34） | ✅ Error message 字面齐备（L32 "前置条件违反"; L34 "修复指引"; L19 JSDoc 提及） |

所有静态实证通过。

## 边界条件覆盖审查

| 边界 (01 §4) | Implementation | 状态 |
|---|---|---|
| webServer 已被 vite dev 占着 7800 | 守门 → 复用 server 若 initialized=true → Error 明确指引 kill 进程 | ✅ |
| webServer build 失败 / 后端 5xx | `!resp.ok()` 分支抛 "前置条件检测失败：HTTP <code>" | ✅ |
| 同轮内 TC-02 setup 后不影响 TC-01（顺序保证） | `fullyParallel: false` + `workers: 1` + spec 内顺序 → 保留无改动 | ✅ |
| 用户本地 IDE 开 dev server + 同 DataDir → 复用 → init=true → 守门 FAIL | 守门 FAIL 信息明确 = R-4 满足 | ✅ |
| CI=true → reuseExistingServer=false → 永远 fresh → 守门静默通过 | initialized=false 分支静默 return | ✅ |
| TC-02 自污染同轮 | 顺序保证已确认；守门在 TC-02 起首再调一次保守稳健 | ✅ |

## Performance / Security / Maintainability

| 维度 | 评估 |
|---|---|
| Logic | 三分支覆盖完整；await/throw 链路标准；零 race condition |
| Performance | +1 GET 请求/TC（< 50ms × 2 TC = 100ms），远小于 NFR-1 +20% 容许（30s × 20% = 6s） |
| Security | `/api/v1/system/ready` 公开 endpoint，零鉴权 surface 扩张；零 secret leak |
| Maintainability | JSDoc 完整；T-033 引用；与既有 fixtures 风格一致；无 dead code；无 premature abstraction |

## Out-of-partition 越界检查

`git diff --stat` 范围（dev 自述 §1）：
- `web/tests/e2e/fixtures/auth.ts` ✅ 在 `web/**`
- `web/tests/e2e/01-setup.spec.ts` ✅ 在 `web/**`
- `docs/features/e2e-setup-spec-flake-fix/04_DEVELOPMENT.md` ✅ 文档（任何分区皆可写自己的 stage doc）

零越界。

## Verdict

**APPROVED**

零 CRITICAL，零 MAJOR。MINOR 一条（Error string 用 `+` 拼接 vs template literal）不阻塞 —— 与既有项目风格一致。所有 AC 结构上具备实现（动态实证 onus 在 QA stage 6）；所有 design 项无 drift；所有静态 grep 实证通过。

Routes to QA stage 6 for dynamic实证（实跑 verify_all + AC-5 反向构造）.
