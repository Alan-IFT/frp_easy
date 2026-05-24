# BATCH_PLAN — post-t032-followup

> 2026-05-24 创建。基于 T-032 交付完成后用户反馈的两个 follow-up：
>
> - **C.1 E2E flake**（用户在 T-032 中 3 次 git stash 对照证明非本任务引入，T-031 归档后仍 FAIL → 长期基线问题）
> - **reviewer 不落盘陷阱第 5-6 次复现**（insight L41/L44/L48/L50/L60；T-030 frontmatter fix 在 SDK Opus 派发路径未生效）

## Baseline 状态（2026-05-24 batch 启动时）

- `bash scripts/verify_all.sh`：**23 PASS / 2 FAIL / 0 WARN / 0 SKIP**
  - C.1 E2E smoke (playwright) FAIL（本批次 T-033 修）
  - E.6 Adversarial tests heading FAIL（**本批次不在 scope**；记录在 BATCH_REPORT follow-up）
- `pwsh scripts/verify_all.ps1`（用户侧）：24 PASS / 1 FAIL，只挂 C.1（E.6 PS 实现假阳性 PASS — insight L52）

**回归判定**：每个 task 跑完后 verify_all FAIL 数不上涨即不算回归（基线已 FAIL，标准 batch skill "refuse on FAIL" 规则在此豁免，原因：本批次的目的就是修基线 FAIL）。

## 任务表

| ID | Slug | Goal | Mode | Depends on | Status |
|---|---|---|---|---|---|
| T-034 | reviewer-write-tool-dispatch-verify | 端到端验证 sub-agent frontmatter `tools: Read, Write, Glob, Grep` 是否在 Claude Code Task tool 派发路径下让 reviewer 真能 Write 落盘；找根因 + 改 agent 契约/PM 派发模式让"reviewer 不落盘"陷阱物理不可能复发 | full | — | done |
| T-033 | e2e-setup-spec-flake-fix | 根因定位 + 修 `web/tests/e2e/01-setup.spec.ts` 长期 FAIL：Playwright webServer 启动竞态 / data.db fixture 残留 / 后端 setup endpoint 已初始化时不跳 三选一（或都修）；让 verify_all C.1 PASS 且后续 N 次 run 稳定 | full | — | done |

**Topo order**: T-034 → T-033

**理由**：T-034 修的是"reviewer 是否真能落盘"的元工具问题，影响后续所有 task 的 review 阶段质量。先做 T-034 让 T-033 自己的 stage 3 + stage 5 reviewer 不再走 fallback。两者之间无代码层依赖（reviewer agent 契约 vs E2E 测试 / Playwright config），但流程层 T-034 先做更稳。

## 用户已传达的决策原则

- 用户体验好
- 符合软件工程标准
- 长期易使用易维护
- AI 决策，commit + push 由 AI 操作

## 本批次的 strong-signal 停止条件

- 任一 task pm-orchestrator 返回 FAILED
- 任一 task 跑完后 verify_all FAIL 数 > 2（即超过基线）
- `.harness/intervention.md` 出现 STOP
- 安全 hook 拦截
