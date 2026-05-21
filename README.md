# frp_easy

一个基于 Go + Vue 3 + SQLite 的单二进制 FRP Web 管理 UI —— 用浏览器可视化管理 frpc / frps，无需手写配置文件。

版本：**0.1.0**

---

## 项目简介

[frp](https://github.com/fatedier/frp) 是优秀的内网穿透工具，但它通过手写 `frpc.toml` / `frps.toml` 配置文件来使用，对非专业用户门槛较高，改配置后还要记得重载或重启进程。

**frp_easy** 在 frp 之上提供一层 Web 管理界面：把代理规则、服务端 / 客户端连接、进程启停都搬到浏览器里点选完成；配置变更自动渲染成 frp 的 TOML 并热重载。整个应用（Go 后端 + 内嵌的 Vue 3 前端）打包为**一个可执行文件**，下载即用、零外部依赖、零配置上手。

---

## 功能亮点

### 核心管理能力

- **首启 setup 向导**：首次启动自动进入 `/setup`，创建管理员账号（argon2id 密码哈希），完成后自动登录。
- **部署向导**：新安装自动进入 `/wizard`，3 步流程（选角色 → 填配置 → 完成），支持 frpc-only / frps-only / 两者。
- **frpc / frps 进程控制**：在 Dashboard 一键启动 / 停止 / 重启，2 秒轮询状态与 PID；模式开关持久化，重启后自动恢复。
- **代理规则管理**：tcp / udp / http / https 四类代理增删改查，200 条上限，乐观锁防并发覆盖。
- **服务端 / 客户端配置**：frps 的 bindPort / authToken / dashboard、frpc 的 serverAddr / serverPort / authToken，敏感令牌脱敏显示。
- **配置即生效**：任何配置变更立即触发 DB → TOML 管道（渲染 → 原子写入 `frpc.toml` / `frps.toml` → 通知 frp 热重载）。
- **日志查看**：frpc / frps 日志页面，首屏 tail 500 行 + 2 秒增量轮询。
- **认证安全**：会话 Cookie（HttpOnly + SameSite）、CSRF 防护、登录失败 5 次触发 IP 限流（429 + Retry-After）。

### 零配置与运维体验

- **frp 二进制自动下载**：检测到 frpc / frps 缺失时，UI 顶部弹横幅，一键从 GitHub Releases 异步下载，进度条实时显示 0–100%。
- **公网 IP 检测**：服务端配置页一键检测公网 IP，结果仅展示不自动填充。
- **防火墙提示**：按代理协议生成 ufw / iptables 命令，可一键复制。
- **浏览器自动打开**（T-010）：TTY 启动时自动打开浏览器；`--no-browser` 或环境变量 `FRP_EASY_NO_BROWSER` 可关闭，systemd / 服务模式自动跳过。
- **日志轮转**（T-010）：`ui.log` 按大小 / 历史份数 / 保留天数三轴轮转，长跑服务不爆盘。

### 工程化保障

- **OpenAPI 3.0.3 规范**（T-005）：根目录 [`openapi.yaml`](openapi.yaml) 描述全部 REST 路由。
- **E2E 烟雾测试**（T-006）：Playwright 端到端测试覆盖 setup / 登录 / dashboard 流程。
- **安全加固**（T-007）：`ui.log` 权限收紧至 0600，附带 SQL 注入 / 并发 / 越界等对抗性测试。
- **部署套件**（T-008）：一键打包脚本、部署权威文档、中文 `--help`、systemd 与 Windows Service 安装脚本。
- **跨 shell parity**（T-009）：verify_all 与测试脚本在 PowerShell 与 Git Bash 下行为一致。
- **GitHub Actions CI**（T-010）：push `v*` tag 自动构建并上传 GitHub Releases 资产。

---

## 快速开始

最短上手路径（以 Linux 发布包为例）：

```bash
# 1. 下载并解压发布包
tar xzf frp-easy-<VERSION>-linux-amd64.tar.gz
cd frp-easy-<VERSION>-linux-amd64

# 2. 运行
./frp-easy
```

启动后：

- 本机访问：浏览器打开 <http://127.0.0.1:7800>。
- 同局域网其他设备访问：`http://<本机IP>:7800`（默认绑定 `0.0.0.0`，见下方"配置说明"与"从其他设备访问")。
- 首次进入会引导你完成 setup 向导，创建管理员账号。

> 完整部署指南（发布包 / 源码构建 / 系统服务三条路径）见 **[docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)**。

---

## 配置说明

frp_easy 自身的配置文件是 `frp_easy.toml`（与 frp 的 `frpc.toml` / `frps.toml` 是**不同**的文件）。首次启动若不存在，会自动写入默认值，无需手动创建。

| 字段 | 默认值 | 说明 |
|---|---|---|
| `UIBindAddr` | `0.0.0.0` | UI 服务监听地址（仅主机，不含端口） |
| `UIPort` | `7800` | UI 服务监听端口 |
| `DataDir` | `./.frp_easy` | 数据目录（SQLite 数据库存放路径） |
| `LogDir` | `./.frp_easy/logs` | 日志目录（frpc / frps 子进程日志） |

示例 `frp_easy.toml`：

```toml
UIBindAddr = "0.0.0.0"
UIPort     = 7800
DataDir    = "./.frp_easy"
LogDir     = "./.frp_easy/logs"
```

> 升级时不会改写你已有的 `frp_easy.toml`：默认值变更只作用于**新建配置文件**与**留空字段**。老用户若显式写过 `UIBindAddr` / `UIPort`，升级后保持原值不变。

### 从其他设备访问 Web UI

frp_easy 默认 `UIBindAddr = "0.0.0.0"`，即监听所有网卡。这是有意的设计取舍：frp_easy 本质是远程内网穿透管理工具，运维场景天然需要从其他设备访问 Web UI，默认仅本机会迫使每个用户首启后改配置。

- **同局域网访问**：在其他设备的浏览器打开 `http://<运行 frp_easy 的机器 IP>:7800`。
- **安全说明**：`0.0.0.0` 不等于"无认证暴露" —— frp_easy 已内置认证加固（argon2id 密码哈希、会话 Cookie、CSRF 防护、登录失败 IP 限流）。绑定对外地址时，启动期会在 stderr 打印一条安全提示，引导你**尽快完成 setup 向导**（setup 完成前界面尚无密码保护，这是最需要尽快关闭的暴露窗口）。
- **如对外暴露到公网**：建议放在反向代理（Nginx / Caddy）之后，并确认防火墙规则。

### 仅本机使用：改回 127.0.0.1

如果只在本机使用、不需要其他设备访问，编辑 `frp_easy.toml`：

```toml
UIBindAddr = "127.0.0.1"
```

保存后重启 frp_easy。此时 UI 只有本机可访问，启动期也不再打印对外暴露的安全提示。

---

## 默认端口表

| 用途 | 默认端口 | 由谁监听 |
|---|---|---|
| frp_easy UI（HTTP） | 7800 | frp-easy 进程 |
| frpc admin API（reload / status） | 7400 | frpc 子进程 |
| frps dashboard（Web UI 自带） | 7500 | frps 子进程 |
| frps bindPort（FRP 控制通道） | 7000 | frps 子进程 |

端口真相源：`internal/appconf/config.go` 顶部注释。四者目前无重叠，修改前请核对。（第五项 `proxy.remotePort` 由用户在代理规则中自定义，不在上表中。）

---

## 文档导航

| 文档 | 内容 |
|---|---|
| [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) | 部署权威文档：发布包 / 源码构建 / 系统服务三条路径 + 故障排查 |
| [docs/project-status.html](docs/project-status.html) | 项目状况总览（技术栈 / 已交付功能 / 测试基线 / 技术债，浏览器打开） |
| [docs/architecture.html](docs/architecture.html) | 架构总览（分层架构 / 模块详解 / 数据流 / API 路由，浏览器打开） |
| [docs/dev-map.md](docs/dev-map.md) | 开发导航：项目结构、文件归属、可复用工具 |
| [openapi.yaml](openapi.yaml) | REST API 的 OpenAPI 3.0.3 规范 |

---

## 开发模式（面向贡献者）

双进程开发：Go API + Vite dev server 同时启动，前端热重载，Go 侧开启 CORS 允许 Vite 代理。

```bash
bash scripts/start.sh     # Linux / macOS
.\scripts\start.ps1       # Windows
```

- Go API 在 <http://127.0.0.1:7800>
- Vite dev server 在 <http://127.0.0.1:5173>

按 Ctrl+C 同时停止两个进程。

前置条件：

| 工具 | 最低版本 | 说明 |
|---|---|---|
| Go | 1.25+ | 编译 Go 后端 |
| Node.js | 18+ | 编译 Vue 3 前端 |
| npm | 随 Node.js | 前端包管理 |

> **重要**：`internal/assets/dist/`（Go embed 的输入目录）被 `.gitignore` 排除，不在 git 中。克隆后**必须先运行 `npm run build`**（或直接用 `scripts/build.sh` / `build.ps1`）生成 dist 目录，否则 `go build` 会因缺少嵌入资源而失败。完整构建 / 调试 / 打包流程见 [DEPLOYMENT.md 路径 B](docs/DEPLOYMENT.md#路径-b--源码构建)。

---

## 目录结构速览

```
frp_easy/
├── cmd/frp-easy/   — Go 程序入口（main.go；单二进制）
├── internal/
│   ├── appconf/    — 读写 frp_easy.toml（UI 服务自身配置）
│   ├── assets/     — embed.FS + SPA fallback（嵌入 dist/）
│   ├── auth/       — argon2id 哈希 / session token / CSRF token / IP 限流
│   ├── binloc/     — 按 OS 选 frp_win/ 或 frp_linux/ 二进制路径
│   ├── browseropen/— 跨平台浏览器自动打开 + TTY 检测（T-010）
│   ├── downloader/ — frp 二进制自动下载：异步下载、进度追踪、原子安装（T-002）
│   ├── frpcadmin/  — frpc admin HTTP 客户端（/api/reload、/api/status）
│   ├── frpconf/    — DB→TOML 渲染器（AtomicWrite）
│   ├── httpapi/    — chi router + 全部 REST handler + 中间件链
│   ├── logrotate/  — ui.log 三轴轮转（size / backups / age，T-010）
│   ├── logtail/    — TailLines + ReadFrom 增量读取子进程日志
│   ├── procmgr/    — frpc/frps 子进程生命周期（supervisor goroutine）
│   └── storage/    — SQLite 句柄 + 迁移引擎 + DAO
├── migrations/     — SQLite 迁移文件（权威源）
├── web/            — 前端 Vue 3 + Vite（src/ 目录）
├── frp_win/        — Windows FRP 二进制（vendored）
├── frp_linux/      — Linux FRP 二进制（vendored）
├── scripts/        — verify_all、start、build、package 等辅助脚本
├── docs/           — 部署文档、架构 / 状况 HTML、本导航、任务记录
└── bin/            — 构建产物（gitignore）
```

详细架构与文件归属见 [`docs/dev-map.md`](docs/dev-map.md)。

---

## 许可证

本项目尚未确定开源许可证 —— **开源许可证待项目维护者确定**。许可证选择属于项目维护者的法律决策，在维护者明确选定并添加 `LICENSE` 文件之前，请勿假定本仓库代码采用任何特定许可证。

> 注意：`frp_linux/` 与 `frp_win/` 目录下随附的 frp 二进制（frpc / frps）属于上游项目 [`fatedier/frp`](https://github.com/fatedier/frp)，遵循其 **Apache-2.0** 许可证，与 frp_easy 本身的许可证相互独立。
