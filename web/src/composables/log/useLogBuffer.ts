// T-036 / log-ui-ux-polish · 02 §3.6.2
// 内存缓冲（slice(-500)）+ loadTail / loadIncremental + 连续失败计数 + kindEpoch race 保护。

import { ref, computed, type Ref, type ComputedRef } from 'vue'
import { apiGetLogsTail, apiGetLogsIncremental } from '../../api/logs'
import { parseLogLine, type ParsedLogLine } from './parseLogLine'

export interface UseLogBufferOptions {
  max?: number
  pollIntervalMs?: number
  message?: {
    error: (msg: string) => void
    success?: (msg: string) => void
  }
}

export interface UseLogBufferReturn {
  lines: Ref<string[]>
  parsedLines: ComputedRef<ParsedLogLine[]>
  lastUpdatedAt: Ref<number>
  firstLoading: Ref<boolean>
  firstLoadError: Ref<string | null>
  consecutiveFailCount: Ref<number>
  lastError: Ref<string | null>
  autoRefresh: Ref<boolean>
  setAutoRefresh: (v: boolean) => void
  loadTail: () => Promise<void>
  loadIncremental: () => Promise<void>
  clear: () => void
  stopPolling: () => void
  /** 仅供单测：当前 epoch 计数（私有语义，但暴露用以验证 BC-5 race） */
  __epoch: Ref<number>
}

const MAX_FAIL = 3
const DEFAULT_POLL_MS = 2000
const DEFAULT_MAX = 500

export function useLogBuffer(
  kindRef: () => string,
  opts: UseLogBufferOptions = {},
): UseLogBufferReturn {
  const max = opts.max ?? DEFAULT_MAX
  const pollMs = opts.pollIntervalMs ?? DEFAULT_POLL_MS

  const lines = ref<string[]>([])
  const lastUpdatedAt = ref<number>(0)
  const firstLoading = ref<boolean>(false)
  const firstLoadError = ref<string | null>(null)
  const consecutiveFailCount = ref<number>(0)
  const lastError = ref<string | null>(null)
  const autoRefresh = ref<boolean>(false)
  const epoch = ref<number>(0)

  let currentOffset = 0
  let pollingTimer: ReturnType<typeof setInterval> | null = null
  let hasFirstLoaded = false

  // ParsedLogLine memoization: key = raw 字符串。slice(-500) 后旧 key 自然失活。
  const parseCache = new Map<string, ParsedLogLine>()
  const parsedLines = computed<ParsedLogLine[]>(() => {
    return lines.value.map((raw) => {
      const cached = parseCache.get(raw)
      if (cached) return cached
      const parsed = parseLogLine(raw)
      parseCache.set(raw, parsed)
      return parsed
    })
  })

  function bumpEpoch() {
    epoch.value++
  }

  function clear() {
    lines.value = []
    currentOffset = 0
    parseCache.clear()
  }

  function stopPolling() {
    if (pollingTimer !== null) {
      clearInterval(pollingTimer)
      pollingTimer = null
    }
  }

  function startPolling() {
    if (pollingTimer !== null) return
    pollingTimer = setInterval(() => {
      void loadIncremental()
    }, pollMs)
  }

  function setAutoRefresh(v: boolean) {
    autoRefresh.value = v
    if (v) {
      // 重新开启时清零失败计数（用户显式重试）
      consecutiveFailCount.value = 0
      lastError.value = null
      startPolling()
    } else {
      stopPolling()
    }
  }

  async function loadTail() {
    const epochAtStart = epoch.value
    firstLoadError.value = null
    if (!hasFirstLoaded) firstLoading.value = true
    try {
      const res = await apiGetLogsTail(kindRef(), max)
      if (epochAtStart !== epoch.value) return // BC-5: 过期响应丢弃
      lines.value = res.lines.slice(-max)
      currentOffset = 0
      lastUpdatedAt.value = Date.now()
      hasFirstLoaded = true
      // 首次加载成功后归零失败计数
      consecutiveFailCount.value = 0
      lastError.value = null
    } catch (e) {
      if (epochAtStart !== epoch.value) return
      const msg = e instanceof Error ? e.message : '加载日志失败'
      if (!hasFirstLoaded) {
        firstLoadError.value = msg
      }
      lastError.value = msg
    } finally {
      if (epochAtStart === epoch.value) {
        firstLoading.value = false
      }
    }
  }

  async function loadIncremental() {
    const epochAtStart = epoch.value
    try {
      const res = await apiGetLogsIncremental(kindRef(), currentOffset)
      if (epochAtStart !== epoch.value) return // BC-5: 过期响应丢弃
      if (res.data) {
        const newLines = res.data.split('\n').filter((l) => l !== '')
        if (newLines.length > 0) {
          lines.value = [...lines.value, ...newLines].slice(-max)
        }
      }
      currentOffset = res.nextOffset
      lastUpdatedAt.value = Date.now()
      consecutiveFailCount.value = 0
      lastError.value = null
    } catch (e) {
      if (epochAtStart !== epoch.value) return
      const msg = e instanceof Error ? e.message : '增量拉取失败'
      lastError.value = msg
      consecutiveFailCount.value++
      if (consecutiveFailCount.value >= MAX_FAIL) {
        // 2.4 §21 + BC-6：连续 3 次失败 → 停 polling + 关 autoRefresh + 一次 message.error
        stopPolling()
        autoRefresh.value = false
        opts.message?.error('自动刷新已停止：连续 3 次拉取失败')
      }
    }
  }

  return {
    lines,
    parsedLines,
    lastUpdatedAt,
    firstLoading,
    firstLoadError,
    consecutiveFailCount,
    lastError,
    autoRefresh,
    setAutoRefresh,
    loadTail,
    loadIncremental,
    clear,
    stopPolling,
    __epoch: epoch,
    // bumpEpoch 通过返回扩展暴露给壳组件的 watch（BC-4 / BC-5）
    // —— 这里把它挂在私有路径上，避免污染公共契约
    ...({ __bumpEpoch: bumpEpoch } as Record<string, unknown>),
  } as UseLogBufferReturn & { __bumpEpoch: () => void }
}
