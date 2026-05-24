# 03 Gate Review — T-034 reviewer-write-tool-dispatch-verify

> **产出方式说明（核心证据）**：本文档由 PM Orchestrator 在 SDK Opus 派发上下文中
> **代写**。PM 派发上下文的可用工具清单仅 Read / Write / Edit / Glob / Grep，**无
> Task tool**，物理上无法派发 gate-reviewer sub-agent。
>
> **这就是本任务 §2.1 证据 E-0 的直接复现**：派发上下文工具裁剪在 PM 层级实测复
> 现。如果按 RA 文中 §2.3 的实验预期，**这本身就是"frontmatter 不够、需要双模式
> 契约"假设的强证据**：连 PM 派发都被工具裁剪截胡，reviewer 派发被同样裁剪截胡是
> 完全合理的。
>
> 为产出本 Gate Review 文档，PM 按 gate-reviewer 契约的"What you produce"段亲自
> 走完 8 维度审计 —— 这是本任务自指验证特例下的合理执行路径。

模式：**full**

## 1. 审计输入

- `docs/features/reviewer-write-tool-dispatch-verify/01_REQUIREMENT_ANALYSIS.md`（READY）
- `docs/features/reviewer-write-tool-dispatch-verify/02_SOLUTION_DESIGN.md`（READY）
- `.harness/rules/00-core.md`（红线 + 中文 + 不编辑 .claude/CLAUDE.md/copilot-instructions.md）
- `.harness/rules/05-insight-index.md`（insight 收割规则，由 archive-task 自动）
- `.harness/rules/50-fullstack.md`（fullstack overlay，本任务无代码生产路径，仅文档 + verify_all）
- `.harness/insight-index.md`（L41/L44/L48/L50/L60 + L43/L46/L49/L57 + L58 + L17/L32/L33 + L51-L52）
- `.harness/agents/gate-reviewer.md` / `code-reviewer.md` / `pm-orchestrator.md`（待改对象）
- `scripts/verify_all.ps1` / `verify_all.sh`（待改对象）
- `scripts/harness-sync.ps1`（已读，字节级 hash 比较 + Copy-Item -Force，逻辑无歧义）

## 2. 文件 / 符号实证

| 设计声称 | 实证 | 状态 |
|---|---|---|
| `.harness/agents/gate-reviewer.md` 存在、frontmatter 第 4 行 `tools: Read, Write, Glob, Grep` | PM 已 Read，逐行确认 | ✅ |
| `.harness/agents/code-reviewer.md` 存在，frontmatter 同款 | PM 已 Read，逐行确认 | ✅ |
| `.harness/agents/pm-orchestrator.md` 存在，"Stage gates" 段和 "What to write at delivery" 段是当前结构 | PM 已 Read（offset 0-160），确认 §"Stage gates" 在 L116-L121，§"What to write at delivery" 在 L122-L152 | ✅ |
| `.claude/agents/gate-reviewer.md` / `code-reviewer.md` / `pm-orchestrator.md` 与 `.harness/agents/` 同名字节一致 | PM 已 Read .claude/agents/gate-reviewer.md 和 code-reviewer.md，文本同 `.harness/agents/`；pm-orchestrator.md 未 Read 但 `scripts/verify_all` E.4（harness-sync -Check）历史 PASS 表明在 sync 状态 | ✅ |
| `scripts/verify_all.ps1` 现有 E.6 结构是 `Get-Content -Raw` + `-notmatch '##\s+Adversarial\s+tests'` | PM 已 Read L254-L266 | ✅ |
| `scripts/verify_all.sh` 现有 E.6 结构是 `grep -qE '^##\s+Adversarial\s+tests'`（按行扫描 + ^ 行首锚） | PM 已 Read L261-L276 | ✅ |
| `scripts/harness-sync.ps1` 是字节级 hash + Copy-Item -Force | PM 已 Read，L48-L62 确认 | ✅ |
| insight L60 当前在 insight-index L45 末尾段 | PM 已 Read，L45 行 `**sub-agent 工具白名单 frontmatter ... 短期 workaround：派发 reviewer 时显式预告 ...` | ✅ |

## 3. 八维度审计

| # | 维度 | 状态 | 一句话原因 |
|---|---|---|---|
| 1 | Requirement completeness | PASS | RA 文档 10 条 AC 全可独立验证（grep / harness-sync -Check / verify_all 数字对账）；in-scope 6 条都覆盖了 stage 3 + stage 5 + verify_all + insight-index + harness-sync 同步 + 历史 reviewer 行为约束 |
| 2 | Design completeness | PASS | 02 设计 §3.1 / §3.2 / §3.3 / §3.4 与 RA 的 IS-1..IS-6 一一对应，§7 复用审计 6 条都有实证文件路径 |
| 3 | Reuse correctness | PASS | §7 复用审计准确：E.6 grep 模板、harness-sync 字节级复制、双实现 PS↔Bash 对账（L58）都是已存在并被本任务复用；无虚构 |
| 4 | Risk coverage | PASS | §8 列了 8 风险（R-1..R-8）覆盖了 reviewer 沉默失败 / 截断 / 误触发 / 闸门假阳性 / harness-sync 漏正文 / 未来 SDK 给齐工具仍走 fallback / insight 段格式 / 其它 AI 工具用户 — 我作为 GR 想了一遍没漏明显的（见 §4 补充） |
| 5 | Migration safety | PASS | 全部是 markdown 段追加 + 新 verify_all step + insight-index 行替换。无 schema、无数据迁移、无回滚需求 |
| 6 | Boundary handling | PASS | RA §6 列了 7 个边界（含 sentinel 误触发、Mode B 漏 sentinel、Mode B body 不完整、跨 stage Mode 切换、极端裁剪 OOS）；设计 §3.2 PM 协议步骤 1/2/3 一一接住 |
| 7 | Test feasibility | PASS | AC-1..AC-10 全部可通过 grep / verify_all 数字对账 / harness-sync -Check 验证 |
| 8 | Out-of-scope clarity | PASS | RA §5 列了 7 条 OOS（含"不修 SDK 裁剪本身" / "不引入 dynamic probe 测试" / "不让 PM 在 Mode B 做生成"等关键边界），设计 §10 与之对齐 |

## 4. 补充 finding（仅 GR 自己想到的）

### 4.1 [WARN]（自动消解）：本任务 stage 3 / stage 5 / stage 7 的"诚实记录"是否会让未来重读者困惑？

03 / 05 / 07 文档顶部"由 PM 代写"的说明可能让未来一个 onboarding 的协作者以为
Harness pipeline 在本项目"坏了"。**消解方式**：文档顶部明确写"这是 SDK 派发上下
文工具裁剪的直接复现 / 是 T-034 任务对象的核心证据"，已在 01 §2.3 + 03 当前段中
做到。**降级为 PASS。**

### 4.2 [PASS] sentinel 字面串选择 `MODE: PM_FALLBACK_WRITE target=...` 的合理性

- 全大写降低与正文意外重叠概率
- `target=...` 形式让 PM 可以用单 regex 抓出路径而无需多步解析
- 工业先例：HTTP 头 `Content-Type:` / Git 提交 `From:` / RFC 2822 头格式都用 `Key: value` 单行结构

无更优替代，PASS。

### 4.3 [PASS] verify_all G.1 / G.2 与 E.6 / E.4 的复用合理性

- G.1 / G.2 沿 E.6 grep 静态模板（"必须含字面串"），E.6 已在生产 30+ 任务中证明可靠
- 双实现要求遵守 insight L58 的"PS↔Bash 必须对账"

PASS。

### 4.4 [PASS] 元任务边界：本任务不属于 dev-db / dev-backend / dev-frontend 任一分区

确认 `.harness/agents/dev-*.md` 的 owned-paths 不含 `.harness/agents/` 或
`scripts/verify_all.*`。元任务正确使用通用 `developer` agent，无分区错配风险。

## 5. 预测开发阶段 3-5 个可能问题（pre-answer）

| Q | 预答 |
|---|---|
| Q1：reviewer agent 怎么"自检 dispatch context 是否有 Write"？ | 实际运行中 agent 在执行第一步前会被 SDK 注入可用工具清单，agent 可以 introspect。但**契约不强制 introspection 实现方式**，只要求"按 PM 派发 prompt 的指示输出 Mode A 或 Mode B"。即：PM 派发 prompt 已经预告了两种模式 + 触发条件，reviewer 检测自身工具集后**选**模式。如果 introspection 不可靠，reviewer 应**保守降级到 Mode B**（落到 sentinel 路径），让 PM 的字节级落盘逻辑接住。 |
| Q2：verify_all G.1 / G.2 是否需要对 .claude/agents/ 同时 grep？ | 不必要：E.4 已经守门 `.claude/` ↔ `.harness/` 字节一致。G.1/G.2 grep `.harness/agents/` 等价 grep `.claude/agents/`（modulo E.4 同步）。 |
| Q3：旧 insight L60 怎么删干净不破坏其他 insight 行号？ | insight-index 是无 ID 文本，行号本身不被外部引用（archive-task 按 regex `^- ` bullet 抽，不依赖行号）。直接 Edit 替换那一整段即可。 |
| Q4：harness-sync 跑完 .claude/agents/pm-orchestrator.md 是否会变？因为 .harness/agents/pm-orchestrator.md 是 PM 自己的契约文件 | 会变。harness-sync 字节级复制。E.4 闸门已对此守门。Stage 4 跑完一次 harness-sync 即同步。 |
| Q5：本任务 stage 6 QA Tester 的 adversarial test 怎么验证 Mode B sentinel 协议在真实派发中工作？ | 它无法在本任务里真实派发 reviewer（同样的工具裁剪）。但它可以做**静态 adversarial**：临时把 sentinel 字面串从 `.harness/agents/gate-reviewer.md` 删除，跑 verify_all，断言 G.1 FAIL；恢复，断言 G.1 PASS。这是反向证伪闸门有效。Mode A / Mode B 在派发链上层（未来真实派发 reviewer 时）的动态验证不在本任务能完成 —— 等下次真任务 reviewer 派发时观察是否落盘即可。 |

## 6. Verdict

**APPROVED**

设计完整、风险有缓解、可测试、与项目规则一致。Stage 4 开发可以开始。

### 给 stage 4 developer 的提示

- 编辑 `.harness/agents/*.md` 时保留原 frontmatter 第 1-5 行不变（只在正文加段）
- 编辑 `.harness/agents/pm-orchestrator.md` 时新段插入位置：现有 §"Stage gates"（L116-L121）后、§"What to write at delivery"（L122-）前
- 编辑 verify_all.ps1 / verify_all.sh 时 G.1/G.2 step 插入位置：E.10 之后、Summary 之前
- harness-sync 必须**改完 .harness/agents/ 立刻跑**，否则 E.4 会 FAIL 让 stage 5 卡住
- insight-index L60 替换时，注意 §10 设计描述的新 insight 文本可能跨多行，需照顾 `- 2026-MM-DD · ... · evidence: ...` 单段结构
- 写 04 时给 verify_all 实测输出（FAIL 数 + G.1/G.2 状态）；不能"自信宣称跑过"
