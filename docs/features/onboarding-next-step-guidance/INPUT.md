# 任务输入 — T-062 · onboarding-next-step-guidance

> 由 PM Orchestrator 写入。下游 agent 只读此文件 + 自行读取相关源码/历史文档。
> **模式：full（完整 7-stage 流水线）。输出语言：中文（红线）。分区：dev-frontend（全部前端）。**
> **批次：ux-ui-uplift-2026-05 的第 1 个任务。**

## 一句话目标

补正向 onboarding 引导与跨页连通，消除"配好了不知道下一步"的断点 + token 不一致预警。纯 UI 文案 + 导航 + 一处条件比较，**不碰后端/store/API/路由守卫/数据流**。

## 范围（5 项，全前端 dev-frontend 分区）

1. **Wizard 完成页 + Client.vue 保存成功后**追加"下一步：前往『代理规则』添加要转发的端口 →"链接按钮，点击 `router.push('/proxies')`。
   - Wizard：`web/src/pages/Wizard.vue`，frpc / both 角色的完成分支（step3）。Wizard 是顶级路由 `/wizard`，**不在 AppLayout 内**（insight L30/L31），顶栏入口在向导内不可见，故引导必须就地呈现。
   - **不得破坏 T-057 现有完成逻辑**：missingForRole 二进制缺失分支、binWarning 定格快照、全就绪自动跳转。新引导是在"全就绪自动跳转"与"缺失就地警告"**之外的正向下一步提示**——建议：仅在 frpc/both 且无 frpc 缺失阻断（即所选角色对应二进制就绪）时展示加规则引导，与既有分支正确并存。
   - Client.vue：`web/src/pages/Client.vue`，保存成功（handleSave success 后）追加同款"下一步"引导。
2. **Proxies.vue**（`web/src/pages/Proxies.vue`）：保存（新增/编辑）代理成功后、以及空态文案处，引导用户去 Dashboard 启动 frpc / 去 ServerMonitor 看运行态（`router.push`）。空态已有引导文案（Proxies.vue:31-33 #empty 模板），在其上补"去仪表盘启动"或"去服务端监控"链接即可，避免重复。
3. **Server.vue**（`web/src/pages/Server.vue`）：配置页加"查看运行态 →"链接跳 `/server/monitor`，与 ServerMonitor→Server 既有跳转（ServerMonitor.vue:218-220 goServerConfig→push('/server')）形成双向连通。
4. **ProxyForm.vue**（`web/src/components/ProxyForm.vue`）：远程端口字段加 help 文案/placeholder"需在服务端『端口策略』允许范围内"。**纯文案**，不做跨页校验联动（联动是中-高成本，本任务明确不做）。
5. **Wizard `both` 模式 token 一致性预警**：step2 中 frps token 与 frpc token 都非空且不相等时，给**非阻断** warning 提示（如"两端 token 不一致，frpc 将无法连接 frps"）。不强制阻断（高级用户可能有特殊需求）。token 字段位置见 Wizard.vue:71-79（frps）/ 112-120（frpc）。

## 硬约束 / 红线

- **SPA 内导航一律 `router.push`，禁 `href`/`<a>` 整页刷新**（insight L17，会丢 Pinia 状态 + 重跑路由守卫）。
- **不碰后端 / store / API / 路由守卫 / 数据流**；纯 UI 文案 + 导航 + 一处条件比较任务。
- 不引入新依赖。
- 测试断言**全用 DOM 文本 / `find('button')` 按文本 / getExposed 句柄，零 naive-ui 组件名查询**（insight L45 / T-057 教训）；message 用 `vi.mock('naive-ui')` 单例 spy 范式（若需要）。复用 `web/src/test-utils/`（getExposed / apiError）。
- **e2e 不受影响已由 PM 核实**：01-setup/02-auth/03-dashboard 烟雾测试用 bypassWizard 绕过向导、不进 Proxies/Server 编辑流（insight L29/L34）。03-dashboard 断言仅 `仪表盘`/`frpc（客户端）`/`frps（服务端）`/`退出登录`，无任何新引导文案断言。PM 已 grep e2e spec 确认无冲突（详见 PM_LOG）。
- **新增测试必须同步 bump `scripts/baseline.json`** 的 `frontend_tests` / `test_count`（+version），否则 verify_all B.4 会 FAIL。当前基线：version=27, frontend_tests=500, go_tests=322, test_count=822。
- 更新 `docs/dev-map.md` 相关行（若组件职责/导航有结构性变化）。

## 交付与验证边界

- PM 上下文**无 Bash/PowerShell**（insight L31），无法真跑 `scripts/verify_all`。verify_all 标 **PENDING**，07_DELIVERY 给"执行规格"（预期 PASS、改了哪些测试、frontend_tests 新计数）。真正全量真跑由 batch orchestrator 执行作硬闸门。
- **不要 git commit / 不要 git push / 不要跑 archive-task**——由 batch orchestrator 在 verify 通过后统一处理。

## 产出要求

- `docs/features/onboarding-next-step-guidance/` 下 7 份阶段文档 + PM_LOG.md，全中文。
- `06_TEST_REPORT.md` 必须含**裸标题** `## Adversarial tests` 段（verify_all E.6 闸门 + 红线要求），含反向证伪用例（如：both token 不一致→warning 出现；既有自动跳转/缺失分支未被新引导破坏）。
- `07_DELIVERY.md` 含**裸标题** `## Insight` 段（若有跨任务可复用的真相）。

## 相关历史任务（PM 已确认）

- **T-057** binary-missing-onboarding-ux：Wizard 完成流 missingForRole/binWarning 定格快照/全就绪自动跳转——本任务新引导**必须与之并存不破坏**。
- **T-047** frontend-honest-states：Proxies 三态（失败/空/加载）、Server/Client 加载三态——空态 #empty 模板已在，引导加在其上。
- **T-040** frps-allow-ports-policy：AllowPortsEditor 端口策略 + 单向数据流（ProxyForm.vue 文案要呼应"服务端『端口策略』允许范围"）。
- **T-058** frontend-interaction-polish：Wizard step2 frpc 标题已合并单分支；Server/Client「重新加载」+ dirty 确认。
- **T-041** server-monitor-page-ui：ServerMonitor.vue goServerConfig→push('/server')（既有反向跳转）。
- **T-042** proxy-runtime-status-merge：Proxies.vue runtime 列。
