# 开发记录 — T-011 readme-refresh-and-network-defaults

> Harness 流水线 stage 4 产出。Developer 实现已批准设计（02_SOLUTION_DESIGN.md），遵守 03_GATE_REVIEW.md 的 3 条强制开发期条件。

## 摘要

按设计实现了三件事：① 网络默认值变更（UI 端口 `8080 → 7800`、绑定地址 `127.0.0.1 → 0.0.0.0`），含代码、测试与脚本同步；② 把 main.go 原 NF-S4 的"偏离 127.0.0.1 即 WARN"逻辑重构为面向新默认值的中性安全提示；③ README.md 全量重写为标准开源结构，project-status.html / architecture.html 深度刷新到 T-010 实际状态，并审计同步 DEPLOYMENT.md / dev-map.md 等文档的过时点。verify_all 在 PowerShell 与 Git Bash 双 shell 下均 19 项 PASS。

## 改动文件清单

### 块 1+2 — 网络默认值 + 安全提示重构（代码与测试）

- `internal/appconf/config.go` — 包头端口表注释 `8080→7800`；`Default()` 上方 doc-comment 重写为新默认值表述；`Default()` 出厂值 `UIBindAddr "127.0.0.1"→"0.0.0.0"` / `UIPort 8080→7800`；`Load()` 缺省回填两处目标值同步改（条件判断本身不动，保证 NF-2 用户值优先）。
- `internal/appconf/config_test.go` — `TestLoad_DefaultsWrittenOnMissing` 默认值断言、`TestValidate` 四条用例、`TestListenAddr`、`TestDoc_PortTablePresent` 端口集合断言全部 `8080→7800`；**新增 `TestLoad_ExplicitLoopbackNotOverwritten`** 覆盖 AC-20（用户显式写 `127.0.0.1` + `8080` 经 Load 后不被改写）。
- `cmd/frp-easy/main.go` — 包头 `【NF-S4】` 注释重写为安全提示说明；`usageText` 的 `UI 默认地址` `8080→7800`；NF-S4 WARN 块重构为正向枚举（`== "0.0.0.0" || == "::"`）触发的安全提示；新增 `exposureNotice(port, cfgPath)` 函数，三要素中性文案。浏览器自动打开 URL 改写逻辑（行 268-270）按设计原样保留。
- `internal/browseropen/browseropen_test.go` — 测试常量 `http://127.0.0.1:8080→:7800`（输入 + 断言两处）。

### 块 1 — 前端配置与脚本

- `web/vite.config.ts` — dev proxy 目标 `:8080→:7800`。
- `web/playwright.config.ts` — `baseURL` 与 `webServer.url` `:8080→:7800`。
- `scripts/start.sh` / `scripts/start.ps1` — 注释 `port 8080→7800`；`start.ps1` 的 Go 未安装提示 `1.22+→1.25+`。
- `scripts/start-e2e-server.sh` / `scripts/start-e2e-server.ps1` — 生成的 TOML `UIPort 8080→7800`（按 F-1 条件，`UIBindAddr = "127.0.0.1"` 行已存在，仅改 UIPort 数字，未补 UIBindAddr 行）。
- `scripts/package.sh` / `scripts/package.ps1` — 打包的 `frp_easy.toml.example` 改为新出厂默认值（`UIBindAddr "0.0.0.0"` / `UIPort 7800`）；README 文案按语义区分（访问 URL → `http://127.0.0.1:7800`，监听地址描述 → `0.0.0.0:7800`）。
- `openapi.yaml` — `servers[].url` `:8080→:7800`。
- `scripts/baseline.json` — `version 4→5`、`updated→2026-05-21`、`test_count 223→224`、`passing_count 218→219`、`go_tests 166→167`，notes 更新。

### 块 3 — README.md 重写

- `README.md` — 全量重写为标准开源结构（项目名 + 简介 / 项目简介 / 功能亮点 / 快速开始 / 配置说明 / 默认端口表 / 文档导航 / 开发模式 / 目录结构 / 许可证）。功能亮点覆盖 T-001~T-011；端口 7800、绑定 0.0.0.0；新增"从其他设备访问 Web UI"与"仅本机使用：改回 127.0.0.1"两节；许可证章节如实写"待维护者确定" + frp 二进制属上游 Apache-2.0；不创建 LICENSE 文件。

### 块 4 — HTML 文档深度刷新

- `docs/project-status.html` — 头部更新日期 `2026-05-16→2026-05-21`；§2 补 T-003~T-011 功能表；§3 补 logrotate / browseropen 模块行 + 程序入口启动序列更新；§4 测试基线 `119/45/164→167/57/224`、verify_all `17→19` 项、版本演进表延伸到 T-011；§7 加 T-011 网络默认值变更取舍说明表。
- `docs/architecture.html` — 头部新增更新日期元素（`2026-05-21`）；Go badge `1.22→1.25`；模块详解补 downloader / browseropen / logrotate 三张卡片；前端 Pages 补 Wizard.vue / Settings.vue；安全卡片"默认仅本机"改写为"对外绑定 + 安全提示"；开发模式端口 `8080→7800`；测试覆盖 Go `101→167`、Vitest `45→57`、移除写死的"14 个 AC"表述；对抗性测试卡片补 E2E 与 verify_all 19 项说明。

### 块 5 — docs 过时审计

- `docs/DEPLOYMENT.md` — 启动示例输出 `http://127.0.0.1:8080→http://0.0.0.0:7800` + 新增 0.0.0.0 访问说明 callout；快速开始访问 URL、开发模式 Go API URL、端口占用提示文案（`8080/8081→7800/7801`）、`ss/lsof/Get-NetTCPConnection` 诊断命令端口、F.2 排障 URL 全部同步；F.2 "Connection refused"排障表述按新默认值（0.0.0.0）改写；B.1 前置 `Go 1.22+→1.25+`（与 go.mod `go 1.25.0` 一致）。
- `docs/dev-map.md` — README 行描述更新为 T-011 重写说明；docs 目录树补 `architecture.html` 行、project-status.html 行注明 T-011 刷新。
- `docs/workflow.md` / `docs/spec/README.md` — grep 核对无端口/绑定相关过时内容，无需改动。

## 24 条 AC 逐条落实说明

| AC | 落实 |
|---|---|
| AC-1 | README.md 含 FR-1.1 全部十章节（项目简介 / 功能亮点 / 快速开始 / 配置说明 / 默认端口表 / 文档导航 / 开发模式 / 目录结构速览 / 许可证）。 |
| AC-2 | README.md 重写后无 `8080`；本机访问 URL 为 `http://127.0.0.1:7800`。 |
| AC-3 | 功能亮点"工程化保障"节明确列出 E2E（T-006）/ 部署套件（T-008）/ 浏览器自动打开（T-010）/ 日志轮转（T-010）/ CI（T-010），各 1 处可定位。 |
| AC-4 | 默认端口表四行 `7800 / 7400 / 7500 / 7000`。 |
| AC-4b | 许可证章节写"开源许可证待项目维护者确定" + 注明 frp 二进制属上游 fatedier/frp 的 Apache-2.0；未创建 LICENSE 文件。 |
| AC-5 | project-status.html 头部更新日期 `2026-05-21`。 |
| AC-6 | §4 测试基线 Go `167` / 前端 `57` / 合计 `224`，与 baseline.json 一致。 |
| AC-7 | §2 含 T-003~T-011 功能条目；§3 架构模块表含 `internal/browseropen` 与 `internal/logrotate` 行（downloader 原已在）。 |
| AC-8 | §4 verify_all 表 PASS 数为 `19`。 |
| AC-9 | project-status.html 纯内联 HTML/CSS，未引入任何 `<link>` / `<script src>` 外链。 |
| AC-10 | 全仓库 grep `8080` 兜底（排除 `_archived/`）：剩余命中仅 FR-4.4 白名单的 FRP 代理端口夹具、AC-20 新测试的夹具、以及描述本次变更本身的文档语句，无 UI 服务端口遗漏。 |
| AC-11 | openapi.yaml `servers[].url` 为 `http://127.0.0.1:7800`。 |
| AC-12 | 过时点清单已在 02_SOLUTION_DESIGN.md §7 留档（24 行，文件→行号→现状→目标）。 |
| AC-12b | architecture.html Go 测试 `167`、Vitest `57`，不再是 `101`/`45`；模块描述含 downloader / browseropen / logrotate；头部新增更新日期 `2026-05-21`。 |
| AC-12c | architecture.html 纯内联，无外链。 |
| AC-13 | `go test ./internal/appconf/...` 全 PASS；config_test.go 默认值断言 `UIPort==7800`、ListenAddr 含 `7800`、端口集合含 `7800`。 |
| AC-14 | config.go `Default()` 与 `Load()` 回填均 `7800`；`go build ./...` 成功。 |
| AC-15 | FR-4.4 白名单 5 文件的 `8080`（storage_test.go / qa_t007_adversarial_test.go / qa_ac_test.go / qa_t007_adversarial.spec.ts / ProxyForm.spec.ts）一个未动，grep 确认仍为 8080。 |
| AC-16 | vite.config.ts proxy、playwright.config.ts baseURL 与 webServer.url 均 `7800`；e2e server 脚本 UIPort `7800`；verify_all C.1 E2E PASS。 |
| AC-17 | config_test.go 默认值断言 `UIBindAddr=="0.0.0.0"`；`go test ./internal/appconf/...` PASS；实跑首启生成 toml 确认 `UIBindAddr = '0.0.0.0'`。 |
| AC-18 | 实跑验证：首启无配置（默认 0.0.0.0）时 stderr 输出三要素安全提示；`UIBindAddr="127.0.0.1"` 时不输出。 |
| AC-19 | main.go 行 268-270 浏览器 URL 改写逻辑（0.0.0.0/:: → `http://127.0.0.1:7800`）按设计原样保留。 |
| AC-20 | 新增 `TestLoad_ExplicitLoopbackNotOverwritten` 单测通过；实跑验证显式 `127.0.0.1` 配置 Load 后不被改写。 |
| AC-21 | verify_all 在 PowerShell 与 Git Bash 下均 PASS 19 项，测试总数 224（≥223）。 |

## verify_all 结果

- 基线（变更前）：PASS 19 / WARN 0 / FAIL 0 / SKIP 0。
- 变更后（PowerShell）：PASS 19 / WARN 0 / FAIL 0 / SKIP 0。
- 变更后（Git Bash）：PASS 19 / WARN 0 / FAIL 0 / SKIP 0。
- Delta：0 新失败；Go 测试 +1（166→167，新增 `TestLoad_ExplicitLoopbackNotOverwritten`），test_count 223→224，baseline 同步上调。

Gate Review 3 条强制条件落实：
- **F-1**：e2e server 脚本仅改 `UIPort` 行数字，未补 `UIBindAddr` 行（该行本已存在），无重复键。
- **F-2**：本任务未引入任何 TODO/FIXME 注释；双 shell verify_all 实跑确认 PASS=19、A.3 TODO 预算 PASS。
- **F-3**：新增 AC-20 测试后已更新 baseline.json（go_tests 166→167、test_count 223→224、passing_count 218→219），数字以实跑 `go test ./...` 167 个测试函数核对一致。

## Design drift（设计漂移）

无。实现严格遵循 02_SOLUTION_DESIGN.md 与 03_GATE_REVIEW.md 的 3 条强制条件。

设计 §2.4 关于 e2e server 脚本"未显式写 UIBindAddr"的描述有误（Gate Review F-1 已纠正），实现按 F-1 实测以"该行已存在、只改 UIPort 数字"为准 —— 这是按 Gate Review 强制条件执行，非漂移。

附带说明（设计未明确点名、但属 FR-3 docs 过时审计范围内的发现）：`docs/DEPLOYMENT.md` B.1 前置条件、`scripts/start.ps1` 的 Go 未安装提示、`docs/architecture.html` badge 仍写 `Go 1.22`，而 `go.mod` 实际为 `go 1.25.0`。已一并更新为 `1.25`，与 README 一致。这属于 FR-3.1「审计并更新过时文档」的明确职责范围。

## Open issues for review（待评审注意）

- architecture.html 的 API 路由表仍为 T-001 的 22 条，未补 T-002 新增的 5 条（download-bin / download-status / wizard/status / wizard/complete / public-ip）。FR-3b 的 AC 硬性要求（测试数 / 模块 / 端口 / 日期）已全部落实；API 路由表补全不在 AC-12b 列举范围内，且 architecture.html 该节标题未写死路由数。如评审认为需补全，可作为后续微调。
- `passing_count` 在 baseline.json 设为 219（218+1）。verify_all B.4 实际只校验 `test_count != 0` 与 `>= baseline`，不逐条比对 passing_count；G.2 `go test ./...` 实跑兜底全 PASS。数字按"新增 1 个测试且通过"诚实推算。

## Dev-map updates

`docs/dev-map.md` 改动（本任务不新增代码模块，仅文档结构同步）：
- README 行描述更新为「标准开源结构…；T-011 重写」。
- docs 目录树新增 `architecture.html` 行；`project-status.html` 行注明「T-011 刷新到 T-010 实际」。

## Insight to surface

- `go.mod` 已是 `go 1.25.0`，但 `DEPLOYMENT.md` / `start.ps1` / `architecture.html` 多处仍写"Go 1.22+"——文档型最低版本要求与 go.mod 声明易脱节，过时审计应把 go.mod 顶部 `go X.Y` 作为最低版本真相源核对。 · evidence: docs/DEPLOYMENT.md:146 / go.mod:3

## Code Review 修复（M-1 / M-2）

Code Review（05_CODE_REVIEW.md）verdict=APPROVED，路由回 2 条 MINOR：

- **M-1**：`docs/architecture.html` API 路由表只列 21 条，缺 6 条真实路由。以 `internal/httpapi/router.go` 为权威源核对，补齐：
  - `GET /health`（router.go:72，顶层注册，绕过 ReadyGate 与全部业务中间件，公开）
  - `GET /system/public-ip`（router.go:119，需登录）
  - `POST /system/download-bin`（router.go:120，需登录）
  - `GET /system/download-status/{kind}`（router.go:121，需登录）
  - `GET /wizard/status`（router.go:122，需登录）
  - `POST /wizard/complete`（router.go:123，需登录）
- **M-2**：补齐后路由表共 **27 条**，与同文件 `downloader` 模块卡片（`POST /api/v1/system/download-bin`）一致，亦与 `docs/project-status.html:380`「T-001: 22 条；T-002: +5 条」（22+5=27）自洽。
- 核对：`architecture.html` API 路由表标题仅为「API 路由表」，无写死的路由计数文字，无需修正；`project-status.html` 计数描述已正确，不改动。
- 改动文件：`docs/architecture.html`（+6 行 `<tr>`），未触其它文件，无新增 TODO/FIXME。
- verify_all：Windows `pwsh -File scripts/verify_all.ps1` → PASS 19 / WARN 0 / FAIL 0（不变）。

## Verdict

READY FOR REVIEW
