# 01 — 需求分析 · T-019 windows-service-scm-1053-fix

> Harness 流水线 Stage 1（Requirement Analyst）。模式：**full**。
> 上游只读输入：`INPUT.md`（用户终端原始输出）、`docs/tasks.md`、相关历史任务的 `01/02_*.md`、`.harness/insight-index.md`。
> 本文档**不做技术选型**、**不画 API/模块形状**——那是 Stage 2 Architect 的工作。
> 用户已指示"不要停下来问澄清问题"——故 §8 Open Questions 已全部由 RA 选 RECOMMENDED 候选并标注 `PM-resolved`，verdict 直接 `READY`。

---

## 1. 目标（Goal）

修复 Windows 一键安装路径（`irm ... install.ps1 | iex`）首次或升级安装后，`sc.exe start frp-easy` 返回 **错误 1053（"服务没有及时响应启动或控制请求"）** 导致服务无法进入 RUNNING 状态、UI（`http://127.0.0.1:7800`）不可访问的故障，让 Win11 / Windows Server 管理员终端一条命令完成"装完 → 服务 RUNNING → 浏览器可访问"的完整闭环。

---

## 2. 范围内行为（In-scope · 可测试 / 无歧义）

> 每条对应 §6 的一条或多条 AC。需求只描述 "可观测的最终状态"，不规定实现路径。

1. **IS-1** 在 Win11 22H2+ / Windows Server 2019+ / Windows Server 2022 管理员 PowerShell 5.1 与 PowerShell 7.x 上，对一台**未装过 frp_easy** 的干净主机执行
   ```
   irm https://raw.githubusercontent.com/Alan-IFT/frp_easy/main/scripts/install.ps1 | iex
   ```
   后，install.ps1 步骤 7 必须不再出现 `[SC] StartService 失败 1053` 错误行，且脚本退出码 = 0、最终走到步骤 8 打印"安装完成"横幅。
2. **IS-2** 上述场景结束后立刻执行 `sc query frp-easy` 必须返回 `STATE : 4 RUNNING`（在 install.ps1 收尾后 ≤ 3 秒内）。
3. **IS-3** 上述场景结束后立刻执行 `Invoke-WebRequest http://127.0.0.1:7800 -UseBasicParsing` 必须返回 HTTP 200 / 301 / 302 / 401 其中之一（即"服务在 7800 端口监听并响应 HTTP"），而非连接拒绝 / 超时（在 install.ps1 收尾后 ≤ 10 秒内首次握手成功）。
4. **IS-4** 在**已安装 T-018 或更早版本**（处于 `STOPPED` 或 `RUNNING` 或 `START_PENDING` 或 `marked for delete` 状态）的主机上重新执行同一条 `irm | iex` 命令（升级路径），install.ps1 必须以**单次运行**走完步骤 1-8，无 1053、最终 `sc query frp-easy` = `STATE : 4 RUNNING`。
5. **IS-5** install-service.ps1 单独被调用（用户从已展开的发布包目录运行 `.\scripts\install-service.ps1`）时与 install.ps1 链路同等成功：服务最终进入 RUNNING、`Get-Service frp-easy` 显示 `Status: Running`。
6. **IS-6** install-service.ps1 重复执行（同一主机连续两次 `.\scripts\install-service.ps1` 之间不卸载）以退出码 0 收尾，第二次结束时服务仍 `RUNNING`；不留下"marked for delete" 卡死状态、不留下 1053 失败状态。
7. **IS-7** 服务进入 RUNNING 后，主动手动 stop（`sc stop frp-easy`）必须在 ≤ 30 秒内让 `sc query` 返回 `STATE : 1 STOPPED`（不可超过 SCM 默认停止超时 30 秒、不可触发 1053-stop 等价错误 1061）；再 `sc start frp-easy` 必须重新进入 RUNNING。
8. **IS-8** 服务进入 RUNNING 后，Windows 重启（reboot）；系统重启后 `sc query frp-easy` 自动显示 `STATE : 4 RUNNING`（开机自启幂等，无 1053）。
9. **IS-9** install.ps1 步骤 8 打印的"本机访问"链接（`http://127.0.0.1:7800`）所指 URL 在用户复制粘贴到浏览器时**可访问**（与 IS-3 等价但从用户体验角度断言）。
10. **IS-10** 当服务以非预期方式崩溃（frp-easy.exe 主进程退出码 != 0）时，SCM 的 failure-action 仍按 T-008 既定策略在 5 秒后自动重启服务（即不能因本任务的修复方案让 `sc.exe failure ... reset= 60 actions= restart/5000` 失效）。
11. **IS-11** install.ps1 在步骤 7 输出的中文文案必须明确传达 "服务已启动" 这一最终结果（而非现状的"sc.exe start 失败 1053"）；任何在 SCM 握手过程中需要的等待（如轮询 RUNNING）必须以"中文进度提示行"形式打印，避免静默挂起 > 5 秒无任何输出。
12. **IS-12** 卸载链路保持不变：以管理员运行 `<InstallDir>\scripts\uninstall-service.ps1` 后 `sc query frp-easy` 显示 `服务不存在`（`[SC] EnumQueryServicesStatus:OpenService 失败 1060`）、`frp_easy.toml` 与 `.frp_easy\` 数据目录原位保留（与 T-008 FR-2.4 / AC-9 同款契约）。

---

## 3. 范围外行为（Out-of-scope · 显式不做）

| OOS ID | 不做的事 | 理由 |
|---|---|---|
| **OOS-1** | 卸载流程改造 | `uninstall-service.ps1` 已存在并被 T-008 接受为契约；本任务仅"装得起来"，不重写卸载语义。 |
| **OOS-2** | Linux 路径 / systemd unit 任何行为变更 | 用户原始失败现象与 `INPUT.md` 全部在 Windows；Linux 已在 T-016 / T-017 修复并稳定。本任务对 `install.sh` / `install-service.sh` / `uninstall-service.sh` 零字节改动。 |
| **OOS-3** | macOS 路径 | macOS 在 T-008 OOS 中明确不提供 launchd plist；与 1053 无关。 |
| **OOS-4** | frp 子二进制（frpc.exe / frps.exe）的服务化 | frp 子进程由 frp-easy 父进程通过 `internal/procmgr` 管理，**不**注册为独立 Windows 服务；本任务仅修 frp-easy 自己的服务化。 |
| **OOS-5** | 引入第三方服务化壳（NSSM / WinSW / sc.exe 之外的工具） | 与 T-008 NFR-2 "零运行时依赖" 冲突；不引入新外部 .exe 或 PowerShell module。 |
| **OOS-6** | GUI 安装器（MSI / NSIS / WiX） | 与 T-008 OOS 同款理由：与"压缩包 + 脚本"路线冲突。 |
| **OOS-7** | 服务运行用户从 LocalSystem 切换到普通用户 / Network Service / 专用服务账户 | T-008 PM-resolved Q-4：Linux 默认 `${SUDO_USER:-$(id -un)}`；Windows 由 `sc.exe create` 默认 LocalSystem。切换运行用户会引入 ACL / DPAPI / 用户配置漂移问题，超出"1053 修复"边界。 |
| **OOS-8** | UI 监听地址 role 区分（server vs client） | T-017 在 Linux 上已决议 Windows 不做 role 区分（T-017 OOS-2 / G3）；本任务保持 Windows `UIBindAddr = 0.0.0.0`（与 T-011 默认一致），由 main.go 启动时打中性安全提示。 |
| **OOS-9** | Windows Event Log 自定义事件源 / structured logging | 服务化的"对 SCM 报告启动握手"是 in-scope；进一步把业务日志写入 Windows 事件查看器属增强，留作后续。 |
| **OOS-10** | Windows ARM64 / Windows 11 ARM 支持 | install.ps1 步骤 2 当前仅接受 `AMD64`（与 T-008 OOS 同款）；不在本任务扩展。 |
| **OOS-11** | 自动化首启时 Defender / SmartScreen 信任建立 | 用户首次运行未签名 .exe 触发的 SmartScreen 拦截在管理员 PowerShell 中由系统决策；本任务**仅**在 install.ps1 步骤 1 末尾追加一行中文提示（"如服务首启被 Defender 拦截，请到 Windows 安全中心 → 病毒和威胁防护 → 排除项中添加 `<InstallDir>`"），不做自动化加白名单（需 PowerShell `Add-MpPreference -ExclusionPath` 但与"零依赖 + 不静默改用户安全策略"原则冲突）。具体提示文案与放置位置由 Architect 决定。 |
| **OOS-12** | 修改 verify_all 检查项数量 | 与 T-016 / T-017 / T-018 一致；本任务允许在不新增检查项数量的前提下，调整 Windows 路径的 build / unit-test 用例集（如新增 `internal/winsvc` 包的单测计入既有 B 组）。 |
| **OOS-13** | 修改 release.yml / GitHub Actions 发布产物结构 | 发布包目录布局与文件清单（含 `frp-easy.exe` / `scripts/install-service.ps1` / `scripts/uninstall-service.ps1`）保持不变；只允许 frp-easy.exe 内部新增 Windows Service ABI 代码（产物字节数变化在 NFR-2 允许范围内）。 |

---

## 4. 边界条件 / 错误路径

| BC ID | 场景 | 期望行为 |
|---|---|---|
| **BC-1** | install.ps1 在**非管理员** PowerShell 中执行 | 现有 install.ps1 L127-131 已检测；保留行为不变（中文错误 + `exit 1`），不在本任务退化为"自动 UAC 提权"。 |
| **BC-2** | install.ps1 / install-service.ps1 在缺少 `sc.exe` 的极端环境（例：精简 Windows 容器）执行 | 现有 install-service.ps1 L52-57 已检测；保留行为不变（中文错误 + `exit 1`）。 |
| **BC-3** | `FRP_EASY_INSTALL_DIR` 设为含**中文 / 非 ASCII** 字符的路径（如 `C:\程序\frp-easy`） | 服务必须仍能进入 RUNNING。已有 T-008 insight L17 / install-service.ps1 L79-81 用 `-Encoding Default`（host codepage）解决 wrapper.cmd cd 路径乱码；本任务的服务 ABI 实现（如在 frp-easy.exe 内 in-process）必须不重新引入此乱码——具体即：进程的 cwd / binPath 处理与 wrapper.cmd 等价正确。 |
| **BC-4** | `FRP_EASY_INSTALL_DIR` 设为含**空格**的路径（如 `C:\Program Files\frp easy v1`） | 服务必须仍能进入 RUNNING。`sc.exe create binPath=` 的引号语义已在 install-service.ps1 L96 / L102 `binPath= "\`"$WrapperPath\`""` 正确处理；本任务的修复方案必须保持此契约。 |
| **BC-5** | `FRP_EASY_INSTALL_DIR` 指向**网络盘 / UNC 路径**（如 `\\server\share\frp-easy`） | 视为未支持配置；install.ps1 / install-service.ps1 不专门兼容；若 sc.exe 报错，原样透传退出码即可。**不**为此场景写新代码。 |
| **BC-6** | `FRP_EASY_INSTALL_DIR` 指向**只读盘**（如 CD / 写保护 USB） | install.ps1 步骤 5 解压阶段将先失败（既有 try/catch 已覆盖）；不在本任务新增"只读盘探测"逻辑。 |
| **BC-7** | 旧版本服务遗留为 1053 卡死状态（`sc query` 显示 `STATE : 2 START_PENDING` 或 `CHECKPOINT > 0` 长期不变） | 升级执行 install-service.ps1 必须能恢复：先尝试 `sc.exe stop`（即使失败也继续），等待 SCM 把进程残骸清理掉，再走 config / start 链路最终进入 RUNNING。Wait-ServiceStopped 函数（install-service.ps1 L30-43）已存在；可保留或加强。 |
| **BC-8** | 旧服务处于 `marked for delete` 状态（`sc delete` 后 SCM 句柄未释放） | install-service.ps1 必须能在合理超时（≤ 15 秒）内自然恢复或给出明确中文诊断（"检测到旧服务处于 marked for delete 状态，请关闭所有 services.msc / Get-Service 窗口后重试"），**不**让用户面对 sc.exe 原始英文 1072 错误码。 |
| **BC-9** | Windows Defender 实时保护或 SmartScreen 在 frp-easy.exe 首次启动时拦截 | 不视为本任务的服务化失败：SmartScreen 阻断时进程根本不启动，与 SCM 握手无关；install.ps1 给中文提示（OOS-11 末尾文案）让用户加排除项后重跑。 |
| **BC-10** | 用户在 install.ps1 步骤 7 进行中按 Ctrl+C 中止 | PowerShell `$ErrorActionPreference=Stop` 会让中止冒泡；服务可能处于半安装状态，下次重跑 install.ps1 仍走 IS-4 升级路径并恢复到 RUNNING。**不**强求"中止后零残留"。 |
| **BC-11** | install.ps1 与 install-service.ps1 在 PowerShell ISE / PowerShell Core 7.4 / Windows PowerShell 5.1 三种宿主下执行 | 三者均必须达成 IS-1 / IS-2 / IS-3；PSE 已被 PowerShell 团队 EOL 但仍存在，不主动支持，但不应主动崩溃（任何检测到 ISE 的行为可打中文提示后继续）。 |
| **BC-12** | 服务启动后 7800 端口被其它进程占用 | frp-easy.exe `main.go` L321-335 已有端口占用检测路径，`os.Exit(2)`；服务化层必须把 binary 退出码 2 转化为对 SCM 报告"启动失败 + Win32ExitCode=2 / ServiceSpecificExitCode=2"，而非让 SCM 因 30 秒无握手报 1053。1053 的"没及时响应"与"已响应但启动失败"必须可由用户从 `sc query` / 事件查看器区分。 |
| **BC-13** | 服务化加载阶段的依赖（appconf.Load / storage.Open）耗时 > 30 秒（极端低速磁盘 / 大量历史数据迁移） | SCM 握手超时默认 30 秒；服务化层必须在加载期间向 SCM 周期性报告 `SERVICE_START_PENDING` + 递增 CheckPoint + WaitHint，避免 1053。具体周期由 Architect 选。 |
| **BC-14** | install.ps1 / install-service.ps1 在 PowerShell 5.1 中执行但**禁用了 ExecutionPolicy**（`Restricted` / `AllSigned`） | install.ps1 入口由用户主动 `irm | iex` 触发，已绕过文件级 ExecutionPolicy；install-service.ps1 是磁盘脚本，受策略影响——若用户 ExecutionPolicy 阻断子脚本执行，沿用现有 PowerShell 错误，**不**在本任务自动 bypass（自动 bypass 会绕过用户安全策略）。 |

---

## 5. 验收标准（AC）

> 每条 AC 必须可由 QA 在标准 Windows 主机上复现验证。所有命令在管理员 PowerShell 执行。

| AC ID | 描述 | 验证方法 |
|---|---|---|
| **AC-1** | 干净 Win11 22H2+ 主机 `irm ... install.ps1 | iex` 后无 1053 报错、退出码 0 | 跑命令、grep stderr `1053` 命中数 = 0、`$LASTEXITCODE` = 0 |
| **AC-2** | 上述场景 ≤ 3 秒内 `sc query frp-easy` 返回 STATE 4 RUNNING | `sc query frp-easy | findstr STATE` 输出含 `4  RUNNING` |
| **AC-3** | 上述场景 ≤ 10 秒内 `Invoke-WebRequest http://127.0.0.1:7800` 返回 HTTP 200/301/302/401 | StatusCode ∈ {200,301,302,401} |
| **AC-4** | 升级路径（已有 T-018 安装）重跑 `irm | iex`：步骤 1-8 全部走完、最终 RUNNING、无 1053 | `Get-Service frp-easy` Status = Running |
| **AC-5** | install-service.ps1 单独跑：服务最终 RUNNING | `Get-Service frp-easy` Status = Running |
| **AC-6** | install-service.ps1 连续跑两次：第二次退出码 0、服务仍 RUNNING、无 1053 / 1072 错误码 | 连跑两次后 `$LASTEXITCODE` = 0 + `Get-Service` Running |
| **AC-7** | `sc stop frp-easy` 后 ≤ 30 秒进入 STOPPED；`sc start frp-easy` 后再进入 RUNNING | `sc query` 两次断言 |
| **AC-8** | 系统 reboot 后服务自动 RUNNING | `Restart-Computer` 后 `sc query frp-easy` STATE 4 |
| **AC-9** | install.ps1 步骤 7 stdout 含明确中文 "服务已启动" 或等价完成行；无 "1053 失败" 字样 | grep stdout |
| **AC-10** | 卸载流程不退化：`.\scripts\uninstall-service.ps1` 后 `sc query frp-easy` 报服务不存在；`frp_easy.toml` 与 `.frp_easy\` 仍存在 | 卸载前后 `ls` 对比 |
| **AC-11** | 服务以非 0 退出码崩溃 → SCM 在 5 秒后自动重启（T-008 IS-10 / failure action 不退化） | 手动 `taskkill /F /PID <frp-easy.exe>` 后 ≤ 10 秒 `sc query` 再次 RUNNING |
| **AC-12** | `FRP_EASY_INSTALL_DIR=C:\程序\frp-easy` 安装：服务 RUNNING、UI 可访问 | 端到端跑 AC-1 / AC-2 / AC-3 |
| **AC-13** | `FRP_EASY_INSTALL_DIR=C:\Program Files\frp easy v1` 安装（含空格）：服务 RUNNING | 同上 |
| **AC-14** | 旧版本服务卡在 1053 / marked-for-delete 状态时重跑 install.ps1：≤ 60 秒内恢复到 RUNNING 或给明确中文诊断 | 人为构造卡死状态后跑 install.ps1 |
| **AC-15** | Linux 一键安装路径（`curl -fsSL install.sh | sudo bash`）在 Ubuntu 22.04 / 24.04 / Debian 12 / RHEL 9 全部 verify_all PASS:19 且 `systemctl is-active frp-easy` = active（回归不退化） | 与 T-017 同款 4 发行版断言 |
| **AC-16** | `scripts/verify_all` 输出 `PASS:19`（与 T-018 main HEAD 一致） | 执行脚本 |
| **AC-17** | `06_TEST_REPORT.md` 含**精确英文裸标题** `## Adversarial tests`（insight L29 + L40 红线，不带数字编号、不带中文翻译） | grep 文件 |
| **AC-18** | Win11 PowerShell 7.x 与 Windows PowerShell 5.1 双 host 各跑一遍 AC-1 / AC-2 / AC-3 | 两次执行 |
| **AC-19** | install.ps1 步骤 7 中 SCM 握手等待期间至少打印一行中文进度（避免 > 5 秒静默）| 卡读 stdout |

---

## 6. 非功能性需求（NFR · 仅列对本任务有约束的）

| NFR ID | 需求 |
|---|---|
| **NFR-1** | 修复方案不引入新外部依赖（不依赖 NSSM / WinSW / srvany / Sysinternals psservice 等第三方 .exe，不依赖 PowerShell module）；允许在 frp-easy.exe 内引入 Go 标准库扩展（`golang.org/x/sys/windows/svc`），它已在项目 `go.sum` 中作为 modernc.org/sqlite 的间接依赖存在，提升为 direct 不增加交付物体积超出 NFR-2。 |
| **NFR-2** | 修复后 `frp-easy.exe` 体积相对 T-018 release 增长 ≤ 1 MB（Windows Service ABI 代码量极小）。release zip 整包仍 ≤ T-008 NFR-8 的 25 MB 警告阈值。 |
| **NFR-3** | install.ps1 / install-service.ps1 中文输出（00-core 规则）；任何新增 SCM 状态轮询 / 等待提示行必须中文；frp-easy.exe 在服务模式下写入 Windows 事件查看器的消息（若实现）必须 UTF-8 + 中文。 |
| **NFR-4** | 双 shell 验证：Linux bash + Windows PowerShell `verify_all` 均 PASS:19。 |
| **NFR-5** | 不修改 `verify_all.sh` / `verify_all.ps1` 的检查项数量（与 T-016 / T-017 / T-018 一致）；允许在不增 check 函数的前提下新增覆盖到既有 B 组（go test）的单元测试用例。 |
| **NFR-6** | 修复方案在 PowerShell 5.1（Windows 10 / 11 / Server 2019+ 自带）与 PowerShell 7.x（用户主动安装）双 host 下行为一致；不要求 PowerShell ISE 兼容（BC-11）。 |
| **NFR-7** | 服务 stop 必须在 30 秒内完成（避免 SCM 杀进程 / 触发等价 1053 的 stop-side 错误）；frp-easy.exe 收到 SERVICE_STOP_PENDING 后必须优雅关闭 procmgr / HTTP / storage（与 main.go SIGINT/SIGTERM 链路对齐）。 |
| **NFR-8** | 服务握手 deadline：frp-easy.exe 在被 SCM 启动后必须在**首个 SCM 接受的等待周期内**（可通过 SetServiceStatus + WaitHint 延长）报告 SERVICE_RUNNING 或 SERVICE_STOPPED；如启动加载耗时 > 25 秒必须周期性递增 CheckPoint。 |
| **NFR-9** | 不引入对 frp-easy.exe 启动序列的破坏性重构：现有 `main.go` L88-93 的 `run()` 调用契约在"控制台运行"（dev / 手动跑）与"服务运行"两条入口下行为对等（除信号源不同：控制台用 SIGINT/SIGTERM，服务用 SCM Stop control code）。 |

---

## 7. 相关历史任务

> 从 `docs/tasks.md` 与 `.harness/insight-index.md` 扫描出的强相关任务。

| 任务 | 关系 | 本任务的承接点 |
|---|---|---|
| **T-008 deploy-kit**（`docs/features/_archived/deploy-kit/`） | 首次设计 Windows Service：`scripts/install-service.ps1` / `uninstall-service.ps1`、wrapper.cmd 锁 cwd、`sc.exe failure ... restart/5000`、PM-resolved Q-8 失败重启策略 | **本任务的根因层修复**：T-008 的 wrapper.cmd 只解决了"启动后 cwd 不对"的问题，没解决"frp-easy.exe 是普通控制台程序，不实现 Windows Service ABI"。SCM 启动后等 30 秒无 SetServiceStatus(SERVICE_RUNNING) 即报 1053。本任务必须保留 T-008 的：① install-service.ps1 / uninstall-service.ps1 外部 CLI 与参数契约（FR-2.5 `-DisplayName` / `-ServiceName`）；② failure action（Q-8）；③ `frp_easy.toml` 与 `.frp_easy\` 数据保留契约（FR-2.4 / AC-9）；④ 幂等性（FR-2.3 / AC-8）。 |
| **T-016 install-progress-and-systemd-unit-fix** | install.sh 进度条 / systemd unit 裸 token 语法 / 退出码透传 | 本任务的退出码透传契约（install.ps1 → install-service.ps1）已在 install.ps1 L283-290 `$LASTEXITCODE` 重置 + 透传由 T-016 PM-resolved Q-4 (a) 实现；本任务保持不退化，但 install.ps1 步骤 7 报告的退出码语义在修复后应为 **0**（成功），不再透传 install-service.ps1 的 2。 |
| **T-017 install-role-and-public-ip** | 解包后局部 chown 给 RUN_USER；Windows 路径 G3 决策"完全不动" | 本任务**部分推翻** T-017 G3 的 Windows OOS：T-017 OOS-2 推迟到本任务做 Windows 服务化深度修复。但**保留** T-017 OOS-2 的另一面：Windows 路径不引入 role 区分、不引入 chown 等价物（Windows 用 LocalSystem 跑服务，无 ACL 切换需求）。本任务在 Windows 上**不**新增 role 选择 UI。 |
| **T-018 upload-bin-multiport-ip-probe** | 二进制手动上传 + 多端口批量；install.ps1 公网 IP 探测 | install.ps1 步骤 8 的公网 IP 探测路径不动；本任务对 Get-PublicIPv4 函数与步骤 8 横幅零字节改动。 |
| **T-014 frp-binary-auto-download** | frp 子二进制运行时下载；升级期保留 `frp_win/` | 本任务的升级语义（install.ps1 L249-260 白名单覆盖、保留 `frp_win/`）零字节改动。 |
| **T-011 readme-refresh-and-network-defaults** | `Default() = 0.0.0.0` 与 main.go 安全提示 | 本任务对 main.go 安全提示零字节改动；Windows 服务 RUNNING 后 UI 在 `0.0.0.0:7800` 监听，main.go 启动期 stderr 安全提示行被 Windows Service 模式吞掉（无 TTY）—— 但日志（lumberjack 写入 `.frp_easy\logs\ui.log`）保留该行，符合 T-008 NFR-7 既定模型。 |

---

## 8. Open Questions（由 RA 选 RECOMMENDED 候选并 PM-resolved）

> 用户已明确"不要停下来问澄清问题"，故每条直接选 RECOMMENDED 候选并标 `PM-resolved`；下游 Architect 可在 02 引用本节决议、不需回头追问。

1. **frp-easy.exe 是否在自身二进制中实现 Windows Service ABI（in-process），抑或保留 wrapper.cmd + 引入外部 service helper（如 NSSM）？**
   - 候选 (a)：**in-process**——frp-easy.exe 通过 `golang.org/x/sys/windows/svc` 在 main 入口检测"是否被 SCM 启动"，是则 `svc.Run(...)` 启动服务化分支、否则走现有控制台分支；保留 wrapper.cmd 作为 cwd 锁定层（或直接通过 `sc.exe create binPath= "<exe> ServiceMode"` 单参数语义替代）。
   - 候选 (b)：保留 wrapper.cmd + 引入外部 service helper（NSSM 等）。
   - 候选 (c)：保留 wrapper.cmd + 让 frp-easy.exe 完全不知道自己在服务里跑（沿用现状，不修复——已确认会复发 1053）。
   - **PM-resolved**：**(a) in-process**。理由：与 NFR-1（不引入第三方 .exe）一致；`golang.org/x/sys` 已在 indirect 依赖中（modernc.org/sqlite 引入的），提升 direct 零额外维护成本；in-process 路径还能拿到 SCM stop control code 做真正优雅停服（NFR-7）；Go stdlib 风格的标准 Windows Service 模式，社区案例丰富（kubelet / etcd / consul 均采用同款）。

2. **服务化分支检测策略：用 sc.exe binPath 加哨兵参数（如 `frp-easy.exe service-run`），还是用 `svc.IsWindowsService()` 自动探测？**
   - 候选 (a)：**`svc.IsWindowsService()` 自动探测**——`golang.org/x/sys/windows/svc.IsWindowsService()` 通过 SCM 父进程检测、无需新增 CLI 子命令、对用户透明（双击 .exe 仍走控制台、SCM 拉起 .exe 自动走服务化）。
   - 候选 (b)：哨兵 CLI 子命令（如 `frp-easy.exe service-run`）—— 显式分流但增加 `sc.exe create binPath=` 的复杂度（必须含子命令字符串）。
   - **PM-resolved**：**(a) `svc.IsWindowsService()`**。理由：对用户透明、对 wrapper.cmd 零侵入（wrapper.cmd 仍只是 `cd /d <dir> && "frp-easy.exe"`，无需新参数）、与现有 install-service.ps1 L70-78 wrapper 模板兼容；这是 Go 团队推荐范式（见 `x/sys/windows/svc/example`）。

3. **是否保留 wrapper.cmd 作为 SCM 启动入口，还是改为 sc.exe 直接 binPath 指向 frp-easy.exe（in-process 已自带 cwd 解析）？**
   - 候选 (a)：保留 wrapper.cmd（最小变更，wrapper.cmd 仍负责 cwd 锁定 + 处理中文路径 / 空格 / UNC 等环境差异）。
   - 候选 (b)：移除 wrapper.cmd，sc.exe binPath= 直接指向 `<InstallDir>\frp-easy.exe`；frp-easy.exe 在服务化分支启动时主动 `os.Chdir(filepath.Dir(os.Executable()))` 锁 cwd。
   - **PM-resolved**：**(b) 移除 wrapper.cmd**。理由：① wrapper.cmd 引入了 host codepage 依赖（insight L17）—— 中文路径需 `-Encoding Default` 才不乱码，本身就是一层脆弱壳；② 移除后少一个中间进程，SCM stop 信号直达 frp-easy.exe（NFR-7 优雅停服更可靠）；③ Go 用 `os.Executable()` 拿到 exe 绝对路径后 `os.Chdir(filepath.Dir(...))` 是跨平台标准范式（已被 Caddy / Vault 等采用）；④ uninstall-service.ps1 L70-77 删除 wrapper 的逻辑可同步移除（避免遗留 .cmd 文件成为脏数据来源）。**注意**：本决议要求 install-service.ps1 与 uninstall-service.ps1 同步移除 wrapper.cmd 相关代码段（属本任务 in-scope）。

4. **是否引入 Windows Event Log 自定义事件源用于服务化错误诊断？**
   - 候选 (a)：引入完整 ETW / Event Log source（需 `eventlog.Install` + `mc.exe` 编译消息资源 .dll）。
   - 候选 (b)：仅用 `svc.Log`（`golang.org/x/sys/windows/svc/eventlog`）写到 Application 日志，不注册自定义 source、不编译消息资源。
   - 候选 (c)：完全不写 Event Log；服务化层错误仅写到 frp-easy 自己的 `.frp_easy\logs\ui.log`（与 Linux journalctl 对偶非对等，但用户已习惯查 ui.log）。
   - **PM-resolved**：**(c) 完全不写 Event Log**。理由：与 OOS-9 一致；ui.log 是 frp-easy 已有的统一日志通道（NFR-3 中文 UTF-8）；Event Log 自定义 source 需 mc.exe 编译资源 .dll，违反"零外部依赖"；用户排障时一处看日志比两处更清晰。

5. **SCM 启动握手期间的 CheckPoint / WaitHint 周期？**
   - 候选 (a)：固定 1 秒报告一次、WaitHint = 5 秒（保守，最坏 25 秒 + 5 秒 hint = 30 秒覆盖）。
   - 候选 (b)：固定 2 秒报告一次、WaitHint = 10 秒（频次低、单次 hint 长）。
   - 候选 (c)：自适应（每个启动阶段——appconf / storage / procmgr——之前 +1 CheckPoint + 重设 WaitHint）。
   - **PM-resolved**：**(a) 1 秒 + WaitHint 5 秒**。理由：appconf.Load + storage.Open + sqlite 迁移 + procmgr 启动通常 < 5 秒；偶发慢盘下 1 秒一次心跳能让 SCM 知道服务"在干活"；实现成本低（一个 ticker），不要求 Architect 拆分启动阶段做自适应。

6. **现有 install-service.ps1 的 sc.exe create / config 命令链是否需要新增 / 调整参数（如 `obj=` 指定运行账户）？**
   - 候选 (a)：保留现状（默认 LocalSystem，无 `obj=`），仅调整 binPath（移 wrapper.cmd → 直接 frp-easy.exe）。
   - 候选 (b)：显式 `obj= LocalSystem password=` 写入（无功能差，但显式化）。
   - 候选 (c)：切换 `NT AUTHORITY\NetworkService` 或 `NT AUTHORITY\LocalService`（更安全的内置低权账户）。
   - **PM-resolved**：**(a) 保留现状（LocalSystem）**。理由：与 OOS-7 一致；frp-easy 需写 `.frp_easy\logs\` / `frp_win\` / `frp_easy.toml`，路径在 `C:\Program Files\frp-easy\` 默认对 NetworkService 不可写；切账户会引入 ACL 调整 / DPAPI / 用户配置漂移，超出 1053 修复边界；T-008 PM-resolved Q-4 在 Linux 用 SUDO_USER，Windows 沿用 sc.exe 默认（已是事实标准）。

7. **stop 时如何同时停止 frp-easy 父进程下的 frp 子进程（frpc.exe / frps.exe）？**
   - 候选 (a)：frp-easy.exe 收到 SCM Stop 后通过 procmgr 调用 `Process.Kill()` / 等价 graceful shutdown 终止子进程（procmgr 已实现该路径用于 SIGTERM 链路，复用即可）。
   - 候选 (b)：用 Windows Job Object 把 frp-easy.exe + 子进程绑定一个 Job，frp-easy.exe 退出时 Job 自动清理所有 child。
   - **PM-resolved**：**(a) 复用 procmgr 现有 graceful shutdown**。理由：跨平台一致（Linux SIGTERM 已走该路径）；NFR-9 不引入对启动序列的破坏性重构；Job Object 是更稳健的方案但属增强，留作后续。**注**：若 QA 实测发现 stop 后有子进程残留（procmgr graceful 失败），可在 QA 报告里追加新 Issue，由后续任务 Job Object 化。

8. **install-service.ps1 在检测到旧服务处于 "marked for delete" 状态时的恢复策略？**
   - 候选 (a)：轮询等待最长 15 秒；超时后中文诊断"请关闭所有 services.msc / Get-Service / 任务管理器服务页 后重试"，退出码 2。
   - 候选 (b)：自动 kill 持有 SCM 句柄的进程（services.msc / mmc.exe）。
   - 候选 (c)：忽略，直接走 sc.exe create（让 sc 自己报 1072）。
   - **PM-resolved**：**(a) 轮询 + 诊断**。理由：(b) 自动 kill mmc.exe 会破坏用户其它 MMC 会话（如事件查看器）；(c) 让用户面对 1072 与 1053 同等糟糕。轮询窗口与现有 Wait-ServiceStopped（install-service.ps1 L30-43）模式一致，复用其超时常量风格。

9. **是否在 install.ps1 / install-service.ps1 中加 `Get-Service frp-easy | Wait-ServiceStatus Running` 之类的"启动后端到端就绪轮询"，让 IS-3 的 HTTP 200 与 install.ps1 退出之间没有时间窗？**
   - 候选 (a)：加 —— install.ps1 步骤 7 末尾轮询 `sc query` 直到 STATE 4 RUNNING，再轮询 `Invoke-WebRequest 127.0.0.1:7800` 直到首次 200/3xx/401，超时 30 秒后给中文诊断但不视为失败（服务可能慢启动）。
   - 候选 (b)：不加 —— SCM 报告 RUNNING 即视为成功，HTTP 探测留给用户复制 URL 自己点。
   - **PM-resolved**：**(a) 加 SCM RUNNING 轮询，但不加 HTTP 200 轮询**。理由：① SCM RUNNING 是 1053 修复的直接断言（IS-2 / AC-2）；轮询能确保 install.ps1 退出前 SCM 已真正 RUNNING（而非 START_PENDING），让 IS-2 / AC-2 的 "≤ 3 秒" 在用户视角变成"install.ps1 退出即满足"；② HTTP 200 轮询会引入网络/防火墙抖动，且 in-process 模式下 SCM RUNNING 时 HTTP listen 已建立（main.go 启动序列 4. 启动 HTTP server 在 SetServiceStatus(SERVICE_RUNNING) 之前完成），HTTP 200 是 SCM RUNNING 的必然推论；不加可让 install.ps1 收尾更快、终端输出更干净。

10. **是否在本任务为 `frp-easy.exe` 添加 `--service-debug` 之类的"以控制台姿态模拟服务化分支"调试 flag，便于开发者在本地不真正注册服务也能跑通服务化代码路径？**
    - 候选 (a)：加。
    - 候选 (b)：不加（开发者用 `sc.exe create` 本地装临时服务测试）。
    - **PM-resolved**：**(b) 不加**。理由：增加 CLI 表面积；服务化分支的核心逻辑（SetServiceStatus 序列、CheckPoint 累加、Stop control code 处理）必须真实 SCM 才能验证；本地 `sc.exe create frp-easy-dev binPath= "<path>\frp-easy.exe"` + `sc.exe start frp-easy-dev` 已足够开发测试。

---

## 9. Verdict

**READY**

理由：
- §2 IS-1 ~ IS-12 每条 in-scope 行为均在 §5 AC 有可观测验证方法；
- §3 OOS-1 ~ OOS-13 把"卸载流程改造、Linux/macOS 路径、frp 子进程服务化"等明确切边，避免本任务边界蔓延；
- §4 BC-1 ~ BC-14 覆盖管理员权限、中文/空格/UNC/只读盘路径、卡死服务、marked-for-delete、Defender 拦截等关键错误路径；
- §5 AC-1 ~ AC-19 全部可由 QA 复现（含 verify_all PASS:19 与 06 报告英文裸标题红线 AC-17）；
- §6 NFR 锁定不引入新依赖、不破坏现有契约、双 host 一致；
- §7 列出 6 个相关历史任务及精确承接点，本任务**部分推翻** T-017 G3 的"Windows 不动"决策（明确仅推翻服务化深度修复部分，role / 公网 IP 探测仍保持 G3）、**保留** T-008 / T-016 / T-018 全部既有契约；
- §8 全部 10 条 Open Questions 已由 RA 选 RECOMMENDED 并 PM-resolved（用户指示），下游 Architect 可直接基于本决议进入 Stage 2。

下游 Architect 应基于 §2 in-scope / §5 AC / §6 NFR / §8 决议做：
1. frp-easy.exe 的 Windows Service ABI 集成位置（main.go 是否拆 main_windows.go）；
2. install-service.ps1 / uninstall-service.ps1 的 wrapper.cmd 移除路径与回滚；
3. install.ps1 步骤 7 的 SCM RUNNING 轮询实现；
4. procmgr graceful shutdown 在 SCM Stop control code 下的链路验证；
5. NFR-8 的 CheckPoint / WaitHint 周期实现（Q-5 决议 = 1 秒 + 5 秒 hint）。
