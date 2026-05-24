# 01 — Requirement Analysis · T-033 e2e-setup-spec-flake-fix

> Stage 1 输出。Mode: full. Author: requirement-analyst（PM 派发上下文内角色化执行）.

## 1. Goal

修复 `web/tests/e2e/01-setup.spec.ts` 在本地与 verify_all 全流程下偶发 / 长期 FAIL 的问题，让 verify_all C.1（Playwright E2E smoke）在 N≥5 次连续独立运行中稳定 PASS，从而对 verify_all 基线净减一条 FAIL（25P/2F → 26P/1F，剩 E.6 不在本任务 scope）。

## 2. In-scope behaviors（编号、可测）

- **R-1**：`web/tests/e2e/01-setup.spec.ts` 的 TC-01（未初始化访问 `/` 跳转 `/setup`）必须在每次"全新"运行时稳定 PASS。"全新"定义见 R-3。
- **R-2**：`web/tests/e2e/01-setup.spec.ts` 的 TC-02（setup 表单提交后离开 `/setup`）必须在每次运行后稳定 PASS（无论账户是新建还是上一轮残留）。
- **R-3**：当 Playwright 启动 webServer 时，**测试启动前必须**保证后端 DataDir 处于"未初始化"状态（即 admin 表为空）—— 通过 `start-e2e-server.{ps1,sh}` 每次生成新 tmpdir 已经实现，但**与本地复用 webServer 的语义冲突**需被显式处理（具体方案由 Architect 决定）。
- **R-4**：TC-01 / TC-02 失败时的错误信息必须**直接指明前提条件违反**（例：当前后端 initialized=true 但 TC-01 期望 initialized=false → 报错文本应包含 "前置条件失败：后端已初始化"），而非仅"URL 不匹配 /\/setup/"。
- **R-5**：`verify_all` 在 PowerShell 和 Bash 两套实现中对 C.1 的判定字面与超时**严格一致**（继承 insight L67 双实现对账约束）。
- **R-6**：所有改动必须在本地 + verify_all 两套入口稳定可重入：手工 `npx playwright test --project=chromium` 与 `scripts/verify_all` 内部调用路径行为对齐。

## 3. Out-of-scope（明确不做）

- **OOS-1**：**不修** verify_all E.6（已归档 06_TEST_REPORT.md 标题违规）。该 FAIL 在本任务后仍残留是预期。
- **OOS-2**：**不**改 TC-03 / TC-04 / TC-05 测试本身（02-auth.spec.ts / 03-dashboard.spec.ts）的实现，除非它们与 R-3 共享同款前置条件 fix（如共享 fixture）—— 如果共享，由 Architect 决策是否一起改。
- **OOS-3**：**禁** `test.skip` / `test.fixme` / `retry: 3` 静默掩盖（用户已显式禁止）。
- **OOS-4**：**不**改后端 `/api/v1/setup` endpoint 的语义（PM desk-read 已证伪根因 #3：admin 已存在 → 409，幂等）。
- **OOS-5**：**不**改 `start-e2e-server.{ps1,sh}` 的"独立 tmpdir + 独立 DataDir"模式（PM desk-read 已证伪根因 #2：每次 `[Guid]::NewGuid()` / `mktemp -d`，data.db 不残留）。
- **OOS-6**：**不**改前端路由守卫 `router.beforeEach`（与 TC-01 行为契合，证据见 `web/src/router.ts` L43-45）。

## 4. Boundary conditions

| 边界 | 行为 |
|---|---|
| webServer 已被前一次本地 `vite dev` / `playwright test --headed` 启动并占用 7800 端口 | 由 `reuseExistingServer: !process.env.CI`（playwright.config.ts L26）决定 —— 非 CI 本地复用。**这是本任务最核心的边界条件**。Architect 必须明确：是改 `reuseExistingServer` 默认值，还是让测试侧主动复位状态。 |
| webServer 启动时后端 build 失败 / dist/ 不存在 | start-e2e-server 自重建。失败时 Playwright 直接拿不到 `/api/v1/health` 200 → webServer.timeout=120s 后整轮 FAIL。本任务前序假设 build 正常。 |
| 测试运行时 admin 表已经被 TC-02 写入，**同一轮**之后 TC-01 不会再跑（test 文件 case 顺序固定且 fullyParallel=false / workers=1 已确认）—— **同轮内**不会自污染。**跨轮**才出问题。 |
| 用户本地 IDE 开着 vite dev server + 同 DataDir → webServer 复用 → initialized=true → TC-01 FAIL | 这是用户日常工作流的真实路径。验证手段必须覆盖。 |
| CI 环境（`process.env.CI=true`） | reuseExistingServer=false → 每次都启全新 webServer + 全新 tmpdir → TC-01 永远 PASS（CI 不会复现 flake）。但本地用户日常会复现。 |
| TC-02 成功后留下 admin → 下次 TC-01 复用 server 时 initialized=true → TC-01 fail | 这是**自污染**的可能路径（如果同 server 反复跑测试）。 |

## 5. Acceptance criteria（可验证）

- **AC-1**：在本地工作树（用户已开 vite dev server 或上一次 webServer 仍存活的场景）跑 `cd web && npx playwright test --project=chromium`，TC-01 + TC-02 均 PASS。N≥3 次连续手工跑全 PASS。
- **AC-2**：在"冷启动"工作树（无任何 dev server，bin/frp-easy.exe 可能不存在）跑 `scripts/verify_all`，C.1 PASS。
- **AC-3**：连续跑 5 次 `scripts/verify_all`（不 quick），每次 C.1 都 PASS，无 flake。
- **AC-4**：CI 环境模拟（设置 `$env:CI = 'true'`）跑 `cd web && npx playwright test --project=chromium`，C.1 PASS。
- **AC-5**：故意打破前提条件（例：先手工 `curl -X POST http://127.0.0.1:7800/api/v1/setup -d '{"username":"e2eadmin","password":"E2eTestPass1!"}'` 让后端 initialized=true，然后跑 TC-01）→ **TC-01 必须 FAIL，且 FAIL 信息直接指出 "前置条件违反：后端 initialized=true"**（R-4 实证）。
- **AC-6**：verify_all PowerShell 与 Bash 双实现的 C.1 在同一台机器上结论一致（同 PASS / 同 FAIL）。
- **AC-7**：跑完本任务交付后 `git diff scripts/verify_all.{ps1,sh}` 字节差 ≤ 现状（如果不改 verify_all 则 = 0；如果改也必须双实现同步）。
- **AC-8**：跑完本任务交付后 `web/tests/e2e/01-setup.spec.ts` 不含 `test.skip` / `test.fixme` / `retry` 等掩盖字面。

## 6. Non-functional requirements

- **NFR-1**：测试单轮运行时间不超过当前 baseline 的 +20%（当前约 30s 范围，允许至 36s）。
- **NFR-2**：测试本地非 CI 跑跑跑也不应产生跨工作树副作用（例：不应在用户 `$HOME` 或仓库根写持久化文件）。已由 tmpdir 模式保证；本任务不引入回归。
- **NFR-3**：fix 后的 spec 在 ts 类型层面必须通过 `vue-tsc --noEmit`（Vitest 不强制，但项目约定）。

## 7. Related tasks（历史）

| 历史任务 | 关系 | 应读文件 |
|---|---|---|
| **T-006 e2e-smoke-tests** | 直接前序：本测试套件的引入任务 | `docs/features/_archived/e2e-smoke-tests/02_SOLUTION_DESIGN.md` + `01_REQUIREMENT_ANALYSIS.md`；MEMORY 提及 "NMessageProvider 必须在 App.vue；go build 嵌入静态快照需时间戳重建" |
| **T-009 polish-pass** | 引入 `scripts/start-e2e-server.ps1`（与 .sh 平行） | `docs/features/_archived/polish-pass/02_SOLUTION_DESIGN.md §2` |
| **T-031 install-ps1-host-close-on-completion** | 引入 verify_all E.8/E.9/E.10 静态闸门（双实现对账模式参考） | `docs/features/_archived/install-ps1-host-close-on-completion/02_SOLUTION_DESIGN.md` |
| **T-032 proxy-form-vmodel-oom-fix** | **本任务被用户用作非 T-032 引入的证据**（3 次 git stash 对照） | `docs/features/_archived/proxy-form-vmodel-oom-fix/06_TEST_REPORT.md §5.3` |
| **T-034 reviewer-write-tool-dispatch-verify** | **本任务必须使用其双模式 reviewer 协议** | `.harness/agents/pm-orchestrator.md` "Reviewer dispatch protocol" 段 |

## 8. Root-cause hypothesis evidence-based pruning（必做实证）

PM desk-read 已逐条扫描候选根因。RA（本文档作者）确认证据如下：

### 候选根因 #1：Playwright webServer 启动竞态

**状态：不太可能（低置信）**。证据：
- `playwright.config.ts` L24 `url: 'http://127.0.0.1:7800/api/v1/health'` —— Playwright 在 `url` 200 之前不会启测。
- `start-e2e-server.ps1` 同步 build → 同步启 `& $BIN`；后端 main.go ready 后挂 `/api/v1/health` 200。
- `webServer.timeout: 120_000`（120s）—— 启动竞态最可能表现是 webServer 超时 FAIL，而不是 spec 内 `expect(page).toHaveURL(/\/setup/)` FAIL。
- 用户报告的 FAIL 落在 spec assertion 上（非 webServer timeout）→ 启动竞态被这一类 symptom 反证。

### 候选根因 #2：data.db fixture 残留（同 server 多轮跑）

**状态：不太可能（中-低置信）**。证据：
- `start-e2e-server.ps1` L48-50 用 `[Guid]::NewGuid().ToString("N")` 创独立 tmpdir。
- `start-e2e-server.sh` L48 `mktemp -d` 同款机制。
- 每次 webServer **冷启动**用全新 DataDir → admin 表必空 → initialized=false。
- **但**：`reuseExistingServer: !process.env.CI`（playwright.config.ts L26）打破此前提 —— **复用**已存活 server 意味着也复用了它启动时的 tmpdir + 其内已有的 admin（TC-02 写过）。**所以本根因实际上是根因 #4 的子集**。

### 候选根因 #3：后端 setup endpoint 已初始化时不跳

**状态：被证伪（高置信）**。证据：
- `internal/httpapi/handlers_setup.go` L23-31：`admin != nil` → `409 ALREADY_INITIALIZED`。
- `web/src/pages/Setup.vue` L116：错误分支 `message.error(...)`，**不**做 router.push。
- 提交 setup 已初始化的服务器 → 409 → Setup.vue 显示 error toast → 用户停留在 `/setup` → TC-02 报错"还在 /setup"。**但这是 TC-02 的失败模式，不是 TC-01。**
- TC-01 完全不调 setup endpoint，只 `page.goto('/')` + URL assert。所以 setup endpoint 行为与 TC-01 失败无关。

### 候选根因 #4（PM 新加，置信最高）：webServer 复用 + 跨轮 admin 残留

**状态：极可能（高置信）**。

完整因果链：
1. 用户本地工作流：第一次跑 `verify_all` → Playwright 启 webServer（无 CI env）→ 创独立 tmpdir A → 跑 TC-01（initialized=false → PASS）→ 跑 TC-02（成功 setup，**tmpdir A 内 admin 表 = 1 行**）→ webServer 在测试结束后是否被 SIGTERM 取决于 Playwright 实现；**实际上**：playwright `webServer` 在 test runner 退出时关闭。
2. 用户第二次跑 `verify_all`：webServer 重新启 → **新的 tmpdir B（GUID 唯一）** → admin 表又空 → TC-01 又 PASS。**所以单纯连跑 verify_all 不会复现**。
3. **关键复现路径**：用户**同时**开着 `vite dev` 或上一轮 `playwright test --headed --debug` 残留的 frp-easy.exe 实例占着 7800 端口 → 第二次跑 verify_all 时 Playwright `reuseExistingServer=true` 决定**复用**已有 7800 → 它的 tmpdir 是**上一轮的 A**（含 admin）→ TC-01 报"URL 不在 /setup"。

**这个路径很匹配用户描述的"长期 FAIL"**——用户开发时 IDE 经常开着 dev server 或上次跑剩的进程。CI 永远不复现是因为 CI=true → reuseExistingServer=false → 每轮全新。

**反向证据**：如果**没有**任何残留 7800 进程，Playwright 会启全新 webServer → 全新 tmpdir → TC-01 PASS。所以本根因可以被"先 kill 7800 占用 + 跑 verify_all"验证证伪。

### 候选根因 #5（次置信）：测试侧自污染（同 server 内 TC-02 写完 admin 后某种情况下 TC-01 在下一文件中又跑）

**状态：极不可能**。`fullyParallel: false` + `workers: 1` + 01-setup.spec.ts 内顺序固定（TC-01 先 TC-02 后）→ 不会自污染同轮。

## 9. 建议的"如何证明修好了"实证手段（给 Architect / QA）

- **手段 A：复现脚本**。在 RA 派发完成后由 dev 或 QA 写一段 10 行复现脚本：(1) kill 任何 7800 占用 (2) 起一个长寿命 frp-easy.exe + tmpdir + 写入 admin (3) `cd web && npx playwright test 01-setup --project=chromium`。**修复前必须 FAIL（证伪），修复后必须 PASS（证毕）。** 这是 R-4 错误信息明确化要求的实证。
- **手段 B：N=5 连续跑**。`for ($i=1; $i -le 5; $i++) { scripts/verify_all.ps1; if ($LASTEXITCODE -ne 0) { Write-Host "FAIL on iter $i"; break } }`，全 PASS 即 AC-3 满足。
- **手段 C：CI 模拟跑**。`$env:CI = 'true'; cd web; npx playwright test 01-setup --project=chromium` 应永远 PASS（CI 路径不复现 flake，证明 fix 没引入 CI 回归）。

## 10. Open questions for user

**无**。所有候选根因已实证收敛到 #4。具体 fix 路径属于 Architect 职责（候选方向：让测试 fixture 在 TC-01 前主动重置后端状态 / 改 reuseExistingServer 默认为 false / setup spec 用专属 fresh server 实例 / 测试侧给出更明确的失败信息 —— 至少 2 个候选要在 02_SOLUTION_DESIGN.md 给出取舍）。

## 11. Verdict

**READY**

- 根因收敛到候选 #4（webServer 复用 + 跨轮 admin 残留）+ 候选 #2 子集
- 所有 in-scope behavior 可测、可验证
- 没有未决问题阻塞 Architect

---

## 附：本任务必须读的 insight-index 条目

- **L33** (2026-05-24, T-032): "verify_all 在多任务并行进行的工作树中"非本任务 fail" 归责黄金动作 = `git stash` 暂存窄路径文件 → 裸跑 verify_all → 对照 Summary 数字"
  - 对本任务的意义：架构师 / dev / QA 必须复用此动作对照"修前 / 修后" C.1 计数
- **L34-L37** (2026-05-24, T-034): SDK 派发上下文工具裁剪 + 双模式 reviewer 协议
  - 对本任务的意义：Stage 3 / Stage 5 dispatch prompt 必须包含 "Two-mode output reminder" preamble；接受 Mode B 时 PM 字节级落盘不重写
- **L67** (2026-05-24, T-031): "verify_all 双实现（PowerShell + Bash）regex 锚定 / multiline 标志 / 输入模式严格对账，否则同一闸门可能两侧结论分裂"
  - 对本任务的意义：如果 fix 路径触及 verify_all，必须双侧改并双侧验证
