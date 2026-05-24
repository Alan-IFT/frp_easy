# 01 — 需求分析（T-038 boot-autostart-hardening）

> 由 Requirement Analyst（PM 上下文角色化）产出。模式：`full`。

## 0. 根因实证（先把"用户觉得的"与"实际的"对齐）

PM 在测试机 `alan@192.168.100.90`（Ubuntu 26.04 LTS / systemd 259）实测当前线上 build（commit `4612264`）真实启动序列，铁证如下：

| 观察项 | 实际值 | 含义 |
|---|---|---|
| `/etc/systemd/system/frp-easy.service` 存在 | 是 | system-level unit ✓ |
| `systemctl is-enabled frp-easy` | enabled | 开机自启已就位 ✓ |
| `systemctl is-active frp-easy` | active (running) | reboot 后服务**确实**起来了 ✓ |
| 进程 owner | alan (uid 1001) | `User=alan` 让 ps 看到 alan，但 unit 是 system-level —— 用户主诉"装在用户级"是**误判** |
| journalctl 显示首条 INFO | `00:10:19 Started frp-easy.service` | 比 systemd-multi-user.target 早 ~1s |
| journalctl 显示 frpc 拉起 | `00:10:20.077 INFO locator resolved` → `00:10:20.290 WARN auto-restore failed kind=frpc err="procmgr.Start(frpc): process exited within 3s"` | autoRestore 跑了，但 frpc 起不来 |
| frpc.log 真因 | `connect to server error: dial tcp 43.136.30.208:7001: connect: network is unreachable. With loginFailExit enabled, no additional retries will be attempted` | **boot 时网络栈还没在线** + frp 默认 `loginFailExit=true` → 立即放弃 |
| `frp-easy.service` `After=` | `After=network.target` | **错误依赖** —— `network.target` 只表示"网络配置已下发"，不等于"网络在线"。canonical 修法 = `After=network-online.target` + `Wants=network-online.target` |
| `autoRestoreProcs()` 重试策略 | 一次性，无 backoff，failed → logger.Warn 后就此放弃 | **设计缺陷** |
| `procmgr.waitUntilStable(3s)` | 启动 3 秒内任何 exit 都判 Error | frpc 因 loginFailExit 在 ~50ms 内退出 → 直接死路 |
| `systemd-networkd-wait-online.service` | disabled | 系统侧 |
| `NetworkManager-wait-online.service` | **enabled** | 加 `Wants=network-online.target` 后会被拉起，能正确 gating |

**结论 —— 用户的"reboot 后远程无法连接"是 4 个独立缺陷叠加：**

1. **(systemd 层) unit 用 `After=network.target` 而非 `network-online.target`** —— frp-easy 在网络真正可用前 ~1-3s 就被拉起。
2. **(frp 层) 生成的 frpc.toml 没显式设 `loginFailExit = false`** —— frp 默认行为是首次登录失败直接 exit，没机会等网络起来。
3. **(frp-easy 层) `autoRestoreProcs()` 一次性，无 retry**（cmd/frp-easy/main.go L433-L467）—— 即使前两层修了，任何长尾失败（DNS 慢、frps server 重启中）也会让客户端永久死掉。
4. **(UX 层) 当前完全无 UI 反馈**告诉用户"reboot 后自动恢复失败了，原因是 X"—— 用户只能 `ps`/`journalctl` 自行排障。

用户主诉里的"用户级安装"是误判（unit 实际是 system-level，只是 process owner 是 SUDO_USER），但这个误判本身是 **UX 失败**——我们的 UI 没有告诉用户"服务化是 system-level，开机即起，不依赖登录"。

## 1. Goal

让"开机即可用 frp 远程"成为**物理不可破坏的硬保证**：从 systemd 单元依赖、frp 上游 retry 配置、frp-easy 进程恢复 backoff、UI 可观测面板四个层面同时加固，让任何单一层的瞬时失败都不会让用户 lose 远程访问；并让用户在 UI 上 0 click 看到"是否处于服务化、是否会自动恢复、上次自动恢复结果"。

## 2. In-scope behaviors（编号、可测）

### 2.1 systemd unit 网络就绪门控（实测主因 #1）

- **B-1.1**: `scripts/install-service.sh` 生成的 unit 文件必须包含 `Wants=network-online.target` 与 `After=network-online.target`（替代当前的 `After=network.target`）。
- **B-1.2**: server / client 角色生成的 unit 在 network gating 上**无差异**——server 需要 DNS / 公网 IP 探测，client 需要连到 frps，两者都不能在 network-offline 时启动。
- **B-1.3**: 升级路径（已存在旧 unit）必须刷新 unit 文件至新内容并 `daemon-reload` + `restart`，幂等。
- **B-1.4**: macOS 不动（OOS-3 沿用）。
- **B-1.5**: Windows 侧：`sc.exe config frp-easy depend= Tcpip/Dnscache` 显式声明依赖 TCP/IP 与 DNS 客户端服务，避免 LocalSystem 启动早于网络栈（虽然 LocalSystem 通常已经在 ServicesPipeTimeout 后才拉，但显式声明是软件工程标准）。

### 2.2 frpc/frps 上游 retry 配置（实测主因 #2）

- **B-2.1**: `install.sh`/`install.ps1` 生成的初始 `frp_easy.toml` 不动（这是 UI 的元配置，不是 frpc.toml）。
- **B-2.2**: `internal/frpconf/render.go` 渲染的 `frpc.toml` **必须**显式设 `loginFailExit = false`（默认 frp 是 true，让首次登录失败 = exit）。
- **B-2.3**: `frpc.toml` 必须显式设合理的重连参数（frp 上游有自己的 dial retry / heartbeat 机制；本任务只确保不打破，并加 `loginFailExit = false`）。具体字段：保持默认 `dialServerTimeout`、`dialServerKeepalive`、`heartbeatInterval`、`heartbeatTimeout`——只显式加 `loginFailExit = false`。
- **B-2.4**: 对 `frps.toml` 不需要 retry（server 是被动监听，无 dial 失败概念）。

### 2.3 frp-easy 自动恢复带 backoff retry（实测主因 #3）

- **B-3.1**: `autoRestoreProcs()` 从"一次性 Start → warn → 放弃"改为"指数 backoff retry，最多 5 次"。建议序列：5s → 15s → 45s → 2min → 5min。期间任何一次 `pm.Start(kind)` 成功（waitUntilStable 后 state = running）即算 OK，retry 终止。
- **B-3.2**: retry 全程异步（goroutine），不阻塞 `ready.Store(true)`——HTTP server 在 first attempt 后即可服务请求；UI 反映"恢复中"状态。
- **B-3.3**: 每次 retry 必须写 logger.Info "auto-restore attempt N/5"，方便 journalctl 追溯。
- **B-3.4**: 5 次全失败后，把最终错误连同时间戳写入 kv `system.autorestore.last`，并 logger.Error "auto-restore exhausted"。
- **B-3.5**: 任何时刻用户在 UI 点"启动"按钮（procStart）→ retry goroutine 必须能感知并优雅退出（避免 retry 与 user-initiated start 竞态写状态）。简单实现：retry 在每次循环开头 read `m.Status(kind).State`，若已 running / starting → break。

### 2.4 服务化状态的 UI 可观测（实测主因 #4）

- **B-4.1（API 端点）**: 新增 `GET /api/v1/system/service-status`，返回结构：
  ```json
  {
    "supervised": true,
    "supervisor": "systemd",
    "boot_autostart": true,
    "run_as": "alan",
    "auto_restore": {
      "enabled_kinds": ["frpc", "frps"],
      "last_run": {
        "timestamp": "2026-05-25T00:10:20Z",
        "attempts": [
          {"kind":"frpc","ok":false,"reason":"...","retry_count":5},
          {"kind":"frps","ok":true,"reason":"","retry_count":0}
        ]
      }
    }
  }
  ```
- **B-4.2（探测实现）**: 
  - Linux: `supervised = (os.Getenv("INVOCATION_ID") != "")` —— systemd 通过 environment variable 注入此 ID 给所有 service 进程，是 canonical 探测方法。`supervisor = "systemd"`。`boot_autostart` = 跑 `systemctl is-enabled frp-easy.service` 解析 stdout（5s timeout）。
  - Windows: `supervised = svc.IsWindowsService()`（现有 cmd/frp-easy/service_windows.go 已用）。`supervisor = "windows-service"`。`boot_autostart` = 跑 `sc.exe qc frp-easy` 解析 `START_TYPE` 含 `AUTO_START`。
  - 裸跑：`supervised = false`，`supervisor = "none"`，`boot_autostart = false`。
- **B-4.3（UI 卡片）**: Dashboard（首屏顶部，前端 SA 决定具体放置）展示 "服务化状态" 卡片，含四行：
  - 监管方式：systemd / Windows Service / 未监管（前两种绿色，未监管橙色）
  - 开机自启：是 / 否
  - 运行用户：<username>（解释"system-level service 即使该用户未登录也会启"）
  - 重启后自动恢复：[frpc, frps] / 无；点击展开看 last_run 详情
- **B-4.4（"如何修复"折叠区）**: 当 `supervised=false` 或 `boot_autostart=false` 时，卡片高亮 + 折叠区给出对应平台的精确命令字串：
  - Linux: `sudo /opt/frp-easy/scripts/install-service.sh`
  - Windows: 以管理员 PowerShell 跑 `C:\Program Files\frp-easy\scripts\install-service.ps1`
- **B-4.5（命令字串一致性）**: B-4.4 命令字串、README "或：手动下载发布包" 段、install-service.sh / .ps1 自身的 `--help` 输出三处文案必须含同一锚字串 `[boot-autostart-fix]`，verify_all 守门 grep "三处一致"。

### 2.5 手动下载路径的显式提示（防止用户误以为 ./frp-easy 裸跑等价于安装）

- **B-5.1（README）**: README "或：手动下载发布包（备选）" 段增加显式中文警告："直接 `./frp-easy` 裸跑**不会**注册系统服务，关机后不会自动恢复；如需开机自启，请改用一键安装、或在解压目录跑 `sudo ./scripts/install-service.sh`（Linux）/ `.\scripts\install-service.ps1`（Windows，管理员 PowerShell）"。锚字串 `[boot-autostart-fix]`。
- **B-5.2（运行时横幅）**: `frp-easy` 启动序列在 logger.Info "ready gate opened" 之前，若探测到 `supervised=false` 且 `boot_autostart=false`，在 stderr + ui.log 双轨打印中文一行警告："提示：当前以前台进程运行，关机/重启后不会自动恢复 frpc/frps 子进程。如需开机自启请运行 scripts/install-service.{sh,ps1}。"
- **B-5.3（不打扰已服务化场景）**: 当 supervised=true OR boot_autostart=true 时，B-5.2 不打印——避免在正常服务化场景下污染 ui.log。

### 2.6 安装时的自检与失败上报（防止 install 完后用户不知装没装好）

- **B-6.1（install.sh 自检）**: install.sh 步骤 7 服务注册成功后，新增步骤 7.5："自检" —— 跑 `systemctl is-active frp-easy` + `systemctl is-enabled frp-easy`，两者均 PASS 才进入步骤 8 横幅；任一 FAIL 则 stderr 打印 `[boot-autostart-self-check FAIL]` 段 + journalctl tail，exit 4。
- **B-6.2（install.ps1 自检）**: 同款，跑 `sc.exe query frp-easy` 解析 `STATE: 4 RUNNING` + `sc.exe qc frp-easy` 解析 `START_TYPE: 2 AUTO_START`。

## 3. Out-of-scope（本期不做）

- **OOS-1**: 不重写 install.sh / install.ps1 / install-service.sh / install-service.ps1 的主体；只在末尾追加自检 + 修一行 `After=`。
- **OOS-2**: 不实现"服务化失败时回退到 nohup 前台运行"——硬要求 service-mode，失败就报错引导。
- **OOS-3**: macOS 仍不支持服务化（沿用现状 install.sh `OS == darwin` 分支 exit 0）。本任务的 B-3.1 retry / B-4.x UI 卡片 / B-5.x 警告横幅在 macOS 上仍生效（supervised=false / supervisor=none 路径）。
- **OOS-4**: 不引入 systemd `Restart=always` 替换现有 `Restart=on-failure`——后者更符合 unit 故意 stop 的语义；retry 由 frp-easy 内部 backoff 兜底。
- **OOS-5**: 不引入 watchdog 心跳（systemd `WatchdogSec=`）——增加复杂度，本期收益不显著。
- **OOS-6**: 不创建系统用户 `frp-easy`——继续以 SUDO_USER 跑（避免破坏现有用户数据目录所有权 / D1 红线）。但 UI 卡片"运行用户"行明确说明这是 system-level service，不依赖登录。

## 4. Boundary conditions

- **BC-1**: 升级路径（已存在 /etc/systemd/system/frp-easy.service 用 `After=network.target`）必须刷新为新版 + daemon-reload + restart，幂等。
- **BC-2**: kv `mode.frpc.enabled` 不存在或 false → autoRestoreProcs 跳过 retry goroutine，service-status API 返回 `auto_restore.enabled_kinds=[]`。
- **BC-3**: frpc 二进制缺失（loc.Missing 含 "frpc"）但 mode.frpc.enabled=true → retry 立即 abort（无意义反复 retry 二进制缺失场景），写入 last_run.attempts 标 reason="binary missing"。
- **BC-4**: retry goroutine 在 5 次失败后必须 free 资源，**不**继续在后台 sleep 等下次（避免 process 永远不退出干净）。
- **BC-5**: 用户在 retry 进行中（如第 2 次 sleep 中）从 UI 点"启动" → retry 检测到 state ∈ {starting, running} → break。无竞态。
- **BC-6**: 用户在 retry 进行中点"停止" → state→stopping/stopped，retry 同上 break。
- **BC-7**: frp-easy 进程在 retry sleep 中收到 SIGTERM → context 应能取消 retry goroutine，主循环 30s 内退出（与现有 NFR-7 30s 关停预算一致）。
- **BC-8**: service-status API 在 Windows 上跑 `sc.exe qc` 失败（如 frp-easy 服务实际不存在但被裸跑）→ supervised=false + boot_autostart=false（与 Linux 裸跑路径一致）。
- **BC-9**: service-status API 在 `systemctl is-enabled` 卡住（极少见）→ 5s context timeout 兜底，返回 boot_autostart=false + reason="probe timeout"。
- **BC-10**: install.sh 自检在容器（无 systemd）环境跑 → install.sh 步骤 1 已 exit 1（缺 systemctl），自检 step 不会执行到。

## 5. Acceptance criteria（每条可测）

- **AC-1**: `scripts/verify_all` PASS（含本任务新加 step）。
- **AC-2 单测**:
  - `internal/svcprobe`: probe 在三种 fixture（systemd / scm / bare）下返回不同 `supervised` / `supervisor` / `boot_autostart` 值。
  - `cmd/frp-easy/autoresore_test.go` 或 `main_test.go`: 验证 retry backoff 序列（用 mock procmgr.Manager），第 N 次成功后停止。
  - `internal/frpconf/render`: 生成的 frpc.toml 含 `loginFailExit = false` 字面。
  - install-service.sh 生成的 unit 文件含 `Wants=network-online.target` + `After=network-online.target`（shell test 或 go 单测 read 渲染输出）。
- **AC-3 静态闸门**: 
  - verify_all 加 step 守门 README + UI 卡片"如何修复" + install-service.sh `--help` 三处含 `[boot-autostart-fix]` 锚字串。
  - verify_all 加 step 守门 install-service.sh / install-service.ps1 的渲染源码含 `network-online.target` / `Tcpip/Dnscache` 字面。
- **AC-4 端到端真机验证**:
  - 在测试机 `alan@192.168.100.90` 上：build → scp 上传 → 跑 install-service.sh 升级 unit → reboot 测试机 → 等 systemd 拉起 → 实测 frpc 在 retry 第 1-3 次内拉起成功（journalctl 显示 retry 序列 + 最终 running）。这是 user-observable 的最终 acceptance。
- **AC-5 反向证伪**:
  - **ADV-1**: 临时把 install-service.sh 改回 `After=network.target` → unit 不含 `network-online.target` → verify_all 静态闸门 FAIL；恢复后 PASS。
  - **ADV-2**: 临时把 render.go 的 `loginFailExit = false` 删掉 → verify_all 静态 grep 闸门 FAIL；恢复后 PASS。
  - **ADV-3**: 在测试机用 iptables 模拟"network reachable but frps unreachable" → frp-easy retry 5 次全失败 → service-status API 返回 last_run.attempts[0].ok=false retry_count=5；UI 卡片渲染红色"恢复失败"。
  - **ADV-4**: 测试机 reboot 实测：旧 build（unit `After=network.target`）→ frpc 起不来；新 build → frpc 起来。这是最硬的物理证据。
- **AC-6 UI 卡片可见**: Playwright e2e（或测试机浏览器手动截图）实证 Dashboard 顶部渲染服务化状态卡片，四行字段正确。

## 6. Non-functional requirements

- **NFR-1（兼容）**: 不引入对运行时 Go / glibc / Windows 版本新要求。systemd 230+（已有约束），sc.exe 任意现役 Windows 版本。
- **NFR-2（性能）**: service-status API < 100ms (Linux `systemctl is-enabled` 实测 < 50ms; Windows `sc.exe qc` 实测 < 100ms)。retry goroutine 在 sleep 期间 CPU 占用 = 0。
- **NFR-3（可观测）**: 所有新加失败路径必须 logger.Warn 双轨写 ui.log 与 stderr（NFR-3 沿用 T-022 idiom）。
- **NFR-4（安全）**: service-status API **必须**走认证 middleware（与现有 `/api/v1/system/*` 一致）；裸 supervisor=none 路径不能被未认证用户用来探测内部状态（虽然 supervised=false 本身并不敏感，但与 setup-locked 一致是契约）。
- **NFR-5（可维护）**: probe 实现独立 `internal/svcprobe/` 包，build tag 区分 linux/windows/darwin；不在 main.go 散落 `runtime.GOOS == "linux"` 分支。
- **NFR-6（kv schema 稳定）**: 新 kv key `system.autorestore.last` 用 JSON value，schema 在 02 设计阶段固化；未来加字段必须保持向后兼容（reader 容忍未知字段）。

## 7. Related tasks

- **T-019 windows-service-scm-1053-fix**（`docs/features/_archived/windows-service-scm-1053-fix/`）：现有 Windows Service 状态机；本任务 supervised 探测复用 `svc.IsWindowsService()`。
- **T-022 service-mode-stderr-bridge**：service 模式下 stderr→ui.log 桥接；本任务 NFR-3 沿用。
- **T-008 deploy-kit**：install-service.sh 第一版 + systemd unit。本任务在其末尾追加自检（B-6.1）+ 一行 `After=` / `Wants=` 修订。
- **T-017 install-role-and-public-ip**：role-aware unit；本任务 B-1.2 强调 role 在 network gating 上无差异。
- **T-035 install-sh-role-cli-arg-passthrough**：`bash -s -- --role` 透传。本任务 B-5.1 README 警告段沿用同款 CLI idiom。
- **autoRestoreProcs (cmd/frp-easy/main.go L433-L467)**：本任务对其重构（B-3.x）。
- **procmgr.waitUntilStable (internal/procmgr/manager.go L493-L522)**：本任务不改其行为（保持 3s 判定），由外层 retry 兜底。

## 8. Open questions for user

无。用户明示"你来决策就可以了，我只看结果是否符合需求"。PM 直接决议如下：

- **D-1**: B-3.1 retry backoff 序列 `5s → 15s → 45s → 120s → 300s`（指数 + 上限 5min）。决策依据：覆盖典型 systemd network-online 兜底超时（30-60s）、frps server cold-boot 时间（典型 30-90s）、网络瞬时抖动（< 5s），同时 5 次 = ~8 分钟总时长，与用户主观"reboot 完应该几分钟内就能用"对齐。
- **D-2**: B-4.1 API 路径 `GET /api/v1/system/service-status`，沿用 `/api/v1/system/*` 命名空间（与 `handlers_system.go` 现有 ready / publicip / time 等并列）。
- **D-3**: B-1.5 Windows 加 `sc.exe config frp-easy depend= Tcpip/Dnscache` 是 nice-to-have 不是 must。如果该命令在某些 Windows 版本上有兼容性问题（如 Server Core 缺 Dnscache），install-service.ps1 应当 best-effort（失败 logger.Warn 但不 exit 2）。
- **D-4**: B-5.1 README 警告锚字串选 `[boot-autostart-fix]`（不用 `[autostart]` 防与未来其他 autostart-related 文案冲突）。
- **D-5**: B-3.1 retry 全程在 `cmd/frp-easy/main.go` 而非 `internal/procmgr/manager.go`——procmgr 是单次启动语义，retry 是 application-level policy 不该侵入 procmgr 契约。
- **D-6**: kv `system.autorestore.last` 单 key 覆盖写（不做历史保留）—— 重启计数若需要更多历史可由 journalctl 提供，kv 只反映"最近一次"给 UI 实时展示。
- **D-7**: B-4.3 UI 卡片放在 Dashboard 首屏顶部 hero 位（用户看到 UI 即 0 click 看到）。具体前端实现位置由 SA 在 02 决定。
- **D-8**: B-2.2 `loginFailExit = false` 写到 render.go 生成路径——即每次用户改 server 配置后重写 frpc.toml 都自动含此字段。**不**做迁移脚本改用户已有的 frpc.toml（用户若手动写过自己的 frpc.toml 我们不动），仅未来 UI 改配置触发渲染时确保新字段。

## 9. Verdict

**READY** — 无 open question 阻塞用户，可进入 Solution Architect。
