# 部署文档 — frp_easy

> 本文档是 frp_easy 的权威部署指南，覆盖三种使用场景：
> 普通用户下载发布包、开发者源码构建、运维注册为系统服务。
> 任意命令在文档中均可直接复制运行（占位符见下）。

---

## 占位符约定

文档中如下占位符在你的实际环境中需被替换：

| 占位符 | 含义 | 示例 |
|---|---|---|
| `<INSTALL_DIR>` | 你解压发布包的目录绝对路径 | Linux `/opt/frp-easy` · Windows `C:\Program Files\frp-easy` |
| `<VERSION>` | 你下载的发布包版本号（与文件名一致） | `0.1.0` |

> 除上述两个占位符外，其余命令可**整段复制即用**。

---

## 我属于哪条路径？

| 我是 | 我想做什么 | 推荐路径 |
|---|---|---|
| 非开发者 / 普通用户 / 自用内网穿透 | 下载即用，浏览器打开就能配置 | **路径 A — 下载发布产物** |
| 开发者 / 贡献者 / 二次开发 | 修改前后端代码并本地构建 | **路径 B — 源码构建** |
| 运维 / 生产长驻 / 需要开机自启 | 跑成 systemd / Windows 服务 | **路径 C — 系统服务**（先走完路径 A 或 B） |

---

## 路径 A — 下载发布产物

最快的部署方式，适合大多数普通用户。**无需安装 Go / Node / git**。

### A.0 一键安装（推荐）

最省事的方式，下面一条命令完成下载、安装、注册开机自启。

**Linux / macOS**（需 root / sudo）：

```bash
curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash
```

**Windows**（以管理员身份运行 PowerShell）：

```powershell
irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex
```

> 安全提示：`curl | bash` 会以 root 执行远程脚本。谨慎用户可先下载脚本审阅后再运行：
>
> ```bash
> curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh -o install.sh
> # 审阅 install.sh 内容后
> sudo bash install.sh
> ```

> 安全提示：`irm | iex` 会在当前会话执行远程脚本。谨慎用户可先下载脚本审阅：
>
> ```powershell
> irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 -OutFile install.ps1
> # 审阅 install.ps1 内容后（以管理员身份运行 PowerShell）
> .\install.ps1
> ```

一键安装等价于「本路径 A 的下载解压 + 路径 C 的服务注册」的合并：

- 一键安装下载的是与 `main` 分支同步的**滚动发布**（rolling release）预编译包：
  维护者每次 push `main`，GitHub Actions 会自动重新编译并刷新这个滚动发布，
  因此一键安装拿到的始终是最新一次 `main` 构建，无需维护者手动打 tag 发版。
- 固定安装目录：Linux/macOS `/opt/frp-easy`，Windows `C:\Program Files\frp-easy`
  （高级用户可用环境变量 `FRP_EASY_INSTALL_DIR` 覆盖）。
- 自动注册 systemd / Windows 服务并启动，实现开机自启。
- 安装完成后脚本会打印 UI 访问地址（`http://127.0.0.1:7800` 与 `http://<本机IP>:7800`）。

> 说明：一键安装已自动完成路径 C 的服务注册，**无需再手动执行 install-service**；
> 若想了解服务的状态查询 / 日志 / 卸载命令，见下方[路径 C](#路径-c--作为系统服务)。
> macOS 因无 systemd / launchd 模板，一键安装会下载安装后提示手动启动，不注册服务。

> 不想用一键安装？下方 **A.1–A.3 手动安装（备选）** 是不使用一键安装时的备选路径。

### A.1 下载（手动安装 — 备选）

> 以下 A.1 / A.2 / A.3 是不使用 A.0 一键安装时的手动备选路径。

到发布页面下载与你操作系统匹配的压缩包：

- 下载地址：<https://github.com/Alan-IFT/frp_easy/releases/tag/rolling>
  该地址始终指向与 `main` 分支同步的最新滚动发布构建（每次 push `main` 自动刷新）。
- Linux 用户：`frp-easy-<VERSION>-linux-amd64.tar.gz`
- Windows 用户：`frp-easy-<VERSION>-windows-amd64.zip`

### A.2 解压

**Linux / macOS**：

```bash
tar xzf frp-easy-<VERSION>-linux-amd64.tar.gz
cd frp-easy-<VERSION>-linux-amd64
```

**Windows**（PowerShell）：

```powershell
Expand-Archive -Path .\frp-easy-<VERSION>-windows-amd64.zip -DestinationPath .
cd .\frp-easy-<VERSION>-windows-amd64
```

### A.3 首启

**Linux / macOS**：

```bash
./frp-easy
```

**Windows**：

```powershell
.\frp-easy.exe
```

启动后 stderr 会打印：

```
frp_easy UI 已启动：http://0.0.0.0:7800 （Ctrl+C 退出）
```

> 默认 `UIBindAddr = "0.0.0.0"` 监听所有网卡，stderr 还会打印一条安全提示。
> 本机访问用 <http://127.0.0.1:7800>；同局域网其他设备用 `http://<本机IP>:7800`。
> 仅需本机访问时，把 `frp_easy.toml` 的 `UIBindAddr` 改为 `127.0.0.1` 后重启。

浏览器打开 <http://127.0.0.1:7800>，按 setup 向导创建管理员账号并完成首启配置。

> 自检命令：
>
> ```bash
> ./frp-easy --version    # 输出 frp-easy <VERSION>
> ./frp-easy --help       # 显示中文帮助
> ```

### A.4 升级

> 若通过 [A.0 一键安装](#a0-一键安装推荐)，升级只需**重跑一键安装命令**：脚本会自动识别
> 为升级，停服 → 覆盖二进制与脚本 → 重跑服务注册（幂等、安全），并完整保留你的
> `frp_easy.toml` 与 `.frp_easy/`。下方手动升级步骤仅适用于手动安装（A.1–A.3）的场景。

发布新版本后：

```bash
# Linux
# 1. 停掉旧进程（Ctrl+C 或 kill）
# 2. 解压新包到新目录
tar xzf frp-easy-<新VERSION>-linux-amd64.tar.gz
# 3. 把你的现有 frp_easy.toml 与 .frp_easy/ 拷过来（如果在旧目录里）
cp -a /旧解压目录/frp_easy.toml /旧解压目录/.frp_easy ./frp-easy-<新VERSION>-linux-amd64/
# 4. 启新版
cd frp-easy-<新VERSION>-linux-amd64 && ./frp-easy
```

```powershell
# Windows
# 1. 关掉旧进程（Ctrl+C 或任务管理器）
# 2. 解压新包
Expand-Archive -Path .\frp-easy-<新VERSION>-windows-amd64.zip -DestinationPath .
# 3. 拷贝现有配置到新目录
Copy-Item -Recurse \旧解压目录\frp_easy.toml,\旧解压目录\.frp_easy .\frp-easy-<新VERSION>-windows-amd64\
# 4. 启新版
cd .\frp-easy-<新VERSION>-windows-amd64 ; .\frp-easy.exe
```

数据库迁移在 `storage.Open()` 启动时自动应用，无需手工 SQL。

### A.5 卸载

```bash
# Linux
rm -rf /path/to/frp-easy-<VERSION>-linux-amd64/
# 数据彻底清理（可选）：上一行的目录里就有 .frp_easy/，已一并删除
```

```powershell
# Windows
Remove-Item -Recurse -Force C:\path\to\frp-easy-<VERSION>-windows-amd64\
```

> 若先通过路径 C 注册为系统服务，请先按 **C.2.5 / C.3.5** 卸载服务再删目录。

---

## 路径 B — 源码构建

适合开发者、贡献者、二次开发。

### B.1 前置

| 工具 | 最低版本 | 说明 |
|---|---|---|
| Go | 1.25+ | 编译 Go 后端（与 `go.mod` 的 `go 1.25.0` 一致） |
| Node.js | 18+ | 编译 Vue 3 前端 |
| npm | 随 Node.js | 前端包管理 |
| git | 任意近期版本 | clone + `git describe` 版本注入 |

### B.2 克隆

```bash
git clone https://github.com/Alan-IFT/frp_easy.git
cd frp_easy
```

### B.3 构建

**Linux / macOS**：

```bash
bash scripts/build.sh          # 仅构建 Linux amd64
bash scripts/build.sh --all    # 同时交叉编译 Windows amd64
```

**Windows**：

```powershell
.\scripts\build.ps1            # 仅构建 Windows amd64
.\scripts\build.ps1 -All       # 同时交叉编译 Linux amd64
```

构建产物落在 `bin/`：

- `bin/frp-easy`（Linux 二进制）
- `bin/frp-easy.exe`（Windows 二进制）
- `bin/frp-easy-linux`（Windows 主机交叉编译产出 Linux 时）

构建会自动执行：

1. `npm install --frozen-lockfile`
2. `npm run build`（产出 `internal/assets/dist/`，被 `//go:embed` 嵌入 Go 二进制）
3. `go build -ldflags "-X main.Version=..." ./cmd/frp-easy`

> 重要：`.gitignore` 排除了 `dist/`，**任何 `go build` 之前必须先 `npm run build`**，否则 embed 取不到前端资源。直接走 `scripts/build.*` 不会踩坑。

### B.4 运行

```bash
./bin/frp-easy            # Linux
.\bin\frp-easy.exe        # Windows
```

浏览器开 <http://127.0.0.1:7800>。

### B.5 开发模式（双进程：Go API + Vite dev server）

开发模式下 Go API 与 Vite dev server 同时启动，Vite 提供前端热重载。

```bash
bash scripts/start.sh     # Linux / macOS
```

```powershell
.\scripts\start.ps1       # Windows
```

- Go API 在 <http://127.0.0.1:7800>
- Vite dev server 在 <http://127.0.0.1:5173>

按 Ctrl+C 同时停止两个进程。

### B.6 自行打包发布产物

```bash
bash scripts/package.sh                 # 打 linux-amd64 tar.gz
bash scripts/package.sh --windows       # 同时打 windows-amd64 zip
bash scripts/package.sh --skip-build    # 不重跑 build.sh
```

```powershell
.\scripts\package.ps1                   # 打 windows-amd64 zip（默认）
.\scripts\package.ps1 -Linux            # 同时打 linux-amd64 tar.gz（需 tar.exe）
.\scripts\package.ps1 -SkipBuild        # 不重跑 build.ps1
```

产物写入 `bin/release/frp-easy-<VERSION>-<os>-amd64.<ext>`。

---

## 路径 C — 作为系统服务

适合需要开机自启、长驻、统一日志的运维场景。**前提**：已通过路径 A 解压发布包，或路径 B 构建得到二进制并组织好目录。

### C.1 前置

- Linux：systemd（绝大多数发行版默认有；WSL1 / 容器极简镜像可能没有）
- Windows：自带 `sc.exe`（任何桌面 / 服务器 SKU 默认有）
- 解压目录或构建目录中必须包含：
  - 主二进制 `frp-easy` / `frp-easy.exe`
  - `scripts/install-service.{sh,ps1}` 与 `scripts/uninstall-service.{sh,ps1}`

### C.2 Linux systemd

#### C.2.1 安装

```bash
cd <INSTALL_DIR>
sudo ./scripts/install-service.sh
```

可选参数：

```bash
sudo ./scripts/install-service.sh --user nobody         # 指定运行用户
sudo ./scripts/install-service.sh --name frp-easy-2     # 自定义 unit 名（多实例）
```

脚本会：

1. 写入 `/etc/systemd/system/frp-easy.service`（权限 `0644`）；
2. `systemctl daemon-reload`；
3. `systemctl enable --now frp-easy`。

重复执行幂等：会刷新 unit 并重启服务。

#### C.2.2 状态查询

```bash
systemctl status frp-easy
systemctl is-active frp-easy     # 单纯查 active / inactive
```

#### C.2.3 查看日志

```bash
journalctl -u frp-easy -f                 # 实时流式
journalctl -u frp-easy --since "10 min ago"  # 最近 10 分钟
```

frpc / frps 子进程的日志另存于 `<INSTALL_DIR>/.frp_easy/logs/frpc.log` 与 `frps.log`；UI 自身日志见 `.frp_easy/logs/ui.log`。

#### C.2.4 升级

```bash
sudo systemctl stop frp-easy
# 解压新版本到当前 <INSTALL_DIR>（覆盖二进制；保留 frp_easy.toml 与 .frp_easy/）
tar xzf frp-easy-<新VERSION>-linux-amd64.tar.gz \
    --strip-components=1 -C <INSTALL_DIR> \
    frp-easy-<新VERSION>-linux-amd64/frp-easy \
    frp-easy-<新VERSION>-linux-amd64/frp_linux
sudo systemctl start frp-easy
```

> 升级时**无需重跑 install-service.sh**（除非要改 unit 字段如 `--user`）。
>
> 说明：若你最初是通过 [A.0 一键安装](#a0-一键安装推荐) 部署的，重跑一键安装命令时脚本
> 会自动重跑服务注册（`install-service.sh`）。这与本节"手动升级无需重跑"并不冲突——
> `install-service.sh` 本身是幂等的，重跑只会刷新 unit 并重启服务，不会破坏配置或数据；
> 一键安装为保证服务定义始终与新版本一致而总是重跑它，是有意为之的安全设计。

#### C.2.5 卸载

```bash
sudo ./scripts/uninstall-service.sh
```

脚本会停服 + 删 unit + daemon-reload；**不删除** `frp_easy.toml` 与 `.frp_easy/`。
如需彻底清理，按脚本结尾提示手动执行 `rm -rf <INSTALL_DIR>/.frp_easy <INSTALL_DIR>/frp_easy.toml`。

### C.3 Windows Service

#### C.3.1 安装

以**管理员身份**打开 PowerShell（右键 PowerShell → 以管理员身份运行），然后：

```powershell
cd <INSTALL_DIR>
.\scripts\install-service.ps1
```

可选参数：

```powershell
.\scripts\install-service.ps1 -DisplayName "FRP 测试"     # 自定义显示名（支持中文）
.\scripts\install-service.ps1 -ServiceName "frp-easy-2"  # 自定义服务键名
```

脚本会：

1. 生成 `<INSTALL_DIR>\frp-easy-svc.cmd` 包装脚本（用于锁定 cwd）；
2. `sc.exe create frp-easy binPath= "<...>\frp-easy-svc.cmd" start= auto`；
3. `sc.exe failure frp-easy reset= 60 actions= restart/5000`（崩溃后 5 秒自动重启）；
4. `sc.exe description` 写入中文描述；
5. `sc.exe start frp-easy`。

重复执行幂等：会刷新 binPath / DisplayName / failure 后重启。

#### C.3.2 状态查询

```powershell
sc query frp-easy
Get-Service frp-easy
```

输出 `STATE: 4 RUNNING` 表示运行中。

#### C.3.3 查看日志

- UI 进程日志：`<INSTALL_DIR>\.frp_easy\logs\ui.log`（实时 tail：`Get-Content <INSTALL_DIR>\.frp_easy\logs\ui.log -Tail 200 -Wait`）
- frpc / frps 子进程日志：`<INSTALL_DIR>\.frp_easy\logs\frpc.log` 与 `frps.log`
- Windows Service 控制器日志：**事件查看器** → Windows 日志 → 系统，过滤来源 `Service Control Manager`

#### C.3.4 升级

以管理员身份打开 PowerShell：

```powershell
sc.exe stop frp-easy
# 解压新版本覆盖到 <INSTALL_DIR>（保留 frp_easy.toml 与 .frp_easy）
Expand-Archive -Force -Path .\frp-easy-<新VERSION>-windows-amd64.zip -DestinationPath .
Copy-Item -Force .\frp-easy-<新VERSION>-windows-amd64\frp-easy.exe <INSTALL_DIR>\frp-easy.exe
Copy-Item -Recurse -Force .\frp-easy-<新VERSION>-windows-amd64\frp_win <INSTALL_DIR>\
sc.exe start frp-easy
```

#### C.3.5 卸载

以**管理员身份**：

```powershell
.\scripts\uninstall-service.ps1
```

脚本会停服 + 删除服务注册项 + 删除 `frp-easy-svc.cmd` 包装；**不删除** `frp_easy.toml` 与 `.frp_easy\`。

---

## 升级

- 路径 A 用户：见 [A.4](#a4-升级)
- 路径 C Linux：见 [C.2.4](#c24-升级)
- 路径 C Windows：见 [C.3.4](#c34-升级)

升级要点（通用）：

1. **始终先停服或停进程**，再覆盖二进制；
2. `frp_easy.toml`、`.frp_easy/`（含 SQLite 数据库与日志）**永不覆盖**；新版本的 schema 迁移由 `storage.Open()` 启动时自动应用；
3. 升级**不需要**重新跑 `install-service.*`（除非要改 unit / service 字段）。

---

## 故障排查

### F.1 端口被占用

**症状**：启动后立即退出，stderr 出现：

```
frp_easy UI 启动失败：端口 7800 已被占用。请关闭占用进程，或编辑 frp_easy.toml 中 UIPort = 7801 后重试。
```

**排查**：

```bash
# Linux
ss -ltnp 'sport = :7800' || lsof -iTCP:7800 -sTCP:LISTEN
```

```powershell
# Windows
Get-NetTCPConnection -LocalPort 7800 -State Listen | Select-Object -Property OwningProcess
Get-Process -Id <上一行 OwningProcess>
```

**修复**：编辑 `frp_easy.toml`，把 `UIPort` 改成其它端口（如 `7801`），保存后重启。`frp-easy` 启动退出码为 `2`（与 `--help` 中"退出码"小节一致）。

### F.2 UI 打不开

**症状**：浏览器访问 `http://127.0.0.1:7800` 报 503 / 连接失败 / 转圈。

**排查**：

```bash
# Linux / macOS
curl -i http://127.0.0.1:7800/api/v1/system/ready
```

```powershell
# Windows
Invoke-WebRequest -Uri http://127.0.0.1:7800/api/v1/system/ready -UseBasicParsing
```

- 返回 `HTTP/1.1 503` + `Retry-After: 2`：启动序列未完成（数据迁移、子进程初始化中），等几秒重试；
- 返回 `Connection refused`：进程未启动，或 `UIBindAddr` 被改成了一个本机不持有的地址，看 stderr 与 `<INSTALL_DIR>/.frp_easy/logs/ui.log`（默认 `0.0.0.0` 监听所有网卡，回环访问总是可达）；
- 返回 200：UI 已就绪，可能是浏览器缓存 / proxy 问题。

**日志位置**：`<INSTALL_DIR>/.frp_easy/logs/ui.log`（权限 `0o600`，仅 owner 可读）。

### F.3 systemd 启动失败

**症状**：`systemctl status frp-easy` 显示 `failed`。

**排查**：

```bash
journalctl -u frp-easy --since "10 min ago"
systemctl cat frp-easy   # 看 unit 里的 ExecStart / User / WorkingDirectory
```

常见原因：

1. **路径变动**：unit 里 `ExecStart` 是绝对路径，若安装服务后移动了解压目录，启动会找不到二进制。解决：把目录移回去，或重跑 `sudo ./scripts/install-service.sh`。
2. **`User` 不存在**：如指定了 `--user nobody` 但系统没该账户。解决：`getent passwd <user>` 校验，重跑 install-service.sh 改 user。
3. **端口被占用**：见 F.1。

### F.4 Windows Service 未启动

**症状**：`sc query frp-easy` 返回 `STATE: 1 STOPPED`，或服务无法启动。

**排查**：

```powershell
sc.exe query frp-easy
sc.exe qc frp-easy           # 查询配置（binPath / start type / DisplayName）
sc.exe qfailure frp-easy     # 查询失败动作
```

事件查看器：**Windows 日志 → 系统**，过滤来源 `Service Control Manager`，关注 Event ID 7000 / 7009 / 7011。

常见原因：

1. **binPath 失效**：解压目录被移动 / `frp-easy-svc.cmd` 被删除。解决：以管理员身份重跑 `.\scripts\install-service.ps1` 刷新 binPath。
2. **frp-easy.exe 被杀毒软件隔离**：检查 Windows Defender 或第三方安全软件，把 `<INSTALL_DIR>` 加入排除列表。
3. **权限不足**：`<INSTALL_DIR>` 所在卷只读 / NTFS 权限错误。

实时日志（与服务并行）：

```powershell
Get-Content <INSTALL_DIR>\.frp_easy\logs\ui.log -Tail 200 -Wait
```

### F.5 frp 子二进制缺失

**症状**：UI 顶部红色横幅 "frpc / frps 二进制未找到"；点"启动 frpc / frps"按钮报错。

**排查**：

```bash
# Linux
ls -la <INSTALL_DIR>/frp_linux/frpc <INSTALL_DIR>/frp_linux/frps
```

```powershell
# Windows
Get-Item <INSTALL_DIR>\frp_win\frpc.exe, <INSTALL_DIR>\frp_win\frps.exe
```

**修复**：

- **在线**：UI 顶部横幅点"一键下载"，frp_easy 会从 GitHub Releases 自动下载并解压到对应目录（T-002 功能）；
- **离线**：手动到 <https://github.com/fatedier/frp/releases> 下载对应平台压缩包，解压后把 `frpc` / `frps`（或 `.exe`）放回 `<INSTALL_DIR>/frp_linux/` 或 `<INSTALL_DIR>/frp_win/`，赋可执行位（Linux: `chmod 0755`）。

UI 顶部横幅会自动消失，按钮重新可点。

---

## 附录：默认端口与配置字段

完整默认端口表与 `frp_easy.toml` 字段说明见 [README.md](../README.md#默认端口表)；本文档不重复维护以免漂移。
