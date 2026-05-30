# 03 闸门评审 — T-064 · menu-icons-and-a11y

> Harness stage 3 · Gate Reviewer · mode=**full** · 输出中文。
> 独立验证 01_REQUIREMENT_ANALYSIS.md（READY）+ 02_SOLUTION_DESIGN.md（READY）。GR 已实读引用代码，不盲信上游。

## GR 独立核实记录（读码 + grep）

- ✅ 核实 `web/src/components/AppLayout.vue:144` 与 `:169` 确实**均渲染 `⚙`** —— 缺陷属实。`:122-128` activeKey 含 `/server/monitor` 精确匹配特判，SA 标 OOS-2 不改，正确。`:76` `:collapsed-width="64"` + `:85-87` n-menu collapsed，折叠态仅图标，撞车成立。
- ✅ 核实 `web/src/components/log/LogList.vue:18-24` 滚动容器 `div.log-list-scroll` 无 tabindex/role；`:30-41` paused-banner 已有 `role="button"`/`tabindex="0"`/`@keydown.enter`/`@keydown.space.prevent` —— 范式来源属实，SA 引用准确。滚动容器在 `v-else`（:18）分支，错误态（:4）/加载态（:12）渲染 `.log-empty` 非滚动容器 —— BC-3/AC-6 成立。
- ✅ 核实 `web/src/components/FirewallHint.vue:21`（`copiedCmd === cmd ? '已复制 ✓' : '复制'`）、`:30`（`copiedAll`）与 `web/src/components/PublicIpDetector.vue:21`（`copied`）切换文案元素属实。`copyText`→`copyToClipboard`（T-061 util）逻辑 SA 标 OOS-3 不改，正确。
- ✅ 核实 `web/src/utils/clipboard.ts::copyToClipboard` 存在（T-061），三组件复用属实。
- ✅ 核实 **AppLayout.vue 与 LogList.vue 当前无独立 spec**（grep `web/src/components/__tests__/` 与 `web/src/components/log/` 无对应 spec）—— SA "new spec" 判断属实。FirewallHint.spec / PublicIpDetector.spec 既有。
- ✅ 核实 e2e（insight L34 要求）：`web/tests/e2e/03-dashboard.spec.ts:11` 断言 `getByText('仪表盘').first()`，是唯一按菜单文本的 e2e 断言；本任务不改任何 menu label 文案 → e2e 零影响（AC-11 成立）。
- ✅ insight 一致性：L16（禁硬编码白色文字色）—— 本设计零颜色改动，不触发。L45（DOM 属性断言）—— 设计验收全用 aria-label/role/tabindex/aria-live DOM 属性查询，合规。L42（1:1 行为搬运）—— a11y 属性附加不改行为，合规。

## 1. Audit checklist（8 维）

| # | 维度 | 结论 | 一句话理由 |
|---|---|---|---|
| 1 | Requirement completeness | **PASS** | 三项 in-scope（IS-1~IS-4）均可测，验收 AC-1~AC-11 可反向证伪；无歧义词。 |
| 2 | Design completeness | **PASS** | 设计覆盖每条 in-scope：字形表（IS-1）+ icon span ARIA（IS-2）+ 滚动容器 tabindex/role/aria-label（IS-3）+ button aria-live（IS-4）；含伪代码与 DOM 落点。 |
| 3 | Reuse correctness | **PASS** | 复用审计准确：paused-banner 范式、icon render 结构、copyToClipboard、复制组件测试范式均经 GR 读码确认存在；正确判定不引图标库。 |
| 4 | Risk coverage | **PASS** | 6 条风险覆盖真实风险（MenuOption 透传不确定 / role=log 噪音 / n-button 透传 / 字形豆腐块 / e2e / 无既有 spec），均有缓解 + 降级。GR 未发现遗漏的明显风险。 |
| 5 | Migration safety | **PASS** | 无数据/API/schema 变化；纯前端属性级，git revert 可回滚，向后兼容。 |
| 6 | Boundary handling | **PASS** | BC-1~BC-6 覆盖折叠/展开、有 children 项、LogList 状态分支、复制两态、多按钮共存、PublicIpDetector 无结果态；AC-6 锁住状态分支未被破坏。 |
| 7 | Test feasibility | **PASS** | 每条 AC 可由 DOM 属性查询测（aria-label/role/tabindex/aria-live + 文本切换），无不可验证项；AppLayout/LogList 新建 spec 有挂载范式参考（Risk-6 缓解）。 |
| 8 | Out-of-scope clarity | **PASS** | OOS-1~OOS-7 明确（零依赖/不改结构路由/不改复制逻辑/LogViewer 保真/不造焦点陷阱/不引颜色/不动 banner），开发不会过度构建。 |

## 2. Findings（WARN/FAIL）

无 FAIL，无 BLOCKED。以下为开发期须留意的非阻塞条件（计入 APPROVED WITH CONDITIONS）：

- **C-1（role=log 噪音权衡，GR 行使确认权）**：SA Risk-2 把 `role="log"` 自动跟读（隐含 aria-live=polite）噪音风险标给 GR 确认。**GR 裁定：采用 SA 首选 `role="log"`**。理由：(1) IS-3 主诉求是 tabindex 可聚焦（AC-4），role 为附带语义；(2) `role="log"` 语义最贴日志流，是 paused-banner 同文件团队范式自然延伸；(3) 日志区域被聚焦/进入视口时 AT 跟读是合理 a11y 行为而非"误报噪音"，且本项目日志容器有上限缓冲 + followTail 暂停机制，非无限狂刷。**若 dev-frontend 实测 happy-dom 下 role=log 与既有 followTail/scroll 测试有冲突，准予降级 `role="region"`（AC-5 已兼容），但须在 04 显式记录降级理由 + DESIGN DRIFT 标记交 CR 复核**。默认不降级。
- **C-2（n-menu icon span ARIA 透传，dev 实测）**：SA Risk-1 已规避（不依赖 option 透传，直接 render span 挂属性）。**条件**：dev-frontend 实测 `find('span.n-icon[aria-label]')` 在 n-menu 渲染后可被查到；若 naive-ui 折叠态对 icon span 有包裹/剥属性行为，dev 在 04 记录实际 DOM 落点 + 调整断言选择器（仍须满足 AC-2/AC-3 非空可访问名可被 DOM 查到）。
- **C-3（n-button aria-live 透传，dev 实测）**：SA Risk-3 已给降级方案（外包 span）。**条件**：dev-frontend 实测 `find('button').attributes('aria-live')==='polite'`；若不透传，按 SA 降级方案外包 `<span aria-live="polite">` 并在 04 记录（此为 SA 预批降级，不算 DESIGN DRIFT）。
- **C-4（字形可渲染，NFR-5）**：dev-frontend 实测所选 `settings` 替代字形（首选 `⚒`）在目标环境可渲染且与其余 6 项不重复；测试断言查 ARIA 名（语义稳定）不依赖字形像素，符合。若换用备选字形，更新字形表注释即可，不算 DRIFT。
- **C-5（baseline bump 红线）**：新增前端测试**必须**同步 bump `scripts/baseline.json` 的 `frontend_tests`/`test_count`/`version`（当前 534/867/29），否则 verify_all B.4 闸门会 FAIL（insight notes / T-044 教训）。go_tests 不变 333。

## 3. High-probability questions during development（预测开发提问 + 预答）

- **Q-A：AppLayout.spec 怎么 mount？依赖 auth/app/downloader store + router + naive-ui。**
  预答：复用项目既有挂载范式——`createTestingPinia` + vue-router mock（参照 `web/src/pages/__tests__/Wizard.spec.ts` / `Server.spec.ts` 的 router push spy + pinia 样板）+ naive-ui（按需 `n-config-provider`/`n-message-provider` wrap，参照 MEMORY 项目记忆"NMessageProvider 必须在 App.vue"——测试中若 AppLayout 渲染 useMessage 需提供 provider 或 mock）。最小可测路径：mount 后查 `span.n-icon` 的 aria-label/title 集合，断言 7 项非空 + 两两不同 + server≠settings。
- **Q-B：menuOptions 是 module 内 const 非导出，怎么断言图标？**
  预答：不需导出——mount 整个 AppLayout 后查渲染 DOM 的 `span.n-icon` 节点属性即可（DOM 属性查询，insight L45）。若整体挂载成本高，dev 可评估把 menuOptions 导出供单测（但属结构微调，需保持 OOS-2 不改菜单结构语义——导出常量不改结构，可接受）。
- **Q-C：LogList 列表分支需要哪些 props 才能渲染滚动容器？**
  预答：滚动容器在 `v-else`（非 firstLoadError、非 loading）分支。最小 props：`loading:false, firstLoadError:null` + 其余 props（visibleLines/bufferEmpty/noMatchHint/wrap/heightPx/fontSizePx/followTail/paused）给合法值。断言 `find('.log-list-scroll').attributes()` 的 tabindex/role/aria-label。错误/加载态用例断言 `find('.log-list-scroll').exists()===false`（AC-6）。
- **Q-D：FirewallHint/PublicIpDetector 既有 spec 改动会回归吗？**
  预答：仅追加 a11y 属性断言，不改既有用例。既有复制行为/文案断言不动（OOS-3 / AC-9）。新增断言查 `find('button').attributes('aria-live')`。复用既有 clipboard mock + message 单例 spy 范式（T-058/T-061）。
- **Q-E：aria-live=polite 首次渲染会不会误播报？**
  预答：不会。`aria-live=polite` 语义是仅在区域内容**后续变化**时播报，首次渲染建立基线不播报（BC-4）。无需测播报本身（jsdom/happy-dom 不模拟 AT），只测属性存在即可。

## 4. Verdict（结论）

**APPROVED WITH CONDITIONS**

设计与需求 8 维全 PASS，无 FAIL、无 BLOCKED。可进入开发。开发期须满足条件 C-1（默认 role=log，降级须记 DRIFT）、C-2/C-3（两处透传 dev 实测，有 SA 预批降级）、C-4（字形可渲染 + 唯一）、C-5（baseline bump 红线）。所有条件均为实测确认 + 已有降级方案，不构成路由回退。dev-frontend 可开工。
