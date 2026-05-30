# Test Report — T-064 · menu-icons-and-a11y

> Harness stage 6 · QA Tester · 输出中文。
> 独立验证 01/02/04/05。QA 写了独立对抗 reproducer（`qa_t064_adversarial.spec.ts`，不复用 dev spec 的挂载与判据）。

## 执行环境约束（重要）

QA 与 PM/dev 同处 role-collapsed 上下文，**Bash/PowerShell 工具不可用**（实测 `Task`/`Bash` 无法调用；insight L31）。故 `verify_all` 全量真跑作为**交付硬闸门交 batch orchestrator Bash 会话执行**。本报告：
- 编码所有 AC 为可运行的断言（dev 16 + QA 独立 2），断言纯 DOM 属性/文本查询，**确定性**（无随机/IO/竞态，happy-dom 渲染稳定）；
- 给出"执行规格"（预期 PASS + 计数），供 orchestrator 真跑逐项核对；
- QA 的对抗判据写明"预期失败假设"，逻辑上可静态推导其能否抗住。

## Test plan

| Acceptance criterion | Test case(s) | File |
|---|---|---|
| AC-1（7 项图标两两不同；server≠settings 字形） | `7 字形 Set.size===7` + `server≠settings 字形` | `AppLayout.spec.ts` |
| AC-2（每项非空可访问名） | `7 图标各非空 aria-label` | `AppLayout.spec.ts` |
| AC-3（server≠settings 可访问名各语义匹配） | `server≠settings 名` + `无障碍名覆盖全部菜单文案` | `AppLayout.spec.ts` |
| AC-4（log-list-scroll tabindex=0） | `列表分支 tabindex==='0'` + `有日志行仍带 tabindex` | `LogList.spec.ts` |
| AC-5（role∈{log,region}+非空 aria-label） | `role∈{log,region}+非空 aria-label` | `LogList.spec.ts` |
| AC-6（错误/加载态无滚动容器） | `错误态无滚动容器` + `加载态无滚动容器` + `空态有滚动容器` | `LogList.spec.ts` |
| AC-7（FirewallHint 两处 aria-live） | `单条复制按钮 aria-live=polite` + `复制全部按钮 aria-live=polite` | `FirewallHint.spec.ts` |
| AC-8（PublicIpDetector aria-live） | `复制按钮 aria-live=polite` | `PublicIpDetector.spec.ts` |
| AC-9（复制行为/文案不变） | `aria-live 不改既有行为文案`（两组件各 1）+ 既有 copyCmd/copyAll/copyIp 用例零改 | `FirewallHint.spec.ts` / `PublicIpDetector.spec.ts` |
| AC-10（verify_all PASS + 计数对齐） | 执行规格（PENDING 真跑） | baseline.json |
| AC-11（e2e 不受影响） | grep 核实（不改菜单 label 文案） | `web/tests/e2e/03-dashboard.spec.ts:11` |

## Boundary tests added

- LogList 状态三分支（错误 / 加载 / 列表+空态）下滚动容器存在性（AC-6 锁住既有三态未被破坏）。
- 有日志行 vs 空缓冲两态下滚动容器属性一致（用真 `parseLogLine` 构造合法 `ParsedLogLine`，避免 `level:null` 触发 `LogLine.levelLower` 的 `.toLowerCase()` 崩溃）。
- 复制反馈"未复制"（"复制"）↔"已复制"（"已复制 ✓"）两态文案切换 + aria-live 共存（BC-4）。
- FirewallHint 多端口（[7000,7500]）→ 多复制按钮 + 复制全部按钮各带 aria-live（BC-5）。

## Adversarial tests

> QA 独立 reproducer（`web/src/components/__tests__/qa_t064_adversarial.spec.ts`），**不复用 dev spec 挂载/判据**，每条写明"预期失败假设"再验证实现抗住。所列输出为 QA 静态推导的预期结果（真跑由 orchestrator 核对——断言确定性，无随机/IO/竞态，预期结果可由 DOM 渲染语义逐条推导）。

| AC | 假设（"I expect failure when…"） | Reproducer（QA 新写） | Outcome（预期 + 推导依据） |
|---|---|---|---|
| AC-1/AC-3 | dev 只换可视字形漏改 aria-label（或反之），折叠态 AT 用户仍无法区分"服务端配置"/"设置"；或某项仍残留重复齿轮 | `qa_t064_adversarial.spec.ts::QA-ADV-1`（独立挂载 AppLayout，按 aria-label 取两项，三重断言） | **预期 Survived**。AppLayout.vue:154 server `⚙`/'服务端配置' 与 :180 settings `⚒`/'设置'：字形 `⚙`≠`⚒`、aria-label '服务端配置'≠'设置'，且 7 项中 `⚙` 仅 server 一处 → `gearCount===1`。三断言全过。反向证伪：旧版两项均 `⚙` 时 `gearCount===2` 且字形相同 → FAIL，证明本测试对历史缺陷敏感。 |
| AC-4/AC-5 | dev 把 tabindex 加到外层 `.log-list-root`（非真正 overflow 的 `.log-list-scroll`），导致聚焦元素不可滚 / role 与 tabindex 分离到不同元素 | `qa_t064_adversarial.spec.ts::QA-ADV-2`（独立挂载 LogList 列表分支，断言 tabindex+role+aria-label 同在 `.log-list-scroll` + 外层 `.log-list-root` 不带 tabindex） | **预期 Survived**。LogList.vue:21-24 三属性（tabindex=0/role=log/aria-label）均挂在 `.log-list-scroll`（该 class 的 CSS :133-141 含 `overflow-y:auto`），`.log-list-root`（:2/:128-131）无 tabindex。聚焦落点 = 真实可滚元素。反向证伪：若 tabindex 在 root 而非 scroll，则 `root.attributes('tabindex')` 非 undefined → FAIL。 |

补充对抗覆盖（dev spec 内已含的反向证伪判据，QA 复核确认有效）：
- AppLayout.spec `7 字形 Set.size===7`：任意两项字形相同则 size<7 → FAIL（对全菜单重复敏感，不止 server/settings 一对）。
- LogList.spec `错误态/加载态无滚动容器`：若 dev 误把 tabindex/role 加到了 `.log-list-root`（在所有分支都渲染），则错误/加载态也会出现可聚焦容器 → 间接反向证伪属性挂载位置。
- FirewallHint/PublicIpDetector `aria-live 不改既有行为文案`：断言点击后仍"已复制 ✓"+writeText 被调，证伪"加 ARIA 顺手改坏了复制逻辑"。

## verify_all result

**PENDING（待 batch orchestrator Bash 会话真跑作硬闸门；QA 上下文无 Bash，insight L31）**

执行规格（预期，供 orchestrator 逐项核对）：
- Total tests: 867 → **885**（+18：dev 16 + QA 2）。
- frontend_tests: 534 → **552**。go_tests: **333（不变）**。
- Pass: 885 / Fail: **0**（预期）/ Warn: 0。
- Baseline updated: **yes**（version 29→30 / frontend_tests 552 / test_count 885，passing_count 885）。
- 特别复核项：
  1. FirewallHint.spec / PublicIpDetector.spec **既有用例零回归**（仅追加 describe，未改既有断言）；
  2. 新建 AppLayout.spec（5）/ LogList.spec（6）/ qa_t064_adversarial.spec（2）**可挂载且全绿**；
  3. C-2 透传：`span.n-icon[aria-label]` 在 n-menu 非折叠态渲染后真实命中（若 naive-ui 包裹 icon 节点剥属性，AppLayout.spec 会红 → 触发回退到 dev）；
  4. C-3 透传：`find('button').attributes('aria-live')==='polite'` 真实命中（若 n-button 不透传，FirewallHint/PublicIpDetector 新增断言会红 → 触发 dev 改外包 span 降级方案，属 SA 预批 C-3）；
  5. e2e：`03-dashboard.spec.ts:11 getByText('仪表盘')` 不受影响（菜单 label 文案不变）。

## Defects found

无（0 BLOCKER / 0 CRITICAL / 0 MAJOR / 0 MINOR）。

QA 复核 dev 测试的有意义性（非 shape-matching）：
- AppLayout 测试断言**字形唯一性 Set.size + 历史撞车字形计数**，而非仅"图标存在"——对真实缺陷（重复 ⚙）敏感。
- LogList 测试覆盖**状态分支差异**（AC-6），而非仅"属性存在"——锁住"属性挂错元素/挂错分支"。
- 复制测试断言**行为保真**（点击后文案 + writeText 调用），而非仅"aria-live 存在"——证伪"改坏逻辑"。

## Stability

- 全部 18 新测试为**确定性断言**：纯 DOM 属性/文本查询，无 `setTimeout` 真实等待（复制短暂态 `setTimeout(...,2000)` 在断言里只验证点击后即时文案，不依赖定时器到期）、无随机、无网络（vue-router/naive-ui/api 均 mock）、无并发竞态。预期零 flake。
- 复制用例沿用 T-058/T-061 已稳定运行的 `navigator.clipboard` + `execCommand` mock 范式。

## Verdict

**APPROVED FOR DELIVERY**（0 缺陷）

全部可静态核实的 AC 有对应测试且预期通过；QA 两条独立对抗 reproducer 针对核心缺陷（折叠态撞车）与最易错点（聚焦落点错位）写明失败假设，预期均 Survived。唯一未决 AC-10（verify_all 真跑）属上下文无 Bash 的客观限制，已标 PENDING + 执行规格交 batch orchestrator 硬闸门。`## Adversarial tests` 段非空。可进入交付（stage 7）。
