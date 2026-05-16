import { defineStore } from 'pinia'
import { apiDownloadBin, apiDownloadStatus } from '../api/downloader'
import type { DownloadState } from '../types'

interface DownloaderState {
  frpc: DownloadState
  frps: DownloadState
  _timers: Record<string, ReturnType<typeof setInterval>>
}

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
