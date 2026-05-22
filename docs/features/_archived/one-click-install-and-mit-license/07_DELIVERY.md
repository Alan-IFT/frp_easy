# 07 交付 — T-012 one-click-install-and-mit-license

> Harness 流水线 stage 7 产出。PM Orchestrator 撰写。

## 摘要

T-012 解决用户反馈的"安装成本高"问题（原流程：下载 tar.gz → 手动解压 → 进版本号目录 → `./frp-easy`），并完成 MIT 许可证决策：

1. **一键安装脚本** —— 新增 `scripts/install.sh`（Linux/macOS）与 `scripts/install.ps1`（Windows），实现 `curl | bash` / `irm | iex` 形态：自动探测平台、取 GitHub 最新 Release、下载校验、解压安装到固定目录、复用 `install-service.*` 配置开机自启、打印访问地址。已安装则按白名单升级，绝不动用户的 `frp_easy.toml` 与 `.frp_easy/`。
2. **MIT 许可证** —— 新增仓库根 `LICENSE`（MIT 标准全文，`Copyright (c) 2026 Alan_IFT`）与 `NOTICE`（说明随附 frp 二进制属上游 fatedier/frp 的 Apache-2.0）。README 许可证章节从"待确定"改为正式 MIT。
3. **文档** —— README 快速开始把一键安装置顶为首选路径；DEPLOYMENT.md 新增 A.0 一键安装小节（含安全提示 + "先下载审阅再执行"备选）。

## 流水线轨迹

| 阶段 | Agent | 结果 |
|---|---|---|
| 1 需求 | Requirement Analyst | READY FOR DESIGN（17 FR / 5 NFR / 17 AC / 12 BC；5 开放问题经 PM 裁决） |
| 2 设计 | Solution Architect | READY（单 Developer 分区；curl\|bash 执行模型禁自定位；API 状态码分流） |
| 3 闸门 | Gate Reviewer | APPROVED FOR DEVELOPMENT（8 维全 PASS，5 条 INFO 不阻塞） |
| 4 开发 | Developer | 完成，无 DESIGN DRIFT（首次派发因额度中断，重派后从零完成） |
| 5 评审 | Code Reviewer | APPROVED（0 BLOCKER / 0 MAJOR / 2 MINOR / 3 NIT） |
| 6 测试 | QA Tester | PASS（17 AC 逐条对抗性证伪全部存活；双 shell verify_all PASS 19） |
| 7 交付 | PM | 本文档 |

## 新增 / 改动文件

**新增**
- `LICENSE` — MIT 标准全文
- `NOTICE` — frp 二进制 Apache-2.0 归属说明
- `scripts/install.sh` — bash 一键安装（Linux/macOS）
- `scripts/install.ps1` — PowerShell 一键安装（Windows）

**改动**
- `README.md` — 快速开始一键安装置顶；许可证章节改为正式 MIT
- `docs/DEPLOYMENT.md` — 新增 A.0 一键安装小节；A.1~A.3 降级为备选；升级措辞张力澄清
- `docs/dev-map.md` — 登记新增脚本与许可证文件

## verify_all 结果

```
=== Summary ===
  PASS: 19
  WARN: 0
  FAIL: 0
  SKIP: 0
```

PowerShell 与 Git Bash 双 shell 均 PASS 19。本任务无 .go/.ts 代码改动，baseline.json 未变。
补充机械验证：`bash -n scripts/install.sh` PASS；`pwsh [ScriptBlock]::Create install.ps1` 解析 PASS（环境无 shellcheck，AC-2 以 bash -n + 人工走查替代）。

## 给用户的提示（需关注）

- **一键安装命令（发布 Release 后可用）**：
  - Linux/macOS：`curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash`
  - Windows（管理员 PowerShell）：`irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex`
- **必须先发布 GitHub Release，一键安装才能真正跑通** —— 一键脚本从 GitHub Releases 下载发布包；仓库目前无任何 Release。发布方式：`git tag v0.1.0 && git push origin v0.1.0`，已有的 `.github/workflows/release.yml` 会自动构建并上传发布包。在此之前运行一键安装会得到"未找到 Release"的友好中文报错（已验证）。AC-10/AC-11 的真实联网安装验证属交付后人工步骤。
- **macOS 当前走源码构建** —— 发布流水线只产 Linux/Windows 包，macOS 用户运行一键安装会得到"无 macOS 专用包，请用源码构建"的友好提示。若需 macOS 一键安装，需后续任务让发布流水线增产 darwin 包。
- **NOTICE 未进发布包** —— 本期未改 `package.sh`，发布包内暂不含 `NOTICE` 文件（仅含 `LICENSE`）。从 Apache-2.0 合规严谨角度，建议后续让 `package.sh` 一并打包 `NOTICE`。
- **curl|bash 安全性** —— 该模式把远程脚本直接交给 root 执行，是业界惯例（rustup/Homebrew 同款）。DEPLOYMENT.md 已提供"先 `curl -o` 下载、审阅后再执行"的谨慎备选。

## Insight

- `curl|bash` / `irm|iex` 管道形态下脚本无磁盘路径，禁用 `$0`/`${BASH_SOURCE[0]}`/`$PSScriptRoot` 自定位；正确做法是一切路径锚定"固定安装目录 + mktemp 临时目录"两个显式绝对路径，被复用的子脚本（install-service.*）则因是磁盘文件可正常自定位。证据：本任务 install.sh/install.ps1 设计。
- GitHub API 未认证请求的限流响应（HTTP 403）响应体是合法 JSON；查询 release 必须"先判 HTTP 状态码、后解析 JSON"，且查询步骤不能用 `curl -f`（否则 403/404 直接变 curl 错误，丢失分流能力）。证据：本任务 install.sh API 步骤。
