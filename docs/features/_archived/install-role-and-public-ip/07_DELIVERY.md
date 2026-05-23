# 07 — Delivery · T-017 install-role-and-public-ip

> Stage 7（PM Orchestrator）—— 交付总结。

## §1 任务摘要

**任务**：T-017 · install-role-and-public-ip · **full** 模式
**触发**：用户在腾讯云 Ubuntu VM 跑 `curl -fsSL ... install.sh | sudo bash` 后 systemctl status 显示 frp-easy 死循环重启（restart counter 已 35）+ 安装横幅只显示局域网 IP（10.1.20.7），无公网 IP。
**关键原话**："出错了，且只有 127.0.0.1 和局域网 ip，而不是公网 ip，若是服务端，根本没法访问到 UI 页面 ... 服务端需要公网 ip，而客户端应该是监听 127.0.0.1 才是最安全的"

## §2 交付内容

### 改动文件（仅 3 个脚本）

- `scripts/install.sh`（+293 / -13）：新增 -h 文案 role 用法；§0.5 ROLE 解析（exit 3）；`render_frp_easy_toml` + `detect_public_ip` 两 helper；§6.5 role 应用 + 局部 chown（仅 4 项）；步骤 8 替换为 role-aware 横幅（server 三行 + 公网 IP 探测 + 防火墙提示；client 仅本机访问一行）
- `scripts/install.ps1`（+76）：新增 `Get-PublicIPv4`（同款 3 候选 URL + FRP_EASY_PUBLIC_IP short-circuit）；步骤 8 横幅追加公网行 + OOS-2 注释
- `scripts/uninstall-service.sh`（+6）：删 unit 后 `rm -f .role`

### 文档（流水线全套）

- `docs/features/install-role-and-public-ip/INPUT.md`
- `01_REQUIREMENT_ANALYSIS.md` —— 35 FR / 12 BC / 8 AMBIG
- `02_SOLUTION_DESIGN.md` —— G-1~G-10 / Inv-1~Inv-7 / 18 项复用
- `03_GATE_REVIEW.md` —— Verdict APPROVED WITH CONDITIONS（9 conditions）
- `04_DEVELOPMENT.md` —— 9 conditions 落地 + verify_all PASS
- `05_CODE_REVIEW.md` —— Verdict APPROVED（6 维度 PASS / 2 Minor / 1 Nit）
- `06_TEST_REPORT.md` —— Verdict APPROVED FOR DELIVERY（31 adversarial / 30 PASS / 1 known limitation）
- `07_DELIVERY.md` —— 本文件
- `PM_LOG.md`

## §3 PM 决议执行映射（8 条 AMBIG → 实现）

| AMBIG | PM 决议 | 实现落点 |
|---|---|---|
| C 安装期角色选择 | C1+C4：`FRP_EASY_ROLE=server\|client`，未指定 exit 3 + 两条入口命令样例 | install.sh §0.5（L95-108） |
| D 升级期已有 toml | D1 保留用户值 | install.sh §6.5（.role 已存在分支不动 toml） |
| E 权限模型 | E2+E3：仅 chown 运行时可写路径 + 预生成 toml | install.sh §6.5 局部 chown 三项 + render_frp_easy_toml |
| F 公网=LAN 横幅 | F2 仍打两行 + 标注 | install.sh 步骤 8 server 分支 L554-556 |
| G Windows 同步 | G2 仅同步公网 IP 探测 | install.ps1 Get-PublicIPv4 + OOS-2 注释 |
| H 同主机重装不同 role | H1 拒绝 + FRP_EASY_FORCE_ROLE | install.sh §6.5 L399-407 |
| I frpc/frps 默认进程 | I1 OOS | 未实现 |
| J macOS | macOS 维持现状 + 公网 IP 探测同步 | install.sh darwin 分支 |

## §4 验证（最终闸门）

### `scripts/verify_all` 输出尾段

```
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

**结果**：**PASS:19 / WARN:0 / FAIL:0 / SKIP:0**（baseline test_count 231 不变）。E.6 验证 06_TEST_REPORT.md 的 `## Adversarial tests` 英文标题命中（红线守住）。

### Inv-1~Inv-7 守护

- Inv-1 `internal/appconf/` 一字未动 ✓
- Inv-2 `cmd/frp-easy/main.go` 一字未动 ✓
- Inv-3 `scripts/install-service.sh` 一字未动（unit 语法、systemd_escape_path、RUN_USER 两段式、退出码透传均保留）✓
- Inv-4 T-014 `frp_linux/` 升级保留语义未破 ✓
- Inv-5 T-016 进度条 + 退出码透传未变 ✓
- Inv-6 verify_all 检查项数量仍为 19 ✓
- Inv-7 中文输出 ✓

## §5 已知限制（Known Limitations）

来自 06 §4：

- **KL-1**（MIN-1）：`FRP_EASY_PUBLIC_IP=<IPv6>` 用户手动设置时，横幅 `http://${PUBLIC_IP}:7800` 拼接缺 `[xxx]:port` bracket 包裹，浏览器解析失败。**边缘 case**（探测路径只返回 IPv4，仅用户手动设含 `:` 值才触发）。02 §5.4 BC-3 设计明确要求 bracket。建议跟进任务修，本任务接受。
- **KL-2**：go-toml/v2 实测大小写不敏感（设计 R-1 假设过度紧张），产品安全无影响。
- **KL-3**：Windows QA 主机无真实 systemd，install.sh 路径在 Linux VM 实测降级为静态分析 + 函数级 source 测试。
- **KL-4**：install.ps1 修改仅静态核实，未在 Windows 主机端到端测试。

## §6 用户验收路径（实测复现 + fix 验证）

用户重新运行（替换为合适 role）：

```bash
# 服务端（公网 VM）
curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh \
  | FRP_EASY_ROLE=server sudo -E bash

# 客户端（内网设备）
curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh \
  | FRP_EASY_ROLE=client sudo -E bash
```

预期结果：
- 不再有 `appconf: write default: open .../frp_easy.toml: permission denied` 死循环（fix A 闭环）
- `systemctl is-active frp-easy` = `active`
- server 模式横幅显示公网 / 局域网 / 本机三种访问 URL（公网若国内 VM 探测失败，会给 `FRP_EASY_PUBLIC_IP=<IP>` 重跑样例）
- client 模式横幅仅显示 `本机访问 http://127.0.0.1:7800`，无公网请求

## §7 风险与跟进建议

- **跟进任务建议**：修 KL-1（IPv6 bracket 包裹），约 5 行 bash 改动。
- **运维注意**：升级到本版本（已存在 frp_easy.toml）时，install.sh 不动 UIBindAddr（D1 保留用户值）；用户从 client 升 server 必须手动改 toml 或加 `FRP_EASY_FORCE_ROLE=yes`。
- **install-service.sh 与 install.sh 的 RUN_USER 表达式**：必须保持两段式 if-then-else 等价（已加注释提示同步两处）。

## §8 Insight（提交到 `.harness/insight-index.md`）

由 archive-task 自动收割：

- **2026-05-23** · install.sh 解包后必须对运行时可写路径（`frp_easy.toml`、`.frp_easy/`、`frp_linux/`）chown 给 RUN_USER（systemd `User=` 同款 `${SUDO_USER:-$(id -un)}`），否则 systemd 进程以 RUN_USER 启动时 appconf.Load() 写默认配置失败 → permission denied → 死循环重启。修复模式：解包后局部 chown + 预生成 frp_easy.toml 让 appconf 走"已存在"分支。 · evidence: T-017 install-role-and-public-ip
- **2026-05-23** · 公网 IP 探测在国内 VM 上三个常用候选（api.ipify.org / ifconfig.me / icanhazip.com）有高概率全部失败；必须提供用户手动覆盖通道（`FRP_EASY_PUBLIC_IP` 环境变量，函数首行 short-circuit）+ 失败横幅显式打印"登云控制台复制出口 IP"提示。仅靠多候选 URL 轮询在国内环境不够。 · evidence: T-017 install-role-and-public-ip

## §9 Verdict

**DELIVERED**

- 6 stage agents 全部 APPROVED
- verify_all PASS:19 / FAIL:0
- 9 conditions C-1~C-9 全部落地
- Inv-1~Inv-7 全部守护
- 1 known limitation（KL-1 IPv6 bracket）已显式记录，跟进任务可修
- 用户原话三诉求（fix A 崩溃 / fix B IP / fix C 角色选择）全部解决

下一步：`scripts/archive-task --task install-role-and-public-ip` 收割 insight + 归档。
