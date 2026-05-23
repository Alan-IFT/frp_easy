# PM_LOG — T-019 windows-service-scm-1053-fix

> 任务路由日志。每个 stage 入/出时间、产物路径、决策都追加一行。

## 任务背景

- **触发**：用户在 Win11 管理员终端执行 `irm .../install.ps1 | iex` 升级安装，sc.exe start 失败退出码 **1053**（"服务没有及时响应启动或控制请求"）。
- **根因猜测**（待 RA 确认）：`frp-easy.exe` 是控制台程序，未实现 Windows SCM (`StartServiceCtrlDispatcher`) 协议；包装的 `.cmd` 同样不会让 SCM 收到 RUNNING 状态报告，SCM 默认 30s 超时即报 1053。
- **历史**：T-008 (deploy-kit) 初次设计 Windows Service；T-008 03_GATE_REVIEW 的 MINOR-3 已标注 "sc.exe stop 优雅传播待 QA 验证"；本任务暴露的是更严重的"根本不能 start"问题。Insight 行 18 关于 wrapper.cmd 与 host codepage 的设计仍有效（cwd 锁定），但 SCM 协议层缺失被遗漏。

## Stage 流转

- **2026-05-23 Stage 1 入** → 派发 Requirement Analyst。
- **2026-05-23 Stage 1 出** → `01_REQUIREMENT_ANALYSIS.md` 已落盘 verdict: READY；§8 10 条 Open Questions 均 PM-resolved（用户指示"不停下来澄清"）。决议要点：(Q1/Q2) in-process `golang.org/x/sys/windows/svc` + `IsWindowsService()` 自动探测；(Q3) **移除 wrapper.cmd**，binPath 直指 frp-easy.exe，Go 用 `os.Executable()` + `os.Chdir` 锁 cwd；(Q4) 不写 Event Log；(Q5) CheckPoint 1s + WaitHint 5s；(Q6) 保留 LocalSystem；(Q7) 复用 procmgr graceful shutdown；(Q8) marked-for-delete 走轮询 + 中文诊断；(Q9) 加 SCM RUNNING 轮询不加 HTTP 探测；(Q10) 不加 --service-debug。
- **2026-05-23 Stage 2 入** → 派发 Solution Architect。
- **2026-05-23 Stage 2 出** → `02_SOLUTION_DESIGN.md` 已落盘 verdict: READY；新增 service_windows.go / service_other.go 双入口 + 改 install-service.ps1 移除 wrapper.cmd + 加 SCM RUNNING 轮询 + go.mod 提升 x/sys 为 direct。
- **2026-05-23 Stage 3 入** → 派发 Gate Reviewer。
- **2026-05-23 Stage 3 出** → `03_GATE_REVIEW.md` 已落盘（gate-reviewer 工具集无 Write，PM 代为落盘 — insight L41 红线场景）；verdict **APPROVED WITH CONDITIONS**：C-1/C-2 必须（go.mod tidy 跑法 + run() 两参签名），C-3/C-4 建议（QA 如实记录 AC-15 与体积），C-5 提示（F-5/F-6 列后续任务，不阻塞本任务）。
- **2026-05-23 Stage 4 入** → 派发 Developer，重点提醒 C-1/C-2 与 F-1/F-2/F-3/F-5/F-7。
- **2026-05-23 Stage 4 出** → `04_DEVELOPMENT.md` 已落盘 verdict: READY FOR REVIEW；`verify_all` PASS:19；新增文件 service_windows.go / service_other.go / service_windows_test.go；编辑 main.go (run 两参签名 + service 分流) / install-service.ps1 (移除 wrapper 生成 + 防御性清理 + Wait-ServiceRunning) / uninstall-service.ps1 (注释升级为防御性清理) / go.mod (x/sys 升 direct，手编未跑 tidy)。
- **2026-05-23 Stage 5 入** → 派发 Code Reviewer。
- **2026-05-23 Stage 5 出** → `05_CODE_REVIEW.md` 已落盘（同 Stage 3，由 PM 代为落盘）；verdict **APPROVE WITH MINOR FIXES**：0 P0 + 0 P1，6 P2 + 4 Nit 全部 nice-to-fix；C-1/C-2 必须项已在代码 + 04 双重落地；C-3/C-4 留 Stage 6；C-5 转给 PM 作为后续 T-021 (service-mode-stderr-bridge) backlog 候选。
- **2026-05-23 Stage 6 入** → 派发 QA Tester，强调英文裸标题 `## Adversarial tests`（AC-17 / insight L29+L40 红线）+ C-3 如实记录 Linux 发行版 + C-4 体积对比。
- **2026-05-23 Stage 6 出** → `06_TEST_REPORT.md` 已落盘 verdict: APPROVED FOR DELIVERY；verify_all 三跑稳定 PASS:19；0 BLOCKER + 0 CRITICAL；1 MAJOR D-1 PS5.1 zh-CN BOM 历史遗留（T-018 同款，非 T-019 引入，不阻塞）。Follow-up backlog 编号：QA 建议 T-020/T-021 但 T-020 名已被 claude-settings-context7-fix 占用 → PM 在 07 重编为 T-021 (encoding-ps51-bom) / T-022 (service-mode-stderr-bridge)。11 条 AC 真机 SCM 动态部分 PENDING-USER-VERIFY（QA 主机非管理员），转 §6 用户清单。
- **2026-05-23 Stage 7 入** → PM 直接写 07_DELIVERY.md，跑 verify_all PASS:19 确认，准备 archive-task。
- **2026-05-23 Stage 7 出** → 07_DELIVERY.md 落盘，verdict: DELIVERED；4 条 insight 进入 `## Insight` 段供 archive-task 收割。
