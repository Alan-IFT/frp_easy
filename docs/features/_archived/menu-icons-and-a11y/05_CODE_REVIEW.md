# Code Review — T-064 · menu-icons-and-a11y

> Harness stage 5 · Code Reviewer · 输出中文。
> 独立审计 04_DEVELOPMENT.md 的 dev-frontend 改动，对照 01/02/03。CR 已实读全部改动文件（不盲信 dev 自述）。

## Files reviewed

- `web/src/components/AppLayout.vue`（menuOptions :130-181）
- `web/src/components/log/LogList.vue`（滚动容器 :18-34）
- `web/src/components/FirewallHint.vue`（复制按钮 :15-23, :29-31）
- `web/src/components/PublicIpDetector.vue`（复制按钮 :20-25）
- `web/src/components/__tests__/AppLayout.spec.ts`（新建，5）
- `web/src/components/log/__tests__/LogList.spec.ts`（新建，6）
- `web/src/components/__tests__/FirewallHint.spec.ts`（追加 3）
- `web/src/components/__tests__/PublicIpDetector.spec.ts`（追加 2）
- `scripts/baseline.json`（534→550 / 867→883 / v30）
- `docs/dev-map.md`（4 行补 T-064）

## Findings

### CRITICAL
无。

### MAJOR
无。

### MINOR
无。

### NIT
- [STYLE] `web/src/components/AppLayout.vue:137-138` — `menuIcon` helper 把 `role`/`aria-label`/`title` 三属性合并到一处，消除 7 项重复，可维护性优于逐项内联 `h(...)`。无需改动，记为正向观察。
- [TEST] `web/src/components/__tests__/AppLayout.spec.ts:menuIconSpans` — 以 `attributes('aria-label') !== undefined` 过滤 `span.n-icon`，意图是排除非菜单图标的 n-icon 装饰节点。CR 核实：本任务挂载态（binMissing=[] → 顶栏横幅整块跳过）下页面无其他带 `aria-label` 的 `span.n-icon`，过滤稳健。若未来顶栏新增带 aria-label 的 n-icon 装饰，该计数断言（==7）可能需收紧选择器（如限定在 sider 内）。当前不构成问题，记为未来注意点。

## Requirement coverage check

| Criterion | Implementation | Status |
|---|---|---|
| AC-1（7 项图标两两不同；server≠settings） | `AppLayout.vue:144-180` 字形 ⊙/⇌/⚙/◉/↗/≡/⚒ 全异；`AppLayout.spec.ts` "7 字形 Set.size===7" + "server≠settings 字形" | ✅ |
| AC-2（每项非空可访问名） | `AppLayout.vue:137-138` menuIcon 挂非空 aria-label；spec "7 图标各非空 aria-label" | ✅ |
| AC-3（server≠settings 可访问名且各语义匹配） | aria-label='服务端配置' vs '设置'；spec "server≠settings 名" | ✅ |
| AC-4（log-list-scroll tabindex=0） | `LogList.vue:22` `tabindex="0"`；`LogList.spec.ts` 断言 ==='0' | ✅ |
| AC-5（role∈{log,region}+非空 aria-label） | `LogList.vue:23-24` role=log + aria-label=日志输出；spec 断言 role∈{log,region}+非空 | ✅ |
| AC-6（错误/加载态无滚动容器） | 滚动容器在 `v-else`（:19），错误态 :4 / 加载态 :12 渲染 .log-empty；spec 两用例断言 `.log-list-scroll` 不存在 | ✅ |
| AC-7（FirewallHint 两处 aria-live） | `FirewallHint.vue:19`（单条）+ :30（复制全部）`aria-live="polite"`；spec 两用例断言 ==='polite' | ✅ |
| AC-8（PublicIpDetector aria-live） | `PublicIpDetector.vue:23` `aria-live="polite"`；spec 断言 ==='polite' | ✅ |
| AC-9（复制行为/文案不变） | 三组件 copyText/copyToClipboard 逻辑未动；既有 spec 用例未改 + 新增"行为保真"用例断言"复制"→"已复制 ✓"+writeText 被调 | ✅ |
| AC-10（verify_all PASS + frontend_tests 对齐） | baseline 534→550；**verify_all 真跑 PENDING**（无 Bash，交 orchestrator 硬闸门） | ⏳ PENDING |
| AC-11（e2e 不受影响） | 不改菜单 label 文案；PM/GR/CR 三核 grep 仅 03-dashboard:11 按 getByText('仪表盘') 文案不变 | ✅ |

## Design fidelity check

| Design item（02） | Implementation | Status |
|---|---|---|
| 方案 (a) 互不相同字形 + icon span 挂 aria-label+title+role=img | `AppLayout.vue:137-138` menuIcon helper | ✅ |
| settings ⚙→不同字形（消撞车） | `:180` `⚒` | ✅（字形选择在 02 约束"唯一+语义不悖+可渲染"内） |
| 不改菜单结构/路由 key/activeKey（:122-128 特判） | activeKey 计算 :122-128 未动；menuOptions key 全不变；children 结构不变 | ✅ |
| LogList 滚动容器 tabindex=0 + role=log（首选）+ aria-label | `LogList.vue:22-24` | ✅（C-1 GR 默认裁定 role=log，未降级，无 DRIFT） |
| 复制按钮 aria-live=polite（直接挂 n-button，首选透传） | `FirewallHint.vue:19,30` + `PublicIpDetector.vue:23` | ✅（C-3 首选方案，未降级外包 span） |
| 不改 copyToClipboard 逻辑（OOS-3 / T-061 util） | `web/src/utils/clipboard.ts` 未在改动清单；copyText 体未动 | ✅ |
| 零新依赖（NFR-1） | 无 package.json / lockfile 改动；纯字形+ARIA 属性 | ✅ |
| 不引颜色改动（OOS-6 / insight L16） | 4 SFC 改动无任何 rgba/color；无样式改动 | ✅ |
| 单分区 dev-frontend，全部 web/** | 改动文件全在 `web/**` + baseline/dev-map 文档；零越界后端/DB | ✅ |
| baseline bump（C-5 红线） | version 29→30 / frontend_tests 534→550 / test_count 867→883 / go_tests 333 不变 | ✅ |

## 6 维评审小结

1. **Logic correctness** — PASS。a11y 属性为静态附加，无逻辑分支变化；LogList 状态分支（错误/加载/列表）保真（AC-6 用例锁住）；aria-live=polite 首渲染不播报、仅文案变化播报符合 BC-4。测试 fixture 用真 `parseLogLine` 构造合法 `ParsedLogLine`，规避 `LogLine.levelLower` 的 `.toLowerCase()` 对 null 的崩溃（dev 已修正初版 `level:null` 隐患——CR 复核确认现版用 parseLogLine，正确）。
2. **Requirement fidelity** — PASS。AC-1~AC-9/AC-11 逐条有实现 + 断言，AC-10 待真跑。
3. **Design fidelity** — PASS。无 silent drift；C-1/C-3 均采首选方案未降级；字形选择在 02 约束内。
4. **Performance** — PASS。无热路径、无循环、无分配；menuIcon helper 每次 render 创建 1 个 vnode（与原逐项 h(...) 等价，无新增开销）。
5. **Security** — PASS。无输入/反序列化/注入面；ARIA 名为静态中文字面量；不触 v-html（LogLine 既有 escape 逻辑未动）。
6. **Maintainability** — PASS。menuIcon helper 消 7 处重复；注释解释 WHY（折叠态撞车 / role=img 防逐字朗读 / overflow 原生滚动无需 keydown）；无死代码、无过度抽象。

## Verdict

**APPROVED**（0 CRITICAL / 0 MAJOR / 0 MINOR / 2 NIT）

dev-frontend 改动忠实于设计、覆盖全部可静态核实的验收点、零越界、零新依赖、零行为漂移。唯一未决项 AC-10（verify_all 真跑）属 PM/dev role-collapsed 上下文无 Bash 的客观限制，已标 PENDING + 执行规格交 batch orchestrator 硬闸门。可进入 QA（stage 6）。
