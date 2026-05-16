# frp_easy — 项目规则

> 项目类型：**fullstack** · 技术栈：**Go + Vue 3 + SQLite (Web UI to manage FRP, single-binary deploy)** · 初始化：2026-05-16
>
> 本文件**由 `.harness/rules/*.md` 通过 `scripts/harness-sync.{ps1,sh}` 生成**。
> 编辑 `.harness/rules/*.md` 后重新跑 sync。**不要直接编辑 `CLAUDE.md`**。

## 输出语言（全项目）

**本项目所有 AI 产出必须使用中文。** 适用于：

- 跟用户的所有对话回复。
- Agent 之间的交接（PM 派发、Architect 描述等）。
- `docs/features/<task>/` 下每份阶段文档：`01_REQUIREMENT_ANALYSIS.md`、`02_SOLUTION_DESIGN.md`、`03_GATE_REVIEW.md`、`04_DEVELOPMENT.md`、`05_CODE_REVIEW.md`、`06_TEST_REPORT.md`、`07_DELIVERY.md`、以及 `PM_LOG.md`。
- 对 `docs/tasks.md` 和 `docs/dev-map.md` 的更新。
- 错误消息、状态报告、给用户的解释。
- 适当的代码注释。

**不要混用语言。** 即使用户用其他语言发消息，也用中文回答（内部理解用户意图，外部输出中文）。这样仓库里所有产出都对所有协作者可读。

要修改项目语言，编辑 `.harness/rules/00-core.md` 的"输出语言"章节，然后跑 `scripts/harness-sync`。

## 这个项目怎么开发

本仓库使用 **Harness 7-Agent 流水线**。动手前先读 `docs/workflow.md`。

- 新功能或 bug？→ 让 PM Orchestrator 接手。
- 非琐碎改动（>10 行或包含任何逻辑）**绝不**跳过流水线。
- 琐碎改动（typo、注释、单行修复）可以跳过 agents，**但仍必须通过 `verify_all`**。

## 红线（绝不能突破）

1. **不允许静默方案漂移。** 实现必须偏离已批准方案时，在 `04_DEVELOPMENT.md` 标 `DESIGN DRIFT`。
2. **下游不能改上游文档。** 发现上游有缺陷，通过 PM 提阻塞回退，不要自己改。
3. **测试数只升不降。** 不允许删除测试让 `verify_all` 过。要删过时测试，需 PM 批准。
4. **代码里不能有 secret。** 用 `.env` / 环境变量；verify_all 会扫描硬编码 key。
5. **不允许未确认的生产破坏性操作**（drop table、force push main、删分支）。
6. **声明完成前必须跑 verify_all。** "能编译"不等于完成。
7. **编辑 `.harness/`，不编辑 `.claude/` 或 `CLAUDE.md`。** 后两者是生成的。Stop hook 会在每次 session 结束时自动跑 `scripts/harness-sync`，所以手动 sync 很少需要。如果 hook 没跑成功，`verify_all` 会拦下漂移。
8. **优先让 AI 帮你编辑 `.harness/`，而不是自己手编。** 例：
   - "加一条规则：禁止用 `MessageBox.Show`。" → AI 选合适的 `.harness/rules/NN-*.md`（或新建一个），编辑，Stop hook 同步。
   - "为 `apps/mobile/` 加一个 Developer 分区。" → AI 在 `.harness/agents/dev-mobile.md` 写正确的 owned-paths。
   - "把 Architect 收紧成新模块必须有 ADR 文件。" → AI 编辑 `.harness/agents/solution-architect.md`。
   这是 AI 驱动的路径：人提，AI 编辑，hooks 同步。**自己动手编辑也可以**，但**不是必须**。

## 风格 / 约定

- 代码风格由仓库内的 lint 配置（`eslint`、`ruff`、`gofmt` 等）强制。**不要绕过**。
- Commit message：祈使语气，首行 ≤72 字符，正文解释 why。
- 文件名：已有项目约定优先于默认 — 跟现有的对齐。

## 各种东西在哪里

| 需要 | 看这里 |
|---|---|
| 项目结构 / 文件在哪 | `docs/dev-map.md` |
| 当前在做什么 | `docs/tasks.md` |
| 每个任务的文档 | `docs/features/<task-slug>/` |
| 项目 SPEC / 需求 | `docs/spec/` |
| 7-agent 流水线定义 | `docs/workflow.md` |
| Agent 角色定义（**真相源**） | `.harness/agents/` |
| 项目规则碎片（**真相源**） | `.harness/rules/` |
| Build/test/verify 标准操作（**真相源**） | `.harness/skills/` |
| Claude Code 绑定（**生成的，不要编辑**） | `.claude/` + `CLAUDE.md` |
| 工具绑定同步（`.harness/` → `.claude/` + `CLAUDE.md`） | `scripts/harness-sync.{ps1,sh}` |
| 总验证 | `scripts/verify_all.{ps1,sh}` |
| 测试基线 | `scripts/baseline.json` |
| 回归任务集 | `evals/golden-tasks.md` |

## 拿不准时

- 先读 `docs/dev-map.md` — 它告诉你哪个文件归哪个 feature。
- 开始新工作前查 `docs/tasks.md` 找相关历史任务。
- 规则与情况冲突时，**停下来问用户**，不要自由发挥。
