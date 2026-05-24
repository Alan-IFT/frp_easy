# 01 Requirement Analysis — T-035 install-sh-role-cli-arg-passthrough

> 角色：Requirement Analyst | 模式：full | 上游：`PM_LOG.md` + 用户原报告
> 红线：本文不做技术选型 / 不做实现决策（架构师产出）。仅明确"什么是对、什么是错、用什么验证"。
> 用户授权完全自主决策（OQ 默认 `[PM-resolved]`）。

---

## §0 目标（Goal）

一句话：让 README 推荐的"Linux 一键安装"入口在 **Ubuntu 24+/26+/Debian 13+ 等带较新 sudo（含 `Defaults env_reset` 默认策略 + 限制 `-E` 的发行版）**上一次成功安装客户端 / 服务端 frp_easy，不再因 `sudo -E` 被忽略导致 `FRP_EASY_ROLE` 丢失而进入"必须指定 FRP_EASY_ROLE"错误路径。

---

## §1 现状与触发场景（根因证据链）

### 1.1 用户原报告

```
alan@alan-911:~$ curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | FRP_EASY_ROLE=client sudo -E bash
sudo: preserving the entire environment is not supported, '-E' is ignored
错误：必须指定 FRP_EASY_ROLE=server|client（不允许静默默认）
  服务端（公网 VM）：curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | FRP_EASY_ROLE=server sudo -E bash
  客户端（内网设备）：curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | FRP_EASY_ROLE=client sudo -E bash
  说明：sudo 需 -E 才能透传环境变量；服务端默认监听 0.0.0.0，客户端默认监听 127.0.0.1。
curl: (23) Failure writing output to destination, passed 1378 returned 1273
alan@alan-911:~$
```

环境：Ubuntu 26 LTS（`alan@alan-911`）。

### 1.2 错误链分解

1. shell 解析 `FRP_EASY_ROLE=client sudo -E bash` —— `FRP_EASY_ROLE=client` 作为 sudo 进程的临时环境前缀。
2. `sudo -E` 试图保留所有调用者环境到子进程。**Ubuntu 26 LTS 的 sudo（sudo 1.9.16+ 或 distro 自定义安全策略）打印 `sudo: preserving the entire environment is not supported, '-E' is ignored`**——`-E` 被忽略。
3. 即便 `-E` 不被忽略，sudo 默认 `env_reset` + `env_keep` 白名单不含 `FRP_EASY_ROLE`，自定义 env 不在白名单中**本就会被剥离**——这一层 sudo 配置门槛由发行版而非用户控制。
4. sudo 启动的 bash 进程的 `$FRP_EASY_ROLE` 为空。
5. install.sh L101-L107 校验 `ROLE` 为空 → exit 3，stderr 打印中文错误。
6. install.sh 退出 → curl stdout 管道下游消失 → curl 报 `(23) Failure writing output to destination`。

### 1.3 影响面（穷举要改的位置）

| # | 文件 | 行 | 内容 | 改动类别 |
|---|---|---|---|---|
| 1 | `scripts/install.sh` | L11-L13（脚本头注释）| 推荐入口字串 `... FRP_EASY_ROLE=server sudo -E bash` ×2 | 文案 + 行为 |
| 2 | `scripts/install.sh` | L52-L57（`--help` 推荐入口段）| 同款字串 ×3（含谨慎用户路径）| 文案 |
| 3 | `scripts/install.sh` | L101-L107（环境变量校验失败时的错误提示）| 提示用户再次用 sudo -E bash 的字串 ×2 + "sudo 需 -E 才能透传环境变量" 解释行 | 行为 + 文案 |
| 4 | `scripts/install.sh` | L530（客户端横幅"更新"段）| `FRP_EASY_ROLE=client sudo -E bash` | 文案 |
| 5 | `scripts/install.sh` | L574（服务端 IP 探测失败兜底）| `FRP_EASY_PUBLIC_IP=<your-ip> FRP_EASY_ROLE=server sudo -E bash` | 文案 |
| 6 | `scripts/install.sh` | L589（服务端横幅"更新"段）| `FRP_EASY_ROLE=server sudo -E bash` | 文案 |
| 7 | `scripts/install.sh` | L406（强制覆盖 role 时的提示）| `FRP_EASY_ROLE=${ROLE} FRP_EASY_FORCE_ROLE=yes sudo -E bash ...` | 文案 |
| 8 | `README.md` | L59 / L62 / L65 / L92 | 一键安装两条 + sudo -E 解释段 + 公网 IP 兜底命令 | 文案 + 行为约定 |
| 9 | `docs/DEPLOYMENT.md` | L44 / L47 / L50 / L52 | 同 README，A.0 一键安装段 | 文案 + 行为约定 |
| 10 | `internal/httpapi/handlers_system.go` | L285（注释引用历史）| "此前该 env 只在 install.sh / install.ps1 安装期被读取" | 不改（注释仅引用历史，不引用入口字串）|

### 1.4 与 install.ps1（Windows）的对照

Windows 侧 `install.ps1` 通过 `irm \| iex` 入口 + `param([switch]$Help)` 拿不到角色信息——历史上 Windows 路径**不区分 server / client**（README L81：默认绑定 `0.0.0.0`，与历史行为一致）。本任务**不**扩展 install.ps1 到 role-aware（OOS-1）。修复仅作用于 Linux/macOS `install.sh`。

---

## §2 根因假设矩阵

| # | 候选根因 | 证据强度 | 证伪步骤 |
|---|---|---|---|
| R1 | `sudo -E` 在 Ubuntu 26 LTS（及任何 sudo 编译时开启 `--with-secure-path` + 启用了限制 `-E` 的安全策略的发行版）上被忽略 → 用户传入的 `FRP_EASY_ROLE` env 丢失 | **极强**（用户实测复现 + sudo 显式打印 `'-E' is ignored`）| 已被用户日志直接证伪可疑（即假设已成立）|
| R2 | 即便 `-E` 工作，sudo `env_keep` 默认白名单不含 `FRP_EASY_ROLE`，发行版默认会剥离自定义 env | **强**（man sudoers + Ubuntu / Debian / RHEL 默认 sudoers 配置）| 在 Ubuntu 旧版（22 / 24 LTS）尝试 `sudo -E env \| grep FRP_EASY_ROLE`，命中即 R2 假，未命中即 R2 真 |
| R3 | 用户的 sudoers 是 NOPASSWD 但 `-E` 仍被拒；本机配置个别问题 | 弱（用户报告明确"sudo: preserving the entire environment is not supported, '-E' is ignored"是 sudo 自身打印，与个别配置无关）| 在 Ubuntu 26 LTS 全新装的 VM 上复现一次即可 |
| R4 | 错误信息引导用户"重试同款命令"形成死循环 | **强**（install.sh L103-L105 stderr 输出的是同款 `sudo -E bash`，等于让用户"按原路再死一次"）| 直接读代码即证伪 |
| R5 | 当前架构依赖"环境变量从用户 shell 透传穿越 sudo 边界到 bash 子进程"，这是一条**脆弱契约**：依赖 (1) shell 解析 `VAR=val cmd` 语法、(2) sudo 不剥离、(3) bash 接收，三者任一失败即败 | **强**（设计层面）| 不需证伪——3 条链路里 sudo 这一节由发行版控制，必然存在某发行版让契约失效 |

### 2.1 RA 倾向（非裁决，供架构师参考）

- R1 + R2 + R5 同源：**当前"用环境变量传 role"的设计与 sudo 边界天然冲突**。任何修复必须把 role 信号从 env 通道迁移到不受 sudo 影响的通道。
- 业界主流一键安装脚本的解决方案：
  - **k3s** `... | INSTALL_K3S_VERSION=v1.21.4+k3s1 sh -` —— 注意 **sh -**（不带 sudo），脚本内部 re-exec sudo。
  - **rustup** `... | sh -s -- -y --default-toolchain stable` —— **bash -s -- 命令行参数**（不依赖 env）。
  - **docker** `curl ... -o get-docker.sh && sudo sh get-docker.sh` —— 两步（不混用管道与 sudo env）。
  - **homebrew** `/bin/bash -c "$(curl ...)"` —— 不用 sudo，脚本自检后报错。
- 推荐方向（架构师裁决）：让 install.sh 支持 **CLI 参数 `--role server|client`**，并把管道入口字串从 `... | FRP_EASY_ROLE=client sudo -E bash` 改为 `... | sudo bash -s -- --role client`（或脚本内 re-exec sudo 让外层不带 sudo）；环境变量 `FRP_EASY_ROLE` **保留作回退**（旧用户复用旧入口仍可用），错误提示同时给出 CLI 形态推荐。

---

## §3 功能需求（FR）

每条 FR 都是测试可验证的。

| ID | 需求 | 必/可 |
|---|---|---|
| FR-1 | 在 Ubuntu 22.04 / 24.04 / 26.04 LTS 默认 sudoers 配置下，README 推荐的客户端一键安装命令一次执行成功安装到 `/opt/frp-easy` 且 `.role` = `client`。无需用户额外配置 sudoers / env_keep。 | 必 |
| FR-2 | 服务端同款入口（推荐命令）在同套发行版下一次执行成功安装且 `.role` = `server`。 | 必 |
| FR-3 | 公网 IP 探测兜底命令（用户已知公网 IP 时绕过探测）在同套发行版下一次执行成功。 | 必 |
| FR-4 | 强制覆盖 role 路径（`FRP_EASY_FORCE_ROLE=yes` 等价物）在同套发行版下可用；具体语法由架构师决定（可能改为 `--force-role`）。 | 必 |
| FR-5 | 现存用户若沿用旧的"环境变量 + sudo -E bash"入口（兼容形态），在 sudo 不阻碍 env 透传的发行版（Ubuntu 22 / 旧 RHEL 等）上**仍能成功**，不破坏 T-017 / T-018 / README 老群文档用户。 | 必 |
| FR-6 | install.sh 当 role 参数完全缺失（既无 CLI 参数也无 env 变量）时，错误提示**必须**展示**最新可靠**的入口语法（CLI 形态 + 环境变量回退形态，且明确指出"如果你刚才看到 `sudo -E ... '-E' is ignored` 错误，请改用 CLI 形态"）。 | 必 |
| FR-7 | install.sh 的 step 8 横幅"更新"段（client + server 两处）打印的"重新运行同一条一键安装命令"字串**必须**是最新可靠形态，与 README 一致。 | 必 |
| FR-8 | install.sh 的 step 8 横幅"公网 IP 探测失败兜底"段打印的命令必须是最新可靠形态。 | 必 |
| FR-9 | install.sh 的 `--help` 段（L42-L85）必须同步更新所有入口字串。 | 必 |
| FR-10 | install.sh 的 step 6.5"强制覆盖 role 冲突"段打印的命令必须是最新可靠形态。 | 必 |
| FR-11 | install.sh 顶端注释（L8-L17 用法段）必须同步。 | 必 |
| FR-12 | `README.md` 和 `docs/DEPLOYMENT.md` 中所有出现的 `... \| FRP_EASY_ROLE=... sudo -E bash` 字串必须同步替换；解释段（"`-E` 不能省"）必须改为新模式下的对应解释（或删除如新模式不需要）。 | 必 |
| FR-13 | 修复**不引入**第二份脚本 / wrapper.cmd 类辅助文件，保持单脚本一键发布形态。 | 必 |
| FR-14 | 修复**不**要求用户改 `/etc/sudoers` 或加 `Defaults env_keep`，保持"复制粘贴一条命令"的体感。 | 必 |
| FR-15 | install.sh CLI 参数解析必须能正确处理 `--help` / `-h`（已有），新的 `--role` 加入后不破坏 `--help` 行为。 | 必 |
| FR-16 | install.sh CLI 参数必须支持 `--role=server` / `--role server` 两种语法（GNU 风格 + 简化风格），或者**只支持一种**但在 `--help` 明确说明。 | 条件必（由架构师定，但二选一不能漏文档）|
| FR-17 | 推荐命令必须能在 `set -euo pipefail` 严格模式下执行（用户 shell 配 `set -e` 等场景）。 | 必 |
| FR-18 | macOS 路径上同款修复成立（OS=darwin 分支仍走 install.sh，role 解析逻辑与 Linux 共用一份）。 | 必 |

---

## §4 非功能需求（NFR）

| ID | 类别 | 需求 |
|---|---|---|
| NFR-1 | 用户体感 | 用户从复制 README 命令到看到 step 8 横幅，**全过程零额外操作**。错误情形下错误提示给出**可直接复制粘贴**的新命令（不是"请阅读 docs/XX"类间接指引）。|
| NFR-2 | 可维护性 | 单脚本 + 单 README + 单 DEPLOYMENT.md 内可解释；改动局部、注释引用 sudo / env_keep / 业界 pattern 的 RFC / man 页。不引入额外可选依赖。|
| NFR-3 | 兼容性 | 老用户旧入口（env 形态）在 sudo 允许 env 透传的发行版上仍 work（FR-5）；新用户用新入口在新旧发行版上均 work（FR-1/2/3）。 |
| NFR-4 | 失败可观测 | 任何缺 role / role 非法路径都给出包含**新入口字串**的完整可执行复制粘贴命令；旧 sudo -E 解释行删除或改为"如果你看到 sudo '-E' is ignored 错误..."的诊断引导。|
| NFR-5 | 自动化测试边界 | QA 在 sudo 严格的环境（如 docker `ubuntu:24.04` 容器 + sudo + 默认 sudoers）上自动复现失败前态 + 验证修复后态。允许 `sudo` 在 docker 内静默 NOPASSWD 配置。|
| NFR-6 | 软件工程标准 | CLI 参数解析遵循 GNU getopt 风格（`--role server` / `--role=server` / `-h` / `--help`）；错误信息层级清晰（不是把 4 行命令糊在一起作为单行 stderr）。|
| NFR-7 | 安全 | 不增加新的可执行远程入口；脚本内 re-exec sudo 路径（若架构师采纳）必须用绝对路径 `/usr/bin/sudo` 或经 `command -v` 解析后的绝对路径，避免 PATH 注入。|

---

## §5 验收准则（AC）

> 每条 AC 给出"执行命令 + 期望输出 + 自动 / 手工 标签"。

### AC-1（验 FR-1，Ubuntu 26 LTS / 24 LTS client 成功）

**操作**：在 Ubuntu 26 LTS 干净 VM 中以非 root 用户执行 README 推荐的 client 命令（架构师产出的新形态）：

```bash
# 新形态示例（具体语法由架构师裁决）
curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash -s -- --role client
```

**期望**：脚本跑完 step 8/8，打印客户端横幅；`cat /opt/frp-easy/.role` 输出 `client`；`systemctl is-active frp-easy` 输出 `active`。**手工**（用户协助）+ **自动**（docker `ubuntu:26.04` + sudo NOPASSWD 自动化复现）。

### AC-2（验 FR-2，server 同款）

**操作**：同 AC-1 但 `--role server`。

**期望**：`.role` = `server`，横幅显示公网 IP 探测段落。**手工** + **自动**。

### AC-3（验 FR-3，公网 IP 兜底）

**操作**：

```bash
curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo FRP_EASY_PUBLIC_IP=1.2.3.4 bash -s -- --role server
```

或架构师选定的等价新形态。

**期望**：横幅"公网访问"行显示 `http://1.2.3.4:7800`。**手工**。

### AC-4（验 FR-4，强制覆盖 role）

**操作**：在已装 client 的机器上跑：

```bash
curl -fsSL ... | sudo bash -s -- --role server --force-role
# 或环境变量回退形态：
curl -fsSL ... | sudo FRP_EASY_FORCE_ROLE=yes bash -s -- --role server
```

**期望**：旧 `.role` 备份并被覆盖为 `server`，`frp_easy.toml.bak.<timestamp>` 生成。**手工** + **自动**（docker 模拟）。

### AC-5（验 FR-5，老旧入口兼容）

**操作**：在 sudo 允许 env 透传的容器（如 `ubuntu:22.04` + 自定义 sudoers `Defaults env_keep+="FRP_EASY_ROLE FRP_EASY_PUBLIC_IP FRP_EASY_FORCE_ROLE"`）中跑：

```bash
curl ... | FRP_EASY_ROLE=client sudo -E bash
```

**期望**：仍成功安装，`.role` = `client`。**自动**（docker）。

### AC-6（验 FR-6，错误提示包含新形态）

**操作**：直接跑 install.sh 不传任何 role 信号：

```bash
sudo bash scripts/install.sh
```

**期望**：stderr 输出包含**两段**：
- 推荐用法（新 CLI 形态）：`curl ... | sudo bash -s -- --role server` + `... --role client`
- 兼容用法（环境变量回退）：`curl ... | FRP_EASY_ROLE=server sudo bash -s -- --role server`（或架构师选定的兼容字串）
- 诊断指引：`如果你看到 sudo: ... '-E' is ignored ，请使用 CLI 形态`

退出码 3。**自动**。

### AC-7（验 FR-7，横幅"更新"段）

**操作**：完整安装后查横幅"更新"段（client + server 两处）。

**期望**：打印的"重新运行同一条一键安装命令"字串与 README 当前推荐字串字节级一致；不含任何 `sudo -E` 残留。**手工** + **自动**（grep）。

### AC-8（验 FR-8 / FR-10，所有 install.sh 内的命令字串无残留）

**操作**：

```bash
grep -nE 'sudo[[:space:]]+-E[[:space:]]+bash' scripts/install.sh
grep -nE 'FRP_EASY_ROLE=[a-z]+[[:space:]]+sudo' scripts/install.sh
```

**期望**：两条 grep 输出都不命中（或仅命中"兼容回退提示"段且行号已知 / 不命中 in stderr 主推荐段）。**自动**（verify_all 可加 step）。

### AC-9（验 FR-9，--help 同步）

**操作**：`bash scripts/install.sh --help`。

**期望**：输出包含新 CLI 形态推荐 + 环境变量回退说明 + 不含"sudo 需 -E"的过时解释；退出码 0。**自动**。

### AC-10（验 FR-12，README 和 DEPLOYMENT 同步）

**操作**：

```bash
grep -nE 'sudo[[:space:]]+-E[[:space:]]+bash' README.md docs/DEPLOYMENT.md
```

**期望**：不命中。**自动**。

### AC-11（验 FR-13，无新增 wrapper 文件）

**操作**：

```bash
ls scripts/install*.cmd scripts/install*.bat scripts/install-wrapper* 2>/dev/null
```

**期望**：零命中。**自动**。

### AC-12（验 FR-15 / FR-16，CLI 解析正确性）

**操作**：

```bash
bash scripts/install.sh --help                 # 期望：help 文本，退出 0
bash scripts/install.sh --role                 # 期望：缺 value 错误，退出非 0
bash scripts/install.sh --role bogus           # 期望：role 非法，退出 3
bash scripts/install.sh --role server --role client  # 期望：架构师定义行为（最后一个生效 或 报错）
bash scripts/install.sh --unknown-flag         # 期望：未识别参数错误，退出非 0
```

**期望**：各路径 stderr 中文报错明确，退出码符合架构师方案。**自动**。

### AC-13（验 FR-17，set -euo pipefail 兼容）

**操作**：

```bash
set -euo pipefail
curl -fsSL ... | sudo bash -s -- --role client
echo "exit=$?"
```

**期望**：用户 shell 不中断；安装成功；`exit=0`。**自动**。

### AC-14（验 FR-18，macOS）

**操作**：在 macOS 14+ 上跑同款 CLI 形态命令。

**期望**：脚本走 macOS 降级分支（步骤 7 darwin）打印手动启动指引，退出 0。**手工**（需 macOS 真机或 mac runner）。

### AC-15（验 NFR-4，错误提示中"sudo -E ignored"诊断指引）

**操作**：在 Ubuntu 26 LTS 上故意用**旧**入口跑 `curl ... | FRP_EASY_ROLE=client sudo -E bash`。

**期望**：sudo 打印 `'-E' is ignored` 后 install.sh 的 ROLE-empty 错误**包含**一行类似"如果你看到上方 sudo '-E' is ignored 提示，请改用 CLI 形态：`curl ... | sudo bash -s -- --role client`"。**手工**（用户协助验证）。

### AC-16（验 NFR-5，docker 自动化验证）

**操作**：

```bash
docker run --rm -i ubuntu:24.04 bash -c '
  apt-get update -qq && apt-get install -y curl sudo systemd >/dev/null 2>&1
  useradd -m -G sudo alan && echo "alan ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers
  su - alan -c "curl -fsSL file:///host/install.sh | sudo bash -s -- --role client"
'
```

（具体 docker 形态由 QA 设计，以上为示意）

**期望**：容器内退出 0，`.role` = `client`。**自动**。

---

## §6 Out of Scope（明确不做）

| OOS-ID | 项 | 理由 |
|---|---|---|
| OOS-1 | Windows `install.ps1` 增加 `-Role` 参数支持 server/client 区分 | README L81 历史决策：Windows 路径默认 0.0.0.0，与历史行为一致。本任务仅修 Linux/macOS 路径。如有需要新开任务。|
| OOS-2 | 用户 shell 的 `sudoers` 自动检测与提示（脚本启动时主动检查 `sudo -E env` 行为）| 增加复杂度；FR-1 直接消除依赖即可，无需在脚本内做 sudo 行为探测。 |
| OOS-3 | 改 `Defaults env_keep` / 系统级 sudoers | FR-14 红线，违反"单条命令复制粘贴"用户体感。|
| OOS-4 | install.sh 增加 `--install-dir` / `--public-ip` 等其他 CLI 参数（将所有 env 全迁移到 CLI）| 范围扩大；本任务只解决 role 通道（核心受 sudo 阻断的环境变量）。其他 env 如 `FRP_EASY_PUBLIC_IP` 不阻塞主流程（探测失败有兜底），可保留 env-only。如有需要新开任务。|
| OOS-5 | 引入第二份 `install-cli.sh` 或 GUI 安装器 | FR-13 红线 + NFR-2 易维护原则。|
| OOS-6 | 修 install.sh L23 退出码语义（如把缺 role 从 3 改 1）| 已有外部脚本依赖该退出码；改退出码会破坏向后兼容。|
| OOS-7 | rolling release artifact 内 install-service.sh / uninstall-service.sh 的同款 CLI 参数支持 | install-service.sh 是 install.sh 安装结束后调用的下游，不在 README 推荐入口里，用户不直接调用。如有需要新开任务。|
| OOS-8 | 当前 `.harness/` 流程的元任务改进 | 本任务专注于用户安装路径修复，不混搭 Harness 元任务。|

---

## §7 Open Questions（用户已授权全部 `[PM-resolved]`）

### OQ-1：CLI 参数语法风格

**问题**：`--role` 是否只支持 `--role server` / `--role client`（空格分隔），还是同时支持 `--role=server` 等号语法？

**候选**：
- (a) 只支持 `--role <value>`（GNU getopt 经典形态，最少代码）
- (b) 同时支持 `--role <value>` 和 `--role=<value>`（更宽容）

**`[PM-resolved]` 默认 = (b)**：用户体感原则——"复制粘贴某博客字串"时两种风格都常见；多一行 case 处理可覆盖。FR-16 明确二选一不能漏文档；架构师可改判 (a)，但选定后必须在 `--help` 明确。

### OQ-2：环境变量回退是否保留

**问题**：旧入口 `... | FRP_EASY_ROLE=client sudo -E bash`（在 sudo 允许 env 透传的发行版上仍 work）是否保留？

**候选**：
- (a) 完全删除，只支持 CLI 参数（强制所有用户改新入口）
- (b) 保留作"兼容回退"，CLI 参数优先级高于环境变量

**`[PM-resolved]` 默认 = (b)**：FR-5 已锁定。旧群文档 / Stack Overflow 答案 / 用户私人脚本里黏贴的旧入口在某些发行版仍可用，删除会引发回归报告。新发行版（Ubuntu 26+）上旧入口因 sudo `-E` 被忽略**自然失败**，错误提示引导用户改新形态——这是渐进退路。

### OQ-3：错误提示是否显式提及 sudo `-E` 诊断

**问题**：当 role 缺失时，错误提示是否加一行"如果你看到 sudo: '-E' is ignored 错误，请用 CLI 形态"？

**候选**：
- (a) 加（精准命中 Ubuntu 26+ 用户场景）
- (b) 不加（让错误提示更简洁）

**`[PM-resolved]` 默认 = (a)**：NFR-4 失败可观测 + 用户原报告里 sudo 已经打了 `'-E' is ignored`，install.sh 的提示**紧接其后**显示，正是把诊断闭环的最佳位置。

### OQ-4：是否把"推荐入口"在 stderr 错误中也复述

**问题**：当前 install.sh L103-L105 在 stderr 复述两条完整 curl 命令；改新形态后是否同款复述？

**候选**：
- (a) 是，stderr 复述新 CLI 形态完整命令 ×2（server + client）
- (b) 否，stderr 仅给一行指引"请用 --role 参数；详见 --help"

**`[PM-resolved]` 默认 = (a)**：NFR-1 用户体感 + 错误现场必须可复制粘贴。

### OQ-5：脚本是否做 sudo re-exec（让用户外层不带 sudo）

**问题**：业界 pattern 中 k3s 的 install.sh 是 `curl ... | sh -`（无 sudo），脚本内部检测非 root 时 re-exec `sudo $0 ...`。本任务是否也走这条路？

**候选**：
- (a) 不 re-exec —— 推荐入口保留 `sudo bash -s -- --role X` 形态（用户外层 sudo）
- (b) re-exec —— 推荐入口改为 `curl ... | bash -s -- --role X`（无 sudo，脚本内部 re-exec sudo）

**`[PM-resolved]` 默认 = (a)**：架构师裁决空间。考虑：
- (b) 更友好（用户少敲 4 个字符 "sudo"），但 re-exec 时如何把 stdin 管道（脚本本身已经是 stdin 流）传给 sudo 子进程是 trap—— curl pipe 内容已被 bash 第一遍读完，re-exec 时 sudo 启动新 bash 无法重读管道
- (a) 简单可靠，外层 sudo 让 bash 直接以 root 启动
- 架构师选 (a) 是稳态选择；如能解决 (b) 的管道重入问题（如先把脚本下载到临时文件再 sudo），也可考虑

**强烈倾向 (a)**：(b) 的"先 curl 下载到 /tmp/ 再 sudo bash" 模式会破坏"单条 curl 管道"的体感且与 docker / k3s 主流形态不一致（k3s 用 `sh -` 直跑+脚本内 re-exec 是因为它的脚本本身要求**有源**的 stdin；frp_easy install.sh 不要求）。

### OQ-6：FRP_EASY_FORCE_ROLE 是否也加 CLI 等价

**问题**：当前升级期 role 冲突的强制覆盖只能通过 env 控制。是否加 `--force-role` CLI 参数？

**候选**：
- (a) 加（与 `--role` 对称）
- (b) 不加，仅保留 env 形态

**`[PM-resolved]` 默认 = (a)**：与 OQ-2 (b) 对称——env 仍可用；CLI 同时支持让新入口完整不依赖 env。FR-4 已锁定。

### OQ-7：是否同步加 `--public-ip` CLI 参数

**问题**：`FRP_EASY_PUBLIC_IP` 是否也加 CLI 等价 `--public-ip`？

**候选**：
- (a) 加（一致性最好）
- (b) 不加（OOS-4 限制范围，本任务专注 role 通道）

**`[PM-resolved]` 默认 = (b)**：OOS-4 已锁定。`FRP_EASY_PUBLIC_IP` 不阻塞主流程（探测失败有兜底），且不在 sudo `-E` 的关键路径上（仅在 step 8 server 横幅生成时读，那时 install.sh 已 root 运行）。如未来用户报告同款问题，新开任务。

---

## §8 关联历史任务（必读，不重复设计）

| Task | 关联点 |
|---|---|
| **T-017** `install-role-and-public-ip`（2026-05-23 DELIVERED）| **直接前置**：当前 ROLE / FRP_EASY_FORCE_ROLE / 拒绝静默默认的设计源此任务。架构师**必须**读 02_SOLUTION_DESIGN.md §5.1 + PM 决议 AMBIG-C 子问题 C1.a。本任务**不**改变"拒绝静默默认"红线（FR-1 仍要求显式 role），只迁移信号通道。|
| **T-018** `upload-bin-multiport-ip-probe`| 决定了运行期也读 `FRP_EASY_PUBLIC_IP`（handlers_system.go L284-L286 注释）；本任务不动运行期通道，仅修安装期 install.sh。|
| **T-016** `install-progress-and-systemd-unit-fix` | install.sh 主体形态源此任务（步骤 1-8 编号 + 进度条 + curl `--progress-bar`）。|
| **T-013** `rolling-release-install` | install.sh 引入的初始任务；GitHub Releases API 查询 + 资产 URL 解析模式源此任务。|
| **T-012** `one-click-install-and-mit-license` | 一键安装的第一份脚本；MIT license 同期加入。|
| **T-031 / T-026** `install-ps1-*` | Windows 侧 install.ps1 的 iex/exit 修复，**反例**：那边的 sudo 等价问题是"PS host 关窗"，与本任务的 "sudo env 透传失败"不同根因，但用户体感目标一致（一条命令复制粘贴即装）。|
| **insight L19 / L24 / L25 / L26** | install.ps1 / 一键安装入口字串维护域既有踩坑（PS5.1 BOM / -NoExit 等）；本任务是 Linux 侧的对称问题。|
| **insight L31-L34 / L38** | PM 派发上下文 SDK 工具裁剪 → role-collapse 到 PM 自演 7 stage；本任务沿用该做法。|

---

## §9 风险登记（移交架构师）

| ID | 风险 | 缓解（建议方向，由架构师决定）|
|---|---|---|
| RISK-1 | 旧群文档 / 旧 Stack Overflow 答案上的旧入口仍在传播，用户复用旧入口在 Ubuntu 26+ 上失败的"过渡期"会持续数月 | (1) 错误提示精准引导（OQ-3 a 默认）；(2) README 加"旧入口若失败请改用新入口"段；(3) 横幅"更新"段强制用新入口（用户每次升级看到新字串）|
| RISK-2 | CLI 参数解析增加 install.sh 行数，需复盘 set -euo pipefail / case 块 / quoting 安全（避免 `eval`） | 架构师方案必须用纯 bash case + shift 模式（与 L38-L93 的 `-h\|--help` while 循环对齐扩展），不引入 getopt 外部依赖（不同发行版 getopt 行为差异大）|
| RISK-3 | OQ-2 (b) 保留 env 形态会导致代码内有两条 role 来源（CLI + env），需明确优先级 + 测试覆盖两条来源同时存在的冲突场景 | 优先级：CLI > env；冲突时 stderr 警告 + 取 CLI；AC-12 增加同时设置场景|
| RISK-4 | docker 自动化复现失败前态（旧入口 + Ubuntu 26+ sudo）需要 sudo 严格配置；docker 镜像 `ubuntu:26.04` 默认 sudoers 是否模拟 LTS 真机行为需 QA 验证 | QA 在 06 中给出 docker 命令 + 真机一致性证据|
| RISK-5 | macOS 路径的 sudo 行为与 Linux 不完全一致（macOS sudo 默认 `Defaults env_keep+=HOME`），无 macOS runner 在 CI 难以自动复测 | AC-14 标"手工"；QA 报告中标注 macOS 验证是次要平台手工实测|
| RISK-6 | 修复改 README 推荐入口会影响其他文档（如 CONTRIBUTING.md / SECURITY.md 若有）—— 需全仓库 grep 后联动 | RA 已在 §1.3 影响面穷举（README + DEPLOYMENT），未发现其他文件命中；架构师再次 grep 确认|
| RISK-7 | install.sh 是发布包内嵌的 `scripts/install.sh`（rolling release artifact 内同款拷贝），更新后必须等下一次 CI 滚动发布才生效到下载链——意味着"过渡期"还包括"GitHub raw 已新但 release artifact 内的 install.sh 仍旧"的偏差窗口 | 修复 commit push 后 CI 自动刷新 rolling release（既有机制）；README 推荐入口走 `raw.githubusercontent.com/.../main/scripts/install.sh` 直接拉 main，不走 release artifact 内的脚本，**因此 raw 一被 push 即生效**——无需等 CI。已被 T-013 原设计覆盖。|

---

## §10 Verdict

**READY**

所有 Open Questions 均按用户授权 `[PM-resolved]` 取默认。FR / NFR / AC / OOS 明确可验证。架构师可基于此进入 Stage 2（02_SOLUTION_DESIGN.md），优先证伪 §2 R5 设计层假设、确定 OQ-1/OQ-5 的 CLI 形态裁决（默认值见 OQ-1 b + OQ-5 a），并产出具体的 bash 参数解析代码骨架。

— Requirement Analyst, 2026-05-24
