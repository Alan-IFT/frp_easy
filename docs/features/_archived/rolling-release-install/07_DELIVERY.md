# 07 交付 — T-013 rolling-release-install

> Harness 流水线 stage 7 产出。PM Orchestrator 撰写。

## 摘要

T-013 解决"用户不想维护 GitHub Release"的诉求。改造 GitHub Actions：**每次 push 到 main 分支，自动编译并创建/更新一个名为 `rolling` 的滚动发布**；一键安装脚本始终从这个滚动发布下载预编译二进制。维护者此后零手动发版，用户零工具链。

方案选择经 AskUserQuestion 由用户拍板（"自动滚动发布"，对比"仓库内置二进制""源码构建"两个备选）。

## 流水线轨迹

| 阶段 | Agent | 结果 |
|---|---|---|
| 1 需求 | Requirement Analyst | READY FOR DESIGN（14 FR / 17 AC / 12 BC） |
| 2 设计 | Solution Architect | READY（OQ-1：滚动发布为正式 release + install 查 releases/tags/rolling；OQ-2：tag=rolling；OQ-3：concurrency 含 github.ref） |
| 3 闸门 | Gate Reviewer | APPROVED WITH CONDITIONS（6 PASS/2 WARN/0 FAIL，5 条开发期条件） |
| 4 开发 | Developer | 完成，1 处 DESIGN DRIFT（设计预留的退化路径，合法） |
| 5 评审 | Code Reviewer | APPROVED（0 BLOCKER/0 MAJOR/1 MINOR/2 NIT） |
| 6 测试 | QA Tester | PASS（15 闸门 AC 全满足；shellcheck 实装跑通闭环 M-1） |
| 7 交付 | PM | 本文档 |

## 改动文件

- `.github/workflows/release.yml` — 新增 main 分支触发 + concurrency 组 + 3 个滚动发布 step（Move rolling tag / Purge old rolling assets / Publish rolling release）；保留 v* tag 版本化发布并加互斥 if
- `scripts/install.sh` — API 端点 `releases/latest` → `releases/tags/rolling` + 文案
- `scripts/install.ps1` — 对等改动
- `docs/DEPLOYMENT.md` — A.0 增补滚动发布说明、下载地址改 `releases/tag/rolling`
- `README.md` — CI 说明改为两路径描述
- `docs/dev-map.md` — 描述更新

## DESIGN DRIFT 说明

1 处，合法。BC-7（移动 rolling tag）/ BC-8（清理旧资产）由设计 §4.4 首选方案（`softprops/action-gh-release` 的 `clean_release_attachments`）改为设计 §8 R-3 预留的退化方案（`git tag -f` + `gh release delete-asset`）。原因：Developer 对 `action-gh-release@v2.6.2` 做源码级实证，确认 `clean_release_attachments` 在该版本不存在、且 action 不会自动移动已存在 tag。退化方案是设计作者亲笔预留的 plan B，Gate Reviewer 与 Code Reviewer 均认定合法。

## verify_all 结果

```
=== Summary ===
  PASS: 19
  WARN: 0
  FAIL: 0
  SKIP: 0
```

PowerShell 与 Git Bash 双 shell 均 PASS 19。本任务无 .go/.ts 改动，baseline.json 未变。
静态校验：release.yml 经 actionlint 通过；`bash -n scripts/install.sh` 通过；`shellcheck scripts/install.sh`（QA 实装 shellcheck 0.11.0）退出码 0 零告警；install.ps1 PowerShell 解析 0 错误。

## 给用户的提示（需关注）

- **如何让一键安装生效 —— 你只需做一件事：`git push`**。把本地提交推到 GitHub main 分支后，GitHub Actions 会自动编译并生成 `rolling` 滚动发布。之后每次推送 main 都会自动刷新它，你**不需要再打 tag、不需要手动发版**。
- **目前有 4 个本地 commit 未推送**（T-011 `cab136a`、T-012 `1840484`、本任务 T-013，以及归档提交）。push 之后首次 Actions 运行完成（约 2-3 分钟），一键安装命令即可用：
  - Linux/macOS：`curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash`
  - Windows：`irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex`
- **AC-16/AC-17 未在流水线内验证**：CI workflow 的真实运行、滚动发布的真实产出、真实联网一键安装跑通，都需要你 push main 后自行确认（本任务按约定不自动 push、不触发真实 CI）。workflow 已通过 actionlint 静态校验，但首次运行建议你到 GitHub 仓库 Actions 页面看一眼构建是否绿。
- **正式版发布仍可用**：如果将来想发带版本号的正式版，`git tag v0.1.0 && git push origin v0.1.0` 仍会触发版本化发布，与滚动发布互不干扰。

## Insight

- `softprops/action-gh-release@v2`（实测 v2.6.2）**没有** `clean_release_attachments` 输入参数，且对已存在的 release 不会自动把底层 git tag ref 移到新 commit。滚动发布（固定 tag 反复移动 + 资产名含 commit hash 每次不同名）必须自己加 `git tag -f` step 移 tag、加 `gh release delete-asset` step 清旧资产。证据：本任务 04 对 action 的源码级核实。
- GitHub Actions `concurrency.group` 用于滚动发布时必须含 `${{ github.ref }}`；否则 main 分支触发与 `v*` tag 触发会落入同一并发组、`cancel-in-progress` 会让两类发布互相取消。证据：本任务 release.yml 设计。
