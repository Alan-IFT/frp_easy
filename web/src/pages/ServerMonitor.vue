<template>
  <div class="server-monitor-root">
    <!-- 顶部状态条（FR-3） -->
    <n-space justify="space-between" align="center" class="monitor-toolbar">
      <n-space align="center" :size="8">
        <n-text strong style="font-size: 18px">服务端监控</n-text>
        <n-text depth="3" style="font-size: 13px">{{ lastUpdatedLabel }}</n-text>
      </n-space>
      <n-space align="center" :size="8">
        <n-button size="small" :disabled="isRefreshing" @click="onRefreshClick">
          立即刷新
        </n-button>
        <n-button
          size="small"
          :type="rt.isPolling.value ? 'default' : 'primary'"
          @click="onTogglePolling"
        >
          {{ rt.isPolling.value ? '暂停轮询' : '恢复轮询' }}
        </n-button>
      </n-space>
    </n-space>

    <!-- 连续失败 banner（FR-3.4 / AC-11） -->
    <n-alert
      v-if="showFailureBanner"
      type="error"
      :show-icon="true"
      class="monitor-banner"
    >
      <n-space align="center" :size="8">
        <span>自动刷新已停止：连续 {{ rt.consecutiveFailCount.value }} 次拉取失败。</span>
        <n-button size="tiny" type="error" tertiary @click="onRestartPolling">
          重启轮询
        </n-button>
      </n-space>
    </n-alert>

    <!-- 连接断开 banner（AC-6 / F-5.6 保留上一次数据 + 顶部红色提示） -->
    <n-alert
      v-else-if="showStaleBanner"
      type="warning"
      :show-icon="true"
      class="monitor-banner"
    >
      连接断开：{{ rt.error.value }}（显示的是上一次成功数据）
    </n-alert>

    <!-- 首屏错误（AC-3 / AC-4 / AC-5） -->
    <n-result
      v-if="firstLoadFailed"
      status="error"
      title="无法加载 frps 运行态"
      :description="rt.error.value || ''"
      class="monitor-firstfail"
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
      <n-card title="服务器信息" size="small" class="monitor-card">
        <n-skeleton v-if="firstLoading" text :repeat="3" />
        <n-grid v-else :cols="3" :x-gap="12" :y-gap="8" responsive="screen">
          <n-grid-item>
            <n-statistic label="frps 版本">
              {{ rt.info.value?.version || '—' }}
            </n-statistic>
          </n-grid-item>
          <n-grid-item>
            <n-statistic label="客户端连接数">
              {{ rt.info.value?.clientCounts ?? 0 }}
            </n-statistic>
          </n-grid-item>
          <n-grid-item>
            <n-statistic label="当前连接数">
              {{ rt.info.value?.curConns ?? 0 }}
            </n-statistic>
          </n-grid-item>
          <n-grid-item>
            <n-statistic label="bindPort">
              {{ rt.info.value?.bindPort ?? '—' }}
            </n-statistic>
          </n-grid-item>
          <n-grid-item>
            <n-statistic label="累计流量入">
              {{ formatBytes(rt.info.value?.totalTrafficIn) }}
            </n-statistic>
          </n-grid-item>
          <n-grid-item>
            <n-statistic label="累计流量出">
              {{ formatBytes(rt.info.value?.totalTrafficOut) }}
            </n-statistic>
          </n-grid-item>
        </n-grid>
      </n-card>

      <!-- Proxies n-tabs（FR-2） -->
      <n-card title="代理状态" size="small">
        <n-skeleton v-if="firstLoading" text :repeat="5" />
        <n-empty
          v-else-if="allProxyTypes.length === 0"
          description="暂无连接的 proxy"
        />
        <n-tabs v-else v-model:value="activeType" type="line" animated>
          <n-tab-pane
            v-for="t in allProxyTypes"
            :key="t"
            :name="t"
            :tab="tabLabel(t)"
          >
            <n-alert
              v-if="rt.proxies.value?.errors?.[t]"
              type="error"
              :show-icon="false"
            >
              {{ rt.proxies.value.errors[t] }}
            </n-alert>
            <n-empty
              v-else-if="!rt.proxies.value?.proxies[t]?.length"
              :description="`暂无 ${t} 类型 proxy`"
            />
            <n-data-table
              v-else
              :columns="columns"
              :data="rt.proxies.value.proxies[t] || []"
              :row-key="rowKey"
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
// frps 服务端运行态监控页（只读）。使用 useServerRuntime 5s 轮询。
// SFC 行数自检（insight L28）：纯逻辑行数（去 import / 注释 / interface）目标 < 200。

import { ref, computed, onMounted, onUnmounted, h } from 'vue'
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

// FR-5.7：壳显式调 start + refresh（不在 composable 内自动）
onMounted(() => {
  rt.start()
  void rt.refresh()
})

const isRefreshing = ref(false)

// FR-2.2：固定 type 展示顺序（避免 polling 后 Object.keys 顺序漂移）
const allKnownTypes = ['tcp', 'udp', 'http', 'https', 'stcp', 'sudp', 'xtcp'] as const

// 仅展示"有数据 或 有错误"的 type tab；按 allKnownTypes 固定顺序
const allProxyTypes = computed<string[]>(() => {
  const p = rt.proxies.value?.proxies ?? {}
  const e = rt.proxies.value?.errors ?? {}
  const has = new Set<string>()
  for (const t of Object.keys(p)) {
    if ((p[t]?.length ?? 0) > 0) has.add(t)
  }
  for (const t of Object.keys(e)) {
    has.add(t)
  }
  return allKnownTypes.filter((t) => has.has(t))
})

// FR-2.2 + R-6：activeType ref 独立持有；polling 刷新只更新数据不影响 activeKey
const activeType = ref<string>('tcp')

// 首屏 loading 与 first-fail 互斥三态
const firstLoading = computed(
  () => rt.info.value === null && rt.proxies.value === null && rt.error.value === null,
)
const firstLoadFailed = computed(
  () => rt.info.value === null && rt.proxies.value === null && rt.error.value !== null,
)

// 失败 banner：自动停 polling 后显示重启按钮
const showFailureBanner = computed(
  () => !rt.isPolling.value && rt.consecutiveFailCount.value >= 3,
)

// 陈旧 banner：有上次数据但当前 polling 出现 error（且未进失败 banner 分支）
const showStaleBanner = computed(
  () => !showFailureBanner.value
    && rt.error.value !== null
    && (rt.info.value !== null || rt.proxies.value !== null),
)

// AC-3 / AC-5 引导按钮：错误文案含 "dashboard 未启用" 或 "凭据"
const goServerHint = computed(() => {
  const m = rt.error.value
  return typeof m === 'string' && (m.includes('dashboard 未启用') || m.includes('凭据'))
})

function goServerConfig(): void {
  void router.push('/server')
}

// FR-3.1：lastUpdatedLabel 相对时间。挂一个 1s tickRef 让"刚刚 / N 秒前"显示能跟随时间走，
// 不依赖 5s polling 节拍（GR C-2 建议）。onUnmounted 清理。
const tickRef = ref(0)
const tickTimer = setInterval(() => {
  tickRef.value++
}, 1000)
onUnmounted(() => {
  clearInterval(tickTimer)
})

const lastUpdatedLabel = computed(() => {
  void tickRef.value
  const t = rt.lastUpdated.value
  if (!t) return '尚未刷新'
  const delta = Math.max(0, Date.now() - t)
  if (delta < 5_000) return '刚刚刷新'
  if (delta < 60_000) return `${Math.floor(delta / 1000)} 秒前刷新`
  if (delta < 3_600_000) return `${Math.floor(delta / 60_000)} 分钟前刷新`
  return `${Math.floor(delta / 3_600_000)} 小时前刷新`
})

// FR-1.4 / AC-13：流量字节格式化
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
  const s = u === 0 ? `${v}` : v.toFixed(1).replace(/\.0$/, '')
  return `${s} ${units[u]}`
}

// BC-3：lastStartTime 空 / "0001-01-01..." → "—"
function formatTime(s: string | undefined): string {
  if (!s) return '—'
  if (s.startsWith('0001-')) return '—'
  return s
}

function tabLabel(t: string): string {
  const n = rt.proxies.value?.proxies[t]?.length ?? 0
  return `${t.toUpperCase()} (${n})`
}

// 提供给 n-data-table 的 row-key 函数（避免 inline lambda 触发 Vue patch）
function rowKey(row: ServerRuntimeProxyStatus): string {
  return row.name
}

// FR-2.3 表格列
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
      // GR C-5：status 大小写防御（frps 上游可能返回 "Online" / "online" 不一致）
      const raw = (row.status ?? '').toLowerCase()
      const type: 'success' | 'default' | 'error' =
        raw === 'online' ? 'success' : raw === 'offline' ? 'default' : 'error'
      const text = raw === 'online' ? '在线' : raw === 'offline' ? '离线' : (row.status || '未知')
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
    showFailureBanner,
    showStaleBanner,
    goServerHint,
    formatBytes,
    formatTime,
    tabLabel,
    lastUpdatedLabel,
    onRefreshClick,
    onTogglePolling,
    onRestartPolling,
    goServerConfig,
    columns,
    isRefreshing,
  },
})
</script>

<style scoped>
.server-monitor-root {
  width: 100%;
}
.monitor-toolbar {
  margin-bottom: 16px;
}
.monitor-banner {
  margin-bottom: 16px;
}
.monitor-card {
  margin-bottom: 16px;
}
.monitor-firstfail {
  margin-top: 32px;
}
</style>
