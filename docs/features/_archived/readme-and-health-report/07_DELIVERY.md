# 交付报告 — T-003 readme-and-health-report

**交付日期**：2026-05-16  
**任务 ID**：T-003  
**Slug**：readme-and-health-report  
**PM**：PM Orchestrator

---

## 功能摘要

T-003 为 frp_easy 项目补全了用户文档体系，交付以下内容：

1. **根目录 README.md**（新建）：面向新用户的入口文档，含项目简介、T-001+T-002 功能列表、前置条件、Linux/Windows 快速开始三步流程、默认端口表（8080/7400/7500/7000）、frp_easy.toml 四字段配置说明、更新流程（明确 git pull 不够、需 build）、开发模式（start.sh）、目录结构速览，以及指向 project-status.html 的技术债和优化建议引用。

2. **docs/project-status.html**（新建）：完全自包含的项目状况总览页，可用浏览器 file:// 协议直接打开，左侧 sticky TOC 导航，无外部依赖、无 JS。内容包括：技术栈一览、已交付功能（T-001+T-002）、架构模块表、测试基线（117 Go / 45 Frontend）、8 条技术债（TD-1～TD-8，含影响级别标签）、9 条优化建议（OPT-1～OPT-9，含高/中/低优先级颜色标签）、已知后续事项、更新流程说明。

3. **关键问题解答**：
   - **git pull 后直接重启够不够？** 不够。前端 SPA 通过 `//go:embed all:dist` 嵌入二进制，`dist/` 被 .gitignore 排除（未提交 git），每次更新后必须运行 `scripts/build.sh`（含 npm run build + go build）才能获取最新版本。数据库迁移在新二进制首次启动时自动执行，无需手动 SQL。

---

## 新增文件

| 文件 | 说明 |
|---|---|
| `README.md` | 项目根目录用户入口文档（中文，213 行） |
| `docs/project-status.html` | 项目状况总览 HTML（完全自包含，~620 行） |

---

## verify_all 输出

```
=== Summary ===
  PASS: 12
  WARN:  0
  FAIL:  0
  SKIP:  6
```

---

## 代码评审修复记录

| 问题 | 严重度 | 修复 |
|---|---|---|
| README 快速开始节标题"Linux / macOS"不准确（build.sh 只产 Linux ELF） | MAJOR | 改为"Linux"，新增 macOS 说明段 |
| 端口表脚注"五者"与 4 行表不一致 | MINOR | 改为"四者"，补注第五项为用户自定义 proxy.remotePort |
| HTML TD-6 描述"任何文档中未说明"已过时（README 已记录） | MINOR | 更新描述，注明已文档化，根本修复见 OPT-3 |

---

## 测试基线变化

| 维度 | T-002 结束 | T-003 结束 | 增量 |
|---|---|---|---|
| Go tests | 117 | 117 | 0 |
| Frontend tests | 45 | 45 | 0 |
| 文档文件 | 0 | 2 | +2 |

---

## 技术债清单（文档化，本任务不修复）

已在 HTML 中完整记录 TD-1～TD-8，最重要的：

- **TD-1（中）**：向导路由守卫漏洞——直接访问 `/wizard` 不被重定向
- **TD-3（中）**：verify_all 前端检查因 package.json 路径问题永久 SKIP，45 个前端测试不进质量门禁

## 优化建议（文档化，本任务不修复）

已在 HTML 中完整记录 OPT-1～OPT-9，高优先级：

- **OPT-1（高）**：修复 verify_all 前端路径（切换到 web/ 目录）
- **OPT-2（高）**：补全向导路由守卫

---

## Insight

**dist/ 嵌入依赖是 frp_easy 最容易被新用户踩到的陷阱**：`.gitignore` 中的 `dist/` 规则递归匹配 `internal/assets/dist/`，克隆后直接 `go build` 会因找不到 embed 目录报错，但错误信息不够直观。README 的"前置条件"警告块和"更新流程"章节已明确说明此依赖。若要从根本消除陷阱，OPT-3 建议选择 dist/ 是否提交 git 的明确策略。
