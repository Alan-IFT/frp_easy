# 01 — 需求分析：T-012 one-click-install-and-mit-license

> Stage 1 of 7-stage `/harness` 流水线。
> 任务 ID：**T-012** · Slug：`one-click-install-and-mit-license` · 模式：full · 语言：中文
> 上游：用户原始诉求 + PM 已完成的 4 项预决策（见 `INPUT.md`）+ PM 对 5 个开放问题的裁决（2026-05-22，见 8.2）
> 本文档为下游 Solution Architect 的唯一输入。

---

## 1. 目标

把 frp_easy 的安装从"下载 tar.gz → 手动解压 → 进版本号目录 → `./frp-easy`"的多步流程，压缩为一条 `curl ... | sudo bash`（Linux/macOS）或一条 PowerShell 命令（Windows）即可完成下载、安装、配置开机自启的一键安装；同时为仓库补齐正式的 MIT 许可证文件与上游 frp 二进制的 Apache-2.0 声明。

---

## 2. In-scope 行为（本期必做，可验证）

### A 块 — 一键安装

1. 仓库新增 `scripts/install.sh`（bash，Linux/macOS），脚本顶部含 `#!/usr/bin/env bash` 与 `set -euo pipefail`，可在管道形态 `curl -fsSL <raw-url>/scripts/install.sh | sudo bash` 下执行。
2. `install.sh` 探测当前主机 OS（`linux` / `darwin`）与 CPU 架构（`amd64`），据此决定要下载的发布包名片段 `<os>-<arch>`。
3. `install.sh` 调用 GitHub Releases API `https://api.github.com/repos/Alan-IFT/frp_easy/releases/latest` 解析出最新 release 的版本号与对应平台的发布包资产下载链接。
4. `install.sh` 下载与当前平台匹配的发布包（Linux：`frp-easy-<VERSION>-linux-amd64.tar.gz`），并解压到固定安装目录 `/opt/frp-easy`。
5. `install.sh` 在解压完成后调用解压目录内的 `scripts/install-service.sh` 配置 systemd 开机自启，复用其既有逻辑而不复刻。
6. `install.sh` 安装成功后向 stdout 打印安装目录、UI 访问地址（`http://<本机IP>:7800` 与 `http://127.0.0.1:7800`）、常用 systemd 命令。
7. `install.sh` 接受 `-h` / `--help`，打印中文用法说明（用途、安装目录、所需权限、所需依赖、退出码语义）后以退出码 0 退出。
8. 仓库新增 `scripts/install.ps1`（PowerShell），`$ErrorActionPreference = "Stop"`，实现 Windows 对等的一键安装：探测架构 → 调 GitHub API → 下载 `frp-easy-<VERSION>-windows-amd64.zip` → 解压到固定安装目录 `C:\Program Files\frp-easy` → 调用解压目录内 `scripts/install-service.ps1` 配置 Windows 服务 → 打印访问地址。可在管道形态 `irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex`（管理员 PowerShell）下执行。
9. `install.ps1` 接受 `-Help`（或 `-h`），打印中文用法说明后以退出码 0 退出。
10. 两个安装脚本对每个失败点（见第 4 节边界条件）均以中文友好报错信息写到 stderr，并以非 0 退出码退出，不静默失败。
11. 两个安装脚本对"目标安装目录已存在 frp-easy 安装"的情形按"升级"语义处理：停止现有服务 → 覆盖二进制与脚本 → 重新通过 `install-service.*` 拉起服务；不覆盖安装目录内的 `frp_easy.toml` 与 `.frp_easy/` 数据目录。

### B 块 — MIT 许可证

12. 仓库根新增 `LICENSE` 文件，内容为 MIT 许可证全文，版权行为 `Copyright (c) 2026 Alan_IFT`。
13. 仓库根新增 `NOTICE` 文件，以中文说明随附在 `frp_linux/` 与 `frp_win/` 下的 frp 二进制（frpc / frps）属上游项目 `fatedier/frp`，遵循 Apache-2.0 许可证，与 frp_easy 本身的 MIT 许可证相互独立。
14. `README.md` 的"许可证"章节（当前 L197–L201 "本项目尚未确定开源许可证…"）改写为正式 MIT 声明，并保留对随附 frp 二进制 Apache-2.0 归属的说明。

### A+B — 文档更新

15. `docs/DEPLOYMENT.md` 的"路径 A — 下载发布产物"调整为：把一键安装作为路径 A 的首选子步骤置于最前，手动下载解压流程降级为备选子步骤；一键安装步骤标注安全提示——Linux/macOS 侧"`curl | bash` 模式，谨慎用户可先 `curl -fsSL <url> -o install.sh` 下载脚本审阅后再执行"，Windows 侧"`irm | iex` 模式，谨慎用户可先 `irm <url> -OutFile install.ps1` 下载后审阅再以管理员运行"。文档中 raw-url 写死真实地址 `https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.{sh,ps1}`。
16. `README.md` 的"快速开始"章节（当前 L49 起）把一键安装命令置为首选示例，手动下载示例保留为备选。
17. 所有新增脚本注释、错误输出、文档内容、`--help` / `-Help` 文本均为中文。

---

## 3. Out-of-scope（本期明确不做）

| 不做项 | 理由 |
|---|---|
| `install.sh` / `install.ps1` 内置卸载功能 | 已有 `scripts/uninstall-service.sh` / `uninstall-service.ps1`；卸载入口不重复建设。安装脚本仅在输出中提示卸载命令路径。 |
| 自定义安装目录（作为硬 AC）/ 自定义版本下载 | 本期固定安装目录 `/opt/frp-easy`（Linux/macOS）、`C:\Program Files\frp-easy`（Windows）、固定下载 latest release（PM 裁决 Q2/Q3）。是否给高级用户提供环境变量/flag 覆盖安装目录由 Architect 酌情设计，不列为硬 AC；自定义版本下载留作后续任务。 |
| macOS 的服务化（launchd） | 与 T-008 决策一致，macOS 为次要平台；`install.sh` 在 macOS 上完成下载+解压，但因无 launchd 模板，服务化步骤在 macOS 上降级为"打印手动启动提示"而非失败。 |
| ARM / RISC-V 多架构支持 | 现有 release.yml 与 package.* 仅产出 amd64；`install.sh` 探测到非 amd64 架构时友好报错（见边界条件 BC-3），不尝试下载。 |
| 改写 `release.yml` / `install-service.*` / `package.*` 行为 | 本任务在它们之上加一层，复用其契约，不破坏既有逻辑。 |
| 校验下载包的 GPG / 数字签名 | 仓库当前发布流程无签名产物；本期仅校验下载文件非空、可解压；签名留作后续。 |
| 把一键安装接入 `verify_all` 做真实联网测试 | 真实 `curl` 下载依赖外网与已发布 release，不适合进 CI 闸门；`verify_all` 仅做脚本静态检查（见 AC）。 |
| 重写 frp 子二进制在线下载机制（T-002） | 不涉及。 |

---

## 4. 边界条件与失败场景

> 每条标注脚本必须的可观察行为。所有报错为中文、写 stderr、退出码非 0。

| 编号 | 场景 | 必须的行为 |
|---|---|---|
| **BC-1** | 无网络 / DNS 解析失败 / GitHub API 不可达 | 检测到 `curl` 拉取 API 失败，打印"无法访问 GitHub（请检查网络或代理）"，退出码非 0。 |
| **BC-2** | GitHub API 返回 403 限流（未认证 60 次/小时/IP） | 识别响应为限流（HTTP 403 + rate limit 提示），打印"GitHub API 触发限流，请稍后重试"，退出码非 0；不把限流响应当成正常 JSON 解析。 |
| **BC-3** | 当前主机架构非 amd64（如 arm64） | 探测到不受支持的架构，打印"当前架构 `<arch>` 暂无预编译发布包，请用源码构建（见 docs/DEPLOYMENT.md 路径 B）"，退出码非 0。 |
| **BC-4** | 仓库尚无任何已发布 release（API 返回 404 `Not Found`） | 识别 `latest` release 不存在，打印"仓库尚未发布任何 release，无法一键安装；请改用源码构建或等待首个 release"，退出码非 0。 |
| **BC-5** | latest release 存在但缺少当前平台资产（无匹配 `<os>-<arch>` 文件名） | 打印"最新 release 未包含当前平台的发布包"，退出码非 0。 |
| **BC-6** | `install.sh` 非 root 运行（写 `/opt` 与 systemd 需 root） | 检测 `id -u` 非 0，打印"请以 root / sudo 运行（安装到 /opt 与配置 systemd 需 root 权限）"，退出码非 0。`install.ps1` 同理检测非管理员会话并报错。 |
| **BC-7** | 缺 `tar`（Linux 解压依赖）/ 缺 `curl` | 启动前 `command -v` 检测，缺失则打印缺失工具名与安装提示，退出码非 0。 |
| **BC-8** | 缺 `systemctl`（WSL1 / OpenRC / 极简容器） | `install-service.sh` 已处理该分支；`install.sh` 完成下载解压后调用 `install-service.sh`，由后者报错；`install.sh` 须把该退出码透传，不掩盖。 |
| **BC-9** | 下载的发布包为空 / 损坏 / 解压失败 | 校验下载文件非 0 字节、解压命令退出码为 0；任一失败打印"发布包下载或解压失败"，退出码非 0，并清理已下载的临时文件。 |
| **BC-10** | 目标安装目录 `/opt/frp-easy` 已存在旧安装 | 按 In-scope 11 的"升级"语义处理；不删除 `frp_easy.toml` 与 `.frp_easy/`。 |
| **BC-11** | macOS 主机运行 `install.sh` | 下载+解压正常完成；服务化步骤因无 launchd 模板降级为打印手动启动提示（`cd /opt/frp-easy && ./frp-easy`），脚本以退出码 0 收尾。 |
| **BC-12** | 安装中途失败（部分文件已落地） | 失败时清理本次下载的临时压缩包；已部分解压的安装目录在升级场景下保持可被下次重跑覆盖（脚本幂等）。 |

---

## 5. 验收标准（AC）

> 验收方式分两类并明确标注：
> **【静态/自动】** 可在 `verify_all` 环境或本地无网络下机械验证；
> **【集成/人工】** 需真实联网或目标平台环境，由 QA 在 `06_TEST_REPORT.md` 的 `## Adversarial tests` 段记录人工/集成结果。

| ID | 描述 | 验证方式 |
|---|---|---|
| **AC-1** | `bash -n scripts/install.sh` 通过（无语法错误） | 【静态/自动】 |
| **AC-2** | `scripts/install.sh` 通过 `shellcheck`（与项目既有 .sh 同等级别，无 error 级告警） | 【静态/自动】 |
| **AC-3** | `pwsh -NoProfile -Command "$null = [ScriptBlock]::Create((Get-Content -Raw scripts/install.ps1))"` 解析通过（无语法错误） | 【静态/自动】 |
| **AC-4** | `scripts/install.sh` 含 `set -euo pipefail`；`scripts/install.ps1` 含 `$ErrorActionPreference = "Stop"` | 【静态/自动】grep |
| **AC-5** | `scripts/install.sh -h` 与 `scripts/install.ps1 -Help` 输出中文用法说明（含安装目录、所需权限、退出码语义），退出码 0 | 【静态/自动】非联网路径，help 分支在依赖检测之前 |
| **AC-6** | 每个 BC-1…BC-12 失败分支在脚本源码中可定位到对应的中文 stderr 报错与非 0 退出（BC-11 为退出码 0），且报错文案与第 4 节描述语义一致 | 【静态/自动】Code Review 逐条走查 + grep |
| **AC-7** | `install.sh` 不复刻 systemd unit 写入逻辑，而是 `exec`/调用解压目录内 `scripts/install-service.sh`；`install.ps1` 同理调用 `install-service.ps1` | 【静态/自动】源码走查 |
| **AC-8** | `install.sh` 对 GitHub API 的调用 URL 精确为 `https://api.github.com/repos/Alan-IFT/frp_easy/releases/latest` | 【静态/自动】grep |
| **AC-9** | 升级语义（BC-10）：脚本逻辑在目标目录已存在时不删除 `frp_easy.toml` 与 `.frp_easy/` | 【静态/自动】源码走查 |
| **AC-10** | 在一台联网的标准 Linux（Ubuntu 22.04+，已发布 release 的前提下）执行 `curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh \| sudo bash`，安装完成后 `systemctl is-active frp-easy` 输出 `active`，浏览器可打开打印的 UI 地址 | 【集成/人工 — 交付后验证，非完成闸门】见 8.2-Q1：本任务不打 tag/发 release，此 AC 由 PM 在 `07_DELIVERY.md` 写明为"需先发布首个 GitHub Release 才能实测"的交付后人工步骤，交还用户执行。 |
| **AC-11** | 在一台联网的 Windows 10 22H2+ 执行 `irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 \| iex`（管理员），安装完成后 `sc query frp-easy` 显示 `RUNNING` | 【集成/人工 — 交付后验证，非完成闸门】同 AC-10，依赖首个 release 已发布。 |
| **AC-12** | 仓库根存在 `LICENSE`，首行/版权行含 `Copyright (c) 2026 Alan_IFT`，正文为 MIT 标准全文 | 【静态/自动】文本比对 |
| **AC-13** | 仓库根存在 `NOTICE`，含对 `fatedier/frp` 与 Apache-2.0 的中文说明 | 【静态/自动】grep |
| **AC-14** | `README.md` 许可证章节不再出现"尚未确定" / "待项目维护者确定"字样，改为 MIT 声明 | 【静态/自动】grep |
| **AC-15** | `README.md` 快速开始章节首选示例为一键安装命令；`docs/DEPLOYMENT.md` 路径 A 首选子步骤为一键安装，含 `curl \| bash`（Linux/macOS）与 `irm \| iex`（Windows）安全提示（建议先下载审阅）；命令中 raw-url 为写死的真实地址 `https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.{sh,ps1}` | 【静态/自动】目视 + grep |
| **AC-16** | 引入本任务全部改动后 `scripts/verify_all` 输出 PASS 数不低于当前基线（当前 19），WARN/FAIL 不新增 | 【静态/自动】执行 `scripts/verify_all` |
| **AC-17** | `install.sh` / `install.ps1` 顶部含 ≥5 行中文注释（用途、用法、参数、输出、退出码），与 T-008 既有脚本风格一致 | 【静态/自动】目视 |

---

## 6. 非功能性需求（NFR）

- **NFR-1 跨平台** — `install.sh` 在 Ubuntu 22.04+/Debian 12 默认 bash 下不依赖 GNU 扩展；如必须依赖某工具（`curl`、`tar`），脚本顶部 `command -v` 检测并给中文安装提示。`install.ps1` 在 Windows 10 22H2+ 自带 PowerShell 5.1 与 PowerShell 7 上均可运行。
- **NFR-2 安全** — 文档必须提示 `curl | bash` 模式的固有风险：脚本以 root 执行，谨慎用户应先 `curl -fsSL <url> -o install.sh` 下载审阅再运行。脚本本身不写入任何明文凭据。GitHub API 调用为未认证匿名请求（限流 60 次/小时/IP），脚本对限流响应须友好降级（BC-2），本期不引入 token 认证。
- **NFR-3 可观察性** — 安装脚本每个阶段（探测平台 / 查询 release / 下载 / 解压 / 配置服务）向 stdout 打印中文进度行，便于用户与 issue 排障判断卡在哪一步。
- **NFR-4 幂等** — 安装脚本重复执行不报错、不残留旧临时文件、不产生重复服务（依赖 `install-service.*` 的既有幂等性）。
- **NFR-5 语言** — 全部产出（脚本注释、报错、help、文档、`LICENSE` 除外——MIT 全文为英文标准文本，`NOTICE` 为中文）使用中文。

---

## 7. 相关任务

| 任务 | 关系 | 关键文件 |
|---|---|---|
| **T-008 `deploy-kit`** | 本任务直接前序。建立了 `scripts/install-service.{sh,ps1}`、`uninstall-service.{sh,ps1}`、`package.{sh,ps1}`、`docs/DEPLOYMENT.md` 三路径结构。本任务复用其 `install-service.*` 且在 DEPLOYMENT.md 路径 A 之上加一键安装。需求文档：`docs/features/_archived/deploy-kit/01_REQUIREMENT_ANALYSIS.md`（其 Open Question 7 已预见"后续补 LICENSE"）。 |
| **T-010 `deploy-polish-and-ci`** | 建立 `.github/workflows/release.yml`，push `v*` tag 自动构建并上传 `*.tar.gz`/`*.zip` 到 GitHub Releases，release 名为去 `v` 前缀的版本号。本任务的 `install.sh` 依赖此发布产物命名约定与 release 命名约定。 |
| **T-002 `zero-config-quickstart`** | frp 子二进制 GitHub Releases 自动下载机制；本任务不改动它。 |
| **T-011 `readme-refresh-and-network-defaults`** | 上一次 README 重排（默认端口 7800、网络默认值）；本任务再次小幅改 README（快速开始、许可证两节），需与其结果对齐不冲突。 |

相关 insight（`.harness/insight-index.md`）：
- 2026-05-19 · `sudo` 下 `id -un` 返回 root，拿真实调用者用 `${SUDO_USER:-$(id -un)}` —— `install.sh` 若需展示"以谁身份运行"沿用此模式。
- 2026-05-19 · systemd unit `ExecStart`/`WorkingDirectory` 路径含空格需双引号 —— 安装目录固定为无空格的 `/opt/frp-easy`，风险低，但仍由 `install-service.sh` 既有逻辑负责。
- 2026-05-19 · Windows PowerShell 写文件编码陷阱（BOM / codepage）—— `install.ps1` 若生成任何文件须注意编码（实际生成动作交由 `install-service.ps1`）。
- verify_all E.6 要求 `06_TEST_REPORT.md` 含精确英文标题 `## Adversarial tests`。

---

## 8. 风险与给 PM/用户的开放问题

### 8.1 风险（已识别，缓解写入 in-scope/边界，无需用户裁决）

| 风险 | 影响 | 缓解 |
|---|---|---|
| `curl \| bash` 以 root 执行任意远程脚本，是公认的安全敏感模式 | 用户对供应链攻击有顾虑 | NFR-2 / In-scope 15：文档明确提示风险并给"先下载审阅"的替代命令。 |
| GitHub API 未认证匿名请求限流 60 次/小时/IP | 同一出口 IP 多人安装时触发 403 | BC-2 友好报错；本期不引入 token。 |
| **仓库当前无任何已发布 release**（PM 已核查：仓库无任何 git tag） | `install.sh` 取 `latest` 会得到 404，一键安装当下无法真实跑通 | BC-4 友好报错；AC-10/AC-11 已降级为"交付后人工验证、非完成闸门"。本任务**不打 tag/不发 release**（对外发版属用户决策）；PM 在 `07_DELIVERY.md` 注明需先发首个 release 才能实测。详见 8.2-Q1 裁决。 |
| raw-url 的具体形态依赖默认分支名 | 文档里写死的安装命令在分支改名后失效 | org `Alan-IFT`、默认分支 `main` 均已确定（git status 显示 `main`）；文档写死 `https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh`（及 `.ps1`），保证"复制即用"。分支若改名属对外破坏性变更，由维护者届时同步文档。 |

### 8.2 开放问题（PM 已全部裁决 — 2026-05-22）

> RA 提出的 5 个开放问题，PM 已逐条裁决如下。无剩余开放问题。

1. **【Q1 — 阻塞项】仓库当前是否已有已发布 release？若没有，本任务的"集成验收"如何收尾？**
   候选见原稿 (a)/(b)/(c)。
   **`PM-resolved` → 方案 (b)。** PM 已核查：仓库无任何 git tag，几乎确定无已发布 release。本任务的"完成"以静态/自动 AC（AC-1…AC-9、AC-12…AC-17）为准；`install.sh` / `install.ps1` 在取不到 latest release 时给出友好中文报错（已是 BC-4）。AC-10/AC-11 的真实 `curl | bash` / `irm | iex` 联网安装**降级为"交付后人工验证步骤"，不作为任务完成闸门**——由 PM 在 `07_DELIVERY.md` 写明"需先发布首个 GitHub Release 才能实测一键安装"，交还用户。**本任务不自动打 tag、不发 release**（对外发布软件版本属用户决策的对外动作，不由本流水线代行）。AC-10/AC-11 已据此重标注为"交付后验证，非完成闸门"。

2. **【Q2】安装目录是否需要支持用户自定义？**
   候选见原稿 (a)/(b)/(c)。
   **`PM-resolved` → 方案 (a)：不支持。** 一键安装的价值在于零选择。是否允许通过环境变量 / flag 给高级用户覆盖安装目录，属可选项、非必需，由 Architect 在设计阶段酌情决定，**不列为硬 AC**。固定安装目录见 Q3。

3. **【Q3】Windows 一键安装的固定安装目录用哪个？**
   候选见原稿 (a)/(b)/(c)。
   **`PM-resolved` → 方案 (a)：`C:\Program Files\frp-easy`。** 理由：与 `docs/DEPLOYMENT.md` 已有的 `<INSTALL_DIR>` 示例一致。Linux/macOS 侧维持 `/opt/frp-easy`。

4. **【Q4】Windows 一键安装命令在文档中以何种形态给出？**
   候选见原稿 (a)/(b)/(c)。
   **`PM-resolved` → 方案 (a)：`irm <url> | iex` 作为主形态**（与 `curl | bash` 对称的 PowerShell 惯用法）。具体命令为 `irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex`。In-scope 15 / NFR-2 的"先下载审阅"安全提示在 Windows 侧对应给出 `irm <url> -OutFile install.ps1` 的谨慎备选。

5. **【Q5】`README.md` / `DEPLOYMENT.md` 中一键安装命令里的 raw-url 写死还是占位符？**
   候选见原稿 (a)/(b)。
   **`PM-resolved` → 方案 (a)：写死。** 写死 `https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh`（及 `.ps1` 对应）。org `Alan-IFT`、分支 `main` 均已确定，写死保证"复制即用"。

---

## 9. Verdict

**READY FOR DESIGN** — RA 提出的 5 个开放问题已由 PM 于 2026-05-22 全部裁决（见 8.2，逐条标注 `PM-resolved`），无剩余阻塞项。本文档全部 17 条 In-scope 行为、12 条边界条件、17 条 AC 均已定稿。可推进至 Stage 2（Solution Architect）。

> 下游说明：
> - 第 2 节 In-scope、第 3 节 Out-of-scope、第 4 节边界条件、第 5 节 AC 均不再依赖开放问题。
> - Architect 不得重新质疑 PM 在 `INPUT.md` 中的 4 项预决策、以及 8.2 中 Q1…Q5 的裁决。
> - 本任务**不打 git tag、不发 GitHub Release**；AC-10/AC-11 为"交付后人工验证、非完成闸门"，PM 在 `07_DELIVERY.md` 注明实测前提。任务完成闸门为静态/自动 AC（AC-1…AC-9、AC-12…AC-17）。
> - 安装目录已定：Linux/macOS = `/opt/frp-easy`，Windows = `C:\Program Files\frp-easy`。
> - 一键安装命令 raw-url 写死为 `https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.{sh,ps1}`；Windows 主形态为 `irm <url> | iex`。
