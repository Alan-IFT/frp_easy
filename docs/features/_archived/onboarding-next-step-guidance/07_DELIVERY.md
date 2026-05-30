# 交付摘要 — T-062 · onboarding-next-step-guidance

- 任务：onboarding-next-step-guidance —— 补正向 onboarding 引导 + 跨页连通 + Wizard both token 不一致预警，消除"配好了不知道下一步"断点。
- 模式：full（7-stage）。分区：dev-frontend（单分区）。批次：ux-ui-uplift-2026-05（第 1 个任务）。
- 阶段历程（时间戳）：
  - Stage 1 Requirement Analyst — 2026-05-30 — READY（7 IS / 13 AC / 10 BC / 7 OOS，无开放问题）
  - Stage 2 Solution Architect — 2026-05-30 — READY（逐文件设计 + 零新依赖 + Partition assignment 单分区）
  - Stage 3 Gate Reviewer — 2026-05-30 — APPROVED WITH CONDITIONS（8 维度全 PASS，无 FAIL；C-1~C-6）
  - Stage 4 Developer/dev-frontend — 2026-05-30→05-31 — READY FOR REVIEW（5 源 + 5 spec + baseline + dev-map，无 DESIGN DRIFT）
  - Stage 5 Code Reviewer — 2026-05-31 — APPROVED（13/13 AC ✅，9 项设计保真 ✅，0 CRITICAL/0 MAJOR）
  - Stage 6 QA Tester — 2026-05-31 — APPROVED FOR DELIVERY（含裸 ## Adversarial tests，QA 独立 2 反向证伪存活，0 缺陷）
  - Stage 7 Delivery — 2026-05-31 — 本文档
- 回退次数：**0**。无任何 stage 回退（GR 评估 R-1 张力后判定不构成需求层缺陷，转开发条件 C-1，未回退 RA）。
- 最终 verify_all 结果：**PENDING（预期 PASS）** —— PM/dev/QA 上下文无 Bash/PowerShell（insight L31），无法真跑。静态 + 确定性反向证伪预测全绿。**全量真跑（含 e2e）由 batch orchestrator Bash 会话执行作硬闸门。**
- 基线变化：frontend_tests 500→**534**（+34）；test_count 822→**856**；go_tests 322 不变；version 27→**28**；passing_count 856。
  - 新增分布：Wizard.spec +14（IS-7 token 预警 5 + IS-1 加规则引导 5 + Adversarial 4：dev 2 + QA 独立 2）、Client.spec +4、Proxies.spec +8（IS-3 5 + IS-4 2 + Adversarial 1）、Server.spec +5、ProxyForm.spec +3。
- 改动文件（git diff stat — 12 个，全 web/** + 文档惯例）：
  - `web/src/pages/Wizard.vue`（IS-1 step3 加规则引导 + IS-7 step2 token 不一致 warning；+computed import）
  - `web/src/pages/Client.vue`（IS-2 保存成功引导；+useRouter +NAlert）
  - `web/src/pages/Proxies.vue`（IS-3 保存成功双入口 + IS-4 空态连通入口；+useRouter +NAlert）
  - `web/src/pages/Server.vue`（IS-5 查看运行态链接；+useRouter，N* 零新）
  - `web/src/components/ProxyForm.vue`（IS-6 远程端口端口策略纯文案；+NText）
  - `web/src/pages/__tests__/{Wizard,Client,Proxies,Server}.spec.ts`（+31）
  - `web/src/components/__tests__/ProxyForm.spec.ts`（+3）
  - `scripts/baseline.json`（version/计数 bump + notes）
  - `docs/dev-map.md`（5 行职责说明）
- 未碰：后端 `internal/**` `cmd/**`、`internal/storage/**`、`migrations/**`、`web/src/router.ts`、路由守卫、store、API 层、useProxyForm 校验、useServerRuntime 数据流。零新依赖。

## 关键改动（载荷性细节）

- **Wizard IS-1 与 T-057 并存策略**：引导按钮在 `<template v-if="binWarning.length === 0">`（就绪分支）内 + `v-if="selectedRole === 'frpc' || selectedRole === 'both'"`，缺失分支 `v-else` 不加。既有全就绪自动 `router.push('/dashboard')` + success toast 与缺失分支 warning + 「进入仪表盘」按钮均未改（AC-11 护栏 + 既有 30+ T-057 用例零回归）。
- **Wizard IS-7 非阻断保证**：`tokenMismatch` 仅用于 template `v-if`，`handleNext` 完全未引用——不阻止推进、不写 configError（QA 反向证伪锁死：不一致仍推到 step3、PUT 照常）。BC-4 用 `.trim()` 防纯空白误报。
- **导航全 router.push**（insight L17）：5 项导航（goToProxies / goToDashboard / goToMonitor ×2 / Server goToMonitor）均 `void router.push(路径字符串)`，无 href/tag=a。
- **Server↔Monitor 双向连通**：Server.vue loaded 态加 push('/server/monitor')，补齐 ServerMonitor.vue:218-220 既有 goServerConfig→push('/server') 的反方向。

## 待办风险

- **OOS-1**（ProxyForm 远程端口与服务端 allowPorts 跨页实时校验联动）本任务明确不做（纯文案）。若未来需要"输入越界端口时实时高亮提示"，是独立中-高成本任务候选（需读 server store / allowPorts 数据 + 校验联动）。
- **IS-1 全就绪态 UX 局限（已知，接受）**：就绪分支引导按钮会被同 tick 自动 push('/dashboard') 抢先，真实用户极难点到。高 UX 价值在 Client.vue（IS-2，无自动跳转）。GR C-1 已定调"附加不阻断、验收点为存在+可点击"。若未来想让向导完成后停在加规则引导（不自动跳），需重审 T-057 自动跳转节奏——超出本任务范围。

## 下一步（给用户 / orchestrator）

1. batch orchestrator 在 Bash 会话真跑 `scripts/verify_all`（含 e2e），预期 PASS（前端 534 passed / 0 failed，B.4 计数闸门、E.6 标题闸门绿）。**特别复核**：Wizard T-057 既有用例零回归 + frontend_tests==534。
2. verify 通过后由 orchestrator 统一 commit / archive（本任务按要求未 commit / 未 archive / 未跑 archive-task）。

## Insight

- 2026-05-31 · 向"既有自动跳转分支"追加正向引导按钮（如 Wizard 全就绪态自动 push('/dashboard') 旁加"去加规则→push('/proxies')"）时，引导的真实可达性受自动跳转抢先压制——同 tick 渲染+跳转让用户几乎点不到该按钮；高 UX 价值的引导锚点应选**无自动跳转的成功态**（如表单保存成功后停留页内，本任务 Client.vue IS-2），而非"完成即自动离开"的过渡态。判别法：放引导前先问"这个状态用户会停留多久"——会立即被导航走的状态放引导=语义正确但实效低，测试只能验"存在+可点击"而非"用户真会用"。设计如实记录此张力优于假装高价值 · evidence: T-062 Wizard.vue:147-161 就绪分支引导 vs Client.vue IS-2 + 03 GR F-1/C-1 + 06 §Adversarial QA 分支互斥用例
- 2026-05-31 · 给"原本不 import useRouter 的页面组件"加 SPA 内导航时，其既有 *.spec.ts 需新增模块级 `vi.mock('vue-router', () => ({ useRouter: () => ({ push: pushSpy }) }))`——此 mock 对既有用例零回归的前提是**既有用例不依赖 router**（多数三态/CRUD 组件如此）；范本是同项目 Wizard.spec:58-59（已与 getExposed + naive-ui importOriginal mock 共存验证）。新增后须在 beforeEach 加 `pushSpy.mockReset()`。导航断言用 `pushSpy.mock.calls.filter(c => c[0] === 路径).length` 可精确验"某路径恰调 N 次"，比 toHaveBeenCalledWith 更能证伪"两路径串扰/互相抑制" · evidence: T-062 Client/Proxies/Server.spec 新增 vue-router mock + Wizard.spec QA 独立用例 push 两路径独立计数
