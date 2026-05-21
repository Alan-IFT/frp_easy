# 01 需求分析 — T-011 readme-refresh-and-network-defaults

> Harness 流水线 stage 1 产出。模式：**full**（7-stage）。
> 上游输入只读：`docs/features/readme-refresh-and-network-defaults/INPUT.md`（含 PM 5 项预决策）。
> 本文档把 PM 预决策结构化为可验证需求。原 3 个开放问题（§8 Q-1/Q-2/Q-3）已由 PM 裁决回填，verdict 已转 `READY FOR DESIGN`。

---

## 1. 背景

frp_easy 已交付 T-001 ~ T-010，是一个 Go + Vue 3 + SQLite 的单二进制 FRP Web 管理 UI。当前存在三类问题：

1. **文档失真**：根 `README.md` 像功能罗列模板，新用户读完无法快速判断"这是什么 / 能解决什么问题 / 如何上手"；`docs/project-status.html` 头部更新日期停留 `2026-05-16`、内容停在 T-005（测试基线 119+45=164、verify_all 17 项、§7 写"无后续事项"），与实际 T-010（baseline.json：Go 166 + 前端 57 = 223、verify_all 19 项）严重脱节；`docs/` 下其余文档（DEPLOYMENT.md、dev-map.md、architecture.html、spec/、根 openapi.yaml）存在 T-005~T-010 期间未同步的过时点。
2. **默认端口冲突风险**：UI 服务默认端口 `8080`（`internal/appconf/config.go` `Default()`）是开发环境最常见的占用端口之一，首启即撞端口的概率高。
3. **默认绑定地址不便运维**：UI 默认绑定 `127.0.0.1`，仅本机可访问。frp_easy 本质是远程内网穿透管理工具，运维场景天然需要从其他设备访问 Web UI；当前默认值迫使每个用户首启后手动改配置。

T-011 一次性处理：网络默认值变更（端口 + 绑定地址）+ 配套安全提示重构 + 全量文档刷新与审计。

关联历史任务（`docs/tasks.md`）：

| 任务 | 关联点 | 关键产出文件 |
|---|---|---|
| T-001 web-ui-mvp | 端口 8080 / 绑定 127.0.0.1 的原始决策（Q-10、NF-S4） | `docs/features/_archived/web-ui-mvp/01_REQUIREMENT_ANALYSIS.md` §Q-10、`03_GATE_REVIEW.md` F-7 |
| T-003 readme-and-health-report | 现 README.md 与 project-status.html 的创建任务；端口表 AC | `docs/features/_archived/readme-and-health-report/07_DELIVERY.md` |
| T-005 docs-and-api-schema | openapi.yaml 创建（28 路由）、project-status.html 末次刷新基准 | `docs/features/_archived/docs-and-api-schema/` |
| T-008 deploy-kit | DEPLOYMENT.md 创建、main.go usage 文本、package.{sh,ps1} | `docs/features/_archived/deploy-kit/` |
| T-010 deploy-polish-and-ci | browseropen（0.0.0.0→127.0.0.1 URL 改写）、CI、当前测试基线 | `docs/features/_archived/deploy-polish-and-ci/` |

> 本任务是上述任务的**文档与默认值后继**，不重新设计任何功能模块。

---

## 2. 目标

一句话：把 frp_easy 的对外文档刷新到 T-010 实际状态，并把 UI 服务默认端口改为 `7800`、默认绑定地址改为 `0.0.0.0`，同时把启动期安全提示重构为面向新默认值的简明提示。

---

## 3. 功能需求（FR）

### FR-1 README.md 重写为标准开源项目风格

- **FR-1.1** `README.md` 必须按以下固定章节顺序组织：① 项目名称 + 一句话简介；② 项目简介（这是什么 / 解决什么问题，2~5 句）；③ 功能亮点；④ 快速开始（最短上手路径）；⑤ 配置说明（`frp_easy.toml` 四字段表）；⑥ 默认端口表；⑦ 文档导航（指向 DEPLOYMENT.md / project-status.html / dev-map.md / openapi.yaml 等）；⑧ 开发模式（面向贡献者）；⑨ 目录结构速览；⑩ 许可证。
- **FR-1.2** 功能亮点章节必须覆盖 T-001 ~ T-010 已交付的全部对用户可见能力（含 T-006 E2E、T-008 部署套件、T-010 浏览器自动打开 / 日志轮转 / CI），不得停留在 T-001+T-002。
- **FR-1.3** README 中所有 UI 默认地址引用必须为变更后的值（绑定地址 `0.0.0.0` 的对外表述、本机访问 URL `http://127.0.0.1:7800`），不得出现 `8080`。
- **FR-1.4** README 必须包含许可证章节。PM 裁决 Q-2：本任务**不创建** `LICENSE` 文件（许可证选择属维护者法律决策，AI 不得擅自代选）；许可证章节必须如实写明"开源许可证待项目维护者确定"，并另行注明 `frp_linux/` 与 `frp_win/` 下随附的 frp 二进制属上游 `fatedier/frp`、遵循其 Apache-2.0 许可。
- **FR-1.5** 截图位为可选项；若加入，必须使用占位文本或相对路径占位，不得引入二进制图片导致仓库膨胀（本任务不要求实拍截图）。

### FR-2 刷新 docs/project-status.html

- **FR-2.1** `docs/project-status.html` 头部 `更新日期` 必须更新为本任务交付日期；`§4 测试基线` 必须更新为 `scripts/baseline.json` 当前值（Go 166 / 前端 57 / 合计 223 / passing 218）。
- **FR-2.2** `§2 已交付功能` 必须补齐 T-003 ~ T-010 的功能条目（至少覆盖：T-005 OpenAPI、T-006 E2E 烟雾测试、T-007 安全加固、T-008 部署套件、T-009 跨 shell parity、T-010 浏览器自动打开 / 日志轮转 / CI）。
- **FR-2.3** `§3 架构模块表` 必须补齐 T-010 后实际存在的模块（至少新增 `internal/browseropen/`、`internal/logrotate/`）。
- **FR-2.4** `§4 测试基线` 的版本节点演进表必须延伸到 T-010；`verify_all 项` 表必须更新为当前实际数（19 项 PASS，依据 commit `d22d0d8` "verify_all PASS:19"）。
- **FR-2.5** `§5 技术债清单` / `§6 优化建议清单` / `§7 已知后续事项` 必须更新到 T-010 实际状态（含 T-011 本次新增的网络默认值变更带来的取舍说明）。
- **FR-2.6** project-status.html 必须是一个可直接在浏览器打开的独立 HTML 文件（不依赖外部资源），交付时向用户提供其文件路径作为"链接"。

### FR-3 docs/ 文档过时审计与更新

- **FR-3.1** 必须通读并审计以下文档，列出过时点并更新到 T-010 实际状态：`README.md`、`docs/DEPLOYMENT.md`、`docs/dev-map.md`、`docs/architecture.html`、`docs/project-status.html`、`docs/workflow.md`、`docs/spec/README.md`、根 `openapi.yaml`。
- **FR-3.2** 审计结果必须在 `02_SOLUTION_DESIGN.md` 或本任务交付文档中以"过时点清单"形式留档（文件 → 行号 → 现状 → 目标）。
- **FR-3.3** 所有受端口变更（FR-4）与绑定地址变更（FR-5）影响的文档位置必须同步更新。

### FR-3b docs/architecture.html 深度刷新到 T-010

> PM 裁决 Q-1：`docs/architecture.html` 创建于 `2026-05-16`，明显过时（标"101 个 Go 测试""覆盖 14 个 AC"，未含 downloader / browseropen / logrotate 模块，端口 `8080`）。与 `project-status.html` 同列为必须深度刷新文档。

- **FR-3b.1** `docs/architecture.html` 的测试覆盖章节必须以 `scripts/baseline.json` 为真相源更新：Go 集成测试数 `101 → 166`、前端 Vitest 数 `45 → 57`、"覆盖全部 14 个 AC" 的表述更新到当前实际 AC 覆盖范围（不再写死 `14`，或更新为当前实际值）。
- **FR-3b.2** `docs/architecture.html` 的架构 / 模块描述必须补齐 T-002 ~ T-010 期间新增的模块，至少包括 `internal/downloader/`、`internal/browseropen/`、`internal/logrotate/`。
- **FR-3b.3** `docs/architecture.html` 中所有 UI 服务端口引用（第 650 行 `go run ./cmd/frp-easy`（端口 8080）、第 652 行 Vite proxy `http://127.0.0.1:8080`）必须改为 `7800`；UI 默认绑定地址表述必须与 FR-5 新默认值（`0.0.0.0`）一致。
- **FR-3b.4** `docs/architecture.html` 头部更新日期 / 版本标识（若有）必须更新为本任务交付日期。
- **FR-3b.5** `docs/architecture.html` 交付后必须可在浏览器独立打开（纯内联 HTML/CSS，无外链依赖）。

### FR-4 UI 默认端口 8080 → 7800

- **FR-4.1** `internal/appconf/config.go` 的 `Default()` 函数 `UIPort` 必须改为 `7800`；`Load()` 中补默认分支（`cfg.UIPort == 0` 时）必须改为 `7800`；包顶部端口表注释（"本 UI 服务（HTTP）" 行）必须改为 `7800`。
- **FR-4.2** 以下**UI 服务端口**位置必须从 `8080` 改为 `7800`：
  - `cmd/frp-easy/main.go` usage 文本 `UI 默认地址`（第 68 行）
  - `internal/appconf/config_test.go`（默认值断言、ListenAddr 断言、端口集合断言 `{"8080","7400","7500","7000"}`）
  - `internal/browseropen/browseropen_test.go`（`http://127.0.0.1:8080` 测试常量，第 116 / 127 行）
  - `web/vite.config.ts`（dev proxy `/api` 目标）
  - `web/playwright.config.ts`（`baseURL`、`webServer.url` 健康检查）
  - `scripts/start.sh` / `scripts/start.ps1`（注释中 "port 8080"）
  - `scripts/start-e2e-server.sh` / `scripts/start-e2e-server.ps1`（生成的 `UIPort = 8080`）
  - `scripts/package.sh` / `scripts/package.ps1`（生成的 `UIPort = 8080` 及 README 文案中的 `127.0.0.1:8080`）
  - `README.md`、`docs/DEPLOYMENT.md`、`docs/architecture.html`、`docs/project-status.html`、`openapi.yaml`（`servers[].url`）
- **FR-4.3** 端口被占用友好提示（`main.go` 第 237~239 行）当前建议值为 `UIPort+1`；本任务保持"建议值 = 当前端口+1"的相对逻辑（变更后即建议 `7801`），无需写死。
- **FR-4.4** 以下位置是 **FRP 业务代理端口（proxy.remotePort）或测试夹具端口**，**禁止修改**：
  - `internal/storage/storage_test.go:397` `LocalPort: 8080`（代理 LocalPort 示例）
  - `internal/storage/qa_t007_adversarial_test.go:47` `rp := 8080`（remotePort 示例）
  - `internal/httpapi/qa_ac_test.go:480` `rp := 8080 + idx`（remotePort 批量夹具）
  - `web/src/components/__tests__/qa_t007_adversarial.spec.ts`（`remotePort: 8080`，第 13 / 82 行）
  - `web/src/components/__tests__/ProxyForm.spec.ts`（`form.value.remotePort = 8080`，第 55 / 61 / 66 行）
  - `docs/features/_archived/` 下所有归档文档中的 `8080`（历史归档，只读，不修改）

### FR-5 UI 默认绑定地址 127.0.0.1 → 0.0.0.0

- **FR-5.1** `internal/appconf/config.go` 的 `Default()` 函数 `UIBindAddr` 必须改为 `0.0.0.0`；`Load()` 中补默认分支（`cfg.UIBindAddr == ""` 时）必须改为 `0.0.0.0`；包/函数相关注释（第 44~45 行 "默认仅监听 127.0.0.1（NF-S4）"）必须改写为符合新默认值的表述。
- **FR-5.2** `cmd/frp-easy/main.go` 现有的 NF-S4 WARN 逻辑（第 135~139 行，"绑定地址非 127.0.0.1 时打 WARN"）必须重构为**面向新默认值的简明安全提示**，满足以下全部条件：
  - (a) 当 `UIBindAddr` 为 `0.0.0.0` 或 `::`（对外可达）时，在 stderr 打印一条中文安全提示，内容必须包含：① UI 当前对局域网/公网可达的事实；② 提醒尽快完成 setup 创建管理员账号（未 setup 前的暴露窗口最危险）；③ 说明如何改回仅本机访问（编辑 `frp_easy.toml` 把 `UIBindAddr` 设为 `127.0.0.1`）。
  - (b) 当 `UIBindAddr` 为 `127.0.0.1` / `::1` / `localhost` 时，不打印该安全提示。
  - (c) 该提示是 stderr 单条文本，不阻塞启动、不改变退出码。
- **FR-5.3** `main.go` 现有"绑定 `0.0.0.0`/`::` 时把浏览器自动打开 URL 改写为 `127.0.0.1`"逻辑（第 268~270 行）必须保留并继续正确工作（新默认值下用户首启即触发改写路径）。
- **FR-5.4** README、DEPLOYMENT.md、project-status.html 必须清楚说明该取舍：为何默认 `0.0.0.0`、已有的认证加固（argon2id + session + CSRF + 限流）、以及如何在仅需本机访问时改回 `127.0.0.1`。

### FR-6 commit 由执行 agent 完成

- **FR-6.1** 本任务所有改动的 commit 由流水线执行（用户已授权"所有 commit 由你来操作"），commit message 遵循 `00-core.md` 约定（祈使语气、首行 ≤72 字符、正文解释 why、标注 `T-011`）。

---

## 4. 非功能需求（NF）

- **NF-1（验证闸门）** 全部改动完成后 `scripts/verify_all` 必须 PASS，且 PASS 项数不得低于变更前（当前 19）。测试数只升不降（`00-core.md` 红线 3）。
- **NF-2（兼容性）** `appconf.Load()` 对已存在的旧 `frp_easy.toml`（显式写了 `UIBindAddr = "127.0.0.1"` / `UIPort = 8080`）必须保持用户显式值不变 —— 默认值变更只影响**新建配置文件**与**字段缺省补默认**，不得静默改写用户既有配置。
- **NF-3（安全）** 默认 `0.0.0.0` 不得削弱任何现有认证机制（argon2id 哈希 / session cookie / CSRF / IP 限流），且 setup 未完成前的接口暴露面不得扩大（沿用现有 ReadyGate / setup 守卫行为）。
- **NF-4（跨平台）** 端口 / 绑定地址变更后，`verify_all` 在 PowerShell 与 Git Bash 双 shell 下均须 PASS（T-009/T-010 已建立的 parity 约束）。
- **NF-5（文档自洽）** 交付后全仓库（排除 `docs/features/_archived/`）不得再出现把 `8080` 当作 UI 服务端口、把 `127.0.0.1` 当作默认绑定地址的表述。

---

## 5. 边界条件

| 场景 | 期望行为 |
|---|---|
| 新环境首启（无 `frp_easy.toml`） | 写入默认配置：`UIBindAddr = "0.0.0.0"` / `UIPort = 7800`；stderr 打印 FR-5.2 安全提示；浏览器自动打开 URL 改写为 `http://127.0.0.1:7800` |
| 已有 `frp_easy.toml` 显式写 `UIPort = 8080` | 保持 `8080` 不变（NF-2）；不打安全提示（绑定仍 127.0.0.1 时） |
| 已有 `frp_easy.toml` 缺 `UIPort` 字段 | 补默认为 `7800`（FR-4.1） |
| `UIBindAddr` 显式设为 `127.0.0.1` | 不打 FR-5.2 安全提示；浏览器打开 `http://127.0.0.1:7800` |
| 端口 `7800` 也被占用 | 沿用现有友好提示路径，建议 `UIPort = 7801`（FR-4.3），退出码 2 |
| `LICENSE` 文件不存在 | 不创建 `LICENSE`；README 许可证章节写"开源许可证待项目维护者确定"（PM 裁决 Q-2，FR-1.4） |
| 归档文档 `docs/features/_archived/**` | 只读，不修改（FR-4.4 末条、FR-3.1 不含 _archived） |
| project-status.html / architecture.html 在无网络环境打开 | 必须完整渲染（FR-2.6 / FR-3b.5，纯内联 HTML/CSS，无外链） |

---

## 6. 验收标准（AC）

每条均可被 QA 独立验证。

### README

- **AC-1**（文本检查）`README.md` 含 §2 项目简介、§3 功能亮点、§4 快速开始、§5 配置说明、§10 许可证 等 FR-1.1 全部章节标题。
- **AC-2**（grep）`README.md` 中不出现字符串 `8080`；UI 本机访问 URL 出现为 `127.0.0.1:7800`。
- **AC-3**（文本检查）`README.md` 功能亮点提及 T-006 之后能力（E2E / 部署套件 / 浏览器自动打开 / 日志轮转 / CI 至少各 1 处可定位）。
- **AC-4**（文本检查）`README.md` 默认端口表四行端口为 `7800 / 7400 / 7500 / 7000`。
- **AC-4b**（文本检查）`README.md` 许可证章节文本含"开源许可证待项目维护者确定"语义，并提及 `frp_linux/` / `frp_win/` 下随附 frp 二进制属上游 `fatedier/frp` 的 Apache-2.0 许可；仓库根**不新增** `LICENSE` 文件。

### project-status.html

- **AC-5**（文本检查）`docs/project-status.html` 头部 `更新日期` 为本任务交付日期，不再是 `2026-05-16`。
- **AC-6**（文本检查）`§4 测试基线` 数字为 Go `166` / 前端 `57` / 合计 `223`，与 `scripts/baseline.json` 一致。
- **AC-7**（文本检查）`§2 已交付功能` 含 T-005 ~ T-010 的功能条目；`§3 架构模块表` 含 `internal/browseropen` 与 `internal/logrotate` 两行。
- **AC-8**（文本检查）`§4` verify_all 表 PASS 数为 `19`。
- **AC-9**（人工 / 浏览器）`docs/project-status.html` 在浏览器中可独立打开，TOC 导航、各 §1~§8 段落正常渲染，无外链依赖。

### docs 审计

- **AC-10**（grep）排除 `docs/features/_archived/` 后，全仓库不出现把 `8080` 作为 UI 服务端口的文本（`README.md` / `docs/DEPLOYMENT.md` / `docs/architecture.html` / `docs/project-status.html` / `openapi.yaml` 均无 `8080`）。
- **AC-11**（文本检查）`openapi.yaml` `servers[].url` 为 `http://127.0.0.1:7800`。
- **AC-12**（文本检查）过时点清单已在 `02_SOLUTION_DESIGN.md` 或交付文档留档（文件→行号→现状→目标）。
- **AC-12b**（文本检查）`docs/architecture.html` 测试覆盖章节 Go 测试数为 `166`、前端 Vitest 为 `57`，不再是 `101` / `45`；架构/模块描述含 `internal/downloader`、`internal/browseropen`、`internal/logrotate` 三个模块；头部更新日期不再是 `2026-05-16`。
- **AC-12c**（人工/浏览器）`docs/architecture.html` 在浏览器中可独立打开，各章节正常渲染，无外链依赖。

### 端口变更

- **AC-13**（单元测试）`go test ./internal/appconf/...` 全 PASS；`config_test.go` 默认值断言为 `UIPort == 7800`、`ListenAddr()` 含 `7800`、端口集合断言含 `7800`。
- **AC-14**（grep + 编译）`internal/appconf/config.go` `Default()` 与 `Load()` 补默认分支均为 `7800`；`go build ./...` 成功。
- **AC-15**（grep）FR-4.4 列出的 FRP 业务代理端口 / 测试夹具 `8080` 全部**仍为 8080**（未被误改）。
- **AC-16**（编译 / 运行）`web/vite.config.ts` dev proxy、`web/playwright.config.ts` baseURL 与 webServer.url 均指向 `7800`；E2E（`verify_all` C.1）PASS。

### 绑定地址变更

- **AC-17**（单元测试）`internal/appconf/config_test.go` 默认值断言为 `UIBindAddr == "0.0.0.0"`；`go test ./internal/appconf/...` PASS。
- **AC-18**（运行 / 人工）首启无配置文件时，stderr 输出包含 FR-5.2(a) 三要素的中文安全提示；`UIBindAddr` 为 `127.0.0.1` 时不输出该提示。
- **AC-19**（运行 / 人工）首启无配置文件时，自动打开的浏览器 URL 为 `http://127.0.0.1:7800`（非 `0.0.0.0:7800`）。
- **AC-20**（单元测试 / 人工）已存在且显式写 `UIBindAddr = "127.0.0.1"` 的 `frp_easy.toml` 经 `Load()` 后值仍为 `127.0.0.1`（NF-2 不静默改写）。

### 闸门

- **AC-21**（verify_all）`scripts/verify_all` 在 PowerShell 与 Git Bash 下均 PASS，PASS 项数 ≥ 19，测试总数 ≥ 223。

---

## 7. 风险

| ID | 风险 | 缓解 |
|---|---|---|
| R-1 | 误改 FRP 业务代理端口（FR-4.4 那批 `8080`），破坏 storage/httpapi/ProxyForm 测试语义 | FR-4.4 显式枚举禁改清单；AC-15 专门校验这批仍为 8080 |
| R-2 | `appconf.Load()` 默认值变更意外改写用户既有 `frp_easy.toml` | NF-2 + AC-20 明确：默认值只作用于新建文件与缺省字段；现有 `Load()` 逻辑已是"用户值优先"，不得改动该语义 |
| R-3 | T-001 NF-S4（默认仅 127.0.0.1）是已批准的安全约束，本任务反转其默认值 | 这是 PM 预决策 2 的明确授权；NF-3 要求不削弱认证、不扩大 setup 前暴露面；FR-5.4 要求文档说明取舍。NF-S4 的"安全提示"意图通过 FR-5.2 重构后保留 |
| R-4 | architecture.html / DEPLOYMENT.md 中 `8080` 出现位置多且分散，易遗漏 | FR-3.1 全量审计 + AC-10 grep 兜底 |
| R-5 | 0.0.0.0 默认下浏览器自动打开若改写逻辑失效，用户首启即打不开 UI | FR-5.3 要求保留并验证现有改写逻辑；AC-19 专项验证 |
| R-6 | project-status.html 是大块手工 HTML，刷新时数据易与 baseline.json / 实际不一致 | FR-2.1 锚定 `baseline.json` 为数字真相源；AC-6/AC-8 grep 校验 |

---

## 8. 开放问题（已由 PM 裁决，回填闭环）

> 原 3 个开放问题已全部由 PM 裁决，结论回填如下；无遗留开放问题。

**Q-1 — `docs/architecture.html` 刷新深度？→ 裁决：深度刷新到 T-010（候选 b）。**
理由：用户原始需求明确要求"检查项目是否有过时的文档，若有则更新到最新实际情况"；PM 核查 `docs/architecture.html` 创建于 `2026-05-16`，明显过时（标"101 个 Go 测试""覆盖 14 个 AC"，未含 downloader / browseropen / logrotate 模块，端口 `8080`）。`architecture.html` 与 `project-status.html` 同列必须深度刷新文档。已升为正式需求 **FR-3b**（测试基线锚定 `scripts/baseline.json`、模块表补齐 T-002~T-010 新增模块、端口改 `7800`、AC/测试数更新到实际），对应 **AC-12b / AC-12c**。

**Q-2 — `LICENSE` 文件与 README 许可证章节？→ 裁决：不创建 `LICENSE`，README 如实写"开源许可证待项目维护者确定"（候选 c 变体）。**
理由：仓库根无 `LICENSE` 文件，许可证选择属项目维护者的法律决策，AI 不得擅自代选。README 许可证章节并须注明 `frp_linux/` / `frp_win/` 下随附的 frp 二进制属上游 `fatedier/frp`、遵循其 Apache-2.0 许可。已回填 **FR-1.4**，对应 **AC-4b**。PM 在交付报告中单独提示用户。

**Q-3 — `docs/spec/` 是否新增网络默认值 SPEC？→ 裁决：不新增（候选 a）。**
理由：`internal/appconf/config.go` 头部注释端口表 + 本任务需求/设计文档 + README 配置章节已足够承载该约束；新增 spec 文档反而增加维护面，违背"长期易维护"原则。`docs/spec/` 保持现状。

---

## 9. 范围边界

### In scope（本任务做）

- README.md 全量重写（FR-1）。
- project-status.html 刷新到 T-010（FR-2）。
- `docs/` 下非归档文档的过时审计与更新（FR-3）。
- architecture.html 深度刷新到 T-010（FR-3b）。
- UI 服务默认端口 `8080 → 7800`（FR-4，含代码 / 测试 / 脚本 / 文档 / 配置全部 UI 端口位置）。
- UI 服务默认绑定地址 `127.0.0.1 → 0.0.0.0` 及 main.go 安全提示重构（FR-5）。
- 所有改动的 commit（FR-6）。

### Out of scope（本任务明确不做）

- **不修改 FRP 业务代理端口（proxy.remotePort）/ 测试夹具端口**：`internal/storage/storage_test.go` `LocalPort: 8080`、`internal/storage/qa_t007_adversarial_test.go` `rp := 8080`、`internal/httpapi/qa_ac_test.go` `rp := 8080 + idx`、`web/src/components/__tests__/qa_t007_adversarial.spec.ts` 与 `ProxyForm.spec.ts` 的 `remotePort = 8080` —— 这些是 FRP 代理端口示例值，与 UI 服务端口无关，保持不变（FR-4.4 / AC-15）。
- **不修改 `docs/features/_archived/` 下任何归档文档**（历史只读；其中的 `8080` 是当时事实记录）。
- **不修改 frpc admin / frps dashboard / frps bindPort 端口**（7400 / 7500 / 7000 保持不变）。
- **不新增功能、不重构功能模块、不改 API 形状**（纯文档 + 默认值 + 启动提示文本改动）。
- **不实拍 / 不引入二进制截图文件**（FR-1.5：截图位仅为可选占位）。
- **不改 verify_all 检查项本身**（仅要求其继续 PASS）。
- **不创建 `LICENSE` 文件**（PM 裁决 Q-2：许可证选择属维护者法律决策，AI 不代选）。
- **不新增 `docs/spec/network-defaults.md`**（PM 裁决 Q-3：现有载体已足够，避免增加维护面）。

---

## 10. Verdict

**READY FOR DESIGN**

原 3 个开放问题（Q-1 架构 HTML 刷新范围、Q-2 LICENSE 文件与许可证类型、Q-3 是否补 spec）已全部由 PM 裁决并回填 §8，无遗留开放问题。功能需求 FR-1 ~ FR-6（含新增 FR-3b）与验收标准 AC-1 ~ AC-21（含新增 AC-4b / AC-12b / AC-12c）完整且可验证。PM 推进至 stage 2（Solution Architect）。

需求计数：功能需求 **7 个 FR 组**（FR-1、FR-2、FR-3、FR-3b、FR-4、FR-5、FR-6）、非功能需求 5 个（NF-1 ~ NF-5）、验收标准 **24 条**（AC-1 ~ AC-21，含 AC-4b、AC-12b、AC-12c）。
