# 01 — Requirement Analysis · T-016 install-progress-and-systemd-unit-fix

> 阶段 1 / 7（Requirement Analyst）。模式：**full**。
> 上游只读输入：`docs/features/install-progress-and-systemd-unit-fix/PM_LOG.md` 中的用户原话与 PM 预扫线索。

---

## 1. 背景与价值

T-012 引入了 `scripts/install.sh` / `scripts/install.ps1` 一键安装；T-013 引入了 `rolling` 滚动发布；T-014 移除了二进制内嵌的 frp，改由 UI 横幅按需下载（与本任务无直接交互）。线上首次真实环境验证暴露两类问题：

1. **用户体验缺口**：当前下载步骤（`scripts/install.sh` L193 `curl -fsSL`、`scripts/install.ps1` L151 `Invoke-WebRequest -UseBasicParsing`）在数十 MB 的发布包下载期间无任何输出，用户在慢速网络下会怀疑脚本已挂起；中文用户尤其反复反馈。
2. **正确性缺口**：在 Ubuntu VM（`curl ... | sudo bash`）实测中，`systemctl enable --now frp-easy` 失败，原因是 unit 文件中 `ExecStart="${BINARY}"` 与 `WorkingDirectory="${INSTALL_DIR}"` 的整体双引号语法被 systemd 拒为 `bad unit file setting`；并且 `install.sh` 把退出码报为 `0`，与 `install-service.sh` 明确写了 `exit 2`（L142-145）矛盾——意味着错误检测/透传路径存在第二个独立 bug，掩盖了真实失败级别。

价值受众：

- 任何首次跑 `curl -fsSL .../install.sh | sudo bash` 的 Linux 用户（当前主要平台，amd64 Ubuntu/Debian/RHEL 系）。
- 任何在 Windows 管理员 PowerShell 跑 `irm ... | iex` 的用户（次要平台，但需保持一致 UX）。
- 维护者：未来排查脚本错误需信任退出码语义。

修复后效益可观察：

- 下载阶段有"已下载/总大小或百分比"反馈；
- 同一 Ubuntu VM 命令链路至 `systemctl is-active frp-easy` 返回 `active`；
- 任意子脚本失败时 install.sh 报告的退出码与子脚本真实退出码一致。

历史相关任务（详见 §7）：T-008、T-012、T-013、T-014。

---

## 2. 功能性需求

按主题分三组：**A. 下载进度显示**、**B. systemd unit 语法修复**、**C. 退出码透传/失败检测 robustness**。

### A. 下载进度显示

- **FR-A.1** `scripts/install.sh` 步骤 5（下载发布包）必须向用户的终端 stderr 或 stdout 输出可见的下载进度反馈，反馈内容必须至少包含以下任一形式：进度条、百分比、已下载字节/总字节、当前速率。
- **FR-A.2** `scripts/install.ps1` 步骤 5（下载发布包）必须向 PowerShell 主机输出可见的下载进度反馈，反馈内容形式同 FR-A.1（其中至少一种）。
- **FR-A.3** 进度反馈必须在交互式终端（TTY / 控制台主机）中可见；在非交互式 / 重定向到管道的场景下，反馈输出形式必须不破坏其余阶段中文文本行的可读性（不能产生大段乱码或冗长的二进制控制序列）。
- **FR-A.4** 进度反馈机制必须不引入新的外部依赖（不引入 `wget`、`aria2`、`pv`、PowerShell module 等）；必须仅基于 `curl`、Bash 内建能力（Linux 路径）与 PowerShell 内建能力（Windows 路径）。
- **FR-A.5** 启用进度反馈后，下载失败的检测与错误分流（网络层失败 → exit 1）与现行实现等价：失败时仍输出明确的中文错误并退出码 1；进度反馈输出不能淹没或替换错误信息。
- **FR-A.6** 进度反馈的输出在用户报错复制粘贴日志时不得产生大量无意义重复行（例：终端宽度内的覆盖式进度条在被重定向到非 TTY 时必须自动降级为不输出或单行/分段输出，避免日志膨胀）。

### B. systemd unit 语法修复

- **FR-B.1** `scripts/install-service.sh` 生成的 systemd unit 文件，`ExecStart=` 与 `WorkingDirectory=` 两行的值在默认安装目录 `/opt/frp-easy`（不含空格）下，必须被 systemd（systemd ≥ 230，覆盖 Ubuntu 20.04+/RHEL 8+）接受为合法 unit 并能成功 `systemctl daemon-reload && systemctl enable --now frp-easy`。
- **FR-B.2** 在用户通过环境变量 `FRP_EASY_INSTALL_DIR` 自定义为含空格的安装路径时，生成的 unit 文件必须仍被 systemd 接受并能成功启动（约束：仅 ASCII 字符与空格；含 shell 元字符如 `$`、` ` `（反引号）、`"`、`\` 的路径属 out-of-scope，见 §5）。
- **FR-B.3** 在 BC-B.1（详见 §4）场景下，`install-service.sh` 必须自检 unit 文件合法性并在 `systemctl daemon-reload` 之前/之后给出明确的中文报错；当 daemon-reload 或 enable --now 失败时，必须以非零退出码退出（具体值由 FR-C 系列约束）。
- **FR-B.4** unit 文件中除 `ExecStart` / `WorkingDirectory` 之外的字段（`Description`、`After`、`Documentation`、`Type`、`User`、`Restart`、`RestartSec`、`StandardOutput`、`StandardError`、`WantedBy`）保持与现行 T-008 实现一致，本任务不调整这些字段。

### C. 退出码透传 / 失败检测 robustness

- **FR-C.1** `scripts/install.sh` 步骤 7 调用 `scripts/install-service.sh` 失败时，`install.sh` 自身退出码必须等于 `install-service.sh` 的真实退出码（当前为 `2`）。具体地，用户看到的"退出码 N"提示中的 N 必须与 `install-service.sh` 内部 `exit N` 语句一致。
- **FR-C.2** `scripts/install.sh` 在任意子命令失败导致退出时，输出的中文错误行中标注的退出码必须等于该子命令的真实退出码；这一行为不依赖于 `set -e` 在 `if` 条件块中的语义差异。
- **FR-C.3** `scripts/install-service.sh` 的 `systemctl enable --now` 失败路径必须保留并强化：除现行的"请查看 journalctl -u <unit>"提示外，必须额外输出至少一条诊断信息，明确指明用户应当如何取到根因（例如：直接打印 `systemctl status <unit> --no-pager` 输出片段，或打印 unit 文件内容路径以便审阅，或打印 `journalctl -u <unit> --no-pager -n 20` 摘要）。具体形式由阶段 2 设计。
- **FR-C.4** `scripts/install.ps1` 步骤 7 调用 `scripts/install-service.ps1` 失败的退出码透传链路也必须满足 FR-C.1 的等价语义：用户看到的退出码与 `install-service.ps1` 真实退出码一致。
- **FR-C.5** 在 `install-service.sh` daemon-reload 成功但 `enable --now` 失败的"半成功"场景下，必须不留下损坏的运行态：unit 文件已落盘是可接受的（便于用户审阅），但失败诊断必须明确告知用户该 unit 已写入 `/etc/systemd/system/<name>.service` 并可手工 `systemctl disable <name>` 清理。

---

## 3. 非功能性需求

- **NFR-1（跨发行版兼容）** 修复后的 install.sh + install-service.sh 必须在以下平台手测通过：Ubuntu 22.04 LTS（systemd 249）、Ubuntu 24.04 LTS（systemd 255）、Debian 12（systemd 252）、RHEL 9 / CentOS Stream 9（systemd 252）。"通过"定义为：完整 8 步走通，`systemctl is-active frp-easy` 返回 `active`。
- **NFR-2（macOS 不回归）** install.sh 在 macOS（Darwin）路径下的"降级提示，退出 0"语义保持不变（install.sh L250-260）。
- **NFR-3（Windows 不回归）** install.ps1 + install-service.ps1 在 Windows 管理员 PowerShell 5.1 与 7.x 下行为不变；唯一允许的变更是 FR-A.2 / FR-C.4 引入的进度显示与退出码透传修复。
- **NFR-4（不破坏一键安装一行式 UX）** `curl -fsSL .../install.sh | sudo bash` 与 `irm .../install.ps1 | iex` 两条推荐用法在用户视角不需要新增任何参数；用户的复制粘贴习惯不变。
- **NFR-5（不引入新外部依赖）** 不引入 `jq`、`wget`、`aria2`、`pv` 等；仍仅依赖 `curl`、`tar`、`bash`、`systemctl`（Linux）与 PowerShell 内建（Windows）。
- **NFR-6（输出仍为中文）** 所有用户可见的进度与错误文本保持中文（与 CLAUDE.md 项目规则一致）。
- **NFR-7（verify_all 不退化）** 修复后 `scripts/verify_all` 必须仍然 PASS:19（与当前 main 一致）；A.1 secrets scan 正则不被新增样例字面量误中。
- **NFR-8（脚本可重入 / 幂等）** 用户重跑同一条安装命令必须仍按"升级"语义工作（保留 `frp_easy.toml` 与 `.frp_easy/`；保留 `frp_linux/` 已下载的 frp 二进制），与 T-014 引入的语义一致。
- **NFR-9（管道形态下 curl 进度不破坏 shell 解析）** 关键约束：`curl|bash` 形态下 install.sh 主体本身正在被 curl 输出到 bash stdin，但**步骤 5 的下载是 install.sh 内部发起的独立 curl 调用**，其进度输出去向终端 stderr/stdout 而非污染 bash stdin（这是物理隔离的，本条约束确认实现不能误把进度往 stdin 写）。

---

## 4. 边界条件

### BC-A（进度显示）

- **BC-A.1** 慢速网络（< 100 KB/s）：进度反馈应持续刷新或周期性输出，避免用户误判挂起；具体刷新频率由阶段 2 设计。
- **BC-A.2** 极快网络（< 1 秒下载完）：进度反馈出现一闪而过或完全不出现都属可接受，不算 FR-A 违反。
- **BC-A.3** `Content-Length` 缺失（GitHub Releases 资产 URL 实测带 Content-Length，但仍需考虑 CDN 边缘异常）：进度反馈必须降级为"已下载字节 + 速率"形式而非崩溃。
- **BC-A.4** 终端宽度极窄（< 40 列）：进度反馈不能让覆盖式进度条破坏布局；可接受降级为简短百分比。
- **BC-A.5** stdout 被重定向（如 `install.sh > install.log`）：进度反馈降级为不写入日志或单行/分段（见 FR-A.6）。

### BC-B（systemd unit）

- **BC-B.1** 默认路径 `/opt/frp-easy`（不含空格、不含元字符）—— 必须接受（FR-B.1）。
- **BC-B.2** 自定义路径含空格如 `/opt/frp easy/v1`—— 必须接受（FR-B.2）。
- **BC-B.3** 自定义路径含 shell 元字符（`$`、` ` ` 反引号、`"`、`\`、控制字符）—— **out-of-scope**，详见 §5。
- **BC-B.4** systemd 老版本（< 230，例：CentOS 7）—— **out-of-scope**：项目早已要求 systemd ≥ 230；install-service.sh 的 daemon-reload/enable 失败时给中文报错即可，不专门兼容。
- **BC-B.5** 已存在的 unit 文件（升级语义）：覆盖写、stop 旧 → 写新 → daemon-reload → enable --now，与现行 install-service.sh L100-105 等价语义保持。
- **BC-B.6** unit 文件已写但 enable --now 失败：见 FR-C.5，留 unit 在盘 + 明确诊断。

### BC-C（退出码透传）

- **BC-C.1** `install-service.sh` 内部 `exit 1`（前置失败：非 root / 缺 systemctl / 二进制缺失 / user 不存在）→ install.sh 必须以 `exit 1` 终止。
- **BC-C.2** `install-service.sh` 内部 `exit 2`（systemctl 调用失败）→ install.sh 必须以 `exit 2` 终止。**当前 bug：实测显示 install.sh 报"退出码 0"，与 install-service.sh L142-145 矛盾**。修复后 install.sh 必须报告 `2`。
- **BC-C.3** `install-service.sh` 因 `set -euo pipefail` 在中间某行因未预见错误意外终止（非显式 exit）：install.sh 必须把该真实非零退出码透传给用户。
- **BC-C.4** install-service.sh 正常返回（exit 0）：install.sh 步骤 7 继续到步骤 8 打印安装结果，与现行一致。

---

## 5. Out-of-scope

- **OOS-1** 不为 install.sh / install.ps1 增加并行/分块下载（aria2 / Range 请求等）；仅修复进度可见性，不优化吞吐。
- **OOS-2** 不为含 shell 元字符（`$`、` ` ` 反引号、`"`、`\`、控制字符、非 ASCII）的安装路径做兼容；用户使用此类路径属未支持配置。
- **OOS-3** 不重写 install-service.sh 的 unit 模板字段集（仅修 `ExecStart` / `WorkingDirectory` 的语法，其余字段保持 T-008 现状）。
- **OOS-4** 不引入对 systemd < 230 的兼容代码；BC-B.4。
- **OOS-5** 不引入 `jq` / `wget` / `aria2` / `pv` 等外部依赖；NFR-5。
- **OOS-6** 不修改 install.sh / install.ps1 的步骤总数（仍为 8 步）与步骤分工；阶段提示文案的字数微调允许。
- **OOS-7** 不修改 release.yml / GitHub Actions 发布产物结构（与 T-013、T-014 已稳定）。
- **OOS-8** 不变更 macOS 路径行为（NFR-2）。
- **OOS-9** 不变更 verify_all 的 19 个检查项数量；仅允许在 A.1 secrets scan 不退化前提下通过（NFR-7）。
- **OOS-10** 不在本任务把 README/DEPLOYMENT 文档大改；仅在 install.sh 内 help 文案如有进度相关项可补一行。文档化任务另开。

---

## 6. 验收标准

每条对应可独立验证的命令或观察。

- **AC-1（对应 FR-A.1, FR-A.3）** 在一台干净 Ubuntu 22.04 VM 上跑 `curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash`，下载阶段（步骤 5）的终端输出包含以下任一可视进度：百分比字符、进度条字符（如 `#`、`=`、`█`、`▓`）、`/` 分隔的字节数、`KB/s` 或 `MB/s` 速率字样。
- **AC-2（对应 FR-A.2）** 在一台 Windows 11 管理员 PowerShell 5.1 与 PowerShell 7.x 各跑一次 `irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex`，步骤 5 主机输出含 PowerShell 进度反馈（任一形式：`Write-Progress` 进度条、百分比、字节数）。
- **AC-3（对应 FR-A.5）** 模拟下载失败（断网或将 ASSET_URL 临时替换为 404 路径）：步骤 5 报错"发布包下载失败，请检查网络后重试。"，install.sh 以 `exit 1` 终止；进度反馈不掩盖此错误。
- **AC-4（对应 FR-B.1）** Ubuntu 22.04 VM 上从干净状态跑完整一键安装后，`systemctl is-active frp-easy` 输出 `active`；`systemctl status frp-easy --no-pager` 不含 `bad unit file setting`。
- **AC-5（对应 FR-B.2）** 在 Ubuntu 22.04 VM 上设 `FRP_EASY_INSTALL_DIR="/opt/frp easy/v1"`（含空格）后跑安装，`systemctl is-active frp-easy` 输出 `active`。
- **AC-6（对应 FR-B.1, NFR-1）** 在 Ubuntu 24.04 / Debian 12 / RHEL 9 三个发行版各跑一遍 AC-4，全部 `active`。
- **AC-7（对应 FR-C.1, BC-C.2）** 人为破坏 binary（如 `chmod 000 /opt/frp-easy/frp-easy` 后重跑 install-service.sh）让 enable --now 必败：install.sh 报告的"退出码 N"中 N == `2`（与 install-service.sh L143 一致）。
- **AC-8（对应 FR-C.3）** install-service.sh 启动失败场景下，stderr/stdout 包含至少一条 systemctl status 或 journalctl 的实际输出片段（不仅是"请查看 journalctl"提示）。
- **AC-9（对应 FR-C.4）** Windows 上故意让 install-service.ps1 失败（例：临时删除 frp-easy.exe 后再单独跑），install.ps1 报告的 `$LASTEXITCODE` 与 install-service.ps1 实际退出码一致。
- **AC-10（对应 NFR-2）** macOS（Darwin）路径下 install.sh 仍打印 `==> [7/8] macOS 不支持 systemd 服务化，跳过服务注册` 并以 `exit 0` 收尾，与现行一致。
- **AC-11（对应 NFR-7）** `scripts/verify_all` 输出 `PASS:19`，与当前 main HEAD 一致。
- **AC-12（对应 NFR-8）** 已安装一次后再跑一次一键安装命令：步骤 6 走"升级"分支，`frp_easy.toml` 与 `.frp_easy/` 与 `frp_linux/` 未被修改（mtime 或 sha256 不变）；服务最终仍 `active`。
- **AC-13（对应 FR-B.3, FR-C.5）** 当 enable --now 失败时，install-service.sh stdout/stderr 包含 unit 文件绝对路径（让用户能 `cat /etc/systemd/system/frp-easy.service` 审阅），且指明可用 `systemctl disable frp-easy` 清理 symlink。

---

## 7. 相关历史

从 `docs/tasks.md` 与 `.harness/insight-index.md` 扫描出的强相关任务：

- **T-008 deploy-kit**（`docs/features/_archived/deploy-kit/`）— 首次落地 `install-service.sh` 与 unit 模板；insight-index 第 18 行"systemd unit `ExecStart=${PATH}` 与 `WorkingDirectory=${PATH}` 必须用双引号包路径"来源即此任务的 Code Review MAJOR-1。**本任务的线上证据反驳该 insight**：默认路径下整体双引号反而触发 `bad unit file setting`。本任务收尾时必须更新 / 替换该 insight 行，给出 systemd unit 转义 / 引用的精确正确范式（systemd 实际语法详情属阶段 2 设计）。
- **T-012 one-click-install-and-mit-license**（`docs/features/_archived/one-click-install-and-mit-license/`）— `scripts/install.sh` / `scripts/install.ps1` 主体引入。本任务在其上修步骤 5（进度）与步骤 7（退出码透传）。
- **T-013 rolling-release-install**（`docs/features/_archived/rolling-release-install/`）— `rolling` 滚动 tag、release.yml 调整、API 查询语义稳定。本任务不动 release.yml 与 GitHub API 查询路径。
- **T-014 frp-binary-auto-download**（`docs/features/_archived/frp-binary-auto-download/`）— 升级语义保留 `frp_linux/` 与 `frp_win/` 的策略。本任务的 NFR-8 / AC-12 必须维持此契约。
- **T-008 / 2026-05-19 insight**（"`curl|bash` / `irm|iex` 管道形态下脚本无磁盘路径"）— install.sh 走管道形态时不能依赖 `$0` / `${BASH_SOURCE[0]}` / `$PSScriptRoot`；进度显示设计阶段需注意 install.sh 主体（管道）vs install-service.sh（已落盘）的区分。

---

## 8. 待定问题（PM 已默认决策）

按用户指示"不要停下来问问题"，所有歧义点已由 RA 在最合理范围内做出默认决策并列出供事后回退。每条标注 `[PM 默认]`。

1. **进度反馈具体形式**：(a) 进度条字符 / (b) 百分比 / (c) 字节数 + 速率 / (d) 上述组合。**[PM 默认 = (d)：在交互式终端展示组合形式，非交互式重定向时降级为不输出或单行**。具体实现选择留待阶段 2。
2. **install-service.sh 失败时是否自动打印 `systemctl status` 与 `journalctl` 摘要**：(a) 仅 status 一句、(b) status + journalctl 最近 20 行、(c) 不自动打印仅给命令提示。**[PM 默认 = (b)：status --no-pager + journalctl -u <unit> --no-pager -n 20**。让用户报错时一次粘贴即可调试。
3. **systemd unit `ExecStart` / `WorkingDirectory` 修复策略**：(a) 默认路径走 unquoted、含空格路径用 systemd 自身的转义语法、(b) 始终 unquoted 但拒绝含空格的自定义路径、(c) 用 systemd 5.0+ `ExecStart=` 的特殊引用语法。**[PM 默认 = (a)：unquoted 处理默认路径 + systemd 自身的转义对含空格路径**。具体语法细节属阶段 2 设计责任，本 RA 只约束行为（FR-B.1、FR-B.2）。
4. **install.sh 步骤 7 退出码透传修复策略**：(a) 用 `bash "$SERVICE_SCRIPT"; rc=$?` 直行接 `if [[ $rc -ne 0 ]]; then`、(b) 把 `if !` 形式改为不丢失 `$?`、(c) 全局 `set -e` + `trap ERR` 捕获。**[PM 默认 = (a)：最直接、最可读；和 install.ps1 的 `$LASTEXITCODE` 检查保持对称**。
5. **进度反馈是否同时引入到 install-service.sh / install-service.ps1**：(a) 是、(b) 否（这两个脚本本身没有大文件下载步骤）。**[PM 默认 = (b)：install-service.* 没有耗时下载，只有 daemon-reload + enable --now，无需引入进度**。
6. **是否在 install.sh / install.ps1 的 `--help` 文案中加一行"步骤 5 会显示下载进度"说明**：(a) 加、(b) 不加。**[PM 默认 = (b)：进度本身是 UX 改进而非 CLI 契约，不需要在 help 中专门声明**。

---

## 9. Verdict

**READY**

理由：所有歧义点已由 PM 默认决策固定为可执行的需求约束（§8）；§2 / §3 的每一条 FR / NFR 均在 §6 有对应可观察的 AC；§4 边界、§5 OOS 已明确切边。阶段 2（Solution Architect）可在此基础上做技术选型与方案设计。
