import { defineStore } from 'pinia'
import { apiDownloadBin, apiDownloadStatus, apiCancelDownload } from '../api/downloader'
import type { DownloadState } from '../types'

interface DownloaderState {
  frpc: DownloadState
  frps: DownloadState
  _timers: Record<string, ReturnType<typeof setInterval>>
}

// T-027 F-7：显式标注返回类型，让 'canceled' / 'failed' / 'success' 等 union 成员
// 在工厂使用处也能流入；避免编译器把它窄化成 {status:'idle', progress:0} 字面量类型。
const idleState = (): DownloadState => ({ status: 'idle', progress: 0 })

export const useDownloaderStore = defineStore('downloader', {
  state: (): DownloaderState => ({
    frpc: idleState(),
    frps: idleState(),
    _timers: {},
  }),

  getters: {
    isDownloading: (state) => (kind: 'frpc' | 'frps'): boolean =>
      state[kind].status === 'downloading',
  },

  actions: {
    async downloadBin(kind: 'frpc' | 'frps'): Promise<void> {
      try {
        await apiDownloadBin(kind)
      } catch {
        // 409 PROC_BUSY means already downloading — just start polling
      }
      this.startPolling(kind)
    },

    // T-027 FR-10：取消下载。后端 FR-7 保证返回时 state 已是 canceled，
    // 本 action 直接把返回的 state 灌进 store；finally 内 stopPolling 让
    // R-3"短暂回弹"风险归零（即使后端 cancel 200 早于轮询 tick，也无回弹）。
    async cancelDownload(kind: 'frpc' | 'frps'): Promise<void> {
      try {
        const next = await apiCancelDownload(kind)
        this[kind] = next
      } finally {
        this.stopPolling(kind)
      }
    },

    startPolling(kind: 'frpc' | 'frps'): void {
      // Stop any existing polling for this kind
      this.stopPolling(kind)

      const timer = setInterval(async () => {
        try {
          const state = await apiDownloadStatus(kind)
          this[kind] = state
          if (state.status !== 'downloading') {
            this.stopPolling(kind)
          }
        } catch {
          // ignore transient poll errors
        }
      }, 1000)

      this._timers[kind] = timer
    },

    stopPolling(kind: 'frpc' | 'frps'): void {
      if (this._timers[kind]) {
        clearInterval(this._timers[kind])
        delete this._timers[kind]
      }
    },

    downloadState(kind: 'frpc' | 'frps'): DownloadState {
      return this[kind]
    },
  },
})
