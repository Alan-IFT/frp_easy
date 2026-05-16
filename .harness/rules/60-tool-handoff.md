## 工具切换（Claude Code ↔ GitHub Copilot）

本项目可能在多个 AI 工具间开发 — 通常 Claude Code 是主，Copilot 作为额度限制时的后备，或者协作者用不同 IDE。任何读到此文件的 AI（Claude Code 的 PM Orchestrator、Claude Code 的 sub-agent、或 GitHub Copilot）都必须遵守这套协议，保证工具切换后工作连续。

### 核心原则

**所有任务状态保存在文件里，不在会话记忆里。** 任何进行中任务的完整可恢复状态是：

- `docs/tasks.md` — 任务看板：有哪些任务、各自在哪个 stage。
- `docs/features/<task-slug>/` — 每个任务的 01–07 阶段文档 + `PM_LOG.md`。
- `.harness/agents/*.md` — 每个 agent 必须遵守的角色契约。
- `.harness/rules/*.md`（本文件就在这里）— 项目级规则。

**不在这些文件里的就不是状态** — 那是会话噪音。交接时，把重要的持久化到这些文件；恢复时，先读这些文件。

### 如何恢复任务（任何 AI 工具，上下文切换后）

当用户说"continue task T-XXX"或"what's in progress"：

1. 读 `docs/tasks.md`。找 `stage` ≠ `done` 的行（多个时取最近开始的）。
2. 读 `docs/features/<task-slug>/PM_LOG.md` — 最后一条记录告诉你下一步该哪个 agent 工作，以及任何阻塞。
3. 按顺序读所有已有阶段文档：`01_REQUIREMENT_ANALYSIS.md`、`02_SOLUTION_DESIGN.md`、`03_GATE_REVIEW.md`、`04_DEVELOPMENT.md`（分区模式下是 `04a_DEVELOPMENT_<partition>.md`）、`05_CODE_REVIEW.md`、`06_TEST_REPORT.md`。**跳过不存在的** — 不存在的那个就是当前或还没到的 stage。
4. 决定要扮演的角色：
   - **用 Claude Code 时**：PM Orchestrator（如果你就是 PM）从 `02_SOLUTION_DESIGN.md` 读 partition assignment，通过 Task tool 派合适的 sub-agent。Sub-agents 不直接进 resume 模式 — PM 路由它们。
   - **用 Copilot 或其他无 sub-agent 派发能力的工具时**：读步骤 2 PM_LOG 指示的下一个 agent 对应的角色文件 `.harness/agents/<role>.md`。**亲自扮演那个角色**。严格按它的契约工作 — 读它要读的，写它要写的，分区任务遵守 owned paths。
5. 产出下一 stage 的文档（或者继续当前 stage 的文档如果你在中途）。写到 `docs/features/<task-slug>/`。
6. 给 `PM_LOG.md` 追加一行：时间戳 · agent 名 · "完成 stage X，下一 stage Y 由 Z agent 处理"。
7. 更新 `docs/tasks.md` 这个任务的 `stage` 字段。

### 任务中途交接

当你（任何 AI）到达停止点 — 额度即将耗尽、用户切 IDE、会话结束：

1. 尽量完成当前 stage。如果不行，在 `docs/features/<task-slug>/` 写一个 `PARTIAL.md`，明确写清你停在哪里、下一个 agent 该做什么。
2. 给 `PM_LOG.md` 追加最后一条：时间戳 · "stage X 中途交接 · 下一动作：Y 由 agent Z"。
3. 确保 `docs/tasks.md` 的 `stage` 列是最新的。
4. 如果部分改动后需要重跑 verify_all，在 PM_LOG 标注。

### 工具特有说明

- **Claude Code**：PM Orchestrator 是路由 agent。它读 PM_LOG，派 sub-agents。完整 7-stage 流水线是原生能力。`.claude/settings.json` 里的 Stop hook 会在 session 结束时自动跑 `scripts/harness-sync`，所以 `.harness/` 的编辑会自动流到 `CLAUDE.md` + `.github/copilot-instructions.md`，用户什么都不用做。
- **GitHub Copilot**：没有 sub-agent 派发。你（Copilot）按协议指向扮演哪个角色。**一次只扮演一个角色**。完成你的 stage 后停下来，让用户"switch to next agent" — **不要悄悄换成另一个角色继续**。跨 stage 交接经过用户（通常用户会切回 Claude Code 让 PM 路由，或者手动告诉你扮演下一角色）。

### 非 Claude Code 工具的文档同步责任

上面的 Stop hook 是 **Claude Code 专属** — Copilot、Cursor、手工编辑都不会触发它。如果你（Copilot 或任何非 Claude Code 的 AI）编辑了 `.harness/` 下的任何文件，有两种同样有效的方式来防止 `CLAUDE.md` 和 `.github/copilot-instructions.md` 过时：

1. **声明本轮完成前先跑 sync。** 编辑后立刻执行 `pwsh -File scripts/harness-sync.ps1`（Windows）或 `bash scripts/harness-sync.sh`（macOS / Linux）。然后把重新生成的 `CLAUDE.md` / `.github/copilot-instructions.md` 跟你的 `.harness/` 改动一起 stage。
2. **靠 git pre-commit hook 兜底。** 如果 init 时跑过 `scripts/install-hooks.{ps1,sh}`，`.git/hooks/pre-commit` 会跑 `harness-sync --check`，任何带漂移的 commit 会被拦下来，错误信息会告诉你跑 sync。

pre-commit hook 是工具无关的兜底 — 任何工具、任何人编辑都能抓到。"完成前跑 sync"是更温和、更快的路径，保证整个 session 期间工作树始终一致。

**手工编辑生成文件**（`CLAUDE.md`、`.github/copilot-instructions.md`、`.claude/agents/*.md`、`.claude/skills/*/SKILL.md`）**永远是错的** — 下次 sync 会清掉。编辑 `.harness/` 后 sync。

### 跨工具的硬规则

- **不要编辑** 上游 agent 写的 `docs/features/<task>/01–07` 文档。如果你（当前 agent）需要它改，写阻塞到 PM_LOG 并停下来 — 原作者重做。
- 恢复时**不要跳 stage**。如果 `03_GATE_REVIEW.md` 缺但 `02_SOLUTION_DESIGN.md` 在，你（或 PM）必须先跑 Gate Review 才能开发。
- 没有 `07_DELIVERY.md` 和最终 `verify_all` PASS，**不要**声明任务完成。
- 上面"输出语言"规则在恢复时同样适用。中文项目里 Copilot 也用中文继续。
