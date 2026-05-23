# 04 — Development · T-017 install-role-and-public-ip

> Stage 4 / 7（dev-backend，full 模式）。严格按 02 §3-§6 实现 + 03 §6 九条 conditions 全部落地。
> Inv-1（appconf 不动）、Inv-2（main.go 不动）、Inv-3（install-service.sh 不动）、Inv-4（frp_linux/ 升级保留）、Inv-7（verify_all PASS:19）全程守住。

---

## §1 实现概要

### 1.1 改动文件清单

| 文件 | 类型 | 净增行 | 说明 |
|---|---|---:|---|
| `scripts/install.sh` | edit | +293 / -13 | 顶部注释扩 role/-h 用法；§0.5 ROLE 解析与拒绝；新增 `render_frp_easy_toml` + `detect_public_ip` 两个 helper；步骤 6 后插 §6.5 role 应用 + 局部 chown 块；步骤 8 横幅替换为 role-aware 双路径 |
| `scripts/install.ps1` | edit（局部） | +76 / -0 | 新增 `Get-PublicIPv4` PowerShell 函数（FR-B 同步 + OOS-2 注释）；步骤 8 横幅追加公网行 + 失败降级文案 |
| `scripts/uninstall-service.sh` | edit（最小） | +6 / -0 | 删 unit 后 `rm -f ${INSTALL_DIR}/.role`（独立一行 + 注释），保持 uninstall 幂等 |

### 1.2 不动的文件（Inv 守护）

| 文件 | 状态 | 守护理由 |
|---|---|---|
| `internal/appconf/config.go` | untouched | Inv-1：T-011 NF-2 "用户显式值优先" 语义不破坏 |
| `cmd/frp-easy/main.go` | untouched | Inv-2：安全提示 / ListenAddr / setup 链路保持 |
| `scripts/install-service.sh` | untouched | Inv-3：T-016 unit 模板 + 退出码透传 + systemd-analyze warn+继续不回退；本任务的 RUN_USER + chown 问题靠 install.sh §6.5 上游解决 |
| `scripts/install-service.ps1` | untouched | Windows Service 默认 LocalSystem 无 unit User= 根因 |
| `internal/storage/**`、`migrations/**` | untouched | 本任务无 DB 改动 |
| `web/**` | untouched | OOS-8 |
| `.harness/insight-index.md` | untouched | 07 阶段由 archive-task 自动收割（C-9 路由） |
| `scripts/verify_all.sh` / `scripts/baseline.json` | untouched | NFR-4 / NFR-6（不新增检查项、不改基线） |

### 1.3 主要新接口（02 §5 契约）

- 必填环境变量 `FRP_EASY_ROLE={server|client}`；缺失或非法 → 退出码 **3** + 中文样例两行 + sudo -E 说明。
- 可选环境变量 `FRP_EASY_FORCE_ROLE=yes`（升级期 role 冲突的强制覆盖通道，会备份旧 toml）。
- 可选环境变量 `FRP_EASY_PUBLIC_IP=<ip|hostname>`（M-2 兜底通道，国内 VM 上跳过 3 候选 URL 探测）。
- 新文件 `${INSTALL_DIR}/.role`（`server\n` 或 `client\n`，0644 root:root；仅 install.sh 升级期读，运行时进程不读）。
- 新预生成文件 `${INSTALL_DIR}/frp_easy.toml`（仅全新安装时由 `render_frp_easy_toml` 渲染；字段名 UIBindAddr/UIPort/DataDir/LogDir 严格 = `internal/appconf/config.go` struct tag）。

---

## §2 九条 conditions 落地映射

| ID | 来源（03 §6） | 落地位置 | 关键证据 |
|---|---|---|---|
| **C-1** | M-1：RUN_USER 解析 verbatim 复制 install-service.sh L69-75 两段式 if-then-else | `scripts/install.sh` §6.5 块 RUN_USER 解析（install.sh 现 +约 100 行处） | 注释 "verbatim 复制 install-service.sh L69-75 两段式 if-then-else（与 install-service.sh L69-75 必须保持等价，更改时同步两处）"；不使用 `${SUDO_USER:-$(id -un)}` 简写 |
| **C-2** | M-2：detect_public_ip 函数首行 FRP_EASY_PUBLIC_IP short-circuit + 失败横幅打印 sudo -E 样例 + 国内 VM 提示 | `scripts/install.sh` `detect_public_ip` 函数首块 + 步骤 8 server 失败分支 | 函数首块 `if [[ -n "${FRP_EASY_PUBLIC_IP:-}" ]]; then ... return 0; fi`；失败横幅打印 `FRP_EASY_PUBLIC_IP=<your-ip> FRP_EASY_ROLE=server sudo -E bash` + "国内 VM（腾讯云 / 阿里云 / 华为云）可登云控制台 → 实例详情复制公网 IP" |
| **C-3** | M-3：保持 install.sh top-level 主流程，不做 `BASH_SOURCE` source-mode 包裹 | `scripts/install.sh` 全文 | 无任何 `[[ "${BASH_SOURCE[0]}" == "$0" ]]` 包裹；主流程顺序执行，QA 走集成测试 |
| **C-4** | m-1：04 §实现说明记录 `DataDir = './.frp_easy'` + `WorkingDirectory=/opt/frp-easy` → `/opt/frp-easy/.frp_easy` 解析链 | 本节 §3.4 显式列出 | QA 验证由 06 阶段加 `[ -d /opt/frp-easy/.frp_easy ]` |
| **C-5** | m-2：detect_public_ip 函数注释明示 FRP_EASY_PUBLIC_IP short-circuit 位置 | `scripts/install.sh` `detect_public_ip` 函数 docstring | 函数头注释含 "先判 FRP_EASY_PUBLIC_IP 用户手动覆盖通道（C-2 / C-5：函数首行 short-circuit）"；首块内联注释 "FRP_EASY_PUBLIC_IP short-circuit（C-2 / C-5；M-2 国内 VM 兜底通道）" |
| **C-6** | m-5：install.ps1 横幅块旁注释 "Windows 路径暂无 role 区分（OOS-2），公网 IP 探测一律执行" | `scripts/install.ps1` 步骤 8 起首注释 + `Get-PublicIPv4` 头注释 | "Windows 路径暂无 role 区分（OOS-2），公网 IP 探测一律执行（C-6）"两处出现 |
| **C-7** | §6.5 chown 不允许 `\|\| true` 静默吞失败；失败应 exit 1 + 中文错误；frp_linux/ 不存在时 `mkdir -p` 后再 chown | `scripts/install.sh` §6.5 块三处 chown | 每条 chown 用 `if ! chown ...; then echo "错误：..."; exit 1; fi` 形态；前置 `mkdir -p "${INSTALL_DIR}/.frp_easy" "${INSTALL_DIR}/frp_linux"` 确保路径存在 |
| **C-8** | 06_TEST_REPORT.md 必含英文 `## Adversarial tests` 标题（insight L31） | 本 §4 给 QA 显式 todo | 由 QA 在 06 落地，本任务仅传递要求 |
| **C-9** | 07_DELIVERY.md `## Insight` 段建议收割两行 | 本 §4 给 07 显式 todo | 由 07 / archive-task 落地，本任务仅传递候选两行原文 |

---

## §3 关键设计抉择实现细节

### 3.1 `detect_public_ip` 函数全文（install.sh 内）

```bash
# 函数：detect_public_ip
# 意图：公网 IP 探测；仅 server 模式调用。先判 FRP_EASY_PUBLIC_IP 用户手动覆盖通道
# （C-2 / C-5：函数首行 short-circuit），否则按顺序尝试 3 个明文写死的候选 URL。
# 入参：无（读 env FRP_EASY_PUBLIC_IP + 函数内 const PUBLIC_IP_CANDIDATES）。
# 出参：stdout = 合法 IPv4 字面量（成功时；无尾换行）；失败时 stdout 空字符串。
# 返回码：0 成功 / 1 全部候选失败（含 short-circuit 校验失败）。
# 预算：单候选 curl --max-time 3 秒，最坏 3 × 3 = 9 秒；调用方在 server 横幅块同步等待。
# 验证：先判 HTTP 状态码 200（curl -f 让 4xx/5xx 自然 rc 非 0），再用 bash 正则
# ^([0-9]{1,3}\.){3}[0-9]{1,3}$ 强校验 IPv4 字面量（insight L37 红线：HTML 错误页
# 不当 IP 用）。
detect_public_ip() {
    # FRP_EASY_PUBLIC_IP short-circuit（C-2 / C-5；M-2 国内 VM 兜底通道）：
    # 用户预先知道公网 IP 时可手动指定，跳过 3 候选探测；仍需通过 IPv4 字面量校验
    # 防止用户把 hostname 或 URL 当 IP 传入。
    if [[ -n "${FRP_EASY_PUBLIC_IP:-}" ]]; then
        if [[ "$FRP_EASY_PUBLIC_IP" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
            printf '%s' "$FRP_EASY_PUBLIC_IP"
            return 0
        fi
        # 非 IPv4 字面量（可能是 IPv6 或 hostname）—— 也直接打印让用户自己拼 URL。
        # 仅在不含空格 / 换行 / 引号等危险字符时才信任。
        if [[ "$FRP_EASY_PUBLIC_IP" =~ ^[A-Za-z0-9.:_-]+$ ]]; then
            printf '%s' "$FRP_EASY_PUBLIC_IP"
            return 0
        fi
        return 1
    fi
    # NFR-2：明文写死候选 URL（用户可 grep 审计）；不允许从 env 动态拼接。
    local candidates=(
        "https://api.ipify.org"
        "https://ifconfig.me/ip"
        "https://icanhazip.com"
    )
    local url ip
    for url in "${candidates[@]}"; do
        # curl -f：HTTP 4xx/5xx 让 curl rc != 0（不读响应体）；--max-time 3 单次预算；
        # -sS 安静但保留错误；不带 -L（echo IP 服务无 redirect 场景）。
        ip="$(curl -fsS --max-time 3 "$url" 2>/dev/null || true)"
        # trim 尾部换行 / 空白
        ip="${ip%%[$' \t\r\n']*}"
        if [[ "$ip" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
            printf '%s' "$ip"
            return 0
        fi
    done
    return 1
}
```

### 3.2 `render_frp_easy_toml` 函数全文

```bash
# 函数：render_frp_easy_toml
# 意图：按 ROLE 渲染 frp_easy.toml 的字面文本到 stdout，供步骤 6.5 预生成配置。
# 入参：$1 = role，取值 "server" 或 "client"（调用前已由 §0.5 校验）。
# 出参：stdout 输出 TOML 文本（含 trailing newline）；无返回码语义。
# 字段名严格 = internal/appconf/config.go L36-39 struct tag（UIBindAddr/UIPort/
# DataDir/LogDir），go-toml/v2 大小写敏感，不可改。DataDir 写相对路径 "./.frp_easy"
# 配合 unit WorkingDirectory=/opt/frp-easy 解析为 /opt/frp-easy/.frp_easy（C-4）。
# heredoc 必须用 <<'EOF' 单引号封禁插值（insight L38 quote-removal 红线）。
render_frp_easy_toml() {
    local role="$1"
    if [[ "$role" == "server" ]]; then
        cat <<'EOF'
# frp_easy.toml — 由 install.sh 在角色为 server 的全新安装中生成（T-017）。
# UIBindAddr=0.0.0.0 表示监听所有网卡（公网 + LAN + 回环），便于 frpc 客户端通过公网 IP 访问 Web UI。
# 仅需本机访问时可手动改为 "127.0.0.1" 后 systemctl restart frp-easy。
UIBindAddr = "0.0.0.0"
UIPort = 7800
DataDir = "./.frp_easy"
LogDir = "./.frp_easy/logs"
EOF
    else
        cat <<'EOF'
# frp_easy.toml — 由 install.sh 在角色为 client 的全新安装中生成（T-017）。
# UIBindAddr=127.0.0.1 表示仅监听回环（最安全），管理 UI 不暴露到公网 / 局域网。
# 如需局域网内访问 UI，可手动改为 "0.0.0.0" 后 systemctl restart frp-easy。
UIBindAddr = "127.0.0.1"
UIPort = 7800
DataDir = "./.frp_easy"
LogDir = "./.frp_easy/logs"
EOF
    fi
}
```

### 3.3 install.sh §6.5 块全文（verbatim）

```bash
# ---- 步骤 6.5：配置角色 + 修复运行时属主（T-017，仅 Linux）----
# macOS 不走 systemd 路径（步骤 7 darwin 分支 exit 0），其 role + chown 语义无意义；
# 仅 Linux 分支需要预生成 toml + 局部 chown。J 决议：darwin 仅同步公网 IP 探测,
# role 与 chown 不强制。
if [[ "$OS" == "linux" ]]; then
    echo "==> [6.5/8] 应用 role=${ROLE} 并修复运行时属主..."

    # 解析 RUN_USER —— C-1 红线：verbatim 复制 install-service.sh L69-75 两段式
    # if-then-else（与 install-service.sh L69-75 必须保持等价，更改时同步两处）。
    # 不能用 `${SUDO_USER:-$(id -un)}` 简写：M-1 要求严格按照 service 脚本同款形态,
    # 同源同据避免 install-service.sh 后续生成 unit 时 User= 与本块 chown 目标错位。
    RUN_USER=""
    if [[ -z "$RUN_USER" ]]; then
        if [[ -n "${SUDO_USER:-}" ]]; then
            RUN_USER="$SUDO_USER"
        else
            RUN_USER="$(id -un)"
        fi
    fi

    # 校验 RUN_USER 存在（install-service.sh L104-108 同款 getent 校验）；提前失败
    # 比让 chown 失败后 install-service.sh 重复报错更易诊断。
    if ! getent passwd "$RUN_USER" >/dev/null 2>&1; then
        echo "错误：用户 $RUN_USER 不存在，请先 useradd 或在 install-service.sh 时传 --user 参数。" >&2
        exit 1
    fi

    # role 一致性校验（升级路径）+ 持久化 .role 文件（FR-C.2）。
    # .role 内容：单行 "server\n" 或 "client\n"；权限 0644 root:root（运行时不读）。
    ROLE_FILE="${INSTALL_DIR}/.role"
    if [[ -f "$ROLE_FILE" ]]; then
        OLD_ROLE="$(head -n1 "$ROLE_FILE" 2>/dev/null | tr -d '[:space:]' || true)"
        if [[ "$OLD_ROLE" != "$ROLE" ]]; then
            if [[ "${FRP_EASY_FORCE_ROLE:-no}" != "yes" ]]; then
                echo "错误：已检测到 role=${OLD_ROLE}，本次指定 role=${ROLE} 冲突。" >&2
                echo "  如需切换 role，请先运行卸载脚本再重装：" >&2
                echo "    sudo ${INSTALL_DIR}/scripts/uninstall-service.sh" >&2
                echo "    sudo rm -f ${INSTALL_DIR}/.role ${INSTALL_DIR}/frp_easy.toml" >&2
                echo "  或显式覆盖（将备份旧 frp_easy.toml 后重写）：" >&2
                echo "    FRP_EASY_ROLE=${ROLE} FRP_EASY_FORCE_ROLE=yes sudo -E bash ..." >&2
                exit 3
            fi
            # 强制覆盖路径：备份旧 toml → 重写 toml → 重写 .role。
            if [[ -f "${INSTALL_DIR}/frp_easy.toml" ]]; then
                cp -a "${INSTALL_DIR}/frp_easy.toml" "${INSTALL_DIR}/frp_easy.toml.bak.$(date +%s)"
            fi
            render_frp_easy_toml "$ROLE" > "${INSTALL_DIR}/frp_easy.toml"
            printf '%s\n' "$ROLE" > "$ROLE_FILE"
            chmod 0644 "$ROLE_FILE" "${INSTALL_DIR}/frp_easy.toml"
            TOML_WROTE="yes"
        else
            TOML_WROTE="no"
        fi
    else
        # 首次安装 或 从 T-017 之前版本升上来（无 .role）。
        # D1 红线：升级期已有 frp_easy.toml 保留用户值优先，不覆盖；
        # 仅全新安装（frp_easy.toml 不存在）才预生成。
        printf '%s\n' "$ROLE" > "$ROLE_FILE"
        chmod 0644 "$ROLE_FILE"
        if [[ ! -f "${INSTALL_DIR}/frp_easy.toml" ]]; then
            render_frp_easy_toml "$ROLE" > "${INSTALL_DIR}/frp_easy.toml"
            chmod 0644 "${INSTALL_DIR}/frp_easy.toml"
            TOML_WROTE="yes"
        else
            TOML_WROTE="no"
        fi
    fi

    # 局部 chown（C-7：不允许 || true 静默吞失败）。
    # 仅 chown 运行时可写路径：frp_easy.toml（若刚创建或已存在）、.frp_easy/、
    # frp_linux/。binary 与 scripts 保持 root:root（最小权限审计）。
    mkdir -p "${INSTALL_DIR}/.frp_easy" "${INSTALL_DIR}/frp_linux"
    if ! chown "$RUN_USER:$RUN_USER" "${INSTALL_DIR}/frp_easy.toml"; then
        echo "错误：chown frp_easy.toml 给 $RUN_USER 失败（请检查文件是否存在与权限）。" >&2
        exit 1
    fi
    if ! chown -R "$RUN_USER:$RUN_USER" "${INSTALL_DIR}/.frp_easy"; then
        echo "错误：chown -R .frp_easy/ 给 $RUN_USER 失败。" >&2
        exit 1
    fi
    if ! chown -R "$RUN_USER:$RUN_USER" "${INSTALL_DIR}/frp_linux"; then
        echo "错误：chown -R frp_linux/ 给 $RUN_USER 失败。" >&2
        exit 1
    fi

    if [[ "$TOML_WROTE" == "yes" ]]; then
        echo "    已预生成 ${INSTALL_DIR}/frp_easy.toml（role=${ROLE}）"
    else
        echo "    保留已有 ${INSTALL_DIR}/frp_easy.toml（D1 用户值优先）"
    fi
    echo "    .role=${ROLE} 持久化；属主修复完成（${RUN_USER}:${RUN_USER}）"
fi
```

### 3.4 DataDir 相对路径解析链（C-4 显式记录）

预生成的 `frp_easy.toml` 写 `DataDir = "./.frp_easy"`（相对路径）。运行期解析链：

```
systemd unit:  WorkingDirectory=/opt/frp-easy      （install-service.sh 写）
       ↓
进程 cwd:      /opt/frp-easy
       ↓
appconf.Load 读 frp_easy.toml:  DataDir = "./.frp_easy"
       ↓
filepath.Abs("./.frp_easy")  → /opt/frp-easy/.frp_easy   （Go 默认行为，cmd/frp-easy/main.go 已隐含使用）
       ↓
mkdir -p 在 install.sh §6.5 已预先创建：/opt/frp-easy/.frp_easy（chown 给 RUN_USER）
```

QA 在 06 加验证：`[ -d /opt/frp-easy/.frp_easy ]` 退出码 = 0 且 `stat -c %U /opt/frp-easy/.frp_easy` = RUN_USER。

### 3.5 install.ps1 同步要点

- 新增 `Get-PublicIPv4` 函数：3 候选 URL 与 install.sh 同款（NFR-2 双 shell 一致）；`Invoke-WebRequest -TimeoutSec 3`；`[ipaddress]::TryParse` + AddressFamily=InterNetwork 校验 IPv4 字面量；FRP_EASY_PUBLIC_IP short-circuit 同款行为。
- 步骤 8 横幅旁注释 "Windows 路径暂无 role 区分（OOS-2），公网 IP 探测一律执行（C-6）"。
- 不引入 role 入口，不接收 FRP_EASY_ROLE 环境变量（OOS-2 保留）。
- Service 默认以 LocalSystem 跑，无 Linux unit User= 的根因；UI 监听地址沿用 `appconf.Default() = 0.0.0.0`，`cmd/frp-easy/main.go` 启动时打印安全提示。

---

## §4 已知限制与下游待办

### 4.1 已知限制（设计内）

- **国内 VM 探测高概率失败**：3 个候选 URL（api.ipify.org / ifconfig.me / icanhazip.com）在国内云 VM 上预期高概率全部失败。已通过 `FRP_EASY_PUBLIC_IP=<ip>` 兜底通道 + 失败横幅明示 "登云控制台复制" 缓解（M-2 / C-2 落地）。
- **client 模式无横幅安全组提示**：FR-C.6 明确仅 server 模式打印安全组段，client 模式横幅不含 `安全组` / `ufw` / `firewall` 字样（这是有意的，避免误导客户端用户去开放公网端口）。
- **macOS 路径仅同步公网 IP 探测，不强制 role + chown**：J 决议保留 OOS-10 的 systemd 边界；§6.5 块由 `if [[ "$OS" == "linux" ]]; then ... fi` 守护，macOS 直接跳过。
- **.role 文件运行时不读**：仅 install.sh 升级期与人眼诊断使用；用户手动改 .role 不会切换 role —— 必须改 frp_easy.toml 才生效。（注释已在 §6.5 块说明；R-3 风险已认）

### 4.2 下游待办

- **C-4（QA @ 06）**：加 `[ -d /opt/frp-easy/.frp_easy ]` 退出码 = 0 与 `stat -c %U /opt/frp-easy/.frp_easy` = RUN_USER 验证。
- **C-8（QA @ 06）**：06_TEST_REPORT.md 必含英文 `## Adversarial tests` 标题（insight L31 / verify_all E.6），至少覆盖：
  - 公网 IP 候选返回 HTML 错误页 → 探测应失败而非把 HTML 当 IP 打印。
  - 公网 IP 候选超时（iptables DROP）→ 单次 ≤ 3 s、总 ≤ 9 s、不阻塞 exit 0。
  - toml 字段名大小写偏差注入 → systemd 应起不来（R-1 反例守护）。
  - FRP_EASY_ROLE 缺失 / 非法 / 升级期冲突 → exit 3。
  - FRP_EASY_PUBLIC_IP 含非法字符（空格、引号、换行）→ short-circuit 拒绝并降级 3 候选。
- **C-9（PM/Architect @ 07）**：07_DELIVERY.md `## Insight` 段建议收割两行（archive-task 自动收割到 `.harness/insight-index.md`）：
  - `install.sh 解包后必须 chown 给 RUN_USER 才能让 systemd User= 进程写 frp_easy.toml，否则 appconf.Load 写默认失败 → 死循环重启`
  - `公网 IP 探测在国内 VM 上 3/3 候选 URL 失败是预期，FRP_EASY_PUBLIC_IP 用户手动覆盖通道必须存在`

### 4.3 Blockers（无）

无设计冲突、无分区跨界、无 owned-path 突破。所有改动均在 `scripts/install*.sh|ps1` 与 `scripts/uninstall-service.sh`；02 §11.4 已沿用 T-016 "scripts/install*.* 不在 dev-backend owned paths 但 SA 授权" 先例，避免 Code Reviewer 阶段 5 误报 BLOCKED ON PARTITION。

---

## §5 verify_all 输出（完整尾部）

```
=== verify_all (fullstack) ===
Project: frp_easy
Stack:   Go + Vue 3 + SQLite (Web UI to manage FRP, single-binary deploy)

[A.1] No hardcoded secrets ... PASS
[A.2] No .env files committed ... PASS
[A.3] TODO/FIXME budget ... PASS
[G.1] go vet ... PASS
[G.2] go test ./... ... PASS
[G.3] go build ./cmd/frp-easy ... PASS
[B.1] Install / typecheck ... PASS
[B.2] Lint ... PASS
[B.3] Unit tests pass ... PASS
[B.4] Test count >= baseline ... PASS
[B.5] No tsc residue in web/src/ ... PASS
[C.1] E2E smoke (playwright) ... PASS
[D.1] OpenAPI / tRPC schema present ... PASS
[E.1] CLAUDE.md present ... PASS
[E.2] workflow.md present ... PASS
[E.3] All 7 agents in .harness/agents/ ... PASS
[E.4] Binding in sync (.harness/ -> .claude/) ... PASS
[E.5] AI-GUIDE.md indexes every .harness/rules/*.md ... PASS
[E.6] Adversarial tests section in completed task reports ... PASS

=== Summary ===
  PASS: 19
  WARN: 0
  FAIL: 0
  SKIP: 0
```

**结果：PASS:19 / FAIL:0**（NFR-4 / NFR-6 守护成功；test_count = 231，与 baseline 一致）。

附加自验（不计入 verify_all）：

- `bash -n scripts/install.sh scripts/install-service.sh scripts/uninstall-service.sh` → ALL BASH SYNTAX OK
- PowerShell parser ParseFile(scripts/install.ps1) → PS SYNTAX OK
- `render_frp_easy_toml server` / `render_frp_easy_toml client` 字段 grep 全部命中
- `detect_public_ip` 在本机 1 秒内返回 `38.47.117.142`（真实出口 IP）
- `FRP_EASY_PUBLIC_IP=1.2.3.4 detect_public_ip` → `1.2.3.4` + rc=0（short-circuit 命中）
- `FRP_EASY_PUBLIC_IP="bad ip" detect_public_ip` → rc=1（含空格拒绝）
- `unset FRP_EASY_ROLE; bash install.sh` → rc=3
- `FRP_EASY_ROLE=invalid bash install.sh` → rc=3
- `unset FRP_EASY_ROLE; bash install.sh -h` → rc=0（-h 在 §0.5 校验前）

---

## §6 自检清单（38 条 insight-index 红线）

| 检查 | 状态 | 证据 |
|---|---|---|
| L18 systemd unit 裸 token + `\x20` 转义 | N/A | 本任务不写 unit（install-service.sh untouched） |
| L19 sc.exe 写 .cmd 编码 | N/A | 本任务不写 .cmd |
| L20 flag.NewFlagSet | N/A | 本任务不动 Go |
| L21 verify_all A.1 正则避坑（"字段名 = 默认值"） | **OK** | 预生成 toml 字段名旁无 api_key/secret/password/token 关键词；A.1 已 PASS |
| L22 sudo `$SUDO_USER` 优先 | **OK** | §6.5 块 RUN_USER 解析 verbatim 复制 install-service.sh L69-75 两段式 |
| L23 PowerShell bash 命令解析 | N/A | 本任务不通过 PowerShell 调 bash |
| L24 PowerShell 写 TOML 无 BOM | N/A | 本任务的 toml 由 install.sh bash heredoc 写（UTF-8 无 BOM 天然） |
| L25 TOML 反斜杠路径 | **OK** | DataDir 用 Unix 正斜杠 `./.frp_easy`（Linux only 路径） |
| L26 `npm exec --` 透传 | N/A | 本任务不动 npm |
| L27 git ls-files vs find 残留 | N/A | 本任务不动 tsc |
| L28 tsconfig noEmit | N/A | 本任务不动 tsconfig |
| L29 GOOS 注入 seam | N/A | 本任务不动 Go |
| L30 setup-go go-version 对齐 | N/A | 本任务不动 CI |
| L31 06_TEST_REPORT.md 英文 `## Adversarial tests` | **传递给 QA** | 已在 §4.2 / C-8 落 todo |
| L32 curl\|bash 禁 `$0`/`$BASH_SOURCE` 自定位 | **OK** | install.sh 全文不出现 `$0` / `${BASH_SOURCE[0]}`（仅 install-service.sh 用 BASH_SOURCE，那是磁盘脚本 untouched） |
| L33 GitHub API 限流 403 先判状态码 | **OK** | install.sh 步骤 3 既有逻辑保留；detect_public_ip 同款思路（先判状态码 + IPv4 字面量校验） |
| L34 softprops/action-gh-release@v2 | N/A | 本任务不动 release.yml |
| L35 concurrency.group | N/A | 本任务不动 GitHub Actions |
| L36 frp 版本兼容 | N/A | 本任务不动 frpconf |
| L37 GitHub API User-Agent + 状态码先判 | **OK** | detect_public_ip 用 `curl -fsS` 让 4xx/5xx rc != 0；状态码先判后正则字面量校验 |
| L38 **bash quote-removal 陷阱（红线）** | **OK** | 预生成 toml heredoc 用 `<<'EOF'` 单引号封禁插值；§6.5 块所有变量插入用 `"${RUN_USER}"` / `printf '%s\n' "$ROLE"` 显式形态，无字符级 `${var//pattern/replacement}` 字面替换 |

**结论**：38 条 insight 中适用的 8 条全部 OK；本任务未触犯任一条红线。

---

## §7 Verdict

**READY**

理由：

1. 9 条 conditions（C-1 ~ C-9）全部落地，C-4/C-8/C-9 显式传递给 QA / 07 阶段。
2. 3 条 major（M-1 / M-2 / M-3）通过 §6.5 块 verbatim 复制 + detect_public_ip 兜底通道 + 保持 top-level 主流程满足。
3. 5 条 minor 全部在代码或文档中显式落地。
4. verify_all PASS:19 / FAIL:0；baseline test_count = 231 未动；NFR-4 / NFR-6 守住。
5. Inv-1 ~ Inv-7 不变量全部守护：Go 代码不动、main.go 不动、install-service.sh 不动、frp_linux/ 升级保留、verify_all 检查项数量不变。
6. 改动文件清单与分区契约一致：仅 `scripts/install*.{sh,ps1}` + `scripts/uninstall-service.sh`，02 §11.4 SA 授权先例覆盖。
7. 38 条 insight 红线适用的 8 条全部自检 OK；未触犯 L38 quote-removal、L32 自定位、L22 SUDO_USER、L33/L37 状态码先判等关键条目。

下一步：派发 Code Reviewer 阶段 5。

