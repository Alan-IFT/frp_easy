# 02 — 方案设计 · T-035 install-sh-role-cli-arg-passthrough

> Harness 流水线 Stage 2（Solution Architect）。模式：**full**。
> 上游：`docs/features/install-sh-role-cli-arg-passthrough/01_REQUIREMENT_ANALYSIS.md`（Verdict = READY）。
> 本文档只做技术决议与可执行步骤，不重述 FR / AC（编号沿用 01）。
> 关联前置 T-017 / T-018 / T-031 / T-026 见 01 §8。

---

## §1 设计目标与硬约束

### §1.1 设计目标（引述 01 §0）

> 让 README 推荐的"Linux 一键安装"入口在 Ubuntu 24+/26+/Debian 13+ 等带较新 sudo（含 `Defaults env_reset` 默认策略 + 限制 `-E` 的发行版）上一次成功安装客户端 / 服务端 frp_easy。

### §1.2 硬约束（设计绝不可违反）

| ID | 约束 | 来源 |
|---|---|---|
| C-1 | 不要求用户改 `/etc/sudoers` / `Defaults env_keep` | 01 FR-14 |
| C-2 | 不新增 wrapper.cmd / install-cli.sh 类辅助文件 | 01 FR-13 + OOS-5 |
| C-3 | 旧入口（env + sudo -E）在 sudo 允许 env 透传的发行版上**仍能成功** | 01 FR-5 + OQ-2 b 默认 |
| C-4 | role 静默默认仍被拒绝（拒绝静默是 T-017 红线，本任务不动） | T-017 02 §5.1 + 01 §0 |
| C-5 | 退出码 3 = role 缺失 / 非法 / role 冲突保持不变（外部脚本可能依赖） | 01 OOS-6 |
| C-6 | 不引入外部依赖（getopt / argbash / argp 等） | 01 NFR-2 |
| C-7 | CLI 参数解析在 `set -euo pipefail` 下健壮 | 01 FR-17 |
| C-8 | 错误提示新文案包含 CLI 主推荐 + env 兼容回退 + sudo `-E` 诊断引导 | 01 FR-6 + OQ-3 a + OQ-4 a |

### §1.3 不应违反的红线（往届 insight / 决议）

| 红线 | 出处 | 含义 |
|---|---|---|
| install.sh 解析 `--help` 用 while/case 形态 | 现有 L38-L93 | 本任务扩展该 while 循环而非引入新 framework |
| 注释强制 explain WHY | T-026 / T-031 注释规约 | 新增 CLI 解析必须有为什么用 case 而非 getopt 的注释 |
| 错误信息中文 + 可复制粘贴 | T-017 02 + 01 NFR-4 | stderr 文案不能引用"详见 docs/XX"间接指引 |
| `FRP_EASY_FORCE_ROLE` env 与 `--force-role` 必须等价 | 01 FR-4 + OQ-6 a | 旧用户脚本中 env 形态必须保留 |
| Windows install.ps1 不动 | 01 OOS-1 | 本任务不扩展 Windows 路径 |
| handlers_system.go L284-L286 注释引用不动 | 01 §1.3 #10 | 不影响 Go 后端 |

---

## §2 候选方案评估

### §2.1 方案 A：CLI 参数 `--role <value>` + 外层 sudo（推荐）

**做法**：

```bash
# 新推荐入口
curl -fsSL .../install.sh | sudo bash -s -- --role client

# 兼容回退（旧入口仍可用，sudo 允许 env 透传时）
curl -fsSL .../install.sh | FRP_EASY_ROLE=client sudo -E bash
```

install.sh 在现有 while loop 中扩展 case 块，新增 `--role server|client`、`--role=server|client`、`--force-role`、`--public-ip <ip>`（OQ-7 b 默认拒，本任务不加 `--public-ip`）。

**评估**：

| 项 | 评分 |
|---|---|
| 满足 FR | FR-1 ✓ / FR-2 ✓ / FR-3 ✓ / FR-4 ✓（加 `--force-role`）/ FR-5 ✓（env 形态保留作兼容）/ FR-6 ✓ / FR-7 ✓ / FR-8 ✓ / FR-9 ✓ / FR-10 ✓ / FR-11 ✓ / FR-12 ✓ / FR-13 ✓ / FR-14 ✓ / FR-15 ✓ / FR-16 ✓ / FR-17 ✓ / FR-18 ✓ |
| 实施代价 | install.sh +~30 行 case 块（已是 while loop 内最小扩展），文案改 ~15 处；README ~10 行 diff；DEPLOYMENT ~8 行 diff；总 ≤100 行 |
| 风险 | (i) 旧用户复用旧 env 入口在新 Ubuntu 上仍败——错误提示精准引导新形态（C-8）；(ii) bash `getopts` 不支持长选项，必须手写 case shift——已有先例 |
| 长期可维护性 | **5/5**（与业界主流 rustup / k3s / nvm CLI 形态一致；GNU getopt 风格用户/工具友好）|
| 用户体感 | **5/5**（新入口跨发行版稳定；不依赖 sudo 行为）|

### §2.2 方案 B：脚本内 re-exec sudo（去掉外层 sudo）

**做法**：

```bash
# 推荐入口（无外层 sudo）
curl -fsSL .../install.sh | bash -s -- --role client
```

install.sh 启动后 `if [[ "$(id -u)" -ne 0 ]]; then exec sudo "$0" "$@"; fi`。

**评估**：

| 项 | 评分 |
|---|---|
| 满足 FR | FR-1 ⚠（**致命**：脚本本身从 stdin 管道来，`$0` 不是磁盘文件，re-exec sudo 时 sudo 启动新 bash 没有 stdin pipe，脚本内容丢失）|
| 实施代价 | 需先 `cat > /tmp/install-$$.sh` 保存脚本再 `exec sudo bash /tmp/install-$$.sh "$@"`——破坏单管道形态，引入临时文件清理风险 |
| 风险 | (i) 临时文件清理 trap 与 bash stdin 状态耦合；(ii) `/tmp` race / `noexec`；(iii) k3s 之所以能用是因为它脚本明确要求 `sh -`（不是 bash 管道），且不依赖 stdin pipe 在 re-exec 后存活|
| 长期可维护性 | 3/5（多一层 stdin 拯救逻辑，维护期 footgun 高）|
| 用户体感 | 4/5（少打 4 字符 sudo）|

**否决**：FR-1 致命风险（stdin pipe 不可重入），且收益 = 用户少打 4 字符——成本/收益不合算。

### §2.3 方案 C：sudo wrapper 函数（`sudo env FRP_EASY_ROLE=X bash`）

**做法**：

```bash
curl -fsSL .../install.sh | sudo env FRP_EASY_ROLE=client bash
```

**评估**：

| 项 | 评分 |
|---|---|
| 满足 FR | FR-1 ⚠（`sudo env VAR=val cmd` 在某些 sudo 配置下 `env` 不在 `secure_path` 中会失败；且 `sudo env` 默认仍走 env_reset，VAR=val 在 sudo 之后 = 给 env 命令的参数，与 sudoers env_keep 行为绑定）|
| 实施代价 | 推荐字串变长；用户教育成本 + 与"k3s/rustup 主流形态"偏离 |
| 风险 | 高 sudo 实现差异敏感；测试矩阵爆炸 |
| 长期可维护性 | 2/5 |
| 用户体感 | 3/5 |

**否决**：方案 A 的 `bash -s -- --role X` 形态完全等价且无 sudo 实现差异敏感。

### §2.4 方案 D：仅改 README 推荐用"先下载再 sudo"两步形态

**做法**：README 推荐：

```bash
curl -fsSL .../install.sh -o /tmp/install.sh
sudo FRP_EASY_ROLE=client bash /tmp/install.sh
```

**评估**：

| 项 | 评分 |
|---|---|
| 满足 FR | FR-1 ✓ / FR-2 ✓ |
| 实施代价 | 文档改动最小（脚本零改）|
| 风险 | (i) 两步形态破坏"一条命令" 体感（NFR-1 a）；(ii) `/tmp/install.sh` 残留文件（卫生差）；(iii) `FRP_EASY_ROLE` 在 `sudo VAR=val cmd` 形态下是给 sudo 的 env 前缀——同样依赖 sudo 不剥离自定义 env，发行版差异问题**没有解决**只是换了形态|
| 长期可维护性 | 3/5（脚本侧 0 改，但文档对用户体感是降级）|
| 用户体感 | 2/5（多一步操作；NFR-1 a 红线）|

**否决**：FR-1 表面满足但 `sudo VAR=val cmd` 仍受 sudoers env_keep 影响，没有根治。

### §2.5 决策矩阵

| 维度 | A（CLI + 外层 sudo）| B（re-exec sudo）| C（sudo env）| D（两步） |
|---|---|---|---|---|
| 满足全部 FR | ✓ | ✗（stdin pipe）| ⚠（sudo 差异）| ⚠（仍依赖 sudoers）|
| 用户体感 | 5 | 4 | 3 | 2 |
| 长期可维护性 | 5 | 3 | 2 | 3 |
| 业界主流一致 | ✓（rustup / k3s pattern）| 部分 | ✗ | ✗ |
| 实施代价 | 中（~100 行）| 高 | 中 | 低 |
| 风险 | 低 | 高 | 中 | 中 |

**选定方案 A**。

---

## §3 详细实施设计（方案 A 拆解）

### §3.1 install.sh CLI 参数解析骨架

```bash
# 在现有 while loop (L38-L93) 内扩展 case 块。
# 设计原则：
# - 长选项 `--role <value>` + 等号形态 `--role=<value>` 双支持（01 OQ-1 b 默认）
# - CLI 优先级 > env：if [[ -n "$ROLE_FROM_CLI" ]]; then ROLE="$ROLE_FROM_CLI"; else ROLE="${FRP_EASY_ROLE:-}"; fi
# - 不引入 getopt 外部依赖（C-6）—— bash builtin case + shift 自给自足
# - `--role <value>` 时若下一个 token 以 `--` 开头视为"缺 value"错误（防 `--role --force-role` 吞参）

ROLE_FROM_CLI=""
FORCE_ROLE_FROM_CLI=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help)
            # 现有 help 块（文案需更新，见 §3.4）
            cat <<'EOF'
...
EOF
            exit 0
            ;;
        --role)
            # 形态 1：`--role <value>`
            if [[ $# -lt 2 || "$2" == --* ]]; then
                echo "错误：--role 缺少取值（server|client）。" >&2
                exit 3
            fi
            ROLE_FROM_CLI="$2"
            shift 2
            ;;
        --role=*)
            # 形态 2：`--role=<value>`
            ROLE_FROM_CLI="${1#--role=}"
            if [[ -z "$ROLE_FROM_CLI" ]]; then
                echo "错误：--role= 后必须紧跟取值（server|client），不能为空。" >&2
                exit 3
            fi
            shift
            ;;
        --force-role)
            FORCE_ROLE_FROM_CLI="yes"
            shift
            ;;
        --)
            # POSIX 终止符：之后的参数不再解析
            shift
            break
            ;;
        *)
            echo "错误：未识别的参数 $1，运行 bash install.sh --help 查看用法。" >&2
            exit 1
            ;;
    esac
done

# CLI > env 优先级合并（设计依据：01 FR-5 + OQ-2 b + RISK-3）
if [[ -n "$ROLE_FROM_CLI" ]]; then
    ROLE="$ROLE_FROM_CLI"
else
    ROLE="${FRP_EASY_ROLE:-}"
fi

# FORCE_ROLE 同款合并
if [[ -n "$FORCE_ROLE_FROM_CLI" ]]; then
    FORCE_ROLE_EFFECTIVE="yes"
else
    FORCE_ROLE_EFFECTIVE="${FRP_EASY_FORCE_ROLE:-no}"
fi
```

### §3.2 ROLE 校验段新文案（替换 L101-L107）

```bash
if [[ -z "$ROLE" || ( "$ROLE" != "server" && "$ROLE" != "client" ) ]]; then
    echo "错误：必须指定 --role server|client（不允许静默默认）" >&2
    echo "" >&2
    echo "推荐用法（CLI 形态，跨发行版稳定，Ubuntu 24+/Debian 13+ 必需）：" >&2
    echo "  服务端（公网 VM）：curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash -s -- --role server" >&2
    echo "  客户端（内网设备）：curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash -s -- --role client" >&2
    echo "" >&2
    echo "兼容用法（环境变量形态，仅在 sudo 允许 env 透传的发行版上有效，Ubuntu 22 LTS 及更旧）：" >&2
    echo "  curl -fsSL .../install.sh | FRP_EASY_ROLE=client sudo -E bash" >&2
    echo "" >&2
    echo "诊断：如果你刚才看到 sudo 输出 \"'-E' is ignored\" 或类似 env 透传被拒提示，" >&2
    echo "       说明本机 sudo 安全策略不允许保留环境变量。请改用上方 \"CLI 形态\" 命令。" >&2
    echo "" >&2
    echo "说明：服务端默认监听 0.0.0.0，客户端默认监听 127.0.0.1。" >&2
    exit 3
fi
echo "    role=${ROLE} (来源: ${ROLE_FROM_CLI:+CLI}${ROLE_FROM_CLI:-环境变量})"
```

### §3.3 FRP_EASY_FORCE_ROLE 合并到 FORCE_ROLE_EFFECTIVE

L400 当前判定：

```bash
if [[ "${FRP_EASY_FORCE_ROLE:-no}" != "yes" ]]; then
```

改为：

```bash
if [[ "$FORCE_ROLE_EFFECTIVE" != "yes" ]]; then
```

L406 旧提示：

```bash
echo "    FRP_EASY_ROLE=${ROLE} FRP_EASY_FORCE_ROLE=yes sudo -E bash ..." >&2
```

改为：

```bash
echo "    curl -fsSL .../install.sh | sudo bash -s -- --role ${ROLE} --force-role" >&2
```

### §3.4 `--help` 段文案更新（L42-L85）

将旧 "推荐用法（curl | bash 形态，需 root / sudo 权限；sudo -E 透传环境变量）" 段替换为：

```
推荐用法（CLI 形态，跨发行版稳定 —— Ubuntu 24+/Debian 13+ 必需）:
  服务端：curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash -s -- --role server
  客户端：curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash -s -- --role client

兼容用法（环境变量形态，仅在 sudo 允许 env 透传的发行版上有效）:
  服务端：curl -fsSL .../install.sh | FRP_EASY_ROLE=server sudo -E bash
  客户端：curl -fsSL .../install.sh | FRP_EASY_ROLE=client sudo -E bash

谨慎用户（先下载脚本审阅后再执行）:
  curl -fsSL .../install.sh -o install.sh
  sudo bash install.sh --role server   # 或 client
```

环境变量段补充：`FRP_EASY_ROLE` 与 `--role` 等价（CLI 优先）。`FRP_EASY_FORCE_ROLE=yes` 与 `--force-role` 等价。

参数段补充：

```
  -h, --help              显示本帮助后退出
  --role server|client    必填；与环境变量 FRP_EASY_ROLE 等价（CLI 优先）
  --role=server|client    --role 的等号形态
  --force-role            升级期切换 role 时强制覆盖；与环境变量 FRP_EASY_FORCE_ROLE=yes 等价
```

### §3.5 step 8 横幅新字串

#### 客户端横幅（L530）

```bash
echo "    curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash -s -- --role client"
```

#### 服务端横幅（L589）

```bash
echo "    curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash -s -- --role server"
```

#### 公网 IP 探测失败兜底（L574）

```bash
echo "      curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo FRP_EASY_PUBLIC_IP=<your-ip> bash -s -- --role server"
```

> 注意：这里 `FRP_EASY_PUBLIC_IP` 仍是 sudo 命令前的 env 前缀；但**与 `-E` 不同**，`sudo VAR=val cmd` 形态下 `VAR=val` 是 sudo 接受的"env to set"语法，不依赖 sudoers env_keep。`sudoers(5)` 文档明确："The default is to allow only those variables explicitly enumerated. ... However, `sudo` accepts variable assignments on the command line in the form of `VAR=value`"——这是 sudo 命令行 env 设置（非透传），任何 sudo 版本均支持。
>
> 为对齐"CLI 优先"风格，可考虑加 `--public-ip <ip>` CLI 等价（01 OQ-7 b 默认拒）；本任务仍走兼容路径，保持范围聚焦。

### §3.6 脚本顶端注释（L8-L17）更新

```bash
# 用法：
#   推荐（CLI 形态，跨发行版稳定 —— Ubuntu 24+/Debian 13+ 必需）：
#     服务端（公网 VM，监听 0.0.0.0，需要公网 IP）：
#       curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash -s -- --role server
#     客户端（内网设备，仅监听 127.0.0.1，最安全）：
#       curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash -s -- --role client
#   兼容（环境变量形态，仅在 sudo 允许 env 透传的发行版上有效）：
#       curl -fsSL .../install.sh | FRP_EASY_ROLE=server sudo -E bash
#   谨慎用户（先下载审阅再执行）：
#     curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh -o install.sh
#     sudo bash install.sh --role server
# 参数：-h | --help              显示本帮助后退出（退出码 0）
#       --role server|client     必填；与 FRP_EASY_ROLE 等价（CLI 优先）
#       --force-role             与 FRP_EASY_FORCE_ROLE=yes 等价
#       环境变量 FRP_EASY_ROLE          server | client（CLI 形态优先；env 兼容回退）
#       环境变量 FRP_EASY_FORCE_ROLE    yes（与 --force-role 等价）
#       环境变量 FRP_EASY_PUBLIC_IP     合法 IPv4/IPv6（绕过公网 IP 自动探测，server 模式适用）
#       环境变量 FRP_EASY_INSTALL_DIR   覆盖安装目录（默认 /opt/frp-easy）
```

### §3.7 README.md 同步

#### L56-L65（一键安装两条命令 + sudo -E 解释段）

旧：

```bash
# 服务端（公网 VM，要让外部 frpc 客户端能连进来）
curl -fsSL .../install.sh | FRP_EASY_ROLE=server sudo -E bash

# 客户端（内网设备，仅本机访问 UI 最安全）
curl -fsSL .../install.sh | FRP_EASY_ROLE=client sudo -E bash
```

> 注意 `sudo -E` 的 `-E` 不能省 ...

新：

```bash
# 服务端（公网 VM，要让外部 frpc 客户端能连进来）
curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash -s -- --role server

# 客户端（内网设备，仅本机访问 UI 最安全）
curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash -s -- --role client
```

> 这条 CLI 形态在 Ubuntu 24/26 LTS、Debian 13 等"较新 sudo 默认不允许 `-E` 保留环境变量"的发行版上一次成功；旧用户若沿用 `... | FRP_EASY_ROLE=client sudo -E bash` 形态在新发行版会因 sudo 拒绝 `-E` 而失败（脚本错误提示会引导你改用上方 CLI 形态）。

#### L92（公网 IP 兜底）

旧：

```bash
curl -fsSL .../install.sh | FRP_EASY_PUBLIC_IP=<你的公网IP> FRP_EASY_ROLE=server sudo -E bash
```

新：

```bash
curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo FRP_EASY_PUBLIC_IP=<你的公网IP> bash -s -- --role server
```

### §3.8 docs/DEPLOYMENT.md 同步

L41-L67 同款替换为 CLI 形态。`> sudo -E 的 -E 不能省 ...` 段删除，改为：

```
> 这条 CLI 形态在 Ubuntu 24/26 LTS、Debian 13 等"较新 sudo 默认不允许 -E"的发行版上必需；旧用户若沿用 `... | FRP_EASY_ROLE=client sudo -E bash` 形态在新发行版会失败。环境变量入口仍作为兼容回退保留（CLI 优先）。
```

L67 "审阅 install.sh 内容后" 段：

旧：

```bash
sudo FRP_EASY_ROLE=server bash install.sh  # 或 client
```

新：

```bash
sudo bash install.sh --role server  # 或 client
```

---

## §4 受影响模块（按文件路径）

| 文件 | 操作 | 行数估计 | Partition |
|---|---|---|---|
| `scripts/install.sh` | 编辑：扩展 while loop、ROLE 校验文案、L400 / L406 / L530 / L574 / L589 横幅、L8-L17 注释、L42-L85 help 段 | ~+45 / -25 | dev（fullstack 项目无 dev-ops 分区，单 developer 模式）|
| `README.md` | 编辑：L56-L65 一键安装段、L65 解释段、L92 公网 IP 兜底 | ~+8 / -5 | dev |
| `docs/DEPLOYMENT.md` | 编辑：L41-L67 A.0 段、相关解释 | ~+6 / -5 | dev |

无新增文件。无删除文件。

### §4.1 Reuse audit

| Need | Existing code | File path | Decision |
|---|---|---|---|
| 长选项 case 解析 | 现有 `-h\|--help` while loop | `scripts/install.sh` L38-L93 | 扩展同一个 while loop（C-6 + C-7 已隐含）|
| ROLE 校验 | 现有 ROLE empty + non-server/client 校验 | `scripts/install.sh` L101-L107 | 改文案，保留逻辑 |
| FORCE_ROLE 处理 | 现有 `FRP_EASY_FORCE_ROLE` env 读取 | `scripts/install.sh` L400 | 抽到 FORCE_ROLE_EFFECTIVE 变量统一来源 |
| 横幅字串复用 | 现有客户端 / 服务端横幅打印 | `scripts/install.sh` L508-L596 | 局部替换字串 |
| help 段 heredoc | 现有 heredoc | `scripts/install.sh` L42-L85 | 局部文案改 |
| 中文错误信息 idiom | 现有 stderr 中文报错 | `scripts/install.sh` 全文 | 保留风格 |

无新增依赖。无新模块。

### §4.2 Partition assignment

项目使用单 Developer 模式（`.harness/agents/dev-*.md` 仅有 `developer.md`、无 frontend/backend 分区 agent）：

| File | Partition | New / Edit | Dependency |
|---|---|---|---|
| `scripts/install.sh` | developer | edit | — |
| `README.md` | developer | edit | depends on install.sh（文案需一致）|
| `docs/DEPLOYMENT.md` | developer | edit | depends on install.sh |

#### Dispatch order

1. developer（单步完成全部 3 个文件改动）

#### Parallelism

None — 单 developer 一次性提交 3 个文件改动，保证文案字节级一致。

---

## §5 流程图（用户安装路径 vs install.sh 内部）

```
                  ┌─────────────────────────────────────────────────────────────────────┐
                  │ 用户在 shell 粘贴推荐命令                                            │
                  │ curl ... | sudo bash -s -- --role client                            │
                  └────┬────────────────────────────────────────────────────────────────┘
                       │
                       │  curl 下载脚本 → stdout pipe
                       │  sudo 提权（用户可能要输密码）
                       │  bash -s -- --role client：bash 从 stdin 读脚本；
                       │  --role client 通过位置参数传给 bash → $1=--role $2=client
                       ▼
       ┌──────────────────────────────────────────────────────────────────┐
       │ install.sh 启动                                                   │
       │ ─ while loop 解析 --role client → ROLE_FROM_CLI=client            │
       │ ─ ROLE = ROLE_FROM_CLI（CLI 优先；env 未设也 OK）                  │
       │ ─ ROLE 校验通过 → echo "role=client (来源: CLI)"                  │
       │ ─ step 1-8 正常跑（与 T-017 后既有逻辑无差异）                     │
       └──────────────────────────────────────────────────────────────────┘
```

兼容回退路径：

```
                  ┌─────────────────────────────────────────────────────────────────────┐
                  │ 旧用户复用旧入口                                                     │
                  │ curl ... | FRP_EASY_ROLE=client sudo -E bash                        │
                  └────┬────────────────────────────────────────────────────────────────┘
                       │  sudo 允许 -E 的发行版：env 透传成功
                       │  sudo 拒绝 -E 的发行版：env 丢失
                       ▼
       ┌──────────────────────────────────────────────────────────────────┐
       │ install.sh                                                        │
       │ ─ while loop 解析 → ROLE_FROM_CLI="" / FORCE_ROLE_FROM_CLI=""     │
       │ ─ ROLE = ${FRP_EASY_ROLE:-} （env 兼容路径）                       │
       │ ─ 若 ROLE = "client" → 通过 → 正常走 step 1-8（FR-5 兼容）        │
       │ ─ 若 ROLE = "" → 进入新文案错误路径：                              │
       │     ─ 第一段提示 CLI 形态                                         │
       │     ─ 第二段提示 env 兼容形态                                     │
       │     ─ 第三段诊断 "'-E' is ignored" → 引导 CLI                    │
       │     ─ exit 3                                                      │
       └──────────────────────────────────────────────────────────────────┘
```

---

## §6 风险分析与缓解

| ID | 风险 | 概率 | 影响 | 缓解 |
|---|---|---|---|---|
| R-1 | `bash -s --` 这种 stdin 脚本 + 位置参数透传形态在某些 bash 5.x bug / 旧 `/bin/sh` 软链接下可能解析异常 | 低 | 中 | install.sh shebang `#!/usr/bin/env bash` 已锁定 bash 执行；推荐字串明确写 `sudo bash -s --`（不是 `sudo sh -s --`），强制 bash 解释；QA AC-13 验证 `set -euo pipefail`|
| R-2 | 用户复制粘贴时漏 `--`（POSIX 终止符），变 `sudo bash -s --role client` —— bash `-s` 后跟 `--role` 会被 bash 当成自身 flag 试图解析（实测 bash `-s` 后参数原样透传，但 `--role` 因含 `--` 可能被 bash 误判）| 中 | 低 | bash `-s` 文档明确：`-s` 后所有 arguments 都成为 `$1, $2, ...`；但**保险**起见推荐字串保留 `--`；README + 注释中显式写明 `--` 不可省；AC-12 验证 `bash -s --role server` 与 `bash -s -- --role server` 行为差异，确认是否需要进一步教育用户 |
| R-3 | `--role` 与已有 `--help` 解析顺序：用户写 `... --role server --help` 期望什么？ | 低 | 低 | bash case 解析按顺序 shift；`--help` 命中即 exit 0（已有行为）；混用时 help 优先 = "help 总是赢"——与 GNU 主流一致 |
| R-4 | env 与 CLI 同时存在且值冲突（如 `FRP_EASY_ROLE=server` + `--role client`）| 低 | 低 | CLI 优先（§3.1 已实现）；为可观测性，echo 行打印 `(来源: CLI)` / `(来源: 环境变量)`；若 env 与 CLI 同时存在且值不同，CLI 仍胜——不打 warning（避免噪音）；如真有冲突场景 QA AC-12 已覆盖 |
| R-5 | `sudo bash -s -- --role client` 在用户 shell 有 `alias bash=/usr/local/bin/bash` 时可能走错解释器 | 极低 | 低 | sudo 默认 `secure_path` 走 `/usr/bin/bash`，alias 不生效；不缓解 |
| R-6 | 老群文档 / 老 Stack Overflow 答案传播旧入口；过渡期数月 | 高 | 中 | (1) 错误提示精准引导（§3.2）；(2) README 新增"如你看到旧入口失败"提示段；(3) 横幅"更新"段强制新形态——用户每次升级自动看到新字串 |
| R-7 | docker `ubuntu:24.04` / `ubuntu:26.04` 镜像默认 sudoers 与 LTS 真机行为可能不同 | 中 | 低 | QA 在 06 给出 docker 命令 + 与用户报告（Ubuntu 26 LTS 真机）一致性证据；R-7 不阻塞本任务（用户报告已锁定真根因）|
| R-8 | rolling release artifact 内的 `install.sh` 与 raw.githubusercontent.com/.../main/scripts/install.sh 短暂不同步 | 低 | 极低 | README 推荐入口走 raw，不走 release artifact；commit push 后 raw 即生效；CI 滚动发布刷新 ≤5 min（既有机制）|
| R-9 | macOS sudo 默认 `Defaults env_keep+=HOME` 与 Linux 不同；macOS 真机无 CI runner | 中 | 低 | macOS 路径仅走 step 7 darwin 分支退出 0；不依赖 ROLE-aware 逻辑深度（macOS 无 systemd）；AC-14 标手工 |
| R-10 | `--role=` 等号形态如 `--role=` 后空字符串 vs `--role ""` 处理 | 低 | 低 | §3.1 case `--role=*` 已显式校验 `${1#--role=}` 为空报错 |
| R-11 | 用户传 `--role Server`（大小写不同）| 中 | 低 | 现有校验已严格匹配 `server` / `client` 小写；不放宽（与 T-017 一致）；错误信息中文已说明 `server|client` 小写 |
| R-12 | step 8 横幅"更新"段是字符串字面量，未来 README 推荐入口再改时易漏 | 中 | 中 | 在 install.sh 顶端注释加 "RECOMMENDED_INSTALL_CMD 字符串多处出现，未来联动改时 grep `bash -s -- --role`"；不抽常量（C-6 不引入复杂；bash 字符串常量跨多 heredoc 不实用）|

---

## §7 迁移 / 回滚计划

### §7.1 迁移

无 schema 变更，无 DB 迁移。本任务仅修脚本 + 文档。

**升级期**：

- 既有用户安装在 `/opt/frp-easy`：不动任何运行时状态。新版 install.sh 升级路径（步骤 6 "检测到已存在安装"分支）行为完全不变；ROLE 仍从 CLI/env 解析得来。
- 既有用户用旧 env 入口跑升级（兼容路径）：在允许 env 透传的发行版上仍 work；在拒绝的发行版上看到新错误提示并被引导到 CLI 形态——一次性教育成本。

### §7.2 回滚

- 若新版 install.sh 发布后发现严重回归（如 case 解析在某些 bash 版本下挂死）：
  - 短期：用户回退用 raw + 指定 commit SHA 拉旧版本 install.sh（GitHub raw 支持 `https://raw.githubusercontent.com/Alan-IFT/frp_easy/<sha>/scripts/install.sh`）
  - 长期：revert commit，重新走 7-stage pipeline 修

### §7.3 兼容性保证矩阵

| 用户场景 | 修复前行为 | 修复后行为 |
|---|---|---|
| Ubuntu 22 / 旧 RHEL + 旧 env 入口 | 成功 | 成功（FR-5 兼容路径）|
| Ubuntu 22 / 旧 RHEL + 新 CLI 入口 | N/A（不存在） | 成功 |
| Ubuntu 24/26 LTS + 旧 env 入口 | **失败**（用户报告）| 失败（同样路径），但错误提示精准引导 CLI 形态（C-8）|
| Ubuntu 24/26 LTS + 新 CLI 入口 | N/A（不存在） | **成功**（FR-1 核心目标）|
| 已装机器升级（任意发行版 + 任意入口）| 视入口而定 | 视入口而定，逻辑不变 |
| `--role bogus` / `--role` 空值 | N/A | exit 3，中文错误 |
| `--help` | 显示 help 退 0 | 显示新 help 退 0 |
| `--unknown-flag` | exit 1（现有 *） 分支 | exit 1（保持）|

---

## §8 Out-of-scope 设计澄清

- 本设计不动 `install.ps1`（Windows）。
- 本设计不引入 `--public-ip` CLI（OQ-7 b 默认，§3.5 兜底字串仍走 `sudo VAR=val bash`，与 `-E` 无关）。
- 本设计不优化 install-service.sh / uninstall-service.sh 的 CLI 参数（OOS-7）。
- 本设计不调整 ROLE 校验大小写、不放宽 `Server` / `Client` 大写匹配（与 T-017 一致）。
- 本设计不抽 `RECOMMENDED_INSTALL_CMD` bash 常量（实施代价超过收益；R-12 用注释提示缓解）。

---

## §9 验收检查清单（对齐 01 AC）

| AC | 设计如何满足 |
|---|---|
| AC-1 | §3.1 CLI 解析 + ROLE_FROM_CLI 优先级；用户报告环境真机复测 |
| AC-2 | 同 AC-1，server role |
| AC-3 | §3.5 `--role server` + env 形态 `FRP_EASY_PUBLIC_IP` |
| AC-4 | §3.1 `--force-role` + §3.3 FORCE_ROLE_EFFECTIVE 合并 |
| AC-5 | env 通道在 §3.1 ROLE 合并后**仍工作**（兼容路径，FR-5）|
| AC-6 | §3.2 新错误文案三段（CLI / env / sudo `-E` 诊断）|
| AC-7 | §3.5 横幅字串新 CLI 形态 |
| AC-8 | grep `sudo -E bash` 仅命中错误提示的"兼容路径"行（已知容忍）；主推荐字串零命中 |
| AC-9 | §3.4 help 段更新 |
| AC-10 | §3.7 + §3.8 README + DEPLOYMENT 同步 |
| AC-11 | 无新 wrapper 文件（FR-13）|
| AC-12 | §3.1 + R-2 / R-3 / R-4 / R-10 风险缓解覆盖 |
| AC-13 | §3.1 case + shift 在 `set -euo pipefail` 下健壮（现有 while loop 已是同款） |
| AC-14 | macOS 路径无 ROLE-aware 深度逻辑（step 7 darwin 分支退 0），CLI 解析在 macOS bash 上同款工作 |
| AC-15 | §3.2 第三段诊断引导 |
| AC-16 | QA 自动化 docker 复现（NFR-5）|

---

## §10 Verdict

**READY**

设计完整、可独立实施。所有 01 FR / NFR / AC 在 §3 / §4 / §9 中对齐。所有 01 OQ 默认值在 §1-§3 中具象化为代码骨架。架构师推荐选定方案 A，Developer 可直接按 §3 拆解实施。

— Solution Architect, 2026-05-24
