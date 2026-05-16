# AI-GUIDE — frp_easy 项目指南

> 工具无关入口。任何 AI 工具（Claude Code、GitHub Copilot、Cursor、…）**开始任务前**先读本文。

## 项目

类型：**fullstack** · 技术栈：**Go + Vue 3 + SQLite (Web UI to manage FRP, single-binary deploy)** · 初始化：2026-05-16

本项目使用 **Harness 7-agent 流水线**：PM Orchestrator → Requirement Analyst → Solution Architect → Gate Reviewer → Developer → Code Reviewer → QA Tester。

## 真相源（在仓库里，版本控制）

- `.harness/rules/*.md` — 规则片段
- `.harness/agents/*.md` — 7 个 agent 的角色契约
- `.harness/skills/*/SKILL.md` — 标准动作（build / test / verify）

**不要直接编辑** `.claude/`、`CLAUDE.md`、`.github/copilot-instructions.md` —— 它们是工具特定的 stub 或 binding。

## 规则片段（按"什么时候读"挑）

- **`.harness/rules/00-core.md`**（**总是**）：7-stage pipeline、红线、默认产出格式
- **`.harness/rules/05-insight-index.md`**（**设计/实现任务开始时**）：跨任务踩坑事实的捕获机制；非琐碎决策前先看 `.harness/insight-index.md`
- **`.harness/rules/50-fullstack.md`**（**碰代码时**）：项目类型 overlay、分区、stack 约定
- **`.harness/rules/60-tool-handoff.md`**（**切 Claude Code ↔ Copilot 时**）：状态在文件、不在记忆

如果新加片段，在上面追加一行：文件名 + 1 行描述 + 触发条件。

**记忆层**：
- **`.harness/insight-index.md`** —— ≤30 行证据支持的项目特有事实。任务开始时读；任务结束时追加（仅在有证据支持的情况下）。

## Skills（标准动作，对应任务类型时调用）

- **`.harness/skills/build/`**：编译/打包命令
- **`.harness/skills/test/`**：测试运行
- **`.harness/skills/verify/`**：总验证闸门（`scripts/verify_all`）

## Agents

完整角色契约在 `.harness/agents/<name>.md`，**扮演或派发时按需读**。

- `pm-orchestrator` — 接新任务、派活
- `requirement-analyst` — 写 `01_REQUIREMENT_ANALYSIS.md`
- `solution-architect` — 写 `02_SOLUTION_DESIGN.md`
- `gate-reviewer` — 写 `03_GATE_REVIEW.md`
- `developer`（或分区：`dev-frontend` / `dev-backend` / `dev-db` / `dev-api` / `dev-services`）— 写 `04_DEVELOPMENT.md`
- `code-reviewer` — 写 `05_CODE_REVIEW.md`
- `qa-tester` — 写 `06_TEST_REPORT.md`

Claude Code 用户用 Task tool 派发；Copilot 用户读 agent 文件自己扮演（一次一个角色）。

## 项目文档

- `docs/workflow.md` — 完整 7-stage pipeline 定义
- `docs/tasks.md` — 当前任务看板
- `docs/dev-map.md` — 项目导航
- `docs/features/<task-slug>/` — 每个任务的阶段产出
- `docs/spec/` — 项目规格
- `evals/golden-tasks.md` — 回归任务集

## 工作流入口 —— 挑对的模式

| 模式 | 中文触发词 | English trigger | Skill |
|---|---|---|---|
| 完整 7-stage 流水线 | "加一个 ..." / "修个 bug" / "重构 ..." | "Add X" / "Fix Y" / "Refactor Z" | `/harness` |
| 只做设计（stages 1-3） | "评审一下..." / "先别动手" / "设计上行不行" | "Vet this design" / "evaluate" | `/harness-plan` |
| 探索 / 可行性 | "能不能..." / "可行吗" / "调研一下" | "Can we do X?" / "Is Y feasible?" | `/harness-explore` |
| 目标循环（Dev + QA） | "持续优化到..." / "循环改进直到..." | "Keep improving until X" | `/harness-goal` |

Declare-done 闸门：`scripts/verify_all` PASS + （7-stage 或 goal 模式下）QA 的 `06_TEST_REPORT.md` 有 `## Adversarial tests` 段。

任务完成后，跑 `scripts/archive-task --task <task-slug>` 把 insight（从 `07_DELIVERY.md` 的 `## Insight` 段）收割到 `.harness/insight-index.md`，并把阶段文档移到 `docs/features/_archived/`。

## 编辑规则

- 要改规则：编辑对应的 `.harness/rules/*.md` 片段。新加片段就在本文上面"规则片段"里加一行索引。
- 要改 agent：编辑 `.harness/agents/<name>.md`，然后跑 `scripts/harness-sync` 把改动复制到 `.claude/agents/`（Claude Code 强要求那个路径）。
- 要改 skill：编辑 `.harness/skills/<name>/SKILL.md`。同样需要 sync。

本文件和 `CLAUDE.md` / `.github/copilot-instructions.md` 都**不**自动重新生成 —— 它们引用 `.harness/`，更新通过引用流动，而不是重新组合。
