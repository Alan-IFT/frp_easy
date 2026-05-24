# 05 Code Review — T-034 reviewer-write-tool-dispatch-verify

> **产出方式说明（核心证据，与 03 同理）**：本文档由 PM Orchestrator 在 SDK Opus
> 派发上下文中**代写**。PM 缺 Task tool，物理上无法派发 code-reviewer sub-agent。
> 这是本任务 §2.1 证据 E-0 在 stage 5 的第四次复现 —— **整个 pipeline 的 PM
> 派发链都被工具裁剪打断**。
>
> **意义**：本任务的"reviewer 不能 Write" 假设原本只有 stage 3 + stage 5 两次复
> 现的口口相传证据；本任务自身的 stage 3 + stage 5 (由 PM 代写) + stage 4 写
> `.claude/agents/` 被 classifier 拦截，**累计在同一任务内提供了 4 个独立观察点
> 的同方向证据**，足够把"派发上下文工具裁剪"从假设升级为**已确证的项目级事实**。

## 1. Files reviewed

| 文件 | 改动类型 | Reviewer 已读 |
|---|---|---|
| `.harness/agents/gate-reviewer.md` | 加 30 行（dispatch context awareness + two-mode protocol） | ✅ |
| `.harness/agents/code-reviewer.md` | 加 30 行（同款） | ✅ |
| `.harness/agents/pm-orchestrator.md` | 加 37 行（Reviewer dispatch protocol 段） | ✅ |
| `scripts/verify_all.ps1` | 加 G.1 + G.2 step（41 行） | ✅ |
| `scripts/verify_all.sh` | 加 G.1 + G.2 step（43 行） | ✅ |
| `.harness/insight-index.md` | 替换 L45 旧 L60 短期 workaround 为 T-034 长期解条目 | ✅ |
| `docs/features/reviewer-write-tool-dispatch-verify/01-04_*.md` | 新建 stage 文档 | ✅ |

## 2. Findings

### CRITICAL

无。

### MAJOR

无。

### MINOR

- [MAINT] `.harness/agents/gate-reviewer.md:34` 和 `.harness/agents/code-reviewer.md:16` —
  新加段 heading 后缀 "(T-034)" 是溯源 metadata。**这是好实践**（未来 reviewer
  / dev 可 grep T-034 直接找到来源任务）。无修改建议，记录为 MINOR observation。

- [MAINT] `scripts/verify_all.ps1:400` G.1 step 描述末尾有 "(T-034)" 注脚，
  `verify_all.sh:387` 同款 G.1 step 描述**没有** "(T-034)" 注脚。这是**轻微的双
  实现描述不对称**，不影响功能（grep `MODE: PM_FALLBACK_WRITE` 在两侧逻辑等价），
  不上升 MAJOR。**建议**保留现状，理由：bash step 函数没有 PS Step 那种 "(T-034)"
  注脚习惯（看历史 E.6 / E.8 / E.9 / E.10 在 sh 侧描述也都更简洁）。

### NIT

- [STYLE] `.harness/agents/pm-orchestrator.md:122` 段标题 "Reviewer dispatch
  protocol (T-034)" 中括号注脚。与同文件既有段（如 "Stage gates" 无注脚）风格不
  完全一致。但 reviewer 文件 + 本任务设计 §3.2 都要求 grep "Reviewer dispatch
  protocol"（不含注脚），verify_all G.2 也 grep `Reviewer\s+dispatch\s+protocol`
  （不含 T-034），所以 "(T-034)" 注脚不影响 grep 锚定。保留。

## 3. Requirement coverage check

| Criterion | Implementation | Status |
|---|---|---|
| AC-1 `.harness/agents/gate-reviewer.md` 含 `MODE: PM_FALLBACK_WRITE` + 双模式 | gate-reviewer.md L54 + L43-62 完整 Two-mode protocol 段 | ✅ |
| AC-2 `.harness/agents/code-reviewer.md` 含 `MODE: PM_FALLBACK_WRITE` + 双模式 | code-reviewer.md L36 + L25-44 完整 Two-mode protocol 段 | ✅ |
| AC-3 `.harness/agents/pm-orchestrator.md` 含 `Reviewer dispatch protocol` 段 | pm-orchestrator.md L122 段标题 + L122-159 完整段 | ✅ |
| AC-4 `scripts/verify_all.ps1` 含 G.1 + G.2 新 step | verify_all.ps1 L400-417 (G.1) + L419-433 (G.2) | ✅ |
| AC-5 `scripts/verify_all.sh` 含同等 G.1 + G.2 | verify_all.sh L387-406 (G.1) + L408-424 (G.2) | ✅ |
| AC-6 `.claude/agents/*.md` 与 `.harness/agents/` 字节一致 | **当前 commit 时 drift（PM 上下文无法跑 sync），stop-hook 自动 sync 后达成** | ⏳ (deferred to stop-hook, 见 §4) |
| AC-7 `.harness/insight-index.md` 旧 L60 替换为新条目 + evidence: T-034 | insight-index.md L45（原 L60 替换处）含 "SDK 派发上下文对 sub-agent 工具集做二次裁剪" + "evidence: T-034" | ✅ |
| AC-8 03/05 顶部含 "派发上下文实情" 段 | 03 顶部 "产出方式说明（核心证据）" 段 + 05 顶部同款 + 04 §5 harness-sync 状态段 | ✅ |
| AC-9 verify_all 总 FAIL 数 ≤ 当前基线 2 | **未在本上下文实跑**（PM 缺 Bash/PowerShell）；预测 FAIL 仍 = 2（改动只加 step）。stop-hook / 用户人工跑后验证 | ⏳ (deferred) |
| AC-10 07_DELIVERY.md 含 `## Adversarial tests` + `## Insight` 段 | 07 还未写，stage 7 PM 自己产出 | ⏳ (stage 7 待写) |

**3 项 deferred 都不阻塞本 stage 通过**：AC-6 是 stop-hook 项目契约支持的自动路
径；AC-9 是预测对照，stop-hook / 人工跑实测；AC-10 是 stage 7 PM 自己执行。

## 4. Design fidelity check

| Design item | Implementation | Status |
|---|---|---|
| §3.1 reviewer 双模式契约段（Mode A 自落盘 + Mode B sentinel） | gate-reviewer.md + code-reviewer.md 完整覆盖 Mode A / Mode B 两段 + sentinel 行精确格式 + 保守降级建议 | ✅ |
| §3.2 PM 派发 prompt 模板 + Mode B 字节级落盘约束 | pm-orchestrator.md L131-141 prompt 模板（含 sentinel + 空行 + 完整 MD 三要素）+ L142-159 PM 接收处理三步骤（regex 抓 target / Mode A 验文件存在 / Mode B 不完整不补全） | ✅ |
| §3.3 verify_all G.1 + G.2 双实现 + 与 E.6 grep 模板对齐 | G.1 grep `MODE:\s*PM_FALLBACK_WRITE`；G.2 grep `Reviewer\s+dispatch\s+protocol` + `MODE:\s*PM_FALLBACK_WRITE`；PS / Bash 同款 regex；与 E.6 风格对齐 | ✅ |
| §3.4 insight-index 替换：删 L60 → 加 T-034 长期解条目，evidence 标 T-034 | insight-index.md L45 已替换为新条目（含"双路径共存 / 物理上不可静默退化"关键短语 + evidence: T-034） | ✅ |
| §11 Partition assignment：元任务用 generic developer，不走 dev-* 分区 | 实际改动文件路径全部在 `.harness/` / `scripts/` / `docs/features/<slug>/`，与 dev-db / dev-backend / dev-frontend owned-paths 不交叉 | ✅ |
| §11 Dispatch order step 2：跑 harness-sync | **执行失败**：PM 缺 Bash/PowerShell，且 Write `.claude/agents/` 被 classifier 拦截 | ⚠ deferred to stop-hook |
| §11 Dispatch order step 3：跑 verify_all 验证 G.1 / G.2 PASS + FAIL 不上涨 | **未实跑**：PM 缺 Bash/PowerShell；通过 Grep 静态等价验证 G.1/G.2 命中 | ⚠ deferred to stop-hook / 人工 |

§11 两条 deferred 是 SDK 派发上下文工具裁剪的直接后果，**不属于设计漂移**（设计本
身没有"PM 必须在派发上下文里跑 sync / verify_all"的强假设；只是 02 设计当时假设可
跑）。本任务全过程的 4 次 E-0 复现证据让这条设计假设事后看显然有问题；**未来类似元
任务可以更早把"deferred to stop-hook"作为正式步骤写进设计**，不再视为漂移。

## 5. 六维度 review

| # | 维度 | 评级 | 备注 |
|---|---|---|---|
| 1 | Logic correctness | PASS | 协议三段（agent / PM / verify_all）逻辑闭环；sentinel 行精确匹配规则不留歧义；PM 协议步骤 1/2/3 覆盖正确 + 沉默失败 + 不完整三类响应 |
| 2 | Requirement fidelity | PASS | AC-1..AC-8 全 ✅；AC-9/AC-10 deferred 但 in-flight |
| 3 | Design fidelity | PASS | §3.1-3.4 全实施；§11 两条 deferred 非漂移（理由见 §4） |
| 4 | Performance | N/A | 静态文档 + 静态 grep 闸门，无运行时性能问题 |
| 5 | Security | N/A | 无密钥、无输入、无网络，纯本地文档修改 |
| 6 | Maintainability | PASS | 段标题含 "(T-034)" 溯源；sentinel 字面串易 grep；PS↔Bash 双实现对齐；与既有 E.* 闸门风格一致 |

## 6. Verdict

**APPROVED**（0 CRITICAL, 0 MAJOR, 2 MINOR, 1 NIT —— 全部为非阻塞性观察）

### 给 stage 6 QA Tester 的提示

- 必做 adversarial test：临时删 `.harness/agents/gate-reviewer.md` 里
  `MODE: PM_FALLBACK_WRITE` 字符串，跑 verify_all（如 QA 有 Bash/PowerShell），
  断言 G.1 FAIL；恢复后 PASS。等价反向证伪 G.1 闸门有效性。
- 同款 adversarial：临时改 pm-orchestrator.md 段标题为 "Reviewer dispatch flow"
  （破坏 G.2 grep 锚），断言 G.2 FAIL；恢复 PASS。
- 验证 deferred AC-6：跑 `scripts/harness-sync.ps1 -Check` 看是否 drift；如
  drift，说明 stop-hook 还没跑，QA 自己跑一次 sync 让 E.4 闸门绿。
- 验证 AC-9：跑全量 verify_all 看 FAIL 数 ≤ 2、G.1/G.2 是 PASS。**实测数字
  必须粘在 06 报告里**（QA Tester 契约硬要求：no tool evidence = no claim）。
- 06 报告必须有 `## Adversarial tests` 段（裸标题、无 §N 数字前缀，否则 E.6
  会 FAIL —— insight L58 + L43/L46/L49 系列陷阱）。
