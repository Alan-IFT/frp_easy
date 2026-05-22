# Development Record — T-012 one-click-install-and-mit-license

> Harness 流水线 stage 4 产出。Developer 按 `02_SOLUTION_DESIGN.md` 精确实现。
> 上游：01（READY FOR DESIGN）、02（READY）、03（APPROVED FOR DEVELOPMENT）。

## Summary

按已批准设计实现 frp_easy 一键安装能力：新增 `scripts/install.sh`（bash，Linux/macOS）与
`scripts/install.ps1`（PowerShell，Windows）两个互为对等的一键安装编排脚本，以 `curl|bash` /
`irm|iex` 管道形态运行（探测平台 → 调 GitHub Releases API → 下载校验解压 → 调用解压包内
`install-service.*` 注册服务）。同时补齐仓库根 `LICENSE`（MIT 全文）与 `NOTICE`（上游 frp
二进制 Apache-2.0 归属），并把 README / DEPLOYMENT.md 的安装入口改为「一键安装优先、手动下载备选」。
本任务无 .go/.ts 代码改动，未触碰 `scripts/baseline.json`。

## Files changed

新增：

- `LICENSE` — MIT 许可证标准全文（英文），版权行 `Copyright (c) 2026 Alan_IFT`，LF 换行、无 BOM、末尾留一换行。
- `NOTICE` — 中文说明：frp_easy 自身 MIT；`frp_linux/`、`frp_win/` 下随附 frpc/frps 属上游 `fatedier/frp`，遵循 `Apache-2.0`。
- `scripts/install.sh` — bash 一键安装编排脚本（Linux/macOS）。`#!/usr/bin/env bash` + `set -euo pipefail`；顶部 22 行中文注释（用途/用法两形态/参数/输出/退出码/说明）；8 步流程；`curl|bash` 形态下不用 `$0`/`${BASH_SOURCE[0]}` 自定位，一切路径基于 `mktemp -d` 临时目录与固定安装目录 `/opt/frp-easy` 两个显式绝对路径。
- `scripts/install.ps1` — PowerShell 一键安装编排脚本（Windows）。`$ErrorActionPreference = "Stop"`；顶部中文注释块；安装目录 `C:\Program Files\frp-easy`；不用 `$PSScriptRoot` 自定位。

修改：

- `README.md` — 「快速开始」新增「一键安装（推荐）」小节（curl|bash / irm|iex 命令置顶，raw-url 写死），手动下载降为「或：手动下载发布包（备选）」；「许可证」章节从「尚未确定」改为正式 MIT 声明 + NOTICE 链接。
- `docs/DEPLOYMENT.md` — 路径 A 新增 A.0「一键安装（推荐）」子节（两平台命令 + 两个安全提示块「先下载审阅再执行」 + 与路径 C 关系澄清 + macOS 降级说明）；A.1 标题加「手动安装 — 备选」过渡句；A.4 升级处加一键安装升级说明；C.2.4 加 Gate Review F-2 要求的张力澄清（一键安装重跑 install-service.sh 幂等安全，与「手动升级无需重跑」不冲突）。
- `docs/dev-map.md` — 根文件区登记 `LICENSE` / `NOTICE`；`scripts/` 区登记 `install.{sh,ps1}`；README 条目补 T-012 改动注记。

## verify_all result

- Baseline（实现前）：PASS 19 / WARN 0 / FAIL 0 / SKIP 0
- After changes：PASS 19 / WARN 0 / FAIL 0 / SKIP 0
- Delta：0 新增失败、0 新增 WARN，基线保持。本任务全部产出为 shell/PowerShell 脚本 + Markdown +
  纯文本许可证文件，不属 verify_all 现有检查项覆盖的 .go/.ts；A.1 secrets scan 已确认不误命中
  （install.sh / LICENSE / NOTICE 内无 `(api_key|secret|password|token)[:=]"..."` 形态串）。

额外静态自检（设计 §13 静态 AC）：

- `bash -n scripts/install.sh` → 通过（AC-1）。
- `[ScriptBlock]::Create((Get-Content -Raw scripts/install.ps1))` → 解析通过（AC-3）。
- `bash scripts/install.sh -h` → 退出码 0，输出中文用法（含安装目录/权限/依赖/退出码语义）（AC-5）。
- `bash scripts/install.sh --bogus` → 中文报错 + 退出码 1。
- `pwsh -File scripts/install.ps1 -Help` → 退出码 0，输出中文用法（AC-5）。
- `shellcheck`：本环境未安装，已做 `bash -n` 校验并手工复核（所有变量引用均双引号；对 `grep` 无匹配 / `tar` / `hostname -I` / `curl` 探测等「失败属正常」命令显式 `|| true` / `2>/dev/null` / `|| api_curl_ok=0`）。AC-2 的 shellcheck 走查留给 Code Reviewer（设计 §11 R-5 明确 verify_all 不含 shellcheck，AC-2 为 Code Review 手工走查项）。

## 17 AC 逐条落实

| AC | 落实说明 |
|---|---|
| AC-1 `bash -n install.sh` 通过 | 已本地验证通过。 |
| AC-2 `shellcheck install.sh` 无 error | 本环境无 shellcheck；已 `bash -n` + 手工复核（双引号 + `|| true` 全覆盖），留 Code Reviewer 走查。 |
| AC-3 `install.ps1` 解析通过 | `[ScriptBlock]::Create` 已验证通过。 |
| AC-4 `set -euo pipefail` / `$ErrorActionPreference = "Stop"` | install.sh 第 24 行字面 `set -euo pipefail`；install.ps1 含字面 `$ErrorActionPreference = "Stop"`。 |
| AC-5 `-h`/`-Help` 中文用法 + exit 0 + 依赖检测前 | 两脚本的 help/参数解析（步骤 0）均在前置依赖检测（步骤 1）之前；已验证无 root/无网络下退出码 0。 |
| AC-6 BC-1…BC-12 逐条可定位中文 stderr + 非 0 退出（BC-11 退 0） | 见下方 BC 表。 |
| AC-7 不复刻 unit/sc.exe，调用解压目录内 install-service.* | install.sh 调用 `$INSTALL_DIR/scripts/install-service.sh`；install.ps1 调用 `$InstallDir\scripts\install-service.ps1`，均为解压包内副本，不重写服务注册逻辑。 |
| AC-8 API URL 精确字面量 | 两脚本均含整串字面量 `https://api.github.com/repos/Alan-IFT/frp_easy/releases/latest`，未用变量拼接，可被 `grep -F` 命中。 |
| AC-9 升级不删 `frp_easy.toml` / `.frp_easy/` | 升级分支用白名单逐项覆盖（仅 `frp-easy[.exe]` / `frp_linux`(或 `frp_win`) / `scripts` / `README.txt` / `VERSION` / `LICENSE` / `frp_easy.toml.example`）；脚本中无任何针对 `frp_easy.toml` 或 `.frp_easy` 的 `rm`/`cp` 目标。`rm -rf` 目标均为 `$INSTALL_DIR/<固定子目录>`，永不为空、永不为 `$INSTALL_DIR` 本身。 |
| AC-10 联网 Linux 真实安装 | 集成/人工，非完成闸门（PM Q1 裁决）。脚本 BC-4 友好报错保证当前无 release 时可达。 |
| AC-11 联网 Windows 真实安装 | 同 AC-10。 |
| AC-12 `LICENSE` 存在 + MIT 全文 + 版权行 | 仓库根 `LICENSE`，首段 `MIT License` + `Copyright (c) 2026 Alan_IFT` + 标准全文。 |
| AC-13 `NOTICE` 存在 + frp/Apache-2.0 中文说明 | 仓库根 `NOTICE`，含 `fatedier/frp`、`Apache License 2.0（Apache-2.0）`、`Apache-2.0` 多处。 |
| AC-14 README 许可证章节无「尚未确定」改 MIT | 已改写，全文不再含「尚未确定」「待项目维护者确定」「待确定」。 |
| AC-15 README 快速开始 + DEPLOYMENT 路径 A 一键安装首选 + 安全提示 + 写死 raw-url | README「快速开始」一键安装置顶；DEPLOYMENT A.0 为路径 A 首选子节，含两平台安全提示块；所有命令 raw-url 写死 `https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.{sh,ps1}`。 |
| AC-16 verify_all PASS ≥ 19、不新增 WARN/FAIL | PASS 19 / WARN 0 / FAIL 0，与基线一致。 |
| AC-17 install.* 顶部 ≥5 行中文注释 | install.sh 顶部 22 行中文注释、install.ps1 顶部 21 行中文注释，覆盖用途/用法/参数/输出/退出码/说明，与 install-service.* 风格一致。 |

## 12 BC 逐条落实

| BC | 落实说明 |
|---|---|
| BC-1 无网络 | install.sh：`curl` 命令本身非 0（`api_curl_ok=0`）→ 「无法访问 GitHub（请检查网络或代理）。」exit 1。install.ps1：catch 中 `$_.Exception.Response` 为空 → 同文案 exit 1。 |
| BC-2 API 403 限流 | 先判 `http_code == 403`（install.sh）/ `StatusCode -eq 403`（install.ps1），在 JSON 解析之前 → 限流文案 exit 1，不把限流响应当正常 JSON。 |
| BC-3 非 amd64 | install.sh：`uname -m` 非 `x86_64/amd64` → 架构报错 exit 1。install.ps1：`$env:PROCESSOR_ARCHITECTURE` 非 `AMD64` → 同。 |
| BC-4 无 release（404） | `http_code == 404` / `StatusCode -eq 404` → 「仓库尚未发布任何 release…」exit 1。 |
| BC-5 缺当前平台资产 | 资产 URL 解析为空 → 「最新 release 未包含当前平台…」exit 1（macOS 走定制文案，见 BC-11）。 |
| BC-6 非 root / 非管理员 | install.sh：`id -u != 0` → 中文报错 exit 1。install.ps1：`IsInRole(Administrator)` 为否 → 中文报错 exit 1。 |
| BC-7 缺 tar / curl | install.sh：`command -v curl` / `command -v tar` 缺失 → 带安装提示报错 exit 1。install.ps1：检 `sc.exe` 存在（Windows 下载解压不依赖 curl/tar，用内置 Invoke-WebRequest/Expand-Archive）。 |
| BC-8 缺 systemctl | install.sh 调用解压包内 `install-service.sh`，由后者报缺 systemctl（退出 1）；install.sh `if ! bash "$SERVICE_SCRIPT"` 捕获后 `exit "$rc"` 透传，不掩盖。install.ps1 同理 `exit $LASTEXITCODE`。 |
| BC-9 下载空/损坏/解压失败 | install.sh：`curl -fsSL` 下载失败 / `[[ ! -s ]]` 空文件 / `tar tzf` 校验失败 / `tar xzf` 解压失败，各有独立中文文案 exit 1；trap 清理临时目录。install.ps1：下载异常 / `Length -le 0` / `Expand-Archive` 异常，各有中文文案 exit 1；finally 清理。 |
| BC-10 已存在旧安装 | `[[ -e "$INSTALL_DIR/frp-easy" ]]` / `Test-Path frp-easy.exe` → 走升级分支：先停服 → 白名单逐项覆盖 → 重跑 install-service.*，不退出（继续到步骤 8）。 |
| BC-11 macOS 运行 | install.sh darwin 分支两条均实现：①资产存在 → 下载解压安装 → 跳过 install-service.sh → 打印 `cd /opt/frp-easy && ./frp-easy` 手动启动提示 → exit 0；②资产不存在（当前现实，release.yml 无 darwin-amd64）→ macOS 定制文案 + exit 1（设计 §4.11 裁决）。 |
| BC-12 中途失败清理 | install.sh：`trap 'rm -rf "$TMP_DIR"' EXIT`，trap 在 `TMP_DIR` 赋值之后设置。install.ps1：`try/finally`，finally 中 `Remove-Item -Recurse -Force ... -ErrorAction SilentlyContinue`。 |

## Design drift (if any)

无 DESIGN DRIFT。实现严格遵循 `02_SOLUTION_DESIGN.md` 的步骤 / 失败分支 / 退出码 / 中文文案，
包括 §4.11 的 macOS 双分支裁决、§4.6 的「先判状态码后解析 JSON」、§4.9 的白名单逐项覆盖。

实现细节澄清（非偏离，均在设计授权范围内）：

- install.ps1 升级分支的「先停服」后加了 `Start-Sleep -Milliseconds 500` 给 SCM 一点停服时间。
  `install-service.ps1` 自身已有 `Wait-ServiceStopped` 轮询机制兜底，此 500ms 仅是 install.ps1
  调用 `sc.exe stop` 后到下一步覆盖文件之间的轻量缓冲，不影响幂等性与设计语义。
- install.ps1 的局域网 IP 探测额外过滤了 `127.*` 与 `169.254.*`（链路本地）地址，使打印的
  `http://<本机IP>:7800` 更可能是可用地址；取不到时回退 `<本机IP>` 占位、不影响退出码（设计
  §5.9「取不到不影响退出码」）。

## Open issues for review

- **install.sh 可执行位**：设计 §12.18 建议 `scripts/install.sh` 提交时为 `0755`（与
  `install-service.sh` 一致）。本任务在 Windows 文件系统下开发，Unix 可执行位不被 NTFS 保留；
  `curl|bash` 生产形态本身不依赖文件可执行位，「先下载审阅再 `bash install.sh`」也显式带
  `bash`。请 PM 在 commit 时确认 `git update-index --chmod=+x scripts/install.sh` 设置可执行位
  （Code Reviewer 可用 `git ls-files -s scripts/install.sh` 核对模式为 `100755`）。
- AC-2 的 `shellcheck` 走查：本环境无 shellcheck，已 `bash -n` + 手工复核，建议 Code Reviewer
  在有 shellcheck 的环境补一次 `shellcheck scripts/install.sh` 确认无 error 级告警。

## Dev-map updates

`docs/dev-map.md` 追加/修改：

- 根文件区新增 `LICENSE`、`NOTICE` 两行（T-012 新增）。
- `README.md` 条目补「T-012 快速开始置顶一键安装、许可证改 MIT」注记。
- `scripts/` 区追加 `install.{sh,ps1}（T-012 新增：一键安装编排脚本…）` 一行。

## Verdict

READY FOR REVIEW
