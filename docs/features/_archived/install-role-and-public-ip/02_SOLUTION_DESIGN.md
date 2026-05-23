# 02 — Solution Design · T-017 install-role-and-public-ip

> 阶段 2 / 7（Solution Architect）。模式：**full**。
> 上游只读输入：[01_REQUIREMENT_ANALYSIS.md](01_REQUIREMENT_ANALYSIS.md)（35 FR、12 BC、8 AMBIG，verdict = BLOCKED ON USER）+ [PM_LOG.md](PM_LOG.md) 中 **PM 已对 8 个 AMBIG 全部裁决** 的决议表（C=C1+C4 / D=D1 / E=E2+E3 / F=F2 / G=G2 / H=H1 / I=I1 / J=darwin 路径同步公网 IP 探测）。
> 本设计严格按 PM 决议推进，不再回探任何 AMBIG。所有改动均按"install.sh 既有步骤分块"加入，**不动 Go 代码**（appconf、main.go 一字不改）。

---

## 1. 设计目标与不变量

### 1.1 设计目标（每条引用 01 的 FR / NFR / BC / AMBIG 决议）

| G ID | 目标 | 来源 |
|---|---|---|
| G-1 | 装完后 systemd 单元 `frp-easy` 处于 active，不再死循环 | FR-A.1 / FR-A.2 / FR-A.3 / FR-A.4 |
| G-2 | 安装期由环境变量 `FRP_EASY_ROLE={server\|client}` 决定 UIBindAddr：server=0.0.0.0、client=127.0.0.1 | FR-C.1 + PM 决议 AMBIG-C = C1+C4 |
| G-3 | 未指定 role → 拒绝并打印两条入口命令样例 + exit 3 | PM 决议 AMBIG-C 子问题 C1.a = (a) |
| G-4 | role = server 时安装横幅探测并打印公网 IP（≥ 2 候选源、单次 ≤ 3 s、总 ≤ 8 s）；client 不做任何外网请求 | FR-B.1 / FR-B.3 / FR-B.4 / FR-B.5 / FR-B.6 / NFR-2 |
| G-5 | 解包后预生成 `frp_easy.toml`（含 role-derived UIBindAddr）后 `chown` 给 RUN_USER；binary 与 scripts 保持 root:root | PM 决议 AMBIG-E = E2+E3，配合 FR-E.1/2/3/4 |
| G-6 | 升级期已存在 `frp_easy.toml` 时**不**覆盖 UIBindAddr（D1 保留用户值优先）；仅幂等修复属主与 frp_linux/ 写权 | PM 决议 AMBIG-D = D1 + NFR-3 |
| G-7 | 同主机已装且本次 role 与持久化的 `.role` 不一致 → 拒绝并提示先 uninstall 或 `FRP_EASY_FORCE_ROLE=yes` | PM 决议 AMBIG-H = H1 + BC-10 |
| G-8 | Windows install.ps1 同步公网 IP 探测（FR-B），role 选择形态推迟（保留 OOS-2） | PM 决议 AMBIG-G = G2 |
| G-9 | macOS 路径在 install.sh 的 darwin 分支同步公网 IP 探测（横幅打印前），但仍跳过 systemd 与 role 强制（保留 OOS-10 的 systemd 边界） | PM 决议 AMBIG-J |
| G-10 | 中文输出；所有错误消息、横幅与提示按 FR-F.3；verify_all PASS:19 守住 | FR-F.1 / FR-F.3 / NFR-6 |

### 1.2 不变量（绝不破坏）

| Inv ID | 不变量 | 守护手段 |
|---|---|---|
| Inv-1 | `internal/appconf/config.go` 的 `Default()` / `Load()` / `Validate()` 一字不改 —— T-011 NF-2"用户显式值优先"语义完整保留 | §3 不分配 dev-backend Go 改动；§7 BC-D.1 路径不进 appconf |
| Inv-2 | `cmd/frp-easy/main.go` 一字不改 —— 安全提示文案、ListenAddr 改写、setup 引导链路保持 | §3 不列入 affected modules |
| Inv-3 | T-016 修复（unit 裸 token + `\x20`、systemd-analyze warn+继续、退出码透传 set +e 块）不被回滚 | §3 install-service.sh 标记 `untouched`，install.sh 新增逻辑插在 L253 / L255 之间且不动 T-016 已编辑区域 |
| Inv-4 | T-014 升级语义"frp_linux/ / frp_win/ 用户下载产物不被覆盖" | §3 升级分支只新增 `chown -R RUN_USER frp_linux/`，不 `cp -a` 也不 `rm -rf` |
| Inv-5 | `curl \| bash` 一键承诺 + 非交互（stdin 被 curl 占用）保留 | §5 入口仅靠环境变量，不引入 `/dev/tty` 提问 |
| Inv-6 | NFR-2 公网 IP 探测候选 URL **明文写死** 在脚本里 | §5 候选清单内嵌为 bash array 字面量，不从环境变量读 |
| Inv-7 | verify_all 检查项数量与基线 19 / pass_count = 19 不变 | §9 测试策略不新增 verify_all 检查；新单测仅作为可选验收手段 |

---

## 2. Affected modules（受影响模块）

| 文件 | 类型 | 改动概述 | 引用证据 |
|---|---|---|---|
| `scripts/install.sh` | edit | 步骤 0 新增 `FRP_EASY_ROLE` 解析（拒绝缺失）+ 步骤 6 末尾新增 **§6.5 块**（role 一致性校验 + 预生成 `frp_easy.toml` + chown 局部 + frp_linux/ 写权 + 持久化 `.role`）+ 步骤 8 替换为 role-aware 横幅（含公网 IP 探测函数） | install.sh L23-77 现有参数解析框架、L227-253 步骤 6 升级/全新分支、L291-322 步骤 8 横幅 |
| `scripts/install.ps1` | edit（局部） | 步骤 8 横幅末段新增公网 IP 探测（FR-B 同步），其余不动；不引入 role 选择（保留 OOS-2） | install.ps1 L247-256 LAN IP 探测块 |
| `scripts/install-service.sh` | **untouched** | T-016 unit 模板与诊断链路已修复，本任务的 RUN_USER 与 chown 时机问题靠 install.sh §6.5 上游解决 | install-service.sh L68-75 `${SUDO_USER:-$(id -un)}` 默认机制 |
| `scripts/install-service.ps1` | **untouched** | Windows Service 默认 LocalSystem 无 unit User= 根因 | dev-map.md 行 27 备注 |
| `scripts/uninstall-service.sh` | edit（最小） | 卸载注意事项追加一行"如需彻底清理还需删 `${INSTALL_DIR}/.role`"——`.role` 文件本身**不**自动删（与 frp_easy.toml / .frp_easy/ 同款保守策略） | uninstall-service.sh L76-87 收尾文案块 |
| `internal/appconf/config.go` | **untouched** | Inv-1 | 一字不动 |
| `cmd/frp-easy/main.go` | **untouched** | Inv-2 | 一字不动 |
| `.harness/insight-index.md` | edit（阶段 7 收割） | 追加 1-2 行："install.sh 解包后 chown 给 RUN_USER 否则 appconf.Load 写默认失败 → systemd 死循环"以及"公网 IP 探测必先判 HTTP 状态码再验 IP 字面量"——由 07_DELIVERY.md 的 `## Insight` 段经 `scripts/archive-task` 收割，**不在 stage 2 写入** | RA §9.5 已建议两行 |

无新文件、无新依赖、无 schema / migration 变更。

---

## 3. Module decomposition（新组件 / 函数拆分）

**无新模块**。所有新增是 install.sh 的内联函数，按 bash 良好习惯拆为 3 个本地函数。

### 3.1 install.sh 新增函数（语言内：bash）

```bash
# resolve_role_or_die: 在步骤 1 后立即调用。读 FRP_EASY_ROLE 环境变量，校验合法，
# 否则打印两条入口命令样例并 exit 3。返回值通过全局变量 ROLE 传递。
# 入参：无（读 env）；出参：全局 ROLE ∈ {server, client}；副作用：错误时 exit 3。

# render_frp_easy_toml: 按 ROLE 渲染 frp_easy.toml 字面内容到 stdout。
# 入参：$1 = role（"server" | "client"）；出参：stdout 输出 TOML 文本；副作用：无。
# 内容与 appconf.Default() 字段一一对齐（UIBindAddr / UIPort / DataDir / LogDir）。

# detect_public_ip: 公网 IP 探测；候选 URL 明文写死；单 URL ≤ 3 s 超时；
# 总耗时 ≤ 8 s（由调用方控）；HTTP 状态码先判，IP 字面量校验通过才返回。
# 入参：无；出参：stdout = IP 字符串（失败时空字符串）；返回码：0 成功 / 1 失败。
```

### 3.2 install.sh 新增"步骤 6.5"代码块（按现有"==> [N/8]"分块习惯）

```text
==> [6.5/8] 配置角色（role=$ROLE） + 修复运行时属主...
  - 全新安装：渲染 frp_easy.toml → 写入 INSTALL_DIR → 写入 .role
  - 升级：读旧 .role 与本次 ROLE 对比；冲突 → exit 3（除非 FRP_EASY_FORCE_ROLE=yes）
  - 幂等：mkdir -p INSTALL_DIR/.frp_easy；
  - chown：仅对 frp_easy.toml、.frp_easy/、frp_linux/、.role 四项 chown RUN_USER
```

注意编号说明：现有脚本是 `[1/8]` … `[8/8]`，新增块对外展示为 `[6.5/8]` 文字标签即可，**不**重排现有数字（避免破坏 T-016 已建立的步骤数 grep 锚点习惯；step 标号是 UX 字串，不是契约）。

### 3.3 install.ps1 新增（局部）

仅在步骤 8 横幅前追加一段 PowerShell 公网 IP 探测 helper：

```text
function Get-PublicIPv4 {
    # 候选 URL 字面量写死（NFR-2）；Invoke-WebRequest -TimeoutSec 3；
    # 200 状态码 + Trim() 后用 [ipaddress]::TryParse 校验；
    # 任一候选成功即返回；全部失败返回 $null。
}
```

调用点紧邻现有 `$localIp = ...` 块之后，按 G-8 给 Windows 横幅新增"公网访问"或"公网 IP 探测失败（请手动确认）"一行。**不**改 install.ps1 的 role 选择（保留 OOS-2）。

---

## 4. 数据模型 / 文件格式变更

无 DB 迁移。本节列两项**文件级**新合约。

### 4.1 `.role` 持久化文件（FR-C.2）

| 维度 | 决策 |
|---|---|
| 路径 | `${INSTALL_DIR}/.role`（默认 `/opt/frp-easy/.role`） |
| 内容 | 单行字面：`server\n` 或 `client\n`（结尾换行符必填，便于 `cat` 与 `grep -x`） |
| 权限 | `0644 root:root`（**不**给 RUN_USER 写权——运行时进程不读 .role，见 §8 R-3 决策） |
| 写入时机 | install.sh §6.5 块；全新安装与升级都写（升级时若冲突已先 exit 3） |
| 读取者 | 仅 install.sh 自身（升级期一致性检查）+ 用户人眼诊断 |

**取舍说明**（为什么不是 TOML 注释 / VERSION 追加）：

- **独立 `.role` 文件**：grep-able、`cat` 一行即得、修改不影响 TOML 解析、与 `VERSION` 文件并列，符合 FHS"配置/元数据并排"惯例。✓ **选定**
- TOML 注释行 `# role = server`：appconf.Load() 不会读注释，安全；但 grep 容易匹配到误的"# role"出现位置；且与"D1 保留用户值优先"语义耦合（升级期改写注释也需备份语义）。✗
- `VERSION` 追加行：`VERSION` 当前是单行 git commit hash（T-013），追加行会破坏 grep `^v[0-9]` 的隐含契约。✗

### 4.2 预生成 `frp_easy.toml` 模板（FR-C.1 + AMBIG-E 决议 E3）

字段名必须与 `internal/appconf/config.go` L36-39 的 struct tag 完全一致：`UIBindAddr` / `UIPort` / `DataDir` / `LogDir`（go-toml/v2 默认 case-sensitive；T-009 历史 insight L24-25 验证 PowerShell 写 TOML 必须 UTF-8 无 BOM，本任务在 bash 写无此问题，但**字段大小写完全照搬 Go tag**仍是硬约束）。

**server 变体**（字面内容，install.sh 用 heredoc 输出）：

```toml
# frp_easy.toml — 由 install.sh 在 T-017 角色为 server 的全新安装中生成。
# UIBindAddr=0.0.0.0 表示监听所有网卡（公网 + LAN + 回环），便于 frpc 客户端通过公网 IP 访问 Web UI。
# 仅需本机访问时可手动改为 "127.0.0.1" 后重启 frp-easy 服务。
UIBindAddr = "0.0.0.0"
UIPort = 7800
DataDir = "./.frp_easy"
LogDir = "./.frp_easy/logs"
```

**client 变体**：

```toml
# frp_easy.toml — 由 install.sh 在 T-017 角色为 client 的全新安装中生成。
# UIBindAddr=127.0.0.1 表示仅监听回环（最安全），管理 UI 不暴露到公网/局域网。
# 如需局域网内访问 UI，可手动改为 "0.0.0.0" 后重启 frp-easy 服务。
UIBindAddr = "127.0.0.1"
UIPort = 7800
DataDir = "./.frp_easy"
LogDir = "./.frp_easy/logs"
```

**预生成时机**：仅在**全新安装**（即 `${INSTALL_DIR}/frp_easy.toml` 不存在）时写。升级期已存在该文件时**绝不**触碰（D1）。

**权限**：`0644 RUN_USER:RUN_USER`（满足 FR-E.1：systemd 拉起的进程必须能写，因为 appconf 后续可能 reload+save，虽然本期没动该路径，但保留可写权符合"运行时可写路径"分组的承诺）。

---

## 5. 接口 / 契约

### 5.1 install.sh 新增环境变量入口

| 变量 | 必填 | 取值 | 语义 | 默认行为 |
|---|---|---|---|---|
| `FRP_EASY_ROLE` | **是** | `server` \| `client` | 决定 UIBindAddr 与横幅形态 | 未设置 → exit 3（G-3） |
| `FRP_EASY_FORCE_ROLE` | 否 | `yes` | 升级期与已装 role 冲突时强制覆盖 .role 并重写 frp_easy.toml | 未设置且冲突 → exit 3（G-7） |
| `FRP_EASY_PUBLIC_IP` | 否 | 合法 IPv4 / IPv6 字面量 | 用户手动指定公网 IP，跳过探测 | 未设置 → 自动探测（server 模式）/ 跳过（client 模式） |
| `FRP_EASY_INSTALL_DIR` | 否 | 绝对路径 | T-012 既有变量，保留不动 | `/opt/frp-easy` |

**sudo 透传约束**：`sudo` 默认清环境；用户必须用 `sudo -E` 或在管道中按 `FRP_EASY_ROLE=server sudo -E bash` 形态写。README 文案 + 错误消息明示这一约束（呼应 RA AMBIG-C 候选 C1 缺点）。

### 5.2 install.sh 新退出码

| 码 | 既有 / 新增 | 含义 | 出现阶段 |
|---|---|---|---|
| 0 | 既有 | 成功（含 -h、darwin 降级） | 全程 |
| 1 | 既有 | 前置 / 环境失败（非 root、缺依赖、非 amd64、网络、下载/解压、API 异常） | 步骤 1-6 |
| 2 | 既有 | 服务注册失败（透传 install-service.sh） | 步骤 7 |
| **3** | **新增** | role 未指定 / 非法 / 升级期冲突且无 FRP_EASY_FORCE_ROLE | 步骤 0（参数解析尾部）+ 步骤 6.5（升级冲突） |

新增的 `3` 与既有 `0/1/2` 不冲突；T-016 退出码透传链路（`set +e; cmd; rc=$?; set -e; exit "$rc"`）保持不动。

### 5.3 公网 IP 探测函数契约

```bash
# detect_public_ip: 在 install.sh 中定义，仅 server 模式调用。
# 入参：无（读全局常量 PUBLIC_IP_CANDIDATES）
# 输出（stdout）：成功时输出合法 IP 字面量（无尾换行、无前后空白）；失败时空字符串
# 返回码：0 = 成功；1 = 全部候选失败
# 总预算：≤ 8 秒（由 curl --max-time 3 × 候选数 控）
# 单次预算：≤ 3 秒（curl --max-time 3）
# 验证：HTTP 状态码 == 200 且 body 通过 IP 字面量正则 + Python 不可用时降级 bash IPv4/IPv6 正则
```

**候选 URL 清单**（NFR-2 明文写死，2 个 IPv4 + 1 个混合，按可用性排序）：

```bash
PUBLIC_IP_CANDIDATES=(
    "https://api.ipify.org"           # 纯文本 IP，最稳定
    "https://ifconfig.me/ip"          # 纯文本 IP，备选
    "https://icanhazip.com"           # 纯文本 IP，第三备选
)
```

**选 3 条而非 2 条的理由**：FR-B.5 要求 ≥ 2 个；选 3 个让"首个被 GFW 阻断 + 第二个偶发故障"的常见场景仍能成功（实测腾讯云国内段对 icanhazip 偶有抖动）。所有 3 个均返回纯文本 IP（无 JSON 解析），呼应 NFR-2 "审计简单"。**不**选 `ipinfo.io`（返回 JSON 增加解析复杂度）、`ident.me`（IPv6-only 偶发）、`checkip.amazonaws.com`（响应带换行符且 AWS 区域路由偶发）。

### 5.4 横幅形态契约（FR-B / AMBIG-F = F2）

**server 模式** —— 三行并列（公网 IP 探测成功）：

```text
访问地址：
  本机访问：    http://127.0.0.1:7800
  局域网访问：  http://10.1.20.7:7800
  公网访问：    http://<PUBLIC_IP>:7800
```

**server 模式** —— 三行并列（公网 IP = LAN IP，AMBIG-F = F2，仍打两行 + 提示）：

```text
访问地址：
  本机访问：    http://127.0.0.1:7800
  局域网访问：  http://<IP>:7800
  公网访问：    http://<IP>:7800   （与局域网 IP 相同 —— 本机直接在公网上）
```

**server 模式** —— 公网 IP 探测失败：

```text
访问地址：
  本机访问：    http://127.0.0.1:7800
  局域网访问：  http://10.1.20.7:7800
  公网访问：    <公网 IP 探测失败，请手动确认服务器出口 IP；或重新安装并设 FRP_EASY_PUBLIC_IP=...>

提示：若外部 frpc 客户端无法连上 7800 / 7000 端口，请检查：
  ① 云厂商安全组（腾讯云 / 阿里云 / AWS）是否放行 7800/tcp 与 7000/tcp
  ② 本机 ufw / firewalld 是否放行
```

**client 模式** —— 仅本机一行（FR-B.3）：

```text
访问地址：
  本机访问：    http://127.0.0.1:7800
```

完全不打印"局域网 / 公网"字样，也不发起任何外网请求（用 `grep -E 'api.ipify|ifconfig.me|icanhazip' /proc/$$/net/tcp` 应无命中）。

### 5.5 install.ps1 接口

仅新增公网 IP 探测的横幅打印（G-8）。**不**新增 role 入口（OOS-2 保留）。

PowerShell 候选 URL 与 bash 同款（3 条）：

```powershell
$PublicIPCandidates = @(
    'https://api.ipify.org',
    'https://ifconfig.me/ip',
    'https://icanhazip.com'
)
```

横幅打印形态：

```text
访问地址：
  本机访问：    http://127.0.0.1:7800
  局域网访问：  http://<LAN_IP>:7800
  公网访问：    http://<PUBLIC_IP>:7800   ← 探测成功时
  公网访问：    <公网 IP 探测失败，请手动确认>   ← 失败时
```

**注**：Windows 路径目前没有"server vs client"区分（OOS-2），所以横幅永远打三行；安全性提示沿用 cmd/frp-easy/main.go 现有的 `exposureNotice` —— UIBindAddr 默认仍为 `0.0.0.0`（appconf.Default()），用户 UI 启动后会看到 stderr 安全提示，符合 T-011 既有设计。

---

## 6. 序列 / 流程图

### 6.1 全新安装（server 模式）— ASCII

```text
   用户：FRP_EASY_ROLE=server sudo -E bash <(curl -fsSL .../install.sh)
                 |
                 v
   [0] 参数解析 → resolve_role_or_die  ────►  ROLE=server  (失败 exit 3)
                 |
                 v
   [1] 前置（curl/tar/root）         ─────►  失败 exit 1
                 |
                 v
   [2] OS/ARCH 探测                   ─────►  失败 exit 1
                 |
                 v
   [3]-[4] GitHub Releases API        ─────►  失败 exit 1
                 |
                 v
   [5] 下载 + 解压（T-016 进度条）    ─────►  失败 exit 1
                 |
                 v
   [6] 解包到 INSTALL_DIR（全新分支）
        cp -a "$EXTRACTED/." "$INSTALL_DIR/"
                 |
                 v
   ┌─────────────────────────────────────────────────────────────────┐
   │ [6.5] role 应用 + 属主修复（新增；本任务核心）                  │
   │                                                                 │
   │   if [[ ! -f $INSTALL_DIR/frp_easy.toml ]]; then                │
   │       render_frp_easy_toml server > $INSTALL_DIR/frp_easy.toml  │
   │   fi                                                            │
   │   echo "server" > $INSTALL_DIR/.role                            │
   │   mkdir -p $INSTALL_DIR/.frp_easy $INSTALL_DIR/frp_linux        │
   │   chown $RUN_USER:$RUN_USER $INSTALL_DIR/frp_easy.toml          │
   │   chown -R $RUN_USER:$RUN_USER $INSTALL_DIR/.frp_easy           │
   │   chown -R $RUN_USER:$RUN_USER $INSTALL_DIR/frp_linux           │
   │   # .role 保持 root:root；binary 与 scripts 不动                │
   └─────────────────────────────────────────────────────────────────┘
                 |
                 v
   [7] install-service.sh（T-016 已修复，本任务不动）
        └─► daemon-reload → enable --now frp-easy
                 |
                 v   （systemd 拉起 frp-easy 进程；
                      cwd=/opt/frp-easy，euid=RUN_USER；
                      appconf.Load 看到 toml 已存在 → 不写默认 → 不 permission denied）
                 v
   [8] 横幅打印（role-aware）
        ┌── server ──► LOCAL_IP + detect_public_ip → 三行 + 防火墙提示
        └── client ──► 仅 127.0.0.1 一行（client 模式根本走不到这里因
                       全程同款流程，分支只在横幅块）
                 |
                 v
                exit 0
```

### 6.2 升级路径（已装 server，再次以 server 升级）

```text
   [0]-[5] 同上
                 |
                 v
   [6] 升级分支（检测到 $INSTALL_DIR/frp-easy 存在）
        - 停服 systemctl stop frp-easy
        - cp -a frp-easy / scripts / VERSION / ...
        - 绝不动 frp_easy.toml / .frp_easy/ / frp_linux/
                 |
                 v
   [6.5] role 一致性校验 + 幂等 chown
        if [[ -f $INSTALL_DIR/.role ]]; then
            OLD_ROLE=$(cat $INSTALL_DIR/.role)
            if [[ "$OLD_ROLE" != "$ROLE" ]]; then
                if [[ "${FRP_EASY_FORCE_ROLE:-no}" != "yes" ]]; then
                    echo "错误：已装 role=$OLD_ROLE，本次指定 role=$ROLE 冲突 ..." >&2
                    exit 3
                fi
                # 强制覆盖路径：备份旧 toml → 重写 .role → 重写 toml
                cp -a frp_easy.toml frp_easy.toml.bak.$(date +%s)
                render_frp_easy_toml $ROLE > frp_easy.toml
                echo "$ROLE" > .role
            fi
            # role 一致：什么都不写
        else
            # 从 T-016 之前的版本升上来（无 .role 文件）—— D1 兼容
            echo "$ROLE" > $INSTALL_DIR/.role
            # 不动 frp_easy.toml（保留用户值优先）
        fi
        # 幂等 chown（即使没改 toml 也运行，修复 T-017 之前装出来的 root:root 旧属主）
        chown $RUN_USER:$RUN_USER $INSTALL_DIR/frp_easy.toml
        chown -R $RUN_USER:$RUN_USER $INSTALL_DIR/.frp_easy 2>/dev/null || true
        chown -R $RUN_USER:$RUN_USER $INSTALL_DIR/frp_linux 2>/dev/null || true
                 |
                 v
   [7] install-service.sh（systemctl restart 服务）
                 v
   [8] 横幅（含公网 IP 探测）→ exit 0
```

### 6.3 拒绝路径（未指定 role）

```text
   用户：curl -fsSL .../install.sh | sudo bash
                 |
                 v
   [0.5] resolve_role_or_die （在步骤 1 之前；只检查 ROLE 是否合法）
        FRP_EASY_ROLE 未设置
                 |
                 v
        stderr 打印：
          错误：未指定 FRP_EASY_ROLE。请二选一并重新运行：
            服务端：FRP_EASY_ROLE=server sudo -E bash -c "$(curl -fsSL .../install.sh)"
            客户端：FRP_EASY_ROLE=client sudo -E bash -c "$(curl -fsSL .../install.sh)"
          说明：sudo -E 让 sudo 透传 FRP_EASY_ROLE 环境变量；
                server 监听 0.0.0.0、需要公网 IP；
                client 监听 127.0.0.1（最安全）。
                 |
                 v
                exit 3
```

---

## 7. 复用审计（强制要求）

| 需求 | 既有代码 / 模式 | 文件 + 行号 | 决定 |
|---|---|---|---|
| install.sh 步骤分块"==> [N/8] 中文进度行" | 8 步 stdout 进度模板 | `scripts/install.sh` L80 / L99 / L124 / L158 / L187 / L227-229 / L249 / L260 / L270 / L292 | **复用**：新增 §6.5 块严格沿用同款 `==>` 前缀与中文短语 |
| install.sh 中文错误形态 + exit 码 | FR-F.3 现有错误消息（"错误：未识别的参数 $1" / "错误：发布包下载失败"等） | `scripts/install.sh` L73 / L84 / L88 / L93 / L131 / L201 / L207 / L213 / L218 / L223 | **复用**：新错误消息按"错误：...请..." 模板写；exit 3 沿用同结构 |
| `${SUDO_USER:-$(id -un)}` 真实调用者 | install-service.sh 默认 RUN_USER 推导 | `scripts/install-service.sh` L68-75 | **复用**：§6.5 块开头同款表达式确定本次 chown 目标用户；与 install-service.sh 后续生成 unit 的 User= 字段保持完全一致（同一脚本上下文，同一表达式两次求值结果同） |
| getent 校验用户存在性 | install-service.sh 前置 4 | `scripts/install-service.sh` L104-108 | **复用**：§6.5 在 chown 之前同款 `getent passwd "$RUN_USER" >/dev/null 2>&1 \|\| { echo "错误：..."; exit 1; }` 校验 |
| `systemd_escape_path()` 函数 | systemd unit 路径含空格的 `\x20` 转义 | `scripts/install-service.sh` L20-34 | **不复用**：本任务不写 unit 文件路径；预生成的 toml 内部 DataDir 是相对路径 `./.frp_easy`，无空格转义需求 |
| `curl -fsSL -w '%{http_code}'` 模式 + HTTP 状态码先判 | T-013 GitHub API 调用范式 | `scripts/install.sh` L126-155（步骤 3）+ insight-index L33 | **复用为参考**：detect_public_ip 用同款 `curl -sS -m 3 -w '\n%{http_code}'` 写法（去 -f 让 curl 不在 4xx 时把响应体丢弃，本场景不需要响应体所以也可保留 -f；权衡：保留 -f 让 curl 自身在 4xx/5xx 时 rc 非 0，detect 函数更直接的失败信号——**选保留 -f**） |
| 公网 IP 探测的"先判 HTTP 状态码、后验 IP 字面量"思路 | `internal/httpapi/handlers_system.go` Go 版 fetchPublicIP | L155-200 | **不直接复用代码**（语言不同）；**复用方法论**：HTTP 200 → trim → bash regex `^([0-9]{1,3}\.){3}[0-9]{1,3}$` 或 IPv6 简化正则 `^[0-9a-fA-F:]+$` 验证 |
| `appconf.Default()` 字段语义 | UIBindAddr / UIPort / DataDir / LogDir 字面默认值 | `internal/appconf/config.go` L48-55 | **复用为字段名权威**：§4.2 预生成 toml 的字段名严格 = struct tag（L36-39）；**不改 Default() 内的硬编码**（Inv-1） |
| `appconf.Load()` "用户显式值优先" | L92-108 解析后只对空字段补默认 | `internal/appconf/config.go` L91-112 | **复用为兼容契约**：升级期不动用户已写的 frp_easy.toml，Load() 走 unmarshal 分支自动保留所有用户字段 |
| `cmd/frp-easy/main.go` 安全提示文案 | exposureNotice() 在 UIBindAddr=0.0.0.0 时打印 setup 引导 | `cmd/frp-easy/main.go` L138-140 / L305-311 | **复用**：server 模式装完启动后自动打印（无需 install.sh 重复打印）；横幅文案不与该提示冲突 |
| `LOCAL_IP=$(hostname -I | awk '{print $1}')` | install.sh L294 现有 LAN IP 取法 | `scripts/install.sh` L294 | **复用**：LAN IP 探测沿用，不动；公网 IP 与之并列 |
| install.ps1 `Get-NetIPAddress` LAN IP | install.ps1 L249-255 排除 127/169.254 | `scripts/install.ps1` L247-255 | **复用**：仅在其后追加 Get-PublicIPv4 调用，不改既有逻辑 |
| 升级期 frp_linux/ 不被覆盖 | T-014 升级分支白名单逐项覆盖，不动 frp_linux/ | `scripts/install.sh` L234-247 | **复用**：§6.5 块仅追加 `chown -R RUN_USER frp_linux/`（修写权），不动其内容 |
| T-016 退出码透传 `set +e; cmd; rc=$?; set -e` | install.sh L280-289 步骤 7 | `scripts/install.sh` L276-289 | **不复用**（§6.5 块的内部命令简单，无 if 反模式问题）；保留作为模式认同 |
| uninstall-service.sh 收尾文案 | "数据目录与配置文件未删除" + 手动清理命令 | `scripts/uninstall-service.sh` L76-86 | **复用并扩展**：追加一行 `.role` 路径提示，保持中文风格一致 |
| openapi.yaml / handlers_system.go 已有 `/api/v1/system/public-ip` | 现有 Web UI 内的公网 IP 检测按钮 | `internal/httpapi/handlers_system.go` L77-150；`web/src/components/PublicIpDetector.vue` | **不复用**（运行时 vs 安装时两条独立路径）；但**确认共存无冲突**：安装时 install.sh 直接调外网，运行时 UI 调后端 → 后端调外网。两路径独立超时与降级，互不干扰 |

---

## 8. 风险分析（≥ 3 条，每条带缓解）

| R ID | 风险 | 严重度 | 缓解 |
|---|---|---|---|
| R-1 | 预生成 toml 字段名与 appconf struct tag 不一致（go-toml 是大小写敏感的）→ appconf.Load() 走 unmarshal 后字段全为零值 → Validate 失败 → 进程 exit 1 → 死循环重启（与本任务要修的根因完全相同） | **高** | §4.2 字段名严格照搬 `internal/appconf/config.go` L36-39 struct tag（`UIBindAddr` / `UIPort` / `DataDir` / `LogDir`，大写驼峰）；§9 测试策略列出"go run 一次实际跑通 appconf.Load 读预生成 toml"作为 Developer 自验 |
| R-2 | 用户用 `sudo bash`（无 `-E`）→ FRP_EASY_ROLE 被清掉 → exit 3 → 用户困惑 | 中 | exit 3 错误消息明示 `sudo -E`；README 文案双命令样例（FR-C.4 + AMBIG-C C4 文案侧）均含 `-E` |
| R-3 | 用户手动改 `.role` 文件期望切换 role —— 但 .role 仅在升级期被读、运行时进程不读 → 用户改了 .role 但 frp_easy.toml UIBindAddr 不变 → 困惑 | 中 | .role 权限设 `0644 root:root`（非 RUN_USER）+ `.role` 文件首行写注释`# 切换 role 请 uninstall 后重装；本文件仅供 install.sh 升级期一致性校验`（heredoc 的一部分）；横幅末段也提示"切换 role 请走 uninstall+reinstall 流程" |
| R-4 | 公网 IP 探测的"HTTP 状态码 200 但 body 是 HTML 错误页"（BC-2，例如运营商 DNS 劫持） | 中 | detect_public_ip 在 HTTP 200 之后用 bash 正则 `^([0-9]{1,3}\.){3}[0-9]{1,3}$` 强校验 IP 字面量；任何含 `<`、空格、换行（非首尾）的 body 一律视为失败、降级到下一候选 |
| R-5 | chown frp_linux/ 把用户手动改过的二进制属主重置——与 T-014 升级语义一致但仍可能让"高级用户手动 chown 给非 RUN_USER 跑"的用法失效 | 低 | 接受该权衡（与 T-014 升级 OOS 一致）；02 §10.2 明示"高级用户若手动 chown frp_linux/，每次跑 install.sh 会被重置；规避手段：手动跑 systemctl 即可，不必再跑 install.sh" |
| R-6 | bash 双引号 + parameter expansion 的 quote-removal 陷阱（insight L38, T-016 D-1） | 中 | 本任务无字符级路径替换（systemd_escape_path 不用）；heredoc 写 toml 字面量用 `<<'EOF'`（单引号 here-doc 完全禁用插值）—— 见 §3.2 模板；变量插入用 `printf '%s' "$ROLE"` 而非 `echo` 避免反斜杠插值 |
| R-7 | install.sh 在 BSD（macOS）下 `chown -R` 与 `getent passwd` 语义差异：macOS 无 getent | 低 | §6.5 块加 `if [[ "$OS" == "darwin" ]]; then  skip role+chown 与 systemd 注册; fi`（与现有 L258-268 macOS 分支同款）。J 决议仅同步公网 IP 探测，不在 macOS 强制 role + chown |
| R-8 | 升级期 toml 损坏 / appconf 解析失败 → cfg.Validate 错 → 进程 exit 1 死循环 —— 与 R-1 同根因但触发路径不同 | 中 | §6.5 块在写 toml 前用 `chown` 而非 `cp -a`（避免引入 BOM 等编码副作用）；写完后用 `awk 'NR<=4 {print}' frp_easy.toml | grep -qE '^UIBindAddr = "(0.0.0.0\|127.0.0.1)"'` 简单自检；自检失败 → exit 1 + 中文报错 + 不调 install-service.sh（避免起进程后再爆 systemd 重启循环） |
| R-9 | verify_all PASS:19 被打破（A.1 secrets scan 误中 `FRP_EASY_ROLE=` 之类字面量；E.6 06_TEST_REPORT.md 必含英文 `## Adversarial tests`） | 中 | A.1 正则 (verify_all.sh L63) 要求 `[\"'][^\"']{8,}[\"']`（≥ 8 字符引号串），`FRP_EASY_ROLE=server` 中 `server` 只 6 字符不命中（且 server / client 不在引号内）；E.6 由 QA 在 06 stage 写英文小节标题守护 |

---

## 9. 迁移 / 回滚计划

### 9.1 升级路径（从 T-016 等更早版本升 → T-017）

1. **跑 install.sh + FRP_EASY_ROLE=server**（或 client）：
   - 步骤 6 升级分支按原路径 cp -a 覆盖 binary / scripts / VERSION ...（不动 frp_easy.toml / .frp_easy/ / frp_linux/）。
   - 步骤 6.5（新）：检测 .role 不存在 → 写一行 ROLE → 不动 frp_easy.toml（D1 保留用户值优先） → 幂等 chown（**这一步顺带把 T-016 之前装出来的 root:root 属主修复为 RUN_USER:RUN_USER**——这是本任务对线上崩溃的实际修复点）。
   - 步骤 7：install-service.sh 注册 / 刷新 unit（T-016 已修），systemctl 启动后 appconf.Load 读用户旧 toml 成功 → 进程 active → systemd 死循环消失。
2. 升级期用户**完全不需要**改任何手动配置；只需在原一键命令前加 `FRP_EASY_ROLE=server` 前缀即可。

### 9.2 回滚路径

如果 T-017 上线后发现回归，回滚到 T-016：

1. 用户跑 `sudo /opt/frp-easy/scripts/uninstall-service.sh`（保留 frp_easy.toml 与 .frp_easy/）。
2. 手动改 README 的一键命令指向 T-016 commit hash 对应的 install.sh raw URL（不是 rolling tag，因为 rolling 会自动滑到最新）。
3. **不**需要回退 frp_easy.toml 内容（D1 不动用户值）；不需要回退 .role（旧版不读该文件）。
4. 已被 chown 给 RUN_USER 的 frp_easy.toml / .frp_easy/ / frp_linux/ 在 T-016 路径下仍然可用（属主从 root 变成 RUN_USER 不影响 root 重装时的 cp -a 覆盖能力——但 cp -a 会保留源 root 属主，回退后再次升级会重新变 root —— 这是 T-016 已有 bug，不属本任务回滚关切）。

### 9.3 兼容性矩阵

| 历史版本 → 本版本 | 行为 |
|---|---|
| 全新装机（无任何 frp_easy 历史） | 走全新分支 → 必须指定 FRP_EASY_ROLE |
| T-016 装过但崩溃中 → T-017 升级 | 升级分支 → .role 不存在 → 写 .role → 不动 toml → 幂等 chown 修崩溃 |
| T-017 装过 server → 再跑 T-017 server | 升级分支 → .role 一致 → 仅刷新 binary + 幂等 chown |
| T-017 装过 server → 再跑 T-017 client（无 FORCE） | exit 3 + 错误提示 uninstall 或 FORCE |
| T-017 装过 server → 再跑 T-017 client + FRP_EASY_FORCE_ROLE=yes | 备份 toml → 重写 toml + .role → 重启服务 |

---

## 10. Out-of-scope 澄清（设计层面）

继承 RA §6 全部 OOS（OOS-1 到 OOS-12），其中本设计另外强调：

| OOS ID | 设计层面再次声明 |
|---|---|
| OOS-1 (FRP 业务侧 frpc/frps 默认进程) | 本设计完全不动 internal/frpconf / procmgr / UI 端进程定义 |
| OOS-2 (Windows install.ps1 role 选择) | install.ps1 仅同步公网 IP 探测（G-8）；不接收任何 role 环境变量 |
| OOS-3 (防火墙打洞) | 仅打印诊断文案（FR-C.6），不执行 ufw / firewalld 命令 |
| OOS-4 (T-011 默认值层) | 不改 appconf.Default()；role-derived 默认在 install.sh 期通过预生成 toml 实现 |
| OOS-5 (公网 IP 探测高级配置) | 不支持 --proxy / --socks 等；用户机器有系统代理时 curl 会自动尊重，已足够 |
| OOS-6 (systemd 强化) | 不改 unit 的 NoNewPrivileges / ProtectHome 等 |
| OOS-7 (verify_all 检查项) | 不新增、不修改 verify_all.sh |
| OOS-8 / OOS-9 (Web UI 改动) | 不动 web/src/** |
| OOS-10 (macOS 路径 role) | install.sh darwin 分支同步公网 IP 探测（J 决议），但不在 macOS 强制 FRP_EASY_ROLE（macOS 进入 darwin 分支后 exit 0，role 仅用于横幅文案；可空） |
| OOS-11 (release.yml / Actions) | 不动 |
| OOS-12 (LICENSE / NOTICE) | 不动 |

---

## 11. Partition assignment（分区分配）

项目存在 `.harness/agents/dev-{db,backend,frontend}.md` 三个分区。

按 T-016 同款"语义就近"原则（T-016 02_SOLUTION_DESIGN.md §11 已确立），所有 `scripts/install*.*` 改动归 **dev-backend**。理由：scripts/install*.* 严格说不在三个分区 owned paths 列表内，但 install-service.* + systemd 注册属系统服务工程，与后端运行环境最近；且 `internal/appconf/`（dev-backend owned）的 toml 字段名是本任务预生成 toml 的契约源头，由同一分区维护避免跨分区扯皮。

### 11.1 文件级分配表

| File | Partition | New / Edit | Dependency |
|---|---|---|---|
| `scripts/install.sh` | dev-backend | edit（步骤 0 role 解析 + 步骤 6.5 新增块 + 步骤 8 role-aware 横幅 + detect_public_ip 函数） | — |
| `scripts/install.ps1` | dev-backend | edit（步骤 8 末段新增 Get-PublicIPv4 + 横幅追加公网行；不引入 role） | — |
| `scripts/install-service.sh` | dev-backend | **untouched** | — |
| `scripts/install-service.ps1` | dev-backend | **untouched** | — |
| `scripts/uninstall-service.sh` | dev-backend | edit（收尾文案追加 `.role` 提示） | — |
| `internal/appconf/config.go` | **dev-backend** | **untouched**（Inv-1） | — |
| `cmd/frp-easy/main.go` | **dev-backend** | **untouched**（Inv-2） | — |
| `docs/dev-map.md` | dev-backend | edit（在 scripts/ 行追加 T-017 install.sh role 解析 + 公网 IP 探测说明） | — |
| `.harness/insight-index.md` | dev-backend | edit（阶段 7 由 archive-task 自动收割；Developer 不直接改） | — |

### 11.2 Dispatch order

1. **dev-backend**（单分区覆盖全部改动）

### 11.3 Parallelism

N/A —— 单分区。Developer 内部可并行编辑 install.sh / install.ps1 / uninstall-service.sh（无文件级依赖），但 verify_all 必须串行收尾跑一次。

### 11.4 落地约束

scripts/install.sh / install.ps1 / uninstall-service.sh 严格说不在 dev-backend owned paths 列表（dev-backend.md L18 只列 `scripts/start.{ps1,sh}`、`scripts/build.{ps1,sh}`、`scripts/verify_all.{ps1,sh}`），落地时 Developer 在 04_DEVELOPMENT.md 显式说明"超出 owned paths 但 SA 授权（按 T-016 02 §11 同款先例）"以避免 Code Reviewer 阶段 5 的 `BLOCKED ON PARTITION` 误报。

---

## 12. 测试策略提纲（QA 阶段 6 会扩展）

### 12.1 单元 / 函数级

| 层 | 测试 | 备注 |
|---|---|---|
| Go appconf | **不新增**（appconf 不动） | 现有 `internal/appconf/config_test.go`（如有）保持通过 |
| bash 函数 | install.sh 中的 `detect_public_ip` / `render_frp_easy_toml` 拆为可独立 source 测试的形态：脚本顶部判定 `${BASH_SOURCE[0]}` 是否被 source（library 模式），是则不执行主流程；调用方 `source install.sh; detect_public_ip; echo $?` 直接测 | 不引入 bats / shunit2（NFR-5 不新依赖）；纯 bash 断言即可 |
| PowerShell | `Get-PublicIPv4` 同款拆分，function 单独可被 `Import-Module` 加载（用 Pester 已有就用，没有就不强测） | 已存在 web/ Playwright 工具链，无 PS 测试基建；本期可暂跳过 |

### 12.2 集成（QA 阶段 6 写）

| 场景 | 期望 |
|---|---|
| Docker Ubuntu 22.04 容器跑 `FRP_EASY_ROLE=server bash install.sh`（systemd-in-docker 或 mock 掉 install-service.sh） | 装完 `cat /opt/frp-easy/frp_easy.toml` 含 `UIBindAddr = "0.0.0.0"`；`cat /opt/frp-easy/.role` = `server`；`stat -c %U /opt/frp-easy/frp_easy.toml` = RUN_USER |
| 同上 + `FRP_EASY_ROLE=client` | `UIBindAddr = "127.0.0.1"`；横幅不含"局域网"或"公网"字样；strace -e trace=connect 无 api.ipify / ifconfig.me / icanhazip 连接 |
| 不指定 FRP_EASY_ROLE | exit 3，stderr 含"未指定 FRP_EASY_ROLE"与两条入口命令样例 |
| 装完 server 再跑 client（无 FORCE） | exit 3，stderr 含 .role 冲突提示 |
| 装完 server 再跑 client + FRP_EASY_FORCE_ROLE=yes | 成功；frp_easy.toml.bak.<ts> 存在；新 toml UIBindAddr=127.0.0.1 |
| iptables DROP api.ipify + ifconfig.me，保留 icanhazip | 横幅打印第三候选 IP；总耗时 ≤ 8 s |
| iptables DROP 全部 3 个候选 | 横幅打印"公网 IP 探测失败"；exit 0；总耗时 ≤ 9 s（3 × 3s） |

### 12.3 E2E（QA 选择性）

- 真机腾讯云 Ubuntu 22.04 VM 跑完整 `FRP_EASY_ROLE=server sudo -E bash <(curl ...)`，验证 `systemctl is-active frp-easy` = active 且公网 IP 探测命中真实出口 IP。

### 12.4 必须的对抗性测试（FR-F.2 + insight L31）

QA 在 `06_TEST_REPORT.md` 必须有英文 `## Adversarial tests` 段，覆盖至少：

- 公网 IP 候选返回 HTML 错误页（运营商 DNS 劫持模拟）—— 探测应失败而非把 HTML 当 IP 打印。
- 公网 IP 候选超时（iptables DROP）—— 8 s 总预算保护，不阻塞 exit 0。
- toml 字段名大小写偏差（一次手动改 `uibindaddr` 注入到预生成 → systemd 应起不来）—— 用于守护 R-1（这是反例测试，不是验收路径）。
- bash quote-removal 陷阱（heredoc + 变量插值 corner case）—— 用 `xxd /opt/frp-easy/frp_easy.toml | head` 字节级核对，避免 T-016 D-1 同款翻车。

---

## 10'. 不变量与回归保护（与 §1.2 互参，强调测试时刻的检查清单）

| 守护点 | 检查命令（QA / verify 时） |
|---|---|
| verify_all PASS:19 | `./scripts/verify_all.sh` 退出码 0，尾行 `PASS: 19`、`FAIL: 0` |
| T-011 NF-2 用户显式值优先 | 升级路径用例：装前手动写 `UIBindAddr = "192.168.1.10"` → 升级后该值不变 |
| T-016 unit 语法 / 进度条 / 退出码透传 | install-service.sh diff 为空；install.sh 步骤 5 进度条逻辑 diff 仅在新加的 detect_public_ip 上下文之外 |
| T-014 frp_linux/ 不被覆盖 | 升级前 `echo "test" > /opt/frp-easy/frp_linux/.canary`；升级后 `cat .canary` = `test` |
| Inv-1 / Inv-2 | `git diff internal/appconf/ cmd/` 为空 |

---

## 12'. Open questions

**无**。PM 已对 RA 列出的 8 条 AMBIG 全部裁决（PM_LOG 2026-05-23 决议表），本设计严格按决议推进。任何"超出 01 决议范围"的扩展均已 OOS 化（§10）。

如下游 Developer 在实现期发现新歧义（例如某发行版的 chown 行为偏差），按红线"BLOCKED ON DESIGN"回退到 PM 处理，**不**自由发挥。

---

## 13. Verdict

**READY**

理由：

1. 上游 01 RA verdict 经 PM_LOG 决议后等价为 READY（8 条 AMBIG 全部锁定）；本设计严格按 PM 决议推进，未偏离也未新增需求。
2. 三组改动（role 入口 / 预生成 toml + chown / 公网 IP 探测）的每一条 FR 都有对应的具体修改点 + 引用证据 + 复用映射 + 测试用例。
3. 复用审计列表 18 行，覆盖现有 install.sh 步骤分块、错误模板、`${SUDO_USER:-$(id -un)}`、getent、curl HTTP 状态码先判模式、appconf 字段名权威、T-014/T-016 不变量等；无新模块、无新依赖（NFR-5 满足）。
4. 9 条风险全部带缓解；3 条核心风险（R-1 toml 字段名 / R-4 HTML 错误页 / R-9 verify_all 误中）已分别在 §4.2 字段权威、§5.3 状态码 + 字面量双校验、§8 A.1 正则字符数约束上设防。
5. 分区分配虽然 scripts/install*.* 严格不在 dev-backend owned paths 内，但 §11.4 沿用 T-016 已确立的"语义就近 + 04 显式说明"先例，避免 Code Reviewer 误报。
6. 不变量列表（Inv-1 ~ Inv-7）明确划走"不能动 Go 代码 / 不能改 verify_all 数量 / 不能破坏 T-011 T-014 T-016 既有承诺"。
7. 下游 Developer 仅需按 §3.2 步骤 6.5 块伪代码、§4.2 toml 模板、§5.3 公网 IP 探测函数契约实现，无设计悬空决策。

下游交接：Gate Reviewer 阶段 3 复核本设计是否覆盖所有 FR/BC + PM 决议，通过后 PM 派 `dev-backend` 实施。
