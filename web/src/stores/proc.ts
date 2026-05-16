import { defineStore } from 'pinia'
import { apiGetProcStatus, apiStartProc, apiStopProc, apiRestartProc } from '../api/proc'
import type { ProcessInfo } from '../types'

interface ProcState {
  frpc: ProcessInfo | null
  frps: ProcessInfo | null
  pollingTimer: ReturnType<typeof setInterval> | null
}

const defaultInfo = (kind: string): ProcessInfo => ({
  kind,
  state: 'stopped',
  pid: 0,
  lastErr: '',
  changedAt: new Date().toISOString(),
})

export const useProcStore = defineStore('proc', {
  state: (): ProcState => ({
    frpc: null,
    frps: null,
    pollingTimer: null,
  }),

  getters: {
    frpcInfo: (state): ProcessInfo => state.frpc ?? defaultInfo('frpc'),
    frpsInfo: (state): ProcessInfo => state.frps ?? defaultInfo('frps'),
  },

  actions: {
    async pollStatus(): Promise<void> {
      try {
        const status = await apiGetProcStatus()
        this.frpc = status.frpc
        this.frps = status.frps
      } catch {
        // 一時エラーは無視
      }
    },

    startPolling(): void {
      if (this.pollingTimer !== null) return
      void this.pollStatus()
      this.pollingTimer = setInterval(() => {
        void this.pollStatus()
      }, 2000)
    },

    stopPolling(): void {
      if (this.pollingTimer !== null) {
        clearInterval(this.pollingTimer)
        this.pollingTimer = null
      }
    },

    async startProc(kind: string): Promise<ProcessInfo> {
      const info = await apiStartProc(kind)
      if (kind === 'frpc') this.frpc = info
      else if (kind === 'frps') this.frps = info
      return info
    },

    async stopProc(kind: string): Promise<ProcessInfo> {
      const info = await apiStopProc(kind)
      if (kind === 'frpc') this.frpc = info
      else if (kind === 'frps') this.frps = info
      return info
    },

    async restartProc(kind: string): Promise<ProcessInfo> {
      const info = await apiRestartProc(kind)
      if (kind === 'frpc') this.frpc = info
      else if (kind === 'frps') this.frps = info
      return info
    },
  },
})
