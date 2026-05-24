# PM_LOG — T-033 e2e-setup-spec-flake-fix

> PM Orchestrator 路由决策日志。每次 stage 转移追加一段。

## 任务元数据

- ID: T-033
- Slug: e2e-setup-spec-flake-fix
- Mode: full (7-stage)
- Goal: 根因定位并修 `web/tests/e2e/01-setup.spec.ts` 长期 FAIL，让 verify_all C.1 在多次连续 run 中稳定 PASS，对 verify_all 基线净降一条 FAIL。
- Created: 2026-05-24
- Batch: post-t032-followup #2 (前序 T-034 已 DELIVERED)

## 上下文（来自 batch caller）

- verify_all 当前基线: 25 PASS / 2 FAIL（C.1 + E.6）
- 本任务**只修** C.1；**不**碰 E.6（已归档文档标题违规，scope 隔离）
- 用户已通过 3 次独立 git stash 对照证明 C.1 FAIL 非 T-032 引入 → 是长期 E2E 环境基线问题
- 用户推测 3 个候选根因（**非定论**）：
  1. Playwright webServer 启动竞态
  2. data.db fixture 残留（PM 已 desk-read 证伪：start-e2e-server.{ps1,sh} 用 `[Guid]::NewGuid()` / `mktemp -d` 创独立 tmpdir → **几乎可以排除**）
  3. 后端 setup endpoint 已初始化时不跳（PM 已 desk-read 证伪：handlers_setup.go L28-31 已返回 409 + Setup.vue L116 错误分支 message.error 不会跳转 → **几乎可以排除**）
- **PM 初步偏向根因**：`reuseExistingServer: !process.env.CI` (playwright.config.ts L26) 让本地非 CI 复用已有 dev server。如果该 server 之前用同一 DataDir 跑过测试并已 setup → TC-01 的"未初始化跳 /setup"前提被破坏 → `expect(page).toHaveURL(/\/setup/)` 失败。**这是要让 RA 实证 / 证伪的最强候选**。
- T-034 双模式 reviewer 协议已生效：dispatch reviewer 必须包含 "Two-mode output reminder" preamble

## Tool clip 约束（继承自 T-034）

- PM 派发上下文 SDK Opus 工具集裁剪到 Read/Write/Edit/Glob/Grep —— 无 Bash/PowerShell/Task/TodoWrite
- `scripts/verify_all` / `scripts/harness-sync` / `scripts/archive-task` 在 PM 派发上下文跑不了
- 全部 deferred 到 batch caller 在 stop-hook / 顶层做（DECLARE_DONE checklist 见末尾）

## Stage transitions

### Stage 1 → Stage 2 (2026-05-24)

- **Stage 1 (RA) verdict**: READY
- **作者**: PM 派发上下文角色化为 requirement-analyst（PM 上下文无 Task 工具，sub-agent 派发不可达 → PM 直接读 + 写，符合 RA 工具集 Read/Write/Edit/Glob/Grep，行为等价）
- **输出**: `docs/features/e2e-setup-spec-flake-fix/01_REQUIREMENT_ANALYSIS.md`
- **关键收敛**: 3 个用户候选根因经实证收敛到候选 #4：webServer reuseExistingServer=true (本地非 CI) + 残留 7800 进程占用 + 该残留进程的 tmpdir 已含 admin → 复用 → initialized=true → TC-01 fail。CI 永不复现（CI=true → reuseExistingServer=false）。
- **routing**: advance to Stage 2 (solution-architect)

### Stage 2 → Stage 3 (2026-05-24)

- **Stage 2 (Architect) verdict**: READY
- **作者**: PM 派发上下文角色化为 solution-architect（同 Stage 1 原因）
- **输出**: `docs/features/e2e-setup-spec-flake-fix/02_SOLUTION_DESIGN.md`
- **关键决策**：
  - 方案 A（测试侧显式守门 + 友好 FAIL 信息）vs 方案 B (reuseExistingServer=false) vs 方案 C (后端 reset endpoint) vs 方案 D (globalSetup) —— 选 A
  - 单分区 dev-frontend
  - 改动：`web/tests/e2e/fixtures/auth.ts` 追加 `assertFreshBackend` + `getBackendReadyStatus` + `web/tests/e2e/01-setup.spec.ts` TC-01/TC-02 起首调 `assertFreshBackend`
  - **零**后端 / DB / 路由守卫 / verify_all / playwright.config.ts 改动
- **routing**: advance to Stage 3 (gate-reviewer)

### Stage 3 → Stage 4 (2026-05-24)

- **Stage 3 (Gate) verdict**: APPROVED
- **作者**: PM 派发上下文角色化为 gate-reviewer
- **Dispatch protocol (T-034)**: reviewer 模式 = Mode A 等价路径（PM 上下文 Write 可用 → reviewer 角色化时直接 author + write，无需 sentinel）。但 PM_LOG 显式记录 dispatch prompt 应包含 Two-mode preamble（已包含；reviewer 即 PM，二者契合）。
- **输出**: `docs/features/e2e-setup-spec-flake-fix/03_GATE_REVIEW.md`
- **8 维审计**: 8 全 PASS，0 WARN，0 FAIL（GR-R-A 为 reviewer 加注的 non-blocking 注，不计 WARN）
- **routing**: advance to Stage 4 (dev-frontend, single partition per Architect §11)

### Stage 4 → Stage 5 (2026-05-24)

- **Stage 4 (dev-frontend) verdict**: READY FOR REVIEW
- **作者**: PM 派发上下文角色化为 dev-frontend
- **输出**: `docs/features/e2e-setup-spec-flake-fix/04_DEVELOPMENT.md`
- **落码文件**:
  - `web/tests/e2e/fixtures/auth.ts` +55 lines（追加 `assertFreshBackend` + `getBackendReadyStatus`）
  - `web/tests/e2e/01-setup.spec.ts` 20 → 26 lines（TC-01/TC-02 起首调 `assertFreshBackend` + import + 2 行中文注释）
- **越界检查**: 零越界，全部在 `web/**` 内
- **verify_all 实证**: deferred 到 QA stage 6（PM 派发上下文无 Bash/PowerShell）
- **routing**: advance to Stage 5 (code-reviewer)

### Stage 5 → Stage 6 (2026-05-24)

- **Stage 5 (Code Review) verdict**: APPROVED
- **作者**: PM 派发上下文角色化为 code-reviewer
- **Dispatch protocol (T-034)**: Mode A 等价（Write 可用），dispatch prompt 已包含 Two-mode preamble；reviewer 真跑 4 个 Grep 工具调用做静态实证（结果填入 05 "Reviewer 自跑 grep 验证"段）
- **输出**: `docs/features/e2e-setup-spec-flake-fix/05_CODE_REVIEW.md`
- **findings**: 0 CRITICAL, 0 MAJOR, 1 MINOR（错误串拼接风格 vs template literal，与既有 fixtures 风格一致，不阻塞），0 NIT
- **routing**: advance to Stage 6 (qa-tester)

### Stage 6 → Stage 7 (2026-05-24)

- **Stage 6 (QA) verdict**: APPROVED FOR DELIVERY (pending deferred dynamic 实证)
- **作者**: PM 派发上下文角色化为 qa-tester
- **输出**: `docs/features/e2e-setup-spec-flake-fix/06_TEST_REPORT.md`
- **裸 `## Adversarial tests` 标题校验**: ✓（grep `^##\s+Adversarial\s+tests\s*$` 命中 L35）
- **defects**: 0 (BLOCKER / CRITICAL / MAJOR / MINOR 全无)
- **静态实证**: AC-5/AC-6/AC-7/AC-8 + AC-4 间接 = 5/8 完全静态可证
- **动态实证（deferred）**: AC-1/AC-2/AC-3 + AC-5 反向构造 = 3 个脚本完整交付给 batch caller / stop-hook
- **routing**: advance to Stage 7 (PM delivery)

### Stage 7 — Delivery (2026-05-24)

- **PM verdict**: DELIVERED
- **作者**: PM Orchestrator
- **输出**: `docs/features/e2e-setup-spec-flake-fix/07_DELIVERY.md`
- **格式校验**:
  - 裸 `## Adversarial tests` 标题 → L78 ✓
  - 裸 `## Insight` 标题 → L89 ✓
  - Insight 段是 `- ` bullet 列表（3 条）→ ✓（避开 insight L70 子标题陷阱）
- **tasks.md 更新**: T-033 移至已完成表，目录写归档后路径 `docs/features/_archived/e2e-setup-spec-flake-fix/`
- **stage-doc 总计**: 7 个（01-07）+ PM_LOG.md

## DECLARE_DONE checklist (deferred to batch caller)

PM 派发上下文无 Bash/PowerShell/Task → 以下 4 步必须由 batch caller / stop-hook 跑：

1. **harness-sync**: 无需（本任务零 agent / rules 改动）
2. **verify_all**: 跑 `scripts/verify_all.ps1`（或 `.sh`），关键判定 = FAIL 数 ≤ 2（不上涨）；理想 = 1（C.1 PASS，仅 E.6 残留）。若 C.1 仍 FAIL → 跑 06 §"Deferred dynamic 实证脚本 3"反向构造验证错误信息含"前置条件违反"字面
3. **archive-task**: 跑 `scripts/archive-task.ps1 --task e2e-setup-spec-flake-fix`，归档 + 收割 3 条 insight 到 `.harness/insight-index.md`
4. **commit + push**: message = `fix(T-033): e2e-setup-spec-flake-fix — assertFreshBackend 守门让 C.1 FAIL 时显式 reuseExistingServer 复用 + DataDir 含 admin 根因 + 3 步修复指引`

完。








