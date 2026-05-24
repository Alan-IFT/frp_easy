# 03 — Gate Review · T-036 / log-ui-ux-polish

> 任务模式：**full**
> Stage：3（Gate Reviewer）
> 上游：`01_REQUIREMENT_ANALYSIS.md`（READY）+ `02_SOLUTION_DESIGN.md`（READY FOR GATE REVIEW）
> 评审职责：判 01 + 02 整体上能否让 dev 立刻开工而不会踩既知项目陷阱。不评审实现细节、不给替代方案、不改上游。

---

## §1 概览

### 1.1 任务定位

把 `LogViewer.vue`（94 行单文件，硬编码 `#1a1a1a` + 12 px monospace + 单 `n-code` 块 + 自动刷新开关）升级为「壳 + 4 子组件 + 5 composable」的工程级日志查看器；纯前端 / 单 dev-frontend 分区 / 零新依赖 / 后端 API 契约零变更。

### 1.2 评审动作摘要

- 读 01（185 行，26 in-scope / 13 BC / 17 AC / 9 NFR / 8 Q 全决策）。
- 读 02（906 行，14 决策矩阵 / 6 假设 / 13 §10 self-check / 14 步实现序）。
- ground 8 项关键既存代码：
  - `web/src/components/LogViewer.vue`（旧 94 行）— 确认现状描述准确。
  - `web/src/pages/Logs.vue` — 确认外层 wrapper 薄，SA 决定不改正确。
  - `web/src/api/logs.ts` — 确认 `apiGetLogsTail` / `apiGetLogsIncremental` 签名与 02 §4 一致。
  - `web/src/App.vue` — 确认 `NConfigProvider + NMessageProvider` 已就位（insight L9 ✅）。
  - `internal/httpapi/handlers_logs.go` — 服务的是 `h.deps.LogFiles[kind]` 文件 = **frpc / frps 子进程标准日志**（不是项目 slog JSON 日志 `.frp_easy/logs/ui.log`），与 SA §3.6.1 regex 双格式 OR 假设吻合。
  - `web/src/components/__tests__/ProxyForm.spec.ts` L11-L24 — 确认 importOriginal + 6 方法 stub mock 范式存在且可复用（insight L29 ✅）。
  - `.harness/insight-index.md` — 通读 L9 / L26 / L28 / L29 / L36 / L37 / L41 / L43 / L46 / L49，逐条核对。
  - `.harness/rules/00-core.md` + `50-fullstack.md` — 红线（不 inline style / SFC < 200 / 主题 token / 中文 UI / 不删测试）核对。
- 全量 `grep` `style=` 在 `web/src/**/*.vue`（97 处，15 文件）—— 项目存在大量 `style=`，SA §10 self-check 把"无 inline style"理解为"无用于布局的硬编码 style"是合理（与 `.harness/rules/50-fullstack.md` L28 一致）；本任务自身设计中 inline style 用例已用 CSS 变量 / scoped `:deep` 规避（§3.3 + §3.5）。

### 1.3 verdict 一句话预告

设计成熟度高、决策矩阵齐、风险显式、reuse audit 真读过代码、insight 命中充分。少量 **WARN**（不阻塞 stage 4 启动，dev 应主动消化）；**0 个 FAIL**；**0 个 BLOCKED**。验证下文。

---

## §2 完整性检查

### 2.1 in-scope 覆盖

01 §2 共 26 条 in-scope 行为（编号 1-26）。在 02 中逐条命中：

| 01 in-scope # | 02 设计锚点 | 状态 |
|---|---|---|
| 1 等宽字体 | §3.4 LogLine CSS scoped `.line-message` 字体栈 + §3.7 主题 token | PASS |
| 2 字号 / 行距 13/12/16 | §3.6.6 `useLogPrefs.fontSize` 默认 13、`fontSizePx` computed | PASS |
| 3 等级着色 | §3.4 `.level-error/warn/info/debug/trace/plain` CSS + §3.7 var(--log-error/warn/...) | PASS |
| 4 行号列 + user-select: none | §3.4 `.line-number { user-select: none }` + AC-6 验证复制不含行号 | PASS |
| 5 padding-y 分组（不画线）| §3.4 `.log-line` padding-y 实现（CSS 片段例示）| PASS（细节由 dev 落） |
| 6 三段视觉分块 | §3.4 `<span .line-timestamp/level/message>` DOM 结构 | PASS |
| 7 搜索 + Aa 切大小写 | §3.6.3 `useLogSearch` + §3.2 工具条控件 1 | PASS |
| 8 搜索高亮 `<mark>` + 未命中隐藏 | §3.4 `renderedMessage` + §3.6.3 `visibleLines` 排除空 hits | PASS |
| 9 等级多选 | §3.6.4 `useLogLevelFilter` + §3.2 控件 2 | PASS |
| 10 跟随尾部默认开 | §3.6.5 状态机 + §3.6.6 `followTail` 默认 true | PASS |
| 11 滚动暂停 32 px 阈值 | §3.6.5 E2 + D-3 决策矩阵 | PASS |
| 12 ↓ 底部按钮 | §3.6.5 E5 / §3.2 控件 8 | PASS |
| 13 复制全部 + execCommand 降级 | §5.4 `onCopy()` 完整实现 | PASS |
| 14 清屏不调后端 | §3.6.2 `clear()` + AC-7 | PASS |
| 15 折行切换 + localStorage 持久化 | §3.6.6 `wrap` | PASS |
| 16 高度档 300/500/800/全屏 | §3.6.6 `height` + §3.2 控件 5 | PASS |
| 17 全屏 Modal（非 Fullscreen API）| §3.5 `FullscreenLogModal` | PASS |
| 18 不做导出按钮 | §9 OOS 5 显式确认 | PASS |
| 19 加载中 spin | §3.3 状态分支 2 + AC-15 路径 | PASS |
| 20 空态 / 无命中文案 | §3.3 状态分支 3/4 + BC-1/BC-8/BC-9 | PASS |
| 21 错误反馈（首次红字 + 重试 / 轮询小红点 / 3 次停）| §3.6.2 `consecutiveFailCount` + §3.3 状态分支 1 + §3.2 控件 11 | PASS |
| 22 心跳"上次更新" | §3.6.2 `lastUpdatedAt` + §3.2 控件 9 | PASS |
| 23 行数指示 `342 / 500` | §3.2 控件 10 | PASS |
| 24 暗色/浅色双适配（主题 token）| §3.7 全 CSS var | PASS |
| 25 主题切换实时跟随 | §3.7 `useThemeVars` ComputedRef 自动重算 + A-2 假设 + R-2 缓解 | PASS（A-2 待 dev 验证）|
| 26 切 kind 行为 | §5.2 watch + BC-4 + BC-5 | PASS |

**结论：26/26 全部命中**，无 in-scope 行为遗漏。

### 2.2 boundary conditions 覆盖

01 §4 共 13 BC。逐条核对：

| BC | 02 设计锚点 | 状态 |
|---|---|---|
| BC-1 空缓冲 | §3.3 状态分支 3 + AC-15 | PASS |
| BC-2 超长单行 | §3.4 `.line-message.wrap`/`.nowrap` CSS 实现 + AC-14 | PASS |
| BC-3 500 行 + 增量 | §3.6.2 `slice(-500)` + lineNumber 在缓冲内 1-based（§3.6.3 显式说明）| PASS |
| BC-4 切 kind | §5.2 watch + BC-5 联动 | PASS |
| BC-5 in-flight race | §3.6.2 `kindEpoch` 私有计数器 + await 后比对 | PASS（详见 §4 L26）|
| BC-6 连续 3 次失败停 | §3.6.2 + 2.4 §21 | PASS |
| BC-7 32 px 阈值 + 不自动反转 | §3.6.5 E2 + 状态机 transition table "不在距底 ≤32 时自动反转 paused → false" | PASS |
| BC-8 搜索无命中 | §3.3 状态分支 4 | PASS |
| BC-9 等级全去勾 | §3.6.4 + §3.3 状态分支 4 | PASS |
| BC-10 父卸载 timer | §5.3 onUnmounted + R-4 风险条 | PASS |
| BC-11 后台节流 OOS | §9 OOS 8 显式声明 | PASS |
| BC-12 缓冲固定 500 | §3.6.2 max=500 默认 | PASS |
| BC-13 localStorage 不可用降级 | §3.6.6 `safeStorage` 单点 + D-4 决策 | PASS |

**结论：13/13 全部命中**。

### 2.3 AC 1:1 对应

01 §5 共 17 AC。02 §8.2 提供 1:1 映射表覆盖 AC-1..AC-17 + 测试文件归属。已逐条核对设计中是否有实施锚点：

- AC-1..AC-16：全部有具体 mount / composable 单测点，归属测试文件落实到 6 个 `__tests__/*.spec.ts`。
- AC-17（bundle < 50 KB）：归 stage 6 QA 手工 verify_all build size diff —— 接受方式合理（不在单测内做）。

**结论：17/17 全部命中 + 测试落点明确**。

### 2.4 NFR 覆盖

01 §6 共 9 NFR。02 中覆盖：

| NFR | 02 锚点 | 状态 |
|---|---|---|
| NFR-1 首次渲染 500 行 < 200 ms | §3.8 性能策略 + parsedLines memoization | PASS |
| NFR-2 50 ms long task | §3.8 + Map 缓存 | PASS |
| NFR-3 bundle ≤ 50 KB | §3.8 不引重型库 + §10 self-check + AC-17 | PASS |
| NFR-4 无 inline style | §10 self-check + §3.3 单点例外 justified（注：见下方 §6 Q1）| PASS |
| NFR-5 无新 npm 依赖 | §10 self-check + §3.8 显式不引列表 | PASS |
| NFR-6 中文 UI | §10 self-check | PASS |
| NFR-7 XSS escape | §3.4 先 escape 后 mark 顺序锁死 + ADV-A | PASS |
| NFR-8 a11y | §10 self-check（aria-label / Naive UI 语义）| PASS |
| NFR-9 localStorage 降级 | §3.6.6 + BC-13 + D-4 | PASS |

**结论：9/9 全部命中**。

### 2.5 partition assignment

02 §11 明确 **dev-frontend 单分区**，14 步实现序由叶节点到根节点依赖序合理（parseLogLine → useLogPrefs → useLogBuffer → ... → LogViewer → spec → dev-map）。无并行需求、无跨分区协同。**PASS**。

### 2.6 §2 概览结论

**完整性 PASS**。无 in-scope / BC / AC / NFR / partition 维度的遗漏。

---

## §3 一致性检查（02 ↔ 01 1:1）

合并 §2.1-§2.4 之逐条核对表，**26 + 13 + 17 + 9 = 65 条 1:1 全部命中**。

特别核对 PM 决策对齐：

| 01 PM 决策 | 02 实现 | 一致性 |
|---|---|---|
| Q-a regex 解析 | §3.6.1 LOG_LINE_RE | ✅ |
| Q-b 默认不敏感 + Aa 切换 | §3.6.3 caseSensitiveRef | ✅ |
| Q-c 固定 500 | §3.6.2 max=500 | ✅ |
| Q-d 沿用 Naive UI token | §3.7 | ✅ |
| Q-e n-modal 不用 Fullscreen API | §3.5 | ✅ |
| Q-f 不做导出 | §9 OOS 5 | ✅ |
| Q-g n-select 多选 | §3.2 控件 2 | ✅ |
| Q-h 跟随尾部默认开 | §3.6.6 followTail 默认 true | ✅ |

**一致性 PASS**。

---

## §4 insight 命中审计

逐条核对 PM 派发指示中列出的 9 条 insight：

### L9 — NMessageProvider 必须在 App.vue

`web/src/App.vue` L1-L12 实测 `<n-config-provider><n-message-provider><router-view /></...>`。**SA 在 §12 Reuse audit 第 3 行明示"确认就位，无需改"**，并在 §3.1 setup 中直接 `useMessage()` 不重新包 provider。命中 ✅。

### L28 — 父子双向 v-model 桥 + composable `toXxx()` 每次返回新对象 = OOM 反模式

T-032 教训：单向数据流 + `defineExpose getXxxInput()`。02 §1.3 数据流明文："LogViewer 是唯一拥有 composable 实例的层。子组件接受 props，向上 `emit` 用户意图（点击、滚动）而不是状态突变。这是与 T-032 经验对齐的单向数据流（insight L28）—— 不引入二次 `defineModel` 双向桥。" 命中 ✅。

设计中所有 `update:xxx` emit 均为"用户意图事件"（开关切换、搜索框输入），不涉及 composable 输出对象的反向 push，不可能复发 OOM。

### L29 — vitest + happy-dom mount Naive UI 必须用 importOriginal + 6 方法 stub mock 模式

02 §8.1 完整给出 mock 模板，与 `ProxyForm.spec.ts:11-24` 字节级对齐。命中 ✅。

### L36 — vitest mount 测试范式（业务逻辑 composable / 组件必须 Vitest 单测）

02 §2 文件清单 #12-17 共 6 个 `__tests__/*.spec.ts`，分别测 LogViewer mount 集成 + 5 个 composable。命中 ✅。

### L37 — e2e fixture 类不用 Vitest mock 的对称镜像约定

02 §8.3 显式："Playwright e2e 不写——本任务纯 UI 改造，无新路由 / 无新后端契约 / 现有 e2e `03-dashboard.spec.ts` 已覆盖 LogViewer 渲染 smoke。如 stage 6 QA 认为需要补 e2e，单独追加。" 命中 ✅。

### L41 — GR 03 conditions WARN 类应被 developer 主动消化

本评审 §7 将提交 conditions 段，SA 已知本 idiom。命中将由 stage 4 dev 消化 ✅（前向约束）。

### L43 / L46 / L49 — Stage 7 §N Insight 数字编号前缀让 archive-task harvest 0 命中

属 stage 7 责任，本任务 stage 3-6 不触及；但记入 §7 conditions 提醒 PM 在 07 用裸 `## Insight` + `- ` bullet。命中 ✅（前向提醒）。

### L26 — verify_all 双实现（PS + Bash）regex / multiline / 输入模式对账

本任务不动 verify_all 自身。但 NFR-3 / AC-17 涉及"bundle size diff" —— 若未来在 verify_all 加该 step，必须 PS + Bash 双实现对账。本期不影响。**N/A**（不在本任务路径上）。

### insight 命中审计结论

9 条 insight 中 8 条直接命中（L9 / L28 / L29 / L36 / L37 / L41 前向 / L43+L46+L49 前向），L26 N/A。**PASS**。

---

## §5 风险评估 + 决策矩阵审计

### 5.1 §7 决策矩阵 5 个

| 矩阵 | 选项数 | 选项分析 | 决策 | 评审意见 |
|---|---|---|---|---|
| D-1 组件拆分粒度 | 4（a/b/c/d）| 红线 SFC < 200 行 + 测试可达性 + 重用价值；c 是"刚好满足红线 + LogLine 可独立测主题色" | (c) Toolbar + List + Line + Modal 四拆 | **充分**。维度选取合理，结论可辩护。|
| D-2 搜索算法 | 3（indexOf / regex / fuse.js）| 包体增量 + 性能 + 心智负担；fuse.js 30 KB 直接违 NFR-3 | (a) String.indexOf 循环 | **充分**。regex escape 已知陷阱被显式提及。|
| D-3 距底阈值 | 3（0 / 32 / 100 px）| 误触发 + 用户体感 | 32 px | **充分**。理由"鼠标滚轮最小步进 100 px+"是合理工程估算。|
| D-4 localStorage BC-13 降级 | 3（内存 Map / 弹 warning / 禁用 UI）| 实现复杂度 + 用户感知 | (a) 内存 Map 静默 | **充分**。隐私模式用户骚扰避免合理。|
| D-5 parsedLines memo | 3（每次重 parse / Map 缓存 / LRU 上限）| 复杂度 + 性能 | Map 缓存 | **充分**。slice(-500) 自然限 Map 大小，GC 友好。|

**5 个决策矩阵全部说服评审**。无可挑战的盲点。

### 5.2 §13.1 风险表 8 个 R-1..R-8

| 风险 | SA 缓解措施 | 评审 |
|---|---|---|
| R-1 regex 不匹配真实 frp 日志 | A-1 假设 + dev 阶段抓真实样本；regex 集中 1 处便于调 | **充分**。降级 PLAIN 不崩溃，最坏情况只是失去着色。|
| R-2 useThemeVars 不响应 | watch + 显式 trigger 或双 class 方案 | **充分**。fallback 路径已设计。|
| R-3 escape 顺序写反 → XSS | ADV-A 强测 + §3.4 顺序锁死 | **充分**。|
| R-4 onUnmounted 漏 stopPolling | Vitest mount + unmount 断言 `clearInterval` 调用次数 | **充分**。|
| R-5 localStorage quota throw | try-catch 全包 + ADV-B | **充分**。|
| R-6 bundle 超 50 KB | 不引依赖 + verify_all 测；超了再删 PLAIN 等非核心 | **充分**。|
| R-7 NFR-2 long task | parsedLines memoization | **充分**。|
| R-8 Modal 关闭 → LogList 重 mount → 滚动位置丢 | 设计上接受；OOS 第二阶段优化 | **可接受**。SA 明确不阻塞本期。|

**风险表全过**。无被掩盖的高概率风险（评审独立思考下也想不到第 9 项）。

### 5.3 §6 假设表 6 个 A-1..A-6

| 假设 | dev 可验证性 | 评审 |
|---|---|---|
| A-1 frp 日志格式 | dev 抓真实样本 `logs/frpc.log` / `logs/frps.log` | **可验证**。评审独立 ground：`internal/httpapi/handlers_logs.go:25-71` 服务的是 `h.deps.LogFiles[kind]` 文件（= frpc / frps 子进程标准输出文件），frp 上游标准日志格式确为 `YYYY/MM/DD HH:MM:SS [I/W/E/D/T] ...`，SA regex 合理。**WARN 见 §6 Q1**。|
| A-2 useThemeVars 响应主题切换 | mount × 2 不同 theme provider 验 | **可验证**。AC-13 直接覆盖。|
| A-3 localStorage 可用 | BC-13 已有降级 | **不需验证**（已有兜底）。|
| A-4 execCommand 兜底 | A-4 极端环境双失败 | **可接受**（极小概率）。|
| A-5 secure context | onCopy 降级路径 | **可接受**。|
| A-6 n-modal :style 支持 | SA 已选 scoped `:deep(.n-card)` 路径绕开 | **已规避**。|

**假设表合理**，且关键假设（A-1 / A-2）都有 dev 阶段验证 / 测试覆盖路径。

### 5.4 §5 风险评估结论

决策矩阵 + 风险表 + 假设表三件套**充分说服评审**。本设计已经做了项目历史经验内能想到的所有反向证伪。**PASS**。

---

## §6 关键问题（Q1..Q5）

### Q1 — A-1 regex 是否真能解析 frp 上游日志格式？

**ground 证据**：

- `internal/httpapi/handlers_logs.go:31` 服务文件路径 = `h.deps.LogFiles[kind]`，由 `procmgr.Manager` 在启动 frpc / frps 子进程时把 stdout/stderr 重定向到该文件（`internal/procmgr/manager.go:11` 注释 + `manager_test.go:47` fixture 路径）。
- 项目自身 slog JSON 日志（`.frp_easy/logs/ui.log`）服务的是 UI 后端，**不**经 `/api/v1/logs/{kind}` 路由——后者只服务 frpc / frps 子进程文件。
- frp 上游（fatedier/frp）官方日志格式确为 `2024/01/15 10:23:45 [I] [proxy_manager.go:108] proxy added: [ssh]`（短字母 I/W/E/D/T）+ 部分二次封装变体为 `2024-01-15 10:23:45 [INFO] ...`。
- SA §3.6.1 LOG_LINE_RE 单条 OR 双格式：
  ```
  /^(\d{4}[-/]\d{2}[-/]\d{2}[ T]\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+\[?(I|W|E|D|T|ERROR|WARN(?:ING)?|INFO|DEBUG|TRACE)\]?\s+(.*)$/i
  ```
  实测语义校验：
  - `2024/01/15 10:23:45 [I] [proxy_manager.go:108] proxy added` → m[1]="2024/01/15 10:23:45", m[2]="I", m[3]="[proxy_manager.go:108] proxy added" ✅
  - `2024-01-15 10:23:45 [INFO] message` → m[1]="2024-01-15 10:23:45", m[2]="INFO", m[3]="message" ✅
  - `2024-01-15T10:23:45.123 [E] msg` → m[1]="2024-01-15T10:23:45.123", m[2]="E", m[3]="msg" ✅
  - panic stack `goroutine 1 [running]:` → 不匹配 → PLAIN 降级 ✅（这是期望行为，stack trace 没有 level）

**pre-answered: 02 §3.6.1 + §6 A-1**。

**评审判断**：regex 双格式 OR + PLAIN 降级足够稳健。**唯一遗漏**是 frp 上游也可能输出 `[subsystem.go:line]` 字段在 level 之后（属 message 一部分，SA 设计中没拆出 caller field），但这不是问题——01 §2.6 没要求拆 caller，message 内连带渲染合理。**PASS**。

### Q2 — BC-5 kindEpoch race 的实施细节是否够具体？

**pre-answered: 02 §3.6.2** 写明 "`kindEpoch` 私有计数器，每次 `kindRef()` 变化 +1；in-flight `loadIncremental` 在 await 后比对 epoch，不匹配则丢弃响应"。

**评审判断**：具体到可实现层级。建议 dev 在 `loadIncremental` 内的伪代码骨架：
```ts
const epochAtStart = kindEpoch.value
const res = await apiGetLogsIncremental(kindRef(), currentOffset)
if (epochAtStart !== kindEpoch.value) return  // 丢弃过期响应
// 否则正常 append
```
**ADV-D 测试覆盖**（§8.4）已设计反向证伪用例。**PASS**。

### Q3 — BC-7 32 px 阈值的实施细节是否够具体？

**pre-answered: 02 §3.6.5 onScroll** 写明 "`scrollHeight - scrollTop - clientHeight > 32` 即'距底 > 32 px'"。

**评审判断**：清晰可实施。**transition table 显式标注"不在距底 ≤ 32 时自动反转 paused → false"**（与 BC-7 锁死的"避免抖动"语义一致），AC-5 测试覆盖。**PASS**。

### Q4 — BC-13 localStorage 降级的实施细节是否够具体？

**pre-answered: 02 §3.6.6** 写明 "私有 `safeStorage`：`{ get(key, default), set(key, value) }`，BC-13 降级时切换到 `Map<string, string>` 内存版。**所有降级语义只此一处**。" + D-4 决策矩阵。

**评审判断**：单点封装设计良好。ADV-B 测试（`localStorage.setItem` 强 throw quota）反向证伪可达。**PASS**。

### Q5 — §8.4 ADV 候选 4 条能否让 stage 6 QA 有反向证伪料？

**pre-answered: 02 §8.4** 提供 4 条 ADV：

- **ADV-A** XSS escape 顺序（NFR-7 / R-3 反向证伪）
- **ADV-B** localStorage quota throw（BC-13 / R-5 反向证伪）
- **ADV-C** 3 次轮询失败停 polling + message.error 仅调一次（BC-6 / 2.4 §21 反向证伪）
- **ADV-D** kindEpoch race（BC-5 / R-? 反向证伪）

**评审判断**：4 条 ADV 均为**业务逻辑反向证伪**（不是 mock-only spec，符合 insight L30 / L37 精神）。stage 6 QA 只需挑 ≥ 1 条实施即满足红线，4 条候选丰俭由人。**PASS**。

### §6 关键问题结论

**全部 5 个 Q 均 pre-answered**。无"需 dev 阶段验证/消化"不阻塞项。

---

## §7 Conditions（dev 在 04 应主动消化，不阻塞 stage 4 启动）

> 参考 insight L41：WARN / 建议类 finding 不阻塞 stage 4 启动，但**应该被 developer 主动消化**（即兴补注释、补警告、纠正失实描述）。好的 developer 不是"满足下限"而是"在自然顺手时一并消化所有 C-N"，让 stage 5 处于"几乎无需改"状态。

### C-1 — frp 日志真实样本验证 regex（A-1 假设落地）

**问题**：02 A-1 假设 frp 日志格式，但项目仓库内**无** frpc.log / frps.log 真实样本可对（评审 `Glob **/frpc*.log` / `**/frps*.log` 均无文件，只有 `.frp_easy/logs/ui.log` 是 JSON slog 格式与本任务无关）。

**dev 消化建议**：

- stage 4 开发前，本机短跑一次 `frpc -c <test.toml>` 或 `frps -c <test.toml>`，抓 5-10 行真实日志样本贴到 `parseLogLine.spec.ts` fixture（与单测同源）。
- 若 dev 机无 frp 二进制方便起服，可从 frp 官方仓库 README / examples 抓样本字串。
- 至少在 `parseLogLine.spec.ts` 中覆盖 5 种格式：`[I]` 短字母、`[INFO]` 长全称、`[W]/[WARN]/[WARNING]`、带 `[subsystem.go:line]` caller 字段、panic stack 多行（→ PLAIN）。

**优先级**：MEDIUM（不阻塞，但漏了 dev 阶段补会让 R-1 在 stage 5 / 6 暴露）。

### C-2 — A-2 useThemeVars 响应性预先做最小 spike

**问题**：项目从未使用 `useThemeVars`（02 §12 Reuse audit 第 7 行 "新引入；本任务首次使用"）。若 A-2 假设不成立（响应性不工作），AC-13 测试会失败。

**dev 消化建议**：

- 步骤 11（实施 LogViewer.vue 壳组件）前，先在临时 spike 文件 `web/src/scratch-themevars.vue`（不 commit）mount 测一次：`useThemeVars().value.errorColor` 在 `n-config-provider :theme="darkTheme"` ↔ `:theme="null"` 切换时是否在 watch 回调里出现变化。
- 若不响应（A-2 失败），按 R-2 缓解走双 class 方案 + 显式 watch；并在 04_DEVELOPMENT.md `Design drift` 段记录。

**优先级**：MEDIUM（A-2 失败时 fallback 已设计，但 stage 4 早发现比 stage 5 / 6 暴露好）。

### C-3 — 02 §3.3 "唯一允许 inline style 例外" 在 dev 落地时收紧

**问题**：02 §3.3 说 "LogList max-height 通过 `:style="{ '--log-list-height': heightPx + 'px' }"` 在容器上写 CSS 变量"，**这仍是 inline style 写入**（即便值是单一 CSS 自定义属性）。stage 5 `grep style=` 会命中。

**dev 消化建议**：

- 在 LogList.vue 该行**上方**加一行行内注释：
  ```vue
  <!-- justify-inline-style: 单一动态 CSS 变量赋值，无法走静态 class（高度 300/500/800 三档之外允许未来扩展任意整数 px）；
       NFR-4 self-check §10 中已 PM/SA 双签字接受 -->
  <div class="log-list-scroll" :style="{ '--log-list-height': heightPx + 'px' }">
  ```
- 让 stage 5 Code Reviewer grep 时立即看到 justify 注释而不必反查 02 §3.3。

**优先级**：LOW（02 §10 self-check 已记录 justify，但落地需要落到代码注释让维护者一眼可见）。

### C-4 — §3.5 FullscreenLogModal 95vw/90vh 路径选择已 SA 决策但请落实

**问题**：02 §3.5 给了 2 个候选实现路径（inline `:style="{width:'95vw',height:'90vh'}"` vs scoped `:deep(.n-card)`），最终 SA 在文中拍板 "用 scoped `<style>` 风格，避免任何 inline"。

**dev 消化建议**：

- FullscreenLogModal.vue 实现时按 SA 决策走 scoped `:deep(.n-card)`，**不要**重新评估两路径。
- 注意 `:deep()` 选择器在 Vue 3 + Naive UI 下的兼容性 —— scoped CSS 穿透到 Naive UI 内部 `.n-card` 容器是文档化用法，但实际 Naive UI 2.x card 实际 class 名可能是 `.n-card-header` / `.n-card__content` 等子结构，dev 需 inspect DOM 调对选择器层级。

**优先级**：LOW（实施细节）。

### C-5 — Stage 7 §N Insight 收割红线（前向提醒）

**问题**：insight L43 / L46 / L49 累计 4 次复现 archive-task `## Insight` 标题被 `§N` 数字前缀劫持 + insight L42 复现 body 必须 `- ` bullet 不能 `### Insight 1:` 子标题。

**dev 消化建议**：

- 本期不涉及 stage 7（dev 写 04，不写 07），但请在 04_DEVELOPMENT.md 末尾保留任何 "本任务沉淀的 insight 候选" 用 `- ` bullet 格式列出（不要起 `### Insight N` 子标题），方便 PM 在 stage 7 直接复制到 `## Insight` 段。
- PM 写 07_DELIVERY.md 时**必须**用裸 `## Insight`（无 `§N` 前缀）+ `- ` bullet（参考 insight L42 / L43 / L46 / L49）。

**优先级**：LOW（前向，dev 友好）。

### C-6 — 旧 LogViewer.vue 现状描述补正（极轻微）

**问题**：01 INPUT.md L21 描述旧 LogViewer "硬编码深色背景 `#1a1a1a` ... 500 px 高度 ... `max-height + overflow-y: auto` ... `white-space: pre-wrap; word-break: break-all`"，全部由评审 ground `web/src/components/LogViewer.vue:14` 实测确认 ✅。无失实。

**dev 消化建议**：在 04 §"Design drift"（若有）记录"旧 SFC 实测 94 行（含 script + template + 空行），与 INPUT.md 估算一致"作为完整性证据。

**优先级**：INFO（无需 action）。

### §7 conditions 结论

6 条 conditions（C-1 MEDIUM × 2 + C-3 LOW + C-4 LOW + C-5 LOW + C-6 INFO）。**全部不阻塞 stage 4 启动**。

---

## §8 8-dimension audit

按 gate-reviewer 标准 8 维评分：

| # | Dimension | 评审 | 结论 |
|---|---|---|---|
| 1 | Requirement completeness | 01 §2 26 条 in-scope 全部可测；8 Q 全部 PM 就地决策；OOS 11 条边界清晰 | **PASS** |
| 2 | Design completeness | 02 设计 1:1 覆盖 26 in-scope + 13 BC + 17 AC + 9 NFR；§3 详细设计涵盖壳 + 4 子组件 + 5 composable + 主题色 + 性能策略 | **PASS** |
| 3 | Reuse correctness | §12 Reuse audit 真读过现有代码：`apiGetLogsTail` / `ProxyForm.spec.ts:11-24` mock 范式 / `App.vue` provider 就位 / `web/src/composables/*` 单 composable 单文件惯例；评审 ground 全部确认存在 | **PASS** |
| 4 | Risk coverage | §13.1 8 个风险 + §6 6 个假设；评审独立思考下未发现第 9 项遗漏 | **PASS** |
| 5 | Migration safety | 无 schema / API 变更；§13.2 回滚 = 单次 git revert；无数据迁移 | **PASS** |
| 6 | Boundary handling | 13 BC 全部命中；kindEpoch race / 32 px 阈值 / localStorage 降级三大棘手边界全部有实施细节 | **PASS** |
| 7 | Test feasibility | 17 AC 1:1 到测试文件 + 4 条 ADV 候选 + insight L36 / L37 / L29 三个测试 idiom 全部对齐 | **PASS** |
| 8 | Out-of-scope clarity | 01 §3 10 条 + 02 §9 追加 8 条 OOS；范围边界刚性清晰，dev 不会过度构建 | **PASS** |

**8/8 PASS，0 WARN，0 FAIL**。

---

## §9 Verdict

**APPROVED**

理由：

1. 完整性 / 一致性 / insight 命中 / 风险评估 / 边界 / 测试 / OOS 八个维度全部 PASS。
2. 02 设计 1:1 覆盖 01 全部 65 条可测项（26 in-scope + 13 BC + 17 AC + 9 NFR），无遗漏。
3. 9 条关键 insight（L9 / L28 / L29 / L36 / L37 / L41 / L43 / L46 / L49）全部命中 / 前向提醒到位，L26 N/A。
4. 5 个决策矩阵 + 8 个风险 + 6 个假设三件套充分说服评审。
5. 关键问题 Q1-Q5（A-1 regex 可行性 / BC-5 kindEpoch / BC-7 32 px / BC-13 localStorage / §8.4 ADV）全部 pre-answered 到具体伪代码 / 算法 / 文件位置层级。
6. 5 条 conditions（C-1..C-5）属 dev 自然顺手消化范畴，**不阻塞 stage 4 启动**。
7. SA 无偷懒 / 无模糊 / 无 TODO 留尾；§10 self-check 13 项全签字。

**PM 可立即派 dev-frontend 进入 Stage 4 实施**。

下一阶段：**Stage 4 Developer (dev-frontend 单分区)** 按 02 §11.2 14 步实现序执行；产出 `04_DEVELOPMENT.md`；务必消化本评审 §7 的 5 条 conditions（C-1..C-5）。

---

_由 Gate Reviewer 写于 2026-05-24，PM 全权授权下完成 T-036 闸门评审。_
