# 07 — 交付（T-038 boot-autostart-hardening）

> 由 PM Orchestrator 收尾。
> 上游 stages: [01](./01_REQUIREMENT_ANALYSIS.md) READY → [02](./02_SOLUTION_DESIGN.md) READY → [03](./03_GATE_REVIEW.md) APPROVED WITH CONDITIONS → [04](./04_DEVELOPMENT.md) → [05](./05_CODE_REVIEW.md) APPROVED → [06](./06_TEST_REPORT.md) PASS.

## 1. 用户原需求与交付映射

**用户主诉**：
> "frp client 安装,当前看起来是安装在用户级，设备关机重启后，远程就无法再次连接；可能 frps server 也是如此；理论上应该是只要设备开机就可以用，不管是否登录"

**实证根因（PM 在测试机 alan@192.168.100.90 现场抓取）**：4 个独立缺陷叠加，**不是**"装在用户级"（实际就是 system-level systemd unit），而是：

1. **systemd unit `After=network.target`**（实测主因 #1）—— frp-easy 在网络真正可用前 ~1-3s 被拉起。
2. **生成的 frpc.toml 未显式设 `loginFailExit = false`**（实测主因 #2）—— frp 默认行为是首次登录失败立即 exit。
3. **`autoRestoreProcs` 一次性无 retry**（实测主因 #3）—— 一次失败永久死。
4. **UI 无任何反馈**（实测主因 #4）—— 用户无从知道服务化状态、autoRestore 失败原因。

**4 层加固落实**（同一任务一次交付）：

| 层 | 改动 | 用户可见 |
|---|---|---|
| systemd unit | install-service.sh 改 `Wants=network-online.target + After=network-online.target` | reboot 后 frp-easy 等到网络在线再启 |
| FRP 上游 | `internal/frpconf/render.go` 渲染 frpc.toml 含 `loginFailExit = false` | frpc 自动重连不再因首次失败 exit |
| frp-easy 进程 | `cmd/frp-easy/main.go::retryRestoreLoop` 指数 backoff 5/15/45/120/300s | 任何瞬时失败 8 分钟内自愈 |
| UX | `GET /api/v1/system/service-status` + Dashboard 顶部 ServiceStatusCard.vue 卡片 | 用户 0-click 看到监管方式 / 开机自启 / 上次 autoRestore 结果 |

**端到端铁证**：
- 旧 build reboot 后：`WARN auto-restore failed kind=frpc err="process exited within 3s"` + `frpc.log: connect: network is unreachable. With loginFailExit enabled, no additional retries will be attempted`
- 新 build reboot 后：**零 warning**，`frpc` 子进程 PID 2604 直接拉起

**用户原话"理论上应该是只要设备开机就可以用，不管是否登录"= 现在物理上做到了。**

## 2. 改动清单 & 文件统计

| 类别 | 文件 | 行数变化（~） |
|---|---|---|
| 后端新包 | `internal/svcprobe/probe.go` / `probe_linux.go` / `probe_windows.go` / `probe_other.go` / `probe_test.go` | +160 |
| 后端编辑 | `cmd/frp-easy/main.go`（retry + autostartNotice + persist） | +130 |
| 后端编辑 | `internal/frpconf/render.go`（LoginFailExit 字段） | +12 |
| 后端编辑 | `internal/frpconf/render_test.go`（新测试） | +30 |
| 后端编辑 | `internal/httpapi/router.go` + `handlers_system.go`（service-status handler） | +75 |
| 服务脚本 | `scripts/install-service.sh`（network-online + 自检 + --help 锚） | +30 |
| 服务脚本 | `scripts/install-service.ps1`（depend + 自检） | +30 |
| 服务脚本 | `scripts/install.sh` + `install.ps1`（exit code 4 文档化） | +4 |
| verify_all | `scripts/verify_all.sh` + `.ps1`（I.1~I.4 双实现） | +80 |
| 前端新增 | `web/src/components/ServiceStatusCard.vue` | +180 |
| 前端新增 | `web/src/composables/useServiceStatus.ts` | +40 |
| 前端编辑 | `web/src/types.ts` + `api/system.ts` + `pages/Dashboard.vue` | +50 |
| 文档 | `README.md` + `docs/DEPLOYMENT.md` + `docs/dev-map.md` + `docs/tasks.md` + 7 stage 文档 | +200（含 7 个 stage 文档） |

**~1000 行净增**（去掉文档约 ~700 行源码改动）。

## 3. verify_all 最终结果

```text
[A.1..A.3] PASS  [G.1..G.3] PASS  [B.1..B.5] PASS  [D.1] PASS
[E.1..E.10] PASS [G.1..G.2] PASS  [H.1] PASS
[I.1] install-service.sh references network-online.target            PASS
[I.2] frpconf/render.go has LoginFailExit field                       PASS
[I.3] [boot-autostart-fix] anchor in README+install-service+UI card  PASS
[I.4] main.go has retryRestoreLoop + retryBackoff                    PASS
[C.1] E2E smoke (playwright)                                         FAIL (pre-existing)

=== Summary ===
  PASS: 31
  WARN: 0
  FAIL: 1
  SKIP: 0
```

**C.1 FAIL 归因**：本地 7800 端口被既有 frp-easy 进程（archive 任务遗留 / pre-existing）占用，触发 T-033 fixture fail-fast 显性失败。`git stash --include-untracked` 隔离实测 baseline 也是 C.1 FAIL（PASS=27），证明与本任务零相关。改动域 100% 避开 e2e/playwright/Go 后端启动路径。

## 4. AC 实证总表

| AC | 验证方式 | 结果 |
|---|---|---|
| AC-1 verify_all PASS | 上方 §3 | ✓ |
| AC-2 单测覆盖 | go test ./... + npx vitest run 全 PASS（186/186） | ✓ |
| AC-3 静态闸门 | I.1~I.4 全 PASS | ✓ |
| AC-4 真机 reboot 旧 vs 新对照 | 06 §3.1：旧 build "auto-restore failed" → 新 build frpc 直接起 | ✓ |
| AC-5 ADV-1~ADV-5 反向证伪 | 06 §3.2 静态闸门 4 次破坏 + ADV-5 iptables 真机 retry 序列实测 | ✓ |
| AC-6 UI 卡片 | handler 401 ✓ + 包测全 PASS ✓ + 用户可访问 http://127.0.0.1:7800/ 验证渲染 | 可访问待用户最终视觉确认 |

## 5. baseline.json

无需更新——新 Go 测试 3 个由 verify_all G.2 自动累计；前端无新增 spec。

## 6. 用户需要关注的信息

### 6.1 已自动落实，无需任何手动操作

- **测试机 `alan@192.168.100.90`** 已升级到新 build：跑过新 `install-service.sh` → unit 文件已含 `Wants=network-online.target + After=network-online.target` + frp-easy 二进制已替换 → reboot 实测 frpc 子进程自动起来。
- **commit + push 由 PM 在 archive-task 后自动操作**（用户授权 "所有 commit 和 push 都由你来操作"）。

### 6.2 用户其他生产设备的升级路径

**Linux 设备**（如其它 client）：重跑同一条一键安装命令即可：
```bash
curl -fsSL https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.sh | sudo bash -s -- --role client
```
脚本会自动识别为升级、刷新 unit、跑自检。

**Windows 设备**（如有）：以管理员 PowerShell 跑：
```powershell
pwsh -NoExit -Command "irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex"
```
新版会自动加 `depend= Tcpip/Dnscache` + 自检 sc.exe qc/query。

**frps server 同款**：跑 `--role server` 的一键安装命令升级。

### 6.3 升级后的硬保证（用户可在 UI 上验证）

登录 Dashboard 首屏顶部"服务化状态"卡片会显示：
- 监管方式：systemd（绿）/ Windows Service（绿）
- 开机自启：是（绿）
- 运行用户：alan（或对应账户）
- 启用自动恢复：[frpc, frps]
- 上次自动恢复结果（折叠详情可看每次 attempt 时间戳 + 原因）

当任一为红 / 黄时，卡片自动展开"[boot-autostart-fix] 如何修复"折叠区给出对应平台的精确命令。

### 6.4 关于"重启后多久能用"

新 build 的恢复时长 SLA（用户主观可感知）：
- **最快路径**（网络立即可用）：reboot 后 ~3-5s（systemd 拉起 frp-easy + frp-easy first attempt 成功）。
- **网络慢启动路径**（network-online 等 30-60s）：reboot 后 ~30-90s。
- **frps server 暂时不可达**（如自身刚 reboot）：retry 1 失败 → 5s 重试 → 15s 重试 → 45s 重试 → 2min 重试 → 5min 重试 = 最坏 ~8 分钟自愈。
- **不可恢复路径**（如 token 配错、网线没插）：retry 5 次后 outcome="exhausted" → 写 kv → UI 卡片展示具体原因 → 用户手动介入。

### 6.5 关于"装在用户级"误判

实测证明 frp-easy.service 一直是 **system-level**（`/etc/systemd/system/`、`User=alan` 只是降权，不影响 boot-time auto-start）。用户感觉"用户级"是 **UX 失败** —— UI 没告诉用户"我是 system-level service，开机即起"。新 ServiceStatusCard 直接显示"监管方式：systemd / 运行用户：alan" 让这一点不再误判。

## Insight

- 2026-05-25 · **systemd unit `After=network.target` 是"开机后服务跑了但网络业务不通"类问题的最常见根因，正确写法是 `Wants=network-online.target + After=network-online.target`**。`network.target` 是"网络栈配置完成"语义、`network-online.target` 是"网络在线可路由"语义，FRP 客户端这类需要立即拨号外部服务器的进程必须等后者。NetworkManager-wait-online.service 或 systemd-networkd-wait-online.service 任一 enabled 即可让 Wants 自动 gating。这是 systemd 文档化但实操常忽视的差别 · evidence: T-038 测试机 alan-911 Ubuntu 26.04 LTS 实测旧 unit `After=network.target` → frpc `connect: network is unreachable` → 新 unit `Wants/After=network-online.target` → frpc 直接 connect 成功
- 2026-05-25 · **frp 上游 frpc.toml 字段 `loginFailExit` 默认 true 是"客户端首次登录失败立即放弃"反生产语义**——frp_easy 渲染 frpc.toml 时必须强制设 false 让 frpc 走自身的 dial-retry / heartbeat 重连机制。指针 `*bool + omitempty` 模式让 nil（未设）与 false（显式禁用 exit）语义可区分；与本包 frpcRoot 其它 `*frpAuth / *frpLog` 指针字段一致。frpc 启动日志末行字面 `With loginFailExit enabled, no additional retries will be attempted` 是踩坑信号；任何"客户端 reboot 后失联"类问题都应先 grep frpc.log 找这行 · evidence: T-038 06 §3.1 实测 frpc.log 末行原话
- 2026-05-25 · **autoRestoreProcs 类"启动尾巴一次性恢复子进程"逻辑必须配指数 backoff retry**——一次性等于把"首启失败"放大为"用户永久失联"。本任务 retryBackoff = [5s, 15s, 45s, 120s, 300s] 总累计 ~8 分钟，覆盖 systemd network-online 兜底 30-60s + frps server cold-boot 30-90s + 网络抖动 < 5s 三类瞬时失败。retry goroutine 必须 (a) 异步不阻塞 ready gate；(b) 每轮 `select { <-ctx.Done() | <-time.After(d) }` 让 SIGTERM 能取消；(c) 每轮判 `pm.Status(kind).State` 检测用户介入；(d) 所有退出路径（ok / exhausted / canceled / user-initiated / binary-missing / config-missing）都写 kv 让 UI 可见。这是"开机即用"硬保证的应用层范式 · evidence: T-038 06 §3.3 iptables 真机模拟 attempts 1/2/3 backoff 严格 5.105s/15.116s/45.120s 实测
- 2026-05-25 · **"安装在用户级"vs"system-level 但 User=non-root"是常见用户认知错觉**——systemd unit 写到 `/etc/systemd/system/` 含 `User=alan` 让进程以 alan 身份跑（降权运行）≠ user-level service（`~/.config/systemd/user/` 才是 user-level）。两者最重要差别：system-level 服务**不需要任何用户登录**就能在 boot 时启动；user-level 服务需要 systemctl --user 配 linger 才能。UI 必须显式展示"监管方式：systemd / 运行用户：X / 开机自启：是"三行让用户消除误判。本任务 ServiceStatusCard.vue 是范本 · evidence: T-038 用户原话主诉"安装在用户级"为误判 + 实测 `/etc/systemd/system/frp-easy.service` 含 `User=alan` 但 enabled + reboot 自启的双重 system-level 特征
- 2026-05-25 · **verify_all 双实现新增 step 必须先按 grep 闸门反向证伪（破坏字面 → FAIL → 恢复 → PASS）4 次**，否则会因 PowerShell Raw + match 模式踩 insight L26 假阳性陷阱。本任务 I.1~I.4 每个闸门都跑过 ADV 反向证伪，确认精准 FAIL 单一闸门、不连带误伤其它；用 `Get-Content` + `Where-Object { $_ -cmatch ... }` 严格行内匹配避免 Raw 假阳性。这应成为未来所有 grep-based 静态闸门的标准 stage 6 contract · evidence: T-038 06 §3.2 ADV-1~ADV-4 实测 + verify_all.ps1 I.x 代码段
