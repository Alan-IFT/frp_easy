# 01 — 需求分析 · T-017 install-role-and-public-ip

> Harness 流水线 Stage 1（Requirement Analyst）。模式：**full**。
> 上游只读输入：`INPUT.md`、`PM_LOG.md`、`docs/tasks.md`、相关历史任务的 `02_SOLUTION_DESIGN.md`。
> 本文档**不做技术选型**、**不画 API/模块形状**——那是 Stage 2 Architect 的工作。
> 本文档的核心交付物是 §5 的 **AMBIGUITIES**（用户必须先回答，否则 Stage 2 无法进行）。

---

## 1. 背景与用户原话

### 1.1 用户原话引用（2026-05-23，来自一台腾讯云 Ubuntu VM 实测）

> 出错了，且只有 127.0.0.1 和局域网 ip，而不是公网 ip，若是服务端，根本没法访问到 UI 页面，
> 需要修复错误，并更正 ip 选择，理论上好像只能在安装脚本运行过程中选择是服务端还是客户端，
> 没法在安装后，因为服务端需要公网 ip，而客户端应该是监听 127.0.0.1 才是最安全的。

### 1.2 实测现场（journalctl 节选，来自 `INPUT.md`）

- 安装结果横幅：
  - `本机访问：    http://127.0.0.1:7800`
  - `局域网访问：  http://10.1.20.7:7800`（VM 内网网卡，非公网出口 IP）
- systemd 服务死循环重启（restart counter 已到 35），关键错误：

  ```
  frp-easy[…]: 加载 frp_easy.toml 失败：appconf: write default: open /opt/frp-easy/frp_easy.toml: permission denied
  systemd[1]: frp-easy.service: Main process exited, code=exited, status=1/FAILURE
  ```

### 1.3 当前代码状态（事实快照）

- `internal/appconf/config.go` Default() = `UIBindAddr: "0.0.0.0", UIPort: 7800`（T-011 决策）。
- `internal/appconf/config.go` Load() 在 `frp_easy.toml` 不存在时调 `os.WriteFile(abs, out, 0o644)`，路径来自调用者，CWD = unit 的 `WorkingDirectory=/opt/frp-easy`。
- `scripts/install-service.sh` 默认 `RUN_USER` = `${SUDO_USER:-$(id -un)}`（T-008 / T-016 沿用）→ 在 `curl|sudo bash` 形态下 = 真实调用者（如 `ubuntu`）。
- `scripts/install.sh` 步骤 6（升级 + 全新两条分支）解包后**未对 `$INSTALL_DIR` 做 `chown -R $RUN_USER`**——所有文件属主在 `cp -a` 后仍是 root（解压目录的属主）。
- `scripts/install.sh` 步骤 8 末尾：`LOCAL_IP="$(hostname -I 2>/dev/null | awk '{print $1}' || true)"`——只能拿到本机第一块网卡 IP（NAT 后获取不到出口 IP）。
- `scripts/install.ps1` 步骤 8 同形态：`Get-NetIPAddress -AddressFamily IPv4` 排除 `127.*`/`169.254.*` 后取第一条——同样只取本机网卡。
- `curl -fsSL <url> | sudo bash` 形态下 install.sh 的 stdin 被 curl 占用（已被 T-012 / T-013 的设计记录）；**stdin 直接读取交互输入不可用**。

### 1.4 因果链（崩溃根因显然）

```
install.sh 步骤 6：cp -a 解压内容到 /opt/frp-easy/（root 拥有）
      ↓
install-service.sh 写 unit：User=ubuntu（SUDO_USER）
      ↓
systemd 启动 frp-easy 进程：cwd=/opt/frp-easy/，euid=ubuntu
      ↓
appconf.Load("frp_easy.toml") → 文件不存在 → os.WriteFile(/opt/frp-easy/frp_easy.toml)
      ↓
ubuntu 用户对 /opt/frp-easy/（root:root 0755）无写权限
      ↓
write default: permission denied → 进程 exit 1 → systemd Restart=on-failure 死循环
```

---

## 2. 用户诉求拆分（A / B / C / D / E 五块）

### 2.1 A — 服务启动崩溃修复

**诉求**：装完后 `systemctl is-active frp-easy` 必须为 `active`；`frp_easy.toml` 必须能被成功写入并被 systemd 拉起的进程读到。

**已知失败模式**：`appconf: write default: open /opt/frp-easy/frp_easy.toml: permission denied`。

**约束**：修复方案不允许削弱 systemd User= 隔离（继续以非 root 用户跑 frp-easy 进程，不退化为 `User=root`），除非用户在 AMBIGUITIES §5 E 显式选择。

### 2.2 B — 公网 IP 显示与 role 关联

**诉求**：服务端场景必须能给出公网可达 IP（外部 frpc 才能连服务端 UI 与 frps `bindPort=7000`）；纯客户端场景不应误导用户去开放公网。

**已知失败模式**：当前 `hostname -I` / `Get-NetIPAddress` 仅打印 LAN IP（如腾讯云 VM 的 `10.1.20.7`），用户看不到公网 IP。

**约束**：公网 IP 探测必须有降级路径——无网 / 防火墙拒外 / 内网无公网 IP / IPv6-only 等场景必须不阻塞安装、不打印误导内容。

### 2.3 C — 安装期角色选择（设计核心）

**诉求**：装机过程中决定本机是 **服务端**（frps，需公网 IP，监听 0.0.0.0）还是 **客户端**（frpc，监听 127.0.0.1 最安全）。

**已知约束**：`curl -fsSL <url> | sudo bash` 形态 stdin 被 curl 占用，**默认不可交互**。

**这是用户必须决定的形态问题**——五种候选见 §5 AMBIG-1（C1-C5），AC 取决于用户的选择。本文档不预先采用某一候选。

### 2.4 D — 与 T-011 既有默认值（0.0.0.0）的兼容/迁移

**诉求**：升级路径上**已存在** `frp_easy.toml` 时，安装期角色选择对该文件的行为：保留？覆写？提示？

**已知事实**：T-011 决策 `Default() = 0.0.0.0`；`Load()` 的"用户显式值优先"语义保证已写值不被覆盖。本任务若要按 role 自动改 `UIBindAddr`，势必与 T-011 的"用户显式值优先"语义冲突——必须在 AMBIGUITIES §5 D 由用户裁决。

### 2.5 E — `frp_easy.toml` 与 `/opt/frp-easy/` 权限模型

**诉求**：fix A 必须给出明确的"哪个用户能写 frp_easy.toml / 哪个用户能写 .frp_easy/ 数据目录 / 哪个用户能改 unit 配置"模型。

**已知候选**（必须用户裁决，见 §5 E）：
- E1. install.sh 装完即 `chown -R $RUN_USER /opt/frp-easy/`（含 binary、scripts、配置、数据）
- E2. install.sh 装完仅 `chown $RUN_USER /opt/frp-easy/{frp_easy.toml,.frp_easy/}`（仅配置 + 数据目录），其余保留 root:root
- E3. install.sh 预先生成 `/opt/frp-easy/frp_easy.toml`（含 role 选择结果）后 chown 给 RUN_USER，确保首启不走"写默认值"分支
- E4. 不动文件属主，改 systemd `User=root`（最坏路径，违反最小权限）
- E5. 引入独立服务用户（如 `frp-easy:frp-easy`），install.sh 自动 useradd —— 但需用户接受新建系统用户的副作用

每种方案影响升级期（已有用户改动是否丢失）、安全审计（攻击面）、跨平台一致性（Windows 无 systemd User= 语义，sc.exe 默认 LocalSystem）。

---

## 3. Functional Requirements（FR）—— 每条 AC 可测试

### 3.1 FR-A · 服务启动崩溃修复

| FR ID | 需求 | AC（可测试） |
|---|---|---|
| FR-A.1 | 装完后 systemd 单元 `frp-easy` 必须处于 `active` 状态。 | `systemctl is-active frp-easy` 退出码 = 0、stdout = `active`。在 Ubuntu 22.04 / 24.04 / Debian 12 / RHEL 9 各跑一次。 |
| FR-A.2 | 装完后 `/opt/frp-easy/frp_easy.toml` 必须存在且可被 systemd 拉起的进程**写**（升级期下次启动也能写）。 | `[ -f /opt/frp-easy/frp_easy.toml ]` 退出码 = 0；以 unit `User=` 指定的用户身份 `test -w /opt/frp-easy/frp_easy.toml` 退出码 = 0（即 `sudo -u $RUN_USER test -w …`）。 |
| FR-A.3 | journalctl 中**绝不**再出现 `write default: open .*: permission denied` 模式。 | `journalctl -u frp-easy --no-pager` 输出 grep 关键字 `permission denied` 命中数 = 0。 |
| FR-A.4 | systemd restart counter 在装完后 60 秒内必须为 ≤ 1（允许首次启动一次抖动）。 | `systemctl show frp-easy -p NRestarts --value` ≤ 1。 |
| FR-A.5 | 修复方案**不得**回退到 `User=root`，除非用户在 §5 E 显式选 E4。 | `grep -E '^User=' /etc/systemd/system/frp-easy.service` 命中行 != `User=root`（除非 AMBIG-E = E4）。 |

### 3.2 FR-B · 公网 IP 与访问地址打印

> FR-B.x 的具体 AC 形态**取决于 §5 AMBIG-C 的 role 选择形态**。下方按"role 已知（server / client）"两种状态给出。

| FR ID | 需求 | AC（可测试） |
|---|---|---|
| FR-B.1 | role = server 时，安装横幅必须**尝试**打印公网 IP；探测成功时打印 `http://<PUBLIC_IP>:7800`，探测失败时打印明确的降级文案（不打印错误的 LAN IP 冒充公网 IP）。 | `bash install.sh` 输出 stdout：当探测成功时 grep `http://<PUBLIC_IP>:7800` 命中；当探测失败时 grep `<公网 IP 探测失败>` 或等价中文降级文案命中，且**绝不**出现"公网访问：http://10.x.x.x" / "192.168.x.x" / "172.16-31.x.x" 等私有段冒充公网。 |
| FR-B.2 | role = server 时，横幅必须仍打印本机访问 `http://127.0.0.1:7800` + 局域网 `http://<LAN_IP>:7800`，三者并列（公网 / LAN / 本机）。 | 横幅 grep `本机访问`、`局域网访问`、`公网访问`（或等价标签）三行均命中。 |
| FR-B.3 | role = client 时，横幅**只**打印本机访问 `http://127.0.0.1:7800`，**不**触发公网 IP 探测（避免无谓的外网请求与隐私泄漏）。 | role=client 安装期间无任何对 ifconfig.me / api.ipify.org / icanhazip.com / ident.me / ipinfo.io 等公网 IP 服务的网络请求（可通过 `strace -e trace=connect` 或临时 iptables block 出站 + 安装仍成功并不报"无公网 IP"错来验证）。 |
| FR-B.4 | 公网 IP 探测有**超时上限**与**总耗时上限**：单次探测 ≤ 3 秒、所有降级尝试合计 ≤ 8 秒。超时不能阻塞安装收尾（不能让用户等 30 秒）。 | 模拟所有公网 IP 服务无响应（iptables DROP 出站 443/80 给 echo IP 服务），从步骤 8 开始到 install.sh `exit 0` 的总耗时 ≤ 15 秒（含步骤 7 已完成）。 |
| FR-B.5 | 公网 IP 探测的"可信源"必须 ≥ 2 个且首次失败自动降级到下一个。 | 代码（或脚本）含 ≥ 2 个不同 host 的 echo IP URL；阻断第一个时第二个生效（可测：iptables block 仅第一个 host）。 |
| FR-B.6 | 公网 IP 探测结果必须是合法 IPv4（或 IPv6）字面量，不能是 HTML / 错误页 / 空字符串。 | 横幅打印的 IP 字符串 `<PUBLIC_IP>` 通过 `python3 -c "import ipaddress; ipaddress.ip_address('<PUBLIC_IP>')"` 退出码 = 0；不含字符 `<`、空格、换行。 |
| FR-B.7 | role = server 但**本机就在公网上**（出口 IP = 本机一块网卡 IP）时，横幅显示的"公网 IP" 与"LAN IP"可能相同——这种情况打一行还是两行合并？由 §5 AMBIG-F 用户裁决；本 FR 占位。 | 取决于 AMBIG-F。 |

### 3.3 FR-C · 安装期角色选择（**等待 AMBIG-C 用户裁决再细化**）

> 以下 FR 是**所有候选共有**的硬需求；候选特定的 FR（如 C1 的环境变量名）见 AMBIG-C 决议后由 Architect 落地。

| FR ID | 需求 | AC（可测试） |
|---|---|---|
| FR-C.1 | 安装期必须能区分 server 与 client 两种 role，并据此决定 `UIBindAddr` 默认（server=0.0.0.0、client=127.0.0.1）与横幅形态（FR-B）。 | 安装后 `cat /opt/frp-easy/frp_easy.toml | grep UIBindAddr` 命中：server 安装 = `UIBindAddr = "0.0.0.0"`；client 安装 = `UIBindAddr = "127.0.0.1"`。 |
| FR-C.2 | role 必须可被持久化记录，以便后续诊断（用户日后 `cat /opt/frp-easy/<某文件>` 能看出装的是哪种）。位置由 Architect 决定，**但必须有**——避免"装完看不出是 server 还是 client"。 | 安装后存在一个可读文件含 role 字面量（如 `/opt/frp-easy/.role` = `server` 或 `client`，或 `frp_easy.toml` 内含注释行 `# role = server`，或 `VERSION` 文件追加 role 行）；具体形态 Architect 选。 |
| FR-C.3 | 安装期必须有"未指定 role 时怎么办"的明确行为：拒绝安装并报错 / 默认 server / 默认 client / 进入"安装向导后置模式"。**这条由 §5 AMBIG-C 决议**。 | 取决于 AMBIG-C 候选。 |
| FR-C.4 | 安装期 role 选择必须能在 `curl|sudo bash` 形态下工作（非交互），不要求用户先下载脚本到本地。 | 用户用一条 `curl ... | sudo bash` 形态命令 + 至多一个环境变量 / 一个 URL 参数 / 两个不同的 URL 入口 / 后置 Web UI 选择，能完成 server vs client 选择。具体落地由 AMBIG-C 决议。 |
| FR-C.5 | 安装期 role 选择**只在 install.sh 第一次执行**有效；升级（已存在安装）期间 role 不应被静默改变，行为见 FR-D。 | 已装 server，再跑 `curl ... | sudo bash`（无任何 role 指定）—— `cat frp_easy.toml` 中 `UIBindAddr` 仍为 `0.0.0.0`。 |
| FR-C.6 | 防火墙打开（如 `ufw allow 7800/tcp`、firewalld zone-port）**不在本任务范围**，但 install.sh 必须在 server 模式下**打印**一段诊断文案告诉用户"如果外部访问不到 7800，可能需要：①云厂商安全组 ②本机 ufw/firewalld"。 | server 安装横幅 grep `安全组` 或 `ufw` 或 `firewall` 命中。client 安装横幅**不**打印此段。 |

### 3.4 FR-D · 与 T-011 默认值的兼容/迁移

> 取决于 §5 AMBIG-D 用户裁决。下方先列**所有候选共有**的硬需求。

| FR ID | 需求 | AC（可测试） |
|---|---|---|
| FR-D.1 | 已存在 `frp_easy.toml`（升级路径）—— install.sh 在写入 role-derived `UIBindAddr` 前必须先**读**该文件，对照用户既有显式值。 | install.sh 中包含一段读 `/opt/frp-easy/frp_easy.toml` 的逻辑（grep `frp_easy.toml` 在升级分支命中）；行为取决于 AMBIG-D 候选（D1/D2/D3）。 |
| FR-D.2 | install.sh 改写 `frp_easy.toml` 时**绝不**丢失用户已写的 `DataDir` / `LogDir` / `UIPort` 三个字段（仅 `UIBindAddr` 在 AMBIG-D 决议允许时被改）。 | 升级前 `cat frp_easy.toml`、升级后 `cat frp_easy.toml`，三字段 diff = 空。 |
| FR-D.3 | install.sh 改 `frp_easy.toml` 时必须**先备份**到 `frp_easy.toml.bak.<timestamp>`，备份属主与原文件一致。 | 升级后 `ls /opt/frp-easy/frp_easy.toml.bak.*` 至少命中一个；备份文件 `cat` 内容 = 改前内容。 |

### 3.5 FR-E · 权限模型（**等待 §5 AMBIG-E 用户裁决再细化**）

| FR ID | 需求 | AC（可测试） |
|---|---|---|
| FR-E.1 | install.sh 装完后，systemd unit 中 `User=` 指定的用户必须对 `/opt/frp-easy/frp_easy.toml` 与 `/opt/frp-easy/.frp_easy/` 有**读写**权限。 | `sudo -u $RUN_USER test -w /opt/frp-easy/frp_easy.toml && sudo -u $RUN_USER test -w /opt/frp-easy/.frp_easy/`，退出码 = 0。 |
| FR-E.2 | 二进制 `/opt/frp-easy/frp-easy` 必须对 `User=` 指定的用户**可执行**。 | `sudo -u $RUN_USER test -x /opt/frp-easy/frp-easy`，退出码 = 0。 |
| FR-E.3 | 二进制 `frp_linux/frpc`、`frp_linux/frps`（T-014 引入的运行时下载落地路径）的目录必须对 `User=` 指定的用户**可写**（UI 内下载需要写入）。 | `sudo -u $RUN_USER test -w /opt/frp-easy/frp_linux/`，退出码 = 0。 |
| FR-E.4 | 权限模型必须在升级路径上幂等：再次跑 install.sh 不会把用户已经 chmod/chown 过的私人调整（如外部 systemd-tmpfiles 改的属主）重置——除非用户在 AMBIG-E 显式选 E1（全 chown -R）。 | 装完后手动 `sudo chown myuser:myuser /opt/frp-easy/frp_easy.toml`；再跑 `curl ... | sudo bash` 升级；该文件属主仍为 `myuser:myuser`。**仅当 AMBIG-E ≠ E1 时此 AC 生效**。 |

### 3.6 FR-F · 其他横切

| FR ID | 需求 | AC（可测试） |
|---|---|---|
| FR-F.1 | `verify_all` PASS:19 必须保持。 | `./scripts/verify_all.sh` 退出码 = 0 且尾行 PASS 计数 ≥ 19。 |
| FR-F.2 | 06_TEST_REPORT.md 必须含**英文标题** `## Adversarial tests`（insight-index E.6 红线）。 | `grep -c '^## Adversarial tests' docs/features/install-role-and-public-ip/06_TEST_REPORT.md` ≥ 1。 |
| FR-F.3 | 中文输出（00-core 规则）：所有阶段文档、所有横幅文案、所有错误消息必须中文。 | grep 不出现"Error:" / "Failed:" / "Warning:" 等英文级别词作为消息开头（horizontal banner / journalctl 错误文本除外）。 |
| FR-F.4 | 不允许新增 `verify_all` 检查项（OOS 守护）。 | `scripts/verify_all.sh` diff 不含新增 check 函数。 |

---

## 4. Non-Functional / 边界条件 / 错误路径

### 4.1 NFR

| NFR ID | 需求 |
|---|---|
| NFR-1 | 公网 IP 探测的所有 HTTP 请求**不携带 `Authorization` / `Cookie` / `User-Agent` 中的用户标识**（避免泄漏到第三方 echo IP 服务）。 |
| NFR-2 | 公网 IP 探测的第三方 host 必须在脚本里**明文写死**——用户能在 install.sh 里 grep 出来审计（不允许从环境变量动态拼接，不允许 DNS 查询前先调一次"获取 echo IP 服务列表"的元服务）。 |
| NFR-3 | 升级路径必须保留 T-014 决策："`frp_linux/` 与 `frp_win/` 下用户下载的 frp 二进制不被升级覆盖"。 |
| NFR-4 | 双 shell 验证（NF · T-011）：Linux bash + Windows PowerShell 均跑 verify_all PASS。但本任务**Windows 改动是否同步**由 §5 AMBIG-G 用户裁决。 |
| NFR-5 | 不引入新依赖（无新 Go 包、无新 CLI 工具）。公网 IP 探测用现成 curl / Invoke-WebRequest。 |
| NFR-6 | 不修改 `verify_all.sh` 的检查项数量（NFR-4 与 T-011 一致）。 |

### 4.2 边界条件 / 错误路径

| BC ID | 场景 | 期望行为 |
|---|---|---|
| BC-1 | 公网 IP 探测**超时**（所有 echo IP 服务无响应） | 横幅打印明确的中文降级文案（如 `公网 IP 探测失败（请手动确认）`），不阻塞 install.sh `exit 0`。 |
| BC-2 | 公网 IP 探测**返回 HTML 错误页**（而非 IP） | 探测视为失败，走 BC-1 降级文案，**绝不**把 HTML 片段当 IP 打印。 |
| BC-3 | 公网 IP 探测返回 **IPv6** 地址（IPv4 服务不可达 / IPv6-only 网络） | 横幅打印 `http://[<IPv6>]:7800` 或同等合法形式；不打印不合法 URL（`http://2001:db8::1:7800` 不合法）。 |
| BC-4 | role = server 但**本机无公网 IP**（纯内网 VM、家庭 NAT 后） | 探测可能成功（返回 NAT 出口公网 IP）也可能失败；若返回的公网 IP 不指向本机，**仍然**打印它（因为这是 frpc 客户端连本机 NAT 转发后看到的入口），但同时打印中文提示"该 IP 可能为 NAT 出口，需要在路由器/云厂商安全组开 7800 端口转发"。 |
| BC-5 | role = client 且 `--public-ip` / `FRP_EASY_PUBLIC_IP=` 之类参数被显式传入 | client 模式**忽略**公网 IP 输入（client 不需要公网 IP），警告一行"client 模式忽略公网 IP 参数"后继续。 |
| BC-6 | TTY 不可用（`curl ... | sudo bash`）且 AMBIG-C 选 C3（重打开 /dev/tty 提示）| 若 /dev/tty 也不可用 → 走该候选定义的默认值（要在 AMBIG-C 决议时明确"默认 = server 还是 client"）。 |
| BC-7 | 升级期已存在 `frp_easy.toml` 且**显式**写了 `UIBindAddr = "127.0.0.1"`，本次升级用户选 server | 按 §5 AMBIG-D 决议处理：D1 拒绝覆盖（保留 127.0.0.1，警告 role 与配置不符）；D2 备份后覆盖；D3 询问用户。 |
| BC-8 | 升级期已存在 `frp_easy.toml` 且 `UIBindAddr` 是合法但非 0.0.0.0 / 127.0.0.1 的具体 IP（如 `192.168.1.10`） | 视为"用户高级用法"，不动 UIBindAddr，仅打印警告。 |
| BC-9 | 容器 / WSL2 / WSL1 / LXC | systemd 可用性各异。install-service.sh 已有 `command -v systemctl` 探测；本任务的"装完 active"AC 仅在 systemd 可用环境断言。WSL1 / 无 systemd 环境 install-service.sh 直接 exit 1，不属本任务关注的失败路径。 |
| BC-10 | 同主机多次跑 `curl ... | sudo bash`（幂等性） | role 选择必须幂等：第二次跑（无 role 参数）必须保留第一次的 role，不静默切换。第二次跑（带 role 参数且与第一次不同）行为见 AMBIG-H。 |
| BC-11 | install.sh 步骤 5 下载失败 / 步骤 6 解包失败 | 早于 role 适用阶段——保留 T-016 的现有失败语义（exit 1 + 中文报错），不变。 |
| BC-12 | macOS 安装路径（install.sh 在 `OS=darwin` 分支打印手动启动提示后 exit 0） | macOS 无 systemd——role 选择对 macOS 是否有意义？建议 OOS（见 §6），但需在 AMBIG 中确认。 |

---

## 5. AMBIGUITIES（用户必须裁决）

> **本任务的核心交付**。下面每一条用户必须给出明确选择，Stage 2 Architect 才能动手。
> 候选答案标号供用户回复时引用（"AMBIG-C 选 C1"）。

### AMBIG-C · 安装期角色选择形态（**最重要**）

> 用户原话："理论上好像只能在安装脚本运行过程中选择是服务端还是客户端"。
> 用户**没有**明确说一定要"装机时选"——只说"理论上好像只能"。所以"是否后置选择（C5）"也是候选。

**候选 C1：环境变量**

形态：`curl -fsSL <url> | FRP_EASY_ROLE=server sudo -E bash`（或 `=client`）。
- 优点：bash 单行；非交互；与 T-012 / T-016 现有 `FRP_EASY_INSTALL_DIR` 同款机制；用户可在历史命令里看出装的是 server 还是 client。
- 缺点：用户必须知道 `-E` 让 sudo 透传环境变量（默认 sudo 清环境）；不带 `-E` 时静默走默认 role。
- 子问题 C1.a：未指定 `FRP_EASY_ROLE` 时默认是什么？（a）拒绝安装并报错；（b）默认 server；（c）默认 client；（d）默认 server 但打印警告。

**候选 C2：命令行参数（要求先下载到本地）**

形态：`curl -fsSL <url> -o install.sh && sudo bash install.sh --role server`。
- 优点：参数最直观；不依赖 `-E`；help 文本可直接列出来。
- 缺点：用户必须**两步**（curl -o、bash），破坏"一键"承诺；与 T-012 的"`curl|bash` 一键"红线冲突。
- 子问题 C2.a：是否仍保留 `curl|bash` 形态（无 --role 走默认）？默认是什么？

**候选 C3：重新打开 /dev/tty 提示**

形态：`curl ... | sudo bash` —— 脚本内部 `exec < /dev/tty` 重接交互输入，提示 "请选择 [1] 服务端 [2] 客户端"。
- 优点：用户体验最像传统交互安装器；不需要额外参数或环境变量。
- 缺点：CI / 容器 / SSH 异步执行环境 /dev/tty 不可用，必须有非 TTY 降级路径；自动化用户不喜欢被弹问题。
- 子问题 C3.a：/dev/tty 不可用时默认 role 是什么？
- 子问题 C3.b：是否同时支持环境变量覆盖（让 CI 用户能 bypass）？

**候选 C4：两条独立的"一键服务端 / 一键客户端"命令**

形态：
- 服务端：`curl -fsSL https://.../install-server.sh | sudo bash`
- 客户端：`curl -fsSL https://.../install-client.sh | sudo bash`
- 优点：README 文案最直白；用户无须理解任何参数语义。
- 缺点：两个脚本入口的维护成本；可能落地为"一个脚本 + 不同 wrapper" 或"一个脚本 + 不同 URL query"（但 GitHub raw 不解析 query）。
- 子问题 C4.a：实现上是真两个文件，还是一个文件 + 两个 raw URL 指向同一个 commit 但用 wrapper（如 `install-server.sh` 内容 = `FRP_EASY_ROLE=server exec bash <(curl ...) install.sh`）？

**候选 C5：装机时不选，装完先以"127.0.0.1 安装向导"形式跑，向导内二选一**

形态：install.sh 永远装"中立"状态（绑 127.0.0.1，无明确 role）；启动 UI 后**进入向导第一步**：服务端 / 客户端二选一；用户选完后 UI 自动 rewrite `frp_easy.toml` + 触发 `systemctl reload-or-restart frp-easy`（或写一个 `restart-self` 子进程）。
- 优点：保留"一键"承诺；用户在熟悉的 Web UI 内选择；可附带其它向导步骤（管理员密码、frp 二进制下载等）合并一处。
- 缺点：装完到向导完成之间的时间窗 UI 仅监听 127.0.0.1——server 用户必须能用 ssh tunnel / sudo systemctl edit 临时改绑定才能远程接入向导；与用户原话"理论上好像只能在安装脚本运行过程中选择"略有出入；改 `frp_easy.toml` 后 reload 的实现复杂度（systemd reload vs 子进程 SIGHUP vs 重启）非平凡。
- 子问题 C5.a：server 用户初次远程接入向导的路径——SSH tunnel？UI 内点一个按钮触发"切换到 0.0.0.0 重启"？
- 子问题 C5.b：向导完成前 UI 是否需要"setup token"防护？（暴露面：装完到向导完成期间 UI 无密码——T-011 安全提示已覆盖此风险，但 C5 让窗口变长）

**默认选项倾向**：本文档**不**给倾向。Architect 文档无法继续直到 AMBIG-C 决议。

---

### AMBIG-D · 升级期已有 frp_easy.toml 的处理

候选 D1：**保留用户值优先**——已有 `UIBindAddr` 显式值则保留，本次 role 选择仅影响**新建** `frp_easy.toml` 与横幅打印；横幅可警告"检测到 role=server 但配置文件 UIBindAddr=127.0.0.1，本次升级未改动配置"。
- 优点：完全兼容 T-011 NF-2"用户显式值优先"语义；最不破坏。
- 缺点：用户从 client 升级到 server 时必须**手动**改 `frp_easy.toml`，否则升级后服务端仍然只监听 127.0.0.1（与 role 名义不符）。

候选 D2：**role 显式时覆盖**——只要本次 install.sh 收到了明确的 role 参数（环境变量 / CLI flag），就 backup + rewrite 配置文件的 `UIBindAddr`。
- 优点：用户切换 role 时一条命令完成。
- 缺点：违反 T-011 NF-2 语义；用户对配置文件的精细调整可能被无声重置（注释丢失、字段顺序变化）。

候选 D3：**交互询问**——升级期检测到冲突时 install.sh 用 §5 AMBIG-C 的同款机制（环境变量 `FRP_EASY_FORCE_REWRITE=yes` / `--force-rewrite` / `/dev/tty` 提问）让用户裁决。
- 优点：行为透明。
- 缺点：增加心智负担；与 C1-C5 的非交互精神冲突。

**默认选项倾向**：本文档无倾向。

---

### AMBIG-E · 权限模型

候选 E1：`chown -R $RUN_USER:$RUN_USER /opt/frp-easy/`——install.sh 步骤 6 末尾全量 chown。
- 优点：所有读写路径"一刀切"工作；T-014 frp 二进制下载也 OK。
- 缺点：`$RUN_USER` 拥有 binary 本身 + scripts/——任何能改 binary 的进程都能升级权限（虽然 `$RUN_USER` 已经在跑 binary，但从最小权限审计视角不完美）；用户后续 `sudo` 改 binary 需要先 `sudo chown root` 才符合常识。

候选 E2：仅 chown `frp_easy.toml` + `.frp_easy/`——binary、scripts、`frp_linux/`、`frp_win/` 保持 root:root；只有"运行时需写入"的路径属于 RUN_USER。
- 优点：最小权限；与 Linux FHS 习惯一致（`/opt/<app>/` root 拥有，运行时数据另放）。
- 缺点：`frp_linux/` 的 frp 二进制下载需要 RUN_USER 能写——必须显式给 `frp_linux/` chown。

候选 E3：install.sh 装完后**预先生成** `/opt/frp-easy/frp_easy.toml`（含 role 选择结果）再 chown 给 RUN_USER —— appconf.Load() 走"文件已存在"分支，不再尝试写默认值。
- 优点：彻底回避 `appconf.Load()` 在缺失文件时尝试写入的权限问题；首启确定性强。
- 缺点：与 D2/D3 升级语义耦合（升级期已有文件就不再"预生成"）；install.sh 需嵌入 TOML 模板（小复杂度）。
- 注：E3 可与 E1 或 E2 组合（"先预生成再 chown 局部 / 全量"）。

候选 E4：unit 改 `User=root`——绕过所有权限问题。
- 优点：实现成本最低。
- 缺点：违反最小权限；frp-easy 进程以 root 运行管理 UI 是安全大忌。

候选 E5：install.sh 自动 `useradd --system --no-create-home frp-easy`，专用服务用户。
- 优点：与企业级 systemd 服务实践一致（如 nginx:nginx、postgres:postgres）；与现有"SUDO_USER 默认"冲突，但更安全。
- 缺点：副作用大（新建系统用户）；卸载时是否 userdel？现 uninstall-service.sh 不动用户。
- 注：E5 也可与 E1/E2/E3 组合。

**默认选项倾向**：本文档无倾向。但需指出：**E2 + E3 组合**在工程审计视角最干净（最小权限 + 首启确定性），但用户必须确认接受 frp_linux/ 也被 chown 这一点。

---

### AMBIG-F · 公网 = LAN 时的横幅形态

候选 F1：合并为一行 `公网/局域网访问：http://<IP>:7800`（出口 IP = 本机一块网卡 IP 时）。
候选 F2：仍打两行，标注"公网 IP 与局域网 IP 相同（本机直接在公网上）"。
候选 F3：只打"公网访问"一行，省略 LAN 行。

---

### AMBIG-G · Windows install.ps1 是否同步本任务改动

候选 G1：**同步**——install.ps1 也加 role 选择、公网 IP 探测；Windows Service 默认 LocalSystem 不存在 unit User= 那个根因，但仍需公网 IP 探测和 role 标记。
- 优点：跨平台一致；README 不分叉。
- 缺点：本任务工作量翻倍。

候选 G2：**部分同步**——只同步公网 IP 探测（FR-B），role 选择形态推迟。
- 优点：用户原话提到的是"安装脚本"，Linux 实测崩溃；Windows 用户可后续单独决定。
- 缺点：跨平台一致性降低；README 需说明 Windows 不支持 role 选择。

候选 G3：**完全不动 Windows**——本任务仅 Linux/macOS。
- 优点：工作量最小，聚焦根因。
- 缺点：明显跨平台分叉。

---

### AMBIG-H · 同主机重复执行（带不同 role 参数）

候选 H1：第二次跑 + 不同 role —— **拒绝并报错**"已检测到 role=server，再次安装请先 uninstall 或 export FRP_EASY_FORCE_ROLE=yes"。
候选 H2：第二次跑 + 不同 role —— **静默切换**（按 D2 改 frp_easy.toml + 重启服务）。
候选 H3：第二次跑 + 不同 role —— **走 D3 交互询问**。

---

### AMBIG-I · 是否在 install.sh 中改 frp 业务侧默认（frpc / frps 切换）

> 这一条 PM 已倾向 OOS，但需用户**显式确认**在 §6 OOS 中划走。
> 因为用户原话说"服务端 vs 客户端"——除了 UI 监听地址外，还可能意味着 frp_easy 默认运行 frps 还是 frpc？

候选 I1：**OOS（推荐）**——本任务只管 UI 监听地址 + 横幅；frp 进程类型（frps/frpc）由用户进入 UI 后自己加进程定义。
候选 I2：**包含**——role=server 自动给 frps 创建一个默认进程定义、role=client 自动给 frpc 创建一个默认进程定义。

---

## 6. Out-of-scope（OOS · 显式不做的事）

| OOS ID | 不做的事 | 理由 |
|---|---|---|
| OOS-1 | FRP 业务侧（frpc/frps）默认值的改动 —— 例如 server 自动生成 frps 默认进程、client 自动生成 frpc 默认进程 | 用户原话未明确要求；AMBIG-I 候选 I1 倾向 OOS；本任务先解决"UI 自身能起来 + IP 显示对"。**待用户在 AMBIG-I 确认。** |
| OOS-2 | Windows install.ps1 改动 | 取决于 AMBIG-G。**默认按 G3（完全不动 Windows）走，待用户确认。** |
| OOS-3 | 防火墙打洞 / ufw / firewalld 自动配置 | 跨发行版差异大；安全敏感（自动开端口与"最小惊扰"冲突）；FR-C.6 仅要求**提示**，不要求自动执行。 |
| OOS-4 | 修改 T-011 的"用户显式值优先"语义（默认值层面） | 仅在 AMBIG-D = D2 / D3 时，对**已有 frp_easy.toml** 的本地 rewrite 行为做局部覆盖，**不**改 Default() / Load() 函数本身。 |
| OOS-5 | 公网 IP 探测的 IPv6 / 代理 / SOCKS 高级配置 | 现有 `curl -fsSL` 走系统代理已足够；不引入 `--proxy` / `--socks` 等参数。 |
| OOS-6 | systemd User= 之外的隔离机制（NoNewPrivileges / ProtectHome / CapabilityBoundingSet 等强化） | 本任务聚焦"装完能跑"。systemd 强化在另一个 feature（如 `harden-systemd-unit`）单独做。 |
| OOS-7 | 修改 verify_all.sh / baseline.json / `.harness/insight-index.md`（除非本任务确实产生新 insight） | 与 T-011/T-014/T-016 保持一致。 |
| OOS-8 | 修改 `web/` 前端代码（除非 AMBIG-C = C5 时不可避免——见 OOS-9） | 用户原话明确指向"安装脚本"。 |
| OOS-9 | 若 AMBIG-C = C5（"装完进向导二选一"），前端**新增** role 选择步骤的实现成本 | C5 选中后将变成 in-scope，但 RA 阶段先标 OOS-9——若 C5 被采纳，本表 OOS-9 自动撤销，由 Architect 在 02 §"分区分配"中分给 `dev-frontend`。 |
| OOS-10 | macOS 路径的 role 选择（macOS 走 install.sh `darwin` 分支已 exit 0，无 systemd） | 见 BC-12；按现状不动。 |
| OOS-11 | 修改 `release.yml` / GitHub Actions / 滚动发布 tag 机制 | 本任务在已发布的 install.sh 上做改动，不动 CI。 |
| OOS-12 | LICENSE / NOTICE 文档改动 | 与 T-012 决策保持。 |

---

## 7. 与历史任务的关系

| 历史任务 | 关系 | 本任务的承接 |
|---|---|---|
| **T-011** readme-refresh-and-network-defaults | 决定 `Default() = 0.0.0.0`、安全提示重构 | **本任务是 T-011 的精细化**：把"一刀切默认 0.0.0.0"拆为 role-based（server=0.0.0.0、client=127.0.0.1）。FR-D 必须显式处理与 T-011 NF-2"用户显式值优先"的兼容（AMBIG-D）。`cmd/frp-easy/main.go` 的安全提示文案逻辑保留不动——本任务仅在 install.sh 期决定 `UIBindAddr` 初值。 |
| **T-012** one-click-install-and-mit-license | 决定 install.sh / install.ps1 整体形态（`curl|bash`、TMP_DIR、INSTALL_DIR） | **本任务在 T-012 的脚本框架上加 role 分支**。AMBIG-C 的所有候选都必须兼容 T-012 §3.1 的"`curl|bash` 非交互"约束。 |
| **T-013** rolling-release-install | 决定固定 `rolling` tag 滚动发布、`API_URL=/repos/.../releases/tags/rolling` | 本任务**不**改 release.yml；install.sh 的 API_URL 与解析逻辑保留。 |
| **T-014** frp-binary-auto-download | 决定 frp 二进制运行时下载、`frp_linux/` `frp_win/` 不再随包分发；升级期保留这两目录用户下载产物 | **本任务的 FR-E.3（frp_linux/ 必须 RUN_USER 可写）直接源自 T-014**。AMBIG-E 任何候选都必须保证 frp_linux/ chown 正确。 |
| **T-016** install-progress-and-systemd-unit-fix | 修 install.sh 步骤 5 进度条、systemd unit 语法（裸 token + `\x20`）、退出码透传、enable--now 失败诊断 | **本任务的崩溃根因（FR-A）是 T-016 的遗留**：T-016 修好了 unit 语法，但 `User=$SUDO_USER` 与 `cp -a` 后的文件属主不匹配未处理。本任务**不**改 T-016 已修复的 unit 语法、不改进度条逻辑、不改 systemd-analyze verify 自检；仅在 install.sh 步骤 6 之后、步骤 7（调 install-service.sh）之前**新增** chown / 预生成 toml 逻辑（取决于 AMBIG-E）。 |
| **T-008** deploy-kit | 引入 `${SUDO_USER:-$(id -un)}` 默认运行用户机制 | 本任务**保留**该机制不动；权限问题在 install.sh 解包后阶段处理，不在 install-service.sh 内。 |
| **T-002** zero-config-quickstart | 引入 UI 内 frp 二进制下载横幅 | 本任务的 FR-E.3 保护这条路径在 server / client 双模式下都能用。 |

---

## 8. Verdict

**BLOCKED ON USER**

未决议的 AMBIGUITIES（共 8 条 + 多个子问题）：

| ID | 待决 | 阻塞下游 |
|---|---|---|
| AMBIG-C | 安装期角色选择形态（C1-C5）+ C1.a / C2.a / C3.a / C3.b / C4.a / C5.a / C5.b 子问题 | FR-C 全部 AC、Architect §3（执行模型）、AMBIG-D / AMBIG-H |
| AMBIG-D | 升级期已有 frp_easy.toml 处理（D1-D3） | FR-D 全部 AC、Architect §9（迁移） |
| AMBIG-E | 权限模型（E1-E5、可组合） | FR-A.2 / FR-A.5 / FR-E 全部 AC、Architect §6（流程） |
| AMBIG-F | 公网 IP = LAN IP 时的横幅形态（F1-F3） | FR-B.2 / FR-B.7 |
| AMBIG-G | Windows install.ps1 是否同步（G1-G3） | OOS-2、本任务工作量边界、Architect §11（分区分配） |
| AMBIG-H | 同主机重复执行 + 不同 role（H1-H3） | BC-10、FR-C.5 |
| AMBIG-I | frp 业务侧 frps/frpc 默认是否同步（I1-I2） | OOS-1 |
| AMBIG-J | macOS 路径是否纳入（BC-12 / OOS-10）| 边缘场景；可与 G 一同决议（"非主要平台不动"） |

PM 在用户回复以上 AMBIG 后，把决议追加到 `INPUT.md` 末尾或 `PM_LOG.md`，并重派 RA（增量编辑本文档将 verdict 改为 READY），再派 Stage 2 Architect。

---

## 9. 给 Architect 的预输入（决议后即可用）

> 一旦 AMBIG 全部决议，以下事实是 Architect Stage 2 的直接输入（不需要再次走需求路径）：

1. **崩溃根因已锁定**（§1.4），不需要 Architect 再做根因分析；只做修复方案选型。
2. **公网 IP 探测的可信源**：建议候选清单（Architect 自选，本 RA 不锁定）—— `https://api.ipify.org`、`https://ifconfig.me/ip`、`https://icanhazip.com`、`https://ident.me`、`https://checkip.amazonaws.com`。所有都返回纯文本 IP（无 JSON parsing 成本）；NFR-2 要求 ≥ 2 个明文写死。
3. **role 持久化位置**（FR-C.2）：Architect 选——`/opt/frp-easy/.role` / `/opt/frp-easy/VERSION` 附加行 / `frp_easy.toml` 注释。本 RA 不锁定，但要求**有**。
4. **chown 时机**：必须在 install.sh **步骤 6 末尾、步骤 7 之前**——install-service.sh 调用时 unit 中的 `User=` 才能立即对 `/opt/frp-easy/` 拥有读写。
5. **insight-index 候选新行**（仅供 Architect / Developer 参考，由 07 stage 落地）：
   - "install.sh 解包后必须 chown 给 RUN_USER，否则 systemd User=RUN_USER 进程对 /opt/<app>/ 无写权 → appconf.Load() 写默认值失败 → 死循环重启。"（仅在本任务 verify 后追加。）
   - "公网 IP 探测必须先判 HTTP 状态码再读 body；echo IP 服务返回 HTML 错误页时不要当 IP 用。"（与 T-014 GitHub API 类似但场景不同。）

---

**RA 责任到此为止**——AMBIG 等用户。
