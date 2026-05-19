# 01 — 需求分析：T-008 deploy-kit

> Stage 1 of 7-stage `/harness` 流水线。
> 任务 ID：**T-008** · Slug：`deploy-kit` · 模式：full · 语言：中文
> 上游：用户原始诉求 + PM 已完成的缺口评估（见 `PM_LOG.md`）
> 本文档为下游 Solution Architect 的唯一输入。

---

## 1. 背景与动机

### 1.1 用户原始诉求（read-only）

> "当前项目是否可以直接写个部署文档，直接跟着部署文档就能部署和配置使用？最好能傻瓜式使用；若不能，则你来决策进行实现；以用户体验好，符合软件工程标准，长期易使用易维护为原则来决策。"

### 1.2 当前状态评估

`frp_easy` 自 T-001 起即以"单二进制 + 嵌入式前端"为目标架构（见 `cmd/frp-easy/main.go` 1 行注释、`README.md` §快速开始），但**截至 T-007 完成时，向终端用户交付的方式仍要求**：

1. **本地安装 Go 1.22+ 和 Node.js 18+**（`README.md` L33–L40 前置条件）；
2. **`git clone` 仓库**（`README.md` L48-L58 / L66-L76）；
3. **运行 `scripts/build.sh` / `build.ps1`** 触发 `npm install` + `npm run build` + `go build`；
4. **手工启动 `./bin/frp-easy`**，没有进程托管。

这与"傻瓜式部署"的诉求存在质的差距。证据：

- `README.md` 把"安装 / 开发 / 更新 / dev-mode"四类信息混排，非开发者难以找到自己的入口（L1–L218，无独立部署章节）；
- 仓库无 `scripts/package.*`、无 `scripts/install-service.*`，也无 `docs/DEPLOYMENT.md`（`Glob scripts/*` 结果验证）；
- `cmd/frp-easy/main.go` 没有 `--version` / `--help` flag（L50–L210），用户拿到二进制后无法自检版本；
- `bin/` 在 `.gitignore` 中，且 `internal/assets/dist/` 也被排除（README L40），意味着**仓库本身无任何可直接运行的产物**。

### 1.3 痛点与目标用户

| 用户类型 | 当前痛点 | 目标体验 |
|---|---|---|
| **傻瓜用户**（非开发者：内网穿透自用、家庭服务器、小型工作室） | 装 Go/Node、跑构建脚本、辨别 `frpc.toml` 与 `frp_easy.toml`、手工拉起进程 | 下载压缩包 → 解压 → 双击/运行 → 浏览器自动打开 |
| **开发者**（贡献者、二次开发） | 信息散落于 README，更新流程的"为什么需要 npm build"埋在 L140 之后 | 单一 `docs/DEPLOYMENT.md` 索引三条路径，开发路径明确指向 `scripts/build.sh` |
| **运维**（生产环境、长驻服务） | 无 systemd / Windows Service 安装脚本，需自行写 unit；无干净卸载方法 | 一条命令安装为系统服务，一条命令卸载，幂等可重跑 |

### 1.4 与前序任务的关联

| 任务 | 与本任务关系 |
|---|---|
| T-001 `web-ui-mvp` | 奠定单二进制启动序列（main.go），本任务在其外层加打包/服务化壳 |
| T-002 `zero-config-quickstart` | 已实现 frp 子二进制 GitHub Releases 自动下载，使发布产物可以**不内嵌 frp_win/frp_linux**，进一步压缩包体 — 但本期为降低用户首启依赖网络的概率，仍**全量打包 frp 子二进制** |
| T-003 `readme-and-health-report` | 创建了 `README.md` 与 `docs/project-status.html`；本任务将重排 README、新增 `docs/DEPLOYMENT.md` |
| T-005 `docs-and-api-schema` | 沉淀了"openapi.yaml 字段名以 Go 常量为权威"的纪律 — 本任务的 DEPLOYMENT.md 命令同样必须**以脚本实际输出为权威**，不可手抄 |
| T-007 `hardening-pass-audit` | 已收紧 `ui.log` 至 `0o600` 等权限项；本任务的服务安装脚本须保留此类权限语义 |

---

## 2. 范围

### 2.1 In-scope（本期必做）

1. **跨平台打包脚本** `scripts/package.sh`（Linux/macOS bash）+ `scripts/package.ps1`（Windows PowerShell）。
2. **服务安装/卸载脚本** `scripts/install-service.sh` / `uninstall-service.sh`（Linux systemd）+ `scripts/install-service.ps1` / `uninstall-service.ps1`（Windows Service via `sc.exe`）。
3. **CLI flag**：`cmd/frp-easy/main.go` 新增 `--version` 与 `--help`（同义短 flag `-v`、`-h`）。
4. **部署文档** `docs/DEPLOYMENT.md`，结构为三条路径：A 二进制傻瓜路径 / B 源码开发者路径 / C 系统服务运维路径。
5. **README 重排**：顶部加入"部署"快速链接；删除 / 折叠现有的"更新流程"、"开发模式"与"快速开始"中与部署重复的细节，统一指向 `docs/DEPLOYMENT.md`。
6. **`scripts/verify_all` 集成**：打包脚本必须能在 `verify_all` PASS 后被独立调用且不破坏现有 18 项 PASS。
7. **所有产出语言为中文**（脚本注释、文档、错误提示）。

### 2.2 Out-of-scope（本期明确不做）

| 不做项 | 理由 |
|---|---|
| GitHub Actions / CI 自动 release pipeline | 与 PM 决策一致；本地脚本可重复运行即满足交付需求；CI 留作后续任务 |
| 多架构 Docker image / Helm chart | 用户诉求是傻瓜部署，容器化反而增加门槛；留作未来 |
| macOS 原生 service（launchd plist） | T-003 已注明 macOS 为"次要平台"，本期 macOS 仅保证 `package.sh` 可产出 darwin 二进制（若用户在 macOS 主机上跑），不提供 launchd 模板 |
| 自动升级 / in-place update | 复杂度高、风险大；本期仅承诺"下新包 → 解压覆盖 → 重启服务"的有据可循流程 |
| ARM / RISC-V 多架构产物 | 当前 `scripts/build.sh` 仅 amd64；本期保持一致，仅打 `linux-amd64` + `windows-amd64` |
| GUI 安装器（NSIS / WiX / pkg） | 与"压缩包 + 脚本"路线冲突；本期不引入新工具链 |
| 签名 / 公证（Authenticode / notarization） | 需付费证书 & Apple ID；超出 OSS 项目本期资源 |
| 重写现有 `scripts/build.sh` 行为 | 本任务在 build.sh 输出基础上"再加一层打包"，不破坏既有契约 |

### 2.3 与既有任务的边界

- T-002 的 frp 子二进制自动下载流程**继续存在**：用户在线时若发布包损坏或新版本 frp，可继续触发 UI 内下载。本任务的发布包**预置一份 frp 子二进制**，让首启即可用（不依赖外网）。
- T-007 的安全配置（`ui.log` 0o600、TOML 权限等）由 main.go 在运行时强制，本任务的服务脚本**不重新设置这些权限**，避免双重维护点。

---

## 3. 用户故事

按角色至少 3 类。每条故事描述"我做什么 → 我期望看到什么"。

### US-1 傻瓜用户（非开发者）

> 作为一个想在家用 NAS 上跑内网穿透的非开发者，我希望：
> 1. 在 GitHub Releases 页面下载 `frp-easy-<version>-linux-amd64.tar.gz`；
> 2. `tar xzf` 解压；
> 3. 阅读包内的 `README.txt` 跟两步指令；
> 4. 运行 `./frp-easy`，看到 stderr 输出"UI 已启动：http://127.0.0.1:8080"；
> 5. 浏览器打开后进入 setup 向导。
>
> 我**不需要**装 Go、Node、git、make，**不需要**编辑任何配置文件就能完成首启。

### US-2 开发者（贡献者）

> 作为一个想本地修改前端代码并验证的贡献者，我希望：
> 1. 在 `docs/DEPLOYMENT.md` 中找到"路径 B：源码构建"小节；
> 2. 看到 `scripts/build.sh` 仍是构建命令的权威入口；
> 3. README 的"开发模式"指引仍然可用（`scripts/start.sh`），但是不要喧宾夺主放在 README 顶部。

### US-3 运维（生产长驻）

> 作为一个想把 frp-easy 跑成系统服务的运维，我希望：
> 1. 在解压目录里运行 `sudo ./scripts/install-service.sh`（Linux）或 `Start-Process powershell -Verb RunAs scripts/install-service.ps1`（Windows）；
> 2. 安装脚本**幂等**：重复运行不会创建重复 unit，会刷新文件并重新 `systemctl daemon-reload` / 重启服务；
> 3. 安装完看到 `systemctl status frp-easy`（或 `sc query frp-easy`）显示 active / running；
> 4. 需要卸载时运行 `sudo ./scripts/uninstall-service.sh` / `uninstall-service.ps1`：服务停止 + unit 文件删除 + 服务注册项消失；**数据目录 `.frp_easy/` 与 `frp_easy.toml` 不被自动删除**（防误删）；
> 5. 服务日志能用 `journalctl -u frp-easy` / Windows Event Viewer 看到 stderr。

### US-4 升级用户（已部署 → 新版）

> 作为一个已经运行 v0.1.0 想升级到 v0.2.0 的运维，我希望：
> 1. 下载新发布包；
> 2. 停服务 → 解压覆盖二进制 → 启服务，三步即可完成；
> 3. `frp_easy.toml`、`.frp_easy/`（SQLite + 日志）不被覆盖；
> 4. 二进制本身向后兼容 DB schema（由 storage migration 引擎保证，本任务不重做该机制）。

---

## 4. 功能性需求（FR）

> 每条 FR 后附"验收指针"，对应第 6 节的 AC 编号。

### FR-1 打包脚本

- **FR-1.1** `scripts/package.sh` 必须在 Linux / macOS 上可执行（`#!/usr/bin/env bash` + `set -euo pipefail`），调用既有 `scripts/build.sh` 完成前后端构建，再产出发布包。【AC-1】
- **FR-1.2** `scripts/package.ps1` 必须在 Windows PowerShell 5.1+ 与 PowerShell 7+ 上可执行，`$ErrorActionPreference = "Stop"`，调用既有 `scripts/build.ps1` 完成前后端构建，再产出发布包。【AC-1】
- **FR-1.3** 输出目录固定为 `bin/release/`（相对仓库根），文件命名规则：`frp-easy-<version>-<os>-<arch>.<ext>`，其中：
  - `<version>` 从 `git describe --tags --always --dirty` 取，与 `build.sh` 当前的版本注入策略一致；
  - `<os>` 取自 `linux` / `windows`（本期不出 darwin/macos 发布包，但脚本允许通过参数指定）；
  - `<arch>` 取自 `amd64`；
  - `<ext>` Linux 为 `tar.gz`，Windows 为 `zip`。【AC-1】【AC-2】
- **FR-1.4** 发布包内容（解压后顶层目录名 = 压缩包去后缀名）必须包含：
  1. `frp-easy` 或 `frp-easy.exe`（主二进制）；
  2. `frp_linux/` 或 `frp_win/` 子目录之**一**（仅与目标 OS 匹配的那份，避免无谓体积）；
  3. 样例配置 `frp_easy.toml.example`（不与运行时配置同名，避免首启被误覆盖）；
  4. 部署快速开始文件 `README.txt`（≤ 80 行，纯文本，含三步指令）；
  5. 子目录 `scripts/`，内置对应平台的 `install-service.*` 与 `uninstall-service.*`；
  6. `LICENSE`（若仓库根有）与 `VERSION`（单行版本号）。【AC-3】
- **FR-1.5** 打包脚本必须在打包前**校验** `bin/frp-easy` / `bin/frp-easy.exe` 已存在且非空，校验 `frp_linux/` / `frp_win/` 子目录内至少含 `frpc` 与 `frps` 两个 frp 子二进制；任一缺失则中止并以非 0 退出码报错。【AC-4】
- **FR-1.6** 打包脚本必须**幂等**：重复运行不会因为旧产物存在而失败；旧 `bin/release/<同名>.tar.gz` 会被覆盖。【AC-5】

### FR-2 服务安装/卸载脚本

- **FR-2.1 Linux**：`scripts/install-service.sh` 创建 `/etc/systemd/system/frp-easy.service` unit；`uninstall-service.sh` 停止并移除该 unit。
  - unit 字段最少集合：`[Unit] Description=`、`After=network.target`；`[Service] Type=simple`、`ExecStart=<absolute path>/frp-easy`、`WorkingDirectory=<absolute path>`、`Restart=on-failure`、`User=<安装时指定或当前用户>`；`[Install] WantedBy=multi-user.target`。
  - 安装脚本需 `systemctl daemon-reload` + `systemctl enable --now frp-easy`。
  - 卸载脚本需 `systemctl disable --now frp-easy` + 删除 unit + `systemctl daemon-reload`。【AC-6】【AC-7】
- **FR-2.2 Windows**：`scripts/install-service.ps1` 通过 `sc.exe create frp-easy binPath= "<absolute>\frp-easy.exe" start= auto DisplayName= "FRP Easy"` 创建服务，并 `sc.exe start frp-easy`；`uninstall-service.ps1` 调用 `sc.exe stop` + `sc.exe delete`。
  - 检测当前会话非管理员时，必须**立刻**报错退出，提示用户右键以管理员运行；不得静默失败。【AC-6】【AC-7】
- **FR-2.3 幂等性**：两平台脚本重复运行不创建重复服务；若服务已存在，应刷新可执行路径并重启。【AC-8】
- **FR-2.4 干净卸载**：`uninstall-service.*` 仅移除服务注册项与 unit 文件；**不**删除 `frp_easy.toml`、`.frp_easy/` 数据目录与日志，避免误删用户配置。卸载脚本结束时打印一段中文提示，告知数据目录路径与手工清理命令。【AC-9】
- **FR-2.5** 安装脚本必须可接受**至少一个**参数：服务运行用户（Linux `--user <name>`）或服务显示名覆盖（Windows `-DisplayName "<...>"`）。无参数时使用默认（Linux 当前 `id -un`，Windows `"FRP Easy"`）。【AC-10】

### FR-3 CLI flag

- **FR-3.1** `cmd/frp-easy/main.go` 接受 `--version` 与 `-v`：打印 `frp-easy <Version>`（其中 Version 取自既有 ldflags 注入变量）后以退出码 0 退出，**不**执行启动序列。【AC-11】
- **FR-3.2** 接受 `--help` 与 `-h`：打印中文帮助，至少包含：用法、可用 flag 列表（`--version`、`--help`）、配置文件位置（`frp_easy.toml`，可通过 `FRP_EASY_CONFIG` 环境变量覆盖）、UI 默认地址（`http://127.0.0.1:8080`）、退出码含义（`0` 正常 / `1` 一般错误 / `2` 端口占用），随后以退出码 0 退出。【AC-12】
- **FR-3.3** flag 解析必须在加载 `appconf` **之前**执行；即用户即使没有 `frp_easy.toml` 也能跑 `--version` / `--help` 查看程序信息。【AC-13】
- **FR-3.4** 未知 flag 应打印简短错误（中文）+ 提示运行 `--help`，以退出码 2 退出（与既有"端口占用退出 2"语义一致）。【AC-14】

### FR-4 部署文档 `docs/DEPLOYMENT.md`

- **FR-4.1** 文档必须分三条路径标题清晰：
  - 路径 A — **下载发布产物**（推荐普通用户）：3 步内完成首启；含 Linux 与 Windows 各自的 `tar` / Expand-Archive 命令；
  - 路径 B — **源码构建**（开发者）：直接引用 `scripts/build.sh` / `build.ps1` 的 `--all` 等参数；
  - 路径 C — **作为系统服务运行**（运维）：分别给出 Linux systemd 与 Windows Service 的安装、状态查询、查看日志、卸载命令。【AC-15】
- **FR-4.2** 所有命令行片段必须**复制即可运行**：相对路径以发布包解压后的工作目录为基准；绝对路径需用占位符（如 `/opt/frp-easy` 或 `C:\Program Files\frp-easy`）并在路径首次出现处注释占位含义。【AC-16】
- **FR-4.3** 文档顶部必须放一张"我属于哪条路径？"决策表（角色 → 推荐路径）。【AC-17】
- **FR-4.4** 文档底部必须含"故障排查"小节，至少覆盖：端口被占用、UI 打不开、systemd 启动失败、Windows Service 未启动、frp 子二进制缺失五种场景；每种场景给出明确日志位置（`journalctl -u frp-easy` / `.frp_easy/logs/ui.log` / Windows Event Viewer 路径）。【AC-18】

### FR-5 README 重排

- **FR-5.1** README 顶部（项目标题正下方）必须新增一段"快速开始"指引，指向 `docs/DEPLOYMENT.md` 三条路径，并保留一条最短示例命令（让仅看 README 的访客 30 秒内知道下一步）。【AC-19】
- **FR-5.2** 现有 README 的"更新流程"小节（L119–L158）整体迁移至 `docs/DEPLOYMENT.md` 路径 A / C 内，README 仅保留一行链接。【AC-20】
- **FR-5.3** 现有 README 的"开发模式"小节（L161–L175）保留但下沉至 README 后段，明确标注"面向贡献者"。【AC-21】
- **FR-5.4** README 不得新增任何重复出现于 DEPLOYMENT.md 的命令；以"权威源"原则避免双重维护。【AC-22】

### FR-6 与 verify_all 的集成

- **FR-6.1** `scripts/package.*` 不得修改 `verify_all` 检测项；现有 18 项 PASS 在引入新脚本后仍 PASS。【AC-23】
- **FR-6.2** 打包脚本本身不必被 `verify_all` 调用（属于发布动作而非验证动作），但其失败行为必须以非 0 退出码体现，便于未来在 CI 接入。【AC-24】

---

## 5. 非功能性需求（NFR）

### NFR-1 跨平台

- **NFR-1.1** Linux：脚本须在 Ubuntu 22.04+ 与 Debian 12 默认 bash 环境下不依赖 GNU 扩展即可运行（不依赖 `realpath` 之类非 POSIX 工具时尽量用 fallback；如必须依赖，于脚本顶部 `command -v` 检测并打印中文安装提示）。
- **NFR-1.2** Windows：脚本须在 Windows 10 22H2+ 与 Windows Server 2019+ 自带 PowerShell 5.1 上跑通，亦兼容安装了 PowerShell 7 的环境。

### NFR-2 零运行时依赖

- **NFR-2.1** 解压发布包后，用户的目标主机**不需**额外安装 Go、Node、npm、git、curl、make 等任何工具即可启动 UI。**例外**：路径 C（系统服务）在 Linux 下需要 systemd（默认已存在）、Windows 下需要 `sc.exe`（默认已存在）。
- **NFR-2.2** 发布包内的二进制使用 `CGO_ENABLED=0` 静态链接（与既有 `build.sh` 一致），无外部 `.so`/`.dll` 依赖。

### NFR-3 安装幂等与可恢复

- **NFR-3.1** 服务安装脚本可重复运行不报错、不残留旧 unit 文件。
- **NFR-3.2** 卸载脚本即使在"服务从未安装"的状态下运行，也只打印中文提示信息并以退出码 0 收尾（友好降级）。

### NFR-4 文档可复制即用

- **NFR-4.1** `docs/DEPLOYMENT.md` 中每个代码块标注 shell 类型（`bash` / `powershell`）。
- **NFR-4.2** 涉及版本号、端口、路径的位置使用与脚本一致的占位符，避免出现 `<your-version>` 这种"用户得自己理解"的玄学。

### NFR-5 可维护性

- **NFR-5.1** 所有脚本顶部必须有≥ 5 行中文注释说明：用途、用法、参数、输出位置、退出码。
- **NFR-5.2** 文档与脚本的命令一致性由 **手工 + Code Review** 保证；本期不强求自动同步检查。

### NFR-6 语言

- **NFR-6.1** 所有新增脚本注释、错误输出、文档内容、`--help` 文本均为中文（与项目其它产出一致）。
- **NFR-6.2** systemd unit 字段值（如 `Description=`）使用中文短句（systemd 支持 UTF-8 字段）。

### NFR-7 安全

- **NFR-7.1** 服务安装脚本不得明文写入任何凭据；frp_easy 自身在首启会通过 `ensureFrpcAdminCreds`（`main.go` L261）生成 frpc admin 凭据并入 SQLite，本任务的服务脚本对该流程**只读引用**。
- **NFR-7.2** Linux unit 文件权限设为 `0o644`（systemd 标准），数据目录权限保持 frp_easy 进程运行时的 `0o755` / `0o700` 现状（由 `main.go` 决定）。
- **NFR-7.3** 发布包压缩前**移除**任何 `.git/`、`.frp_easy/`、`node_modules/`、`bin/release/` 自身的引用，避免循环包含或泄露开发主机信息。

### NFR-8 包体上限

- **NFR-8.1** 单平台发布包目标 ≤ 25 MB（参考：frp_win 与 frp_linux 各约 10 MB + frp-easy 二进制约 10 MB + 文档可忽略）。若超出，打包脚本以 WARN 提示但不失败。

---

## 6. 验收标准（AC）

> 每条 AC 必须可由 QA 在标准 Linux / Windows 环境复现验证。

| ID | 描述 | 验证方法 |
|---|---|---|
| **AC-1** | `scripts/package.sh --linux` 与 `scripts/package.ps1 -Windows` 在各自平台分别产出 `bin/release/frp-easy-<version>-linux-amd64.tar.gz` 与 `frp-easy-<version>-windows-amd64.zip` | 执行脚本，`ls bin/release/` 看到对应文件，文件非 0 字节 |
| **AC-2** | 文件名版本号与 `git describe --tags --always --dirty` 输出一致 | `tar tzf` / `Expand-Archive` 后顶层目录名与文件名（去后缀）一致 |
| **AC-3** | 解压发布包后顶层目录包含：主二进制、对应 OS 的 frp 子二进制目录（仅一份）、`frp_easy.toml.example`、`README.txt`、`scripts/install-service.*`、`scripts/uninstall-service.*`、`VERSION`、`LICENSE`（若仓库根有） | 解压目录后 `ls -la` / `Get-ChildItem` |
| **AC-4** | 当 `bin/frp-easy` 不存在时，运行 `package.sh` 立即报错并以非 0 退出 | 删除 `bin/frp-easy` 后执行 |
| **AC-5** | 重复执行 `package.sh` 两次，第二次成功且覆盖第一次产物（无 "file exists" 错误） | 连续执行两次，对比文件 mtime |
| **AC-6** | Linux：`sudo ./scripts/install-service.sh` 后 `systemctl is-active frp-easy` 输出 `active`；Windows：以管理员运行 `install-service.ps1` 后 `sc query frp-easy` 显示 `STATE: 4 RUNNING` | 标准发行版 + 标准 Windows 主机各一次 |
| **AC-7** | 同上场景，`uninstall-service.*` 后 `systemctl status frp-easy` 报 `Unit frp-easy.service could not be found`；Windows 报 `service does not exist` | 两平台各一次 |
| **AC-8** | 连续执行 `install-service.*` 两次，第二次仍以退出码 0 收尾，且服务仍处于 active / running | 两平台各一次 |
| **AC-9** | 卸载后 `frp_easy.toml` 与 `.frp_easy/` 仍存在原位 | 卸载前后对比 `ls` / `Get-Item` |
| **AC-10** | `install-service.sh --user nobody` 后 unit 中 `User=nobody`；`install-service.ps1 -DisplayName "FRP 测试"` 后 `sc query` 显示中文 DisplayName | 两平台分别校验 |
| **AC-11** | `./frp-easy --version` 输出 `frp-easy <非空版本号>` 且退出码 0；`./frp-easy -v` 行为同上 | 直接运行 |
| **AC-12** | `./frp-easy --help` 与 `-h` 输出含中文用法说明、flag 列表、配置文件位置、UI 默认地址、退出码语义，退出码 0 | 直接运行 |
| **AC-13** | 在没有 `frp_easy.toml` 的临时目录下运行 `./frp-easy --version` 仍正常输出，不报"加载 frp_easy.toml 失败" | `cd $(mktemp -d) && /path/to/frp-easy --version` |
| **AC-14** | `./frp-easy --foo` 打印中文未知 flag 错误并以退出码 2 退出 | 直接运行 |
| **AC-15** | `docs/DEPLOYMENT.md` 文件存在，含三个顶级章节"路径 A"/"路径 B"/"路径 C" | grep + 目视 |
| **AC-16** | DEPLOYMENT.md 内每条命令直接复制粘贴到目标 shell 即可运行（不需要替换 `<...>` 之外的占位符） | QA 走查 |
| **AC-17** | DEPLOYMENT.md 顶部含决策表（至少 3 行：傻瓜用户 / 开发者 / 运维 → 推荐路径） | 目视 |
| **AC-18** | DEPLOYMENT.md 底部"故障排查"小节覆盖列出的 5 种场景且每种含日志位置 | 目视 |
| **AC-19** | README 顶部存在"快速开始 / 部署"指引段，含 `docs/DEPLOYMENT.md` 链接与一行示例命令 | 目视 |
| **AC-20** | README 不再包含与 DEPLOYMENT.md 重复的"更新流程"详细命令；以一行链接代替 | diff README 前后 |
| **AC-21** | README "开发模式"小节仍存在，位置位于"快速开始 / 部署"小节之后 | 目视 |
| **AC-22** | grep README.md 与 DEPLOYMENT.md，相同命令行片段（≥ 一行）不重复 | grep 走查 |
| **AC-23** | 引入本任务全部改动后 `scripts/verify_all` 输出 `PASS: 18 / WARN: 0 / FAIL: 0` | 执行脚本 |
| **AC-24** | `scripts/package.sh` 在前置缺失时返回非 0 退出码（供未来 CI 集成） | 见 AC-4 验证副产物 |

---

## 7. 非目标 / 风险

### 7.1 明确的非目标

- **不做**自动升级、不做热替换、不做差量升级；用户升级方式为"停服 → 解压覆盖 → 启服"。
- **不做** GitHub Actions、不做发布签名、不做 Docker；这些为 Open Question PM-resolved 拍板的延后项。
- **不做**对 macOS 的服务化（launchd）；交付仅在 Linux 与 Windows 两平台保证生产级体验。

### 7.2 风险与缓解

| 风险 | 影响 | 缓解 |
|---|---|---|
| Windows 用户未以管理员运行 `install-service.ps1` 导致 `sc.exe` 静默失败 | 服务未实际创建 | 脚本入口检测当前会话权限，非管理员立即报错退出（FR-2.2） |
| `git describe` 在仓库无 tag 时返回 `dev`，导致发布包名为 `frp-easy-dev-linux-amd64.tar.gz` | 用户拿到包不知版本 | 接受此 fallback；同时在 `VERSION` 文件与 `--version` 输出中保持一致，CR 阶段提示用户在发布前打 tag |
| `frp_linux/` 与 `frp_win/` 自身体积大（各 ~10 MB），交叉打包时两份都嵌入会让包变大 | 单包超过 25 MB 警告阈值 | FR-1.4 约束"只包含目标 OS 的 frp 子目录" |
| systemd unit 中 `ExecStart` 写绝对路径，用户解压后移动目录会让服务失效 | 服务启动失败 | install-service.sh 在生成 unit 前用 `realpath` 锁定绝对路径，并在 README.txt 中提示"安装服务后不要移动目录" |
| 用户在已存在 `frp_easy.toml` 的目录解压新版本，覆盖问题 | 配置被覆盖 | 发布包内为 `frp_easy.toml.example`，**不**为 `frp_easy.toml`，避免覆盖现有配置 |
| 文档中命令与脚本实际行为漂移 | 文档失真 | Code Review 必须逐条对照脚本输出与文档命令；本期不引入自动校验 |
| Windows zip 解压后丢失可执行位（POSIX 模式） | Linux 用户在 Windows 上下载 zip 解压再传到 Linux 会 chmod +x | 仅在 Linux 平台发 `tar.gz`；zip 仅给 Windows |

---

## 8. Open Questions（含 PM-resolved 答复）

> 即使已默认决策，仍写在此处便于追溯。

1. **是否本期引入 GitHub Actions 自动 release？**
   - 候选 (a)：引入；(b)：本地脚本即可。
   - **PM-resolved**：(b) 本地脚本即可。理由：用户诉求是"傻瓜部署"，与发布自动化是两个维度；CI 留作单独任务。

2. **是否打 macOS 发布包？**
   - 候选 (a)：打 darwin-amd64 + darwin-arm64；(b)：本期仅 Linux + Windows。
   - **PM-resolved**：(b)。理由：macOS 不是 frp_easy 主要部署目标，且 `scripts/build.sh` 当前默认 GOOS=linux；本期保持一致。

3. **发布包内是否嵌入 frp 子二进制？**
   - 候选 (a)：嵌入（首启即可用，包变大）；(b)：留空，靠 T-002 在线下载（包小，依赖外网）。
   - **PM-resolved**：(a) 嵌入。理由：傻瓜用户首启时 GitHub 在中国网络不稳定，嵌入提升首启成功率；包体积 ≤ 25 MB 可接受。

4. **服务运行用户的默认值？**
   - 候选 (a)：root；(b)：当前 `id -un`；(c)：新建专用账户 `frp-easy`。
   - **PM-resolved**：(b) 当前 `id -un`。理由：最小惊讶；运维如需更安全可显式传 `--user nobody` 或自建账户。新建账户会涉及 useradd 副作用，本期不做。

5. **`--help` 与 `--version` 在二进制中是否使用第三方 flag 库（如 `pflag`、`cobra`）？**
   - 候选 (a)：标准库 `flag`；(b)：引入 `cobra`。
   - **PM-resolved**：(a) 标准库 `flag`。理由：本期 flag 数极少，且引入新依赖会扩大攻击面与维护负担。

6. **发布包顶层目录名是否带版本号？**
   - 候选 (a)：是（`frp-easy-v0.1.0-linux-amd64/`）；(b)：否（`frp-easy/`）。
   - **PM-resolved**：(a) 带版本号。理由：用户解压多个版本并存时不会冲突；与 Kubernetes / Node.js 等知名项目惯例一致。

7. **是否在发布包内置 `LICENSE`？**
   - 候选 (a)：是；(b)：仅靠 GitHub 仓库链接。
   - **PM-resolved**：(a) 是。理由：合规要求，分发二进制时应附 license 文本；若仓库根 LICENSE 缺失则跳过并 WARN，留作后续任务补 LICENSE。

8. **Windows 服务的失败重启策略**？
   - 候选 (a)：默认（`sc.exe` 不配置 failure action，崩溃后不自动重启）；(b)：配置 `sc.exe failure frp-easy reset= 60 actions= restart/5000`。
   - **PM-resolved**：(b)。理由：与 Linux 端 `Restart=on-failure` 行为对齐，给运维同等保障。

9. **DEPLOYMENT.md 中"路径 A"是否引用具体下载链接（GitHub Releases）？**
   - 候选 (a)：使用占位符 `https://github.com/<org>/frp_easy/releases/latest`；(b)：写死实际仓库地址。
   - **PM-resolved**：(a) 占位符。理由：仓库 org 名尚未敲定（见 README 当前用 `your-org/frp_easy`），统一占位符避免到处替换。

10. **如果运维已经通过其它方式（手写 systemd unit、Docker）跑了 frp-easy，再跑 `install-service.sh` 会冲突吗？**
    - 候选 (a)：脚本检测 `frp-easy.service` 已存在则交互式确认；(b)：脚本无脑覆盖。
    - **PM-resolved**：(b) 无脑覆盖。理由：脚本是幂等设计的必然结果（FR-2.3）；用户应清楚他们运行了 install-service.sh。脚本输出会清晰记录"已刷新现有 unit"。

---

## 9. Verdict

**READY** — 所有 Open Question 已被 PM 预先解决，可推进至 Stage 2（Solution Architect 编写 `02_SOLUTION_DESIGN.md`）。

下游 Architect 应基于本文件第 4 节 FR、第 5 节 NFR、第 6 节 AC 做技术选型与目录/接口设计，**不得**重新讨论第 2 节 In/Out-of-scope 与第 8 节 PM-resolved 答案。

---

## 附录 A — 参考文件与版本基线

| 路径 | 用途 |
|---|---|
| `C:\Programs\frp_easy\README.md` (218 行) | 当前用户文档，将被重排 |
| `C:\Programs\frp_easy\scripts\build.sh` (43 行) | Linux/macOS 构建脚本，被 package.sh 复用 |
| `C:\Programs\frp_easy\scripts\build.ps1` (53 行) | Windows 构建脚本，被 package.ps1 复用 |
| `C:\Programs\frp_easy\scripts\verify_all.sh` (277 行) | 18 项检查闸门，本任务交付前必须 PASS |
| `C:\Programs\frp_easy\cmd\frp-easy\main.go` (322 行) | 程序入口，FR-3 在此新增 flag |
| `C:\Programs\frp_easy\docs\dev-map.md` (150 行) | 项目导航，本任务交付后需追加 `docs/DEPLOYMENT.md` 与 `scripts/package.*` 索引 |
| `C:\Programs\frp_easy\.harness\insight-index.md` (17 行) | 跨任务事实清单；本任务若发现新坑则追加 |
| `C:\Programs\frp_easy\docs\tasks.md` | 任务看板，本任务 ID = T-008 |
