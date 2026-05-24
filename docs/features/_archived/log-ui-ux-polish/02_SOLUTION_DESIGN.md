# 02 — Solution Design · T-036 / log-ui-ux-polish

> 任务模式：**full**
> Stage：2（Solution Architect）
> 上游：`docs/features/log-ui-ux-polish/01_REQUIREMENT_ANALYSIS.md`（Verdict = READY；26 in-scope / 13 BC / 17 AC / 9 NFR）
> 范围：纯前端，仅触 `web/**`；分区 = **dev-frontend** 单一。

---

## §1 概览

### 1.1 一句话架构

把现有 ~94 行单文件 `LogViewer.vue` 拆为「**1 个壳组件 + 4 个子组件 + 5 个 composable**」的分层结构：壳组件 `LogViewer.vue` 负责数据生命周期（loadTail / polling / kind 切换 / cleanup），4 个子组件分别承担工具条、列表、单行、全屏 Modal 的视图渲染，5 个 composable 把内存缓冲、搜索、等级筛选、跟随尾部状态机、本地偏好持久化等纯逻辑从 SFC 中抽离 → 每个 SFC 严格 < 200 行（红线 `.harness/rules/50-fullstack.md`），并让全部逻辑可在不挂 DOM 的情况下被 Vitest 直接断言。

后端 API 契约 **零变更**（`web/src/api/logs.ts` 不动）；不引入任何新 npm 依赖（NFR-3 / NFR-5）；所有颜色经 `useThemeVars()` 走 Naive UI token，主题切换实时跟随（AC-13 / 2.5 §24-25）。

### 1.2 组件树（ASCII）

```
Logs.vue (page 薄包裹，几乎零变化)
  └── <LogViewer :kind>                          ← 壳：数据生命周期 + 子组件协调
        ├── <LogToolbar>                         ← 工具条（搜索 / 等级 / 跟随 / 折行 / 高度 / 复制 / 清屏 / 全屏 / 心跳 / 计数）
        ├── <LogList>                            ← 滚动容器 + 空态 + 加载态 + 暂停跟随提示条
        │     └── v-for <LogLine>                ← 单行（行号 + timestamp / level / message 三段 + 搜索高亮）
        └── <FullscreenLogModal v-model:show>    ← 全屏 Modal；其内部再 mount 一份 LogList（复用同一数据源 / composable 实例）
```

### 1.3 数据流（ASCII）

```
        ┌────────────────────────────────────────────────────────┐
        │  LogViewer.vue (Setup 中实例化全部 composable)          │
        │                                                        │
        │   useLogBuffer  ── lines[] ──┐                         │
        │       ↑                      │                         │
        │   loadTail / loadIncremental │                         │
        │   (polling 2s)               │                         │
        │                              ▼                         │
        │   useLogLevelFilter ─ filteredByLevel ─┐               │
        │                                       ▼               │
        │   useLogSearch ──────────── visibleLines ──┐          │
        │                                            ▼          │
        │   useFollowTail  ←── scroll events ── <LogList>        │
        │   useLogPrefs (localStorage proxy)  ←─ user toggles    │
        │                                                        │
        └─────────────────┬──────────────────────────────────────┘
                          │ props down
                          ▼
        ┌────────────┬───────────┬────────────────────────┐
        │ LogToolbar │  LogList  │ FullscreenLogModal     │
        │ (UI 双向)  │ (只读视图)│ (复用 LogList 实例)    │
        └────────────┴───────────┴────────────────────────┘
                          │
                          ▼
                       LogLine[]
                  (静态 props 渲染)
```

「父子边界」契约：

- **LogViewer 是唯一拥有 composable 实例的层**。子组件接受 props，向上 `emit` 用户意图（点击、滚动）而不是状态突变。这是与 T-032 经验对齐的单向数据流（insight L28）—— 不引入二次 `defineModel` 双向桥。
- **`useLogPrefs` 是唯一与 `localStorage` 对话的层**；其他 composable 把 ref 当作"普通 reactive"消费，不知道持久化存在。BC-13 降级路径全部封在 `useLogPrefs` 内部。

---

## §2 文件清单 / Partition assignment

**分区：dev-frontend 单分区。** 全部文件落 `web/**`，无后端 / DB 改动。

| # | 文件 | 新 / 改 | 摘要 |
|---|---|---|---|
| 1 | `web/src/components/LogViewer.vue` | 改（重写） | 壳组件；持有全部 composable 实例 + 子组件协调；< 200 行 |
| 2 | `web/src/components/log/LogToolbar.vue` | 新 | 工具条（搜索 / 等级 / 跟随 / 折行 / 高度 / 复制 / 清屏 / 全屏 / 心跳 / 计数 / 失败小红点） |
| 3 | `web/src/components/log/LogList.vue` | 新 | 滚动容器 + 空态 + 加载态 + 错误重试 + 暂停跟随提示条 |
| 4 | `web/src/components/log/LogLine.vue` | 新 | 单行（行号 + timestamp / level / message 三段 + `<mark>` 搜索高亮）|
| 5 | `web/src/components/log/FullscreenLogModal.vue` | 新 | `n-modal` 包装 LogList；不复制数据 |
| 6 | `web/src/composables/log/useLogBuffer.ts` | 新 | 内存缓冲 + slice(-500) + loadTail / loadIncremental + 失败计数 + 行解析 |
| 7 | `web/src/composables/log/useLogSearch.ts` | 新 | 搜索关键字 + 大小写敏感开关 + 子串匹配 + escape |
| 8 | `web/src/composables/log/useLogLevelFilter.ts` | 新 | 等级多选过滤（含 PLAIN） |
| 9 | `web/src/composables/log/useFollowTail.ts` | 新 | 跟随尾部状态机（auto-follow / paused / resume） |
| 10 | `web/src/composables/log/useLogPrefs.ts` | 新 | localStorage 持久化封装 + BC-13 内存降级 |
| 11 | `web/src/composables/log/parseLogLine.ts` | 新 | 单行 regex 解析（timestamp / level / message / PLAIN 兜底） |
| 12 | `web/src/components/__tests__/LogViewer.spec.ts` | 新 | mount 级 AC 覆盖（AC-1/4/5/8/9/10/11/12/13/14/15/16） |
| 13 | `web/src/components/__tests__/useLogBuffer.spec.ts` | 新 | composable 单测（AC-7 行为 + BC-3 slice + BC-5 kind 切换响应丢弃） |
| 14 | `web/src/components/__tests__/useLogSearch.spec.ts` | 新 | AC-2 搜索 / 大小写 / escape / NFR-7 XSS |
| 15 | `web/src/components/__tests__/useFollowTail.spec.ts` | 新 | AC-4 / AC-5 / BC-7 状态机转移表 |
| 16 | `web/src/components/__tests__/useLogPrefs.spec.ts` | 新 | AC-8 持久化 + BC-13 降级 |
| 17 | `web/src/components/__tests__/parseLogLine.spec.ts` | 新 | regex 命中 5 级 + PLAIN 降级 |
| 18 | `docs/dev-map.md` | 改（小） | 在 components / composables 段补 `log/` 子目录 |

`web/src/pages/Logs.vue` **不改**（外层 wrapper 已经够薄，无修改必要）。
`web/src/api/logs.ts` **不改**。

新增文件统计：14 个生产文件 + 6 个测试文件 + 1 个 dev-map 编辑。

---

## §3 详细设计

### 3.1 LogViewer.vue（壳，~150 行内）

**职责：** 数据生命周期 + composable 编排 + 子组件协调。

**Setup 顺序：**

```ts
const props = defineProps<{ kind: string }>()
const themeVars = useThemeVars()
const message = useMessage()

// 1. 持久化偏好（localStorage 优先，BC-13 降级内存）
const prefs = useLogPrefs()  // { wrap, height, fontSize, followTail, caseSensitive }

// 2. 缓冲（含 loadTail / loadIncremental / failCount / kindEpoch）
const buf = useLogBuffer(() => props.kind, { max: 500, message })

// 3. 等级筛选（含 PLAIN）
const filter = useLogLevelFilter(buf.parsedLines)

// 4. 搜索（消费 filter 输出；输出含 highlight ranges）
const search = useLogSearch(filter.filteredLines, prefs.caseSensitive)

// 5. 跟随尾部状态机
const follow = useFollowTail(prefs.followTail)
```

**生命周期：**

- `onMounted` → `buf.loadTail()`；初始化 `prefs`；不自动启动 polling（保持 T-001 默认行为）。
- `watch(() => props.kind, ...)` → 见 BC-4：停 polling + reset autoRefresh + 清缓冲 + reset offset + bump kindEpoch + loadTail。**不**重置搜索 / 等级 / 折行 / 高度 / 跟随尾部（per BC-4 跨 kind 保留偏好层）。
- `onUnmounted` → `buf.stopPolling()` + `prefs.flush()`（同步写一次 localStorage，防 Modal 内未及时写）。

**模板（伪代码）：**

```vue
<template>
  <div class="log-viewer-root" :style="rootCssVars">
    <LogToolbar
      :search="search.query.value"           @update:search="search.setQuery"
      :case-sensitive="prefs.caseSensitive"  @update:case-sensitive="prefs.setCaseSensitive"
      :levels="filter.activeLevels.value"    @update:levels="filter.setActiveLevels"
      :follow-tail="follow.enabled.value"    @update:follow-tail="follow.toggle"
      :wrap="prefs.wrap"                     @update:wrap="prefs.setWrap"
      :height="prefs.height"                 @update:height="prefs.setHeight"
      :last-updated="buf.lastUpdatedAt.value"
      :count="buf.lines.value.length"
      :max-count="500"
      :auto-refresh="buf.autoRefresh.value"  @update:auto-refresh="buf.setAutoRefresh"
      :fail-count="buf.consecutiveFailCount.value"
      :last-error="buf.lastError.value"
      @copy="onCopy"
      @clear="buf.clear"
      @fullscreen="fullscreenOpen = true"
      @scroll-to-bottom="follow.scrollToBottom"
    />
    <LogList
      v-if="!fullscreenOpen"
      :visible-lines="search.visibleLines.value"
      :wrap="prefs.wrap"
      :height-px="prefs.heightPx"
      :font-size-px="prefs.fontSizePx"
      :loading="buf.firstLoading.value"
      :first-load-error="buf.firstLoadError.value"
      :follow-tail="follow.enabled.value"
      :paused="follow.paused.value"
      @scroll="follow.onScroll"
      @retry="buf.loadTail"
      @resume-follow="follow.resume"
      @clear-filters="onClearFilters"
    />
    <FullscreenLogModal
      v-model:show="fullscreenOpen"
      :visible-lines="search.visibleLines.value"
      :wrap="prefs.wrap"
      :font-size-px="prefs.fontSizePx"
      :loading="buf.firstLoading.value"
      :first-load-error="buf.firstLoadError.value"
      :follow-tail="follow.enabled.value"
      :paused="follow.paused.value"
      @scroll="follow.onScroll"
      @retry="buf.loadTail"
      @resume-follow="follow.resume"
    />
  </div>
</template>
```

`rootCssVars` 是计算属性，把 `themeVars` 的相关 token 投到 CSS 自定义属性上（见 §3.7）。

### 3.2 LogToolbar.vue（~140 行）

**props（只入不出）**：search、caseSensitive、levels、followTail、wrap、height、lastUpdated、count、maxCount、autoRefresh、failCount、lastError。

**emit**：update:search、update:caseSensitive、update:levels、update:followTail、update:wrap、update:height、update:autoRefresh、copy、clear、fullscreen、scrollToBottom。

**控件清单**（从左到右）：

1. `n-input` 搜索框（占位符 "搜索关键字…"）+ `Aa` `n-button text` 切大小写敏感（高亮态用 `themeVars.primaryColor`）。
2. `n-select` 多选等级（选项：ERROR / WARN / INFO / DEBUG / TRACE / PLAIN，默认全选；最大宽度 240px）。
3. `n-switch` 跟随尾部 + 文案 "跟随尾部"。
4. `n-switch` 折行 + 文案 "折行"。
5. `n-select` 高度档（300 / 500 / 800 / 全屏）—— 选"全屏"等价于触发 `emit('fullscreen')`，select 立即回退到上一档（避免持久化"全屏"档）。
6. `n-button` 复制（触发 `emit('copy')`）。
7. `n-button` 清屏（触发 `emit('clear')`）。
8. `n-button` ↓ 底部（触发 `emit('scrollToBottom')`）。
9. `n-text depth="3"` 心跳"上次更新：HH:MM:SS"。
10. `n-text depth="3"` 计数 `342 / 500`。
11. 失败小红点 `<span class="fail-dot">` + `n-tooltip` 显示最近一次错误（仅当 `failCount > 0`）。
12. `n-switch` 自动刷新（与 §11 失败 3 次自动关联动；polling 是 buf 的事，开关只是 UI 边界）。

控件不写 inline style；全部走 scoped CSS class + CSS 变量（NFR-4）。

### 3.3 LogList.vue（~120 行）

**职责：** 滚动容器渲染 + 状态分支（空态 / 加载态 / 错误 / 列表）。**不**持有滚动状态机，事件原样向上 `emit`。

**props**：visibleLines、wrap、heightPx、fontSizePx、loading、firstLoadError、followTail、paused。

**emit**：scroll（透出 `{ scrollTop, scrollHeight, clientHeight }`）、retry、resumeFollow、clearFilters。

**状态分支（按优先级）：**

1. `firstLoadError` 非空 → 渲染红字 + `重试` 按钮（AC-16）。
2. `loading` → `n-spin` + 文案 "正在加载日志…"。
3. `visibleLines.length === 0` 且缓冲也为 0 → 空态 "暂无日志输出"（BC-1 / AC-15）。
4. `visibleLines.length === 0` 且缓冲 > 0 → "无匹配日志（已应用筛选 / 搜索）" + "清空筛选" 链接（BC-8 / BC-9）。
5. 正常 → `<div class="log-list-scroll" ref="scrollEl">` + `v-for LogLine`。

**滚动事件透出**：`<div @scroll="onScrollNative">`，方法内构造 `{ scrollTop, scrollHeight, clientHeight }` 后 `emit('scroll', payload)`。useFollowTail 在父侧消费判断"距底 > 32 px"。

**暂停跟随提示条**：当 `paused === true` 时在列表内容顶部（sticky）渲染一条 banner "已暂停跟随；点击此处回到底部"，click → `emit('resumeFollow')`。

**`max-height` 应用**：通过 `:style="{ '--log-list-height': heightPx + 'px' }"` 在容器上写 CSS 变量，scoped CSS `.log-list-scroll { max-height: var(--log-list-height); }`。**这是项目唯一允许 inline style 的位置 —— 单一动态像素值，无法走 class**（明确 justify NFR-4，stage 5 reviewer 可白名单）。

### 3.4 LogLine.vue（< 80 行）

**职责：** 单行视觉；纯展示，无状态。

**props**：`{ lineNumber, parsed, searchHits, wrap }`，其中：

```ts
interface ParsedLogLine {
  raw: string
  timestamp?: string       // 解析失败为 undefined
  level: LogLevel | 'PLAIN'
  message: string          // 解析失败 = raw
}
interface SearchHit { start: number; end: number }  // message 字段内的命中
```

**DOM 结构：**

```html
<div class="log-line" :class="['level-' + level.toLowerCase(), wrap ? 'wrap' : 'nowrap']">
  <span class="line-number" aria-hidden="true">{{ lineNumber }}</span>
  <span class="line-timestamp" v-if="parsed.timestamp">{{ parsed.timestamp }}</span>
  <span class="line-level" v-if="parsed.level !== 'PLAIN'">{{ parsed.level }}</span>
  <span class="line-message" v-html="renderedMessage" />
</div>
```

`renderedMessage` 是 computed：

1. 先用 `escapeHtml(parsed.message)` 转义全部 `& < > " '`（NFR-7 XSS 防御）。
2. 再按 `searchHits` 区间用 `<mark class="search-hit">` 包裹（注意 hits 坐标必须基于"原始 message 字符索引"，包裹时按转义后字符串重定位 —— 用 split-by-index 策略，不要 regex replace）。
3. **必须保留**这种"先 escape 后包裹"顺序；如果反过来 `<mark>` tag 自身会被 escape 成 `&lt;mark&gt;`。
4. `searchHits` 为空时跳过包裹，直接返回 escape 后字符串。

`.line-number` CSS `user-select: none; -webkit-user-select: none`（2.1 §4 复制不带行号）。

`.line-message.wrap` → `white-space: pre-wrap; word-break: break-all`；`.line-message.nowrap` → `white-space: pre; overflow-x: auto`（BC-2）。

### 3.5 FullscreenLogModal.vue（< 60 行）

**职责：** `n-modal preset="card"` 包装一份 LogList，复用同一 props 集合。

```vue
<n-modal :show="show" preset="card" :style="{ width: '95vw', height: '90vh' }"
         :title="`${kind} 日志（全屏）`"
         @update:show="emit('update:show', $event)">
  <LogList v-bind="$attrs" />
</n-modal>
```

Modal 关闭时父侧 `fullscreenOpen` 切 false，LogList 在主体再次挂载；同一份 composable 实例下数据不丢（BC-10）。

注：`width: 95vw; height: 90vh` 是 Q-e PM 决策的字面表达，作为唯一允许的 inline style 例外（理由：modal 尺寸属于"单组件本身的 fundamental 配置"，比 class 抽象更直白；stage 5 白名单）。也可以选择写成 scoped `<style>` + `:style="{ '--modal-w': ..., '--modal-h': ... }"`，更严格 —— **决策（SA 拍板）**：用 scoped `<style>` 风格，避免任何 inline。给 `.fullscreen-log-modal :deep(.n-card)` 写 `width: 95vw; height: 90vh`。

### 3.6 composable 契约

#### 3.6.1 `parseLogLine.ts`（纯函数）

```ts
export type LogLevel = 'ERROR' | 'WARN' | 'WARNING' | 'INFO' | 'DEBUG' | 'TRACE'
export type LogLevelOrPlain = LogLevel | 'PLAIN'

export interface ParsedLogLine {
  raw: string
  timestamp?: string
  level: LogLevelOrPlain
  message: string
}

// frp 日志格式（参见 §6 假设 A-1）：`YYYY/MM/DD HH:MM:SS [I] [subsystem.go:123] message`
// 也兼容 `YYYY-MM-DD HH:MM:SS [LEVEL] message`（PM 指示备选格式）。
// 单条 regex 双格式 OR：
const LOG_LINE_RE =
  /^(\d{4}[-/]\d{2}[-/]\d{2}[ T]\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+\[?(I|W|E|D|T|ERROR|WARN(?:ING)?|INFO|DEBUG|TRACE)\]?\s+(.*)$/i

const SHORT_TO_LEVEL: Record<string, LogLevel> = {
  I: 'INFO', W: 'WARN', E: 'ERROR', D: 'DEBUG', T: 'TRACE',
}

export function parseLogLine(raw: string): ParsedLogLine {
  const m = LOG_LINE_RE.exec(raw)
  if (!m) return { raw, level: 'PLAIN', message: raw }
  const rawLevel = m[2].toUpperCase()
  const level = (SHORT_TO_LEVEL[rawLevel] ?? rawLevel) as LogLevel
  // WARNING 归一到 WARN
  const normalized = level === 'WARNING' ? 'WARN' : level
  return { raw, timestamp: m[1], level: normalized as LogLevel, message: m[3] }
}
```

**降级语义**：任何不匹配的行 → `{ raw, level: 'PLAIN', message: raw }`，整行作为 message 渲染、无 timestamp / level 段、按默认前景着色。**无任何抛错路径**（确保单行格式异常不污染整页渲染）。

#### 3.6.2 `useLogBuffer.ts`

```ts
interface UseLogBufferOptions {
  max?: number                                      // 默认 500
  message?: { error: (msg: string) => void }
}

interface UseLogBufferReturn {
  lines: Ref<string[]>                              // 原始行（已 join 切片）
  parsedLines: ComputedRef<ParsedLogLine[]>         // 解析缓存（memo by raw）
  lastUpdatedAt: Ref<number>                        // ms epoch；0 = 尚未首次更新
  firstLoading: Ref<boolean>                        // 仅首次 loadTail 期间 true
  firstLoadError: Ref<string | null>                // 首次 loadTail 失败原因，非 null 则渲染重试按钮
  consecutiveFailCount: Ref<number>                 // polling 连续失败计数（0..3+）
  lastError: Ref<string | null>                     // 最近一次错误消息（tooltip 用）
  autoRefresh: Ref<boolean>                         // polling 开关；切 kind 时强制 false
  setAutoRefresh: (v: boolean) => void
  loadTail: () => Promise<void>
  clear: () => void                                 // lines = []; currentOffset = 0; 不调后端
  stopPolling: () => void
}

export function useLogBuffer(
  kindRef: () => string,                            // 闭包读最新 kind，避免 watch 抖动
  opts?: UseLogBufferOptions,
): UseLogBufferReturn
```

**内部要点：**

- `currentOffset` 私有 ref，对外不暴露。
- `kindEpoch` 私有计数器，每次 `kindRef()` 变化 +1；in-flight `loadIncremental` 在 await 后比对 epoch，不匹配则丢弃响应（BC-5）。
- `consecutiveFailCount` 在每次 `loadIncremental` 成功时归零，失败时 +1；达到 3 → 调 `stopPolling()` + `autoRefresh.value = false` + `opts.message?.error('自动刷新已停止：连续 3 次拉取失败')`（2.4 §21）。
- `parsedLines` 用 `computed` + `Map<string, ParsedLogLine>` 缓存（避免 500 行每次重解析；NFR-1 / NFR-2）。
- 行解析在 `parsedLines` computed 内调 `parseLogLine`；缓存键 = `raw`。

#### 3.6.3 `useLogSearch.ts`

```ts
interface UseLogSearchReturn {
  query: Ref<string>
  setQuery: (q: string) => void
  // 已应用搜索的可见行；每行带 `searchHits: { start, end }[]` 指 message 内命中区间
  visibleLines: ComputedRef<Array<{
    lineNumber: number
    parsed: ParsedLogLine
    searchHits: Array<{ start: number; end: number }>
  }>>
}

export function useLogSearch(
  source: Ref<ParsedLogLine[]> | ComputedRef<ParsedLogLine[]>,
  caseSensitiveRef: Ref<boolean>,
): UseLogSearchReturn
```

**算法：**

- `query.value.trim() === ''` → 所有行可见、`searchHits: []`。
- 否则按 `caseSensitiveRef.value` 决定比较模式：
  - 不敏感：`message.toLowerCase().indexOf(query.toLowerCase())` 起步，循环 `indexOf(needle, lastEnd)` 找全部 hits。
  - 敏感：`message.indexOf(query)` 循环。
- 命中 0 处 → 该行从 `visibleLines` 中排除（不是淡出，per 2.2 §8）。
- 命中 ≥ 1 处 → 该行进入 `visibleLines`，附带全部 `{ start, end }`。
- `lineNumber` = 在**当前缓冲**内的本地 1-based 序号（BC-3）。

**性能**：500 行 × 平均 100 字符 × indexOf 是 O(50 KB)，单次过滤 < 5 ms（NFR-1 / NFR-2）。

#### 3.6.4 `useLogLevelFilter.ts`

```ts
const ALL_LEVELS: LogLevelOrPlain[] = ['ERROR','WARN','INFO','DEBUG','TRACE','PLAIN']

interface UseLogLevelFilterReturn {
  activeLevels: Ref<LogLevelOrPlain[]>              // 默认 = ALL_LEVELS
  setActiveLevels: (l: LogLevelOrPlain[]) => void
  filteredLines: ComputedRef<ParsedLogLine[]>
}

export function useLogLevelFilter(
  source: ComputedRef<ParsedLogLine[]>,
): UseLogLevelFilterReturn
```

`filteredLines` = `source.value.filter(l => activeLevels.value.includes(l.level))`。`activeLevels = []` 时返回空数组（BC-9）。

#### 3.6.5 `useFollowTail.ts`

**状态机：**

```
States:
  - autoFollow: boolean  (=== prefs.followTail 的"用户意图"层)
  - paused:     boolean  (用户主动滚开导致的临时挂起)

Derived: shouldStickToBottom = autoFollow && !paused

Events:
  E1 onNewLines              → if shouldStickToBottom then call scrollToBottom()
  E2 onScroll(distFromBot)
       if distFromBot > 32 && shouldStickToBottom: paused = true
       (注意：不在距底 ≤ 32 时自动反转 paused → false，per BC-7)
  E3 onUserToggleFollow(v)   → autoFollow = v; paused = false
       if v: nextTick → scrollToBottom()
  E4 onUserResume            → paused = false; nextTick → scrollToBottom()
       (用户点 "已暂停跟随" 提示条 触发)
  E5 onUserScrollToBottomBtn → nextTick → scrollToBottom()
       (不改 autoFollow / paused；per 2.2 §12 "不修改开关状态")

Transition table:
| 当前 autoFollow / paused | 事件                | 新 autoFollow / paused | 副作用              |
|--------------------------|---------------------|------------------------|---------------------|
| T / F (跟随中)           | E1 newLines         | T / F                  | scrollToBottom      |
| T / F                    | E2 scrollUp>32      | T / T                  | 显示提示条          |
| T / T (paused)           | E1 newLines         | T / T                  | 不滚                |
| T / T                    | E4 userResume       | T / F                  | scrollToBottom      |
| T / T                    | E3 toggle off       | F / F                  | -                  |
| F / F                    | E1 newLines         | F / F                  | 不滚                |
| F / F                    | E3 toggle on        | T / F                  | scrollToBottom      |
| any                      | E5 scrollBtn        | unchanged              | scrollToBottom      |
```

```ts
interface UseFollowTailReturn {
  enabled:   Ref<boolean>                           // = autoFollow
  paused:    Ref<boolean>
  toggle:    (v: boolean) => void                   // E3
  resume:    () => void                             // E4
  scrollToBottom: () => void                        // E5
  onScroll: (payload: { scrollTop: number; scrollHeight: number; clientHeight: number }) => void  // E2
  onNewLines:  () => void                            // E1 由 buffer 在 lines.length 增长时调（用 watch flush=post + nextTick）
  bindScrollEl: (el: HTMLElement | null) => void    // 让 composable 持有 scrollEl ref
}

export function useFollowTail(initial: Ref<boolean>): UseFollowTailReturn
```

实现细节：

- `scrollToBottom()`：`scrollEl.scrollTop = scrollEl.scrollHeight` —— 不做 smooth，per AC-4 误差 ≤ 1 px 硬约束。
- `onScroll` 内判 `scrollHeight - scrollTop - clientHeight > 32` 即"距底 > 32 px"。
- LogViewer 在 `watch(buf.lines, () => follow.onNewLines(), { flush: 'post' })` 触发 E1。

#### 3.6.6 `useLogPrefs.ts`

```ts
type Height = 300 | 500 | 800
const STORAGE_KEYS = {
  wrap:           'logViewer.wrap',
  height:         'logViewer.height',
  fontSize:       'logViewer.fontSize',
  followTail:     'logViewer.followTail',
  caseSensitive:  'logViewer.caseSensitive',
} as const

interface UseLogPrefsReturn {
  wrap:           Ref<boolean>     // 默认 true
  height:         Ref<Height>      // 默认 500
  heightPx:       ComputedRef<number>  // height as number
  fontSize:       Ref<number>      // 12 / 13 / 14 / 15 / 16；默认 13
  fontSizePx:     ComputedRef<string>  // `${fontSize}px`
  followTail:     Ref<boolean>     // 默认 true
  caseSensitive:  Ref<boolean>     // 默认 false
  setWrap:        (v: boolean) => void
  setHeight:      (v: Height) => void
  setFontSize:    (v: number) => void
  setFollowTail:  (v: boolean) => void
  setCaseSensitive: (v: boolean) => void
  flush:          () => void       // onUnmounted 一次性同步
}

export function useLogPrefs(): UseLogPrefsReturn
```

**实现：**

- 启动时 try-catch 读 5 个 key；解析失败 / `localStorage` 不可用（throw）→ 用默认值；不弹 message（BC-13）。
- 每个 setter 触发 `localStorage.setItem` + try-catch；失败时静默（BC-13）。
- 私有 `safeStorage`：`{ get(key, default), set(key, value) }`，BC-13 降级时切换到 `Map<string, string>` 内存版。**所有降级语义只此一处**。

### 3.7 主题色读取

```ts
// LogViewer.vue setup
import { useThemeVars } from 'naive-ui'

const themeVars = useThemeVars()
const rootCssVars = computed(() => ({
  '--log-error':    themeVars.value.errorColor,
  '--log-warn':     themeVars.value.warningColor,
  '--log-text':     themeVars.value.textColor1,
  '--log-text-3':   themeVars.value.textColor3,
  '--log-divider':  themeVars.value.dividerColor,
  '--log-bg':       themeVars.value.codeColor ?? themeVars.value.cardColor,
  '--log-mark-bg':  themeVars.value.primaryColorSuppl,
}))
```

`useThemeVars` 返回的本身就是 `ComputedRef`，主题切换时（`n-config-provider :theme="darkTheme"`）会自动重算（A-2 假设）。`rootCssVars` 二次包装把 token 投到根容器 CSS 变量，子组件全部走 `var(--log-error)` 等读取 → 切主题 0 额外代码即跟随（AC-13）。

Scoped CSS 片段示例：

```css
.log-line.level-error  .line-message,
.log-line.level-error  .line-level { color: var(--log-error); }
.log-line.level-warn   .line-message,
.log-line.level-warn   .line-level { color: var(--log-warn); }
.log-line.level-info   .line-message { color: var(--log-text); }
.log-line.level-debug  .line-message,
.log-line.level-trace  .line-message,
.log-line.level-plain  .line-message { color: var(--log-text); }
.line-number { color: var(--log-text-3); user-select: none; }
.search-hit  { background: var(--log-mark-bg); }
.log-list-scroll { background: var(--log-bg); }
```

### 3.8 性能策略（NFR-1 / NFR-2 / NFR-3）

**决策：不引入虚拟滚动。**

理由：500 行 × ~6 子节点（line-number / timestamp / level / message + 可能 mark）≈ 3000 个 DOM 节点。Chromium 在中端笔记本初次 mount 3000 节点的 SFC 实测 < 100 ms（基于项目内 Proxies.vue 现有规模外推；NFR-1 200 ms 预算有余裕）。

**不做的**：

- 不引入 `@tanstack/virtual` / `vue-virtual-scroller` / 任何虚拟滚动库（NFR-3 / NFR-5）。
- 不引入 `fuse.js` 等搜索库；纯 `indexOf` 足够。
- 不引入 `dayjs` 等时间库；保留后端原文 timestamp 字符串（OOS-8 时区切换）。

**留 OOS（未来 >2000 行场景）**：可以用原生 `IntersectionObserver` 手动可见性裁剪 —— 把不可见行替换为占位 `<div>` 保留高度。**本期不做，记入 §9 OOS。**

`parsedLines` 缓存（§3.6.2）避免 500 行每次过滤都重 regex；这是 NFR-2 的主要保障（regex 不在热路径）。

---

## §4 API 契约

**零变更。** 沿用：

- `apiGetLogsTail(kind: string, lines = 500): Promise<LogsTailResponse>` ← `web/src/api/logs.ts:4`
- `apiGetLogsIncremental(kind: string, offset: number): Promise<LogsIncrementalResponse>` ← `web/src/api/logs.ts:11`

类型 `LogsTailResponse` / `LogsIncrementalResponse` 在 `web/src/types.ts` 已有定义，不动。

后端 `/api/v1/logs/{kind}` 路由、参数、返回结构本期 100% 不动 (OOS-1)。

---

## §5 数据 / 状态边界

### 5.1 数据所属层

| 数据 | 所属 composable / 组件 | 备注 |
|---|---|---|
| `lines: string[]` | useLogBuffer | 私有；不暴露 setter，只能通过 `loadTail` / `loadIncremental` / `clear` 修改 |
| `currentOffset` | useLogBuffer | 完全私有 |
| `kindEpoch` | useLogBuffer | 完全私有 |
| `parsedLines` | useLogBuffer | computed memoized |
| `lastUpdatedAt` | useLogBuffer | ms epoch；工具条格式化为 HH:MM:SS |
| `firstLoading` / `firstLoadError` | useLogBuffer | 仅首次 loadTail 期间生效 |
| `consecutiveFailCount` / `lastError` | useLogBuffer | polling 状态 |
| `autoRefresh` | useLogBuffer | polling 开关；切 kind 时由 buffer 内部强制 false |
| `query` / `visibleLines` | useLogSearch | 与 caseSensitive 联动 |
| `activeLevels` | useLogLevelFilter | 默认全 6 等级 |
| `enabled` / `paused` | useFollowTail | 状态机状态 |
| `wrap` / `height` / `fontSize` / `followTail` / `caseSensitive` | useLogPrefs | localStorage 持久；BC-13 降级内存 |
| `fullscreenOpen` | LogViewer.vue 内部 ref | UI-only，不持久化 |

### 5.2 watch 清单

```ts
// LogViewer.vue
watch(() => props.kind, () => {
  // BC-4: 清缓冲 + reset polling/autoRefresh，但保留偏好
  buf.stopPolling()
  buf.setAutoRefresh(false)
  buf.clear()
  void buf.loadTail()
})

watch(() => buf.lines.value.length, () => {
  follow.onNewLines()
}, { flush: 'post' })

watch(prefs.followTail, (v) => follow.toggle(v))
```

### 5.3 timer / cleanup

- `useLogBuffer` 内部 `pollingTimer = setInterval(...)`；`stopPolling()` 调 `clearInterval`。
- `onUnmounted` (LogViewer)：`buf.stopPolling()` + `prefs.flush()`。
- 全屏 Modal 打开 / 关闭过程中 timer 不重置（BC-10 验证：父 LogViewer 一直存在，Modal 关闭不卸载 buffer）。

### 5.4 复制语义（AC-6）

```ts
// LogViewer.vue
async function onCopy() {
  const text = search.visibleLines.value
    .map(v => v.parsed.raw)         // 原始 raw，不带行号、不带 HTML
    .join('\n')
  try {
    await navigator.clipboard.writeText(text)
    message.success('已复制到剪贴板')
  } catch {
    // execCommand 降级
    const ta = document.createElement('textarea')
    ta.value = text
    document.body.appendChild(ta)
    ta.select()
    try {
      const ok = document.execCommand('copy')
      message[ok ? 'success' : 'error'](ok ? '已复制到剪贴板' : '复制失败')
    } finally {
      document.body.removeChild(ta)
    }
  }
}
```

---

## §6 假设（开放接受 stage 4 验证）

| ID | 假设 | 风险 / 失败时影响 | 后备 |
|---|---|---|---|
| **A-1** | frp 日志单行格式以 `YYYY/MM/DD HH:MM:SS [I/W/E/D/T]` 为主，少数环境为 `YYYY-MM-DD HH:MM:SS [LEVEL]`。详情参见 §3.6.1 单条 regex 双格式 OR。 | regex 不匹配 → 该行降级 PLAIN，仅失去着色 / 分块，**不会**崩；最坏情况整页全 PLAIN（=旧 UX 不带高亮，仍可用） | dev 阶段抓真实 `logs/frpc.log` / `logs/frps.log` 几行样本，必要时调宽 regex；该 regex 集中在 `parseLogLine.ts` 一处 |
| **A-2** | Naive UI `useThemeVars()` 返回的 ComputedRef 在 `n-config-provider :theme` 切换时自动重新计算，绑定到 CSS 变量后子节点立即跟随。 | 如不重新计算 → 主题切换需用户刷新页面（AC-13 失败）| 在 LogViewer 加 `watch(() => themeVars.value, ...)` 显式触发；或改用 `:class="{ 'theme-dark': ... }"` 双 class 方案 |
| **A-3** | `localStorage` 在生产环境（Vite build → embed.FS → 单二进制）正常可用；隐私模式 / 无痕模式 / Safari ITP 是少数 | 概率低；BC-13 已有降级路径 | 无；BC-13 已覆盖 |
| **A-4** | `document.execCommand('copy')` 在主流浏览器仍可用作 clipboard API 不可用时的兜底（虽然已 deprecated 但未移除） | 极端环境两路径都失败 → message.error("复制失败")，不崩溃 | 无；用户可手工选择复制 |
| **A-5** | `navigator.clipboard.writeText` 在 HTTP 非 secure context 下可能 reject；项目运行在 localhost 时 Chromium 视为 secure，但远程 HTTP 部署可能失败 | clipboard 失败 → execCommand 降级；用户感知一致 | 见 onCopy 实现 |
| **A-6** | `n-modal preset="card"` 的 `:style` 在 Naive UI 2.x 支持 `width / height` 字符串覆盖默认尺寸 | 如不支持 → fullscreen Modal 显示为默认 600 px 宽 | 改用 scoped CSS `:deep(.n-card)` 覆盖（§3.5 已选此路径） |

---

## §7 风险 / 替代方案（决策矩阵）

### 决策矩阵 D-1：组件拆分粒度

| 选项 | 子组件数 | 单文件复杂度 | 测试可达性 | 重用价值 |
|---|---|---|---|---|
| (a) 单文件 LogViewer.vue | 1 | 高（>200 行违红线）| 低（mount 全量必要）| 低 |
| (b) Toolbar + List 二拆 | 3 | 中 | 中（搜索 / 跟随仍 mount） | 中 |
| **(c) Toolbar + List + Line + Modal 四拆（采用）** | 5 | 低（每个 < 200） | 高（LogLine 可独立 mount 测主题色）| 高 |
| (d) 极致原子（每个开关一个组件） | 10+ | 极低 | 极高 | 过度抽象 |

**决策：(c)** 4 子组件 + 1 壳。理由：(c) 是"刚好满足红线 + 让 LogLine 着色逻辑可独立测"的最小拆法；(d) 过度工程；(a)/(b) 违红线或测试可达性差。

### 决策矩阵 D-2：搜索算法

| 选项 | 包体增量 | 性能 | 心智负担 |
|---|---|---|---|
| **(a) String.indexOf 循环（采用）** | 0 | 500 行 < 5 ms | 极低 |
| (b) regex `new RegExp(escape(q), 'gi')` | 0 | ~等价 | escape 易写错 |
| (c) fuse.js 模糊匹配 | +30 KB gzip | <5 ms | 引入新依赖（NFR-5 红线） |

**决策：(a)**。理由：NFR-3 / NFR-5；(b) regex escape 是已知陷阱（注入 `*` `(` 等会 throw），(a) 把这类风险根除。

### 决策矩阵 D-3：跟随尾部"距底阈值"

| 阈值 | 误触发 | 用户体感 |
|---|---|---|
| 0 px | 经常（用户滚 1 px 即 paused）| 灾难 |
| **32 px（采用）** | 罕见 | 自然 |
| 100 px | 几乎无 | 但用户在"接近底部"时已应 paused 不滚动 |

**决策：32 px**。1-2 行高度，既不误触发（鼠标滚轮最小步进通常 100 px+），又能在用户主动滚开 1 屏后稳定 paused。01_REQUIREMENT §2.2 §11 已锁定。

### 决策矩阵 D-4：localStorage 失败的 BC-13 降级

| 选项 | 实现复杂度 | 用户感知 |
|---|---|---|
| **(a) 内存 Map 静默降级（采用）** | 低 | 偏好仅 session 内保留；下次刷新丢；不弹消息 |
| (b) 弹 warning 提示用户检查 | 低 | 噪声大；隐私模式用户每页加载都看到 |
| (c) 完全禁用持久化相关 UI | 中 | 用户体验断崖 |

**决策：(a)**。理由：BC-13 已锁定 "不报错；不弹 message"；(b) 在每个隐私窗口都骚扰用户。

### 决策矩阵 D-5：parsedLines memoization

| 选项 | 复杂度 | 性能 |
|---|---|---|
| 每次 computed 重新 parse 全部 500 行 | 低 | 每帧 ~500 × regex ≈ 2-3 ms |
| **WeakMap / Map<raw, ParsedLogLine> 缓存（采用）** | 中 | 仅新增行 parse |
| LRU + size 上限 | 高 | 过度 |

**决策：Map 缓存**。500 行上限天然限制 Map 大小；slice(-500) 后旧 key 失活，GC 友好。

---

## §8 测试策略

### 8.1 Vitest mount 级（核心；insight L29 mock 范式）

**全部 mount 测试文件顶端必须用 importOriginal + 6 方法 stub 模式**（参考 `web/src/components/__tests__/ProxyForm.spec.ts:11-24`）：

```ts
vi.mock('naive-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useMessage: () => ({
      error: vi.fn(), success: vi.fn(), warning: vi.fn(),
      info: vi.fn(), loading: vi.fn(), destroyAll: vi.fn(),
    }),
  }
})
```

### 8.2 AC 覆盖映射

| AC | 描述 | 验证方式 | 测试文件 |
|---|---|---|---|
| AC-1 | ERROR 行 class 含 `level-error` + 主题色 | mount LogLine + 断言 class + computed style | `LogViewer.spec.ts` + `parseLogLine.spec.ts` |
| AC-2 | 搜索过滤 + caseSensitive 切换 | composable 单测 `useLogSearch` | `useLogSearch.spec.ts` |
| AC-3 | 等级多选过滤 | composable 单测 | `LogViewer.spec.ts`（mount 级）|
| AC-4 | 跟随尾部 + push → scrollTop = scrollHeight - clientHeight (±1 px) | mount + 模拟 scrollEl 尺寸 + 触发 onNewLines | `useFollowTail.spec.ts` |
| AC-5 | 用户上滚 → paused + 提示条 | mount + DOM scroll event dispatch | `useFollowTail.spec.ts` |
| AC-6 | 复制按钮 → clipboard.writeText 收到拼接字符串 | mount + spy navigator.clipboard | `LogViewer.spec.ts` |
| AC-7 | 清屏 → lines = [] + 后端 0 调用 | mount + mock api + spy call count | `useLogBuffer.spec.ts` + `LogViewer.spec.ts` |
| AC-8 | 折行开关 + localStorage 同步 | composable 单测 `useLogPrefs` | `useLogPrefs.spec.ts` |
| AC-9 | 高度档 = 800 → max-height computed = 800 px | mount + getComputedStyle | `LogViewer.spec.ts` |
| AC-10 | 全屏 Modal 显示 / 关闭 + 缓冲不丢 | mount + setProps fullscreenOpen | `LogViewer.spec.ts` |
| AC-11 | 切 kind → 缓冲清 + autoRefresh false + 偏好保留 | mount + setProps kind | `LogViewer.spec.ts` |
| AC-12 | 连续 3 次 reject → polling clear + message.error + 小红点 | mount + mock api reject | `useLogBuffer.spec.ts` |
| AC-13 | 暗 / 亮主题背景色不同 | mount × 2（不同 theme provider）| `LogViewer.spec.ts` |
| AC-14 | 2000 字符单行 + pre-wrap 不溢出 | mount + DOM rect | `LogViewer.spec.ts` |
| AC-15 | 空态文案 + 行号列不渲染 | mount | `LogViewer.spec.ts` |
| AC-16 | 首次 loadTail 失败 → 重试按钮 + 再次调用 | mount + mock reject once | `LogViewer.spec.ts` |
| AC-17 | bundle 增量 < 50 KB | scripts/verify_all build size diff | QA stage 6 手工 |

### 8.3 不测的（per insight L30 / L37）

- 不为 `useLogBuffer` / `useLogPrefs` 的 mock 版本写"mock-only" spec（即不写"只验证 mock 被调"的零 adversarial spec）。
- Playwright e2e 不写——本任务纯 UI 改造，无新路由 / 无新后端契约 / 现有 e2e `03-dashboard.spec.ts` 已覆盖 LogViewer 渲染 smoke。如 stage 6 QA 认为需要补 e2e，单独追加。

### 8.4 Adversarial 候选（stage 6 QA 必须挑 ≥ 1 条）

- **ADV-A**：fixture 含 1 行 `<script>alert(1)</script>`，搜索关键字 `<script>` → 命中行 `<mark>` 包裹 `&lt;script&gt;`，**不**生成 `<script>` tag（NFR-7）。
- **ADV-B**：localStorage `setItem` 强 throw（quota）→ `useLogPrefs` setter 不崩 + UI 仍切换（BC-13）。
- **ADV-C**：mock `apiGetLogsIncremental` 3 次 reject → polling timer 必须被 clearInterval + `autoRefresh` 必须切 false + message.error 仅调一次（不是 3 次）。
- **ADV-D**：kindEpoch race —— mock 在 kind = frpc 启动 incremental 后 500 ms 才 resolve；中途切到 frps；frpc 响应到达后**不应**被 append 到 frps 的缓冲。

---

## §9 不在范围

延续 01_REQUIREMENT_ANALYSIS §3，本设计**额外**明确：

1. **虚拟滚动**：不实现；500 行原生 DOM 足够（NFR-1/2）。未来 > 2000 行可考虑 IntersectionObserver 手动可见性裁剪。
2. **多 kind 并排视图**：不做。
3. **自定义主题 token / 色板**：不做（Q-d 决策）。
4. **后端 API 协议 / WebSocket 推流**：不动（OOS-1 / OOS-4）。
5. **导出 .log 文件**：不做（Q-f / OOS-6）。
6. **时区切换 / timestamp 本地化**：不做（OOS-8）；保留后端原文。
7. **regex 搜索**：不做（Q-b）。
8. **后台节流（visibilityState）**：不做（BC-11 / OOS-11）。

---

## §10 红线 self-check

- [x] **无 inline `style="..."` 用于布局**（NFR-4）：唯一例外是 LogList max-height（动态 px），SA 决策（§3.3）改为 CSS 变量 `--log-list-height`，仍是 inline 但仅 1 个 CSS variable 设值；FullscreenLogModal 的 95vw / 90vh 走 scoped CSS `:deep(.n-card)`。Stage 5 reviewer 关注点：`grep style=` 应只在 LogList 根 div 上看到 1 处 CSS var setter，且 justified。
- [x] **单 SFC < 200 行**：LogViewer ≈ 150；Toolbar ≈ 140；List ≈ 120；Line < 80；Modal < 60。全部留余量。
- [x] **无新 npm 依赖**（NFR-5）：纯 Vue / Naive UI / Vitest 现有栈。
- [x] **无重型库**（NFR-3 / xterm.js / monaco / virtual scroll / fuse.js 全 0）。
- [x] **主题 token 化**（2.5 §24-25 / AC-13）：100% 走 `useThemeVars()` + CSS 变量。
- [x] **中文 UI**（NFR-6）：工具条文案、空态、错误、message 全中文。
- [x] **XSS escape**（NFR-7）：LogLine `renderedMessage` 先 escape 后 mark，单元测试覆盖 ADV-A。
- [x] **a11y**（NFR-8）：搜索框 / 开关 / 按钮 / 选择器全部走 Naive UI 默认语义；自定义元素加 `aria-label`（如行号 `aria-hidden`）。
- [x] **localStorage 降级**（NFR-9 / BC-13）：useLogPrefs 单点封装。
- [x] **不删测试**（红线 .harness/rules/00-core.md §3）：本任务新增测试，老测试 0 移除。
- [x] **不改上游文档**（红线 §2）：01_REQUIREMENT_ANALYSIS 不动；如发现缺口由 PM 回退。
- [x] **不动后端 / DB**（分区红线）：100% `web/**`。
- [x] **insight L29 mock 模式**：全部 mount spec 使用 importOriginal + 6 方法 stub。

---

## §11 Partition assignment

**单分区：dev-frontend**。所有改动在 `web/**` 内。无 dev-backend / dev-db 协同需求。

### 11.1 文件 → 分区映射

| 文件 | 分区 | 新 / 改 | 依赖 |
|---|---|---|---|
| `web/src/components/LogViewer.vue` | dev-frontend | 改（重写）| 依赖下列 13 个新文件 |
| `web/src/components/log/LogToolbar.vue` | dev-frontend | 新 | — |
| `web/src/components/log/LogList.vue` | dev-frontend | 新 | 依赖 LogLine.vue |
| `web/src/components/log/LogLine.vue` | dev-frontend | 新 | 依赖 parseLogLine.ts |
| `web/src/components/log/FullscreenLogModal.vue` | dev-frontend | 新 | 依赖 LogList.vue |
| `web/src/composables/log/useLogBuffer.ts` | dev-frontend | 新 | 依赖 parseLogLine.ts + `web/src/api/logs.ts`（已存在不改）|
| `web/src/composables/log/useLogSearch.ts` | dev-frontend | 新 | 依赖 parseLogLine.ts 的类型 |
| `web/src/composables/log/useLogLevelFilter.ts` | dev-frontend | 新 | 依赖 parseLogLine.ts 的类型 |
| `web/src/composables/log/useFollowTail.ts` | dev-frontend | 新 | — |
| `web/src/composables/log/useLogPrefs.ts` | dev-frontend | 新 | — |
| `web/src/composables/log/parseLogLine.ts` | dev-frontend | 新 | — |
| `web/src/components/__tests__/*.spec.ts` × 6 | dev-frontend | 新 | 依赖被测模块 |
| `docs/dev-map.md` | dev-frontend | 改（小）| — |

### 11.2 实现顺序建议（单分区内部依赖序）

1. `parseLogLine.ts` + 单测（叶节点，无依赖）
2. `useLogPrefs.ts` + 单测（叶节点）
3. `useLogBuffer.ts` + 单测（依赖 parseLogLine）
4. `useLogSearch.ts` + 单测
5. `useLogLevelFilter.ts`（trivial，可在第 4 步同批）
6. `useFollowTail.ts` + 单测
7. `LogLine.vue`
8. `LogList.vue`
9. `LogToolbar.vue`
10. `FullscreenLogModal.vue`
11. `LogViewer.vue`（壳，编排全部）
12. `LogViewer.spec.ts`（mount 集成）
13. `docs/dev-map.md`
14. `verify_all` PASS → 完成 04_DEVELOPMENT.md

### 11.3 并行性

单分区单 dev；无并行；上述 14 步严格顺序执行。

---

## §12 Reuse audit

| 需求 | 现有代码 | 文件路径 | 决策 |
|---|---|---|---|
| API 客户端 | `apiGetLogsTail` / `apiGetLogsIncremental` | `web/src/api/logs.ts` | 100% 复用，不动 |
| useMessage stub 测试范式 | T-032 importOriginal + 6 方法 mock | `web/src/components/__tests__/ProxyForm.spec.ts:11-24` | 复用此 idiom |
| `NConfigProvider` + `NMessageProvider` 包裹 | App.vue 已就位（insight L9）| `web/src/App.vue:1-12` | 确认就位，无需改 |
| composable 拆分范式 | `useProxyForm` / `usePortPresets` / `statusUtils` | `web/src/composables/*` | 沿用单文件单 composable 模式；新建 `composables/log/` 子目录归集 |
| 中文 UI label / message | 全项目惯例 | `web/src/components/**/*.vue` | 沿用 |
| Naive UI 组件 | `NSpace / NText / NSwitch / NButton / NCode` 已在 LogViewer 用 | `web/src/components/LogViewer.vue:21` | 扩展引入 `NInput / NSelect / NSlider / NModal / NTooltip / NSpin` |
| 主题 token 读取 | （无）项目尚无 `useThemeVars` 使用案例 | — | 新引入；本任务首次使用 |
| localStorage 持久化 | （无）项目尚无 localStorage 用例 | — | 新引入；single point 在 useLogPrefs |

---

## §13 风险 / 回滚

### 13.1 风险

| ID | 风险 | 概率 | 影响 | 缓解 |
|---|---|---|---|---|
| R-1 | regex 不匹配真实 frp 日志 → 全 PLAIN，等级着色形同虚设 | 中 | 中（仍可用但失去主特性） | A-1 + dev 阶段抓真实样本验证；regex 集中在 1 处便于调 |
| R-2 | useThemeVars 不响应 theme 切换（A-2 失败）| 低 | 中（需刷新页面）| 加 watch + 显式 trigger；或双 class 方案 |
| R-3 | LogLine v-html + escape 顺序写错 → XSS | 极低 | 高（NFR-7）| ADV-A 强测；先 escape 后 mark 顺序在 §3.4 锁死 |
| R-4 | onUnmounted 漏 stopPolling → BC-10 timer 泄漏 | 低 | 中（多个 setInterval 累积）| Vitest mount + unmount 断言 `clearInterval` 调用次数 |
| R-5 | localStorage quota throw 让 setter 报错 | 极低 | 低（BC-13）| try-catch 全包；ADV-B 验证 |
| R-6 | bundle 超 50 KB（NFR-3 / AC-17）| 低 | 中 | 不引依赖；verify_all 测；超了再删 PLAIN 分类等非核心 |
| R-7 | NFR-2 500 行 join + render > 50 ms long task | 低 | 中 | parsedLines memoization §3.6.2；不在热路径做 regex |
| R-8 | FullscreenLogModal 关闭 → LogList 重 mount → 滚动位置丢 | 中 | 低（小 UX 瑕疵）| 设计上接受；OOS 第二阶段优化（不阻塞本期）|

### 13.2 回滚

本任务无 schema / API 变更，回滚 = 单次 git revert commit。无数据 / 配置迁移。

---

## §14 Verdict

**READY FOR GATE REVIEW**

- 上游 01_REQUIREMENT_ANALYSIS.md = READY ✅
- 8 个 Open questions 全部已就地决策（01 §8）✅
- 26 in-scope 行为、13 BC、17 AC、9 NFR 在本设计中均有对应实现锚点 ✅
- 分区单一（dev-frontend），无跨分区协同 ✅
- 红线 self-check（§10）全过 ✅
- 决策矩阵（§7）覆盖 5 个非平凡选项 ✅
- 测试策略（§8）AC 1:1 映射 + ADV 候选 ≥ 4 条 ✅
- Reuse audit（§12）非空，证 SA 读过现有代码 ✅
- 假设（§6）显式列出 6 条，stage 4 dev 可校验 ✅

下一阶段：**Stage 3 Gate Reviewer** 审本设计；APPROVED 后由 PM 派 dev-frontend 单分区进入 Stage 4。

---

_由 Solution Architect 写于 2026-05-24，PM 全权授权下纯前端单分区设计。_
