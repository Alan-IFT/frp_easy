# 06 Test Report — T-012 one-click-install-and-mit-license

> Harness 流水线 stage 6 产出。QA Tester 独立对抗性验证。
> 上游：01（READY FOR DESIGN）、02（READY）、03（APPROVED FOR DEVELOPMENT）、04（READY FOR REVIEW）、05（APPROVED）。
> 验证环境：Windows 11 + Git Bash（GNU bash 5.2.37 / curl 8.18.0）+ PowerShell 7。本环境**无 shellcheck**。
> 本任务产出全为 shell / PowerShell 脚本 + Markdown + 纯文本许可证文件，不属 verify_all 现有 Go/TS 测试覆盖范围；
> 验证方式以「机械静态检查 + 源码逐条走查 + 端到端对抗复现」为主，不新增 .go/.ts 单元测试（无对应被测代码）。

## 1. 验证范围

17 条 AC（AC-1…AC-9、AC-12…AC-17 为完成闸门；AC-10/AC-11 为 PM 已裁决的「交付后人工验证、非闸门」）
+ 12 条 BC。全部逐条机械/走查验证。

## 2. 机械静态检查结果（Code Review M-1 留给 QA 的缺口，已补做）

| 检查 | 命令 | 结果 |
|---|---|---|
| AC-1 bash 语法 | `bash -n scripts/install.sh` | **PASS** — `BASH_N_PASS install.sh` |
| AC-2 shellcheck | `shellcheck scripts/install.sh` | **环境无 shellcheck**（`which shellcheck` → `NO_SHELLCHECK`）。以 `bash -n` 通过 + 人工走查替代：变量引用全双引号；对「失败属正常」命令（`grep` 无匹配、`tar` 校验、`hostname -I`、`curl` 探测、`systemctl stop`）均显式 `\|\| true` / `2>/dev/null` / `\|\| api_curl_ok=0`；`command -v`/`tar` 检测在 `if` 结构内不触发 `set -e`。无 error 级隐患。 |
| AC-3 ps1 解析 | `pwsh -NoProfile -Command "[ScriptBlock]::Create((Get-Content -Raw scripts/install.ps1))"` | **PASS** — `PS_PARSE_PASS install.ps1` |
| AC-16 verify_all (PowerShell) | `pwsh -File scripts/verify_all.ps1` | **PASS 19 / WARN 0 / FAIL 0 / SKIP 0** |
| AC-16 verify_all (Git Bash) | `bash scripts/verify_all.sh` | **PASS 19 / WARN 0 / FAIL 0 / SKIP 0** |

两 shell 的 verify_all 均与基线（PASS 19）一致，不新增 WARN/FAIL。

## 3. Test plan（17 AC × 验证手段）

| AC | 验证手段 | 结果 |
|---|---|---|
| AC-1 `bash -n install.sh` | 机械执行 | PASS |
| AC-2 `shellcheck` 无 error | 环境无 shellcheck → `bash -n` + 人工走查替代（见 §2） | PASS（替代验证） |
| AC-3 `install.ps1` 解析 | `[ScriptBlock]::Create` | PASS |
| AC-4 `set -euo pipefail` / `$ErrorActionPreference="Stop"` | `grep` | PASS — install.sh L22 / install.ps1 L27 字面命中 |
| AC-5 `-h`/`-Help` 中文用法 + exit 0 + 在依赖检测前 | 端到端运行（非 root、非联网环境） | PASS — 见 §4 对抗 AC-5 |
| AC-6 12 BC 可定位中文 stderr + 非 0 退出（BC-11 退 0） | 源码逐行 `grep` 定位 + 端到端 | PASS — 见 §5 |
| AC-7 不复刻 unit/sc.exe，调 install-service.* | 源码走查 | PASS — install.sh L269 `bash "$SERVICE_SCRIPT"`；install.ps1 L211 `& $svc`；退出码透传 |
| AC-8 API URL 精确字面量 | `grep -cF` | PASS — install.sh L25 / install.ps1 L30 各 1 处整串字面量 |
| AC-9 升级不删 `frp_easy.toml`/`.frp_easy/` | 全文 `grep` + 升级分支走查 | PASS — 见 §4 对抗 AC-9 |
| AC-10 联网 Linux 真实安装 | 交付后人工项（PM Q1 裁决，非闸门） | 见 §4 对抗 AC-10 — 当前无 release，脚本走 BC-4 友好报错（已端到端模拟验证） |
| AC-11 联网 Windows 真实安装 | 交付后人工项（同上，非闸门） | 见 §4 对抗 AC-11 |
| AC-12 `LICENSE` MIT 全文 + 版权行 | 文本比对 | PASS — 21 行 1065 字节，`MIT License` + `Copyright (c) 2026 Alan_IFT`，三关键短语全命中，末尾单换行 |
| AC-13 `NOTICE` frp/Apache-2.0 中文说明 | `grep` | PASS — `fatedier/frp`、`Apache License 2.0`、`Apache-2.0` 命中 5 处 |
| AC-14 README 许可证章节无「尚未确定」 | `grep` | PASS — 「尚未确定/待项目维护者确定/待确定」零命中；L221 已为正式 MIT 声明 |
| AC-15 README 快速开始 + DEPLOYMENT 路径 A 一键安装首选 + 安全提示 + 写死 raw-url | 目视 + `grep` | PASS — README L51「一键安装（推荐）」置顶；DEPLOYMENT A.0（L36）为路径 A 首选，两平台命令 + 两安全提示块（L52/L60）+ macOS 降级 + C.2.4 张力消解（L349）；raw-url 写死（README 2 处 / DEPLOYMENT 4 处） |
| AC-16 verify_all PASS ≥ 19，不新增 WARN/FAIL | 双 shell 执行 | PASS — 见 §2 |
| AC-17 install.* 顶部 ≥5 行中文注释 | 目视 | PASS — install.sh 顶 21 行（L1-21）/ install.ps1 顶 20 行（L1-20）中文注释，覆盖用途/用法/参数/输出/退出码/说明 |

## 4. Boundary tests added（边界/对抗手段说明）

本任务无可植入单元测试的 .go/.ts 被测体；边界验证以下列独立复现手段执行（非开发者自述）：

- 环境变量 `FRP_EASY_INSTALL_DIR` 空字符串 / 纯空白值对 `INSTALL_DIR` 推导的影响。
- 本地 `python http.server` 起 403 服务，验证 `curl -sSL -w '%{http_code}'` 在 4xx 下的退出码与状态码捕获。
- 真实端口无监听 / DNS 不可解析，验证 `curl` 退出码 → `api_curl_ok` 标记 → BC-1 分流。
- 真实 GitHub API 对不存在仓库 `releases/latest` 返回码（404）验证 BC-4 触发条件。
- 受限 PATH 排除依赖工具验证缺依赖分支（受 msys2 bash 共享库依赖限制，辅以源码顺序走查）。
- install.sh / install.ps1 在非 root / 非管理员、非联网环境下的 `-h`/`-Help` 与未识别参数行为。

## Adversarial tests（对抗性测试 — 每条 AC 一个证伪假设）

> 验收基于「实现是否在对抗下存活」，而非开发者自测是否通过。所有复现器由 QA 独立编写（标 NEW）。

| AC | 证伪假设（「我预期失败，当……」） | 复现器（QA 独立编写） | 结果（含工具输出） |
|---|---|---|---|
| AC-1 | install.sh 含 bash 语法错误 / 未闭合 heredoc | `bash -n scripts/install.sh`（NEW） | **存活** — `BASH_N_PASS install.sh`，退出 0 |
| AC-2 | 变量未引号 / `set -e` 陷阱被 shellcheck 标 error | 环境无 shellcheck；改为 `bash -n` 通过 + 逐行人工走查（NEW） | **存活** — 变量全双引号；`grep`/`tar`/`hostname`/`curl`/`systemctl` 五处「失败属正常」均显式兜底；`command -v` 在 `if` 内。无 error 级隐患 |
| AC-3 | install.ps1 含 PowerShell 解析错误 | `pwsh -NoProfile -Command "[ScriptBlock]::Create((Get-Content -Raw scripts/install.ps1))"`（NEW） | **存活** — `PS_PARSE_PASS install.ps1` |
| AC-4 | 缺 `set -euo pipefail` 或仅注释里出现 | `grep -n 'set -euo pipefail' install.sh` / `grep -n 'ErrorActionPreference = "Stop"' install.ps1`（NEW） | **存活** — install.sh L22 顶层语句、install.ps1 L27 顶层语句，非注释 |
| AC-5 | `-h` 落在依赖/root 检测之后 → 非 root 环境会先报权限错而非打印帮助；或退出码非 0 | 在 `id -u != 0` 且无网络的环境直接 `bash install.sh -h` / `--help` / `pwsh -File install.ps1 -Help` / `-h`（NEW） | **存活** — 四种调用均打印完整中文用法（含安装目录/权限/依赖/退出码语义）后退出码 0；未识别参数 `--bogus` 中文报错 + 退出码 1。help 分支（install.sh L30-76 步骤 0 / install.ps1 L34-70）确在依赖检测（步骤 1）之前。PowerShell 的 `-h` 作为 `-Help` 前缀缩写自动命中，满足 In-scope 9「`-Help`（或 `-h`）」 |
| AC-6 | 某 BC 分支无中文 stderr 或退出码为 0（BC-11 除外） | 对 12 条 BC 文案逐条 `grep` 定位 + BC-1/BC-6/BC-7 端到端（NEW，见 §5） | **存活** — 12 BC 全部可定位中文 stderr + 非 0 退出；BC-11 macOS 分支为 exit 0（符合需求） |
| AC-7 | install.sh 内联写 systemd unit / install.ps1 内联调 sc.exe create 复刻服务注册 | `grep` 服务注册逻辑 + 走查 L249/L269、install.ps1 L206/L211（NEW） | **存活** — 仅 `bash "$SERVICE_SCRIPT"` / `& $svc` 调用解压包内副本；失败 `exit "$rc"` / `exit $LASTEXITCODE` 透传，无复刻 |
| AC-8 | API URL 由变量拼接而非整串，`grep -F` 无法精确命中 | `grep -cF 'https://api.github.com/repos/Alan-IFT/frp_easy/releases/latest' install.{sh,ps1}`（NEW） | **存活** — 两脚本各命中 1 处完整字面量，无拼接 |
| AC-9 | 升级分支存在对 `frp_easy.toml` / `.frp_easy/` 的 `rm`/`cp`；或 `rm -rf` 目标可能为空删掉整个安装目录 | 全文 `grep "frp_easy.toml\|.frp_easy"` + `grep "rm -rf\|Remove-Item"` + 升级分支逐行走查（NEW） | **存活（含 1 NIT）** — install.sh `rm -rf` 仅 `$INSTALL_DIR/frp_linux`、`$INSTALL_DIR/scripts` 两个固定子目录后缀，永不为 `$INSTALL_DIR` 本身；升级用白名单逐项 `cp`（frp-easy/frp_linux/scripts/README.txt/VERSION/LICENSE/frp_easy.toml.example），全文对 `frp_easy.toml`/`.frp_easy` 零 rm/cp。`trap 'rm -rf "$TMP_DIR"'`（L189）在 `TMP_DIR` 赋值（L187）后设置，`mktemp` 失败时 `set -e` 先中止、trap 尚未生效。**NIT**：高级用户显式设 `FRP_EASY_INSTALL_DIR="  "`（纯空白，非空串）时 `:-` 不回退默认，`rm -rf "  /frp_linux"` 退化为相对路径——属用户故意提供畸形值的极端情形，且目标始终带 `/frp_linux` 后缀不会删整目录，不阻塞 |
| AC-10 | 当前无 release 时一键安装直接崩溃 / 静默失败而非友好报错 | 本地 `python http.server` 返回 404 + 真实 GitHub API 对不存在仓库 `releases/latest` 取状态码（NEW） | **存活** — 真实 GitHub API 返回 `http_code=404`；install.sh 状态码分流 case 命中 404 分支 → 中文「仓库尚未发布任何 release……」exit 1（BC-4）。真实 `curl\|bash` 联网安装为交付后人工项，需用户先发首个 Release |
| AC-11 | 同 AC-10，Windows 侧 | install.ps1 catch 块 `$statusCode -eq 404` 分支走查（NEW） | **存活** — install.ps1 L115 `$statusCode -eq 404` → 中文报错 exit 1。真实 `irm\|iex` 为交付后人工项 |
| AC-12 | LICENSE 非 MIT 标准全文 / 版权行错误 / 缺末尾换行 | `wc`、`tail -c \| od -c`、`grep` 三关键短语（NEW） | **存活** — 21 行 1065 字节，`Copyright (c) 2026 Alan_IFT`，`Permission is hereby granted` / `WITHOUT WARRANTY OF ANY KIND` / `MERCHANTABILITY` 全命中，末尾单 `\n` |
| AC-13 | NOTICE 未归属 frp 或未提 Apache-2.0 | `grep -c "fatedier/frp\|Apache" NOTICE`（NEW） | **存活** — 命中 5 处，含 `fatedier/frp`、`Apache License 2.0（Apache-2.0）`、`Apache-2.0` 全文链接 |
| AC-14 | README 许可证章节仍含「尚未确定」类字样 | `grep -n "尚未确定\|待项目维护者确定\|待确定" README.md`（NEW） | **存活** — 零命中；README L221 已为正式 MIT 声明 + NOTICE 链接 |
| AC-15 | 一键安装未置首选 / 缺安全提示 / raw-url 为占位符 | `grep` README L51-69 与 DEPLOYMENT L36-83（NEW） | **存活** — 一键安装均置首选；两安全提示块「先下载审阅」齐全；raw-url 全为写死真实地址（README 2 / DEPLOYMENT 4 处） |
| AC-16 | 引入改动后 verify_all 掉 PASS 或新增 WARN/FAIL | `pwsh -File verify_all.ps1` + `bash verify_all.sh`（NEW，双 shell） | **存活** — 两 shell 均 PASS 19 / WARN 0 / FAIL 0，与基线一致 |
| AC-17 | install.* 顶部中文注释 < 5 行 | 目视 install.sh L1-21 / install.ps1 L1-20（NEW） | **存活** — 分别 21 / 20 行中文注释，远超 ≥5 行 |

### 对抗性深挖：API 状态码分流（BC-2 关键风险点）

> 需求 BC-2 明确「不把限流响应当成正常 JSON 解析」。限流 403 响应体本身是合法 JSON——若顺序反了会被当正常 release 处理。

QA 独立提取 install.sh L134-154 的分流逻辑，喂入四组构造响应：

```
[测试A] 403限流(body是合法JSON {"message":"API rate limit exceeded..."}):
  -> 403分支: 限流报错 exit1 (未解析body)        ✓ 先判状态码
[测试B] 404无release:    -> 404分支: 无release报错 exit1   ✓
[测试C] 200正常:         -> 200分支: 继续解析JSON          ✓
[测试D] 500异常:         -> 异常状态500 报错 exit1         ✓
```

端到端：本地 `python http.server` 返回 `403 + 合法 JSON`，`curl -sSL -w $'\n%{http_code}'`
实测 `api_curl_ok=1`（curl 对 4xx 不报错）、`http_code=[403]` 被正确切出。
连接被拒（端口 59999 无监听）`curl` 退出 7、DNS 不可解析退出 6 → 两者均 `api_curl_ok=0` → 正确落 BC-1 网络分支。
**结论：先判 HTTP 状态码、后 JSON 解析的顺序正确，403 限流不会被误当正常 JSON。**

## 5. 12 BC 逐条核验

| BC | 触发条件 | install.sh 定位 | install.ps1 定位 | 退出码 | 结论 |
|---|---|---|---|---|---|
| BC-1 无网络/DNS失败 | curl 网络层失败 | L130「无法访问 GitHub」 | L110 同文案 | 1 | PASS（端到端：curl exit 6/7 → api_curl_ok=0） |
| BC-2 403 限流 | 先判状态码 | L143 限流文案 | L113 | 1 | PASS（见 §4 深挖，先判码后解析） |
| BC-3 非 amd64 | uname -m / PROCESSOR_ARCHITECTURE | L114 架构文案 | L90 | 1 | PASS |
| BC-4 无 release（404） | 状态码 404 | L147「尚未发布任何 release」 | L116 | 1 | PASS（真实 GitHub API 404 已验证） |
| BC-5 缺平台资产 | ASSET_URL 为空 | L174/L177 | L136 | 1 | PASS |
| BC-6 非 root/非管理员 | `id -u`≠0 / IsInRole | L92 | L77 | 1 | PASS（端到端：非 root 实跑命中 L92 exit 1） |
| BC-7 缺 curl/tar | command -v / sc.exe | L82/L87 | L82 | 1 | PASS（源码顺序：curl L81→tar L86→root L91→API L123，依赖检测在 API 前） |
| BC-8 缺 systemctl | 透传 install-service.sh 退出码 | L269-272 `exit "$rc"` | L212-214 `exit $LASTEXITCODE` | 透传（实测 install-service.sh 缺 systemctl 时 exit 1） | PASS（透传不掩盖；见下方 NIT-1 文案张力） |
| BC-9 空/损坏/解压失败 | `-s` / `tar tzf` / `tar xzf` | L198/L203/L208 | L156/L162 | 1 | PASS |
| BC-10 已存在旧安装 | `-e frp-easy` / `Test-Path frp-easy.exe` | L219 升级分支 | L175 | 不退出（继续到步骤 8） | PASS |
| BC-11 macOS | uname=Darwin 且无资产 / 有资产 | L171（无资产 exit 1）/ L251（有资产降级 exit 0） | 不适用 | 0（降级）/ 1（无资产） | PASS（当前 release.yml 无 darwin 资产→走 exit 1 定制文案，预期行为） |
| BC-12 中途失败清理 | trap / finally | L189 `trap 'rm -rf "$TMP_DIR"' EXIT` | L216-217 finally `Remove-Item` | — | PASS（trap 在 TMP_DIR 赋值后设置） |

## 6. verify_all result

- Total tests（基线）：224（Go 167 + Frontend 57），passing 219 — 本任务无 .go/.ts 改动，测试数不变
- PASS checks：19 → 19（PowerShell 与 Git Bash 双 shell 一致）
- FAIL：0
- WARN：0
- SKIP：0
- New tests added：0（本任务产出为 shell/PowerShell 脚本 + Markdown + 纯文本许可证，无对应 .go/.ts 被测体可植入单元测试；边界以独立端到端复现验证，见 §4/§5）
- Baseline updated：**否**（test_count / passing_count 无变化，基线只升不降原则下保持不动；不下调）

## 7. Defects found

- **无 BLOCKER / CRITICAL / MAJOR。**
- **MINOR / NIT（不阻塞，知会 PM）：**
  - **NIT-1 退出码文案张力**：install.sh / install.ps1 的 `-h`/`-Help` 文本称「退出码 2 = 服务注册阶段失败（透传 install-service.sh 退出码）」，但实测 `install-service.sh` 在「缺 systemctl」分支为 `exit 1`（L70），透传后 install.sh 也 `exit 1` 而非 2。需求 AC-7/BC-8 仅要求「透传、不掩盖」，脚本做到了透传，故不违反 AC；仅 help 文本「2」举例与缺 systemctl 实况不完全对应。建议后续润色（非本任务闸门）。
  - **NIT-2 INSTALL_DIR 畸形值**（即 §4 AC-9 行的 NIT）：`FRP_EASY_INSTALL_DIR` 设为纯空白字符串时 `${:-}` 不回退默认。属高级用户故意畸形输入，`rm -rf` 目标始终带固定子目录后缀不会删整目录，不阻塞。
  - **承接 Code Review M-2**：`scripts/install.sh` 在 Windows/NTFS 下开发，Unix 可执行位未保留。PM 在 commit 前执行 `git update-index --chmod=+x scripts/install.sh` 设为 `100755`。`curl\|bash` / `bash install.sh` 形态不依赖该位，不阻塞完成闸门。

## 8. Stability

- `bash -n scripts/install.sh`、`pwsh [ScriptBlock]::Create install.ps1` 解析检查、`install.sh -h` /
  `install.ps1 -Help` 端到端各运行 3 次，结果稳定一致，无 flake。
- `verify_all`（PowerShell + Git Bash）各运行 1 次，结果一致 PASS 19/0/0；T-010/T-011 已分别验证其跨 shell 稳定性。
- API 状态码分流逻辑复现器（四组构造响应）多次运行结果确定，无 flake。

## 9. 交付后人工验证待办（PM 写入 07_DELIVERY.md，非完成闸门）

- AC-10：用户发布首个 GitHub Release 后，在联网 Ubuntu 22.04+ 执行
  `curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash`，
  验证 `systemctl is-active frp-easy` 输出 `active`、UI 地址可打开。
- AC-11：同上，联网 Windows 10 22H2+ 管理员执行 `irm <url>/install.ps1 | iex`，验证 `sc query frp-easy` 显示 `RUNNING`。
- 当前仓库无任何 release，一键安装命令现在执行会走 BC-4 友好中文报错（已验证），符合 PM Q1 裁决。

## 10. Verdict

**PASS（APPROVED FOR DELIVERY）**

- 15 条静态/自动完成闸门 AC（AC-1…AC-9、AC-12…AC-17）全部对抗性验证存活。
- 12 条 BC 全部可定位中文 stderr 报错与正确退出码（BC-11 为 exit 0，符合需求）。
- AC-10/AC-11 为 PM 已裁决的交付后人工验证项，非完成闸门；当前无 release 时脚本走 BC-4 友好报错（已端到端验证）。
- `verify_all` 双 shell 均 PASS 19 / WARN 0 / FAIL 0，与基线持平，不新增 WARN/FAIL。
- 0 BLOCKER / 0 CRITICAL / 0 MAJOR；2 NIT + 承接 1 项 Code Review M-2 commit 事项，均不阻塞。
- 升级红线对抗性证伪未能攻破：全文对 `frp_easy.toml`/`.frp_easy` 零 rm/cp，`rm -rf` 目标永不为整个安装目录。
- API 状态码分流对抗性证伪未能攻破：先判 HTTP 状态码、后解析 JSON，403 限流不会被误当正常响应。
