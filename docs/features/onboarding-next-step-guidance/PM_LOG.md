# PM 编排日志 — T-062 · onboarding-next-step-guidance

> PM Orchestrator 路由决策日志。模式：full（7-stage）。分区：dev-frontend。
> 批次：ux-ui-uplift-2026-05（第 1 个任务）。

## 任务元信息

- Task ID: T-062
- Slug: onboarding-next-step-guidance
- Mode: full（1→2→3→4→5→6→7）
- Partition: dev-frontend（用户已明确全部前端）
- 输出语言：中文（红线 .harness/rules/00-core.md）
- 当前基线：version=27, frontend_tests=500, go_tests=322, test_count=822

## 启动前 PM 核实（cross-task memory + 红线门禁）

读取 `.harness/insight-index.md`，识别适用条目并将传给下游：
- **L17**：SPA 内导航必须 `router.push`，禁 href/tag=a（本任务核心约束，所有 5 项导航均受约束）。
- **L30/L31**：顶级路由 /wizard 不嵌 AppLayout，顶栏入口在向导内不可见 → Wizard 引导必须就地呈现（不能指望顶栏）。已核 router.ts:10 /wizard 与 / 平级。
- **L31**：PM 上下文无 Bash → verify_all 标 PENDING + 执行规格。
- **L45 / T-057**：测试断言全用 DOM 文本 / find('button') / getExposed，零 naive-ui 组件名查询。

PM 预读关键源文件（为各阶段提供准确依据）：
- Wizard.vue：step3 完成分支已有 binWarning===0（自动跳转）/ else（缺失警告）两分支；missingForRole/goToDashboard/handleNext 已 defineExpose。token 字段 frps L71-79 / frpc L112-120。新引导须作为第 3 路（正向下一步）并存。
- Client.vue：handleSave success 后仅 message.success，无后续引导；defineExpose 已有 form/handleSave/loadConfig 等。
- Proxies.vue：#empty 模板 L31-33 已有"暂无代理规则，点击右上角「新增规则」开始配置"；handleSubmit success 后 message + firewallPorts，无去 Dashboard/Monitor 引导。
- Server.vue：#action 区有"保存配置"/"重新加载"按钮，无"查看运行态"链接。
- ProxyForm.vue：远程端口字段 L56-68，n-input-number 无 help 文案/placeholder 关于端口策略。
- ServerMonitor.vue:218-220 goServerConfig→router.push('/server')（既有反向跳转，本任务补 Server→Monitor 正方向形成双向）。

PM grep e2e spec（web/tests/e2e/）核实断言冲突：
- 03-dashboard.spec.ts 断言仅 getByText('仪表盘')/('frpc（客户端）')/('frps（服务端）')/getByRole('button',{name:'退出登录'})，用 bypassWizard 绕过向导，不进 Proxies/Server 编辑流。
- 01-setup/02-auth 仅 setup/login 表单，无新引导文案断言。
- **结论：新增引导文案对 e2e 零冲突**（符合 insight L34）。无需改 e2e。

分区检测：`.harness/agents/dev-*.md` 存在 dev-db/dev-backend/dev-frontend → 分区模式。本任务用户已明确全前端 → dev-frontend。

## 阶段路由记录

### Stage 1 — Requirement Analyst（完成）
- 时间：2026-05-30
- 动作：派发 requirement-analyst，附 INPUT.md + insight L17/L30/L31/L45 + e2e 核实结论。
- 产出：01_REQUIREMENT_ANALYSIS.md，7 项 IS（IS-1~IS-7）+ 10 BC + 13 AC + 7 OOS。
- 裁决：**READY**，无开放问题（用户 INPUT 已对关键决策点定调）。
- PM 检查：无 BLOCKED 标记，无回退请求。verdict READY。
- 路由决策：**advance → Stage 2（Solution Architect）**。

### Stage 2 — Solution Architect（完成）
- 时间：2026-05-30
- 动作：派发 solution-architect，附 01 + INPUT.md。分区模式 → 产出 Partition assignment 段。
- 产出：02_SOLUTION_DESIGN.md。逐文件设计 6.1~6.5 + 复用审计（零新依赖）+ 5 风险 + Partition assignment（单分区 dev-frontend）。
- 关键设计选择：5 项引导全内联（不抽公共组件，页面特化 + 与 T-048/T-057 既有范式一致）；ProxyForm 用方案 A（常驻 help 文本，可稳定 DOM 断言）；Proxies 空态仅加"去监控"1 入口（避免拥挤）；Client/Proxies/Server.spec 新增 vue-router push mock（复用 Wizard.spec:58-59 范式）。
- 已识别风险 R-1：IS-1 全就绪态引导按钮与 T-057 自动跳转同 tick 的 UX 张力，设计明确"不破坏 T-057 优先、接受局限、验收点为按钮存在+可点击 push('/proxies')"，留 GR 评估放置。
- 裁决：**READY**（单分区，含 Partition assignment）。
- 路由决策：**advance → Stage 3（Gate Reviewer）**。

### Stage 3 — Gate Reviewer（完成）
- 时间：2026-05-30
- 动作：派发 gate-reviewer（Mode A：PM 有 Write，直接落盘 03）。GR 实读 5 个 SFC + router.ts + Wizard.spec + grep import 现状。
- 产出：03_GATE_REVIEW.md。8 维度全 PASS，无 FAIL。
- 关键裁定：R-1（IS-1 自动跳转张力）经独立评估**不构成需求层缺陷，不回退 RA**——验收点是"按钮存在+可点击 push"，移除自动跳转会破坏 T-057（设计已正确拒绝）。转为开发条件 C-1。
- GR 补充事实（F-2）：import 增量 = Client +NAlert / Proxies +NAlert(+NText 若用) / ProxyForm +NText；Wizard/Server 零新 import。
- 裁决：**APPROVED WITH CONDITIONS**（C-1~C-6）。满足 stage gate（明确通过裁决）。
- PM 检查：无 BLOCKED，无回退。3 维度通过。
- 路由决策：**advance → Stage 4（Developer）**。读 02 Partition assignment → 单分区 dev-frontend → 派发 dev-frontend。

### Stage 4 — Developer / dev-frontend（完成）
- 时间：2026-05-30 → 2026-05-31
- 分区检测：02 §11 Partition assignment 全部 dev-frontend（web/** + baseline + dev-map）。单分区。
- 动作：派发 dev-frontend，附 01/02/03 + 条件 C-1~C-6 + 红线。
- 产出：04_DEVELOPMENT.md + 5 源文件 + 5 spec + baseline + dev-map 实改。
- 改动摘要：IS-1~IS-7 全实现。Wizard step3 加规则引导（仅 frpc/both + binWarning===0）+ step2 token 不一致 warning；Client/Proxies 保存成功引导；Proxies 空态连通入口；Server 查看运行态链接；ProxyForm 端口策略纯文案。+32 前端测试（500→532 / version 28）。
- 条件落实：C-1（验收点+AC-11 护栏）/ C-2（复用 Wizard.spec router mock 范式）/ C-3（import 精确）/ C-4（baseline bump）/ C-6（router.push + 不越界）全达成；C-5 裸标题段交 06。
- 无 DESIGN DRIFT。所有导航 router.push（insight L17）。未碰后端/store/router.ts/守卫/数据流。
- verify_all：**PENDING**（dev 上下文无 Bash，insight L31）；执行规格交 07。stage gate（Stage 4 verify PASS）按批次约定标 PENDING，真跑由 orchestrator 作硬闸门。
- 裁决：**READY FOR REVIEW**。
- 路由决策：**advance → Stage 5（Code Reviewer）**。

### Stage 5 — Code Reviewer（完成）
- 时间：2026-05-31
- 动作：派发 code-reviewer（Mode A，直接落盘 05）。CR 实读全部 12 改动文件，逐 AC 走查 01、逐 §6 走查 02。
- 产出：05_CODE_REVIEW.md。13/13 AC 覆盖 ✅，9 项设计保真 ✅ 无 drift，6 维度全 PASS。
- 发现：0 CRITICAL / 0 MAJOR / 1 MINOR（引导标志不重置，GR Q2/Q3 已预答可接受）/ 1 NIT（IS-1 R-1 张力，GR C-1 已定调）。
- 测试质量：反向证伪有效（token 不阻断/纯空白不误报/失败态不出现空态入口均真证伪）；T-057 既有用例零回归；+32 计数一致。
- 裁决：**APPROVED**。
- Mode A 文件存在性核验：05_CODE_REVIEW.md 已落盘 ✅。
- 路由决策：**advance → Stage 6（QA Tester）**。

### Stage 6 — QA Tester（完成）
- 时间：2026-05-31
- 动作：派发 qa-tester（实际可用工具 Read/Write/Edit/Glob/Grep，无 Bash，insight L31）。
- 产出：06_TEST_REPORT.md（含裸标题 `## Adversarial tests`，C-5 ✅ / E.6 闸门 ✅）+ QA 独立新增 2 个反向证伪用例（Wizard.spec：缺失态分支互斥、push 两路径独立共存）。
- QA 净增：Wizard +2（总 +14）→ 全任务 +34 前端测试。baseline 再 bump：frontend_tests 532→534、test_count 854→856（version 维持 28）。
- 反向证伪：5 条对抗（QA 独立 2 + dev 3 复核）实现全部存活；核心张力（IS-1 分支互斥/token 非阻断/空态入口隔离）锁死。
- 回归：T-057 既有 30+ 用例零回归；e2e 零冲突（QA 复核 grep，insight L34）。
- verify_all：**PENDING**（无 Bash），执行规格 856 passed / 0 failed，交 orchestrator 真跑。
- 裁决：**APPROVED FOR DELIVERY**。0 缺陷。
- 路由决策：**advance → Stage 7（Delivery）**。stage gate：Stage 5 APPROVED + Stage 6 APPROVED FOR DELIVERY 双 PASS ✅。

### Stage 7 — Delivery（完成）
- 时间：2026-05-31
- 产出：07_DELIVERY.md（含裸 ## Insight 2 条）。
- 更新 docs/tasks.md：T-062 从 进行中 移到 已完成，DELIVERED。
- 最终状态：7 阶段全过，0 rollback。+34 前端测试（500→534 / 822→856 / version 28）。
- verify_all：PENDING（无 Bash），执行规格交 orchestrator 真跑硬闸门。
- 按批次约定：**未 commit / 未 push / 未跑 archive-task**（交 batch orchestrator 在 verify 通过后统一处理）。

## 任务终态

- 结果：**DELIVERED**
- 07 路径：`docs/features/onboarding-next-step-guidance/07_DELIVERY.md`
- files_changed：12（5 源 + 5 spec + baseline + dev-map）
- frontend_tests：500 → 534
- rollback：0
- verify_all：PENDING（预期 PASS）
- 关键改动：补 Wizard/Client 加规则引导 + Proxies/Server 跨页连通 + ProxyForm 端口策略文案 + Wizard both token 不一致非阻断预警，纯前端导航/文案/一处 computed，不破坏 T-057。
