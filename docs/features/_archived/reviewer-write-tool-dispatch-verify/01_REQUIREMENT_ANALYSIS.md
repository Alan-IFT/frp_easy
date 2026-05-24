# 01 需求分析 — T-034 reviewer-write-tool-dispatch-verify

> **产出方式说明**：本文档由 PM Orchestrator 在 SDK Opus 派发上下文中代写（该上下文
> 可用工具仅 Read / Write / Edit / Glob / Grep，无 Task tool，**无法**实际派发 RA
> sub-agent）。这一现象本身是本任务的核心证据 E-0，详见 §2.1。

模式：**full**

## 1. 目标（一句话）

**确保 gate-reviewer / code-reviewer 在 SDK Opus 派发路径下的 stage 文档落盘行为
是确定性的、用户体验良好的、物理上不可回退的** —— 不再依赖"frontmatter 声明 + 期
望 SDK 派发上下文一定给齐工具"这条单点假设。

## 2. 任务背景的关键现场观察

### 2.1 证据 E-0（最重要）：派发上下文工具裁剪现象

本任务 PM Orchestrator 自身在 SDK Opus 派发上下文中执行时，可用工具清单**只有
Read / Write / Edit / Glob / Grep**。具体观察：

- **缺 Task tool**：PM 物理上无法派发 sub-agent。任何"创建 stage 1-7 子 agent"的
  指令在此上下文中不可执行。
- **缺 Bash / PowerShell**：PM 无法跑 `scripts/verify_all` / `scripts/archive-task`。
- **缺 TodoWrite**：PM 无法用 TodoWrite 记任务进度。

这与 `.harness/agents/pm-orchestrator.md` 契约约定的 PM 工作模型存在显著差异 ——
契约假设 PM 用 Task tool 派发，但 SDK 派发路径下 PM 拿不到 Task tool。

**这给"reviewer 不落盘"现象提供了一个比 frontmatter bug 更可信的根因假设**：

> SDK 派发上下文对每个 agent 的工具清单做"二次裁剪"，frontmatter 声明的工具集是
> "理论上限"而非"实际下发"。reviewer 拿到的实际工具清单可能不含 Write，并非
> frontmatter 同步问题。

### 2.2 历史复现链（来自 insight-index）

| 任务 | Stage | 现象 |
|---|---|---|
| T-027 | 3 + 5 | reviewer 把 review 内容塞消息体让 PM 代写 |
| T-032 | 3 + 5 | 同上 |
| L41 / L44 / L48 / L50 / L60 | — | 同主题累计 5-6 次复现 |

T-030 修了 frontmatter 把 `Write` 加进 reviewer 工具清单，**但 T-032 复现**。这与
"二次裁剪"假设一致。

### 2.3 本任务自指的特殊性

本任务的对象是 Harness pipeline 本身的 sub-agent 工具集行为。这意味着：

- 本任务 **stage 3 + stage 5 派发到 reviewer agent 时，reviewer 能否自己 Write
  对应文档是核心实证数据**。
- 但当前 PM 派发上下文连"派发 reviewer agent"都做不到 —— 所以 stage 3 / stage 5
  也将由 PM 代写。**这本身就是数据**：印证了 SDK 派发上下文的工具裁剪不仅影响
  reviewer，也影响 PM 自身。
- 任何"端到端实证 Write 工具"的 probe **必须由更上层的 SDK 父进程发起**，而不是
  由 PM 在本上下文里发起。本任务在文档层面捕获这个约束，并在设计层面绕过这个
  约束（不寄希望于派发能成 → 让 fallback 路径**正式化、低成本化**）。

## 3. 用户提供的原则（用作 in-scope 行为筛选锚）

1. **用户体验好** — reviewer 该自己落盘就自己落盘
2. **符合软件工程标准** — frontmatter 契约应被运行时遵守
3. **长期易使用易维护** — 不再让"短期 workaround"沉淀

在 §2.1 证据下，对这三条原则的**精确翻译**是：

| 原则 | 精确翻译 |
|---|---|
| UX 好 | reviewer 自己落盘的成功路径不变 + 失败路径（reviewer 缺 Write）的 fallback **不能让 PM 临时编 200+ 行内容**，必须有标准化协议 |
| 软工标准 | frontmatter 声明被尊重时优先走自落盘路径；不被尊重时**契约文档已预声明 fallback**，仍属于规范化行为而非"workaround" |
| 长期可维护 | fallback 不再是 insight L60 散落的口头约定，**进 agent 契约 + 派发 prompt 模板**，物理上不可遗忘 |

## 4. In-scope 行为（编号、可测试）

### IS-1：gate-reviewer / code-reviewer agent 契约文档明确"双模式产出"

`.harness/agents/gate-reviewer.md` 和 `.harness/agents/code-reviewer.md` 必须在
"What you produce" 段下明确两条产出路径：

- **Mode A（自落盘）**：若派发上下文的工具集含 Write，agent 直接写
  `03_GATE_REVIEW.md` / `05_CODE_REVIEW.md`，消息体仅返回简短摘要（≤200 字 +
  verdict + 文件路径）。
- **Mode B（fallback / PM 协写）**：若派发上下文的工具集**不**含 Write，agent
  必须在消息体顶部用**固定 sentinel 行**声明 `MODE: PM_FALLBACK_WRITE` +
  目标文件路径 + 紧接着的完整 Markdown 文档体（按既有格式）。PM 据此精确落盘，
  无需 PM 再做任何重写 / 总结。

### IS-2：PM 派发 prompt 模板包含 Mode 检测协议

PM Orchestrator 派发 reviewer 时的 prompt 模板（沉淀在
`.harness/agents/pm-orchestrator.md` "Reviewer dispatch protocol" 段）必须显式：

- 让 reviewer 自检派发上下文是否含 Write tool
- 显式预告两种产出模式
- 显式预告 PM 在 Mode B 下做的事："PM 将把消息体 sentinel 行后的完整 Markdown 原样
  落盘到声明的文件路径，不做任何裁剪 / 重写 / 总结"

### IS-3：verify_all 加守门闸门检测 reviewer 契约 + sentinel 协议落地

`scripts/verify_all.{ps1,sh}` 加一对新静态 step：

- **G.1**：grep `.harness/agents/gate-reviewer.md` + `.harness/agents/code-reviewer.md`
  必须各自含 `MODE: PM_FALLBACK_WRITE` 字符串（fallback 协议在 agent 契约里），
  否则 FAIL
- **G.2**：grep `.harness/agents/pm-orchestrator.md` 必须含 `Reviewer dispatch
  protocol` 段标题（PM 派发 prompt 模板已沉淀），否则 FAIL

这两条把"协议存在与否"变成 verify_all 静态闸门，物理不可静默退化。

### IS-4：harness-sync 同步 + 跨工具一致

修改后的 `.harness/agents/*.md` 必须跑 `scripts/harness-sync.ps1` 同步到
`.claude/agents/*.md`，并通过 `harness-sync.ps1 -Check` 验证零 drift。

### IS-5：在 insight-index.md 替换 L60 → 新 insight（"派发上下文工具裁剪"现象 +
sentinel 协议）

旧 insight L60 是 "短期 workaround：派发 reviewer 时显式预告 fallback"。新 insight
应当：

- 命名现象："SDK 派发上下文工具裁剪"
- 引用根因证据（本任务 PM 上下文 E-0）
- 指向长期解（agent 契约里的双模式 + sentinel）
- 标注 "T-034 起 fallback 已正式化，不再是 workaround"

### IS-6：本任务自身 stage 3 + stage 5 文档诚实记录派发上下文实情

`03_GATE_REVIEW.md` 和 `05_CODE_REVIEW.md` 顶部诚实记录：

- 产出方式：由 PM 代写（pipeline-executor 模式）
- 派发上下文实情：PM 自身无 Task tool，无法派发 reviewer sub-agent
- 这一观察是 §2.1 证据 E-0 的直接延伸

## 5. Out-of-scope

| 编号 | 不做 | 理由 |
|---|---|---|
| OOS-1 | 不试图反向工程 SDK 派发上下文工具裁剪规则 | 黑盒、不稳定、不在我们控制下；改设计绕过它更可靠 |
| OOS-2 | 不让 PM 在 fallback 路径下重写 / 总结 reviewer 内容 | 这正是要消除的"PM 编内容"问题；PM 仅做字节级原样 Write |
| OOS-3 | 不改 `developer` / `qa-tester` agent 契约 | 它们既往实测能 Write 落盘（T-031 / T-032 等），不在本任务问题域 |
| OOS-4 | 不改 `requirement-analyst` / `solution-architect` agent 契约 | 同 OOS-3 |
| OOS-5 | 不引入 dynamic probe（"派发 sentinel 文件断言"）测试到 verify_all | 这类测试需要 Task tool 在派发链上层；现阶段做静态闸门已经物理足够 |
| OOS-6 | 不试图修复 PM 派发上下文工具缺位本身 | 不在 PM agent 控制下，是 SDK 行为 |
| OOS-7 | 不删现有 `.claude/agents/gate-reviewer.md` / `code-reviewer.md` 重新生成 | harness-sync 已字节级一致；保留 |

## 6. 边界条件

| 边界 | 设计要求 |
|---|---|
| reviewer 派发上下文有 Write tool（理论最优） | Mode A 正常落盘，消息体短 |
| reviewer 派发上下文无 Write tool（已知现象） | Mode B sentinel 触发，PM 原样落盘 |
| reviewer 派发上下文工具集变化（SDK 升级） | 双模式 agent 契约对两种情况都覆盖，无需改契约 |
| sentinel 行出现在文档正文里（误触发） | sentinel 行必须是消息体**第一行**，且消息体严格"sentinel + 空行 + 完整 Markdown"；任何其他位置出现的同字面字符串不触发 PM fallback 落盘 |
| reviewer 在 Mode B 下漏 sentinel 行 | PM 必须能检测到这种情况并 BLOCKED ON DISPATCH，**不**自由编内容；这点写进 PM 派发协议段 |
| reviewer 在 Mode B 下消息体里只放摘要不放完整文档 | PM 必须能识别为 incomplete fallback 并要求 reviewer 重发；不**编**补全 |
| 同一任务里 reviewer 第一次 Mode A 第二次 Mode B | 两种模式都可独立运作，无跨调用状态依赖 |
| PM 上下文也无 Write tool（极端裁剪） | 不在本任务可解空间；记为 OOS-6 |

## 7. 验收准则（每条必须可被独立验证）

| ID | 准则 | 验证方式 |
|---|---|---|
| AC-1 | `.harness/agents/gate-reviewer.md` 含 `MODE: PM_FALLBACK_WRITE` 字符串和双模式产出说明 | grep 命中 1+ |
| AC-2 | `.harness/agents/code-reviewer.md` 含 `MODE: PM_FALLBACK_WRITE` 字符串和双模式产出说明 | grep 命中 1+ |
| AC-3 | `.harness/agents/pm-orchestrator.md` 含 `Reviewer dispatch protocol` 段（标题或显著 anchor） | grep 命中 1+ |
| AC-4 | `scripts/verify_all.ps1` 含新 G.1 + G.2 step，跑通时新增 2 个 PASS（其它 FAIL 数不上涨） | 跑脚本对比 Summary |
| AC-5 | `scripts/verify_all.sh` 含同等 G.1 + G.2 step（双实现对账，insight L58） | grep + diff PS↔Bash 行为锚 |
| AC-6 | `.claude/agents/gate-reviewer.md` / `code-reviewer.md` / `pm-orchestrator.md` 与 `.harness/agents/` 同名文件字节一致 | `scripts/harness-sync.ps1 -Check` exit 0 |
| AC-7 | `.harness/insight-index.md` 旧 L60 替换为新条目，文末标注本任务 ID 为 evidence | grep + 人眼 |
| AC-8 | 本任务 `03_GATE_REVIEW.md` + `05_CODE_REVIEW.md` 顶部含"派发上下文实情"段（诚实记录证据 E-0 延伸） | grep |
| AC-9 | `scripts/verify_all` 总 FAIL 数 ≤ 当前基线 2（C.1 + E.6），不上涨 | 跑脚本对比 |
| AC-10 | `07_DELIVERY.md` 含 `## Adversarial tests` 段（裸标题，无数字前缀）+ `## Insight` 段（bullet 列表） | grep |

## 8. 非功能性要求

- **跨工具一致**：rule + agent 文件改动是 `.harness/` 真相源；harness-sync hook
  覆盖 `.claude/` binding。不能编辑 `.claude/` 或 `CLAUDE.md`（红线）。
- **零回退**：不能让 verify_all 既有 PASS step 变成 FAIL。
- **PS5.1 + zh-CN 兼容**：所有改动的 .ps1 / .sh 脚本继续遵守 insight L17 / L32-L33
  的编码约定。verify_all.ps1 已是 UTF-8 BOM 路径。
- **insight 段格式**：07_DELIVERY.md 的 `## Insight` 段用 bullet 列表（insight L57
  陷阱）+ 标题无数字前缀（insight L43/L46/L49 陷阱）。

## 9. 相关历史任务

- **T-027 / T-032** — sub-agent reviewer 不落盘最近两次复现（stage 3 + stage 5）
- **T-030** — frontmatter 加 Write 工具（基础修复，必要但不充分）
- **T-028** — archive-task regex 容错（同类"PM 跨 stage 写作格式陷阱"长期解模板）
- **T-031** — verify_all 双实现 PS↔Bash 对账（本任务 AC-5 直接复用此约束）
- **T-026** — install.ps1 双子作用域 / iex 形态 vs 磁盘形态 → 同类"上下文差异"
  设计模式参考

## 10. Open questions for user

无 —— 用户在派发文中已对原则、红线、DECLARE_DONE 闸门、commit 授权都给了明确指
令。任务进入 stage 2 不需要进一步澄清。

## 11. Verdict

**READY**
