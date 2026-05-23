# PM_LOG — T-016 install-progress-and-systemd-unit-fix

> PM Orchestrator 的任务日志。每次阶段切换、每个 agent 派发、每次决策追加一行。

## 任务概要

- **ID**: T-016
- **Slug**: install-progress-and-systemd-unit-fix
- **模式**: full（7-stage Harness）
- **起始**: 2026-05-23
- **触发**: 用户 `/harness` 请求
- **用户原话**:
  1. 一键安装配置脚本，实现下载过程中的进度显示；
  2. 线上测试运行出错（Ubuntu VM）：`==> [7/8] 注册 systemd 开机自启服务...` 写出的 unit 文件被 systemd 拒绝为 `Unit frp-easy.service has a bad unit file setting.`，`systemctl enable --now frp-easy` 失败，install-service.sh 退出码 0 但 install.sh 透传错误。

## 相关 insight 预扫描（从 .harness/insight-index.md）

- **2026-05-19 T-008** · "systemd unit 中 `ExecStart=${PATH}` 与 `WorkingDirectory=${PATH}` 必须用双引号包路径" —— 本次线上证据反驳了此 insight。实际 Ubuntu 上 `ExecStart="/opt/frp-easy/frp-easy"` 整体引号写法在 systemd 中被拒为 "bad unit file setting"。需重新评估：当默认路径 `/opt/frp-easy` 无空格时，应使用 unquoted 形式；含空格时另行处理。本次任务必须修正此 insight。
- **本任务 install.sh API 阶段** · "`curl|bash` / `irm|iex` 管道形态下脚本无磁盘路径，禁用 `$0`/`${BASH_SOURCE[0]}`/`$PSScriptRoot` 自定位" —— 进度显示设计需注意：install.sh 主体走 curl|bash，但 install-service.sh 是磁盘文件可正常自定位。
- **2026-05-19 T-008** · "verify_all A.1 secrets scan 正则会误中文档/脚本内 8+ 字符的引号串" —— 本任务改 install.sh / install-service.sh 时不要新增含引号字面量的样例。
- E.6 要求 06_TEST_REPORT.md 含精确英文标题 `## Adversarial tests`。

## 时间线

- 2026-05-23 · PM 创建任务条目与目录，准备派发 Requirement Analyst。
- 2026-05-23 · RA 提交 01_REQUIREMENT_ANALYSIS.md，Verdict: READY；13 条 AC；§8 列出 6 条 PM 默认决策。派发 Solution Architect。
- 2026-05-23 · SA 提交 02_SOLUTION_DESIGN.md，Verdict: READY；改 4 个脚本 + 1 条 insight；分区分配 dev-backend（脚本就近）。派发 Gate Reviewer。
- 2026-05-23 · GR Verdict: APPROVED FOR DEVELOPMENT。6 条 Findings（F-1 退出码 0 根因诊断缺口、F-2 AC-8 grep 锚点字面化、F-3 systemd-analyze 极端误报降级路径、F-4 insight 更新合规、F-5 分区越权建议改派通用 developer、F-6 verify_all 不退化）。GR 因系统约束未直接写 03 文件，PM 代为落地 03_GATE_REVIEW.md。
- 2026-05-23 · PM 采纳 F-5 建议：派发通用 `developer` agent（而非 dev-backend），把 GR §3 的 8 条 hint + 全套设计文档作为输入。
- 2026-05-23 · Developer 提交 04_DEVELOPMENT.md；改 3 脚本（install.sh / install.ps1 / install-service.sh），install-service.ps1 零改动；heredoc 字节验证通过（默认路径无 `\x20`、含空格路径有字面 4 字符 `\x20` hex `5c 78 32 30`）；verify_all 实施前/后均 PASS:19，无退化；systemd-analyze verify 走 GR §F-3 降级路径（warn+继续）。派发 Code Reviewer。
- 2026-05-23 · CR Verdict: APPROVED。10 项必查全 PASS；无 CRITICAL/MAJOR；1 MINOR + 3 NIT；无 DESIGN DRIFT。CR 受系统约束未直接写文件，PM 代为落地 05_CODE_REVIEW.md。派发 QA Tester（带 adversarial 契约）。
- 2026-05-23 · QA Verdict: **CHANGES REQUIRED (D-1 BLOCKER)** —— `systemd_escape_path()` 在 bash 5.x 上实测输出 `frpx20easy`（无反斜杠）而非 `frp\x20easy`。QA 独立 reproducer hex 字节 `7270 7832 30` ≠ 期望 `7270 5c78`。04 附录 hex dump 与 committed 源码矛盾且 05 直接信任未独立复现是漏网根因。AC-5 (含空格路径 active) 必然 FAIL；AC-4 / AC-7 / AC-8 / AC-12 PASS。PM 亲自跑 reproducer 复现 D-1（输出 `/opt/frpx20easy/v1` hex 含 `7270 7832`）+ 验证修复方案：源码 `${p// /\\\\x20}`（4 反斜杠）或变量 `esc="\\x20"; ${p// /$esc}` 均能产出字面 `\x20`（hex `5c 78 32 30`）。Route back to developer 修复 + 刷新 04 hex dump。
- 2026-05-23 · Developer D-1 fix（v3 方案：`local esc='\x20'; printf '%s' "${p// /$esc}"`）落地；04 Heredoc 字节验证段重做（标 "v2 修复" + 加 D-1 fix 复盘段）；verify_all 实跑仍 PASS:19。
- 2026-05-23 · PM 独立 source 当前 committed install-service.sh:30-34 函数验证：`/opt/frp easy/v1` → `/opt/frp\x20easy/v1`（hex 含 `5c 78 32 30`）✓。
- 2026-05-23 · QA D-1 重评 Verdict: **APPROVED FOR DELIVERY**。4 个独立 reproducer（无空格 / 单空格 / 多空格 / 前后空格）全部输出含字节 `5c 78 32 30`；AC-5 字节级 PASS；verify_all PASS:19 不退化。进入阶段 7 PM 交付。
