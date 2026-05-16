# 方案设计 — T-003 readme-and-health-report

**任务 ID**：T-003  
**Slug**：readme-and-health-report  
**阶段**：design  
**日期**：2026-05-16  
**方案架构师**：Solution Architect  

---

## 1. 方案概述

本任务为 frp_easy 项目补充两份纯文档文件：项目根目录 `README.md`（面向新用户的安装与使用指南）和 `docs/project-status.html`（自包含 HTML 项目状况总览，可离线用浏览器直接打开）。两个文件均不修改任何 Go / TypeScript / Vue 源代码，不新增或删除 API 端点，不触发数据库迁移，`scripts/verify_all` 结果维持 0 FAIL（AC-13）。文档内容完全来源于现有代码库注释、配置默认值、脚本逻辑和已归档的任务交付记录。需求文档标注的两个开放问题（Q-1、Q-2）已由用户直接回答，设计无阻塞项。

---

## 2. 受影响模块

| 文件 | 当前状态 | 本次操作 |
|---|---|---|
| `README.md`（项目根） | 不存在（`docs/spec/README.md` 是规格索引，非用户文档） | **新建** |
| `docs/project-status.html` | 不存在 | **新建** |

其余所有文件保持不变。

---

## 3. 新模块规格

### 3.1 README.md

**职责**：作为项目唯一的用户入口文档，覆盖安装、配置、更新、开发七大主题，符合标准 Markdown（NF-1）。

**语言**：中文为主，代码片段例外（NF-4）。

**章节大纲与关键数据来源**：

| 章节 | 内容摘要 | 权威数据来源（文件路径 + 行号） |
|---|---|---|
| 项目简介 | 一句话描述（Go + Vue 3 + SQLite，单二进制 FRP Web 管理 UI）+ 版本徽章占位 | 固定文案（版本写死 0.1.0） |
| 功能列表 | T-001 已交付 9 项核心功能；T-002 已交付 4 项零配置功能 | `docs/features/web-ui-mvp/07_DELIVERY.md` 第 22-31 行；`docs/features/_archived/zero-config-quickstart/07_DELIVERY.md` 第 10-21 行 |
| 前置条件 | Go 1.22+，Node.js 18+，npm | `scripts/build.sh` 第 23 行（npm 调用）；项目类型（Go 1.25 实际使用，1.22 是最低要求） |
| 快速开始 | 步骤一：`git clone`；步骤二：`scripts/build.sh`（Linux/macOS）或 `scripts\build.ps1`（Windows）；步骤三：运行 `bin/frp-easy`（Linux）或 `bin\frp-easy.exe`（Windows） | `scripts/build.sh` 第 23-34 行；`scripts/build.ps1` 第 23-40 行 |
| 默认端口表 | 4 行端口表（8080 / 7400 / 7500 / 7000） | `internal/appconf/config.go` 第 8-14 行注释 |
| 配置说明 | UIBindAddr / UIPort / DataDir / LogDir 四字段、默认值、安全警告 | `internal/appconf/config.go` `Default()` 函数第 46-53 行；Validate() 第 44-46 行注释 |
| 更新流程 | 明确 "git pull 不够"；完整步骤 git pull → build.sh → 重启；四点原因说明 | F-3 规格（需求文档第 77-93 行）；Q-1 答案（dist/ 未提交 git） |
| 开发模式 | 双进程模式：`go run ./cmd/frp-easy`（8080）+ Vite dev（5173） | `scripts/start.sh` 第 44-55 行 |
| 目录结构速览 | 12 个关键目录各一行说明 | `docs/dev-map.md` 第 11-84 行目录布局块 |
| 技术债与优化建议 | 一段简述 + 链接指向 `docs/project-status.html` | 指引句（F-4/F-5 完整内容在 HTML） |

**"更新流程"章节的必含四点原因**（对应 AC-4 和 AC-12）：

1. **前端需重建**：`internal/assets/dist/` 被 `.gitignore` 的 `dist/` 规则排除，`git pull` 不会更新前端产物；`build.sh` 自动执行 `npm run build`，将新前端嵌入 Go 二进制。
2. **DB 迁移自动运行**：`storage.Open()` 启动时对所有未应用迁移执行 `applyOne()`，无需用户手动执行 SQL。
3. **配置向后兼容**：`appconf.Load()` 对缺失字段补默认值，`frp_easy.toml` 在 `.gitignore` 中，`git pull` 不覆盖用户配置。
4. **仅后端变更的简化路径**（可选快捷方式，须在文档中标注前提条件）。

### 3.2 docs/project-status.html

**职责**：提供可本地浏览的项目状况快照，包含技术栈、已交付功能、架构模块、测试基线、技术债和优化建议的完整清单。文件放在 `docs/project-status.html`（Q-2 答案）。

**布局方案**（纯 CSS，无 JS）：

```
┌──────────────────────────────────────────────────────────────┐
│  标题栏（深色背景）：frp_easy 项目状况总览 v0.1.0 · 2026-05-16  │
├─────────────┬────────────────────────────────────────────────┤
│  左侧 TOC   │  右侧内容区                                      │
│  position:  │  §1 技术栈一览                                   │
│  sticky     │  §2 已交付功能（T-001 / T-002）                  │
│  top: 1rem  │  §3 架构模块表                                   │
│  宽: 200px  │  §4 测试基线                                     │
│             │  §5 技术债清单（TD-1..TD-8）                     │
│             │  §6 优化建议清单（OPT-1..OPT-9）                 │
│             │  §7 已知后续事项                                  │
└─────────────┴────────────────────────────────────────────────┘
```

**实现约束**（对应 AC-7、AC-11）：

- 单文件 HTML，所有样式写在顶部 `<style>` 块内，无 `<link rel="stylesheet">` 外部样式表。
- 无任何 `<script>` 标签（AC-7：`file://` 打开无 JS 错误）。
- 左侧导航使用 `<a href="#sec-N">` 锚点 + `position: sticky`，纯 CSS 实现滚动跟踪。
- 无 CDN 字体、图标引用（NF-2、AC-11）。
- 优先级标签使用 `<span>` + CSS class，内联样式兜底确保离线渲染正确。

**颜色方案**：

| 元素 | 颜色 | 说明 |
|---|---|---|
| 标题栏背景 | `#343a40` 深灰 | 对比度高 |
| 标题栏文字 | `#ffffff` | — |
| 正文背景 | `#ffffff` | 标准白底 |
| 正文文字 | `#212529` | 接近黑 |
| TOC 背景 | `#f8f9fa` 浅灰 | 区分内容区 |
| 优先级：高 | `#dc3545` 红色背景，白字 | 醒目 |
| 优先级：中 | `#fd7e14` 橙色背景，白字 | 次要警示 |
| 优先级：低 | `#6c757d` 蓝灰背景，白字 | 信息性 |
| 表格斑马纹 | 偶数行 `#f8f9fa` | 可读性 |
| 链接 | `#0d6efd` 蓝色 | 标准链接色 |

**各 Section 内容与数据来源**：

| Section | 内容 | 数据来源（文件路径） |
|---|---|---|
| §1 技术栈一览 | Go 1.25、Vue 3 + Vite、SQLite via modernc.org/sqlite、chi router v5、argon2id、Naive UI | `docs/dev-map.md` + `docs/features/web-ui-mvp/07_DELIVERY.md` 第 33-36 行 |
| §2 已交付功能 | T-001 9 项功能（标注任务ID）；T-002 4 项功能（标注任务ID）| `docs/features/web-ui-mvp/07_DELIVERY.md` 第 22-31 行；`docs/features/_archived/zero-config-quickstart/07_DELIVERY.md` 第 10-21 行 |
| §3 架构模块表 | 直接摘录 `docs/dev-map.md` §功能在哪里 的 14 行表格 | `docs/dev-map.md` 第 88-103 行 |
| §4 测试基线 | Go tests: 117，Frontend tests: 45，verify_all PASS:12 / SKIP:6 / FAIL:0 | `docs/features/_archived/zero-config-quickstart/07_DELIVERY.md` 第 72-77 行 |
| §5 技术债清单 | TD-1..TD-8 全部 8 条，含影响级别标签和来源注明 | `docs/features/readme-and-health-report/01_REQUIREMENT_ANALYSIS.md` 第 103-111 行 F-4 表格 |
| §6 优化建议清单 | OPT-1..OPT-9 全部 9 条，含优先级标签（高/中/低）和说明 | `docs/features/readme-and-health-report/01_REQUIREMENT_ANALYSIS.md` 第 119-139 行 F-5 表格 |
| §7 已知后续事项 | TD-1 向导路由守卫漏洞修复建议；TD-2 ParseIPFromJSON 统一建议 | `docs/features/_archived/zero-config-quickstart/07_DELIVERY.md` 第 82-83 行 |

---

## 4. 数据模型变更

**无**。本任务不涉及任何数据库改动，不添加迁移文件，不修改现有 schema。

---

## 5. API 契约

**无**。本任务不添加、修改或删除任何 REST 端点。OPT-7（健康检查端点）仅文档化为建议，本任务不实现。

---

## 6. 请求流程

不适用。本任务交付物为纯静态文档文件，无运行时请求流，不影响 frp_easy 进程的任何行为。

---

## 7. 复用审计

| 需求 | 现有内容 | 文件路径 | 决策 |
|---|---|---|---|
| 默认端口值 | `Default()` 函数注释表（UIPort=8080 等） | `internal/appconf/config.go` 第 7-16 行 | 直接引用注释数值写入 README 和 HTML |
| 配置字段默认值 | `Default()` 函数返回值 | `internal/appconf/config.go` 第 46-53 行 | 逐字段读取写入 README §配置说明 |
| 目录结构 | `## 目录布局` 代码块 | `docs/dev-map.md` 第 11-84 行 | 精简后引入 README §目录结构速览 |
| 功能模块表 | `## 功能在哪里` 表格 | `docs/dev-map.md` 第 88-103 行 | 完整引入 HTML §3 架构模块表 |
| 构建步骤（Linux） | build.sh 脚本逻辑 | `scripts/build.sh` 第 23-34 行 | 精确引用命令写入 README §快速开始 |
| 构建步骤（Windows） | build.ps1 脚本逻辑 | `scripts/build.ps1` 第 23-40 行 | 精确引用命令写入 README §快速开始 |
| 开发模式命令 | start.sh 脚本逻辑 | `scripts/start.sh` 第 44-55 行 | 精确引用命令写入 README §开发模式 |
| T-001 功能摘要 | `## 功能清单` + `## 技术架构` | `docs/features/web-ui-mvp/07_DELIVERY.md` 第 22-36 行 | 内容摘录到 README §功能列表 + HTML §2 |
| T-002 功能摘要 | `## 功能摘要` | `docs/features/_archived/zero-config-quickstart/07_DELIVERY.md` 第 10-21 行 | 内容摘录到 README §功能列表 + HTML §2 |
| 测试基线数字 | `## 测试基线变化` 表格 | `docs/features/_archived/zero-config-quickstart/07_DELIVERY.md` 第 72-77 行 | 数字直接写入 HTML §4 |
| 技术债清单 | F-4 表格 TD-1..TD-8 | `docs/features/readme-and-health-report/01_REQUIREMENT_ANALYSIS.md` 第 103-111 行 | 完整引入 HTML §5 |
| 优化建议清单 | F-5 表格 OPT-1..OPT-9 | `docs/features/readme-and-health-report/01_REQUIREMENT_ANALYSIS.md` 第 119-139 行 | 完整引入 HTML §6 |
| T-002 已知后续事项 | `## 已知后续事项` | `docs/features/_archived/zero-config-quickstart/07_DELIVERY.md` 第 82-83 行 | 引入 HTML §7 |

---

## 8. 风险分析

| # | 风险描述 | 可能性 | 影响 | 缓解措施 |
|---|---|---|---|---|
| R-1 | README 中端口值或命令与代码不一致（写错），用户构建失败 | 中 | 用户体验差 / AC-3 失败 | Developer 须逐字核对 `internal/appconf/config.go` 第 7-16 行端口注释和 `scripts/build.sh` 第 33 行命令；AC-3/AC-5 文本搜索可自动检测关键数值 |
| R-2 | HTML 在不同浏览器中布局错乱（尤其是 `position: sticky` 兼容性） | 低 | 可读性下降 / AC-7 测试感知问题 | 仅使用 CSS Flexbox + sticky（兼容 Chrome 56+/Firefox 59+/Safari 13+）；AC-7 要求浏览器打开验证 |
| R-3 | HTML 误引入外部 CDN 资源，离线时样式丢失 | 低 | AC-11 失败 | 设计约束：禁止 `<link>` 外部样式表和 CDN 字体；Code Reviewer 检查无 `http://` 或 `https://` 出现在 link/script 标签内 |
| R-4 | "更新流程"说明不够显眼，用户仍跳过 `npm run build` | 中 | 用户看到旧前端 | README 更新流程章节使用 Markdown 块引用 `> **警告**：...` 格式明确标注；Q-1 答案已确认 dist/ 未提交，须在文档中显式说明 |
| R-5 | HTML 中 TD/OPT ID 文本缺失，AC-8/AC-9 检查失败 | 低 | AC 失败 | Developer 写完后用文本搜索 "TD-1".."TD-8" 和 "OPT-1".."OPT-9" 逐一验证；各条目 ID 须出现在可见文本中（不仅在注释里） |
| R-6 | README 中快速开始使用了错误的 Windows 命令格式（正斜杠 vs 反斜杠） | 低 | Windows 用户迷惑 | 分平台展示：Linux/macOS 用 `scripts/build.sh`，Windows 用 `scripts\build.ps1`；`bin/frp-easy` 和 `bin\frp-easy.exe` 分别标注 |

---

## 9. 迁移 / 上线计划

**无迁移需求**。本任务仅新建两个文本文件，不修改任何现有文件：

- `README.md`（项目根目录，全新创建）
- `docs/project-status.html`（全新创建，`docs/` 目录已存在）

**回滚**：`git revert` 或直接删除两文件即可完全撤销，不影响任何运行时行为。

**verify_all 影响**：`scripts/verify_all.sh` 检测 Go 源码、TypeScript/Vue 代码，不扫描 `.md` 和 `.html` 文件，因此 PASS/FAIL/SKIP 计数不变（AC-13）。

---

## 10. 范围外说明

- **不修改**任何 Go、TypeScript、Vue 源代码（TD 和 OPT 仅文档化）。
- **不修改** `scripts/verify_all.sh` / `verify_all.ps1`（OPT-1 仅列为建议）。
- **不修改** `scripts/build.sh` / `build.ps1`（OPT-5 版本注入仅列为建议）。
- **不实现** OPT-7 健康检查端点（仅列为建议）。
- **不生成** OpenAPI schema（OPT-9 仅列为建议）。
- **不修复** TD-1 向导路由守卫漏洞（仅列为建议）。
- **不统一** TD-2 ParseIPFromJSON 重复（仅列为建议）。
- **不部署**文档到 GitHub Pages 或任何外部网站。
- **不添加**英文版 README（NF-4 已明确中文为主）。
- `docs/project-status.html` 不挂载为 frp_easy UI 的任何路由（需改 Go 代码，超出范围）。

---

## 11. 分区分配

本任务为纯文档，不涉及 Go / TypeScript / Vue 代码变更，无需按 DB/后端/前端功能分区。两个交付文件均分配给 **dev-backend**（最了解整体系统架构，能准确引用所有模块路径和端口）。

| 文件 | 分区 | 新建/编辑 | 依赖 |
|---|---|---|---|
| `README.md` | dev-backend | 新建 | — |
| `docs/project-status.html` | dev-backend | 新建 | README.md（术语一致性参考） |

### 派发顺序

1. **dev-backend**（写 `README.md`）
2. **dev-backend**（写 `docs/project-status.html`）

### 并行性

无并行，同一分区顺序完成。建议先写 README.md，让 HTML 复用"更新流程"和"功能列表"的措辞保持一致。

---

## 12. AC 覆盖矩阵

| AC ID | 条件 | 满足方式 | 交付文件与章节 |
|---|---|---|---|
| AC-1 | `README.md` 存在于项目根 | 新建 `README.md` | `README.md`（整个文件） |
| AC-2 | 含快速开始章节（git clone → build → run 三步） | README §快速开始：三步骤代码块，含 clone / build.sh / bin/frp-easy 三行 | `README.md` §快速开始 |
| AC-3 | 端口表含 8080 / 7400 / 7500 / 7000 四行 | README §默认端口表：4 行表格，数值来自 `internal/appconf/config.go` 第 9-14 行 | `README.md` §默认端口表 |
| AC-4 | 更新流程明确"仅 git pull + 重启不足够" | README §更新流程：`> 警告` 块 + "需重新构建"文字；四点原因第一条解释 dist/ | `README.md` §更新流程 |
| AC-5 | frp_easy.toml 四字段 UIBindAddr / UIPort / DataDir / LogDir 均出现 | README §配置说明：四字段表格含字段名、默认值、说明 | `README.md` §配置说明 |
| AC-6 | `docs/project-status.html` 存在 | 新建 `docs/project-status.html` | `docs/project-status.html`（整个文件） |
| AC-7 | HTML 用浏览器 `file://` 打开无 JS 错误 | HTML 无 `<script>` 标签，纯 CSS + HTML 实现导航和布局 | `docs/project-status.html`（整体设计约束） |
| AC-8 | TD-1 至 TD-8 全部 8 条出现 | HTML §5 技术债清单：8 行表格，每行首列包含 "TD-1".."TD-8" 可见文本 | `docs/project-status.html` §5 |
| AC-9 | OPT-1 至 OPT-9 全部 9 条出现 | HTML §6 优化建议清单：9 行表格，每行首列包含 "OPT-1".."OPT-9" 可见文本 | `docs/project-status.html` §6 |
| AC-10 | 测试基线数字 117 和 45 出现在测试基线区域 | HTML §4 测试基线：明确写出 "117 个 Go 测试" 和 "45 个前端测试（Vitest）" | `docs/project-status.html` §4 |
| AC-11 | 无外部 CDN 硬依赖，断网可正常渲染 | HTML `<style>` 块内联所有 CSS，无 `<link rel="stylesheet">` 外部引用，无 CDN 字体 | `docs/project-status.html`（整体设计约束） |
| AC-12 | 更新流程说明 DB 迁移自动执行 | README §更新流程：第二点明确 "storage.Open() 启动时自动执行所有未应用的迁移，无需用户手动执行 SQL" | `README.md` §更新流程 |
| AC-13 | `scripts/verify_all` 仍为 0 FAIL | 无任何源代码变更；verify_all 不扫描 .md / .html 文件 | 设计约束保证（无代码改动） |

---

## 13. 交付顺序建议

Developer 按以下顺序写文件（同一分区顺序执行）：

**步骤 1：`README.md`（先写）**

内容来自已知的命令字符串和配置注释，确定性最高。完成后可立即验证 AC-1 / AC-2 / AC-3 / AC-4 / AC-5 / AC-12。

参考顺序：项目简介 → 功能列表 → 前置条件 → 快速开始 → 默认端口表 → 配置说明 → 更新流程 → 开发模式 → 目录结构速览 → 技术债与优化建议（引导至 HTML）。

**步骤 2：`docs/project-status.html`（后写）**

内容量较大（TD × 8 + OPT × 9 + 架构表），先有 README 的措辞作参考，术语保持一致。Developer 需从需求文档 F-4/F-5 表格逐条复制 TD/OPT 条目，写完后用文本搜索确认 AC-8/AC-9 覆盖。完成后验证 AC-6 / AC-7 / AC-8 / AC-9 / AC-10 / AC-11。

**步骤 3：运行 `scripts/verify_all`**

确认 AC-13（0 FAIL）。

---

## 14. Verdict

**READY**

需求文档 `01_REQUIREMENT_ANALYSIS.md` 原标注 `BLOCKED ON USER`，但两个开放问题已在本次任务触发时由用户直接回答：

- **Q-1**：`internal/assets/dist/` 未提交 git（`.gitignore` 的 `dist/` 规则递归排除），更新流程必须包含 `npm run build`。已在设计的 §3.1 更新流程章节和 §8 R-4 风险中体现。
- **Q-2**：HTML 总览页路径为 `docs/project-status.html`。已在 §2 受影响模块和 §3.2 中明确。

设计无阻塞项，可进入 Gate Review 阶段。
