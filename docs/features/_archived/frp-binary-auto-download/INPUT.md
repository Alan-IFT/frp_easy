# T-014 原始输入

> 用户原话与 PM 决策，PM Orchestrator 留档。

## 用户原话

1. frp 的二进制文件，不要项目内置，而是自动从官方 git 仓库 release 下载最新版本。
2. 更新功能能用，但用户不知道，你来改动就可以了。
3. 以用户体验好、符合软件工程标准、长期易使用易维护为原则来决策；你来决策，用户只看结果是否符合需求；所有 commit 由 PM 操作。

## 事实基线（PM 已核查）

- 仓库当前 git 内置 frp 二进制：`frp_linux/{frpc,frps,frpc.toml,frps.toml,LICENSE}`、`frp_win/{frpc.exe,frps.exe,frpc.toml,frps.toml,LICENSE}`。
- `internal/downloader/downloader.go` 已有 frp 二进制下载能力（T-002），但**下载的是写死的固定版本**（`FRPVersion` 常量，注释"Verified by running frp_win/frpc.exe --version on the vendored binary"），下载源 `https://github.com/fatedier/frp/releases/download`。
- `scripts/package.sh` 把 `frp_linux/`、`frp_win/` 打进发布包，且**前置检查这些二进制存在、缺失即 exit 1**（L117-127）。
- `internal/binloc/` 按 `runtime.GOOS` 在 `frp_linux/` 或 `frp_win/` 定位 frpc/frps，`Missing()` 反馈缺失项；UI 检测到缺失时弹下载横幅（T-002）。
- 仓库根 `NOTICE`（T-012 新增）正文写"`frp_linux/` `frp_win/` 下随附的 frpc/frps 二进制属上游 fatedier/frp"——若不再内置，NOTICE 需改写。
- `install.sh` / `install.ps1` 安装成功的输出含"常用命令""卸载"，**无"更新"说明**；DEPLOYMENT.md 一键安装小节亦无。更新机制本身可用（重跑一键安装命令即触发升级路径，保留 frp_easy.toml 与 .frp_easy/）。

## PM 已定决策（用户授权全权决策，原则：用户体验好 / 符合软件工程标准 / 长期易维护）

### A 块：frp 二进制不再内置、改为运行时自动下载最新版
1. 从 git 移除内置 frp 二进制（`frpc`/`frps`/`frpc.exe`/`frps.exe`）；`package.sh` 不再打包 frp 二进制、移除对应前置检查。
2. `internal/downloader` 从"下载写死版本"改为"下载 fatedier/frp 的**最新** release"。须处理 frp release 资产命名、平台/架构匹配、GitHub API 限流与失败降级。
3. 首启体验：复用现有 downloader 机制（横幅 + 进度，T-002 已测）。是否在首启自动触发下载（免点横幅）由 Solution Architect 定夺，原则：启动不可被下载阻塞、离线时 frp_easy 仍能正常启动（仅 frpc/frps 暂不可用）。
4. 离线场景降级为"文档化的手动放置 frp 二进制"（DEPLOYMENT.md F.5 已有手动下载说明）——这是"不内置"决策的可接受代价。
5. `frp_linux/`/`frp_win/` 目录作为下载落地目标保留；目录占位方式（`.gitkeep` / `.gitignore` 忽略二进制）、frp 自带的 `LICENSE`/`frpc.toml`/`frps.toml` 文件去留，由 Architect 定夺。
6. 同步改写 `NOTICE`（不再"随附"frp，改为"运行时从 fatedier/frp 下载，遵循其 Apache-2.0"）、README、DEPLOYMENT.md、dev-map.md。

### B 块：更新功能可发现性
7. 在 `install.sh` / `install.ps1` 安装成功输出中新增"更新"说明（重新运行一键安装命令即可，配置与数据保留）；DEPLOYMENT.md 一键安装小节 + README 同步补一句。

下游 agent 不得在未经 PM 的情况下推翻这些决策；有异议在 PM_LOG.md 提阻塞。
