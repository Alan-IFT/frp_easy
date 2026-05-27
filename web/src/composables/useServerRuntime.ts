// T-041 / server-monitor-page-ui · 02 §3.3
//
// frps 运行态轮询 composable。一次 polling 并发拉 info + proxies 两端点（Promise.all）；
// epoch race 防过期响应写组件（T-036 useLogBuffer.ts 范式）；
// 连续 3 次失败自动 stop（D-6 / T-036 BC-6 范式）；
// visibilitychange 自动暂停 / 恢复（FR-5.3 / AC-7）；
// onUnmounted 自动清理 timer + listener（FR-5.4 / AC-10）。
//
// 必须在 Vue 组件 setup() 同步路径调用（因内部使用 onUnmounted）。
// F-5.7：不在 mount 自动 start，由壳组件显式 start() —— 测试 / SSR 友好。
//
// 失败语义（F-5.6）：
//   - error 写入字面文案；info / proxies 保留上一次成功值（不清空）。
//   - consecutiveFailCount 递增；成功后归零。
//   - 达 MAX_FAIL 阈值 → stopInternal(false) 让 isPolling 切 false，
//     但**不**设 userStoppedExplicitly，让用户点"重启轮询"按钮的 restart() 路径清零计数。

import { ref, onUnmounted, type Ref } from 'vue'
import {
  apiGetServerRuntimeInfo,
  apiGetServerRuntimeProxies,
} from '../api/serverRuntime'
import { extractErrorMessage } from '../api/client'
import type { ServerRuntimeInfo, ServerRuntimeProxiesResponse } from '../types'

export interface UseServerRuntimeOptions {
  /**
   * 测试 seam：注入 visibility 判定函数。默认走 `document.visibilityState`。
   * 测试中用 vi.fn 注入可控制 visibility 切换。
   */
  visibilityHidden?: () => boolean
}

export interface UseServerRuntimeReturn {
  info: Ref<ServerRuntimeInfo | null>
  proxies: Ref<ServerRuntimeProxiesResponse | null>
  isPolling: Ref<boolean>
  error: Ref<string | null>
  consecutiveFailCount: Ref<number>
  lastUpdated: Ref<number>
  /** 启动 setInterval；幂等 —— 重复调用不会创建多个 timer */
  start: () => void
  /** 用户显式暂停；设 userStoppedExplicitly，让 visibility 恢复时不自动复位（BC-7） */
  stop: () => void
  /** 立即拉一次（不依赖 timer）；返回 promise 让调用方可 await */
  refresh: () => Promise<void>
  /** D-6 自动 stop 后用户点"重启轮询"按钮：清零计数 + 重新 start */
  restart: () => void
  /** 仅供测试：当前 epoch（验证 race 行为） */
  __epoch: Ref<number>
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
  const epoch = ref<number>(0)

  let timer: ReturnType<typeof setInterval> | null = null
  let pausedByVisibility = false
  // BC-7：用户显式 stop() → 后续 visibility 恢复不自动 resume
  let userStoppedExplicitly = false

  const isHidden = opts.visibilityHidden
    ?? (() => typeof document !== 'undefined' && document.hidden === true)

  async function refresh(): Promise<void> {
    const at = ++epoch.value
    try {
      const [i, p] = await Promise.all([
        apiGetServerRuntimeInfo(),
        apiGetServerRuntimeProxies(),
      ])
      // BC-5：组件已 unmount（onUnmounted 自增 epoch）或新 refresh 已发出 → 当前响应过期
      if (at !== epoch.value) return
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
        // D-6：达上限自动 stop polling；不设 userStoppedExplicitly，让 restart() 能恢复
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
    // BC-7：用户显式停过 → 不要因为 tab 切换自动恢复
    if (userStoppedExplicitly) return
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
        // 切回时立即拉一次让用户即刻看到最新数据
        void refresh()
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
    // 自增 epoch 让 in-flight 响应被丢弃（BC-5）
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
    __epoch: epoch,
  }
}
