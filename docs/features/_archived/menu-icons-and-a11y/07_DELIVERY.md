# Delivery Summary — T-064 · menu-icons-and-a11y

- **Task**: menu-icons-and-a11y — 修侧边栏菜单图标语义缺陷（齿轮 ⚙ 折叠态撞车）+ 补三处真实键盘/屏幕阅读器可访问性。
- **Mode**: full（7-stage）
- **批次**: ux-ui-uplift-2026-05 第 3 个 · 分区 dev-frontend 单分区。

## Stages traversed（含时间戳）

| Stage | Agent | 产出 | Verdict | 时间 |
|---|---|---|---|---|
| 1 | requirement-analyst | 01_REQUIREMENT_ANALYSIS.md | READY | 2026-05-31 |
| 2 | solution-architect | 02_SOLUTION_DESIGN.md | READY | 2026-05-31 |
| 3 | gate-reviewer | 03_GATE_REVIEW.md | APPROVED WITH CONDITIONS（8 维全 PASS） | 2026-05-31 |
| 4 | dev-frontend | 04_DEVELOPMENT.md | READY FOR REVIEW | 2026-05-31 |
| 5 | code-reviewer | 05_CODE_REVIEW.md | APPROVED（0 C/M/Min，2 NIT） | 2026-05-31 |
| 6 | qa-tester | 06_TEST_REPORT.md | APPROVED FOR DELIVERY（0 缺陷） | 2026-05-31 |
| 7 | pm-orchestrator | 07_DELIVERY.md | DELIVERED | 2026-05-31 |

> 派发机制：本 orchestrator 上下文 Task 工具不可用（role-collapsed），按 AI-GUIDE.md L49 "一次扮演一个角色"亲自产出各 stage 文档，严守角色边界（RA 不设计 / SA 不写码 / GR 不改上游不提修法 / CR 不写码 / QA 写独立对抗 reproducer）。

## Rollbacks

**0 次。** 无同 stage 回退。GR APPROVED WITH CONDITIONS（条件均为 dev 实测项 + 已有降级方案，非缺陷回退）；CR/QA 一次过。

## Final verify_all result

**PENDING（预期 PASS）** — PM/dev/QA role-collapsed 上下文无 Bash/PowerShell（insight L31）。全量真跑作交付硬闸门，交 **batch orchestrator Bash 会话**执行。

执行规格（交 orchestrator 逐项核对）：
- `verify_all` 预期 **PASS**；frontend_tests 534→**552**；go_tests **333（不变）**；test_count 867→**885**；Fail=0 / Warn=0。
- 特别复核：(1) FirewallHint/PublicIpDetector 既有用例零回归；(2) 新建 AppLayout.spec(5)/LogList.spec(6)/qa_t064_adversarial.spec(2) 可挂载全绿；(3) C-2 `span.n-icon[aria-label]` 透传命中；(4) C-3 `button[aria-live=polite]` 透传命中（不命中则触发 SA 预批降级：外包 `<span aria-live>`）；(5) e2e `03-dashboard:11 getByText('仪表盘')` 不受影响。

## Baseline changes

- version 29 → 30
- frontend_tests 534 → 552（+18：dev 16 + QA 独立对抗 2）
- test_count 867 → 885
- go_tests 333（不变）
- passing_count → 885

## Outstanding risks

- **C-2/C-3 透传未由真跑确认**（PM/dev 上下文无 Bash）：若 naive-ui n-menu 剥 icon span 属性（C-2）或 n-button 不透传 aria-live（C-3），对应 spec 会红 → orchestrator 真跑即捕获，触发回退到 dev（C-3 有 SA 预批降级方案）。判断依据：naive-ui 默认 `inheritAttrs:true` + 非折叠态原样渲染 icon vnode，透传概率高。
- **role=log 自动跟读噪音**（GR C-1 已裁定接受）：`role="log"` 隐含 aria-live=polite，高频日志可能播报；GR 评估 tabindex 可聚焦是主诉求、本项目日志有缓冲上限 + followTail 暂停，接受。AC-5 兼容 region 备选，未来若收到用户反馈可低成本降级。
- 无数据/安全/迁移风险（纯前端属性级 + 1 字形替换，git revert 可回滚）。

## Files changed

生产代码（4 SFC）：
- `web/src/components/AppLayout.vue`（menuIcon helper + settings ⚙→⚒ + 每 icon span aria-label/title/role=img）
- `web/src/components/log/LogList.vue`（滚动容器 tabindex=0/role=log/aria-label）
- `web/src/components/FirewallHint.vue`（两复制按钮 aria-live=polite）
- `web/src/components/PublicIpDetector.vue`（复制按钮 aria-live=polite）

测试（+18）：
- `web/src/components/__tests__/AppLayout.spec.ts`（新，5）
- `web/src/components/log/__tests__/LogList.spec.ts`（新，6）
- `web/src/components/__tests__/qa_t064_adversarial.spec.ts`（新，QA 2）
- `web/src/components/__tests__/FirewallHint.spec.ts`（+3）
- `web/src/components/__tests__/PublicIpDetector.spec.ts`（+2）

文档/基线：
- `scripts/baseline.json`（534→552 / 867→885 / v30）
- `docs/dev-map.md`（AppLayout/LogList/FirewallHint/PublicIpDetector 4 行补 T-064）
- `docs/tasks.md`（看板更新）
- `docs/features/menu-icons-and-a11y/`（01-07 + PM_LOG.md + INPUT.md）

## Next steps for user

1. batch orchestrator 在 Bash/PS 会话跑 `scripts/verify_all` 作硬闸门，核对 frontend_tests==552 / go_tests==333 / 上述特别复核 5 项。
2. 按批次约定**未 commit / 未 push / 未 archive**，由 batch orchestrator 统一处理。
3. 后续 T-066 主题色任务可承接本任务刻意留出的颜色改动（OOS-6 / insight L16）。

## Insight

- 2026-05-31 · naive-ui `n-menu` 的 icon 无障碍名应直接挂在 `icon: () => h('span', {...})` render 返回的节点上（`role="img"`+`aria-label`+`title`），**不要**依赖 `MenuOption` 对象上的属性透传——前者由 dev 完全掌控 DOM 节点、`find('span.n-icon[aria-label]')` 可稳定断言，后者透传行为版本敏感；折叠态（`:collapsed-width`）仅渲染图标时这是让屏幕阅读器 + 悬停区分同形/无文字菜单项的最小零依赖手法（`role="img"` 关键：否则裸 Unicode 字形被 AT 逐字朗读为无意义符号名）· evidence: T-064 AppLayout.vue:137-138 menuIcon helper + AppLayout.spec.ts/qa_t064_adversarial.spec.ts ⚙ gearCount===1 反向证伪
- 2026-05-31 · 给"可滚动日志/列表容器"补键盘可访问性的正确落点是**真正带 `overflow-y:auto` 的那个元素**（本项目 `.log-list-scroll` 而非外层 `.log-list-root`）：`tabindex="0"` 必须与 `role`/`aria-label` 同挂该元素，否则键盘焦点落在不可滚的包裹层、或可滚区域无 ARIA 身份。这类容器**不需**像 button 语义那样显式 `@keydown` 处理——`overflow` 元素聚焦后浏览器原生支持方向键/PageUp/Down 滚动（区别于 paused-banner 的 `role=button` 需 enter/space）。反向证伪用例应断言"外层 root 不带 tabindex"以锁死落点 · evidence: T-064 LogList.vue:21-24（tabindex+role+aria-label 同在 .log-list-scroll，CSS :133-141 overflow-y:auto）+ qa_t064_adversarial.spec.ts QA-ADV-2 root.tabindex===undefined
