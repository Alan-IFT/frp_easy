# PM_LOG — T-017 install-role-and-public-ip

> PM Orchestrator 在此记录路由决定与阶段交接。**绝不**在此给专业意见。

## 2026-05-23 · 任务创建

- 任务来源：用户运行 `curl | sudo bash` 后 systemctl status 显示 frp-easy 死循环重启 + 安装横幅只显示局域网 IP（10.1.20.7）。
- 模式：**full**（7-stage）。理由：涉及修 bug + 改变安装期 UX + 改默认配置语义，跨 install.sh / install-service.sh / appconf 三个层面，不是琐碎修复。
- 相关历史任务（任务看板扫描）：
  - **T-016 install-progress-and-systemd-unit-fix**（2026-05-23）：systemd unit 写法 / 进度条 / 退出码透传。**本任务的 install-service.sh 故障是其遗留**（unit 写好了，但 RUN_USER 默认到 SUDO_USER 时未授予 INSTALL_DIR 写权限）。须读 T-016 的 02_SOLUTION_DESIGN.md 理解 unit 模型。
  - **T-011 readme-refresh-and-network-defaults**（2026-05-21）：决定默认监听 `0.0.0.0`。本任务要拆这个"一刀切默认"为 role-based 默认 —— 必须读 T-011 的 01 / 02 看决策上下文。
  - **T-012 one-click-install-and-mit-license** + **T-013 rolling-release-install**：install.sh 整体形态与 curl|bash 非交互问题的起源。
  - **T-014 frp-binary-auto-download**：frp 二进制运行时下载的设计，涉及 UI 启动后才能下 frp。
- 已读 `.harness/insight-index.md`：3 条 insight 直接相关 ——
  - `curl|bash` 形态禁用 `$0`/`$BASH_SOURCE` 自定位 → 安装期交互必须显式处理 TTY 是否可用。
  - 06_TEST_REPORT.md 必须含**英文** `## Adversarial tests` 标题（verify_all E.6）。
  - sudo 下 `id -un` 返回 root，真实用户用 `${SUDO_USER:-$(id -un)}` —— 这一条是触发本 bug 的直接原因。

## 2026-05-23 · 派发 Stage 1 → Requirement Analyst

待 analyst 写 `01_REQUIREMENT_ANALYSIS.md`，特别要求覆盖：

- 用户三诉求的拆分（崩溃修复 / IP 显示 / 安装期角色选择）—— 每条独立的可测试 AC。
- 非交互（`curl | sudo bash`）下"安装期角色选择"的可行性（环境变量？stdin redirect？默认值？后置 wizard？）—— 把每种方案的歧义列给用户。
- 公网 IP 探测的"可信源"问题（多家 echo IP API、超时、隐私、内网/防火墙拒外的降级）。
- 与 T-011 既有"默认 0.0.0.0"决策的兼容/迁移策略。

## 2026-05-23 · Stage 1 完成 → Analyst Verdict = BLOCKED ON USER（8 个 ambiguities）

Analyst 严格未替用户决定，列了 AMBIG-C / D / E / F / G / H / I / J 八条。

**用户在本次会话指令上有约束**：要求不停下来问澄清问题，"做合理判断、用户随时可纠正"（system reminder 明示）。
**用户原话已隐含的偏好（PM 据此预先剔除候选）**：
- "理论上好像只能在安装脚本运行过程中选择" → 排除 **AMBIG-C 候选 C5**（装完进 Web 向导）。
- "服务端需要公网 IP，客户端监听 127.0.0.1 最安全" → 锁定 server=0.0.0.0、client=127.0.0.1 的语义。

**PM 决议（写入此处留痕，Architect 据此推进）**：

| AMBIG | 决议 | 理由 |
|---|---|---|
| **AMBIG-C** | **C1 + C4 组合**：核心实现走 C1（环境变量 `FRP_EASY_ROLE=server\|client`），同时 README 提供两条入口命令（实质是 `FRP_EASY_ROLE=…` 前缀的 wrapper 文案，无需新文件）。**子问题 C1.a = (a) 拒绝并报错**——未指定 role 时打印两条入口命令后 exit 1，绝不静默默认。 | 保留 `curl|bash` 一键承诺、非交互、单 install.sh 维护成本；拒绝静默默认呼应用户"装完看得出是哪种"的隐含诉求。 |
| **AMBIG-D** | **D1 保留用户值优先** | 兼容 T-011 NF-2；本任务 install 期会**预生成** frp_easy.toml（见 E3），所以"用户值"只在用户主动改过时存在，D1 不会真的让 role 切换体验糟糕。 |
| **AMBIG-E** | **E2 + E3 组合**：仅 chown 运行时可写路径（`frp_easy.toml` + `.frp_easy/` + `frp_linux/`），且 install.sh 解包后**预生成** `frp_easy.toml`（含 role-derived UIBindAddr）再 chown，避免首启走"写默认值"分支。binary、scripts 保持 root:root。 | 最小权限 + 首启确定性。 |
| **AMBIG-F** | **F2 仍打两行，标注"公网 IP 与局域网 IP 相同（本机直接在公网上）"** | 信息最完整、最不误导；省事的合并形态留给用户自己看出来。 |
| **AMBIG-G** | **G2 部分同步**：Windows install.ps1 同步公网 IP 探测修复（FR-B），role 选择形态推迟（OOS-2 保留）。 | Windows 暂无活跃 server 用户实测反馈；优先修 Linux 根因。 |
| **AMBIG-H** | **H1 拒绝并报错** | 显式比静默切换安全；要切换 role 必须先 uninstall 或 `FRP_EASY_FORCE_ROLE=yes`。 |
| **AMBIG-I** | **I1 OOS** | 用户原话未要求；frp 业务进程定义留给 UI 内创建。 |
| **AMBIG-J** | macOS 维持现状（OOS-10 保留） | macOS 无 systemd 根因不存在；FR-B 公网 IP 探测在 macOS 分支可同步修复（与 G2 一致原则）。 |

下一步：派发 Solution Architect，把上述决议作为锁定输入。

## 2026-05-23 · Stage 2 完成 → READY，转 Gate Review

Architect 产出 02_SOLUTION_DESIGN.md：
- 核心：install.sh §6.5 新块（预生成 toml + 局部 chown + .role 持久化）；新退出码 3（role 缺失/非法）；公网 IP 探测函数 + 3 个明文写死候选（api.ipify.org / ifconfig.me / icanhazip.com）；install.ps1 仅同步 FR-B；**Go 代码 / main.go / appconf / unit 语法全不动**。
- 复用 18 项 vs 新增 3 bash 函数 + 1 PS 函数 + 1 toml 模板 + .role 单行文件。
- 8 条 AMBIG 决议全部映射到具体 §。
- Verdict = READY、Open questions = 空。

下一步：派发 Gate Reviewer 做 8 维度审计 + 实际打开文件验证设计引用的代码存在。

## 2026-05-23 · Stage 3 完成 → APPROVED WITH CONDITIONS（9 条），转 Developer

Gate Reviewer 落盘 03_GATE_REVIEW.md：
- Verdict = **APPROVED WITH CONDITIONS**
- 计数 PASS:5 / WARN:3 / FAIL:0；Critical:0 / Major:3（M-1 RUN_USER 表达式 verbatim、M-2 国内 VM IP 探测失败兜底、M-3 不做 source-mode 包裹）/ Minor:5
- 9 条 conditions（C-1 ~ C-9）已落入 03 §6，由 Developer 在 04 阶段逐条落地

无 critical → 不路由回去。直接进入 Stage 4 dev-backend（设计 §11 分区分配仅 dev-backend）。

## 2026-05-23 · Stage 4 完成 → verify_all PASS:19，转 Code Review

dev-backend 改 3 个脚本（install.sh +293/-13、install.ps1 +76、uninstall-service.sh +6）：
- 新增 -h 文案 role 用法、§0.5 ROLE 解析（exit 3）、render_frp_easy_toml + detect_public_ip 两 helper、§6.5 role 应用 + 局部 chown、步骤 8 role-aware 横幅
- install.ps1 新增 Get-PublicIPv4 + 注释 OOS-2
- uninstall-service.sh 末删 .role

verify_all 结果 = **PASS:19 / WARN:0 / FAIL:0 / SKIP:0**（baseline 不动）。
9 条 conditions 全部落地（C-4 / C-8 / C-9 跨阶段传递部分已写入 04 §4.2）。
Inv-1~7 不变量全守护：Go 代码 / main.go / install-service.sh / frp_linux/ 升级 / verify_all 检查项数量未动。

下一步：派发 Code Reviewer 对照 01/02 + 9 条 conditions 审 04 代码改动。

## 2026-05-23 · Stage 5 完成 → APPROVED，转 QA

Code Reviewer 落盘 05_CODE_REVIEW.md：
- Verdict = **APPROVED**
- 6 维度全 PASS；Critical:0 / Major:0 / Minor:2 / Nit:1
- 9 conditions C-1~C-9 全部落地核实通过（含 file:line 证据）
- Inv-1~Inv-7 全部守护
- MIN-1（IPv6 横幅 bracket 缺失，BC-3 设计要求但实现遗漏）—— 边缘 case，不阻塞合并，QA 作为已知限制覆盖

下一步：派发 QA Tester，重点跑 §7 列出的 10 项 Adversarial tests。

## 2026-05-23 · Stage 6 完成 → APPROVED FOR DELIVERY，转 PM 写 07

QA 落盘 06_TEST_REPORT.md：
- Verdict = **APPROVED FOR DELIVERY**
- 31 Adversarial tests / 30 PASS / 1 FAIL（唯一 fail = AT-MIN-1 IPv6 bracket，已识别为 KL-1 已知限制）
- verify_all PASS:19 / FAIL:0（最终闸门连跑 3 次稳定）
- Inv-1~Inv-7 全守护（appconf / main.go / install-service.sh 与 main 分支 byte-level 一致）
- E.6 红线（`## Adversarial tests` 英文标题）命中
- baseline.json 不修改（test_count 231 不变）

## 2026-05-23 · Stage 7 完成 → DELIVERED

PM 写 07_DELIVERY.md：
- 8 条 AMBIG 决议执行映射全部落实
- 9 conditions C-1~C-9 全落地
- 4 项 Known Limitations 显式记录（KL-1 IPv6、KL-2 go-toml 大小写、KL-3/4 Windows 路径降级测试）
- 2 条 insight 已追加到 `.harness/insight-index.md`（手动追加，archive-task.sh 仅移动文件）
- 阶段文档已 archive 到 `docs/features/_archived/install-role-and-public-ip/`
- docs/tasks.md 更新为 DELIVERED

任务关闭。
