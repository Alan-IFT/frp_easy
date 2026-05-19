# 02 — 解决方案设计：T-008 deploy-kit

> Stage 2 of 7-stage `/harness` 流水线。
> 任务 ID：**T-008** · Slug：`deploy-kit` · 模式：full · 语言：中文
> 唯一上游输入：`docs/features/deploy-kit/01_REQUIREMENT_ANALYSIS.md`（verdict READY）
> 本文档是下游 Gate Reviewer 与 Developer 的唯一设计入口；Developer 看完即可动手，不应再做设计决策。

---

## 1. 架构总览

本任务**不引入任何 Go 依赖、不引入任何新服务**。它在仓库现有 `build → bin/` 链路之外再叠加一层「打包 → bin/release/」、并在仓库根 `scripts/` 下新增「服务安装 / 卸载」薄包装，全部用各平台原生 shell（bash + PowerShell）实现。`cmd/frp-easy/main.go` 仅在启动序列**最早期**插入标准库 `flag` 解析，提供 `--version` / `--help`，不动既有启动流程的任何一步。

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                              开发者主机（构建侧）                              │
│                                                                              │
│   scripts/build.sh / build.ps1    ← 既有；本任务不修改                       │
│           │                                                                  │
│           ▼                                                                  │
│   bin/frp-easy / bin/frp-easy.exe ← 既有产物                                 │
│           │                                                                  │
│           │   ┌──────────────────────────────────────────────┐               │
│           └──►│ scripts/package.sh  (本任务新增；wrap build) │               │
│               │ scripts/package.ps1 (本任务新增；wrap build) │               │
│               └────────────────┬─────────────────────────────┘               │
│                                ▼                                             │
│             bin/release/frp-easy-<ver>-linux-amd64.tar.gz                    │
│             bin/release/frp-easy-<ver>-windows-amd64.zip                     │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
                                  │
                                  │ 分发（GitHub Releases / 任意渠道）
                                  ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                            终端用户主机（部署侧）                              │
│                                                                              │
│   tar xzf … / Expand-Archive …                                               │
│   解压至 ./frp-easy-<ver>-<os>-<arch>/                                       │
│      ├── frp-easy(.exe)                                                      │
│      ├── frp_linux/  或  frp_win/    （仅当前 OS 一份）                      │
│      ├── frp_easy.toml.example                                               │
│      ├── README.txt                                                          │
│      ├── VERSION                                                             │
│      ├── LICENSE (若仓库根有)                                                │
│      └── scripts/                                                            │
│           ├── install-service.sh   (Linux only;  本任务新增)                 │
│           ├── uninstall-service.sh (Linux only;  本任务新增)                 │
│           ├── install-service.ps1  (Windows only; 本任务新增)                │
│           └── uninstall-service.ps1(Windows only; 本任务新增)                │
│                                                                              │
│   ┌── 路径 A：./frp-easy 直接前台运行 ──────────────────────────┐            │
│   │                                                            │            │
│   ├── 路径 C：sudo ./scripts/install-service.sh                 │            │
│   │   ──► 生成 /etc/systemd/system/frp-easy.service (0o644)    │            │
│   │   ──► systemctl daemon-reload && enable --now             │            │
│   │   （注意：unit 文件不在解压目录内，**在系统目录**）         │            │
│   │                                                            │            │
│   └── 路径 C-Windows：管理员 PowerShell install-service.ps1     │            │
│       ──► sc.exe create frp-easy binPath=...                   │            │
│       ──► sc.exe failure frp-easy reset=60 actions=restart/5000│            │
│       ──► sc.exe start frp-easy                               │            │
│                                                                │            │
└──────────────────────────────────────────────────────────────────────────────┘
```

要点：
- **解压目录** = 用户工作目录，包含二进制 + 样例 + 服务脚本；用户可任意放置（推荐 `/opt/frp-easy` 或 `C:\Program Files\frp-easy`），但 install-service 会用 `realpath` / `Resolve-Path` 把绝对路径写进 unit / service binPath，**安装服务后不要移动目录**。
- **systemd unit 文件**最终落在 `/etc/systemd/system/frp-easy.service`（系统目录，0o644），**不在**解压目录内；这是 uninstall 找它的唯一权威路径。Windows Service 注册项落在注册表 `HKLM\SYSTEM\CurrentControlSet\Services\frp-easy`，由 `sc.exe` 管理。
- **数据目录** `frp_easy.toml` + `.frp_easy/` 始终留在用户工作目录（运行 frp-easy 时的 cwd），与解压目录、unit 文件互不重叠。卸载脚本只删 unit / service 注册项，**不**碰数据目录。

---

## 2. 模块分解（每个文件的责任 + 完整路径）

| # | 路径（绝对） | 状态 | 责任 |
|---|---|---|---|
| 1 | `C:\Programs\frp_easy\cmd\frp-easy\main.go` | **edit** | 在 `run()` 顶部插入 `flag.NewFlagSet` 解析；新增 `--version` / `-v` / `--help` / `-h` 处理；其余既有启动序列字节级保留 |
| 2 | `C:\Programs\frp_easy\scripts\package.sh` | **new** | Linux/macOS 打包脚本：调 `build.sh` → 组装 staging 目录 → `tar czf` → 写入 `bin/release/` |
| 3 | `C:\Programs\frp_easy\scripts\package.ps1` | **new** | Windows 打包脚本：调 `build.ps1` → 组装 staging 目录 → `Compress-Archive` → 写入 `bin\release\` |
| 4 | `C:\Programs\frp_easy\scripts\install-service.sh` | **new** | Linux：生成 `/etc/systemd/system/frp-easy.service` + daemon-reload + enable --now；支持 `--user <name>` |
| 5 | `C:\Programs\frp_easy\scripts\uninstall-service.sh` | **new** | Linux：disable --now + 删 unit + daemon-reload；保留数据目录 |
| 6 | `C:\Programs\frp_easy\scripts\install-service.ps1` | **new** | Windows：管理员校验 → `sc.exe create/config/failure/start frp-easy`；支持 `-DisplayName` |
| 7 | `C:\Programs\frp_easy\scripts\uninstall-service.ps1` | **new** | Windows：`sc.exe stop` + `sc.exe delete`；保留数据目录 |
| 8 | `C:\Programs\frp_easy\docs\DEPLOYMENT.md` | **new** | 部署文档主体，三条路径（A 二进制 / B 源码 / C 系统服务）+ 决策表 + 故障排查 |
| 9 | `C:\Programs\frp_easy\README.md` | **edit** | 重排：顶部新增"快速开始 / 部署"指针；"更新流程"折叠为一行链接；"开发模式"下沉 |
| 10 | `C:\Programs\frp_easy\docs\dev-map.md` | **edit** | 在 `## 目录布局` 的 `scripts/` 行后追加 `package.{sh,ps1}` 与 `install-service.*` 索引；新增 `docs/DEPLOYMENT.md` 索引 |

不新增 Go 文件、不新增 web 文件、不新增 migration、不新增 systemd unit 模板文件（unit 在 install-service.sh 内联生成，避免引入第二个真相源）。

---

## 3. 每个脚本的接口契约

### 3.1 `scripts/package.sh`

**Shebang + 安全选项**：`#!/usr/bin/env bash` + `set -euo pipefail`。

**参数**（POSIX 长参数风格）：

| 参数 | 默认 | 说明 |
|---|---|---|
| `--linux` | 启用 | 打 `linux-amd64` 包 |
| `--windows` | 关闭 | 同时打 `windows-amd64` zip（需先存在 `bin/frp-easy.exe`；若不存在自动调 `build.sh --all`） |
| `--version <s>` | `git describe --tags --always --dirty` | 覆盖版本号；用于本地无 git 上下文时手工指定 |
| `--skip-build` | 关闭 | 若用户刚跑过 `build.sh`，跳过重复构建 |
| `-h` / `--help` | — | 打印用法（中文）后退出 0 |

**退出码**：
- `0`：成功；`bin/release/<...>.tar.gz`（或 .zip）已就绪
- `1`：前置缺失（`bin/frp-easy` 不存在且 `--skip-build` 仍指定 / `frp_linux/frpc` 缺失等）
- `2`：构建失败（透传 build.sh 退出码或包装为 2）

**stdout**：每步一行中文进度（`==> 构建 ...` / `==> 组装 staging ...` / `==> 打包 ...` / `==> 完成：<绝对路径>`）。
**stderr**：仅错误与 WARN（如包体 > 25 MB）。

**外部命令依赖与降级**：

| 命令 | 用于 | 缺失降级 |
|---|---|---|
| `bash` 4.0+ | 脚本宿主 | 必需，无降级 |
| `git` | `git describe` 取版本 | 缺失或仓库无 tag → 用 `dev` 字符串；不报错 |
| `tar` (gnu/bsd) | 压缩 tar.gz | 必需；macOS 自带 bsdtar 兼容 |
| `gzip` | tar 内联调用 | 一般随 tar 提供，不另行检测 |
| `realpath` | 解析仓库根 | 不可用时退化为 `cd "$(dirname "$0")/.." && pwd`（已是 build.sh 的做法） |
| `du` / `stat` | 包体超阈值 WARN | 缺失 → 跳过 WARN，仍 PASS |
| `cp` / `rm` / `mkdir` | 文件操作 | 必需 |
| `find` | 清理 staging | 必需 |

### 3.2 `scripts/package.ps1`

**参数（PowerShell CmdletBinding）**：

```powershell
[CmdletBinding()]
param(
  [switch]$Linux,        # 同时打 linux-amd64 tar.gz（需 tar 可执行）
  [switch]$Windows = $true,  # 默认开
  [string]$Version,      # 覆盖 git describe
  [switch]$SkipBuild
)
$ErrorActionPreference = "Stop"
```

**退出码**：与 .sh 一致（0 / 1 / 2）。

**外部命令依赖**：

| 命令 | 用于 | 缺失降级 |
|---|---|---|
| `pwsh` 5.1+ 或 7+ | 宿主 | 必需 |
| `git.exe` | 版本号 | 缺失或无 tag → `dev` |
| `Compress-Archive` | zip 打包 | PowerShell 5.0+ 内建，必需 |
| `tar.exe`（仅 `-Linux` 时） | Windows 10 22H2 自带 bsdtar，可制作 tar.gz | 缺失则报错并提示用户单平台运行（即 `-Linux` 跳过） |

### 3.3 `scripts/install-service.sh`

**参数**：

| 参数 | 默认 | 说明 |
|---|---|---|
| `--user <name>` | `$(id -un)`（当前运行者） | systemd `User=` 字段；脚本会 `getent passwd "$name"` 校验存在 |
| `--name <name>` | `frp-easy` | unit 名（含 `.service` 后缀自动补），允许重命名以并行跑多实例（高级用法，文档不主推） |
| `-h` / `--help` | — | 打印中文用法后退出 0 |

**前置检查**：
1. 当前 EUID == 0（即 `sudo` 或 root 直跑），否则报错"请以 root / sudo 运行"，退出 1。
2. 存在 `systemctl` 命令；不存在则报错"未检测到 systemd（如 WSL1 默认无），无法安装为系统服务，请改用前台运行"，退出 1。
3. 存在 `realpath`（`command -v realpath`）；不存在则用 `cd "$(dirname "$0")/.." && pwd` 作为 fallback。
4. 同级二进制 `frp-easy` 可执行且非空（`[[ -x ../frp-easy && -s ../frp-easy ]]`）；缺失退出 1。
5. 用户传 `--user X` 且 `getent passwd X` 为空 → 退出 1，报中文"用户 X 不存在"。

**Unit 文件原子写**（强制采用，参考 `.harness/insight-index.md` 2026-05-19 AtomicWrite 经验）：
1. 写到 `/etc/systemd/system/.frp-easy.service.tmp`，`chmod 0644`；
2. `mv -f` 到 `/etc/systemd/system/frp-easy.service`（Linux 上覆盖式 rename 是 POSIX 原子操作；insight-index 2026-05-16 那条只针对 Windows，Linux 不踩坑）；
3. **再次** `chmod 0644 /etc/systemd/system/frp-easy.service`（双重 chmod 模式，防 umask 让最终权限变宽）。

**幂等处理**：
- 如果 `frp-easy.service` 已存在，先 `systemctl stop frp-easy || true`，覆盖写新 unit，再 `daemon-reload + enable --now`。
- 输出 `==> 已刷新现有 unit` 而非 `==> 新建 unit`。

**退出码**：`0` 成功 / `1` 前置失败 / `2` systemctl 调用失败。

### 3.4 `scripts/uninstall-service.sh`

**参数**：仅 `--name <name>`（默认 `frp-easy`）+ `-h`。

**行为**：
1. 不存在 `/etc/systemd/system/frp-easy.service` → 打印中文"未检测到已安装的 frp-easy 服务"，**退出 0**（友好降级，NFR-3.2）。
2. 存在 → `systemctl disable --now frp-easy || true` → `rm -f /etc/systemd/system/frp-easy.service` → `systemctl daemon-reload`。
3. 结尾打印中文提示：`数据目录 (./.frp_easy/) 与配置 (./frp_easy.toml) 未删除。如需彻底清理，请手动执行：rm -rf ./.frp_easy ./frp_easy.toml`。

**退出码**：始终 0（友好降级），除非 `rm` 因权限不足真正失败 → 1。

### 3.5 `scripts/install-service.ps1`

**参数**：

```powershell
[CmdletBinding()]
param(
  [string]$DisplayName = "FRP Easy",
  [string]$ServiceName = "frp-easy"
)
$ErrorActionPreference = "Stop"
```

**前置检查（按序）**：
1. **管理员权限检测**（FR-2.2 硬约束）：
   ```powershell
   $isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
   if (-not $isAdmin) {
     Write-Error "请以管理员身份运行此脚本（右键 PowerShell → 以管理员身份运行）"
     exit 1
   }
   ```
2. `sc.exe` 存在（`Get-Command sc.exe`），缺失则报错退出 1（Windows 自带，仅极端裁剪 SKU 缺失）。
3. 同级目录有 `frp-easy.exe` 且 `Test-Path -PathType Leaf`。

**幂等处理**：
- `sc.exe query $ServiceName` 退出码 0 表示已存在 → 先 `sc.exe stop` → `sc.exe config $ServiceName binPath= "<新绝对路径>" start= auto DisplayName= "$DisplayName"` → `sc.exe failure ...` → `sc.exe start`。
- 不存在 → `sc.exe create $ServiceName binPath= "<绝对路径>\frp-easy.exe" start= auto DisplayName= "$DisplayName"` → `sc.exe failure $ServiceName reset= 60 actions= restart/5000` → `sc.exe start $ServiceName`。

> `sc.exe` 的等号语法字面坑：**等号后必须有空格**（`binPath= "..."` 而非 `binPath="..."`），脚本需严格遵守。

**退出码**：`0` 成功 / `1` 前置失败 / `2` `sc.exe` 调用失败（透传 `$LASTEXITCODE`）。

### 3.6 `scripts/uninstall-service.ps1`

**参数**：`-ServiceName "frp-easy"` 默认。

**行为**：
1. 管理员权限检测（同 3.5）。
2. `sc.exe query $ServiceName` 退出码非 0 → 打印中文"未检测到已安装的 frp-easy 服务"，**退出 0**。
3. 否则 `sc.exe stop $ServiceName | Out-Null`（忽略非运行错误）→ `sc.exe delete $ServiceName`。
4. 结尾打印中文数据目录保留提示。

**退出码**：始终 0（友好降级），除非 `sc.exe delete` 真正失败 → 1。

---

## 4. 打包目录布局

解压后顶层目录名 = 压缩包文件名去后缀 = `frp-easy-<version>-<os>-<arch>/`。

**Linux 包**（`frp-easy-<ver>-linux-amd64.tar.gz`）：

```
frp-easy-<ver>-linux-amd64/
├── frp-easy                       ← 来源：仓库根 bin/frp-easy（build.sh 产物，CGO_ENABLED=0）；权限 0755
├── frp_linux/                     ← 来源：仓库根 frp_linux/ 全量复制
│   ├── frpc                       ←   权限 0755
│   ├── frps                       ←   权限 0755
│   ├── frpc.toml                  ←   样例（FRP 上游自带，仅供参考）
│   ├── frps.toml                  ←   样例
│   └── LICENSE                    ←   FRP 上游 license
├── frp_easy.toml.example          ← 来源：脚本内联生成（基于 README §配置说明 默认值；故意不取名 frp_easy.toml 避免覆盖用户配置）
├── README.txt                     ← 来源：脚本内联生成（80 行内，含三步指令 + 安装服务可选指引）
├── VERSION                        ← 来源：脚本内联生成；内容单行 = <version>
├── LICENSE                        ← 来源：仓库根 LICENSE（**当前仓库根缺失**，WARN 并跳过；不阻断打包）
└── scripts/
    ├── install-service.sh         ← 来源：仓库根 scripts/install-service.sh 复制
    └── uninstall-service.sh       ← 来源：仓库根 scripts/uninstall-service.sh 复制
```

**Windows 包**（`frp-easy-<ver>-windows-amd64.zip`）：

```
frp-easy-<ver>-windows-amd64/
├── frp-easy.exe                   ← 来源：bin/frp-easy.exe
├── frp_win/                       ← 来源：frp_win/ 全量复制
│   ├── frpc.exe
│   ├── frps.exe
│   ├── frpc.toml
│   ├── frps.toml
│   └── LICENSE
├── frp_easy.toml.example          ← 内联生成
├── README.txt                     ← 内联生成（Windows 版指令）
├── VERSION                        ← 内联生成
├── LICENSE                        ← 仓库根（若有；当前缺失 → WARN）
└── scripts/
    ├── install-service.ps1
    └── uninstall-service.ps1
```

**FR-1.4 校验点**：打包前 staging 完成后做一次 `find staging -type f | wc -l` ≥ 6 健全性检查；以及 `[[ -x staging/frp-easy ]]` / `[[ -s staging/frp_linux/frpc ]]` / `[[ -s staging/frp_linux/frps ]]` 三条断言。

**FR-1.5（打包前前置校验）**：在调用 `build.sh` 前若未指定 `--skip-build`，强制刷新；指定 `--skip-build` 时必须断言 `bin/frp-easy` 存在、非空；缺失立即非 0 退出。

**FR-1.6（幂等）**：staging 目录每次打包前 `rm -rf` 清空；旧 tar.gz 直接覆盖（tar 默认覆盖；PowerShell 用 `Compress-Archive -Force`）。

---

## 5. 字段表

### 5.1 systemd unit (`/etc/systemd/system/frp-easy.service`)

```ini
[Unit]
Description=FRP Easy — frp 可视化管理 UI
After=network.target
Documentation=https://github.com/<org>/frp_easy

[Service]
Type=simple
ExecStart=<INSTALL_ABS>/frp-easy
WorkingDirectory=<INSTALL_ABS>
User=<RUN_USER>
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

| 字段 | 值来源 | 备注 |
|---|---|---|
| `Description` | 字面量中文 | 与 NFR-6.2 一致；systemd UTF-8 OK |
| `After` | `network.target` | UI 需绑定端口；不强依赖 DNS |
| `Documentation` | 占位符 `<org>`，与 DEPLOYMENT.md 占位一致 | Open Question 9 PM-resolved |
| `ExecStart` | `realpath ../frp-easy` 解析得到 | 锁定绝对路径，移动目录失效（风险已在 README.txt 警示） |
| `WorkingDirectory` | 同上 | 让 `frp_easy.toml` 与 `.frp_easy/` 相对路径生效（appconf 默认 `./` 起算） |
| `User` | `--user` 参数 或 `id -un` | Open Question 4 PM-resolved (b)；getent 校验 |
| `Restart` | `on-failure` | 与 Open Question 8 Windows `sc.exe failure` 对齐 |
| `RestartSec` | `5` | 防快速崩溃循环灌满 journal |
| `StandardOutput/Error` | `journal` | `journalctl -u frp-easy` 直读，与 DEPLOYMENT.md 故障排查一致 |
| `WantedBy` | `multi-user.target` | 标准做法 |

**unit 文件权限**：`0644`（NFR-7.2 显式要求；systemd 不接受 0600 unit）。

### 5.2 Windows Service (`sc.exe create frp-easy ...`)

| 字段 | 命令片段 | 来源 |
|---|---|---|
| 服务键名 | `sc.exe create frp-easy` | `-ServiceName` 参数（默认 `frp-easy`） |
| DisplayName | `DisplayName= "FRP Easy"` | `-DisplayName` 参数；FR-2.5 中文示例 `"FRP 测试"` 验证 |
| binPath | `binPath= "<INSTALL_ABS>\frp-easy.exe"` | `(Resolve-Path "$PSScriptRoot\..\frp-easy.exe").Path` |
| 启动类型 | `start= auto` | 开机自启 |
| 失败动作 | `sc.exe failure frp-easy reset= 60 actions= restart/5000` | Open Question 8 PM-resolved (b)：60 秒计数窗口，崩溃后 5 秒重启 |
| 描述 | `sc.exe description frp-easy "FRP 可视化管理 UI"` | NFR-6.2 中文 |

注意：`sc.exe` 不直接提供 WorkingDirectory；frp-easy 启动时 cwd 默认为服务运行账户的目录（一般为 `C:\Windows\System32`）。因此 Windows 路径下 frp-easy 读取 `frp_easy.toml` 必须**通过环境变量 `FRP_EASY_CONFIG`** 指定绝对路径，或者运维在 `install-service.ps1` 安装时通过 `sc.exe config frp-easy binPath= "<abs>\frp-easy.exe --config <abs>\frp_easy.toml"` 传 flag。

> **本期决策**：install-service.ps1 在 binPath 上**只写二进制路径**，让用户在解压目录运行；如果用户希望 cwd 锁定为解压目录（这是大多数运维需求），脚本会在 `sc.exe create` 前 `Set-Content` 一份小 `start-frp-easy.cmd` 包装（`cd /d "<INSTALL_ABS>" && "%~dp0frp-easy.exe"`），binPath 指向该 .cmd。这是 Windows Service 锁 cwd 的标准做法，避免引入新的 Go flag。该包装脚本路径：`<INSTALL_ABS>\frp-easy-svc.cmd`（uninstall 一并删除）。

---

## 6. `cmd/frp-easy/main.go` flag 解析改动

### 6.1 改动范围

| 位置 | 当前内容 | 改动 |
|---|---|---|
| Line 48 之后、Line 50 `func main()` 之前 | `var Version = "0.1.0"` | 在其后插入一个 const block 与 `usageText` 常量 |
| Line 57 `func run() error {` 之后第一行（即 Line 58 注释 `// 1. appconf` 之前） | `cfgPath := envOr(...)` | 插入 flag 解析块（约 30 行） |
| 文件 import 区（Line 19–45） | 当前已 import `flag` 否？**否**（未 import） | 新增 `"flag"` 到 import 区，按字母顺序插在 `"errors"` 之后、`"fmt"` 之前 |

### 6.2 新增常量

```go
// 紧跟 Line 48 var Version = "0.1.0" 之下
const usageText = `用法: frp-easy [选项]

frp-easy 是 FRP 可视化管理 UI 的单二进制服务进程。

选项:
  -h, --help       显示本帮助并退出
  -v, --version    显示版本号并退出

配置:
  配置文件         frp_easy.toml（与本程序同目录；可通过环境变量 FRP_EASY_CONFIG 覆盖路径）
  UI 默认地址      http://127.0.0.1:8080
  数据目录默认     ./.frp_easy

退出码:
  0   正常退出
  1   一般错误（启动失败、配置错误等）
  2   端口被占用 / 未知 flag

更多文档：docs/DEPLOYMENT.md
`
```

### 6.3 新增 flag 解析块（插在 `run()` 第一行）

伪代码（Developer 据此实现，签名按 stdlib `flag`）：

```go
// 0. flag 解析（必须早于 appconf.Load；--version / --help 不依赖任何运行时状态）
fs := flag.NewFlagSet("frp-easy", flag.ContinueOnError)
fs.SetOutput(io.Discard) // 自定义 usage 输出
var (
    showVersion bool
    showHelp    bool
)
fs.BoolVar(&showVersion, "version", false, "")
fs.BoolVar(&showVersion, "v", false, "")
fs.BoolVar(&showHelp, "help", false, "")
fs.BoolVar(&showHelp, "h", false, "")
if err := fs.Parse(os.Args[1:]); err != nil {
    // 未知 flag：中文错误 + 提示 --help，退出码 2（与既有"端口占用退出 2"语义一致，FR-3.4）
    fmt.Fprintf(os.Stderr, "frp-easy: 未识别的参数。运行 'frp-easy --help' 查看用法。\n")
    os.Exit(2)
}
if showHelp {
    fmt.Fprint(os.Stdout, usageText)
    return nil // 走正常 run 返回，main() 退出码 0
}
if showVersion {
    fmt.Fprintf(os.Stdout, "frp-easy %s\n", Version)
    return nil
}
// 以下保持既有 cfgPath := envOr(...) 起的全部代码不变
```

**关键约束**：
- 不使用 `flag.CommandLine`，避免污染 Go 标准库默认 FlagSet（防其它包注入未知 flag 影响 Parse）。
- 用 `ContinueOnError` 而非 `ExitOnError`，错误处理收归脚本统一中文化。
- `--version` / `--help` 都走 `return nil` 让 `main()` 自然退出码 0（既有 `main()` Line 51–55 在 `run()` 返回 nil 时不调用 os.Exit，进程正常 0 退出）。
- 不调用 `appconf.Load`、不打开 SQLite、不创建日志文件——满足 AC-13（无 `frp_easy.toml` 时仍能跑 `--version`）。
- **不引入新 Go 依赖**，标准库 `flag` 已足够（Open Question 5 PM-resolved (a)）。

---

## 7. README 重排前后对照表

> 按 FR-5 各子条逐条映射。README 当前 218 行已读完，下表精确指明删/留/搬迁。

| 当前章节 | 行号 | 操作 | 目标位置 | 对应 FR |
|---|---|---|---|---|
| `# frp_easy` 标题 + 版本 | L1–L7 | **保留** | README 顶部不动 | — |
| `## 功能列表` | L9–L29 | **保留** | README 不动（产品价值描述，DEPLOYMENT.md 不重复） | — |
| **新增**：`## 快速开始 / 部署` 段（含 3 行决策 + DEPLOYMENT.md 链接 + 1 行最短示例 `tar xzf … && ./frp-easy`） | — | **插入**到 L7 后 / L9 前 | README 顶部 | **FR-5.1 / AC-19** |
| `## 前置条件` | L32–L40 | **保留**（贡献者仍需读） | 原位置或下沉至"开发模式"上方 | — |
| `## 快速开始 / Linux + Windows` | L44–L76 | **删除**（详细命令） | 整体搬迁至 `docs/DEPLOYMENT.md` 路径 B；README 仅留一句"源码构建详见 [DEPLOYMENT.md 路径 B](docs/DEPLOYMENT.md#路径-b-源码构建)" | **FR-5.4 / AC-22** |
| `## 默认端口表` | L80–L89 | **保留** | 不动；DEPLOYMENT.md 路径 A 故障排查会**引用**而非重复 | **AC-22 避免重复** |
| `## 配置说明` | L93–L115 | **保留** | 不动（schema 文档性质，部署文档不重复 schema） | — |
| `## 更新流程` | L119–L158 | **删除并搬迁** | 全量迁移至 DEPLOYMENT.md 路径 A §升级 与 路径 C §升级；README 仅留一行"升级流程详见 [DEPLOYMENT.md](docs/DEPLOYMENT.md#升级)" | **FR-5.2 / AC-20** |
| `## 开发模式` | L161–L174 | **保留并下沉** | 移到 README 后段（"目录结构速览"之前），标题改为 `## 开发模式（面向贡献者）`，明确受众 | **FR-5.3 / AC-21** |
| `## 目录结构速览` | L178–L204 | **保留** | 不动 | — |
| `## 技术债与优化建议` | L208–L217 | **保留** | 不动 | — |

**校验方法**（AC-22）：重排后 `git diff README.md` 不应在 DEPLOYMENT.md 与 README.md 中出现相同的多行代码块（`bash scripts/build.sh` 这类单行命令出现一次在 DEPLOYMENT.md 路径 B 即可，README 不再出现）。

---

## 8. `docs/DEPLOYMENT.md` 大纲（仅标题层级 + 每节要点；不写正文）

```
# 部署文档 — frp_easy

## 我属于哪条路径？           ← AC-17 决策表
  表头：角色 | 我想做什么 | 推荐路径
  - 行 1：非开发者 / 普通用户  | 跑起来用 | 路径 A
  - 行 2：开发者 / 贡献者      | 改代码 | 路径 B
  - 行 3：运维 / 生产长驻      | 跑成系统服务 | 路径 C（先做完 A）

## 路径 A — 下载发布产物（推荐）      ← AC-15 / FR-4.1
  ### A.1 下载                       占位链接 https://github.com/<org>/frp_easy/releases/latest
  ### A.2 解压                       Linux: tar xzf … / Windows: Expand-Archive
  ### A.3 首启                       ./frp-easy 或 frp-easy.exe；浏览器开 http://127.0.0.1:8080
  ### A.4 升级                       停 → 解压新包覆盖二进制 → 启；frp_easy.toml/.frp_easy/ 不动
  ### A.5 卸载                       删解压目录 + 手工删 .frp_easy/ 与 frp_easy.toml

## 路径 B — 源码构建（开发者）        ← AC-15 / FR-4.1
  ### B.1 前置                       Go 1.22+ / Node 18+
  ### B.2 克隆                       git clone …
  ### B.3 构建                       scripts/build.sh / build.ps1（含 --all）
  ### B.4 运行                       ./bin/frp-easy
  ### B.5 开发模式                   scripts/start.sh / start.ps1（双进程 Vite + Go）

## 路径 C — 作为系统服务（运维）      ← AC-15 / FR-4.1
  ### C.1 前置                       已走完路径 A；二进制 + scripts/ 已就位
  ### C.2 Linux systemd
    - C.2.1 安装                    sudo ./scripts/install-service.sh [--user nobody]
    - C.2.2 状态查询                systemctl status frp-easy / is-active
    - C.2.3 查看日志                journalctl -u frp-easy -f
    - C.2.4 升级                    停服 → 覆盖二进制 → 启服（不重跑 install）
    - C.2.5 卸载                    sudo ./scripts/uninstall-service.sh
  ### C.3 Windows Service
    - C.3.1 安装                    管理员 PowerShell：.\scripts\install-service.ps1 [-DisplayName "..."]
    - C.3.2 状态查询                sc query frp-easy
    - C.3.3 查看日志                Windows 事件查看器 / .frp_easy\logs\ui.log
    - C.3.4 升级                    sc.exe stop → 覆盖 → sc.exe start
    - C.3.5 卸载                    管理员 PowerShell：.\scripts\uninstall-service.ps1

## 故障排查                           ← AC-18 / FR-4.4（必须覆盖 5 场景）
  ### F.1 端口被占用                 frp-easy 启动报 "端口 8080 已被占用"；编辑 frp_easy.toml 改 UIPort 后重启
  ### F.2 UI 打不开                  curl http://127.0.0.1:8080/api/v1/system/ready 看 503；检 .frp_easy/logs/ui.log
  ### F.3 systemd 启动失败           journalctl -u frp-easy --since "10 min ago"；常见为路径变动 / User 不存在
  ### F.4 Windows Service 未启动     sc query frp-easy；事件查看器 "Windows 日志 → 系统" 过滤 Service Control Manager
  ### F.5 frp 子二进制缺失           UI 顶部横幅一键下载；离线场景手动放回 frp_linux/ 或 frp_win/

## 升级（小节锚点为 README 链接目标）
  路径 A：见 A.4；路径 C：见 C.2.4 / C.3.4

## 命令引用脚本（NFR-4.1）
  本文件每段代码块标注 ```bash / ```powershell；占位符约定 <INSTALL_DIR> = 解压目录绝对路径
```

**FR-4.2 占位符约定**（在文档顶部"我属于哪条路径？"下声明）：
- `<INSTALL_DIR>` = 用户解压发布包的目录，Linux 推荐 `/opt/frp-easy`，Windows 推荐 `C:\Program Files\frp-easy`
- `<VERSION>` = 实际下载的版本号（如 `0.1.0`）
- `<ORG>` = GitHub 组织名占位符（Open Question 9 PM-resolved (a)）

---

## 9. 风险与缓解

| # | 风险 | 影响范围 | 缓解 |
|---|---|---|---|
| R-1 | **跨平台路径差异**：bash 用 `/`，PowerShell 用 `\`；脚本里硬编码任一会破跨平台 | package.sh / package.ps1 / install-service.* | 严格分平台：.sh 用 `/`，.ps1 用 `Join-Path`；不试图共用一份脚本。所有"绝对路径解析"分别用 `realpath`（POSIX）与 `(Resolve-Path).Path`（Windows）。 |
| R-2 | **Windows 管理员权限检测**遗漏导致 `sc.exe create` 静默失败 | install-service.ps1 / uninstall-service.ps1 | 脚本入口强制 `WindowsPrincipal.IsInRole(Administrator)` 检测；非管理员**立即** `Write-Error` + `exit 1`。FR-2.2 硬约束。 |
| R-3 | **systemd 不存在的 Linux**（WSL1 默认无、容器内、Alpine 装 OpenRC 等） | install-service.sh | 脚本顶部 `command -v systemctl >/dev/null` 检测，缺失打印中文"未检测到 systemd（如 WSL1 / OpenRC），请改用前台运行（./frp-easy）或自行集成"后退出 1。不试图支持 OpenRC / launchd / SysV-init（Open Question 范围已锁定）。 |
| R-4 | **tar 与 zip 行为差异**：tar 保留 POSIX 权限位，zip 会丢可执行位（NFR Risk 已识别） | package.* | Linux 包只发 tar.gz；Windows 包只发 zip。Linux 用户即使误下载 zip，README.txt 含 `chmod +x frp-easy frp_linux/*` 兜底指令（在故障排查 F-5 之外另加一条"如下载错平台压缩包"提示）。 |
| R-5 | **git describe 在浅克隆下失败**（CI 浅 clone / GitHub Codespaces 默认 `--depth 1`） | package.* 版本注入 | 既有 build.sh L19 已用 `\|\| echo "dev"` 兜底，package 脚本继承该行为；同时支持 `--version <s>` / `-Version <s>` 显式覆盖，运维可手工指定。最终 `VERSION` 文件写入版本字符串，与 `--version` 输出一致。 |
| R-6 | **realpath 在 macOS bsd 与 GNU 行为不同**（macOS 默认 `realpath` 不接受 `-m`） | install-service.sh | 仅使用最基本调用 `realpath "$path"`，不传 flag；缺失时 fallback `cd "$(dirname "$0")/.." && pwd`（既有 build.sh L16 已是该写法）。 |
| R-7 | **`/etc/systemd/system/frp-easy.service` 非原子写**导致 daemon-reload 读到半成品 | install-service.sh | 写 `.tmp` 同目录文件 → `mv -f` 原子 rename（Linux POSIX 保证）→ 双重 `chmod 0644`（insight-index 2026-05-19 经验）。 |
| R-8 | **Windows zip 内可执行位丢失**：Linux 用户在 Windows 拿到 tar.gz 解压再传 | 用户端 | 只在 Linux 平台发 tar.gz；README.txt 不在 Windows 包内写 chmod 指令。文档明确"用什么平台下什么包"。 |
| R-9 | **`bin/release/` 旧产物累积撑爆磁盘** | 开发主机 | 打包脚本不自动清理旧 release（防误删用户保留的回归版本）；只覆盖同名。文档建议运维手动 `rm bin/release/*.tar.gz` 周期清理。 |
| R-10 | **`Windows Service` 的 cwd 默认在 `System32`**，导致 frp-easy 找不到 `frp_easy.toml` | install-service.ps1 / 用户体验 | install-service.ps1 生成 `<INSTALL_ABS>\frp-easy-svc.cmd` 包装脚本（`cd /d "<INSTALL_ABS>" && frp-easy.exe`），sc.exe binPath 指向该 .cmd；uninstall 一并删除。 |
| R-11 | **`sc.exe` 语法等号后空格规则陷阱**（`binPath="x"` 与 `binPath= "x"` 含义不同） | install-service.ps1 | 脚本注释顶部明示该规则；用 PowerShell 数组传参确保等号后真的有空格而不是被 shell 吞掉。 |
| R-12 | **包体 > 25 MB** 超过 NFR-8.1 软上限 | 用户下载体验 | 打包脚本 `du -m` 计算压缩后大小；> 25 MB 时 stderr 打 `WARN: 包体 28 MB 超出 25 MB 软上限`，**不**失败。后续可考虑用 UPX 压缩 Go 二进制（本期不做）。 |

---

## 10. AC 可实现性映射（24 条）

| AC | 描述 | 本设计具体行为 / 文件 | 可验证手段 |
|---|---|---|---|
| AC-1 | 产出 `tar.gz` / `zip` 文件 | §3.1 package.sh / §3.2 package.ps1；输出 `bin/release/frp-easy-<ver>-<os>-<arch>.<ext>` | `ls bin/release/` + `file …` |
| AC-2 | 文件名版本号 = `git describe …` | §3 各脚本 `VERSION=$(git describe --tags --always --dirty || echo dev)`；继承 build.sh L19 | tar tzf 查看顶层目录名 |
| AC-3 | 包内含主二进制、对应 OS frp 子目录、`.toml.example`、`README.txt`、`scripts/`、`VERSION`、`LICENSE` | §4 目录布局 | `tar tzf` 列表逐项 grep |
| AC-4 | `bin/frp-easy` 缺失时立即非 0 退出 | §3.1 前置校验 5 条；`[[ -x .../frp-easy ]]` 缺失 → exit 1 | `rm bin/frp-easy && ./scripts/package.sh; echo $?` |
| AC-5 | 重复执行第二次成功覆盖 | §4 staging 每次 `rm -rf`；tar 覆盖；`Compress-Archive -Force` | `./scripts/package.sh && ./scripts/package.sh; ls -la` |
| AC-6 | `install-service.sh` 后 `systemctl is-active` = `active` | §3.3 daemon-reload + enable --now；§5.1 unit Type=simple | `sudo ./install-service.sh && systemctl is-active frp-easy` |
| AC-7 | `uninstall-service.*` 后 `systemctl status` 报 not found | §3.4 disable --now + rm unit + daemon-reload；§3.6 `sc.exe delete` | 验证命令文档 |
| AC-8 | 连续 install 两次第二次 exit 0 | §3.3 / §3.5 幂等分支"已刷新现有 unit" | 两次执行 + `echo $?` |
| AC-9 | 卸载后 `frp_easy.toml` / `.frp_easy/` 仍在 | §3.4 / §3.6 卸载脚本**只**删 unit / service，不碰数据目录；结尾打印保留提示 | `ls .frp_easy/ && ls frp_easy.toml` 对比前后 |
| AC-10 | `--user nobody` / `-DisplayName "FRP 测试"` 生效 | §3.3 `--user` 参数 + getent 校验；§3.5 `-DisplayName` 参数透传到 `sc.exe DisplayName=` | `grep User= /etc/systemd/system/frp-easy.service` / `sc query frp-easy` |
| AC-11 | `./frp-easy --version` / `-v` 输出 `frp-easy <ver>` + exit 0 | §6.3 flag 解析块 `if showVersion { fmt.Printf("frp-easy %s\n", Version); return nil }` | 直接运行 |
| AC-12 | `--help` / `-h` 输出含中文用法、flag 列表、配置位置、UI 地址、退出码 | §6.2 `usageText` 常量包含全部要求项 | grep usageText 输出 |
| AC-13 | 无 `frp_easy.toml` 时仍能跑 `--version` | §6.3 flag 解析早于 `appconf.Load`；`return nil` 直接 main 退出 | `cd $(mktemp -d) && /path/to/frp-easy --version` |
| AC-14 | `--foo` 未知 flag → 中文错误 + exit 2 | §6.3 `fs.Parse` 失败分支 `os.Exit(2)` | `./frp-easy --foo; echo $?` |
| AC-15 | DEPLOYMENT.md 含三个顶级章节 | §8 大纲三个 `## 路径 X` | grep `^## 路径` |
| AC-16 | 命令复制即可运行 | §8 占位符约定 + 每段 bash/powershell 代码块直接可用 | QA 走查 |
| AC-17 | DEPLOYMENT.md 顶部决策表 ≥ 3 行 | §8 第一个 H2 "我属于哪条路径？" | grep 表头 |
| AC-18 | 故障排查覆盖 5 场景 + 日志位置 | §8 F.1–F.5 | grep `^### F\.` |
| AC-19 | README 顶部"快速开始 / 部署"段含 DEPLOYMENT.md 链接 + 1 行示例 | §7 表"**插入**"行 | grep README.md 顶部 30 行 |
| AC-20 | README "更新流程" 详细命令删除 | §7 表 L119–L158 整体搬迁，README 留一行链接 | `git diff README.md` |
| AC-21 | "开发模式" 仍存在但下沉 | §7 表 L161–L174 移到目录结构之前，标题改为"（面向贡献者）" | 目视 + grep 行号 |
| AC-22 | README 与 DEPLOYMENT.md 命令不重复 | §7 表 各行"对应 FR" 列；FR-5.4 权威源原则 | `comm -12 <(grep -E '^(bash\|sudo\|sc\.exe\|systemctl\|tar\|Expand)' README.md) <(grep -E ... DEPLOYMENT.md)` 应为空 |
| AC-23 | verify_all 仍 PASS 18 项 | 本任务**不**修改 `verify_all.sh`；新文件均落在 `scripts/` 与 `docs/`，不触发现有 18 项的 FAIL 条件（无新 Go 文件、无新前端文件、无新 import）| `./scripts/verify_all.sh` |
| AC-24 | package.sh 前置缺失返回非 0 | §3.1 退出码 1 / 2；同 AC-4 副产物 | 见 AC-4 |

---

## 11. 实现顺序建议

Developer（dev-backend，本任务全部归属此分区——见 §12）按下列顺序分批落地，每批可单独跑 `scripts/verify_all`：

1. **第 1 批 — `cmd/frp-easy/main.go` flag 解析**（§6）
   - 改动量小（约 35 行），影响面可控；先做让 `--version` / `--help` 立即可用，便于打包脚本里调用 `frp-easy --version` 做 sanity check。
   - 验证：`go build` + 跑 `./bin/frp-easy --version` / `--help` / `--foo`；`verify_all` 18 项仍 PASS。

2. **第 2 批 — 服务安装/卸载脚本 4 个**（§3.3–§3.6）
   - Linux 两个先做（systemd 行为可在 Linux 主机直接验证）；Windows 两个后做（需 Windows 主机）。
   - 验证：手工跑 `install-service.sh` / `uninstall-service.sh`，对照 AC-6 / AC-7 / AC-8 / AC-9 / AC-10。

3. **第 3 批 — 打包脚本 2 个**（§3.1 / §3.2 + §4）
   - 调用第 2 批已就位的 install-service.* 复制进 staging。
   - 验证：`./scripts/package.sh` 后 `tar tzf bin/release/*.tar.gz` 对照 AC-3。

4. **第 4 批 — `docs/DEPLOYMENT.md`**（§8）
   - 此时所有命令、参数、退出码都已固定，文档可以以"权威源"原则照抄实际行为。
   - 验证：QA 走查 AC-15 / AC-16 / AC-17 / AC-18。

5. **第 5 批 — README 重排 + `docs/dev-map.md` 索引更新**（§7）
   - 最后做，避免文档与脚本不同步。
   - 验证：`git diff README.md` 对照 §7 表；AC-19 / AC-20 / AC-21 / AC-22。

6. **收尾**：`scripts/verify_all.sh` 全绿（AC-23）；归档 `04_DEVELOPMENT.md`。

---

## 12. Partition assignment（分区分配）

本任务所有改动均落在 `dev-backend` 的 owned paths（见 `.harness/agents/dev-backend.md` §Owned paths：`cmd/**`、`scripts/build.{ps1,sh}` 与脚本目录、`docs/dev-map.md`、`README.md` 属于通用根目录文档）。无前端、无 DB、无 migration 改动。

| 文件 | 分区 | 新增 / 编辑 | 依赖 |
|---|---|---|---|
| `cmd/frp-easy/main.go` | dev-backend | edit | — |
| `scripts/package.sh` | dev-backend | new | 依赖既有 `scripts/build.sh` 不变 |
| `scripts/package.ps1` | dev-backend | new | 依赖既有 `scripts/build.ps1` 不变 |
| `scripts/install-service.sh` | dev-backend | new | — |
| `scripts/uninstall-service.sh` | dev-backend | new | 引用 install-service.sh 产生的 unit 路径 |
| `scripts/install-service.ps1` | dev-backend | new | — |
| `scripts/uninstall-service.ps1` | dev-backend | new | 引用 install-service.ps1 产生的服务名 |
| `docs/DEPLOYMENT.md` | dev-backend | new | 所有脚本与 main flag 完成后再写 |
| `README.md` | dev-backend | edit | DEPLOYMENT.md 就绪后重排 |
| `docs/dev-map.md` | dev-backend | edit | 所有新文件就位后追加索引 |

### Dispatch 顺序

1. dev-backend（**单分区**，按 §11 五批顺序内部串行）

### Parallelism

无跨分区并行；分区内部按 §11 顺序，不可并行（main.go → 服务脚本 → 打包脚本 → 文档 → README，每批依赖前一批的产出稳定）。

---

## Verdict — READY FOR GATE REVIEW

设计完成。无新 Go 依赖、无 verify_all 改动、无 storage/migration 改动；全部 24 条 AC 在 §10 表中有可实现性映射；风险 §9 共 12 条均有缓解；实现顺序 §11 已分 5 批可独立验证。下一步 PM 派 Gate Reviewer 走第 3 阶段评审。
