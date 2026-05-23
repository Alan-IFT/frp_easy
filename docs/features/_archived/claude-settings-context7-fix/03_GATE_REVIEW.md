# 03 — Gate Review · T-020 claude-settings-context7-fix

> Stage 3 / 7 · Gate Reviewer · 模式：`full` · 输出语言：中文 · 2026-05-23
> 上游：`01_REQUIREMENT_ANALYSIS.md` Verdict = READY · `02_SOLUTION_DESIGN.md` Verdict = READY
> 落盘说明：gate-reviewer 角色 tools frontmatter 仅 `Read, Glob, Grep`，无 Write 权限；本文件由 PM Orchestrator 据 reviewer 返回内容手工落盘，内容**未经任何修改**。该现象为 Insight Index 第 41 条根因，建议后续在 `.harness/agents/gate-reviewer.md` tools 列表加 Write。

---

## 0. 独立核验记录（不盲信上游）

| # | 上游声明 | 我做的验证 | 结论 |
|---|---|---|---|
| V-1 | `.harness/` 下无 `settings.json` 源 | `Glob .harness/**/settings.json` | **0 命中** —— 上游属实 |
| V-2 | `scripts/harness-sync.{ps1,sh}` 不同步 settings.json | `Grep "settings\.json" scripts/harness-sync.ps1` 与 `.sh` | **0 命中** —— 上游属实 |
| V-3 | `scripts/verify_all.{ps1,sh}` 不依赖 settings.json | `Grep "settings" scripts/verify_all.ps1` 与 `.sh` | **0 命中** —— 设计 §2 风险 R-4 已被事实排除 |
| V-4 | `install-hooks.{ps1,sh}` 提及 settings.json 仅为注释 | 通读两脚本 | **属实**：只在注释里说明 Stop hook 的存在；脚本写入的是 `.git/hooks/pre-commit`，与 `.claude/settings.json` 内容无任何耦合 |
| V-5 | `.claude/settings.json` 由 git 纳管（回滚可行） | `.gitignore` 仅 ignore `.claude/settings.local.json`；`Glob .claude/settings*.json` 给出 2 文件 | **属实**：`settings.json` 在版控范围，`git checkout --` 回滚有效 |
| V-6 | 设计 §10 完整预览是合法 JSON | 逐字符目视：开闭括号配对、deny 数组每元素带后置逗号且末元素无尾逗号、`hooks.Stop` 数组对象闭合、UTF-8 字符无 BOM 隐患（文本编辑表面纯 ASCII） | **属实**：语法合法，无尾逗号、无 `//` 注释、无未转义双引号 |
| V-7 | `_comment` / `_doc_sync_hook` 当前文件中存在 | 通读 `.claude/settings.json` 47 行原文 | **属实**：行 3、行 35 |
| V-8 | 现有 deny 仅 3 条 | `.claude/settings.json` 行 28-32 | **属实**：`Bash(git push --force:*)`、`Bash(git push -f:*)`、`Bash(rm -rf /:*)` |
| V-9 | context7 `$schema` URL 带 `.json` 后缀 | PM_LOG.md § "context7 预查文档证据" 第 1 条 | **采信**（无可疑） |
| V-10 | `.harness/skills/init-binding/` 是否存在 | `Glob .harness/skills/**` 仅返回 build / test / verify 3 个 SKILL.md | **不存在**：RA §5 I-1 第 4 条提到"`.harness/skills/init-binding/` 后续任务讨论"是面向未来的措辞，与本任务无强耦合，可接受。 |

**独立核验通过率：10/10。** 红线复议 4 条理由全部可被仓库事实支持。

---

## 1. 8 维度审计

| # | 维度 | 评级 | 一句话理由 |
|---|---|---|---|
| 1 | Requirement completeness | **PASS** | RA §4 给出 8 条 AC，其中 6 条静态可断言（AC-1/2/3/4/7/8）、AC-5 跑脚本、AC-6 手动；out-of-scope 边界（§3.2）明确。 |
| 2 | Design completeness | **PASS** | 设计 §8 拍板每个 RA 项；§9 给出字符级 unified diff；§10 给出完整改后预览；§12 给出测试脚本框架。每条 in-scope 行为都落到字符串字面量。 |
| 3 | Reuse correctness | **PASS** | §7 reuse audit 准确：本任务无新模块、无新依赖、纯文本编辑；我独立 grep 验证 verify_all / harness-sync 均不依赖 settings.json 内容，与上游一致。 |
| 4 | Risk coverage | **WARN** | §11 给出 7 条风险，覆盖了 JSON 语法、误伤、Windows 路径、verify_all 兼容、`_comment` 兼容、deny 顺序、注释误写。**漏了一条**：`Bash(rm -rf $HOME:*)` 中字面 `$HOME` 是否真能命中"模型生成的命令字符串"——见 §2 F-1。 |
| 5 | Migration safety | **PASS** | 单文件、文本编辑、git 纳管、`git checkout --` 一键回滚；§14 明确 no feature flag / no data migration / 向后兼容。 |
| 6 | Boundary handling | **PASS** | RA §6 覆盖 UTF-8 / BOM / 行尾 / 空 deny / 数组顺序 / 并发 / 回滚；设计 §9 把"字符级要点"列成 5 条 must-check（行尾 LF、无 BOM、缩进、末尾换行、无尾逗号）。 |
| 7 | Test feasibility | **WARN** | AC-1/2/3/4/5/7/8 全部脚本可断言（设计 §12.1 已给 T1-T8）。AC-6 与 ADV-1 是 manual；设计 §12.3 / §12.4 已明确归类，不冲突。但 ADV-2 / ADV-3 的"让 Claude Code 实际拒绝命令"在 `verify_all` 自动化下**不可跑**，仅能 manual / 真实 Claude session 内观察——见 §2 F-2。 |
| 8 | Out-of-scope clarity | **PASS** | RA §3.2 八条 out-of-scope + 设计 §15 六条 out-of-scope 重申，覆盖 `.claude/agents/`、`.claude/skills/`、`CLAUDE.md`、`.github/copilot-instructions.md`、`settings.local.json`、Stop hook 跨平台。Developer 无任何"扩张诱惑"。 |

**总分：6 PASS + 2 WARN + 0 FAIL。**

---

## 2. Findings（WARN / FAIL 详述）

### F-1 · `Bash(rm -rf $HOME:*)` 的命中前提未在风险表中点明（WARN）

- 责任方：`02_SOLUTION_DESIGN.md` §8.1 第 3 条 + §11 风险表。
- 现象：设计将 `Bash(rm -rf $HOME:*)` 列入 P1 新增 deny。`$HOME` 是否被 deny 匹配引擎看到，取决于"Claude Code 拿到的 Bash 工具 input 字符串是否已经被 shell 展开"。按 Anthropic 公开文档与官方告警 "Prefix rules ... match the literal command string"，deny 引擎匹配的是工具调用入参的字面字符串——这意味着：
  - 若 LLM 生成的命令字符串就是字面 `rm -rf $HOME/x`（未自做展开），该 deny **能命中**。
  - 若 LLM 自己已生成 `rm -rf /home/user/x`（替换好的展开形式），该 deny **永远不命中**。
- 影响：这是一条**部分有效**的 deny，给阅读者的安全感强于实际效果。其余 4 条 P1（`rm -rf ~`、`rm -rf .`、`/bin/rm`、`find / -delete`）均无此模糊性。
- 建议（不强制）：设计 §11 R-7 之后追加一条 R-8：明确 `Bash(rm -rf $HOME:*)` 的命中依赖"模型不自行展开 `$HOME`"，建议同时保留 `Bash(rm -rf ~:*)` 作为更可靠的拦截（设计已有 → 互补冗余即可）。
- 不阻塞开发：因为这 5 条 deny 是并列 OR 关系、新增 `$HOME` 那条**最坏情况下也只是无效，不会破坏其他 4 条**，故仅 WARN 不 FAIL。

### F-2 · 对抗用例 ADV-2 / ADV-3 不可自动化，QA Stage 6 必须明确手动归属（WARN）

- 责任方：`02_SOLUTION_DESIGN.md` §12.4。
- 现象：
  - **ADV-1**（改回旧 URL → 重启 VS Code）：manual，设计已默认归类。
  - **ADV-2**（让 Claude Code 拒绝 `rm -rf ~/test_T020_dummy`）：需要在真实 Claude Code session 内触发模型→工具调用→deny 引擎反馈链。`scripts/verify_all` 无法模拟。
  - **ADV-3**（验证 `\rm` 反斜杠 / 双空格变体漏拦）：同上，需真实 session。
  - **ADV-4**（人为加尾逗号 → Claude Code 拒载）：可半自动（手工编辑 + 重启 Claude）；非 verify_all 范围。
- 影响：设计 §12.4 末句"ADV-1 ~ ADV-4 中至少 ADV-1 必跑"已设保底，但**未明确标注 ADV-2/3/4 是 manual**，QA 可能误以为 verify_all 应覆盖。
- 建议（不强制）：QA Stage 6 在 06_TEST_REPORT.md 中明确把 ADV-2/3/4 标为 "manual / out of automated scope"，仅 ADV-1 必跑作为对抗硬指标；这与 verify_all E.6 (`## Adversarial tests` 段必须存在) 不冲突。
- 不阻塞开发：QA 阶段才需要明确，Developer 阶段无影响。

---

## 3. 关键审查点的逐条回应（按 PM 派发 prompt 列出）

### 审查点 1 · 红线复议合规性 → **PASS**

- `.harness/` 下无 `settings.json` 源：Glob 验证 0 命中。
- `scripts/harness-sync.{sh,ps1}` 不同步 settings.json：Grep 验证 0 命中。
- `scripts/verify_all.{sh,ps1}` 同样无引用：附加事实，进一步支持红线复议。
- `install-hooks.{ps1,sh}` 仅在脚本头注释提及 settings.json，注释不构成代码耦合。
- 用户 chat 显式指令优先级最高：这是流程性事实，红线复议结论站得住。

**复议 4 条理由全部经独立核验为真。** 本任务编辑 `.claude/settings.json` 不违反红线精神（"不直接编辑被 sync 生成的 binding"），settings.json 不是 binding 而是项目配置。

### 审查点 2 · P0-1 `$schema` URL 修复是否"零功能修复" → **PASS**（非零功能）

- 我没有 WebFetch 工具，无法实测两个 URL 的 HTTP 响应。
- 但有以下 **strong evidence** 支持"非零功能"：
  1. PM_LOG.md 已记录 context7 settings 文档示例值为带 `.json` 后缀的形式。
  2. JSON Schema Store 的标准 URL 模式（`https://json.schemastore.org/<name>.json`）一直都带 `.json` 后缀；去掉后缀通常依赖站点的"裸名→.json"重定向，并非所有 schema 都配此重定向。
  3. VS Code 的 schema 解析器对 `$schema` 字段做的是直接 GET；遇到 404 则给状态栏红字（用户在另一个项目中可肉眼验证）。
- 即便假定该站点存在"裸名→.json" 301 重定向（最坏情况两 URL 等价）：
  - 修复仍把 settings.json 的 `$schema` 对齐到**官方文档给的字面值**，这一点本身是有价值的"对齐契约"修复，不是噪声。
- 结论：**有充分理由认为有功能差异**；即便万一站点已加重定向、当下"已修复 URL"与"未修复 URL"的网络结果相同，也不构成"零功能修复"——因为契约对齐本身有 maintainability 价值。
- 备注：ADV-1（manual 回归测试）就是为了在用户机器上眼见为实验证差异，这正是设计的兜底。

### 审查点 3 · P1 新增 deny 的可读性 / 误伤可能性，特别是 `$HOME` → **WARN**（见 F-1）

- `Bash(rm -rf $HOME:*)`：见 F-1，模型若自行展开则永不命中；保留无害（OR 关系不阻碍其余 deny），但应在风险表点明限制。
- `Bash(/bin/rm:*)`：极少误伤（项目 grep `/bin/rm` 0 命中），合法用例为零。
- `Bash(find / -delete:*)`：误伤面极窄（必须以 `find / -delete` 开头），项目 grep `find /` 在 scripts/ 中 0 命中。
- `Bash(rm -rf ~:*)`、`Bash(rm -rf .:*)`：误伤面也极窄；设计已隐含承认"严格的开发流程下用户不应在 chat 中要求 `rm -rf .`"。

### 审查点 4 · P2 Read deny 与 Windows 路径 → **WARN→PASS**（设计 §11 R-3 已声明，不空话）

- POSIX 形式 `Read(./.env)` 必中前提：模型/工具调用传入的路径参数字面以 `./` 开头。Claude Code 工具调用的路径规范是 POSIX/绝对路径优先，所以**`./` 前缀场景必中**。
- Windows 路径形式（`.\.env`、`C:\…\.env`）可能漏拦：设计 §11 R-3 已声明"不承诺完备拦截"。
- 关键判断：这是**预防性 deny**（项目当前无 `.env` 文件，grep `.env` 0 命中真实文件），属于**未来兜底**而非当前防御漏洞；设计 §8.2 引用官方示例、不自创路径形式，是稳妥做法。
- **非空话**：设计 §11 R-3 明确把限制写成"漏拦截而非误拦"+"明确写在 §8.1 不做完备防御"+"后续若引入真实 .env，可在 Insight Index 追加观察条目"——三段联动是真实风险声明，不是免责套话。

### 审查点 5 · JSON 严格性 → **PASS**

- §10 完整预览逐字符审查：
  - 行 191 `{` 与行 245 `}` 对偶；
  - 行 194 `"permissions": {` 与行 231 `}` 对偶；
  - 行 195 `"allow": [` 与行 217 `]` 对偶；20 个字符串元素均带后置逗号，第 21 个（行 216 `"Bash(bash scripts/harness-sync.sh:*)"`）无尾逗号；
  - 行 218 `"deny": [` 与行 230 `]` 对偶；11 个字符串元素中前 10 个带后置逗号，第 11 个（行 229 `"Read(./secrets/**)"`）无尾逗号；
  - 行 232 `"hooks": {` 与行 244 `}` 对偶；`Stop` 数组对象的 `type` / `command` 两键带正确逗号分隔。
- 无 `//` 注释、无 `/* */`、无尾逗号、无未转义引号、字符串值内的 `\x` 类转义不存在。
- 结论：**语法合法**。Developer 写盘后跑 `Get-Content … | ConvertFrom-Json` 应通过。

### 审查点 6 · scope creep → **PASS**

- RA §3.2 列了 8 条 out-of-scope；设计 §15 列了 6 条 out-of-scope 重申。
- 我从设计 §1-§16 逐节扫描，未发现任何项触及 RA 已 out-of-scope 的领域：
  - 未改 `.claude/agents/`、`.claude/skills/`、`CLAUDE.md`、`.github/copilot-instructions.md`；
  - 未改 `_comment` / `_doc_sync_hook` 文案；
  - 未做 Stop hook 跨平台改造；
  - 未触 `settings.local.json`；
  - 未新增 allow 条目（设计 §8.3 最终 allow 数组与原 21 条完全一致）。
- 结论：**零 scope creep**。

### 审查点 7 · 回滚可行性 → **PASS**

- `.gitignore` 行 86 仅 ignore `settings.local.json`，不 ignore `settings.json`。
- `Glob .claude/settings*.json` 返回 `settings.json` 与 `settings.local.json` 两个文件。
- `settings.json` 当前内容（修改前的基线）在工作树中可见；该文件在历史 commit 中存在（init 时引入），`git checkout -- .claude/settings.json` 必能恢复。
- 结论：**100% 还原可行**。

### 审查点 8 · 对抗用例可执行性 → **WARN**（见 F-2）

- ADV-1 manual（设计已归类），与 verify_all 自动化路径正交。
- ADV-2 / ADV-3 / ADV-4 实质都是 manual / 半 manual；QA Stage 6 必须明确把它们标 "manual"，避免 verify_all E.6 (`## Adversarial tests` 段) 误以为需要自动化。
- 不阻塞 Developer 阶段；只是 QA 提示。

---

## 4. 高概率开发者问题（pre-answered）

### Q1: 如果 PowerShell 写 `.claude/settings.json` 引入 BOM 怎么办？

**A**：Developer 强制用 `[System.IO.File]::WriteAllText($path, $content, [System.Text.UTF8Encoding]::new($false))` 显式无 BOM（Insight Index 第 19 条已证此为 PS5 / PS7 通用范式）。**不要**用 `Set-Content`/`Out-File -Encoding utf8`，那会在 PS5 上写 BOM。最稳的方式：在 Edit 工具里做"字符串替换 + 数组追加"两次 Edit 调用，由 Edit 工具保留原文件编码与行尾。

### Q2: deny 数组顺序变化会不会影响 permissions 引擎行为？

**A**：不会。context7 文档 + PM_LOG.md 已确认 deny 是全量 OR 语义、与顺序无关。设计 §8.3 给的分组排序仅为 reviewer 可读性。Developer 可严格照搬，也可保持原有顺序在末尾追加（任选其一，AC-3/7/8 都满足）。

### Q3: §8.3 里看到 `// git 类（原有，不动）` 注释，要不要写进 JSON？

**A**：**绝对不要**。设计 §8.3 + §11 R-7 + §10 完整预览三处共同明示：注释只是 reviewer 视觉分组示意，§10 完整预览是唯一权威落盘版本。JSON 不支持任何形式的注释，写入即解析失败。

### Q4: 改完后 verify_all 怎么跑？需不需要先做什么准备？

**A**：直接 `pwsh -File c:/Programs/frp_easy/scripts/verify_all.ps1`。verify_all 不读 settings.json 内容（已 grep 核验），所以 settings.json 的修改不会影响 verify_all 任何步骤的结果。AC-5 只要求"与改前同等级别"（当前 PASS:19），不要求新增任何 PASS。E.6 要求 `## Adversarial tests` 段——这是 QA Stage 6 在 06_TEST_REPORT.md 中要写的，与 Developer 阶段无关。

### Q5: `_comment` / `_doc_sync_hook` 真的要保留吗？我看着像噪声字段。

**A**：是，必须保留。RA §5 I-1 + 设计 §8 D-4 + AC + 验证 T7 三处共同要求。删除 = 违反设计、QA 必 FAIL。它们承载"为什么这样配 / 跨平台 hook 怎么改"的知识；JSON Schema 默认 `additionalProperties: true`，不触发校验错误。下划线前缀是 JSON 社区扩展字段约定。

### Q6: AC-5 verify_all PASS 是要求 PASS:19，还是任何 PASS 数都行？

**A**：RA AC-5 措辞是"与改前同等级别，不允许任何指标下降"。改前的基线（按 docs/tasks.md 最近提交记录）是 PASS:19。Developer 写 04 时应在最后跑 verify_all 截图/抄录 pass_count，保证 >= 19；任何 < 19 都视为回归。

---

## 5. 与 Insight Index 的交叉检查

- **第 41 条**：reviewer 类倾向不落盘 → 本任务 PM 派发已显式说明，但 gate-reviewer 角色的 tools frontmatter（`Read, Glob, Grep`）**没有 Write**，从根上无法解决；本 reviewer 把完整内容写在返回消息体，由 PM 落盘。建议后续在 `.harness/agents/gate-reviewer.md` 工具列表增加 Write，彻底消除该陷阱。
- **第 19 条**（PowerShell BOM）：与 Q1 关联。
- **其他条目**：无与本任务设计冲突的 insight。

---

## 6. Verdict

**APPROVED FOR DEVELOPMENT**

**判定依据**：
- 6 PASS + 2 WARN + 0 FAIL；
- 2 条 WARN 均为"提示性优化"，不阻塞 Developer 字符级实施；
- F-1 是说明性缺陷（设计仍可正确实施，只是某条 deny 实操命中率不确定）；
- F-2 是 QA 阶段才需要落实的明确化（标 "manual" 即可），不在 Developer 范围；
- 红线复议 4 条理由 100% 被仓库事实支持；
- 设计 §9 unified diff + §10 完整预览到字符级，零歧义。

**移交 Developer 时附带条件**（不阻塞，但建议遵守）：
1. 优先用 Edit 工具做"两次精确字符串替换"实施（一次改 `$schema`，一次在 deny 数组末元素后追加 9 行），避免 Write 整文件可能引入的编码/行尾漂移。
2. 写盘后立即在 04_DEVELOPMENT.md 贴 `Get-Content … | ConvertFrom-Json | Out-Null` 退出码 / `ConvertTo-Json` 回环输出，作为 AC-2 静态证据。
3. 在 04_DEVELOPMENT.md 记录"改前 / 改后 deny 列表 diff"（RA §7 可观测性要求）。
4. 跑 verify_all 全量、抄 PASS 数字到 04，证明 >= 改前基线（19）。

---

## 7. 移交 PM

下一步：Developer（单一分区）按设计 §9 diff + §10 完整预览实施。
