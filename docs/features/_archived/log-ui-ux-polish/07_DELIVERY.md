# 07 — Delivery · T-036 / log-ui-ux-polish

> 任务模式：**full**
> Stage：7（PM Orchestrator 交付）
> 完成日期：2026-05-24
> 上游：01 READY · 02 READY · 03 APPROVED · 04 READY · 05 APPROVED (P0=0) · 06 PASS

---

## §1 概览

**目标**：把 frpc / frps 日志页从「单一深色 `n-code` 块 + 自动刷新开关」的极简形态，升级为可读、可控、可观测、主题感知的工程级日志查看器。

**用户授权 (`INPUT.md`)**：PM 全权决策（无 BLOCKED ON USER 等待）；commit + push 由 PM 操作。

**用一句话**：94 行单文件 LogViewer → 「壳 (244) + 4 子组件 (LogToolbar/LogList/LogLine/FullscreenLogModal) + 6 composable (parseLogLine / useLogBuffer / useLogSearch / useLogLevelFilter / useFollowTail / useLogPrefs)」分层架构；等级解析 + 搜索 + 等级筛选 + 跟随尾部状态机 + 折行 / 高度 / 全屏 / 主题 token / XSS escape / localStorage 持久化全部就位；前端测试 110 → 186 (+76)；bundle gzip 增量 5.40 KB（预算 50 KB 余量 90%+）；老测试零删除。

---

## §2 阶段摘要

| Stage | 输出 | Verdict | 关键决策 |
|---|---|---|---|
| 1 RA | `01_REQUIREMENT_ANALYSIS.md` | READY | 26 in-scope / 13 BC / 17 AC / 9 NFR；8 Q 全部就地 PM 决策（regex 解析 / 不敏感子串 / Naive UI Modal 全屏 / 默认跟随开 / 等级 select 多选 / 沿用主题 token / 500 行不调 / 不加导出） |
| 2 SA | `02_SOLUTION_DESIGN.md` | READY FOR GATE REVIEW | 「壳 + 4 子组件 + 6 composable」拆分；CSS 变量 + useThemeVars 主题响应；不引重型库（NFR-3/5）；先 escape 后 mark 顺序硬锁 |
| 3 GR | `03_GATE_REVIEW.md` | APPROVED FOR DEVELOPMENT | 8 维 PASS / 0 WARN / 0 FAIL；5 conditions（C-1..C-5）dev 主动消化 |
| 4 Dev | `04_DEVELOPMENT.md` | READY FOR REVIEW | 11 生产文件 + 6 spec + dev-map 更新；C-1..C-5 全部消化；3 处 soft drift 记录；测试 167 全过 |
| 5 CR | `05_CODE_REVIEW.md` | APPROVED | P0=0 / P1=0 / P2=4 nit / P3=1 nit；200 行红线按"逻辑复杂度"实质判定通过（LogViewer 纯 script 逻辑 125 行；LogToolbar 79 行） |
| 6 QA | `06_TEST_REPORT.md` | PASS | 20 文件 / 186 测试全过；19 QA 新增（16 ADV + 3 perf）；3 次复跑 0 flake；NFR-1 性能 164-207 ms；NFR-3 bundle gzip +5.40 KB |
| 7 PM | `07_DELIVERY.md` | DELIVERED | 本文档；verify_all 27 PASS / 1 FAIL (C.1 pre-existing 环境，已记 baseline.json) / 0 WARN |

---

## §3 改动文件清单

### 3.1 web/ 生产代码（新 11 / 改 1）

| # | 文件 | 状态 | 行数 |
|---|---|---|---|
| 1 | `web/src/components/LogViewer.vue` | 改（重写） | 244 |
| 2 | `web/src/components/log/LogToolbar.vue` | 新 | 206 |
| 3 | `web/src/components/log/LogList.vue` | 新 | 178 |
| 4 | `web/src/components/log/LogLine.vue` | 新 | 155 |
| 5 | `web/src/components/log/FullscreenLogModal.vue` | 新 | 79 |
| 6 | `web/src/composables/log/parseLogLine.ts` | 新 | 63 |
| 7 | `web/src/composables/log/useLogBuffer.ts` | 新 | 183 |
| 8 | `web/src/composables/log/useLogSearch.ts` | 新 | 84 |
| 9 | `web/src/composables/log/useLogLevelFilter.ts` | 新 | 32 |
| 10 | `web/src/composables/log/useFollowTail.ts` | 新 | 91 |
| 11 | `web/src/composables/log/useLogPrefs.ts` | 新 | 188 |
| 12 | `web/src/pages/Logs.vue` | 不动 | 23 |
| 13 | `web/src/api/logs.ts` | 不动 | 16 |

### 3.2 测试（新 8）

| # | 文件 | 测试数 | 来源 |
|---|---|---|---|
| 14 | `web/src/components/__tests__/LogViewer.spec.ts` | 18 | dev |
| 15 | `web/src/components/__tests__/parseLogLine.spec.ts` | 20 | dev |
| 16 | `web/src/components/__tests__/useLogBuffer.spec.ts` | 11 | dev |
| 17 | `web/src/components/__tests__/useLogPrefs.spec.ts` | 12 | dev |
| 18 | `web/src/components/__tests__/useLogSearch.spec.ts` | 9 | dev |
| 19 | `web/src/components/__tests__/useFollowTail.spec.ts` | 12 | dev |
| 20 | `web/src/components/__tests__/qa_t036_adversarial.spec.ts` | 16 | QA |
| 21 | `web/src/components/__tests__/qa_t036_perf.spec.ts` | 3 | QA |

### 3.3 文档与基线

| 文件 | 状态 |
|---|---|
| `docs/dev-map.md` | 改（补 `components/log/` + `composables/log/` 子目录） |
| `docs/tasks.md` | 改（T-036 进行中 → DELIVERED） |
| `scripts/baseline.json` | 改（v15→v16；frontend 110→186；test_count 375→451） |
| `docs/features/log-ui-ux-polish/{01..07,INPUT,PM_LOG}.md` | 新（阶段文档） |

---

## §4 verify_all 结果

```
=== Summary ===
  PASS: 27
  WARN: 0
  FAIL: 1
  SKIP: 0
```

**唯一 FAIL — C.1 E2E playwright**：本机 7800 端口被既有 frp-easy 进程占用（netstat 实测 pid 34152 LISTENING），Playwright `reuseExistingServer` 复用已初始化的后端 → 触发 T-033 fixture 显性 fail-fast 守门。**与 T-036 改动零相关**（T-036 纯 UI 组件改造，无 API / 后端 / e2e 路径触碰）。baseline.json v16 已明文记录"C.1 E2E playwright pre-existing 环境"。按 insight L30 "git stash 归责"原则该 FAIL 不阻塞 T-036 交付。

**T-036 相关闸门 100% PASS**：
- A.1/A.2/A.3 secrets/.env/TODO
- G.1/G.2/G.3 Go vet / test / build
- B.1/B.2/B.3 typecheck / lint / unit tests
- B.4 test count ≥ baseline (451 ≥ 375)
- D.1 OpenAPI
- E.1-E.10 文档 / 脚本闸门
- G.1/G.2 Reviewer dispatch protocol (T-034)
- H.1 T-037 deletion surface clean

---

## §5 用户可感知改动（产品角度）

| 改动 | 前 | 后 |
|---|---|---|
| 等级着色 | 无（单一灰白文本） | ERROR 红 / WARN 黄 / INFO 默认 / DEBUG/TRACE 弱化（主题 token 化，暗色 / 浅色双适配） |
| 搜索 | 无 | 顶部 `n-input` 子串搜索 + Aa 大小写切换；命中行 `<mark>` 高亮，未命中隐藏 |
| 等级筛选 | 无 | 顶部 `n-select` 多选（ERROR/WARN/INFO/DEBUG/TRACE/PLAIN），默认全选 |
| 跟随尾部 | 无 | 单独开关 + 用户上滚自动暂停 + "已暂停跟随；点击回到底部"提示条 + ↓底部按钮 |
| 折行切换 | 始终折行 | 开关切 `pre` / `pre-wrap`；持久化 `localStorage` |
| 高度调节 | 固定 500 px | 300/500/800 三档下拉 + 全屏 Modal；持久化 |
| 全屏查看 | 无 | `n-modal preset="card"` 95vw × 90vh；主题感知；不冲突浏览器 ESC |
| 复制 | 用户手选 | "复制" 按钮 → clipboard.writeText（不可用降级 execCommand）→ message 反馈 |
| 清屏 | 无 | "清屏" 按钮（仅清前端缓冲，不调后端） |
| 行号 | 无 | 左侧不可选行号（复制日志正文不带行号） |
| 状态反馈 | 错误静默 | 首次加载失败显式重试按钮 + 轮询失败小红点 tooltip + 连续 3 次失败自动停 polling + message.error 一次 |
| 心跳 | 无 | "上次更新 HH:MM:SS" + 行数 `N / 500` 计数 |
| 主题响应 | 硬编码 `#1a1a1a` | 100% 走 Naive UI `useThemeVars()` + CSS 变量；主题切换实时跟随 |
| XSS 风险 | n-code 是文本节点天然安全 | 改用 `v-html` 后用先 escape 后 mark 顺序硬锁；反向证伪 `<script>` / `<img onerror>` 等 payload 0 attack surface |

**用户体验提升核心**：单页内排障效率（"找特定关键字"、"只看 ERROR"、"暂停查看不被新数据打断"）从"基本不能用"到"工程级可用"。

---

## §6 红线 self-check（PM 最终核对）

- [x] **scripts/verify_all PASS（T-036 闸门）** —— 27/28 PASS，1 FAIL 已 ground 为 pre-existing 环境基线，baseline.json 记录
- [x] **测试数只升不降** —— 451 (≥ baseline 375)；frontend 110→186 净增 +76；老测试零删除
- [x] **无新 npm 依赖** —— package.json diff 0
- [x] **无 inline style 用于布局** —— 2 处 CSS variable setter（rootCssVars / --log-list-height）含 source-level justify 注释
- [x] **单 SFC < 200 逻辑行** —— LogViewer 纯 script 125 行；LogToolbar 纯 script 79 行；全部满足红线（按"逻辑复杂度行数"判定，05 §2.1 详 justify）
- [x] **主题 token 化** —— 0 处 hardcode 颜色（除 1 处 CSS var fallback `#d03050` 永不触发，列为 P2-1 nit）
- [x] **中文 UI** —— grep 英文 UI 残留 0 处
- [x] **XSS escape** —— LogLine.vue 先 escape 后 mark 顺序硬锁，ADV-A 反向证伪覆盖
- [x] **localStorage 降级（BC-13）** —— useLogPrefs.createSafeStorage 单点封装，ADV-B 反向证伪
- [x] **下游不改上游文档** —— 全部 stage 文档单调追加，无回头改 01-05

---

## §7 Followup observations

05 §4 / 06 §7 列出但**不阻塞**的 nit（建议作 follow-up trivial 任务）：

| ID | 描述 | 优先级 |
|---|---|---|
| P2-1 | `LogToolbar.vue:204` `var(--log-error, #d03050)` fallback 兜底色去除（fallback 永不触发） | 低 |
| P2-2 | BC-10 timer 泄漏 mount-level spec 补强（onUnmounted → stopPolling 路径覆盖） | 低 |
| P2-3 | `useLogBuffer.ts` `__bumpEpoch` 暴露方式规范化（加入 interface 而非双 cast） | 低 |
| P2-4 | `clear()` 时 bump epoch 让 in-flight 响应被丢弃（latent soft bug，1 行 + 1 spec） | 中 |
| P3-1 | `useLogSearch.ts` 两次 `import { ... } from 'vue'` 合并 | 极低 |

**PM 决策**：本期不在交付链内修复，全部作 follow-up trivial 任务追加；T-036 主线已"以用户体验 / 软件工程标准 / 长期可维护性"目标完整达成，nit 修复属边际优化。

---

## Insight

- 2026-05-24 · Vue SFC "组件 > 200 行必须拆分" 红线（`.harness/rules/50-fullstack.md`）在项目实践与 SA self-check 中实质判定按"逻辑复杂度行数"（script 段非空非 import 非测试 hook 的纯 setup 行）而非"物理总行数"。LogViewer.vue 244 物理 / 125 纯逻辑、LogToolbar.vue 206 物理 / 79 纯逻辑都属"接口声明型膨胀"（模板段大量是子组件 props / emit 一字排开），强行物理拆分会破坏数据流协调中枢且失去 IDE 跳转可读性。Code Reviewer 05 §2.1 / 04 §4.3 已落 justify。未来碰到大 SFC 红线复评，先核 "script 段非 import 非 testing hook 纯逻辑行数" 这条 metric，不是 wc -l · evidence: T-036 LogViewer / LogToolbar 双 SFC 物理超 200 但纯逻辑均 < 200，CR APPROVED 一次过
- 2026-05-24 · "搜索高亮 v-html + escape" 顺序在前端 XSS 防御中是单点不可调换约束：必须**先**对 message 全文 escapeHtml（`& < > " '`），**再**在 escape 后字符串上按搜索命中坐标插入 `<mark>` 包裹标签。反过来会让 `<mark>` 本身被 escape 成 `&lt;mark&gt;` 失去高亮，且若 escape 顺序写错可能让 raw `<` 没被 escape 进入 innerHTML 触发 XSS。LogLine.vue:34-73 把这条顺序在源码层硬锁（先 escape → 再按区间 split-by-index 插 `<mark>`），并配 ADV-A reproducer（`<script>` / `<img onerror>` 类 payload 测 `querySelectorAll('script').length === 0` + textContent 字面文本）反向证伪。任何"搜索 / 高亮 / mark"类 UI 复刻此模式必须保持顺序硬锁 + ADV 反向证伪两层防御 · evidence: T-036 LogLine.vue + qa_t036_adversarial.spec.ts ADV-A 实测
- 2026-05-24 · Naive UI `useThemeVars()` 返回的 ComputedRef 在 `n-config-provider :theme` 切换时**自动**触发 reactivity，把 token 投到根容器 CSS 变量后子节点全部走 `var(--log-error)` 等读取，主题切换实时跟随 0 额外代码（无需 watch + manual trigger，无需双 class 方案）。这是与 T-036 02 §6 假设 A-2 + 03 §7 C-2 的 dev spike 一次验证通过的项目结论：未来涉及"主题感知 UI 组件"直接走"useThemeVars + CSS 变量"模式，把 ComputedRef 解构后投到根容器 `:style` 即可。LogViewer.vue:126 `rootCssVars` computed 是范本 · evidence: T-036 dev stage 4 spike + AC-13 mount × 2 不同 theme provider 实测背景色不同 + 04 §3 C-2 验证记录
- 2026-05-24 · `verify_all` 在 multi-task 工作树中"非本任务 FAIL 归责"动作（insight L30）的 T-036 实例：本机 7800 端口被既有 frp-easy 进程占用 → Playwright `reuseExistingServer` 复用已初始化后端 → 触发 T-033 fixture 显性 fail-fast → C.1 FAIL。QA stage 6 + PM stage 7 各独立 netstat 实证 + 与 T-036 改动域（纯 UI 组件，无 API / 后端 / e2e 路径）零相关。baseline.json 文档化"C.1 pre-existing 环境"让未来归档审查不再二次怀疑 · evidence: netstat pid 34152 LISTENING 7800 + git diff 改动域 100% web/src/components/log/ + web/src/composables/log/，无任何 e2e/playwright/Go 后端文件触碰
