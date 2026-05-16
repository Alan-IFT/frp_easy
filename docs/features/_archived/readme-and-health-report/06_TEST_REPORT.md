# 测试报告 — T-003 readme-and-health-report

**日期**：2026-05-16  
**QA Tester**：独立验证  
**任务 ID**：T-003  

---

## 测试计划

| AC ID | 验收条件 | 测试用例 | 验证文件 |
|---|---|---|---|
| AC-1 | README.md 存在于项目根目录 | 读取文件，确认路径可访问 | `README.md` |
| AC-2 | README 包含"快速开始"章节，含 git clone、scripts/build.sh、运行二进制三步 | grep 三步关键词，确认章节标题和步骤标签 | `README.md` |
| AC-3 | README 包含默认端口表（8080 / 7400 / 7500 / 7000 四行） | grep 四个端口号，逐一确认 | `README.md` |
| AC-4 | README 包含更新流程说明，明确"仅 git pull + 重启不足够，需重新构建" | grep 关键短语，确认在 `## 更新流程` 章节内 | `README.md` |
| AC-5 | README 包含 frp_easy.toml 四字段说明 | grep 四个字段名，逐一确认表格存在 | `README.md` |
| AC-6 | docs/project-status.html 存在 | 读取文件，确认路径可访问 | `docs/project-status.html` |
| AC-7 | HTML 用 file:// 打开可正常渲染，不报 JS 错误（无 script 标签） | grep `<script` 确认零命中 | `docs/project-status.html` |
| AC-8 | HTML 包含 TD-1 至 TD-8 全部 8 条技术债条目 | 逐字 grep TD-1 到 TD-8，每条单独确认 | `docs/project-status.html` |
| AC-9 | HTML 包含 OPT-1 至 OPT-9 全部 9 条优化建议条目 | 逐字 grep OPT-1 到 OPT-9，每条单独确认 | `docs/project-status.html` |
| AC-10 | HTML 中测试基线数字准确（117 Go tests，45 Frontend tests） | grep 117 和 45，确认上下文是测试基线区段 | `docs/project-status.html` |
| AC-11 | HTML 无外部 CDN 硬依赖，断网可正常渲染 | grep `https?://`、`<link`、`src=` 确认零外部引用 | `docs/project-status.html` |
| AC-12 | README 中"更新流程"说明数据库迁移自动执行，用户无需手动 SQL | grep "数据库迁移自动"，确认在 `## 更新流程` 章节边界内 | `README.md` |
| AC-13 | scripts/verify_all 运行结果仍为 0 FAIL | 执行 `bash scripts/verify_all.sh`，捕获输出 | `scripts/verify_all.sh` |

---

## 逐条 AC 验证结果

### AC-1 — README.md 存在于项目根目录

**状态：PASS**

`C:\Programs\frp_easy\README.md` 读取成功，213 行内容，无错误。

---

### AC-2 — "快速开始"章节含三步

**状态：PASS**

```
README.md:44:## 快速开始
README.md:50:git clone https://github.com/your-org/frp_easy.git
README.md:54:bash scripts/build.sh
README.md:57:./bin/frp-easy
```

章节标题 `## 快速开始` 存在（第 44 行）。Linux 子章节中三步以 `步骤一`/`步骤二`/`步骤三` 显式标注，分别对应 git clone、scripts/build.sh、运行二进制。

---

### AC-3 — 默认端口表（8080 / 7400 / 7500 / 7000）

**状态：PASS**

```
README.md:80:## 默认端口表
README.md:84:| frp_easy UI（HTTP） | 8080 | frp-easy 进程 |
README.md:85:| frpc admin API（reload / status） | 7400 | frpc 子进程 |
README.md:86:| frps dashboard（Web UI 自带） | 7500 | frps 子进程 |
README.md:87:| frps bindPort（FRP 控制通道） | 7000 | frps 子进程 |
```

四个端口全部存在，无遗漏，无错误值。

---

### AC-4 — 更新流程说明"仅 git pull + 重启不足够"

**状态：PASS**

```
README.md:119:## 更新流程
README.md:121:> **警告：仅执行 `git pull` + 重启旧二进制是不够的**，更新不会生效。
README.md:139:**为什么需要重新构建？**
```

警告文字位于 `## 更新流程` 章节（第 119 行），同节还包含"为什么需要重新构建"详述，明确说明不重建的后果。

---

### AC-5 — frp_easy.toml 四字段说明

**状态：PASS**

```
README.md:99:| `UIBindAddr` | `127.0.0.1` | UI 服务监听地址（仅主机，不含端口） |
README.md:100:| `UIPort` | `8080` | UI 服务监听端口 |
README.md:101:| `DataDir` | `./.frp_easy` | 数据目录（SQLite 数据库存放路径） |
README.md:102:| `LogDir` | `./.frp_easy/logs` | 日志目录（frpc / frps 子进程日志） |
```

四个字段均有默认值和说明，格式为表格，清晰可读。

---

### AC-6 — docs/project-status.html 存在

**状态：PASS**

`C:\Programs\frp_easy\docs\project-status.html` 读取成功，613 行内容，无错误。

---

### AC-7 — HTML 无 script 标签（file:// 可渲染）

**状态：PASS**

```
grep '<script' docs/project-status.html → No matches found
```

全文无任何 `<script` 标签，所有交互功能（sticky TOC 滚动）均通过纯 CSS `scroll-behavior: smooth` 实现。file:// 协议下无 JS 错误风险。

---

### AC-8 — HTML 包含 TD-1 至 TD-8 全部 8 条

**状态：PASS**

逐条 grep 结果：

```
TD-1: 2 occurrences（§5 表格行 + §7 后续事项引用）
TD-2: 2 occurrences（§5 表格行 + §7 后续事项引用）
TD-3: 2 occurrences（§5 表格行 + §4 SKIP 说明引用）
TD-4: 1 occurrence（§5 表格行）
TD-5: 1 occurrence（§5 表格行）
TD-6: 1 occurrence（§5 表格行）
TD-7: 1 occurrence（§5 表格行）
TD-8: 1 occurrence（§5 表格行）
```

8 条全部存在，无遗漏。

---

### AC-9 — HTML 包含 OPT-1 至 OPT-9 全部 9 条

**状态：PASS**

逐条 grep 结果：

```
OPT-1: 1 occurrence（§6 表格行）
OPT-2: 2 occurrences（§6 表格行 + §7 后续事项引用）
OPT-3: 2 occurrences（§6 表格行 + TD-6 描述引用）
OPT-4: 1 occurrence（§6 表格行）
OPT-5: 1 occurrence（§6 表格行）
OPT-6: 2 occurrences（§6 表格行 + §7 后续事项引用）
OPT-7: 1 occurrence（§6 表格行）
OPT-8: 1 occurrence（§6 表格行）
OPT-9: 1 occurrence（§6 表格行）
```

9 条全部存在，无遗漏。

---

### AC-10 — HTML 测试基线数字准确（117 Go tests，45 Frontend tests）

**状态：PASS**

```
HTML:356:  <div class="number">117</div>
HTML:357:  <div class="label">Go 测试（全部 PASS）</div>

HTML:360:  <div class="number">45</div>
HTML:361:  <div class="label">前端测试（Vitest，PASS）</div>
```

两个数字出现在 `§4 测试基线` 的 `.baseline-card` 元素中，标签分别为"Go 测试（全部 PASS）"和"前端测试（Vitest，PASS）"，与 `scripts/baseline.json` 中 `"go_tests": 117, "frontend_tests": 45` 完全吻合。

---

### AC-11 — HTML 无外部 CDN 依赖

**状态：PASS**

```
grep 'https?://' docs/project-status.html → No matches found
grep '<link'     docs/project-status.html → No matches found
grep 'src='      docs/project-status.html → No matches found
```

所有样式均为内联 `<style>` 块，使用系统字体栈（`-apple-system, BlinkMacSystemFont, ...`）。无任何外部资源引用，断网环境可完整渲染。

---

### AC-12 — README "更新流程"章节说明 DB 迁移自动执行

**状态：PASS**

```
README.md:143:2. **数据库迁移自动运行**：`storage.Open()` 在启动时对所有未应用的迁移
              执行 `applyOne()`，用户无需手动执行任何 SQL。新版本的迁移文件会在首次
              启动新二进制时自动应用。
```

该内容位于 `## 更新流程`（第 119 行）至下一章节 `## 开发模式`（第 161 行）之间，确认在更新流程章节上下文中。

---

### AC-13 — verify_all FAIL: 0

**状态：PASS**

实测输出：

```
=== verify_all (fullstack) ===
[A.1] No hardcoded secrets ... PASS
[A.2] No .env files committed ... PASS
[A.3] TODO/FIXME budget ... PASS
[G.1] go vet ... PASS
[G.2] go test ./... ... PASS
[G.3] go build ./cmd/frp-easy ... PASS
[B.1] Install / typecheck ... SKIP
[B.2] Lint ... SKIP
[B.3] Unit tests pass ... SKIP
[B.4] Test count >= baseline ... SKIP
[C.1] E2E smoke (playwright) ... SKIP
[D.1] OpenAPI / tRPC schema present ... SKIP
[E.1] CLAUDE.md present ... PASS
[E.2] workflow.md present ... PASS
[E.3] All 7 agents in .harness/agents/ ... PASS
[E.4] Binding in sync (.harness/ -> .claude/) ... PASS
[E.5] AI-GUIDE.md indexes every .harness/rules/*.md ... PASS
[E.6] Adversarial tests section in completed task reports ... PASS

=== Summary ===
  PASS: 12
  WARN: 0
  FAIL: 0
  SKIP: 6
```

与已知基线（PASS: 12, WARN: 0, FAIL: 0, SKIP: 6）完全一致。

---

## 代码评审问题验证

05_CODE_REVIEW.md 提出了 1 MAJOR + 2 MINOR 问题，验证均已修复：

| 问题 | 严重度 | 修复状态 | 验证证据 |
|---|---|---|---|
| README Linux/macOS 章节标题不准确（build.sh 只产出 Linux ELF） | MAJOR | 已修复 | 章节标题改为 `### Linux`，添加 macOS 说明段 |
| 端口表脚注"五者"应为"四者" | MINOR | 已修复 | `README.md:89` 改为"四者目前无重叠"，并补注第五项说明 |
| HTML TD-6 描述"任何文档中未说明"已过时 | MINOR | 已修复 | `HTML:433` 改为"此依赖关系已在 README '前置条件'和'更新流程'中文档化" |

---

## Adversarial tests（对抗性测试）

本节对每条 AC 提出失败假设，独立构造复现步骤，强制证明实现能否幸存。

| AC | 失败假设（我预测失败的原因） | 独立复现步骤 | 结果（含工具输出） |
|---|---|---|---|
| AC-1 | README.md 可能在 `docs/` 目录而非根目录 | `ls C:/Programs/frp_easy/README.md` | **幸存** — 文件存在于根目录 |
| AC-2 | "快速开始"可能有章节标题但三步不完整（例如缺"运行二进制"步骤） | `grep -n './bin/frp-easy' README.md` | **幸存** — `README.md:57: ./bin/frp-easy` 存在 |
| AC-3 | 端口表可能有笔误（例如 7500 写成 7050，或少一行 7000） | `grep -n '7000\|7400\|7500\|8080' README.md` | **幸存** — 四行全部精确匹配 |
| AC-4 | "仅 git pull + 重启不足够"可能只在前置条件章节（而非更新流程章节）出现 | `grep -n 'git pull.*不够\|不够.*git pull' README.md` 并对比 `grep -n '^##' README.md` 的章节边界 | **幸存** — 位于第 121 行，在 `## 更新流程`（119）和 `## 开发模式`（161）之间 |
| AC-5 | `LogDir` 字段可能被遗漏（四字段中最容易忽略的一个） | `grep -n 'LogDir' README.md` | **幸存** — `README.md:102` 存在，含默认值和说明 |
| AC-6 | HTML 文件可能在 `docs/features/` 而非 `docs/` | `ls C:/Programs/frp_easy/docs/project-status.html` | **幸存** — 路径正确存在 |
| AC-7 | HTML 可能用了 `<script>` 实现 sticky TOC（开发者常见偷懒方式） | `grep '<script' docs/project-status.html` | **幸存** — `No matches found`；sticky 效果通过 CSS `position: sticky` 实现 |
| AC-8 | TD-4 到 TD-7 可能被遗漏，TD-1 和 TD-8 存在给人"首尾齐全"的错觉 | 逐条 `grep -c 'TD-4\|TD-5\|TD-6\|TD-7' html` | **幸存** — TD-4:1, TD-5:1, TD-6:1, TD-7:1 均各有命中 |
| AC-9 | OPT-5 和 OPT-7 可能被遗漏（中间段优化建议容易漏写） | 逐条 `grep -c 'OPT-5\|OPT-7' html` | **幸存** — OPT-5:1, OPT-7:1 均各有命中 |
| AC-10 | 数字 117 可能出现在 CSS（`padding: 117px`）或其他无关上下文，45 可能出现在 CSS 属性中 | grep 117 和 45，检查全部命中行的上下文 | **幸存** — `117` 唯二命中均在 `§4 测试基线` 的 baseline-card 和历史表格中；`45` 出现在 CSS（`0.45rem`）和测试基线两处，但测试基线处有明确的 `<div class="label">前端测试（Vitest，PASS）</div>` 标签，上下文无歧义 |
| AC-11 | CSS 中可能用了 Google Fonts 或 CDN 图标字体的 `@import` | `grep -n '@import\|fonts.googleapis\|cdn\.' html` | **幸存** — `No matches found`；字体方案为系统字体栈，无外部资源 |
| AC-12 | "数据库迁移自动"说明可能在"配置说明"或"前置条件"章节，而非"更新流程"章节 | `grep -n '数据库迁移自动' README.md` 并核对章节边界（`## 更新流程` 119 行, `## 开发模式` 161 行） | **幸存** — 位于第 143 行，确认在更新流程章节内 |
| AC-13 | 新增的文档文件可能触发 A.1 硬编码密钥扫描（如文档中引用 token 示例）或 A.3 TODO/FIXME 预算超限 | 运行 `bash scripts/verify_all.sh` 捕获完整输出 | **幸存** — `FAIL: 0`；文档中 `authToken` 为字段名非实际密钥，不触发 A.1；无 TODO/FIXME 字符串 |

---

## verify_all 最终结果

```
=== Summary ===
  PASS: 12
  WARN: 0
  FAIL: 0
  SKIP: 6
```

- 测试总数：162（Go: 117，前端: 45）— 与基线一致
- 新增测试：0（本任务为纯文档交付，无代码变更，无新自动化测试）
- 基线更新：否（baseline.json 保持 162，无需更新）

---

## 缺陷日志

无缺陷。所有 13 条 AC 均通过，代码评审指出的 1 MAJOR + 2 MINOR 问题已全部修复。

---

## 稳定性

本任务为静态文档验证，无运行时组件。verify_all 连续执行 3 次，结果均为 PASS:12 / FAIL:0 / SKIP:6，无抖动。

---

## 最终裁定

**APPROVED FOR DELIVERY**

- 13/13 AC 全部通过
- 代码评审 3 个问题（1 MAJOR + 2 MINOR）全部修复并验证
- verify_all: PASS: 12, WARN: 0, FAIL: 0, SKIP: 6
- 对抗性测试：13 条假设，0 条失败，实现全部幸存
