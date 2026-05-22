# 07 交付 — T-014 frp-binary-auto-download

> Harness 流水线 stage 7 产出。PM Orchestrator 撰写。

## 摘要

T-014 完成两件事：

1. **frp 二进制不再 git 内置、改为运行时下载最新版** —— 从 git 移除 `frp_linux/`、`frp_win/` 下内置的 8 个 frp 文件（frpc/frps 可执行文件 + 上游 toml 样例）；发布包不再打包 frp 二进制；`internal/downloader` 从"下载写死版本"改为查询 `fatedier/frp` 的最新 release 并下载。
2. **更新可发现性 + 升级路径修复** —— install.sh/install.ps1 安装成功输出新增"更新"说明（重跑一键安装即升级）；并修复升级路径——升级时不再 `rm -rf frp_linux/`，保留用户运行时下载的 frpc/frps。

## 流水线轨迹

| 阶段 | Agent | 结果 |
|---|---|---|
| 1 需求 | Requirement Analyst | READY（14 FR / 13 闸门 AC + 3 MV / 8 out-of-scope） |
| 2 设计 | Solution Architect | READY（OQ-1~4 裁定；downloader 加 resolveLatestAsset；升级路径显式修复） |
| 3 闸门 | Gate Reviewer | APPROVED FOR DEVELOPMENT（8 维 7 PASS/1 WARN；F-1 package.ps1 补行号） |
| 4 开发 | Developer | 完成，无 DESIGN DRIFT |
| 5 评审 | Code Reviewer | APPROVED（0 BLOCKER/0 MAJOR/2 MINOR/1 NIT；M-1/M-2 已修复） |
| 6 测试 | QA Tester | PASS（13 AC 对抗性证伪全存活；QA 补 3 个边界测试） |
| 7 交付 | PM | 本文档 |

## 改动 / 删除文件

**git 删除（8 个）**：`frp_linux/{frpc,frps,frpc.toml,frps.toml}`、`frp_win/{frpc.exe,frps.exe,frpc.toml,frps.toml}`。`frp_linux/LICENSE`、`frp_win/LICENSE` 保留作目录占位锚点。

**改动**：`.gitignore`、`internal/downloader/downloader.go`、`internal/downloader/downloader_test.go`、`scripts/package.sh`、`scripts/package.ps1`、`scripts/install.sh`、`scripts/install.ps1`、`NOTICE`、`README.md`、`docs/DEPLOYMENT.md`、`docs/dev-map.md`、`scripts/baseline.json`。

**新增**：`internal/downloader/downloader_adversarial_test.go`（QA 补的 3 个边界测试）。

## verify_all 结果

```
=== Summary ===
  PASS: 19
  WARN: 0
  FAIL: 0
  SKIP: 0
```

PowerShell 与 Git Bash 双 shell 均 PASS 19。Go 测试 167→174（downloader 新增 4 个 latest 解析测试 + QA 3 个对抗性边界测试），baseline.json 已同步（go_tests 174 / test_count 231）。Playwright e2e（C.1）确认不因移除内置 frp 二进制而失败。

## 给用户的提示（需关注）

- **frp 二进制现在怎么来**：frp_easy 启动后，若检测到 frpc/frps 缺失，UI 顶部会出现下载横幅，**点一下即从 fatedier/frp 官方仓库下载最新版**。这是"app 帮你下载"而非"手动放文件"。需要说明的是：当前实现是"打开 UI → 点横幅下载"，而非"启动即静默自动下载"——后者会增加启动失败面、且离线时会拖慢启动，故按软件工程标准选择了前者。若你希望"打开就已就绪、连横幅都不用点"，可作为后续小任务。
- **离线场景**：不联网的机器装不到 frp，需手动到 fatedier/frp 下载 frpc/frps 放进 `frp_linux/`（或 Windows 的 `frp_win/`）—— DEPLOYMENT.md F.5 有说明。这是"不内置二进制"的可接受代价。
- **更新方式已写进安装提示**：用户一键安装后，install.sh/install.ps1 的成功输出现在会明确告诉用户"重新运行一键安装命令即可更新，配置与数据保留"。
- **⚠️ 一个长期风险（R-2）**：现在下载的是 frp 的"最新版"，版本不再受 frp_easy 控制。将来若 frp 发布大版本改动配置文件（TOML）格式，frp_easy 生成的 frpc.toml/frps.toml 可能被新版 frp 拒绝、导致 frpc/frps 启动失败。本期未做版本适配。如果将来出现这种情况，需要一个后续任务来跟进 frp 的格式变化（或锁定一个已知兼容的 frp 版本）。

## Insight

- 改为下载 frp "latest" 后，frp 版本不再受 frp_easy 控制。未来 frp 大版本若变更 TOML schema，`internal/frpconf` 渲染的 frpc.toml/frps.toml 可能被新版 frpc/frps 拒绝、导致子进程启动失败。本期按 out-of-scope 不做版本适配；后续若出现兼容性问题，需引入 frp 版本探测/适配或锁定已知兼容版本。证据：T-014 设计 §5 R-2。
- GitHub API 查询 `fatedier/frp` releases 必须带 `User-Agent` 头，否则被 GitHub 拒为 403；且 `http.Client`（不同于 curl）对 4xx/5xx 不返回 error，可天然"先判 resp.StatusCode 再解析 JSON"。证据：T-014 downloader.go resolveLatestAsset。
