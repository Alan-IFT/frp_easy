# 02_SOLUTION_DESIGN — T-042 proxy-runtime-status-merge

> Stage 2 / Solution Architect · 2026-05-28
>
> 依据 01_REQUIREMENT_ANALYSIS.md。本任务为前端为主（dev-frontend 分区），无后端 / openapi 改动。

## 1. 高层设计

**形态**：Proxies.vue 是已有 SFC；在其 setup 中新挂一份 `useServerRuntime()` 实例 → 用 computed 把 `runtime.proxies.value.proxies` 摊平为 `Map<name, ServerRuntimeProxyStatus>` → 在既有 `columns` 数组**尾部追加** 2 列（运行状态 / 流量）。其余渲染 / 表单 / CRUD 代码原封不动。

**关键约束**：
- 单向数据流：Proxies.vue **只读** runtime ref，**不**通过 v-model 绑回（继承 T-032 范式）
- 单 polling 实例：每个 mount 仅一个 `useServerRuntime()` 实例（与 ServerMonitor.vue 各自一份；本任务范围内不引入跨页共享 polling，避免拓扑复杂化）
- 降级：runtime 任何失败 → 列渲染灰点 + tooltip 文案 "frps 监控不可用"；CRUD 通路零关联（既有 `proxiesStore.fetchProxies()` 等不受影响）

## 2. 文件改动清单

| 文件 | 动作 | 行数估算 |
|---|---|---|
| `web/src/utils/format.ts` | NEW · 导出 `formatBytes(n)` + `formatTime(s)`（字节级搬运 T-041 ServerMonitor.vue 内联实现） | ~30 |
| `web/src/utils/proxyStatus.ts` | NEW · 导出 `getProxyStatusTag(raw)` + `STATUS_LABEL` 常量 | ~30 |
| `web/src/utils/__tests__/format.spec.ts` | NEW · 边界值 unit test | ~80 |
| `web/src/utils/__tests__/proxyStatus.spec.ts` | NEW · 大小写防御 unit test | ~50 |
| `web/src/pages/Proxies.vue` | MODIFIED · setup 加 useServerRuntime + runtimeMap computed；columns 尾部加 2 列；start()/refresh() 在 onMounted 调用 | +60 / -0 |
| `web/src/pages/__tests__/Proxies.spec.ts` | NEW · mount × 多态 + 反向构造 + 降级 | ~250 |
| `web/src/pages/__tests__/qa_t042_adversarial.spec.ts` | NEW · QA 反向构造守门（与 Proxies.spec.ts 互补：聚焦"边界回退后不退化") | ~150 |
| `web/src/pages/ServerMonitor.vue` | MODIFIED · 删 inline formatBytes/formatTime，import 自 utils；columns 状态 render 切到 getProxyStatusTag | +10 / -25（净减） |
| `docs/dev-map.md` | MODIFIED · "可复用工具"段 +2 行；Proxies.vue 备注追加 | +3 / -1 |

**净生产代码改动**：5 个 .ts/.vue 文件（含 2 新 utils）
**净测试代码改动**：4 个 .spec.ts 文件

## 3. 模块详细设计

### 3.1 `web/src/utils/format.ts`

```ts
// 字节友好单位格式化。从 T-041 ServerMonitor.vue 内联搬运，零算法变更。
// 共享方：ServerMonitor.vue（总流量 + 单 proxy 今日流量）+ Proxies.vue（T-042 运行态列）

export function formatBytes(n: number | undefined | null): string {
  if (n === undefined || n === null || Number.isNaN(n)) return '—'
  if (n === 0) return '0 B'
  if (n < 0) return '—'  // 边界：负数兜底（不会出现但防御）
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let v = n
  let u = 0
  while (v >= 1024 && u < units.length - 1) {
    v /= 1024
    u++
  }
  const s = u === 0 ? `${v}` : v.toFixed(1).replace(/\.0$/, '')
  return `${s} ${units[u]}`
}

// frps 上游 "0001-01-01 00:00:00" / 空字符串 → 友好 "—"
export function formatTime(s: string | undefined | null): string {
  if (!s) return '—'
  if (s.startsWith('0001-')) return '—'
  return s
}
```

**决策**：负数 → `'—'` 是新增防御行为（T-041 inline 实现不处理负数会进入 while 循环死锁式 0 出口）。这是**修正一个潜在 bug**而非引入回归——下游单测 AC-7 显式覆盖。ServerMonitor 既有用例无负数输入故不破坏。

### 3.2 `web/src/utils/proxyStatus.ts`

```ts
// proxy runtime status → 视觉/文案映射。从 T-041 ServerMonitor.vue 内联 columns render 搬运。
// 共享方：ServerMonitor.vue 状态列 + Proxies.vue 运行态列（T-042）

import type { TagProps } from 'naive-ui'

export type ProxyStatusTagType = NonNullable<TagProps['type']>  // 'success' | 'default' | 'error' | ...

export interface ProxyStatusVisual {
  /** 给 NTag 的 type prop */
  type: ProxyStatusTagType
  /** 中文展示文本 */
  text: string
  /** 圆点颜色（语义化色，可用作内联 style 兜底） */
  dotColor: string
  /** 是否被识别为 "在线" 状态（供 tooltip 文案分支） */
  online: boolean
}

const COLOR_SUCCESS = '#18a058'  // naive-ui 默认 success
const COLOR_DEFAULT = '#999999'  // 灰
const COLOR_ERROR = '#d03050'    // naive-ui 默认 error

export function getProxyStatusTag(raw: string | undefined | null): ProxyStatusVisual {
  const lower = (raw ?? '').toLowerCase()
  if (lower === 'online') {
    return { type: 'success', text: '在线', dotColor: COLOR_SUCCESS, online: true }
  }
  if (lower === 'offline' || lower === '') {
    return { type: 'default', text: '离线', dotColor: COLOR_DEFAULT, online: false }
  }
  return { type: 'error', text: raw || '未知', dotColor: COLOR_ERROR, online: false }
}
```

**决策**：
- 大小写防御沿用 T-041 GR C-5
- 空字符串 → 'offline' 而不是 'error'（与"runtime 无此 proxy 名"语义合并：runtime 没返回 → status 在 Proxies.vue 内默认为空 → 显示离线）
- 颜色硬编码而不是 useThemeVars——utils 是无 setup 上下文的纯函数，调用方可在模板里覆盖

### 3.3 `web/src/pages/Proxies.vue` setup 扩展

新增段落（在既有 store 初始化之后，columns 定义之前）：

```ts
import { useServerRuntime } from '../composables/useServerRuntime'
import { formatBytes } from '../utils/format'
import { getProxyStatusTag } from '../utils/proxyStatus'
import type { ServerRuntimeProxyStatus } from '../types'

// T-042：运行态轮询（5s，与 ServerMonitor 同节拍；onUnmounted 自清）
const runtime = useServerRuntime(5000)

// runtime.proxies.value.proxies 是 Record<type, Status[]>；用 Map<name, Status> 加速行内查找
const runtimeMap = computed<Map<string, ServerRuntimeProxyStatus>>(() => {
  const m = new Map<string, ServerRuntimeProxyStatus>()
  const buckets = runtime.proxies.value?.proxies ?? {}
  for (const t of Object.keys(buckets)) {
    for (const r of buckets[t] ?? []) {
      m.set(r.name, r)
    }
  }
  return m
})

// 降级判定：runtime 完全没数据 + 有 error → 监控不可用
const runtimeUnavailable = computed(
  () => runtime.proxies.value === null && runtime.error.value !== null,
)
```

新增 2 列（追加到 `columns` 数组尾部，**保留**既有"操作"列位置 → 实际插入在"启用"之后、"操作"之前）：

```ts
{
  title: '运行状态',
  key: 'runtimeStatus',
  render: (row) => {
    if (runtimeUnavailable.value) {
      const vis = getProxyStatusTag(null)
      return h(NTooltip, { trigger: 'hover' }, {
        trigger: () => h(NTag, { type: vis.type, size: 'small', round: true },
          { default: () => '监控不可用' }),
        default: () => 'frps 未运行 / 监控暂不可达',
      })
    }
    const r = runtimeMap.value.get(row.name)
    const vis = getProxyStatusTag(r?.status)
    const lastStart = formatTime(r?.lastStartTime)
    const tooltipText = r
      ? `状态：${vis.text}\n上次启动：${lastStart}\n当前连接：${r.curConns ?? 0}`
      : '该 proxy 未在 frps 端注册（离线）'
    return h(NTooltip, { trigger: 'hover', style: 'white-space: pre-line' }, {
      trigger: () => h(NTag, { type: vis.type, size: 'small', round: true },
        { default: () => vis.text }),
      default: () => tooltipText,
    })
  },
},
{
  title: '流量（入 / 出）',
  key: 'runtimeTraffic',
  render: (row) => {
    if (runtimeUnavailable.value) return '—'
    const r = runtimeMap.value.get(row.name)
    if (!r) return '—'
    const text = `${formatBytes(r.todayTrafficIn)} / ${formatBytes(r.todayTrafficOut)}`
    return h(NTooltip, { trigger: 'hover' }, {
      trigger: () => text,
      default: () => `当前连接：${r.curConns ?? 0}`,
    })
  },
},
```

`onMounted` 扩展：

```ts
onMounted(() => {
  void proxiesStore.fetchProxies()
  // T-042：启动 runtime polling；不 await（让配置表加载先返回）
  runtime.start()
  void runtime.refresh()
})
```

**注**：不需要手工 onUnmounted —— useServerRuntime 自带（T-041 insight L11）。

### 3.4 `web/src/pages/ServerMonitor.vue` refactor

- 删除：line 242-254 inline `formatBytes`、line 257-261 inline `formatTime`
- 删除：line 286-289 inline status → type/text 三元逻辑
- 替换为：`import { formatBytes, formatTime } from '../utils/format'`、`import { getProxyStatusTag } from '../utils/proxyStatus'`
- columns 状态 render 段：
  ```ts
  render: (row) => {
    const vis = getProxyStatusTag(row.status)
    return h(NTag, { type: vis.type, size: 'small', round: true }, { default: () => vis.text })
  },
  ```
- `defineExpose.__testing` 保留 formatBytes / formatTime 引用（仍指向 utils 中的函数）→ 既有 spec.ts 用 `t.formatBytes(0)` 拿到的是 utils 函数实现，行为同源 → AC-9 保护

### 3.5 测试模块设计

#### `format.spec.ts`（AC-7）

```ts
describe('formatBytes', () => {
  it.each([
    [0, '0 B'],
    [1, '1 B'],
    [1023, '1023 B'],
    [1024, '1 KiB'],
    [1536, '1.5 KiB'],
    [1024 * 1024, '1 MiB'],
    [1024 * 1024 * 1024, '1 GiB'],
    [Number.MAX_SAFE_INTEGER, expect.stringContaining('PiB')],  // 上限：会被钳在 TiB
  ])('formatBytes(%j) === %j', (n, expected) => { ... })

  it('undefined → "—"', () => { ... })
  it('null → "—"', () => { ... })
  it('NaN → "—"', () => { ... })
  it('负数 → "—"', () => { ... })
})

describe('formatTime', () => {
  it('空字符串 → "—"', () => { ... })
  it('null → "—"', () => { ... })
  it('undefined → "—"', () => { ... })
  it('"0001-01-01 00:00:00" → "—"', () => { ... })
  it('"2025-01-15 10:23:45" → 原样返回', () => { ... })
})
```

#### `proxyStatus.spec.ts`（AC-8）

```ts
describe('getProxyStatusTag', () => {
  it('online → success / "在线" / online=true', () => { ... })
  it('Online（大写）→ success / "在线"', () => { ... })
  it('OFFLINE → default / "离线"', () => { ... })
  it('offline → default / "离线"', () => { ... })
  it('"" → default / "离线"（与"无此 proxy"统一）', () => { ... })
  it('null → default / "离线"', () => { ... })
  it('undefined → default / "离线"', () => { ... })
  it('"error" → error / 原文 "error"', () => { ... })
  it('"unknown_state" → error / 原文', () => { ... })
})
```

#### `Proxies.spec.ts`（AC-1 ~ AC-6 + AC-11）

```ts
// mount 框架同 ServerMonitor.spec.ts：vi.mock('naive-ui') importOriginal + 6 方法 stub
// vi.mock('../../api/serverRuntime')、vi.mock('../../api/proxies')

describe('Proxies.vue — runtime 列 happy path（AC-1 / AC-2 / AC-3）', () => {
  it('配置态 + runtime 都有 "ssh" tcp → "在线" + 流量文本', ...)
  it('表格中含 "1.5 KiB / 2.5 KiB" 流量', ...)
  it('tooltip 含 lastStartTime + curConns', ...)
})

describe('Proxies.vue — 反向构造（AC-4 / AC-5）', () => {
  it('AC-4：配置态有 "web" 但 runtime 无 → 该行运行状态 "离线"', ...)
  it('AC-5：runtime 有 "extra" 但配置态无 → 表格不出现 "extra" 行', ...)
})

describe('Proxies.vue — 降级（AC-6）', () => {
  it('apiGetServerRuntimeProxies reject → runtime 列全显 "监控不可用"', ...)
  it('降级下 store.fetchProxies 仍被调用 + 配置 CRUD spy 正常', ...)
})

describe('Proxies.vue — 既有 CRUD 通路零回归', () => {
  it('点 "新增规则" → showForm=true / formData 默认值', ...)
  it('点 "编辑" → editingProxy / showForm=true', ...)
  it('点 "删除" → showDeleteConfirm=true / deletingProxy 设置', ...)
})
```

#### `qa_t042_adversarial.spec.ts`

```ts
// QA stage 6 反向构造守门：
// ADV-1：runtime 503 → 灰点 + 监控不可用 → recover 后变绿点
// ADV-2：runtime status 大小写漂移（"Online" / "ONLINE" / "online"）→ 都归 success
// ADV-3：runtime 含同名不同 type proxy（理论上 frps 不会发生）→ Map 行为 last-wins 可接受
// ADV-4：runtime curConns 为 undefined / 0 / 负数 → tooltip 文案安全
// ADV-5：单向数据流 grep：spec 内 grep Proxies.vue 源码 forbid v-model:.*runtime
```

## 4. 数据流图（文字版）

```
                ┌──────────────────────────────────────────────┐
                │  Proxies.vue setup()                         │
                │                                              │
                │  proxiesStore.fetchProxies()  ───┐            │
                │           │                      │            │
                │           ▼                      │            │
                │     proxies: Proxy[]             │            │
                │                                  │            │
                │  useServerRuntime(5000)          │            │
                │           │                      │            │
                │           ▼                      │            │
                │     runtime.proxies (ref)        │            │
                │           │                      │            │
                │           ▼                      │            │
                │     runtimeMap (computed)        │            │
                │     Map<name, Status>            │            │
                │           │                      │            │
                │           └──┬───────────────────┘            │
                │              ▼                                │
                │     columns[i].render(row)                    │
                │     · lookup runtimeMap.get(row.name)         │
                │     · getProxyStatusTag(r?.status)            │
                │     · formatBytes(r?.todayTraffic*)           │
                └──────────────────────────────────────────────┘
                              │
                              ▼
                         n-data-table 渲染
```

- 实线箭头 = 数据流向（单向）
- 无反向箭头 = 无 v-model 绑回（insight L13 保护）

## 5. Partition assignment

- **dev-frontend**：本任务所有改动（utils + Proxies.vue + ServerMonitor.vue + spec.ts + dev-map.md 同步）

无后端 / DB / e2e 改动。本项目当前未拆分区 dev-* agents（实测 `ls .harness/agents/dev-*.md` = 0），按单 developer 模式派发。

## 6. 决策矩阵

| 决策点 | 选项 | 选 | 理由 |
|---|---|---|---|
| runtime 数据查找数据结构 | `find()` 遍历 / `Map` / `Record` | Map | N×M 查找按 row render 调用，Map.get 是 O(1) |
| utils 抽取位置 | `composables/` / `utils/` / `lib/` | `utils/` | 项目尚无 utils/ 目录，但纯函数（无 Vue setup 依赖）放 utils 更语义化；composables 留给"有响应式"的封装 |
| 状态色硬编码 vs themeVars | useThemeVars / 硬编码 hex | 硬编码 | utils 是纯函数无 setup 上下文；naive-ui 默认 success/error 色稳定；后续若主题切换需感知可走模板内 useThemeVars 覆盖 |
| polling 节拍 | 5s（同 ServerMonitor）/ 10s / 30s | 5s | 与 ServerMonitor 同步；UX 一致；T-041 已论证 5s 不超载 |
| 跨页共享单 polling 实例 | 引入 Pinia store 共享 / 各页各 instance | 各页各 instance | 跨页共享需重写 useServerRuntime 为单例 + 引入 ref count 复杂；本任务 ROI 不值；两实例并存对 frps 仅 2× 请求量可接受 |
| runtime 列位置 | 表格最右 / "操作"列前 / "启用"列前 | "操作"列前 | "操作"列保留在最右（用户惯性）；运行态紧跟"启用"语义连贯 |
| utils 单测覆盖度 | 仅快乐路径 / 边界全覆盖 | 边界全覆盖 | 用户原则"软件工程标准" + utils 是高复用面 |
| 降级 UI 表现 | 整行隐藏 / 全列灰 / banner | 全列灰 + tooltip | 不引入 banner（避免与 ServerMonitor 重复）；保持 CRUD 完整 |
| 抽 utils 后 ServerMonitor.vue 改动幅度 | 内部全 refactor / 仅替换 inline 函数 | 仅替换 | 最小风险 + AC-9 既有 spec 不改一行 |

## 7. 假设与待验证

- **A-1**：T-041 的 `useServerRuntime` 在两个 mount 并存时（ServerMonitor + Proxies）两份 polling 不互相干扰。**已验证**：composable 内部 timer / pausedByVisibility / userStoppedExplicitly 全部是 closure 局部变量，无 module-level 状态。
- **A-2**：Proxies.vue 加 runtime 列后 script 段纯逻辑行数仍 < 200。**预估**：现 88 行 setup + 新增 ~50 行 = ~138 行 < 200 ✓
- **A-3**：既有 `web/src/pages/__tests__/` 与 `web/src/components/__tests__/` 无 Proxies.spec.ts 文件。**已实测验证**（Glob 命中 0）。本任务**新建**该文件不踩既有 broken。
- **A-4**：抽 utils 不影响既有 ServerMonitor.spec.ts。**风险点**：spec 通过 `t.formatBytes(undefined) === '—'` 直接拿 setup 暴露的函数引用；utils 切换后 `t.formatBytes` 仍指向 utils 中同名函数，行为字节级一致 → 不破坏。

## 8. 给 GR / Dev / QA 的 handoff

- **GR**：核验 § 3.1 ~ § 3.4 是否完备 + § 6 决策矩阵是否合理 + § 7 假设是否被覆盖
- **Dev**：严格按 § 3.1 ~ § 3.5 实现；utils 字节级搬运（不优化）；ServerMonitor.vue 只允许 import 替换
- **QA**：覆盖 § 3.5 所列全部用例 + `## Adversarial tests` 裸标题（L41）

— end —
