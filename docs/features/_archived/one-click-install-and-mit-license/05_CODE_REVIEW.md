# 05 Code Review — T-012 one-click-install-and-mit-license

> Harness 流水线 stage 5 产出。Code Reviewer 独立评审，read-only。
> 上游：01/02/03/04。评审方式：实读全部新增/修改文件 + 逐条 17 AC / 12 BC 走查 + 设计忠实度比对 + 手工 shellcheck 走查。

## Verdict：APPROVED（0 BLOCKER / 0 MAJOR / 2 MINOR / 3 NIT）

实现忠实于 01 需求与 02 设计，九大重点核查项全部 PASS，无 DESIGN DRIFT，无红线文件被改。

## 评审文件
新增：`scripts/install.sh`、`scripts/install.ps1`、`LICENSE`、`NOTICE`。
修改：`README.md`、`docs/DEPLOYMENT.md`、`docs/dev-map.md`。

## 九大核查项结论
1. **执行模型** PASS — install.sh 无 `$0`/`BASH_SOURCE`，install.ps1 无 `$PSScriptRoot`；路径基于 mktemp 临时目录 + 固定安装目录。
2. **升级红线** PASS — 白名单逐项覆盖；全文无针对 `frp_easy.toml`/`.frp_easy` 的 rm/cp；`rm -rf` 目标永不为空；先覆盖后调服务。
3. **API 状态码分流** PASS — `curl -sSL -w` 去 `-f`，先判 403/404/网络失败、后解析 JSON；三分支均中文报错 exit 1。
4. **set -e 陷阱** PASS — `command -v`/`tar` 在 if 结构内；grep 无匹配 `|| true`；`hostname -I`/`systemctl stop` 均兜底。
5. **复用** PASS — 调用解压包内 install-service.{sh,ps1}，未复刻 systemd/sc.exe；退出码透传。
6. **MIT LICENSE/NOTICE** PASS — LICENSE 标准 MIT 全文，版权行 `Copyright (c) 2026 Alan_IFT`；NOTICE 正确归属 frp Apache-2.0；README 许可证章节已改正式 MIT。
7. **shellcheck 手工走查** PASS — 变量全双引号、无裸展开、无未处理 cd、set -e 陷阱全覆盖，无 error 级隐患（本环境无 shellcheck 二进制，留 QA 机械补跑）。
8. **文案/退出码** PASS — 12 BC 失败分支均中文 stderr；退出码 0/1/2 与 install-service.* 一致。
9. **红线文件** PASS — 未触碰 `.harness/`、`.claude/`、`CLAUDE.md`、`copilot-instructions.md`、`release.yml`、`install-service.*`、`package.*`、归档文档。

## 17 AC / 12 BC 覆盖
15 条静态 AC 全部满足；AC-10/AC-11 为 PM 已裁决的交付后人工验证项。12 BC 全部有可定位中文报错与正确退出码。无 DESIGN DRIFT。

## Findings
### BLOCKER / MAJOR
无。

### MINOR
- **M-1 AC-2 机械验证缺口**：评审环境无 shellcheck/bash 二进制，AC-1（`bash -n`）/AC-2（`shellcheck`）只能依赖开发者自述 + 人工走查。人工走查无 error 级隐患。**QA 须在带 shellcheck/bash 的环境补跑 `shellcheck scripts/install.sh`、`bash -n scripts/install.sh`，记入 06_TEST_REPORT.md。**
- **M-2 install.sh 可执行位**：设计 §12.18 建议 `install.sh` 提交为 `100755`。Windows/NTFS 不保留可执行位。**PM commit 前执行 `git update-index --chmod=+x scripts/install.sh`。** 不阻塞（curl|bash 形态不依赖该位）。

### NIT（均无需改）
- N-1 install.sh / install.ps1 文件白名单两处独立维护。
- N-2 macOS 资产缺失分支「提示」行归 stderr。
- N-3 install.ps1 用 `sc.exe` 检测类比 BC-7，命名合理。

## 给 PM / 用户的知会
- AC-10/AC-11 需用户先发布首个 GitHub Release 才能真实跑通；当前无 release 时脚本走 BC-4 友好报错。
- NOTICE 不会被 package.sh 打进发布包（本期 Out-of-scope，预期）。
- macOS 一键安装当前走 exit 1 + 定制文案，是预期行为。
