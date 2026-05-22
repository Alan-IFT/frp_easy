# T-012 原始输入

> 用户原话，PM Orchestrator 留档。

用户反馈：现有安装方式（下载 tar.gz → 手动解压 → 梳理版本号目录 → `./frp-easy`）成本高。诉求："有更易使用的方案吗？直接使用 bash 命令，一键安装？自动配置开机启动，即降低用户使用成本，最好是能 bash 一键安装。"

许可证决策（用户已确认）：**采用 MIT 协议**，版权署名用 git 用户名。

## 事实基线（PM 已核查）

- 仓库：`git@github.com:Alan-IFT/frp_easy.git`，GitHub owner = `Alan-IFT`。
- git `user.name` = `Alan_IFT`（MIT 版权行署名用它）。
- `.github/workflows/release.yml` 已存在：push `v*` tag 触发，自动构建并上传 `bin/release/*.tar.gz`（Linux amd64）+ `*.zip`（Windows amd64）到 GitHub Releases，release 名为去掉 `v` 前缀的版本号。
- `scripts/install-service.sh` 已存在：把解压后的 frp-easy 注册为 systemd 服务、enable --now、幂等。`install-service.ps1` 为 Windows 对应。
- `scripts/package.{sh,ps1}` 产出发布包，包内含主二进制 + `frp_linux/`、`scripts/` 等。
- 现有 `docs/DEPLOYMENT.md` 路径 A（下载发布包）/ 路径 C（系统服务）描述的就是当前的"手动多步"流程。

## PM 已定决策（用户授权全权决策，原则：用户体验好 / 符合软件工程标准 / 长期易维护）

1. **新增一键安装脚本 `scripts/install.sh`**（bash，Linux/macOS）：实现 `curl -fsSL <raw-url>/scripts/install.sh | sudo bash` 形态——自动探测 OS/架构、调用 GitHub API 取最新 release、下载并校验、解压到固定安装目录（建议 `/opt/frp-easy`）、复用 `install-service.sh` 配置 systemd 开机自启、打印访问地址。需 `set -euo pipefail`、对各失败点（无 release / 下载失败 / 非 root / 缺 systemd）给中文友好报错。
2. **Windows 对等脚本 `scripts/install.ps1`**：保持项目"每个脚本 .sh/.ps1 成对"的既有约定（start / package / install-service 均成对），实现 PowerShell 一键安装 + 复用 `install-service.ps1` 配置 Windows 服务。
3. **README 与 DEPLOYMENT.md 更新**：把一键安装作为"快速开始"的首选路径置顶；保留手动下载方式作为备选。
4. **添加 MIT 许可证**：仓库根新增 `LICENSE`（MIT 全文，`Copyright (c) 2026 Alan_IFT`）；新增 `NOTICE` 说明随附的 frp 二进制属上游 `fatedier/frp` 的 Apache-2.0；README 许可证章节从"待确定"改为正式 MIT 声明。

下游 agent 不得在未经 PM 的情况下推翻这些决策；有异议在 PM_LOG.md 提阻塞。
