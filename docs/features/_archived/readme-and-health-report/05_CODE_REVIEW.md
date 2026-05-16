# 代码评审 — T-003 readme-and-health-report

**评审日期**：2026-05-16  
**任务 ID**：T-003  
**Code Reviewer**：独立评审

---

## 评审结论

**CHANGES REQUIRED**（1 MAJOR，2 MINOR）

---

## AC 覆盖矩阵

| ID | 条件 | 状态 |
|---|---|---|
| AC-1 | README.md 存在于项目根目录 | PASS |
| AC-2 | README 含"快速开始"三步 | PASS |
| AC-3 | README 含端口表（8080/7400/7500/7000） | PASS |
| AC-4 | README 明确"仅 git pull 不足" | PASS |
| AC-5 | README 含四字段说明 | PASS |
| AC-6 | docs/project-status.html 存在 | PASS |
| AC-7 | HTML 无 JS 错误 | PASS（无 script 标签） |
| AC-8 | HTML 含 TD-1～TD-8 | PASS |
| AC-9 | HTML 含 OPT-1～OPT-9 | PASS |
| AC-10 | HTML 测试基线 117/45 准确 | PASS |
| AC-11 | HTML 无外部 CDN 依赖 | PASS |
| AC-12 | README 说明迁移自动执行 | PASS |
| AC-13 | verify_all FAIL: 0 | PASS |

---

## 问题列表

### [MAJOR] README Linux/macOS 章节标题不准确

`build.sh` 第 33 行 `GOOS=linux GOARCH=amd64` 只产出 Linux ELF 二进制，macOS 用户执行 `./bin/frp-easy` 会得到 `Exec format error`。修复：将"Linux / macOS"改为"Linux"，添加说明 macOS 用户使用 `scripts/start.sh` 进行本地开发，或参考 build.sh 自行添加 darwin 目标（范围外）。

### [MINOR] README 端口表脚注"五者"应为"四者"

第 87 行"五者目前无重叠"，但 README 端口表只列 4 行。`config.go` 注释中有第五行是用户自定义 proxy.remotePort，README 未收录。改为"四者目前无重叠"或补充说明第五项。

### [MINOR] HTML TD-6 描述"任何文档中未说明"已过时

TD-6 末尾说"此依赖关系在任何文档中未说明"，但 README 已在前置条件和更新流程两处说明此依赖。建议更新 TD-6 描述，注明该依赖已在 README 文档化，根本修复策略见 OPT-3。
