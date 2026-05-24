# 02 方案设计 — T-034 reviewer-write-tool-dispatch-verify

> **产出方式说明**：本文档由 PM Orchestrator 在 SDK Opus 派发上下文中代写（无 Task
> tool 派发 sub-agent）。设计基于 01 RA 文档中 §2.1 证据 E-0 的"派发上下文工具
> 裁剪"假设。

模式：**full**

## 1. 架构摘要（一段话）

把"reviewer 必须能 Write 自己的 stage 文档"这条期望从"frontmatter 单点依赖"重写为
"**双模式契约 + 派发协议**"：reviewer agent 契约同时规范成功路径（Mode A 自落盘）和
失败路径（Mode B 通过消息体 sentinel 让 PM 字节级原样落盘）；PM 派发 prompt 模板
显式让 reviewer 自检工具集并预告两种模式；verify_all 静态闸门保证契约 + 协议在
源码层面不可静默消失。frontmatter 仍保留 `Write` 工具（理论上限），但不再是单点依
赖 —— SDK 派发上下文不下发 Write 时系统仍能正确运转。

## 2. 影响模块

| 文件路径 | 类型 | 改动 |
|---|---|---|
| `.harness/agents/gate-reviewer.md` | 编辑 | 加 "Dispatch context awareness" + "Two-mode output protocol" 段 |
| `.harness/agents/code-reviewer.md` | 编辑 | 加同等两段 |
| `.harness/agents/pm-orchestrator.md` | 编辑 | 加 "Reviewer dispatch protocol" 段（派发 prompt 模板 + Mode B 字节级落盘约束） |
| `.claude/agents/gate-reviewer.md` | 字节同步 | harness-sync 字节级复制 |
| `.claude/agents/code-reviewer.md` | 字节同步 | harness-sync 字节级复制 |
| `.claude/agents/pm-orchestrator.md` | 字节同步 | harness-sync 字节级复制 |
| `scripts/verify_all.ps1` | 编辑 | 加 G.1（reviewer 契约含 sentinel）+ G.2（PM 派发协议段在位） |
| `scripts/verify_all.sh` | 编辑 | 加同等 G.1 + G.2（双实现对账，insight L58） |
| `.harness/insight-index.md` | 编辑 | 替换 L60（旧 "短期 workaround"）为新 insight（"派发上下文工具裁剪 + 双模式契约长期解"） |
| `docs/features/reviewer-write-tool-dispatch-verify/*` | 新 | 7 个 stage 文档 |

## 3. 模块分解（新增 / 大改的 API）

### 3.1 reviewer agent 双模式契约（gate-reviewer + code-reviewer 各加一段）

加在两份 agent 文件 "What you produce" 段下面（不替换原内容）。Markdown 文本，
内容如下（PS：实际写文件时按 §4 步骤）：

> ## Dispatch context awareness
>
> The frontmatter declares `tools: Read, Write, Glob, Grep` as the theoretical
> upper bound. The SDK dispatch context that actually runs this agent may
> tool-clip — if `Write` is **not** in your actually-available tools at runtime,
> you cannot create `docs/features/.../03_GATE_REVIEW.md`. **Do not silently
> dump the full report into the message body and hope PM figures it out.**
>
> ## Two-mode output protocol
>
> **Mode A — self-write (preferred).** You have Write at runtime. Write
> `docs/features/<task-slug>/03_GATE_REVIEW.md` (or 05 for code-reviewer)
> with the full document body. Return to PM a short message: verdict + file
> path + ≤200-char summary. Do not paste the document into the message body.
>
> **Mode B — PM fallback write (degraded but explicit).** You do NOT have
> Write at runtime. Return a message body where:
>
> 1. The very first line is exactly:
>    `MODE: PM_FALLBACK_WRITE target=docs/features/<task-slug>/03_GATE_REVIEW.md`
>    (use `05_CODE_REVIEW.md` for code-reviewer).
> 2. The next line is blank.
> 3. The remainder of the message body is the COMPLETE Markdown document
>    body, no preamble, no summary, no apologies. It will be byte-for-byte
>    written to disk by PM.
>
> Detect your mode by attempting a no-op probe — e.g. introspect available
> tools, or use a structural cue from PM's dispatch prompt that asked you
> to self-check.

### 3.2 PM "Reviewer dispatch protocol" 段（pm-orchestrator.md 新加段）

加在 pm-orchestrator.md "Stage gates" 段后、"What to write at delivery" 段前。
Markdown 文本：

> ## Reviewer dispatch protocol
>
> The gate-reviewer (stage 3) and code-reviewer (stage 5) agents follow a
> **two-mode output protocol** (see their agent files). When you dispatch
> them, your prompt MUST include this preamble:
>
> ```
> Two-mode output reminder:
> - If your dispatch context includes the Write tool, write
>   docs/features/<task-slug>/<03|05>_*.md directly (Mode A). Return only a
>   short verdict + file path + ≤200-char summary in your message.
> - If your dispatch context does NOT include Write, return a message body
>   whose first line is exactly:
>     MODE: PM_FALLBACK_WRITE target=docs/features/<task-slug>/<03|05>_*.md
>   followed by a blank line, followed by the COMPLETE Markdown document.
>   PM will byte-for-byte write that to disk. Do NOT include a summary or
>   preamble in Mode B.
> ```
>
> After receiving the reviewer response:
>
> 1. If the first line of the response matches
>    `^MODE: PM_FALLBACK_WRITE target=(\S+)$`, **byte-for-byte write** the
>    content after the sentinel + blank line to the captured target path.
>    **Do not summarize, rewrite, or augment.**
> 2. Otherwise, treat the response as Mode A. The reviewer is responsible
>    for having written the file. Open the file to verify it exists; if not,
>    treat as `BLOCKED ON DISPATCH` — re-dispatch with explicit Mode B
>    instructions.
> 3. If a Mode B response is missing the sentinel line or has incomplete
>    Markdown (e.g. truncated mid-section), do NOT fabricate or complete the
>    document. Re-dispatch with a clarification that the previous response
>    was invalid; log to PM_LOG.md.

### 3.3 verify_all 新 step G.1 / G.2

**G.1（reviewer 契约 sentinel 存在）**：grep `.harness/agents/gate-reviewer.md`
和 `.harness/agents/code-reviewer.md`，两者必须各自含字面串
`MODE: PM_FALLBACK_WRITE`。任一缺失则 FAIL。

**G.2（PM 派发协议段在位）**：grep `.harness/agents/pm-orchestrator.md` 必须含
字面串 `Reviewer dispatch protocol`（段标题）和 `MODE: PM_FALLBACK_WRITE`
（派发 prompt 模板里的 sentinel 引用）。缺失则 FAIL。

二者实现按 verify_all.ps1 / verify_all.sh 双实现，结构与 E.3 / E.5（已有的
"agent 契约存在/索引"类静态闸门）对齐。

### 3.4 insight-index 替换

旧 L60 全文删除，替换为：

```
- 2026-05-24 · **SDK 派发上下文对 sub-agent 工具集做二次裁剪**：frontmatter
  `tools: ...` 声明的是"理论上限"，SDK 实际下发可能更窄（如 reviewer 派发上下文
  缺 Write，PM 派发上下文缺 Task）。长期解 = T-034 已在 reviewer + PM agent 契约
  里建立"双模式 + sentinel 协议"，verify_all G.1/G.2 静态闸门守门；不再把
  "frontmatter 加 Write 就够" 当作单点依赖。Mode A（自落盘）+ Mode B
  （PM_FALLBACK_WRITE sentinel → PM 字节级原样落盘）双路径共存，物理上不可静默
  退化为"PM 自己编内容"。· evidence: T-034（PM 自身派发上下文实测无 Task 工具
  → 验证了"裁剪"现象的存在）
```

## 4. 数据模型变更

无。本任务全部是文档 + 规则 + verify_all 闸门的改动，无数据库 / schema 变化。

## 5. API 契约

无 REST / RPC API 改动。本任务对象是 Harness pipeline 内部的"agent ↔ PM 消息体
契约"（即 §3.1 / §3.2 的 sentinel 协议）。

## 6. 流程

### 6.1 reviewer 派发到产出的流程（修改后）

```
[PM dispatch] → reviewer agent
                ↓
                Two-mode self-check (agent 契约 §3.1 要求)
                ↓
         ┌──────┴──────┐
         ▼             ▼
       Mode A         Mode B
       (有 Write)      (无 Write)
         ↓             ↓
   Write 落盘 03/05    返消息体: MODE: PM_FALLBACK_WRITE\n\n<完整 MD>
         ↓             ↓
   返简短摘要 + path   PM 按 §3.2 byte-for-byte 落盘
         ↓             ↓
         └──────┬──────┘
                ↓
         03/05 文档存在
                ↓
         PM 进入下一 stage
```

### 6.2 verify_all 静态闸门触发流程

```
开发者改了 .harness/agents/* 但漏了 sentinel 协议段
         ↓
scripts/verify_all 跑
         ↓
G.1 grep 找不到 MODE: PM_FALLBACK_WRITE → FAIL
         ↓
开发者按错误信息补回协议段
         ↓
G.1 PASS
```

## 7. 复用审计

| 需要 | 既有代码 / 模式 | 文件路径 | 决策 |
|---|---|---|---|
| sub-agent 契约文件结构（frontmatter + Markdown body） | 既有 7 agent 文件已是此结构 | `.harness/agents/*.md` | 复用：仅追加段，不重写 |
| harness-sync 字节级同步 | 已有 | `scripts/harness-sync.{ps1,sh}` | 复用：改完 .harness 跑一遍 |
| verify_all 静态 grep 闸门模板 | E.3 / E.5 / E.6 是"文件存在 / grep 命中" 类闸门 | `scripts/verify_all.{ps1,sh}` | 复用模板：G.1 / G.2 沿 E.6 结构 |
| 双实现对账（PS↔Bash） | insight L58 (T-031) | 同上 | 复用：每加 step 必须 PS + Bash 同款 |
| insight-index 替换条目 | 历史上多次替换（如 T-028 之前的 archive-task 短期 fix） | `.harness/insight-index.md` | 复用模式：删一条加一条，文末 evidence 标 T-034 |
| sentinel 行 byte-level 文件协议 | 项目内首次引入；类似的工业先例：HTTP `\r\n\r\n` 分隔头/体、Markdown front-matter `---` | — | 新引入；选择"完整短前缀串 + 空行 + body" 是工业惯例，可读、易 grep、不易误触 |

**新依赖**：无（仅 markdown 段 + grep + harness-sync）。

## 8. 风险分析

| # | 风险 | 严重 | 缓解 |
|---|---|---|---|
| R-1 | reviewer 在 Mode B 下没把消息体首行写成 sentinel，PM 误以为 Mode A 但文件其实没落盘 | High | §3.2 PM 协议明确要求：Mode A 路径必须读文件确认存在，否则 BLOCKED ON DISPATCH 重新派发；不让"沉默失败"变成 PM 编内容 |
| R-2 | reviewer 在 Mode B 下把消息体首行写成 sentinel 但 body 不完整（如截断） | Medium | §3.2 PM 协议要求"不补全、不编",仅重新派发 |
| R-3 | sentinel 行字面 `MODE: PM_FALLBACK_WRITE` 出现在文档正文里被 PM 误触发 | Low | §3.2 PM 协议显式要求"首行精确匹配"，正文内位置不触发；本任务文档自身在 §3.1 / §3.2 里引用 sentinel 串时已不在首行，零误触发 |
| R-4 | verify_all G.1 / G.2 自身写错让闸门成假阳性 PASS | Medium | 实现时跟 E.6 双实现严格对账（insight L58）；QA 阶段 adversarial test 反向证伪：临时删 sentinel 跑 verify_all 看是否 FAIL |
| R-5 | harness-sync 复制 .harness/agents → .claude/agents 后 frontmatter 仍是 4 工具但 Mode B 协议体没复制？ | Low | harness-sync.ps1 是字节级 hash 比较 + Copy-Item -Force，全文复制，不可能漏正文段；E.4 闸门 -Check 模式守门 |
| R-6 | 用户未来跑 /harness 但 SDK 派发上下文给齐了所有工具，reviewer 还是按惯性塞消息体？ | Low | §3.1 reviewer 契约要求"Mode A 优先"，且必须自检；PM 派发协议 §3.2 步骤 2 要求 Mode A 路径"打开文件确认"，文件不存在则 BLOCKED ON DISPATCH → 反向迫使 reviewer 真的去 Write |
| R-7 | 07_DELIVERY.md 的 ## Insight 段写错格式让 archive-task 收割不到（insight L43 / L57） | Medium | PM 自己产出 07 时严格按 bullet 列表 + 裸标题；archive-task 已在 T-028 加 fallback regex，但仍以 bullet 为正解 |
| R-8 | 跨工具差异：Copilot / Cursor 用户走"自扮演 agent" 路径，他们看到 reviewer 契约的"双模式"会困惑 | Low | 契约段开头说明 "tool-agnostic"；自扮演路径下用户就是写文件的人，等同于 Mode A |

## 9. 迁移 / rollout

- 全部改动可一次 commit。
- 无 schema 变化，无回滚需求。
- verify_all 现有 FAIL 基线 = 2（C.1 playwright + E.6 历史任务 06 标题违规归档），新 G.1 / G.2 应当**初始即 PASS**（因为本任务一并把契约段补齐）。
- 若 G.1 / G.2 在加入瞬间 FAIL 说明实现漏了，Stage 5 / 6 必须挡住。

## 10. Out-of-scope 澄清

- 不试图反向工程 SDK 工具裁剪规则（OOS-1）
- 不让 PM 在 Mode B 下做内容生成（OOS-2）
- 不修 developer / qa-tester / RA / SA 契约（OOS-3 / OOS-4）
- 不引入 dynamic probe 测试（OOS-5）：sentinel + 静态闸门已物理足够
- 不修 PM 自身工具裁剪（OOS-6）：不在控制范围内

## 11. Partition assignment

本任务改动文件是 `.harness/agents/*.md` + `scripts/verify_all.{ps1,sh}` +
`.harness/insight-index.md` + `docs/features/<slug>/*` + `.claude/agents/*.md`
（由 harness-sync 间接）。

**这些路径不属于** `dev-db` / `dev-backend` / `dev-frontend` 任一分区（参见
`.harness/agents/dev-*.md` 的 owned-paths）。**本任务是元任务（Harness pipeline
self-modification），使用通用 `developer` agent 派发，不走分区路径。**

| 文件 | 分区 | 新 / 编辑 | 依赖 |
|---|---|---|---|
| `.harness/agents/gate-reviewer.md` | (meta — generic `developer`) | edit | — |
| `.harness/agents/code-reviewer.md` | (meta — generic `developer`) | edit | — |
| `.harness/agents/pm-orchestrator.md` | (meta — generic `developer`) | edit | — |
| `scripts/verify_all.ps1` | (meta — generic `developer`) | edit | 上面三条之后（要 grep 那三个文件） |
| `scripts/verify_all.sh` | (meta — generic `developer`) | edit | 同上 |
| `.harness/insight-index.md` | (meta — generic `developer`) | edit | — |
| `.claude/agents/*.md` | (meta — auto via harness-sync) | sync | 上述 .harness/agents 改动后 |
| `docs/features/.../*.md` | (meta — PM-managed) | new | — |

### Dispatch order

1. （元任务）generic `developer` 一次性改完上述 8 类路径
2. 跑 `scripts/harness-sync.ps1` 把 `.harness/agents/` 三个文件同步到 `.claude/agents/`
3. 跑 `scripts/verify_all.ps1` 验证 G.1 / G.2 PASS、整体 FAIL 数不上涨

### Parallelism

None — strict sequential, 单 developer 单 commit。

## 12. Verdict

**READY**
