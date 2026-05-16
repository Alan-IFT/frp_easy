# frp_easy

基于 Go + Vue 3 + SQLite 的单二进制 FRP Web 管理 UI，无需手动编写配置文件。

版本：**0.1.0**

---

## 功能列表

### T-001 核心功能

- **setup 向导**：首次启动自动跳转 `/setup`，argon2id 密码哈希，session 自动登录
- **frpc/frps 进程控制**：启动 / 停止 / 重启，状态轮询（2 秒），PID 显示
- **模式自动启动**：NSwitch 切换，`mode.{kind}.enabled` KV 持久化，重启后自动恢复
- **frpc 客户端配置**：serverAddr / serverPort / authMethod / authToken
- **frps 服务端配置**：bindPort / authToken / dashboard，token 脱敏显示
- **代理规则管理**：tcp / udp / http / https，增删改查，200 条上限，乐观锁
- **DB→TOML 管道**：配置变更即时写 frpc.toml / frps.toml，原子写入，ApplyConfigChange 热重载
- **日志查看**：tail 500 行 + 2 秒增量轮询，frpc / frps 独立页面
- **认证与安全**：session cookie，CSRF token（X-CSRF-Token），5 次失败 429 + Retry-After，argon2id

### T-002 零配置快速上手

- **frp 二进制自动下载**：首次启动检测到 frpc/frps 缺失时 UI 顶部弹出横幅，一键触发异步下载（GitHub Releases），进度条实时显示 0–100%，成功后横幅自动消失，失败时提供手动下载链接
- **部署向导**：新安装时自动跳转 `/wizard`，3 步流程（选角色 → 填配置 → 完成），支持 frpc-only / frps-only / 两者，向导完成后标记 `wizard.handled` 防止重复弹出
- **公网 IP 自动检测**：`/server` 页面新增"检测公网 IP"按钮，调用 ipify/my-ip.io，超时 3 秒，结果仅展示不自动填充，网络不通时返回友好错误
- **防火墙提示 UI**：FirewallHint 组件，按协议（tcp/udp）过滤 ufw + iptables 命令，含"复制全部"按钮，http/https 代理不显示

---

## 前置条件

| 工具 | 最低版本 | 说明 |
|---|---|---|
| Go | 1.22+ | 编译 Go 后端 |
| Node.js | 18+ | 编译 Vue 3 前端 |
| npm | 随 Node.js | 前端包管理 |

> **重要**：项目根目录的 `.gitignore` 包含 `dist/` 规则，`internal/assets/dist/`（Go embed 的输入目录）不在 git 中。克隆后**必须先运行 `npm run build`**（或直接使用 `scripts/build.sh`）才能 `go build`，否则缺少 dist 目录导致编译失败。

---

## 快速开始

### Linux

```bash
# 步骤一：克隆项目
git clone https://github.com/your-org/frp_easy.git
cd frp_easy

# 步骤二：构建（自动执行 npm install + npm run build + go build）
bash scripts/build.sh

# 步骤三：运行二进制
./bin/frp-easy
```

浏览器打开 <http://127.0.0.1:8080>，首次启动进入 setup 向导。

> **macOS 说明**：`scripts/build.sh` 默认目标为 `linux/amd64`，macOS 用户可修改 `build.sh` 中 `GOOS=linux` 为 `GOOS=darwin` 自行编译；或使用 `scripts/start.sh` 在本地进行开发调试（Go API + Vite dev server 均为本机原生进程，无需编译分发二进制）。

### Windows

```powershell
# 步骤一：克隆项目
git clone https://github.com/your-org/frp_easy.git
cd frp_easy

# 步骤二：构建（自动执行 npm install + npm run build + go build）
.\scripts\build.ps1

# 步骤三：运行二进制
.\bin\frp-easy.exe
```

---

## 默认端口表

| 用途 | 默认端口 | 由谁监听 |
|---|---|---|
| frp_easy UI（HTTP） | 8080 | frp-easy 进程 |
| frpc admin API（reload / status） | 7400 | frpc 子进程 |
| frps dashboard（Web UI 自带） | 7500 | frps 子进程 |
| frps bindPort（FRP 控制通道） | 7000 | frps 子进程 |

端口来源：`internal/appconf/config.go` 顶部注释。四者目前无重叠，修改前请核对。（第五项 proxy.remotePort 由用户在代理规则中自定义，不在上表中。）

---

## 配置说明

frp_easy 自身的配置文件为 `frp_easy.toml`（与 FRP 的 frpc.toml / frps.toml 是不同的文件）。首次启动时自动创建默认配置，无需手动创建。

| 字段 | 默认值 | 说明 |
|---|---|---|
| `UIBindAddr` | `127.0.0.1` | UI 服务监听地址（仅主机，不含端口） |
| `UIPort` | `8080` | UI 服务监听端口 |
| `DataDir` | `./.frp_easy` | 数据目录（SQLite 数据库存放路径） |
| `LogDir` | `./.frp_easy/logs` | 日志目录（frpc / frps 子进程日志） |

示例 `frp_easy.toml`：

```toml
UIBindAddr = "127.0.0.1"
UIPort     = 8080
DataDir    = "./.frp_easy"
LogDir     = "./.frp_easy/logs"
```

> **安全警告**：默认仅监听 `127.0.0.1`（本地回环），UI 只有本机可访问。  
> 若将 `UIBindAddr` 修改为 `0.0.0.0` 或其他公网地址，启动时 frp-easy 将在 stderr 打印 WARN 提示，同时 UI 对局域网 / 公网开放。  
> 请确保防火墙规则正确，或在反向代理（Nginx / Caddy）后部署并启用认证。

---

## 更新流程

> **警告：仅执行 `git pull` + 重启旧二进制是不够的**，更新不会生效。

完整更新步骤：

```bash
# Linux / macOS
git pull
bash scripts/build.sh
# 用新二进制替换旧进程并重启
```

```powershell
# Windows
git pull
.\scripts\build.ps1
# 用新二进制替换旧进程并重启
```

**为什么需要重新构建？**

1. **前端需重建**：`internal/assets/dist/` 被 `.gitignore` 的 `dist/` 规则排除，`git pull` 不会更新前端产物。前端 SPA 通过 `//go:embed all:dist` 嵌入 Go 二进制，`build.sh` 会自动执行 `npm run build` 将新前端嵌入。若仅 `go build` 跳过前端构建，二进制中嵌入的前端仍为旧版本。

2. **数据库迁移自动运行**：`storage.Open()` 在启动时对所有未应用的迁移执行 `applyOne()`，用户无需手动执行任何 SQL。新版本的迁移文件会在首次启动新二进制时自动应用。

3. **配置文件向后兼容**：`appconf.Load()` 对现有 `frp_easy.toml` 中缺失的字段自动补默认值，升级不会破坏现有配置。`frp_easy.toml` 在 `.gitignore` 中，`git pull` 不会覆盖用户配置。

4. **仅后端变更的简化路径**（可选）：若明确只有 Go 代码变更（无前端改动），可执行以下命令后重启，前端不需要重新构建：
   ```bash
   git pull
   CGO_ENABLED=0 go build -o bin/frp-easy ./cmd/frp-easy
   ```
   在不能确定是否有前端变更时，**始终使用 `scripts/build.sh`**。

**明确不足够的情形：**

- `git pull` + 直接重启旧二进制：二进制不变，更新不生效
- `git pull` + 仅 `go build`（跳过 `npm run build`）：后端更新，但前端仍为旧版本

---

## 开发模式

开发模式使用双进程：Go API（端口 8080）+ Vite dev server（端口 5173）独立运行，Go 侧开启 CORS 允许 Vite 代理。

```bash
bash scripts/start.sh
```

- Go API 在 <http://127.0.0.1:8080> 提供 REST 接口
- Vite dev server 在 <http://127.0.0.1:5173> 提供热重载前端

按 Ctrl+C 停止所有进程。

> **注意**：开发模式下 `go run ./cmd/frp-easy` 不读取 `internal/assets/dist/`（dev 阶段资源层返回 404 占位），前端请求全部由 Vite dev server 处理。

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
│   ├── downloader/ — frp 二进制自动下载（T-002）：异步下载、进度追踪、原子安装
│   ├── frpcadmin/  — frpc admin HTTP 客户端（/api/reload、/api/status）
│   ├── frpconf/    — DB→TOML 渲染器（AtomicWrite）
│   ├── httpapi/    — chi router + 全部 REST handler + 中间件链
│   ├── logtail/    — TailLines + ReadFrom 增量读取子进程日志
│   ├── procmgr/    — frpc/frps 子进程生命周期（supervisor goroutine）
│   └── storage/    — SQLite 句柄 + 迁移引擎 + DAO
├── migrations/     — SQLite 迁移文件（权威源）
├── web/            — 前端 Vue 3 + Vite（src/ 目录）
├── frp_win/        — Windows FRP 二进制（vendored）
├── frp_linux/      — Linux FRP 二进制（vendored）
├── scripts/        — verify_all、start、build 辅助脚本
├── docs/           — 规格文档、任务记录、本导航
└── bin/            — 构建产物（gitignore）
```

详细架构说明见 [`docs/dev-map.md`](docs/dev-map.md)。

---

## 技术债与优化建议

项目当前存在若干已知技术债（TD-1 ～ TD-8）和优化建议（OPT-1 ～ OPT-9），涵盖向导路由守卫漏洞、verify_all 前端检查路径、版本注入标准化等议题。

完整清单（含影响级别和优先级）请查阅：[docs/project-status.html](docs/project-status.html)
