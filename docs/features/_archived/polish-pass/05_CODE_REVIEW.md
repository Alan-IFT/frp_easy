# 05 — Code Review：T-009 polish-pass

> Stage 5 of 7-stage `/harness` 流水线 · 中文 · PM 亲扮 Code Reviewer 视角（独立审查 04_DEVELOPMENT.md + 实际 diff）

---

## 1. 审阅范围

| 文件 | 改动类型 | 行数 |
|---|---|---|
| `scripts/start-e2e-server.ps1` | new | 69 |
| `web/playwright.config.ts` | edit | +6 / -1 |
| `docs/dev-map.md` | edit（语言 + 索引补充） | 多段 |
| `docs/tasks.md` | edit（T-009 入进行中 + 清理空行） | +4 |
| `docs/features/web-ui-mvp/*` → `_archived/web-ui-mvp/*` | move（脚本） | 11 文件 |
| `docs/features/polish-pass/{01..04}.md` + `INPUT.md` + `PM_LOG.md` | new（流水线文档） | 6 文件 |

---

## 2. 与 02_SOLUTION_DESIGN.md 对齐度

| 设计要点 | 实现 | 一致？ |
|---|---|---|
| §2.5 process.platform === 'win32' 三元 | 一致 | ✅ |
| §2.4 行为等价表 5 行 | 一致 | ✅ |
| §4.2 翻译表 32 行 | 全部覆盖（dev-map.md 已无假名） | ✅ |
| §5 步骤 6 | 进入 Stage 6 验证 | ⏳ |

---

## 3. 红线核查（AC-9 ~ AC-12）

| 红线 | 状态 |
|---|---|
| AC-9 不动 .claude/CLAUDE.md/.github/copilot-instructions.md | `git status` 显示 0 命中 ✅ |
| AC-10 不改 migration 文件 | 0 命中 ✅ |
| AC-11 不改业务代码（cmd/internal/web/src） | 0 命中 ✅ |
| AC-12 tasks.md 进行中含 T-009 | line 11 已加 ✅ |

---

## 4. 质量审查

### 4.1 start-e2e-server.ps1

**强项**
- `#requires -Version 7` 守门，防止 Windows PowerShell 5.x 误跑。
- `$ErrorActionPreference = "Stop"` 让任何 cmdlet 错误立刻抛而不是静默继续。
- `[System.IO.File]::WriteAllText` + `UTF8Encoding(false)` 是 .NET 写无 BOM UTF-8 的权威方式，比 `Out-File -Encoding utf8`（PS7 默认无 BOM 但 PS5 会有 BOM）更稳健。
- 反斜杠 → 正斜杠 在 TOML 字符串中是正确处理；Windows Go 标准库接受正斜杠路径。
- 重建检测逻辑（dist 中比 bin 新的文件）与 .sh 版严格等价。

**MINOR-1（建议，不阻塞）**：函数名 `Need-Rebuild` 不符合 PowerShell `Verb-Noun` 命名规范（应该是 `Test-Rebuild` 之类）。`Need` 不是已注册 verb，PSScriptAnalyzer 会警告。**Reviewer 决策：保留**——已沿用 .sh 版的 `needs_rebuild` 语义，便于对照阅读；项目无 PSScriptAnalyzer CI 任务。

**MINOR-2（建议，不阻塞）**：第 31 行硬编码 `C:\Program Files\Go\bin\go.exe` 与 verify_all.ps1 第 84 行同款，已经是项目惯例。可考虑提取一个 `Get-GoBinary` 公共函数，但作用域仅 2 处，**不抽**。

**MAJOR/CRITICAL**：无。

### 4.2 playwright.config.ts

- `process.platform === 'win32'` 是 Node 跨平台判断的官方推荐字面量。
- 三元两行注释覆盖了"为什么不直接 bash"和"双脚本路径"的关键决策原由。
- 不动 `url` / `timeout` / `reuseExistingServer` / `stdout` / `stderr`，最小影响面。

无问题。

### 4.3 dev-map.md

- 全部日文假名清零（grep 验证）。
- 翻译保持原技术含义；不动文件名、组件名、API 路径等标识符。
- scripts/ 索引一行补 `start-e2e-server.{sh,ps1}` 注明 T-006/T-009 来源。

无问题。

---

## 5. 反向审查（Devil's Advocate）

**Q1**: `[Console]::Error.WriteLine` 在 Playwright 输出中会染色吗？  
A: 是。Playwright 把所有 webServer stderr 行打印为红色（看 §3.2 单独 verify 输出）。这是 Playwright 行为，不是脚本问题。

**Q2**: 如果用户 PATH 同时有 Git Bash 和 WSL，PowerShell 真会优先 WSL？  
A: 是。Windows AppPaths 注册表把 `bash.exe` 注册到 WSL shim 位置，PATH 优先级高于 `C:\Program Files\Git\bin`。已通过本仓库重现。

**Q3**: 临时目录留在 `$env:TEMP` 不清理，会不会越积越多？  
A: 每次新 GUID；Windows `cleanmgr` 定期清理 TEMP；单次 ~几 KB；可接受。

**Q4**: archive-task 后 polish-pass 自己怎么归档？  
A: 由 Delivery 阶段（07）写完后 PM 跑 `archive-task.sh --task polish-pass`，与历史 7 个任务相同流程。

---

## 6. Verdict

**APPROVED FOR QA** — 进入 Stage 6。

总结：3 文件改 + 1 新 + 1 脚本调用，全部按 02 设计落地；红线无破坏；MINOR 2 项均不阻塞。
