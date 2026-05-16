# 需求分析 — T-003 readme-and-health-report

**任务 ID**：T-003  
**Slug**：readme-and-health-report  
**阶段**：req  
**日期**：2026-05-16  
**相关历史任务**：T-001 (`docs/features/_archived/web-ui-mvp/`)、T-002 (`docs/features/_archived/zero-config-quickstart/`)

---

## 1. 目标

为 frp_easy 项目提供完整的用户文档（README.md）、可本地浏览的 HTML 项目状况总览页、明确的更新流程说明，以及基于代码审查得出的技术债清单与优化建议清单。

---

## 2. 背景与现状（上下文摘要）

| 维度 | 当前状态 |
|---|---|
| 项目根目录 README.md | **不存在**（`docs/spec/README.md` 是规格索引，非用户文档） |
| HTML 总览页 | **不存在** |
| 前端构建产物 | `internal/assets/dist/` 由 `npm run build` 生成，通过 `//go:embed all:dist` 嵌入二进制 |
| 数据库迁移 | `storage.Open()` 启动时自动执行所有未应用的迁移（累加式，绝不修改已合并文件） |
| 配置向后兼容性 | `appconf.Load()` 对缺失字段补默认值，现有 `frp_easy.toml` 升级后不失效 |
| Go tests | 117 个（全部 PASS） |
| Frontend tests (Vitest) | 45 个（PASS，但 `verify_all` 中永久 SKIP，见 TD-3） |
| `.gitignore` 中 `dist/` 规则 | 递归匹配（含 `internal/assets/dist/`），dist 是否已提交需在 F-3 说明中确认 |

---

## 3. 功能需求

### F-1：项目根目录 README.md

在项目根目录 `README.md`（当前不存在）创建面向新用户的文档，覆盖以下内容：

1. 项目一句话描述（Go + Vue 3 + SQLite，单二进制 FRP Web 管理 UI）。
2. 功能列表（当前已交付功能：T-001 + T-002）。
3. **前置条件**：Go 1.22+，Node.js 18+，npm。
4. **快速开始**（从源码构建的完整步骤）：
   - `git clone`
   - `scripts/build.sh`（或 Windows：`scripts/build.ps1`）—— 含自动执行 `npm install && npm run build`
   - 运行 `bin/frp-easy`（Linux）或 `bin/frp-easy.exe`（Windows）
5. **默认端口表**（来自 `internal/appconf/config.go` 注释）：

   | 用途 | 默认端口 | 监听方 |
   |---|---|---|
   | frp_easy UI（HTTP） | 8080 | frp-easy 进程 |
   | frpc admin API | 7400 | frpc 子进程 |
   | frps dashboard | 7500 | frps 子进程 |
   | frps bindPort（FRP 控制通道） | 7000 | frps 子进程 |

6. **配置说明**：`frp_easy.toml` 的四个字段（UIBindAddr / UIPort / DataDir / LogDir），默认值，以及 UIBindAddr 非 127.0.0.1 时的安全警告。
7. **更新流程**（与 F-3 一致）。
8. **开发模式**：`scripts/start.sh`（Go API + Vite dev server）。
9. **目录结构速览**（关键目录一行说明，引用 `docs/dev-map.md`）。

### F-2：HTML 项目状况总览页

创建一个自包含 HTML 文件（`docs/project-status.html`），可用浏览器直接打开（不需要 HTTP server），内容包括：

1. 项目标题与当前版本（0.1.0）。
2. 技术栈一览（Go 1.25，Vue 3 + Vite，SQLite via modernc.org/sqlite，chi router）。
3. 已交付功能（T-001 + T-002）的功能摘要，各条目注明任务 ID。
4. 架构模块表（直接取自 `docs/dev-map.md` §功能在哪里）。
5. 当前测试基线（117 Go tests，45 Frontend tests）。
6. 技术债清单（与 F-4 同步，含优先级标签）。
7. 优化建议清单（与 F-5 同步，含优先级标签：高/中/低）。
8. 已知后续事项（T-002 delivery 中的两条）。
9. HTML 必须完全自包含：样式内联或使用 `<style>` 块，无外部 CDN 依赖（或在有 CDN 时可离线降级）。

### F-3：更新流程说明

明确回答"git pull 后直接重启是否足够"，结论如下（需写入 README 和 HTML）：

**完整更新流程（必须执行全部步骤）：**

```
git pull
scripts/build.sh          # Linux/macOS（含 npm run build + go build）
# 或 Windows：scripts/build.ps1
```

然后以新二进制替换旧进程并重启。

**原因说明（须在文档中解释）：**

1. **前端需要重新构建**：前端 SPA 通过 `//go:embed all:dist` 嵌入 Go 二进制。`dist/` 由 `web/npm run build` 生成，`build.sh` 自动完成此步骤。若用户只执行 `go build` 而跳过前端构建，嵌入的前端将是旧版本。
2. **数据库迁移自动运行**：`storage.Open()` 在启动时对所有未应用的迁移执行 `applyOne()`，用户无需手动执行 SQL。
3. **配置文件向后兼容**：`appconf.Load()` 对现有 `frp_easy.toml` 缺失字段补默认值，升级不会破坏现有配置。`frp_easy.toml` 在 `.gitignore` 中，`git pull` 不会覆盖用户配置。
4. **仅后端变更时的简化路径**：若明确只有 Go 代码变更（无前端改动），可执行 `git pull && CGO_ENABLED=0 go build ./cmd/frp-easy` 后重启，前端不需要重新构建。在不能确定时，始终用 `scripts/build.sh`。

**明确不足够的情形：**
- `git pull` + 重启旧二进制（未重新编译）：二进制不变，更新不生效。
- `git pull` + `go build`（跳过 `npm run build`）：后端更新，前端仍为旧版。

### F-4：技术债清单

基于代码审查得出以下已确认技术债（文档和 HTML 中均列出，含来源和影响级别）：

| ID | 描述 | 影响级别 | 来源 |
|---|---|---|---|
| TD-1 | **向导路由守卫漏洞**：`router.ts` 中 wizard 守卫仅在导航到 `/dashboard` 时触发。向导已处理后直接访问 `/wizard` 不被重定向，用户可重复进入向导流程 | 中 | T-002 delivery 已知问题 |
| TD-2 | **ParseIPFromJSON 重复**：`downloader.ParseIPFromJSON` 已导出，但 `handlers_system.go` 的 `fetchIPFromURL` 有自己的内联 `{"ip":"..."}` JSON 解析，未复用 | 低 | T-002 delivery 已知问题 |
| TD-3 | **verify_all 前端检查永久 SKIP**：`verify_all.sh` 在项目根查找 `package.json`，但 `package.json` 在 `web/` 目录，导致 B.1 typecheck、B.2 lint、B.3 unit tests 全部 SKIP，前端质量门禁实际上未执行 | 中 | 代码审查 + `scripts/verification_history.log` |
| TD-4 | **Version 字符串写死**：`var Version = "0.1.0"` — 若构建时未注入 `-ldflags "-X main.Version=..."` 则版本永远为 0.1.0，无法区分构建 | 低 | `cmd/frp-easy/main.go` |
| TD-5 | **slog 单写模式**：`newLogger` 在 logFile 非 nil 时仅写文件，stderr 不收 slog 输出（仅 startup banner 写 stderr）。`go run` 开发模式下 slog 日志写入文件，不便观察 | 低 | `cmd/frp-easy/main.go:newLogger` |
| TD-6 | **`dist/` .gitignore 歧义**：根目录 `.gitignore` 的 `dist/` 规则递归匹配，会忽略 `internal/assets/dist/`（Go embed 的输入目录）。若此目录未被 git 跟踪，克隆后须先 `npm run build` 才能 `go build`，但此依赖关系在任何文档中未说明 | 中 | `build.sh` + `.gitignore` + `internal/assets/assets.go` |
| TD-7 | **auto-restore 时 TOML 缺失无预检**：`autoRestoreProcs` 在 frpc.toml / frps.toml 不存在时直接 Start，子进程立即失败，状态标记为 error，UI 无法区分"配置未生成"与"进程崩溃" | 低 | `cmd/frp-easy/main.go:autoRestoreProcs` |
| TD-8 | **单 SQLite 连接并发限制**：`SetMaxOpenConns(1)` 使所有读写串行化，适合 MVP（≤200 proxy）但在多 tab 高频轮询时（logs/proc/status 均 2s 轮询）会产生队列等待 | 低 | `internal/storage/store.go` |

### F-5：优化建议清单

基于代码审查的优化建议，分高/中/低优先级：

**高优先级（影响正确性或可观测性）：**

| ID | 建议 | 说明 |
|---|---|---|
| OPT-1 | **修复 verify_all 前端路径** | 修改 `verify_all.sh` 和 `verify_all.ps1`，切换到 `web/` 目录执行 B.1-B.4 前端检查，使 45 个 frontend tests 真正进入质量门禁 |
| OPT-2 | **补全向导路由守卫** | 在 `router.ts` 的 `beforeEach` 中，当 `to.path === '/wizard'` 且 `wizard.handled === true` 时重定向到 `/dashboard`，堵住 TD-1 漏洞 |

**中优先级（改善开发体验和维护性）：**

| ID | 建议 | 说明 |
|---|---|---|
| OPT-3 | **明确 dist/ git 追踪策略** | 方案 A：在 `internal/assets/` 下加 `.gitignore` 内容为 `!dist/`（取消根规则），将构建产物提交到仓库，用户可直接 `go build`。方案 B：保持 dist/ 不提交，在 README 中明确"克隆后必须先 npm run build"。选一个方案并落实 |
| OPT-4 | **slog 双写（tee）** | 修改 `newLogger` 使用 `io.MultiWriter` 同时写文件和 stderr，使开发期 `go run` 和生产期 `bin/frp-easy` 都能在 stderr 看到日志 |
| OPT-5 | **版本注入标准化** | 修改 `build.sh` / `build.ps1` 读 `git describe --tags --always` 作为版本，注入 `-ldflags "-X main.Version=..."`；README 说明版本号由构建脚本管理 |

**低优先级（代码质量和未来扩展）：**

| ID | 建议 | 说明 |
|---|---|---|
| OPT-6 | **ParseIPFromJSON 统一** | 将 `handlers_system.go` 内联解析改用 `downloader.ParseIPFromJSON`，消除 TD-2 重复 |
| OPT-7 | **健康检查端点** | 添加 `GET /api/v1/health` 返回 `{"status":"ok","version":"..."}` 供外部监控（Uptime Kuma、Nginx upstream_check 等）使用 |
| OPT-8 | **auto-restore TOML 预检** | `autoRestoreProcs` 在 Start 之前检查对应 TOML 文件是否存在，不存在时记录 warn 并跳过，而非启动后失败 |
| OPT-9 | **OpenAPI Schema** | 当前无 OpenAPI 文档（verify_all D.1 永久 SKIP）。添加 OpenAPI 3.x schema 可改善前后端契约可见性，并使 D.1 从 SKIP 变为 PASS |

---

## 4. 非功能需求

- **NF-1（可移植性）**：README.md 使用标准 Markdown，在 GitHub / Gitea / 任意 Markdown 渲染器中正确渲染。
- **NF-2（离线可用）**：`docs/project-status.html` 完全自包含，断网时用浏览器打开功能完整。若使用第三方 CSS CDN，必须提供内联样式兜底。
- **NF-3（内容准确性）**：README 和 HTML 中的端口、目录路径、命令行与代码库现状一致；若有差异，以代码为准并在文档中注明。
- **NF-4（中文）**：所有用户文档以中文为主要语言（代码片段例外）。

---

## 5. 范围外（本次迭代不做）

- 不修改任何 Go / TypeScript / Vue 代码（技术债和优化仅列出，本任务不修复）。
- 不添加新 API 端点（OPT-7 健康检查仅列为建议，不实现）。
- 不部署文档到外部网站（GitHub Pages 等）。
- 不生成 OpenAPI schema（OPT-9 仅列为建议）。
- 不修改 `scripts/verify_all`（OPT-1 仅列为建议）。

---

## 6. 验收标准

| ID | 条件 | 验证方法 |
|---|---|---|
| AC-1 | `README.md` 存在于项目根目录 | `ls C:\Programs\frp_easy\README.md` 返回文件 |
| AC-2 | README 包含"快速开始"章节，含 `git clone`、`scripts/build.sh`、运行二进制三步 | 用文本搜索确认三步骤存在 |
| AC-3 | README 包含默认端口表（8080 / 7400 / 7500 / 7000 四行） | 文本搜索 "8080"、"7400"、"7500"、"7000" 均出现 |
| AC-4 | README 包含更新流程说明，明确指出"仅 git pull + 重启不足够，需重新构建" | 文本搜索 "build" 或 "重新构建" 在更新章节中出现 |
| AC-5 | README 包含 `frp_easy.toml` 四个字段说明（UIBindAddr / UIPort / DataDir / LogDir） | 文本搜索四字段名均出现 |
| AC-6 | `docs/project-status.html` 存在 | `ls C:\Programs\frp_easy\docs\project-status.html` 返回文件 |
| AC-7 | HTML 文件用浏览器直接打开（`file://` 协议）可正常渲染，不报 JS 错误 | 浏览器打开，F12 Console 无报错 |
| AC-8 | HTML 包含 F-4 中 TD-1 至 TD-8 全部 8 条技术债条目 | 文本搜索 "TD-1" 至 "TD-8" 均出现 |
| AC-9 | HTML 包含 F-5 中 OPT-1 至 OPT-9 全部 9 条优化建议条目 | 文本搜索 "OPT-1" 至 "OPT-9" 均出现 |
| AC-10 | HTML 中测试基线数字准确（117 Go tests，45 Frontend tests） | 文本搜索 "117" 和 "45" 出现在测试基线区域 |
| AC-11 | HTML 无外部 CDN 硬依赖（或有内联兜底样式），断网可正常渲染 | 断开网络，用浏览器打开，页面布局正常 |
| AC-12 | README 中"更新流程"说明数据库迁移自动执行，用户无需手动 SQL | 文本搜索"迁移"或"migration"在更新章节出现 |
| AC-13 | `scripts/verify_all` 运行结果仍为 0 FAIL（本任务仅新增文档文件，不改代码） | 运行 `scripts/verify_all.sh` 输出 `FAIL: 0` |

---

## 7. 相关任务

- **T-001** (`docs/features/_archived/web-ui-mvp/`)：确立了架构、端口、数据库迁移机制、认证系统。README 的端口表和目录结构直接来源于此。
- **T-002** (`docs/features/_archived/zero-config-quickstart/`)：交付了下载器、向导、公网 IP 检测。HTML 总览页的功能摘要包含此任务成果。T-002 的 07_DELIVERY.md 明确列出两条"已知后续事项"（TD-1、TD-2），本任务文档化这些债务。

---

## 8. 开放问题

**Q-1：`internal/assets/dist/` 是否已提交到 git 仓库？**

背景：`.gitignore` 有 `dist/`（递归），但 `build.sh` 将 Vite 输出写入 `internal/assets/dist/`，且 `assets.go` 用 `//go:embed all:dist` 嵌入该目录。若该目录未提交，克隆后必须先运行 `npm run build` 才能 `go build`。

候选答案：
- (a) 已提交（git track）：用户 `git pull` 会自动获得更新的前端产物，可直接 `go build`。
- (b) 未提交（.gitignore 排除）：克隆和每次 `git pull` 后都必须先 `npm run build`，README 需明确说明此依赖。

此问题影响 README 更新流程和 OPT-3 建议的优先级。

**Q-2：HTML 总览页放在哪个路径？**

背景：用户原始需求是"写个 HTML，给我链接，方便我阅读"，链接形式需要确定文件位置。

候选答案：
- (a) `docs/project-status.html`：在文档目录下，用文件管理器或 `file://` URL 打开。
- (b) `project-status.html`（项目根目录）：更易被发现，但混入根目录。
- (c) 作为 frp_easy UI 的一个路由（如 `/status`）：需修改 Go 代码，超出纯文档范围。

---

## 9. 结论

**BLOCKED ON USER**

存在 2 个开放问题（Q-1、Q-2），需要用户确认后方可推进至方案设计阶段。其中 Q-1 影响 README 更新流程的准确性，Q-2 影响 HTML 文件的交付路径。
