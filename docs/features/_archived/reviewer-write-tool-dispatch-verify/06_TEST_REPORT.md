# 06 Test Report — T-034 reviewer-write-tool-dispatch-verify

> **产出方式说明**：本文档由 PM Orchestrator 在 SDK Opus 派发上下文中代写
> （pipeline-executor 模式）。QA 派发同样被工具裁剪（PM 缺 Task 工具派发
> qa-tester sub-agent）。

## 1. Test plan

本任务的"实现"全部是**静态文档 + 静态闸门**。测试策略只能是**静态闸门反向证伪**
+ **Grep 命中等价验证**。本派发上下文无 Bash / PowerShell，无法跑 verify_all 全
量；改用 Grep 工具对 G.1 / G.2 静态闸门做等价单元验证 + 反向证伪（adversarial）。

| Acceptance criterion | Test case | 方法 |
|---|---|---|
| AC-1 gate-reviewer.md 含 sentinel | T-A1 Grep `MODE: PM_FALLBACK_WRITE` | Grep tool 静态 |
| AC-2 code-reviewer.md 含 sentinel | T-A2 同款 grep | Grep tool 静态 |
| AC-3 pm-orchestrator.md 含 `Reviewer dispatch protocol` 段 | T-A3 grep 段标题 | Grep tool 静态 |
| AC-4 verify_all.ps1 含 G.1 + G.2 | T-A4 grep step 描述 | Grep tool 静态 |
| AC-5 verify_all.sh 含同款 G.1 + G.2 | T-A5 grep step 描述 | Grep tool 静态 |
| AC-6 .claude/ ↔ .harness/ 字节一致 | T-A6 状态检查（见 §6）| 状态记录 |
| AC-7 insight-index 替换 L60 为 T-034 长期解 | T-A7 grep "evidence: T-034" + 新条目关键串 | Grep tool 静态 |
| AC-8 03/05 顶部含"派发上下文实情" | T-A8 grep 顶部短语 | Grep tool 静态 |
| AC-9 verify_all FAIL ≤ 基线 2 | T-A9 静态闸门只加 step、不改既有 step ⇒ 预测 FAIL 不上涨；stop-hook 跑后确认 | 静态分析 + 后续实测 |
| AC-10 07 含 `## Adversarial tests` + `## Insight` 段（裸标题 + bullet 列表） | T-A10 stage 7 PM 自查后报数 | stage 7 任务 |

## 2. Functional correctness（正向单元验证）

| ID | 命令（等价 Grep） | 期望 | 实际 | Status |
|---|---|---|---|---|
| T-A1 | Grep `MODE:\s*PM_FALLBACK_WRITE` in `.harness/agents/gate-reviewer.md` | ≥1 hit | L54 hit | ✅ PASS |
| T-A2 | Grep `MODE:\s*PM_FALLBACK_WRITE` in `.harness/agents/code-reviewer.md` | ≥1 hit | L36 hit | ✅ PASS |
| T-A3 | Grep `Reviewer\s+dispatch\s+protocol` in `.harness/agents/pm-orchestrator.md` | ≥1 hit | L122 hit | ✅ PASS |
| T-A3b | Grep `MODE:\s*PM_FALLBACK_WRITE` in `.harness/agents/pm-orchestrator.md` | ≥1 hit | L135, L144 共 2 hit | ✅ PASS |
| T-A4 | Grep `G\.1.*PM_FALLBACK_WRITE` in `scripts/verify_all.ps1` | ≥1 hit | L400 hit | ✅ PASS |
| T-A4b | Grep `G\.2.*Reviewer dispatch protocol` in `scripts/verify_all.ps1` | ≥1 hit | L419 hit | ✅ PASS |
| T-A5 | Grep `G\.1.*PM_FALLBACK_WRITE` in `scripts/verify_all.sh` | ≥1 hit | L387/L389/L402/L404 共 4 hit | ✅ PASS |
| T-A5b | Grep `G\.2.*Reviewer dispatch protocol` in `scripts/verify_all.sh` | ≥1 hit | L408/L411/L421/L423 共 4 hit | ✅ PASS |
| T-A7 | Grep `evidence: T-034` in `.harness/insight-index.md` | ≥1 hit | L45 行末 hit | ✅ PASS |
| T-A7b | Grep `SDK 派发上下文对 sub-agent 工具集做二次裁剪` in `.harness/insight-index.md` | ≥1 hit | L45 行首 hit | ✅ PASS |
| T-A8 | Grep `产出方式说明` in 03/05 文档 | 各 ≥1 hit | 03 L3 hit, 05 L3 hit | ✅ PASS |

## 3. Boundary tests

| 边界 | 测试 | 实际 |
|---|---|---|
| sentinel 字面串出现在多文档中 | grep 全仓库 `MODE: PM_FALLBACK_WRITE` 是否只在受控位置 | 命中 7 个位置全部在受控源（gate-reviewer 1 + code-reviewer 1 + pm-orchestrator 2 + verify_all.ps1 1 + verify_all.sh 5 + 02 设计 + 03/04/05 stage 文档；所有命中都是"声明用法"，不构成 PM 误触发） |
| sentinel 行精确格式 | 设计要求"第一行精确匹配 + 第二行空"；PM 协议 regex `^MODE: PM_FALLBACK_WRITE target=(\S+)$` | regex 设计正确：`^/$` 锚行首/尾、`\S+` 不含空格捕获 target、严格单行；不会误触正文里同串 |
| Mode B body 不完整 | PM 协议步骤 3 要求"不补全、不编"，仅 re-dispatch | 文本明确，反例（编内容）违反契约可被人类 reviewer 发现 |
| reviewer 误判 Mode A 但实际没 Write | PM 协议步骤 2 要求"打开文件确认存在" | 文本明确："Open the file to verify it exists; if not, treat as BLOCKED ON DISPATCH" |

## Adversarial tests

> 本任务的 acceptance criteria 大多是 "grep 命中"。Adversarial 测试 = **反向证
> 伪闸门有效性**：临时破坏闸门期望的字面串，验证 G.1 / G.2 会真的捕获，证明闸门
> 不是 trivial 假阳性 PASS。

**核心原则（QA agent 契约 §"Adversarial mindset"）**：每个 AC 至少一条独立可复
现的、由 QA 自己写的失败假设 + 实测 + 工具输出。

| AC | 失败假设 | Reproducer | Outcome (tool 实测) |
|---|---|---|---|
| AC-1 / AC-4 / AC-5 (G.1 闸门) | "我猜：如果 gate-reviewer.md 失去 sentinel，G.1 应当 FAIL（grep 命中数 = 0）" | (1) Edit gate-reviewer.md L54 `MODE: PM_FALLBACK_WRITE target=...` → `<ADVERSARIAL_PROBE_SENTINEL_REMOVED_TEMPORARILY>`；(2) Grep `MODE:\s*PM_FALLBACK_WRITE` in gate-reviewer.md → count 模式 | **实测命中 0**（QA 派发上下文 Grep tool 实际跑过）⇒ G.1 verify_all step 在跑时会 throw `Reviewer two-mode protocol missing in: gate-reviewer.md (no PM_FALLBACK_WRITE sentinel)`。验证后立即 Edit 恢复，再 Grep `MODE:\s*PM_FALLBACK_WRITE` 命中数 = 1 ⇒ G.1 重新会 PASS。**反向证伪成功，G.1 闸门非假阳性。** |
| AC-3 / AC-4 / AC-5 (G.2 闸门) | "我猜：如果 pm-orchestrator.md 段标题改字（破坏 G.2 grep 锚），G.2 应当 FAIL" | (1) Edit pm-orchestrator.md L122 `## Reviewer dispatch protocol (T-034)` → `## Reviewer dispatch flow (T-034)`；(2) Grep `Reviewer\s+dispatch\s+protocol` count | **实测命中 0** ⇒ G.2 verify_all step 在跑时会 throw `PM Orchestrator reviewer dispatch protocol incomplete: missing 'Reviewer dispatch protocol' heading`。恢复后 Grep 命中数 = 1 ⇒ G.2 重新会 PASS。**反向证伪成功，G.2 闸门非假阳性。** |
| AC-2 (code-reviewer sentinel) | "我猜：与 AC-1 对称，code-reviewer.md 失去 sentinel 同款会 FAIL" | 同 AC-1 模式，对 code-reviewer.md 操作 | **未跑物理破坏**（与 AC-1 同款 G.1 闸门 + 同款 grep regex + 同款一份独立文件，反向证伪 AC-1 已等价覆盖；不重复打破已恢复的契约文件，避免误 commit） |
| AC-7 (insight-index 替换) | "我猜：若新条目漏 `evidence: T-034`，未来 grep 检索 evidence 无法追溯到本任务" | Grep `evidence:\s*T-034` in insight-index.md | **实测命中 L45**（PM Grep tool 实际跑过）⇒ evidence 标签到位。**正向已验证，无需破坏证伪**（insight-index 非闸门、无 verify_all step） |
| AC-9 (FAIL 不上涨) | "我猜：若 G.1/G.2 实现 bug，会引入新 FAIL；若仅加 step 不改既有 step，FAIL 不会增加" | 静态分析 verify_all 改动 diff | verify_all.ps1 改动**仅插入两个新 Step block 到 Summary 段之前**，未触碰任何既有 Step；verify_all.sh 改动同款仅插入两个新 step block。**静态结论：FAIL 数不会上涨**。未来 stop-hook / 用户跑 verify_all 实测确认 |
| AC-6 (.claude ↔ .harness 字节一致) | "我猜：PM 上下文缺 Bash 无法跑 sync，当前 commit `.claude/agents/*.md` 与 `.harness/agents/*.md` 在 3 处（gate-reviewer / code-reviewer / pm-orchestrator）字节有 drift" | Read `.claude/agents/gate-reviewer.md` 看是否含"Two-mode output protocol"段 | 04 §5 + 05 AC-6 已记录 deferred 状态；stop-hook 跑 harness-sync 后达成。**当前 commit 瞬间确实 drift，符合预测**；这是任务级 known issue，由项目契约支持的 stop-hook 自动消解，不在本任务可解范围 |

## 5. Independent reproducer 说明

QA 契约要求"独立 reproducer 而非 dev 测试"。本任务 reproducer 全部由 QA 阶段（PM
代写）独立写出：

- AC-1/2/3/4/5/7/8 reproducer = Grep 命令字面，**和 dev 写的 verify_all step 实现
  字面（PowerShell `-match 'MODE:\s*PM_FALLBACK_WRITE'`）共享 regex**。这不构成
  "共享假设"风险，因为 regex 本身是契约的一部分；它和 dev 同款是设计要求，**不是
  bug 共享路径**。
- AC-1/AC-3 adversarial 反向证伪是**真物理破坏 → grep → 恢复 → grep** 的端到端
  动作，与 dev 测试完全独立。
- AC-6 + AC-9 的 deferred 状态在 04 + 05 已诚实记录，QA 不在本上下文重跑（无 Bash/
  PowerShell）；stop-hook + 用户接手验证。

## 6. verify_all result（实测 deferred）

**未在本上下文实跑**：PM 缺 Bash / PowerShell tool。

预测（基于 static analysis + grep 等价验证）：

- 基线（任务前）: FAIL = 2 (C.1 playwright + E.6 历史 06 标题违规，来自 insight L58 / L59)
- 本任务改动后:
  - 新增 G.1 PASS + G.2 PASS（基于 Grep 实测命中等价证据）
  - 不动既有 step → 既有 FAIL 数不变
- 净结果预测: FAIL 仍 = 2，PASS +2，符合 AC-9 "FAIL ≤ 基线 2"

**Stop-hook / 用户跑时实测**：若 FAIL > 2 必须重审 G.1/G.2 实现 bug；若 FAIL = 2
+ G.1/G.2 PASS → 任务 DECLARE_DONE 闸门通过。

## 7. Defects found

无 BLOCKER / CRITICAL / MAJOR / MINOR 缺陷。

**已知 known issue（非缺陷，是上下文限制）**：

- `.claude/agents/*.md` 与 `.harness/agents/*.md` 在 commit 瞬间 drift（3 个文件）
- verify_all 未在本上下文实跑

二者都由 stop-hook 自动消解（项目契约支持，CLAUDE.md 红线 #7 / `.harness/rules/00-core.md`
§"红线 7"）。不阻塞本任务交付。

## 8. Stability

测试方法全部是 Grep 工具单次确定性命中检查，**无 flakiness 风险**（grep 是
deterministic）。adversarial 反向证伪步骤 Edit → Grep → Edit-restore → Grep 也都
有确定性结果，已实测验证。

## 9. Verdict

**APPROVED FOR DELIVERY**

- AC-1..AC-5、AC-7、AC-8 实测全 PASS（11 个 grep test + 2 个 adversarial 反向证伪）
- AC-6、AC-9 deferred 到 stop-hook，已在 04 / 05 / 06 §6 诚实记录预测 + 验证路径
- AC-10 是 stage 7 PM 自己执行（写 07 + grep 自查 `## Adversarial tests` + `## Insight`）

**给 stage 7 PM 的提示**：
- 07 必须有 `## Adversarial tests` 段（**裸标题，无 §N 数字前缀**，insight L58 陷阱）
- 07 必须有 `## Insight` 段，**bullet 列表格式**（一条一行 `- `，**不要** `### Insight 1: ...` 子标题，insight L57 陷阱）
- 收割前自 grep `^## Insight\s*$` + `^## Adversarial tests\s*$` 各 ≥1 hit
- archive-task 跑前再次确认 verify_all FAIL 数（让 stop-hook 跑后实测，或在 07
  里诚实标注 deferred）
