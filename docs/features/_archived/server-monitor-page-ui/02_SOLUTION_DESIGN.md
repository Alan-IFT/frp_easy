# 02 — Solution Design · T-041 server-monitor-page-ui

> Stage 2 / 7。基于 01 的 FR / NFR / AC 给出"怎么做"。

## 1. 总览

```
┌────────────────────────────────────────────────────────┐
│ ServerMonitor.vue (page)                               │
│  ├─ ServerInfo 卡片（FR-1）                            │
│  ├─ 顶部状态条（FR-3：暂停 / 刷新 / 失败 banner）       │
│  └─ Proxies n-tabs（FR-2：按 type 表格）                │
│       使用 useServerRuntime composable                  │
└────────────────────────┬──────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────┐
│ useServerRuntime(intervalMs=5000)                    │
│  - setInterval polling                                │
│  - visibilitychange 自动暂停                          │
│  - epoch race（BC-4 / BC-5）                          │
│  - 3 次失败自动 stop（D-6）                            │
└────────────────────────┬─────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────┐
│ web/src/api/serverRuntime.ts                          │
│  apiGetServerRuntimeInfo / Proxies / Traffic(name)    │
└────────────────────────┬─────────────────────────────┘
                         │ axios apiClient (CSRF auto)
                         ▼
        GET /api/v1/server/runtime/{info,proxies,traffic/{name}}
                         │
                         ▼
        T-039 handlers_server_runtime.go
                         │
                         ▼
                  frps :7500 dashboard
```

## 2. 文件清单

### 2.1 新增

| 文件 | 行数预算 | 角色 |
|---|---|---|
| `web/src/api/serverRuntime.ts` | ~40 | API client（3 函数 + JSDoc） |
| `web/src/composables/useServerRuntime.ts` | ~180 | polling composable |
| `web/src/composables/__tests__/useServerRuntime.spec.ts` | ~280 | composable 单测（vi.useFakeTimers + visibilitychange） |
| `web/src/pages/ServerMonitor.vue` | ~250（其中 script 段纯逻辑 < 120 估，模板段 ~130） | 页面壳 |
| `web/src/pages/__tests__/ServerMonitor.spec.ts` | ~320 | mount × 3 态 + setProps + visibility |
| `web/src/pages/__tests__/qa_t041_adversarial.spec.ts`（QA stage 6） | ~200 | 4 adversarial 场景 |

### 2.2 修改

| 文件 | 改动 |
|---|---|
| `web/src/types.ts` | +4 接口（ServerRuntimeInfo / ServerRuntimeProxyStatus / ServerRuntimeProxiesResponse / ServerRuntimeTraffic） |
| `web/src/router.ts` | +1 route `server/monitor` |
| `web/src/components/AppLayout.vue` | menuOptions +1 项 + activeKey 计算兼容 |
| `docs/dev-map.md` | 同步 |

## 3. 详细设计

### 3.1 `web/src/types.ts` 扩展

接在既有 `FrpsConfig` 段之后，加新段：

```typescript
// ---------------------------------------------------------------
// T-041 server-monitor-page-ui：frps 运行态（消费 T-039 API）
// ---------------------------------------------------------------

export interface ServerRuntimeInfo {
  version?: string
  bindPort?: number
  kcpBindPort?: number
  quicBindPort?: number
  vhostHTTPPort?: number
  vhostHTTPSPort?: number
  subdomainHost?: string
  clientCounts: number
  curConns: number
  proxyTypeCount?: Record<string, number>
  totalTrafficIn?: number
  totalTrafficOut?: number
}

export interface ServerRuntimeProxyStatus {
  name: string
  type?: string
  /** "online" | "offline" | 兜底其它 → 红色 dot（D-3） */
  status?: string
  lastStartTime?: string
  lastCloseTime?: string
  todayTrafficIn?: number
  todayTrafficOut?: number
  curConns?: number
  clientVersion?: string
}

export interface ServerRuntimeProxiesResponse {
  /** key = type（tcp/udp/http/...）；value = 该类型 proxy 数组 */
  proxies: Record<string, ServerRuntimeProxyStatus[]>
  /** 上游某些 type 失败时按 type 收集错误文案；不存在的 type key 不出现 */
  errors?: Record<string, string>
}

export interface ServerRuntimeTraffic {
  name: string
  trafficIn: number[]
  trafficOut: number[]
}
```

### 3.2 `web/src/api/serverRuntime.ts` 实现

```typescript
import apiClient from './client'
import type {
  ServerRuntimeInfo,
  ServerRuntimeProxiesResponse,
  ServerRuntimeTraffic,
} from '../types'

/**
 * T-041 · 消费 T-039 GET /api/v1/server/runtime/info
 * 503：dashboard 未启用 / frps 不可达；502：上游凭据失效。
 */
export async function apiGetServerRuntimeInfo(): Promise<ServerRuntimeInfo> {
  const res = await apiClient.get<ServerRuntimeInfo>('/api/v1/server/runtime/info')
  return res.data
}

/**
 * T-041 · 消费 T-039 GET /api/v1/server/runtime/proxies
 * 聚合 N 个 type；部分 type 失败时整体仍 200，errors[type] 透传给 UI 分 tab 展示。
 */
export async function apiGetServerRuntimeProxies(): Promise<ServerRuntimeProxiesResponse> {
  const res = await apiClient.get<ServerRuntimeProxiesResponse>('/api/v1/server/runtime/proxies')
  return res.data
}

/**
 * T-041 · 消费 T-039 GET /api/v1/server/runtime/traffic/{name}
 * 单条 proxy 流量时间序列（in / out 数组）。本任务暂不消费，导出供 T-042 / 后续抽屉用。
 */
export async function apiGetServerRuntimeTraffic(name: string): Promise<ServerRuntimeTraffic> {
  const encoded = encodeURIComponent(name)
  const res = await apiClient.get<ServerRuntimeTraffic>(`/api/v1/server/runtime/traffic/${encoded}`)
  return res.data
}
```

### 3.3 `web/src/composables/useServerRuntime.ts`

```typescript
// T-041 / server-monitor-page-ui · 02 §3.3
// frps 运行态轮询 composable。范式参考 T-036 useLogBuffer（epoch race + 3 次失败自动停）+
// T-038 useServiceStatus polling 节奏。F-5.7：不在 mount 自动 start，由壳显式调 start()。

import { ref, onUnmounted, type Ref } from 'vue'
import {
  apiGetServerRuntimeInfo,
  apiGetServerRuntimeProxies,
} from '../api/serverRuntime'
import { extractErrorMessage } from '../api/client'
import type { ServerRuntimeInfo, ServerRuntimeProxiesResponse } from '../types'

export interface UseServerRuntimeOptions {
  intervalMs?: number
  /** 注入 visibility 检测，默认走 document.visibilityState（测试可注入 fake） */
  visibilityHidden?: () => boolean
  /** 注入 setInterval / clearInterval 让测试可用 vi.useFakeTimers */
  // 测试默认就走全局 setInterval，不需要 inject seam
}

export interface UseServerRuntimeReturn {
  info: Ref<ServerRuntimeInfo | null>
  proxies: Ref<ServerRuntimeProxiesResponse | null>
  isPolling: Ref<boolean>
  error: Ref<string | null>
  consecutiveFailCount: Ref<number>
  lastUpdated: Ref<number>
  /** F-5.2 启动 setInterval；幂等：重复调用不会创建多个 timer */
  start: () => void
  /** F-5.2 清除 setInterval；幂等 */
  stop: () => void
  /** F-5.5 立即拉一次（不依赖 timer）；epoch race 保护 */
  refresh: () => Promise<void>
  /** D-6 自动 stop 后用户点"重启轮询"按钮清零失败计数并重启 */
  restart: () => void
}

const DEFAULT_INTERVAL_MS = 5000
const MAX_FAIL = 3

export function useServerRuntime(
  intervalMs: number = DEFAULT_INTERVAL_MS,
  opts: UseServerRuntimeOptions = {},
): UseServerRuntimeReturn {
  const info = ref<ServerRuntimeInfo | null>(null)
  const proxies = ref<ServerRuntimeProxiesResponse | null>(null)
  const isPolling = ref<boolean>(false)
  const error = ref<string | null>(null)
  const consecutiveFailCount = ref<number>(0)
  const lastUpdated = ref<number>(0)
  // epoch 防止过期响应写组件（BC-4 / BC-5；T-036 范式）
  const epoch = ref<number>(0)

  let timer: ReturnType<typeof setInterval> | null = null
  let pausedByVisibility = false
  let userStoppedExplicitly = false  // BC-7：用户暂停后再切后台 / 切回不要恢复

  const isHidden = opts.visibilityHidden
    ?? (() => typeof document !== 'undefined' && document.hidden === true)

  async function refresh(): Promise<void> {
    const at = ++epoch.value
    try {
      const [i, p] = await Promise.all([
        apiGetServerRuntimeInfo(),
        apiGetServerRuntimeProxies(),
      ])
      if (at !== epoch.value) return  // 过期响应丢弃
      info.value = i
      proxies.value = p
      lastUpdated.value = Date.now()
      consecutiveFailCount.value = 0
      error.value = null
    } catch (e) {
      if (at !== epoch.value) return
      // F-5.6：保留上一次成功数据；只更新 error + 计数
      const msg = extractErrorMessage(e, '加载 frps 运行态失败')
      error.value = msg
      consecutiveFailCount.value++
      if (consecutiveFailCount.value >= MAX_FAIL) {
        // D-6：自动停 polling；用户须显式 restart()
        stopInternal(false)
      }
    }
  }

  function startInternal(): void {
    if (timer !== null) return  // 幂等
    isPolling.value = true
    timer = setInterval(() => {
      void refresh()
    }, intervalMs)
  }

  function stopInternal(setUserFlag: boolean): void {
    if (timer !== null) {
      clearInterval(timer)
      timer = null
    }
    isPolling.value = false
    if (setUserFlag) {
      userStoppedExplicitly = true
    }
  }

  function start(): void {
    userStoppedExplicitly = false
    pausedByVisibility = false
    startInternal()
  }

  function stop(): void {
    stopInternal(true)
  }

  function restart(): void {
    consecutiveFailCount.value = 0
    error.value = null
    start()
  }

  function onVisibilityChange(): void {
    if (userStoppedExplicitly) return  // BC-7：用户意图优先
    if (isHidden()) {
      if (timer !== null) {
        clearInterval(timer)
        timer = null
        isPolling.value = false
        pausedByVisibility = true
      }
    } else {
      if (pausedByVisibility) {
        pausedByVisibility = false
        startInternal()
        void refresh()  // 切回时立即拉一次（用户体验）
      }
    }
  }

  if (typeof document !== 'undefined') {
    document.addEventListener('visibilitychange', onVisibilityChange)
  }

  onUnmounted(() => {
    stopInternal(false)
    if (typeof document !== 'undefined') {
      document.removeEventListener('visibilitychange', onVisibilityChange)
    }
    // 自增 epoch 让 in-flight 响应被丢弃
    epoch.value++
  })

  return {
    info,
    proxies,
    isPolling,
    error,
    consecutiveFailCount,
    lastUpdated,
    start,
    stop,
    refresh,
    restart,
  }
}
```

**关键决策**：
- D-3.1（onUnmounted 调用位置）：composable 内调用 onUnmounted；这要求 useServerRuntime 必须在 setup() 同步路径调用。文档化在 JSDoc 头。
- D-3.2（epoch race）：模仿 T-036 useLogBuffer.ts。in-flight 请求在组件 unmount 后到达不再 write info / proxies ref。
- D-3.3（visibilitychange listener 内置）：composable 自管 listener，省调用方代码；onUnmounted 自动解绑。
- D-3.4（双 endpoint Promise.all）：info + proxies 一次轮询并发发出；二者独立失败时 catch 视为整体失败（保守）。
- D-3.5（不引入 cancel/abort）：5 s 间隔 << 单次响应超时（axios 默认 0 即无限，apiClient 实例无 timeout）；下次 refresh 触发时上次未完成 → epoch 比对丢弃。代码更简洁。
- D-3.6（userStoppedExplicitly flag）：满足 BC-7"用户暂停后切后台再切回不自动恢复"。

### 3.4 `web/src/pages/ServerMonitor.vue`

```vue
<template>
  <div class="server-monitor-root">
    <!-- 顶部状态条（FR-3） -->
    <n-space justify="space-between" align="center" style="margin-bottom: 16px">
      <n-space align="center" :size="8">
        <n-text strong style="font-size: 18px">服务端监控</n-text>
        <n-text depth="3" style="font-size: 13px">
          {{ lastUpdatedLabel }}
        </n-text>
      </n-space>
      <n-space align="center" :size="8">
        <n-button size="small" :disabled="isRefreshing" @click="onRefreshClick">
          立即刷新
        </n-button>
        <n-button size="small" :type="rt.isPolling.value ? 'default' : 'primary'" @click="onTogglePolling">
          {{ rt.isPolling.value ? '暂停轮询' : '恢复轮询' }}
        </n-button>
      </n-space>
    </n-space>

    <!-- 连续失败 banner（FR-3.4 / AC-11） -->
    <n-alert
      v-if="!rt.isPolling.value && rt.consecutiveFailCount.value >= 3"
      type="error"
      :show-icon="true"
      style="margin-bottom: 16px"
    >
      自动刷新已停止：连续 {{ rt.consecutiveFailCount.value }} 次拉取失败。
      <n-button size="tiny" type="error" tertiary @click="onRestartPolling" style="margin-left: 8px">
        重启轮询
      </n-button>
    </n-alert>

    <!-- 连接断开 banner（AC-6 / F-5.6 保留上一次数据 + 顶部红色提示） -->
    <n-alert
      v-else-if="rt.error.value && (rt.info.value !== null || rt.proxies.value !== null)"
      type="warning"
      :show-icon="true"
      style="margin-bottom: 16px"
    >
      连接断开：{{ rt.error.value }}（显示的是上一次成功数据）
    </n-alert>

    <!-- 首屏错误（AC-3 / AC-4 / AC-5） -->
    <n-result
      v-if="firstLoadFailed"
      status="error"
      title="无法加载 frps 运行态"
      :description="rt.error.value || ''"
      style="margin-top: 32px"
    >
      <template #footer>
        <n-space justify="center">
          <n-button @click="onRefreshClick">重试</n-button>
          <n-button v-if="goServerHint" type="primary" @click="goServerConfig">
            前往服务端配置
          </n-button>
        </n-space>
      </template>
    </n-result>

    <template v-else>
      <!-- ServerInfo 卡片（FR-1） -->
      <n-card title="服务器信息" size="small" style="margin-bottom: 16px">
        <n-skeleton v-if="firstLoading" text :repeat="3" />
        <n-grid v-else :cols="3" :x-gap="12" :y-gap="8" responsive="screen">
          <n-grid-item>
            <n-statistic label="frps 版本" :value="rt.info.value?.version || '—'" />
          </n-grid-item>
          <n-grid-item>
            <n-statistic label="客户端连接数" :value="rt.info.value?.clientCounts ?? 0" />
          </n-grid-item>
          <n-grid-item>
            <n-statistic label="当前连接数" :value="rt.info.value?.curConns ?? 0" />
          </n-grid-item>
          <n-grid-item>
            <n-statistic label="bindPort" :value="rt.info.value?.bindPort ?? '—'" />
          </n-grid-item>
          <n-grid-item>
            <n-statistic label="累计流量入" :value="formatBytes(rt.info.value?.totalTrafficIn)" />
          </n-grid-item>
          <n-grid-item>
            <n-statistic label="累计流量出" :value="formatBytes(rt.info.value?.totalTrafficOut)" />
          </n-grid-item>
        </n-grid>
      </n-card>

      <!-- Proxies n-tabs（FR-2） -->
      <n-card title="代理状态" size="small">
        <n-skeleton v-if="firstLoading" text :repeat="5" />
        <n-empty v-else-if="proxyTypesWithData.length === 0 && proxyTypesWithErrors.length === 0"
                 description="暂无连接的 proxy" />
        <n-tabs v-else type="line" animated v-model:value="activeType">
          <n-tab-pane
            v-for="t in allProxyTypes"
            :key="t"
            :name="t"
            :tab="`${t.toUpperCase()} (${(rt.proxies.value?.proxies[t]?.length) ?? 0})`"
          >
            <!-- 单 type 错误：显示文案而非空表 -->
            <n-alert v-if="rt.proxies.value?.errors?.[t]" type="error" :show-icon="false">
              {{ rt.proxies.value.errors[t] }}
            </n-alert>
            <n-empty v-else-if="!rt.proxies.value?.proxies[t]?.length"
                     :description="`暂无 ${t} 类型 proxy`" />
            <n-data-table
              v-else
              :columns="columns"
              :data="rt.proxies.value.proxies[t]"
              :row-key="(row: ServerRuntimeProxyStatus) => row.name"
              size="small"
              :bordered="false"
            />
          </n-tab-pane>
        </n-tabs>
      </n-card>
    </template>
  </div>
</template>

<script setup lang="ts">
// T-041 / server-monitor-page-ui · 02 §3.4
// frps 服务端运行态监控页（只读）。使用 useServerRuntime 5 s 轮询。

import { ref, computed, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import {
  NCard, NGrid, NGridItem, NStatistic, NTabs, NTabPane, NDataTable, NSpace,
  NText, NButton, NAlert, NSkeleton, NEmpty, NResult, NTag,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useServerRuntime } from '../composables/useServerRuntime'
import type { ServerRuntimeProxyStatus } from '../types'

const router = useRouter()
const rt = useServerRuntime(5000)

// FR-5.7：壳显式调 start + refresh（不在 composable 里自动）
onMounted(() => {
  rt.start()
  void rt.refresh()
})

const isRefreshing = ref(false)
const allKnownTypes = ['tcp', 'udp', 'http', 'https', 'stcp', 'sudp', 'xtcp'] as const

const proxyTypesWithData = computed<string[]>(() => {
  const p = rt.proxies.value?.proxies ?? {}
  return Object.keys(p).filter((t) => (p[t]?.length ?? 0) > 0)
})

const proxyTypesWithErrors = computed<string[]>(() => {
  const e = rt.proxies.value?.errors ?? {}
  return Object.keys(e)
})

// allProxyTypes：显示顺序固定（按 allKnownTypes 列表），但仅展示"有数据或有错误"的 type。
const allProxyTypes = computed<string[]>(() => {
  const has = new Set([...proxyTypesWithData.value, ...proxyTypesWithErrors.value])
  return allKnownTypes.filter((t) => has.has(t))
})

const activeType = ref<string>('tcp')

// 首屏 loading（在 info / proxies 仍为 null 且没有 error 时）
const firstLoading = computed(
  () => rt.info.value === null && rt.proxies.value === null && rt.error.value === null,
)

// 首屏错误：从未成功过任何一次（info & proxies 全 null 但已 error）
const firstLoadFailed = computed(
  () => rt.info.value === null && rt.proxies.value === null && rt.error.value !== null,
)

// "前往服务端配置"按钮显示条件：错误文案含 "dashboard 未启用"
const goServerHint = computed(() => {
  const m = rt.error.value
  return typeof m === 'string' && (m.includes('dashboard 未启用') || m.includes('凭据'))
})

function goServerConfig(): void {
  void router.push('/server')
}

// lastUpdated 相对时间格式化（FR-3.1）
const tickRef = ref(0)  // 每秒 +1 强制 computed 重算
// 简化：不引入 setInterval 二号 timer；polling 5 s 已足够频；computed 依赖 lastUpdated 即跟随刷新
const lastUpdatedLabel = computed(() => {
  void tickRef.value  // dummy trigger（如未来需要更细粒度可挂 timer）
  const t = rt.lastUpdated.value
  if (!t) return '尚未刷新'
  const delta = Math.max(0, Date.now() - t)
  if (delta < 5_000) return '刚刚刷新'
  if (delta < 60_000) return `${Math.floor(delta / 1000)} 秒前刷新`
  if (delta < 3_600_000) return `${Math.floor(delta / 60_000)} 分钟前刷新`
  return `${Math.floor(delta / 3_600_000)} 小时前刷新`
})

function formatBytes(n: number | undefined): string {
  if (n === undefined || n === null || Number.isNaN(n)) return '—'
  if (n === 0) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let v = n
  let u = 0
  while (v >= 1024 && u < units.length - 1) {
    v /= 1024
    u++
  }
  // 整数显示整数；非整数保留 1 位
  const s = u === 0 ? `${v}` : v.toFixed(1).replace(/\.0$/, '')
  return `${s} ${units[u]}`
}

function formatTime(s: string | undefined): string {
  if (!s) return '—'
  // frps 上游返回的时间形如 "2025-01-15 10:23:45"；如 "0001-01-01 00:00:00" 视为空
  if (s.startsWith('0001-')) return '—'
  return s
}

// FR-2 表格列
const columns: DataTableColumns<ServerRuntimeProxyStatus> = [
  {
    title: '名称',
    key: 'name',
    render: (row) => row.name,
  },
  {
    title: '状态',
    key: 'status',
    render: (row) => {
      const s = row.status ?? ''
      // FR-2.4：online 绿 / offline 灰 / 其它 红（D-3）
      const type: 'success' | 'default' | 'error' =
        s === 'online' ? 'success' : s === 'offline' ? 'default' : 'error'
      const text = s === 'online' ? '在线' : s === 'offline' ? '离线' : (s || '未知')
      return h(NTag, { type, size: 'small', round: true }, { default: () => text })
    },
  },
  {
    title: '当前连接',
    key: 'curConns',
    render: (row) => row.curConns ?? 0,
  },
  {
    title: '今日流入',
    key: 'todayTrafficIn',
    render: (row) => formatBytes(row.todayTrafficIn),
  },
  {
    title: '今日流出',
    key: 'todayTrafficOut',
    render: (row) => formatBytes(row.todayTrafficOut),
  },
  {
    title: '上次启动',
    key: 'lastStartTime',
    render: (row) => formatTime(row.lastStartTime),
  },
  {
    title: '上次关闭',
    key: 'lastCloseTime',
    render: (row) => formatTime(row.lastCloseTime),
  },
]

async function onRefreshClick(): Promise<void> {
  isRefreshing.value = true
  try {
    await rt.refresh()
  } finally {
    isRefreshing.value = false
  }
}

function onTogglePolling(): void {
  if (rt.isPolling.value) {
    rt.stop()
  } else {
    rt.start()
  }
}

function onRestartPolling(): void {
  rt.restart()
}

// 暴露 testing handle（与 LogViewer.vue 同款；spec 用 __testing 进 internals）
defineExpose({
  __testing: {
    rt,
    allProxyTypes,
    activeType,
    firstLoading,
    firstLoadFailed,
    goServerHint,
    formatBytes,
    formatTime,
    lastUpdatedLabel,
    onRefreshClick,
    onTogglePolling,
    onRestartPolling,
  },
})
</script>

<style scoped>
.server-monitor-root {
  width: 100%;
}
</style>
```

**SFC 行数自检（insight L28）**：
- 模板段约 110 行（含 n-tabs / n-data-table / n-card 大量标签）
- script 段约 175 行总；扣除 import / interface / defineExpose / 注释 → 纯逻辑约 ~115 行（columns 数组占 ~50，computed/function 约 65）
- 满足 < 200 行红线

### 3.5 `web/src/router.ts` 改动

在 `{ path: 'server', component: ... }` 之后追加：

```typescript
{ path: 'server/monitor', component: () => import('./pages/ServerMonitor.vue') },
```

注意路径不带前导 `/`，因为它是 AppLayout 的子路由（继承 SessionAuth）。

### 3.6 `web/src/components/AppLayout.vue` 改动

#### menuOptions 加项

在 `{ label: '服务端配置', key: 'server', ... }` 之后追加：

```typescript
{
  label: '服务端监控',
  key: 'server/monitor',
  icon: () => h('span', { class: 'n-icon' }, '◉'),
},
```

#### activeKey 计算兼容

既有逻辑：
```typescript
const activeKey = computed(() => {
  const path = route.path
  if (path.startsWith('/logs/')) return path
  return path.replace(/^\//, '') || 'dashboard'
})
```

替换为：
```typescript
const activeKey = computed(() => {
  const path = route.path
  if (path.startsWith('/logs/')) return path
  if (path === '/server/monitor') return 'server/monitor'  // T-041
  return path.replace(/^\//, '') || 'dashboard'
})
```

#### handleMenuSelect 改动（兼容 'server/monitor' key）

既有：
```typescript
function handleMenuSelect(key: string) {
  if (key.startsWith('/')) {
    void router.push(key)
  } else {
    void router.push('/' + key)
  }
}
```

无须改动：`'server/monitor'` 不以 `/` 开头，会走 `'/' + 'server/monitor'` = `'/server/monitor'` 路径，正确。✓

### 3.7 `docs/dev-map.md` 改动

#### 3.7.1 目录布局 `web/src/`

在 `pages/` 子树追加：
```
            ├── ServerMonitor.vue ← T-041：frps 服务端运行态监控页（消费 T-039 API；5s 轮询 + visibilitychange 自动暂停）
```

在 `composables/` 子树追加：
```
        │   ├── useServerRuntime.ts ← T-041：frps 运行态轮询 composable（双 endpoint Promise.all + epoch race + 3 次失败自动停 + visibilitychange 自管 listener；T-042 复用）
```

在 `api/` 子树追加：
```
        │   ├── serverRuntime.ts ← T-041：/api/v1/server/runtime/{info,proxies,traffic/{name}} 客户端
```

#### 3.7.2 功能在哪里 表（新增 1 行）

```
| 服务端运行态监控页 | `web/src/pages/ServerMonitor.vue` | T-041 新增。useServerRuntime composable 持有 info / proxies / isPolling / error，5s setInterval polling + visibilitychange 自动暂停 + 3 次失败自动停。表格按 type tabs 分组；status 三色 dot；流量人类友好单位（B/KiB/MiB/GiB/TiB）。 |
```

#### 3.7.3 可复用工具 表（新增 1 行）

```
| frps 运行态轮询（双 endpoint + epoch race） | 是 | `web/src/composables/useServerRuntime.ts` | `useServerRuntime(intervalMs=5000)` → `{ info, proxies, isPolling, error, lastUpdated, start, stop, refresh, restart, consecutiveFailCount }`。T-041 引入，T-042 也消费。F-5.6 保留上一次数据 + F-5.7 不在 mount 自动 start。 |
```

## 4. 测试计划

### 4.1 `web/src/composables/__tests__/useServerRuntime.spec.ts`

| 测试 | 覆盖 |
|---|---|
| start → setInterval 触发 → refresh 被调用 | F-5.2 |
| stop → setInterval 清除 → refresh 不再被调 | F-5.2 |
| refresh 成功 → info / proxies 写入 + lastUpdated 更新 + error null + consecutiveFailCount 归零 | F-5.6 |
| refresh 失败 → error 写入 + info / proxies 保留上一次 + consecutiveFailCount + 1 | F-5.6 / AC-6 |
| 连续 3 次失败 → isPolling 自动 false | D-6 / AC-11 |
| restart → 计数清零 + 重启 polling | AC-11 |
| document.hidden = true → clearInterval；恢复 visible → 自动重启 + 立即 refresh | F-5.3 / AC-7 |
| 用户显式 stop 后切后台再切回 → 不自动恢复 | BC-7 |
| onUnmounted → clearInterval + removeEventListener | AC-10 / F-5.4 |
| start 幂等：连续调 2 次不创建 2 个 timer | D-3 |
| epoch race：refresh 中途 unmount → 响应到达不写 ref | BC-5 / D-3.2 |

**测试技术**：
- vi.useFakeTimers() + vi.advanceTimersByTime
- vi.mock 注入 apiGetServerRuntimeInfo / apiGetServerRuntimeProxies
- visibilityHidden 用 opts 注入（默认走 document.hidden；测试用 mock fn）
- 用 mount 一个 Holder 组件让 composable 运行在真 Vue setup 上下文（onUnmounted 才会触发）

### 4.2 `web/src/pages/__tests__/ServerMonitor.spec.ts`

| 测试 | 覆盖 |
|---|---|
| mount 初始 → loading 文案可见 | FR-1.6 / FR-2.6 |
| mount + tick → 显示 server info 卡 + tabs | AC-1 |
| empty proxies → "暂无连接的 proxy" | BC-1 |
| 一类 type 有 error 一类有数据 → 错误 tab 显示文案，数据 tab 显示表格 | AC-12 |
| 首屏失败（API reject 一次）→ NResult 错误页 + retry 按钮 | AC-4 / AC-5 |
| 首屏失败文案含"dashboard 未启用" → "前往服务端配置"按钮可见 | AC-3 / R-1 |
| 点"立即刷新" → API 调一次 + lastUpdated 跳"刚刚刷新" | AC-9 |
| 点"暂停轮询" → 文案切"恢复轮询" + isPolling false | AC-8 |
| 流量字段 0 → "0 B"；1024 → "1 KiB"；1536 → "1.5 KiB" | AC-13 / BC-2 |
| proxy.status="online" → tag type=success；offline → default；其它 → error | AC-14 / AC-15 / D-3 |
| proxy.lastStartTime="" → "—"；"0001-01-01..." → "—" | BC-3 |

**测试技术**：
- vi.mock('../../api/serverRuntime')（与 LogViewer.spec.ts 范式同款）
- vi.mock('naive-ui')（**必须 importOriginal + 6 方法 stub**，insight L4 / L14）
- mount 包 NConfigProvider + NMessageProvider（与 LogViewer.spec.ts 同款 mountInside 函数）
- 用 __testing handle 拿 internal computed / 方法

### 4.3 Adversarial tests（QA stage 6）

| ID | 反向证伪 |
|---|---|
| ADV-1（AC-4）| mock apiGet 全部 reject "frps 进程不可达" → 首屏 NResult 文案命中 + retry 按钮存在 |
| ADV-2（AC-5）| mock apiGet 全部 reject "dashboard 凭据校验失败" → 友好引导文案"前往服务端配置"按钮可见 |
| ADV-3（AC-7）| visibility hidden 触发 → setInterval 调用次数不增；恢复后立即拉一次 |
| ADV-4（D-6）| 连续 3 次 reject → isPolling 切 false + consecutiveFailCount = 3 → 反向证伪：把 MAX_FAIL 改 999 跑同 spec → 应 isPolling 保持 true（说明 D-6 真的生效，否则证伪）|

## 5. 风险

| ID | 风险 | 缓解 |
|---|---|---|
| R-1 | composable 内调用 `onUnmounted` 要求在 setup() 同步路径 → 不能在 async function 内 await 后调用 | 文档化在 JSDoc 头；spec 4.1 覆盖 "mount via Holder" 验证生命周期 |
| R-2 | vi.useFakeTimers 不拦截 visibilitychange listener；测试中需手工触发 onVisibilityChange | 用 opts.visibilityHidden inject seam，测试不依赖 document.hidden |
| R-3 | extractErrorMessage 已有但未导出？ | client.ts L57 已导出 `extractErrorMessage` ✓ |
| R-4 | n-tabs activeKey 在 polling 重置 → tab 闪 | activeType ref 持有，polling 只更 data，无重置（R-6） |
| R-5 | NResult / NSkeleton / NDataTable / NEmpty 等 naive-ui 组件未在既有 spec mock 列表 → 测试报"组件未注册"警告 | vi.mock('naive-ui') importOriginal + spread 模式自动 hoist 所有真实组件 |

## 6. 回滚计划

- 单文件回滚：`git checkout HEAD -- web/src/pages/ServerMonitor.vue` 等
- 路由 / menu / dev-map 回滚 = 普通 git revert
- 不涉及后端 / 数据库 / 配置文件迁移：回滚干净

## 7. 实现顺序（dev stage 推荐）

1. 写 `web/src/types.ts` 4 接口
2. 写 `web/src/api/serverRuntime.ts` 3 函数
3. 写 `web/src/composables/useServerRuntime.ts` + spec → `npx vitest run useServerRuntime` PASS
4. 写 `web/src/pages/ServerMonitor.vue` + spec → `npx vitest run ServerMonitor` PASS
5. 改 `web/src/router.ts` + `web/src/components/AppLayout.vue`
6. 改 `docs/dev-map.md`
7. `pwsh scripts/verify_all.ps1` → 目标 PASS ≥ 32 / FAIL = 1（baseline）

## 8. 分区分配

本项目单 developer 模式（无 `dev-*` 文件）。所有改动归"frontend / Vue + TS" 范畴。

## 9. SA self-check

| 项 | 状态 |
|---|---|
| 与 01 验收标准每条都有实现路径 | ✅ |
| 与既有架构（LogViewer composable + AppLayout menu + router 嵌套）对齐 | ✅ |
| 不引入新 npm 包 | ✅ |
| insight L4 / L14 / L28 / L29 / L33 适用性已评估 | ✅ |
| Design drift（D-1 降级 uptime；D-10 不做 detail 抽屉）已明示 | ✅ |
| 测试覆盖每个 AC + 反向 case BC | ✅ |
| SFC 行数预算符合 200 行红线（按纯逻辑行数判） | ✅ |

---

**Verdict**：READY FOR GATE REVIEW.
