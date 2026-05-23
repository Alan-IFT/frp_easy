# 01 — Requirement Analysis · T-020 claude-settings-context7-fix

> Stage 1 / 7 · Requirement Analyst · 模式：`full` · 输出语言：中文 · 2026-05-23

## 1. 任务一句话定义

依据 context7 拉取的 Claude Code 官方文档（`code.claude.com/docs/en/settings`、`/permissions`、`/hooks`、`/vs-code`），修复 `c:/Programs/frp_easy/.claude/settings.json` 中**与官方 schema 或安全推荐相违**的内容，使该文件能被官方 schema 校验器正确识别，且 deny 列表与官方安全示例对齐。

## 2. 背景与触发

- 触发：用户在 chat 中显式请求"参考 context7 官方文档，修复 `.claude/settings.json`"。
- 红线复议：默认红线（`CLAUDE.md` 第 7 条 / `AI-GUIDE.md` 第 17 行）"不要直接编辑 `.claude/` 或 `CLAUDE.md`"。
- 复议结论（已记录在 `PM_LOG.md` § 红线复议）：本任务**例外允许**编辑 `.claude/settings.json`，理由：
  1. 文件 `_comment` 自带条款 "Edit hooks/permissions to suit your project."（init 模板预期被项目修改）；
  2. `.harness/` 下无 `settings.json` 源，`scripts/harness-sync.{ps1,sh}` 不同步该文件 —— 它不是绑定产物，是项目实际配置；
  3. 用户显式指令优先级高于默认静态规则；
  4. `.claude/agents/`、`.claude/skills/`、`CLAUDE.md` 仍受红线全力保护，本任务**绝不**触碰。
- 已经预查的 context7 文档证据由 PM 留存于 `PM_LOG.md` 文末，本文档下文按问题点引用。

## 3. 范围

### 3.1 In scope（本任务必做或可做）

1. **P0 — `$schema` URL 修复**：将 `.claude/settings.json` 第 2 行
   `"$schema": "https://json.schemastore.org/claude-code-settings"` 改为
   `"$schema": "https://json.schemastore.org/claude-code-settings.json"`（追加 `.json` 后缀）。
2. **P1 候选 — Bash `rm` deny 加固**（由 Architect 在 02 阶段决定是否纳入；本 RA 默认建议纳入，理由见 §5）：扩展 `permissions.deny`，增加更多 `rm -rf` 绕过路径的字面前缀拦截。
3. **P2 候选 — Read 类敏感文件 deny 补全**（由 Architect 在 02 阶段决定是否纳入）：按官方安全示例新增 `Read(./.env)`、`Read(./.env.*)`、`Read(./secrets/**)` 等 deny。
4. **信息性 — `_comment` / `_doc_sync_hook` 字段保留与否**：本 RA 决定**保留**；理由见 §5。本条不引入实际修改，仅为决议留痕。

### 3.2 Out of scope（本任务明确不做）

1. **`.claude/agents/` 任何文件**：受红线保护，红线复议未对其松绑。
2. **`.claude/skills/` 任何文件**：同上。
3. **`CLAUDE.md`**：受红线保护；它由 `scripts/harness-sync` 从 `.harness/` 生成。
4. **`.github/copilot-instructions.md`**：受红线保护。
5. **Stop hook 跨平台改造**：当前 `pwsh -File scripts/harness-sync.ps1` 仅 Windows 可用；但 `_doc_sync_hook` 字段已声明这是 init 时按 OS 选择的，鼓励"swap freely"。这是配置模板属性而非 bug，非"修复"语义。
6. **`.claude/settings.local.json`**：用户层（gitignore 范围）配置，无 schema 引用，且与本任务"修文档级别问题"目标无关。
7. **`Bash(npm:*)` → `Bash(npm *)` 风格统一**：context7 changelog 确认两者语义等价，非 bug，纯审美问题。
8. **新增 allow 条目**：本任务范围仅修复 bug + 补 deny 安全基线，不扩允许面。

## 4. 验收标准（可验证）

每条验收标准都必须可被人或脚本直接核验。

| # | 验收项 | 验证方式 |
|---|---|---|
| AC-1 | `.claude/settings.json` 中 `$schema` 字段值等于字符串 `"https://json.schemastore.org/claude-code-settings.json"` | 文本搜索 / JSON 解析后断言 |
| AC-2 | `.claude/settings.json` 是合法 JSON（解析不抛错） | `python -c "import json; json.load(open('.claude/settings.json'))"` 或 `Get-Content … | ConvertFrom-Json` |
| AC-3 | `permissions.deny` 是非空数组，且至少保留 T-020 之前已有的 3 条原子 deny（`git push --force:*`、`git push -f:*`、`rm -rf /:*`），未发生**降级**（只增不减） | JSON 解析后断言数组包含原 3 条 |
| AC-4 | `hooks.Stop[0].hooks[0]` 不含 `matcher` 字段（保持官方规则） | JSON path 检索 |
| AC-5 | `scripts/verify_all` PASS（与改前同等级别，不允许任何指标下降） | 跑 `pwsh -File scripts/verify_all.ps1` 退出码 0 |
| AC-6 | （manual）在 VS Code 打开 `.claude/settings.json`，状态栏 schema 解析无 "Cannot resolve schema" 错误，`permissions.allow` 内字段有 IntelliSense 补全 | 人工目测 |
| AC-7 | 若 02_SOLUTION_DESIGN 决定纳入 P1（rm deny 加固），则 `permissions.deny` 至少新增对 `rm -rf ~`、`rm -rf .`、`/bin/rm -rf` 三个常见绕过路径的字面前缀 deny | 文本搜索 + JSON 数组成员断言 |
| AC-8 | 若 02_SOLUTION_DESIGN 决定纳入 P2（Read 安全 deny），则 `permissions.deny` 至少新增 `Read(./.env)`、`Read(./.env.*)`、`Read(./secrets/**)` 三条 | 文本搜索 + JSON 数组成员断言 |

> 注：AC-7、AC-8 是**条件性验收**，仅在 Architect 决定纳入时生效；不纳入则该条不适用。

## 5. 优先级分类（含证据）

> 证据 URL 取 context7 已查证片段（见 `PM_LOG.md` § "context7 预查文档证据"）。所有片段均来自 `https://code.claude.com/docs/en/{settings,permissions,hooks,vs-code}`。

### P0（必修，本任务核心交付）

- **P0-1 · `$schema` URL 缺 `.json` 后缀**
  - 当前值：`"https://json.schemastore.org/claude-code-settings"`
  - 官方值：`"https://json.schemastore.org/claude-code-settings.json"`
  - 证据：context7 `settings` 文档显式给出 `$schema` 示例带 `.json` 后缀；JSON Schema Store 对该资源的实际 URL 也是带 `.json` 后缀的，无 `.json` 后缀会被 VS Code / Cursor 的 schema 解析器 404，IntelliSense 失效。
  - 影响：编辑期失去 schema 校验与补全，间接增加配置出错概率。
  - 处置：直接改字符串，零风险。

### P1（强烈建议纳入；由 Architect 在 02 决定）

- **P1-1 · Bash `rm` deny 仅拦字面前缀 `rm -rf /`**
  - 当前 deny：`Bash(rm -rf /:*)` 只拦截以 `rm -rf /` 开头的命令。
  - 已知绕过：`rm -rf ~`、`rm -rf .`、`rm -rf $HOME`、`/bin/rm -rf /`、`find / -delete` 等。
  - 证据：context7 `permissions` 文档显式告警 "Prefix rules in deny patterns match the literal command string... `Bash(rm *)` does not block `/bin/rm` or `find -delete`."
  - 影响：当前 deny 表象给出安全感但实际容易绕过；针对本项目（fullstack Web UI for FRP）的实际威胁面较低（用户本机开发为主），但属于"基线漏洞"，应补齐常见路径。
  - 默认建议：纳入（追加 3-5 条字面前缀 deny，覆盖 `rm -rf ~`、`rm -rf .`、`/bin/rm -rf`、`find * -delete:*` 等）。
  - 注意：deny 的本质是**字面前缀**，无法做到正则级穷举；P1 目的是把"显而易见的几个绕过路径"堵住，不奢求完备防御。

### P2（可选；由 Architect 在 02 决定）

- **P2-1 · 未对 `.env` / `secrets/` 等敏感文件设 Read deny**
  - 当前 deny 列表无任何 `Read()` 条目。
  - 证据：context7 `permissions` 官方安全示例 deny 列表明确列出 `Read(./.env)`、`Read(./.env.*)`、`Read(./secrets/**)`。
  - 影响：Claude Code 在本仓库读 `.env` 类敏感文件不会被规则拦下，依赖 `.gitignore` + 用户自觉。本项目目前 `.env` 实际不存在（grep 检查）、`secrets/` 目录也不存在，所以此 deny 是**预防性**而非补救性。
  - 默认建议：纳入（成本几乎为零，且对未来引入 `.env` 时立即生效）。

### 信息性（不修改，仅留痕）

- **I-1 · `_comment` / `_doc_sync_hook` 非官方 schema 字段**
  - 这两个字段不在官方 schema 定义内。
  - 行为：JSON Schema 默认 `additionalProperties: true`，绝大多数校验器（含 JSON Schema Store 那一份）静默放行；严格模式下可能 warn 但不会 error。
  - 决议：**保留**。理由：
    1. 它们承载"为什么这样配 / 跨平台 hook 怎么改"的关键文档，删了就丢知识；
    2. 下划线前缀是 JSON 社区常见的"扩展字段"约定；
    3. context7 文档未明令禁止；
    4. 在 init 模板（`.harness/skills/init-binding/`）层面如果有更优写法，可在后续任务讨论，但与"修复 settings.json"无关。
  - 处置：不动。

- **I-2 · Stop hook 跨平台命令仅 Windows 可用**
  - 当前命令 `pwsh -File scripts/harness-sync.ps1`，Linux/macOS 克隆者会遇到 "pwsh: command not found"。
  - `_doc_sync_hook` 已显式声明此项是 init 时按 OS 选择，鼓励切换。
  - 决议：**out of scope**（§3.2 第 5 条）。理由：这是 init 模板属性而非 bug；若要做跨平台自适应（如 hook 内部探测 OS 后分支），是"功能增强"而非"修复"。

## 6. 边界条件 / 错误路径

- **JSON 合法性**：修改后必须保持 UTF-8 无 BOM、JSON 严格语法（无注释、无尾逗号），否则 Claude Code 启动时拒载入。
- **行尾**：保持 LF 或与原文件一致；混合行尾可能让 git diff 充噪声。
- **空 deny 数组的边界**：若未来某项 P1/P2 不纳入，`deny` 数组仍非空（至少有原 3 条），不会触发任何"空数组"边界。
- **数组追加顺序**：deny 数组追加新条目时，将"git 操作"、"rm 操作"、"Read 操作"按类分组（视觉可读性），不影响匹配（deny 全量 OR 语义）。
- **并发**：单文件、本地编辑，无并发问题。
- **回滚**：若改后 `verify_all` FAIL，git checkout `.claude/settings.json` 立即还原（文件已纳版控）。

## 7. 非功能性需求

- **安全**：deny 列表的扩充必须**只增不减**，绝不删除既有 deny 项。
- **兼容性**：保留 `_comment` 和 `_doc_sync_hook` 字段，确保后续 init 模板演化时不丢文档。
- **可读性**：JSON 缩进保持 2 空格（与现状一致）；deny 数组若增长，按类目分组、可加空行（如允许）。
- **性能**：N/A（配置文件无运行时性能影响）。
- **可观测性**：本任务结束时 04_DEVELOPMENT.md 应记录"改前 / 改后"的 deny 列表 diff，便于 review。

## 8. 相关历史任务

扫 `docs/tasks.md`：

- **T-019 windows-service-scm-1053-fix**（in progress，stage req）：与 Windows Service / SCM 相关，与本任务**无 settings.json 交集**；但二者都触及"Windows-only 路径"，注意若 T-019 引入 hooks 字段变更，需在合并时复核。
- **T-018 upload-bin-multiport-ip-probe** 至 **T-011 readme-refresh-and-network-defaults**：均为业务功能或部署任务，与 `.claude/` 配置层无关。
- **T-006 e2e-smoke-tests** 等 init 时期任务：与 `.claude/` 初始化绑定相关，但相关产物是 `.harness/skills/` 而非 `settings.json`。

没有需要先读的相关历史任务设计文档。Insight Index 中也无与 `.claude/settings.json` 相关的条目。

## 9. 开放问题 / 待用户裁定

按 PM 派发 prompt 的指示（"用户已要求无 clarifying questions 模式"），本 RA 在 P1/P2 上做出**默认决定**而不阻塞用户：

- **P1 默认决定**：纳入（rm deny 加固），具体条目交由 Architect 在 02_SOLUTION_DESIGN.md 列出。
- **P2 默认决定**：纳入（Read 敏感文件 deny 补全），具体条目交由 Architect 在 02 列出。
- **I-1 决定**：保留 `_comment` 和 `_doc_sync_hook` 字段。
- **I-2 决定**：Stop hook 跨平台改造 out of scope。

如果 Gate Reviewer 或用户认为某项默认决定不合理，可在 03_GATE_REVIEW.md 标记 BLOCKED ON USER 回退。

**无 open question 阻塞本阶段。**

## 10. Verdict

**READY** — 可推进至 Stage 2 (Solution Architect)。

Architect 在 02 阶段需完成：
1. 给出 P0-1 的精确 diff（一行字符串修改）。
2. 决定 P1-1 是否纳入；若纳入，列出新增 deny 条目的字面前缀清单（建议至少含 `rm -rf ~`、`rm -rf .`、`/bin/rm -rf`、`find * -delete:*`）。
3. 决定 P2-1 是否纳入；若纳入，列出新增 Read deny 条目（建议至少含官方示例的三条）。
4. 给出修改后的完整 `.claude/settings.json` 预览，并标注 diff 块。
5. 给出回滚方案（一句话：`git checkout -- .claude/settings.json`）。
