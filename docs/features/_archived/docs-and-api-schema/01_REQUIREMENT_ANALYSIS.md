# 需求分析 — T-005 docs-and-api-schema

**任务 ID**：T-005  
**Slug**：docs-and-api-schema  
**分析日期**：2026-05-16  
**模式**：full  
**版本**：v2（PM 决策已纳入，开放问题已关闭）

---

## 1. Goal

更新 `README.md` 和 `docs/project-status.html` 以反映 T-004 的交付结果，修复 `scripts/verify_all.sh` 和 `scripts/verify_all.ps1` 中 D.1 检查逻辑（当前前置条件对本项目目录结构永久失效，导致 D.1 始终 SKIP），并在项目根目录创建覆盖全部 28 条 REST API 端点的标准级 OpenAPI 3.x schema 文件。

---

## 2. PM 决策记录（关闭原问题 1 和问题 2）

| 决策 | 内容 | 依据 |
|---|---|---|
| openapi.yaml 位置 | 项目**根目录**（`openapi.yaml`） | 与现有 D.1 检测路径 `[[ -f openapi.yaml ]]` 一致；业界标准位置；修改量最小 |
| OpenAPI schema 完整度 | **标准（B）** | 含路径 + 状态码 + 请求/响应 body 字段名和类型；满足 API 测试客户端和文档生成工具的需求 |

---

## 3. In-scope behaviors

以下每条均可机器验证：

### 子任务 A：文档更新（README.md + project-status.html）

**A-1**  `README.md` 末尾"技术债与优化建议"章节不再包含字符串 `TD-1 ～ TD-8` 或等价表述"存在 8 条技术债"。

**A-2**  `README.md` 准确描述当前状态：TD-1～TD-7 已由 T-004 清偿，TD-8（SQLite 单连接）保留；OPT-1～OPT-8 已由 T-004 实现，OPT-9（OpenAPI schema）由 T-005 处理。

**A-3**  `README.md` 不再将以下三项作为"当前已知问题"示例列出：向导路由守卫漏洞、verify_all 前端检查路径、版本注入——这三项均已由 T-004 修复。

**A-4**  `docs/project-status.html` §5 技术债清单表格中，TD-1～TD-7 每行标注已修复状态（含文本"已修复"或等效 HTML 标记），TD-8 保留未修复标记。

**A-5**  `docs/project-status.html` §6 优化建议清单表格中，OPT-1～OPT-8 每行标注已实现状态，OPT-9 保留未实现标记（T-005 完成后由 QA 更新为已实现）。

**A-6**  `docs/project-status.html` §4 测试基线数字更新为 T-004 交付后的值：Go 测试 119 条，verify_all PASS 16 条，SKIP 2 条。

**A-7**  `docs/project-status.html` §7 "已知后续事项"表格移除 T-004 已修复的两条：向导路由守卫漏洞（OPT-2）、ParseIPFromJSON 重复（OPT-6）。

### 子任务 B：verify_all D.1 修复

**B-1**  `scripts/verify_all.sh` 中 D.1 块的前置条件从 `[[ ! -d src && ! -d apps && ! -d packages ]]` 改为 `[[ ! -f go.mod ]]`（即：本项目以 `go.mod` 存在作为 D.1 前置条件，而非依赖 `src/`/`apps/`/`packages/` 目录）。修改处须添加一行注释，说明替换原因。

**B-2**  修改后，在项目根目录（含 `go.mod`、含 `openapi.yaml`）运行 `bash scripts/verify_all.sh --quick` 时，D.1 输出 `PASS`，不输出 `SKIP`。

**B-3**  修改后，在项目根目录（含 `go.mod`、不含 `openapi.yaml`）运行脚本时，D.1 输出 `WARN`，不输出 `SKIP`，不输出 `FAIL`。

**B-4**  `scripts/verify_all.ps1` 中 D.1 块做相同修改（前置条件同样改为检测 `go.mod`），结果与 `verify_all.sh` 一致。

**B-5**  D.1 修改不影响 A.1～A.3、G.1～G.3、B.1～B.4、E.1～E.6 的判定结果。

### 子任务 C：openapi.yaml 创建

**C-1**  项目根目录存在文件 `openapi.yaml`，内容符合 OpenAPI 3.x 规范，可被 `npx @redocly/cli lint openapi.yaml`（或 `swagger-cli validate openapi.yaml`）无错误解析。

**C-2**  `openapi.yaml` 的 `paths` 字段覆盖 `internal/httpapi/router.go` 中的全部 28 条路由（路由清单见第 5 节），HTTP 方法总计数为 28，唯一路径数为 24。

**C-3**  `openapi.yaml` 中每条端点声明以下内容：HTTP 方法和路径、是否需要 session 认证（对应 `security` 字段或 `401` 响应）、成功响应状态码（200 或 204）、至少一个实际可能的错误响应码（从 400/401/403/404/422/429/500 中选）。

**C-4**（标准级别）`openapi.yaml` 中每个有请求 body 的端点包含 `requestBody` 的 schema，列出字段名和类型；每个有响应 body 的端点包含 `responses` 下对应状态码的 schema，列出字段名和类型。

**C-5**  `openapi.yaml` 不引入任何新的 Go 依赖（不修改 `go.mod`）；文件为静态 YAML 文本，不由运行时代码生成。

---

## 4. Out-of-scope

以下事项本次迭代明确不做：

- 不修改任何 Go handler 代码（`internal/httpapi/handlers_*.go`、`router.go`）。
- 不修改 Vue 前端代码（`web/src/`）。
- 不修改数据库迁移文件（`migrations/`）。
- 不添加 Swagger UI 服务端点（项目不提供内置 openapi 文档网页）。
- 不修复 TD-8（SQLite 单连接）——文档化其现状即可。
- 不处理 T-006 E2E 测试。
- 不对 openapi.yaml 做运行时校验（不在 Go 二进制启动时验证文件存在性）。
- 不为 openapi.yaml 字段添加枚举约束、长度限制或使用示例（完整级别 C，本次仅做标准级别 B）。

---

## 5. Boundary conditions

| 场景 | 要求行为 |
|---|---|
| 运行 verify_all 时 openapi.yaml 不存在 | D.1 输出 WARN，不输出 SKIP，不输出 FAIL；脚本因 warns>0 以 exit 1 退出 |
| 运行 verify_all 时 openapi.yaml 存在且 YAML 语法有效 | D.1 输出 PASS；若无其他 WARN/FAIL，脚本 exit 0 |
| 运行 verify_all 时 openapi.yaml 存在但 YAML 语法错误 | D.1 只检查文件存在性（输出 PASS）；语法验证由开发者用外部工具手动运行，不在脚本范围内 |
| 运行 verify_all 时 go.mod 不存在 | D.1 前置条件不满足，输出 SKIP（与其他 G.x SKIP 行为一致） |
| README.md 中技术债章节被完全删除 | 违反 A-2（须保留信息说明现状），需替换而非删除 |
| project-status.html §4 测试基线数字 | 仅更新 T-004 已确认的数字（119/16/2）；T-005 完成后的测试数字由 QA 在 06_TEST_REPORT.md 中记录，不在本任务中预填 |
| 路由清单（共 28 条，以 router.go 为权威源） | `GET /api/v1/health`（无认证）、`GET /api/v1/system/ready`、`POST /api/v1/setup`、`POST /api/v1/auth/login`、`POST /api/v1/auth/logout`、`POST /api/v1/auth/password`、`GET /api/v1/auth/me`、`GET /api/v1/auth/csrf`、`GET /api/v1/mode`、`PUT /api/v1/mode`、`GET /api/v1/proxies`、`POST /api/v1/proxies`、`PUT /api/v1/proxies/{id}`、`DELETE /api/v1/proxies/{id}`、`GET /api/v1/server`、`PUT /api/v1/server`、`GET /api/v1/client`、`PUT /api/v1/client`、`POST /api/v1/proc/{kind}/start`、`POST /api/v1/proc/{kind}/stop`、`POST /api/v1/proc/{kind}/restart`、`GET /api/v1/proc/status`、`GET /api/v1/logs/{kind}`、`GET /api/v1/system/public-ip`、`POST /api/v1/system/download-bin`、`GET /api/v1/system/download-status/{kind}`、`GET /api/v1/wizard/status`、`POST /api/v1/wizard/complete` |

> 注：任务描述中称"27 条路由"，以 `router.go` 实际代码为准计数为 28 条（T-004 新增的 `/api/v1/health` 已包含）。

---

## 6. Acceptance criteria

每条格式：`AC-<编号>：<条件>，<验证方法>`

**子任务 A：文档更新**

AC-A1：README.md 不再包含字符串"TD-1 ～ TD-8"，`grep -c "TD-1 ～ TD-8" README.md` 输出 `0`。

AC-A2：README.md 不再将"向导路由守卫漏洞"列为当前问题，`grep -c "向导路由守卫漏洞" README.md` 输出 `0`。

AC-A3：README.md 不再将"verify_all 前端检查路径"列为当前问题，`grep -c "verify_all 前端检查路径" README.md` 输出 `0`。

AC-A4：project-status.html 中 TD-1～TD-7 有已修复标记，`grep -c "已修复" docs/project-status.html` 输出值 ≥ 7。

AC-A5：project-status.html 中测试基线含数字 119，`grep -c "119" docs/project-status.html` 输出值 ≥ 1。

AC-A6：project-status.html 不再包含"ParseIPFromJSON 重复"条目（已由 T-004 OPT-6 修复），`grep -c "ParseIPFromJSON" docs/project-status.html` 输出 `0`。

**子任务 B：verify_all D.1 修复**

AC-B1：openapi.yaml 存在时 verify_all.sh D.1 输出 PASS，`bash scripts/verify_all.sh --quick 2>&1 | grep "D\.1"` 含字符串 `PASS` 且不含 `SKIP`。

AC-B2：openapi.yaml 不存在时 verify_all.sh D.1 输出 WARN 不输出 SKIP，临时重命名 openapi.yaml 后，`bash scripts/verify_all.sh --quick 2>&1 | grep "D\.1"` 含 `WARN` 且不含 `SKIP`；验证后恢复文件。

AC-B3：verify_all.sh D.1 块代码中存在替换原因注释，`grep -c "前置条件\|prerequisite\|go\.mod" scripts/verify_all.sh` 在 D.1 相关行输出 ≥ 1（人工审查确认注释在 D.1 块内）。

AC-B4：verify_all.ps1 D.1 在 openapi.yaml 存在时输出 PASS，`pwsh scripts/verify_all.ps1 --quick 2>&1 | Select-String "D\.1"` 含 `PASS`（或在 bash 下用 `grep "PASS"` 对 ps1 输出验证）。

**子任务 C：openapi.yaml 创建**

AC-C1：项目根目录存在 openapi.yaml，`test -f openapi.yaml && echo PASS` 输出 `PASS`。

AC-C2：openapi.yaml 覆盖 28 条路由方法，对 `paths` 下所有 HTTP 方法（get/post/put/delete）计数总和为 28（用 `yq` 或等效工具计数，或人工逐行核对路由清单）。

AC-C3：openapi.yaml 语法有效，`npx @redocly/cli lint openapi.yaml` 以 exit 0 退出，无错误输出（WARN 不计）。

AC-C4：openapi.yaml 中每个有请求 body 的端点（含 requestBody）的 schema 包含至少一个具名字段及其类型，人工审查以下代表性端点：`POST /api/v1/auth/login`、`POST /api/v1/proxies`、`PUT /api/v1/proxies/{id}`、`PUT /api/v1/server`；`yq '.paths."/api/v1/auth/login".post.requestBody' openapi.yaml` 不为 null。

AC-C5：openapi.yaml 中每个有响应 body 的端点（200 含 content）的 schema 包含至少一个具名字段及其类型，人工审查代表性端点：`GET /api/v1/auth/me`、`GET /api/v1/proxies`、`GET /api/v1/server`；`yq '.paths."/api/v1/auth/me".get.responses."200".content' openapi.yaml` 不为 null。

AC-C6：openapi.yaml 不修改 go.mod，`git diff go.mod` 无输出（文件未变更）。

**整体验证**

AC-VERIFY：verify_all 整体 FAIL 计数为 0，`bash scripts/verify_all.sh --quick 2>&1 | grep "FAIL:"` 输出中 `FAIL: 0`。

---

## 7. Non-functional requirements

- **兼容性**：修改后 `verify_all.sh` 仍兼容 bash ≥ 4.0；`verify_all.ps1` 仍兼容 PowerShell 5.1+。
- **可维护性**：D.1 新前置条件须在脚本中添加一行注释说明替换原因，便于未来维护者理解为何不用标准的 `src/`/`apps/`/`packages/` 判断。
- **文件大小**：`openapi.yaml` 为手写或工具生成的静态文件，不引入新的 Go 依赖（不新增 `go.mod` 模块）。

---

## 8. Related tasks

| 任务 | Slug | 关联 | 文档路径 |
|---|---|---|---|
| T-003 | readme-and-health-report | 创建了 `README.md` 和 `docs/project-status.html`（本任务修改这两个文件） | `docs/features/_archived/readme-and-health-report/` |
| T-004 | tech-debt-cleanup | 清偿了 TD-1～TD-7 和 OPT-1～OPT-8，但未更新 README/HTML 文档，未创建 openapi.yaml（OPT-9 推迟至本任务） | `docs/features/_archived/tech-debt-cleanup/07_DELIVERY.md` |

---

## 9. Open questions for user

无。PM 已在 v2 前关闭全部问题：
- 问题 1（openapi.yaml 位置）→ **根目录**
- 问题 2（schema 完整度）→ **标准（B）**

---

## 10. Verdict

**READY**

所有歧义已由 PM 决策关闭。可推进至 Solution Architect 阶段。
