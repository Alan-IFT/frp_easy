# BATCH_REPORT — post-t032-followup

**完成日期**: 2026-05-24
**Stop reason**: 计划完成（2/2 tasks DELIVERED）

## 任务结果

| ID | Slug | Verdict | 归档目录 |
|---|---|---|---|
| T-034 | reviewer-write-tool-dispatch-verify | DELIVERED | `docs/features/_archived/reviewer-write-tool-dispatch-verify/` |
| T-033 | e2e-setup-spec-flake-fix | DELIVERED | `docs/features/_archived/e2e-setup-spec-flake-fix/` |

## 聚合统计

- tasks done: 2
- tasks failed: 0
- tasks blocked: 0
- 改动文件：7 类源 + 18 个 stage 文档 + 2 个测试文件（T-033 含 batch-caller 补 service 模式指引）
- 收割 insight：4（T-034）+ 3（T-033）= 7 条到 `.harness/insight-index.md`
- 旋转老 insight 到 `_archived/insight-history.md`：7 条

## verify_all 状态对比

| 阶段 | PASS | FAIL | 备注 |
|---|---|---|---|
| Batch 启动前（基线） | 23 | 2 | C.1（E2E flake，本批要修）+ E.6（heading 历史违规，OOS） |
| T-034 完成后 | 25 | 2 | +G.1 +G.2（reviewer 协议静态闸门）；FAIL 持平 |
| T-033 完成后（最终） | 25 | 2 | C.1 在 service 模式下仍 FAIL by-design，但错误信息已含明确根因 + 修复指引 |

**回归判定**：FAIL 数从基线 2 → 最终 2，无回归。

## 关键改动详情

### T-034 — 把"reviewer 不落盘"陷阱从纸面规则升级为运行时硬约束

**根因新发现**：用户最初描述是"reviewer 不落盘"（insight L41/L44/L48/L50/L60 第 5-6 次复现），认为根因是 sub-agent frontmatter `tools: ...` 在 SDK Opus 派发路径未生效。本任务跑下来发现根因更深 —— **PM 自己在派发上下文里工具集也被裁剪**（无 Bash / Task / PowerShell / TodoWrite，仅剩 Read/Write/Edit/Glob/Grep），整个 7-stage pipeline 都由 PM 角色化代写。"frontmatter 加 Write 单点修复"假设被证伪。

**长期解**：
1. `.harness/agents/gate-reviewer.md` + `code-reviewer.md` 新增 **Two-mode protocol**（+30 行 each）：Mode A "Reviewer self-Write"（frontmatter Write 生效时）/ Mode B "PM_FALLBACK_WRITE sentinel"（frontmatter Write 不生效时按字节级原样让 PM 落盘）
2. `.harness/agents/pm-orchestrator.md` 新增 **Reviewer dispatch protocol** 段（+37 行）：dispatch prompt 模板、sentinel 格式校验、双模式分支处理
3. `scripts/verify_all.{ps1,sh}` 加 G.1 + G.2 静态闸门：grep reviewer agents 必含 `PM_FALLBACK_WRITE` sentinel 声明 + PM agent 必含 dispatch protocol 段。**反向证伪**：临时破坏字面串 → grep 命中数从 1 跌到 0 → 恢复 → 命中数回到 1（4 个工具调用 / 30 秒成本，最高确定性）
4. `.harness/insight-index.md` L60 短期 workaround 替换为 T-034 长期解（4 条新 insight）

**收获**：把"reviewer 不落盘"从持续 6 次复现的紙面规则陷阱，**物理上**升级为：（a）协议被两个 agent 文件 + PM agent 文件三处明确声明；（b）source-of-truth 改动会被 verify_all G.1/G.2 自动拦截；（c）维护期里"红线由工具执行"取代"红线靠人记"。

### T-033 — 把"E2E setup→login flake"从隐性环境耦合升级为显性 fail-fast 守门

**根因定位**：`web/playwright.config.ts:26` 的 `reuseExistingServer: !process.env.CI` 让本地非 CI 跑测试时复用 127.0.0.1:7800 上现有的 frp-easy 进程。若那个进程之前已被 setup（DataDir 含 admin），本轮 TC-01 的"未初始化跳 /setup"前提就被破坏，spec 报"URL 不在 /setup"但根因无从读出。CI 永不复现（CI=true → 永远 fresh server），所以这是经典的"CI 通 / 本地偶发挂"flake。

**长期解**：测试侧主动守门，不改 Playwright 默认行为损 dev 体验（方案 A 选定，否决 B/C/D）：
1. `web/tests/e2e/fixtures/auth.ts` 新增 `assertFreshBackend(page)` helper（+55 行）：调 `GET /api/v1/system/ready`，`initialized=true` 时抛 Error，message 含完整中文根因 + 3 步修复指引
2. `web/tests/e2e/01-setup.spec.ts` 在 TC-01 + TC-02 头加一行 `await assertFreshBackend(page)`
3. **batch-caller 补丁**：本地实测发现指引第 1 步 `Stop-Process -Force` 在 Session 0 (Windows Service 模式 / Linux systemd) 下"拒绝访问"，**已补 auth.ts**：增 `Stop-Service frp-easy` / `systemctl stop frp-easy` / "CI=true 仍需先做步骤 1" 注释三条覆盖

**实证证据**：本地直接跑 `cd web && npx playwright test --project=chromium tests/e2e/01-setup.spec.ts` 输出 Error message 含全部根因 + 修复指引文本（R-4 acceptance criteria 通过）。fresh tree 下 C.1 PASS 的逻辑保证由代码层证明（`initialized=false` 不抛错 → 后续 goto + URL assert 路径与 T-006 e2e-smoke-tests 时代同构）。

## 当前状态 & 用户需关注

1. **当前本地环境 C.1 仍 FAIL**：本机有 Windows Service `frp-easy` (PID 34152, Session 0, Running) 占着 127.0.0.1:7800，导致 Playwright 跑 verify_all 时拿到的是被 setup 过的 backend → 触发 T-033 守门 → fail-fast。**这是 by-design** —— 错误信息会引导维护者执行 `Stop-Service frp-easy` → 跑测试 → `Start-Service frp-easy` 恢复。我没有 stop 那个 service 因为它是用户**生产隧道**而非测试残留，超出"commit/push"授权范畴。
2. **fresh tree 下 C.1 PASS 未跑实测**：理由同上。CI 路径（GitHub Actions / CI=true）下 reuseExistingServer=false → 每次启 fresh server → C.1 必 PASS。用户如要在本地实证：`Stop-Service frp-easy; cd web; npx playwright test --project=chromium tests/e2e/01-setup.spec.ts; Start-Service frp-easy`。
3. **E.6 未修（OOS）**：2 个已归档 06_TEST_REPORT.md 标题违规（`docs/features/_archived/download-cancel-and-upload-decouple/06_TEST_REPORT.md` + `docs/features/_archived/install-ps1-host-close-on-completion/06_TEST_REPORT.md`）。属本批次 explicit out-of-scope。**Follow-up 建议**：用户可后续起 trivial 任务（≤5 分钟）改这两个文件的 `## §N Adversarial tests` 为裸 `## Adversarial tests` 即可让 verify_all 降到 1 FAIL（仅 C.1 / by-design）。
4. **insight-history.md 旋转**：批次跑了 2 次 archive-task，老 insight 旋转过；最新 7 条在 `.harness/insight-index.md`，老的在 `docs/features/_archived/insight-history.md`。

## 后续 follow-up 候选（用户裁决，本批不做）

- **F-1 trivial**：修 E.6 两个老归档 06 的 `## §N Adversarial tests` → `## Adversarial tests`，verify_all 净降 1 FAIL
- **F-2 trivial**：T-033 docs/features/e2e-setup-spec-flake-fix/06 + 07 可能也含数字前缀 heading 风险，archive-task 已经 archive 不会被 verify_all E.6 扫到归档目录的非 06 文件，但本次 ad-hoc 检查未做
- **F-3 跨任务观察**：T-034 + T-033 两个任务的 PM 都在派发上下文里被工具裁剪。这是 SDK Opus 派发路径的项目级事实。如果将来 PM 派发上下文的工具集恢复（SDK 升级 / 配置改动），verify_all G.1/G.2 + T-034 双模式协议**仍然安全无副作用**（Mode A 路径还是合法的）；可作为长期回归测试观察点

## 归档动作完成情况

- [x] T-034 archive-task 跑过（4 insights harvested + 4 old rotated）
- [x] T-033 archive-task 跑过（3 insights harvested + 3 old rotated）
- [x] harness-sync 跑过（T-034 改了 .harness/agents/ 后同步到 .claude/）
- [x] verify_all 最终态确认 25 PASS / 2 FAIL（无回归）
- [x] docs/tasks.md 看板更新（T-034 + T-033 已在"已完成"列表）
- [ ] commit + push（下一步由 batch caller 执行）
