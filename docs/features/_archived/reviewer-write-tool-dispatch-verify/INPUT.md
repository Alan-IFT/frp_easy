# 任务输入 — T-034 reviewer-write-tool-dispatch-verify

## 任务来源

batch `post-t032-followup`，由 PM Orchestrator 收。

## 模式

**full**（完整 7-stage 流水线）

## 一句话目标

端到端验证 sub-agent frontmatter `tools: Read, Write, Glob, Grep` 是否真的让
gate-reviewer / code-reviewer 在 Claude Code Task tool 派发路径下能 Write 落盘；
如果不能，找根因并修到物理上不可能复发。

## 用户提供的背景

- **insight L41 / L44 / L48 / L50 / L60**（第 5-6 次复现）：sub-agent 工具白名单
  frontmatter `tools: Read, Write, Glob, Grep` 在 SDK Opus 派发路径下可能未生效 ——
  gate-reviewer + code-reviewer 在 T-027 / T-032 等多个任务中拿到的工具集仍只
  Read/Glob/Grep，把 200+ 行 review 内容塞消息体让 PM 代为落盘，浪费 PM 工具
  quota + 注意力。
- **T-030 frontmatter fix 已落**：`.harness/agents/gate-reviewer.md` +
  `.harness/agents/code-reviewer.md` frontmatter `tools` 字段已加 `Write`，并通过
  harness-sync 同步到 `.claude/agents/`（PM 已 Read 过这两个文件，frontmatter
  第 4 行确认是 `tools: Read, Write, Glob, Grep`）。
- **但 SDK Opus 派发路径下仍未生效** —— T-032 的 stage 3 + stage 5 两次 reviewer
  仍走 fallback 模式。
- **用户原话**："T-030 在 frontmatter 加 Write 工具的 fix 在 SDK Opus 派发路径没
  生效。需要端到端验证（找一个简单 reviewer 派发断言能否真的 Write）。短期
  workaround 已记录在 insight L60"。

## 关键文件

- `.harness/agents/gate-reviewer.md` — Stage 3 reviewer 契约（已 Write frontmatter，
  未生效）
- `.harness/agents/code-reviewer.md` — Stage 5 reviewer 契约（已 Write frontmatter，
  未生效）
- `.claude/agents/gate-reviewer.md` / `.claude/agents/code-reviewer.md` — Claude
  Code binding（harness-sync 复制过来）
- `scripts/harness-sync*` — `.harness/` → `.claude/` 同步脚本
- 历史证据：T-027 / T-032 的 `03_GATE_REVIEW.md` / `05_CODE_REVIEW.md` 由 PM 代写

## 用户传达的原则

1. **用户体验好** —— reviewer 该自己落盘就自己落盘，不让 PM 当文件搬运工
2. **符合软件工程标准** —— frontmatter 契约就该被运行时遵守，不能默认失效
3. **长期易使用易维护** —— 不要再让"短期 workaround"沉淀；这次必须**端到端**
   证实修复有效，而不是再次塞 fallback 提示
4. 用户已授权 commit + push 由 AI 操作

## 任务模式特殊性（重要）

这是个**元任务（meta-task）**：任务的对象是 Harness pipeline 本身的 sub-agent
工具集行为。stage 3 / stage 5 两个 reviewer 的派发**本身就是验证数据**：
- 若 reviewer 在本任务里能自己 Write `03_GATE_REVIEW.md` / `05_CODE_REVIEW.md`
  → 强证据 frontmatter 生效
- 若 reviewer 又走 fallback 让 PM 代写 → 强证据未生效，**这是本任务的关键
  实证之一**

## insight-index 相关条目（PM 已读，传给下游）

- L41 / L44 / L48 / L50 / L60 — sub-agent Write 工具白名单陷阱（同主题累计 5-6 次复现）
- L43 / L46 / L49 / L57 — 07_DELIVERY.md `## Insight` 段格式（裸标题 + bullet 列表）
- L60 短期 workaround：派发 reviewer 时显式预告"若工具集无 Write，按 fallback 模式塞消息体"

## DECLARE_DONE 闸门

- `scripts/verify_all` FAIL 数**不上涨**（基线 = 2: C.1 + E.6）
- `07_DELIVERY.md` 必须有 `## Adversarial tests` 段（**无数字前缀**）+ `## Insight` 段（**bullet 列表，无数字前缀**）
- 最后跑 `scripts/archive-task --task reviewer-write-tool-dispatch-verify`

## 红线

- 禁编辑 `.claude/`、`CLAUDE.md`、`.github/copilot-instructions.md`（直接源是
  `.harness/`，跑 harness-sync 让改动流过去）
- 不要在 BATCH_PLAN.md 之外造新 task；本任务跑完就停
