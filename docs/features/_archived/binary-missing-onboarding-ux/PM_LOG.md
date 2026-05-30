# PM_LOG — T-057 binary-missing-onboarding-ux

> PM Orchestrator 路由日志。每个 stage transition 记录决策。
> mode: full（/harness）。

## 任务上下文（启动时核实）

- slug: `binary-missing-onboarding-ux`
- 一句话目标：改善"二进制缺失"首次使用体验两处——Dashboard 缺失提示给可操作入口、Wizard 完成前校验所选角色二进制是否就绪。
- 启动读取：`.harness/insight-index.md`（L14 role-collapse、L22 SFC 纯逻辑行数、L40 裸 Adversarial 标题、L45 getExposed/apiError、L46 baseline 同步）、`AI-GUIDE.md`、`docs/tasks.md`、`docs/dev-map.md`（Wizard 行）。
- 相关历史任务：
  - T-002 zero-config-quickstart（Wizard.vue 引入，顶级路由 /wizard 3 步）
  - T-014 frp-binary-auto-download（一键下载链路）
  - T-018 upload-bin-multiport-ip-probe（UploadBinButton 仅挂 AppLayout banner）
  - T-027 download-cancel-and-upload-decouple（取消下载 + 上传解耦）
  - T-047 frontend-honest-states（Dashboard 开关不静默；三态范式）
  - T-051 frontend-test-coverage（wizard store 测试）
  - T-056 proc-stop-destructive-confirm（Dashboard.spec.ts 当前范式）

## 核实的关键技术事实（影响路由与设计）

1. **Wizard 不在 AppLayout 内**：`router.ts:10` `/wizard` 是顶级路由，与 `/`（AppLayout）平级。
   → 顶栏缺失横幅（含一键下载/上传）在向导阶段不可见 → 必须在向导内显式提示。证实任务前提。
2. **binMissing 在向导阶段已 fetch**：`router.ts:40` `beforeEach` 在任何导航（含进入 /wizard）前 `await app.fetchReady()` 填充 `appStore.binMissing`。
   → 实现仍在 Wizard 完成校验前 `await appStore.fetchReady()` 一次，确保境内用户停留期间（上传/下载后）状态新鲜 + 单测可独立控制（属"仅当确需补 fetch"，写 04 设计依据）。
3. **e2e 不触碰 Wizard 完成流程**：
   - `01-setup.spec.ts` TC-02 仅断言"离开 /setup"（允许 /dashboard 或 /wizard），不进 Wizard step-2 完成。
   - `03-dashboard.spec.ts` 用 `bypassWizard(page)`（调 wizard/complete API）绕过向导，不走 Wizard 组件。
   → 改 Wizard 完成逻辑对 e2e 零影响。但仍按要求实现"无缺失保持自动跳转、缺失才改手动"，双重保险（e2e 后端二进制存在 → 无缺失 → 自动跳转不变）。
4. **frontend_tests 口径**：`vitest.config.ts` exclude `tests/e2e/**` → baseline `frontend_tests` = `vitest run` 全量计数。
5. **AppLayout 横幅文案**（`AppLayout.vue:18`）："二进制缺失: ...。网络不便时可手动上传：" + 一键下载按钮 + UploadBinButton。Dashboard alert 文案应指向"顶部横幅"的这些入口，不重复造按钮。

## Stage transitions

| 时间 | Stage | Agent | 决策 | 依据 |
|---|---|---|---|---|
| 2026-05-30 | 启动 | PM | 创建任务目录 + tasks.md 登记 mode:full | 任务受理 |
| 2026-05-30 | 1 | requirement-analyst | 产出 01；READY（无 open question，技术上下文 orchestrator 已核实充分） | 见 01 §9 |
| 2026-05-30 | 2 | solution-architect | 产出 02；单 partition dev-frontend | 见 02 |
| 2026-05-30 | 3 | gate-reviewer | 产出 03；APPROVED FOR DEVELOPMENT | 见 03 |
| 2026-05-30 | 4 | dev-frontend | 产出 04；verify 静态预测全绿，待 orchestrator 真跑 | 见 04 |
| 2026-05-30 | 5 | code-reviewer | 产出 05；APPROVED | 见 05 |
| 2026-05-30 | 6 | qa-tester | 产出 06；含裸 ## Adversarial tests | 见 06 |
| 2026-05-30 | 7 | PM | 产出 07；DELIVERED（verify_all 全量真跑交 orchestrator Bash 会话硬闸门） | 见 07 |

## Stage gate 检查

- Before stage 4：stage 3 GR verdict = APPROVED FOR DEVELOPMENT ✔
- Before stage 5：stage 4 verify_all 静态预测 PASS（PM 上下文无 Bash/PS，全量真跑标 PENDING 交 orchestrator）✔（按 insight L46 batch orchestrator 自己真跑硬闸门）
- Before stage 7：stage 5 APPROVED + stage 6 含 Adversarial 段 PASS ✔

## Rollback 记录

- 共 0 次 rollback。

## 派发分区检测（stage 4）

- `.harness/agents/dev-*.md` 存在（partitioned mode）。
- 02 Partition assignment：纯前端改动（Dashboard.vue 文案 + Wizard.vue 校验/提示 + 其 spec + baseline.json + dev-map.md）→ 单 partition `dev-frontend`。无 db/backend/api 触碰。
