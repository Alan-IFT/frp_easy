import { defineStore } from 'pinia'
import { apiGetReady } from '../api/system'

interface AppState {
  initialized: boolean
  binMissing: string[]
  version: string
  ready: boolean
}

export const useAppStore = defineStore('app', {
  state: (): AppState => ({
    initialized: false,
    binMissing: [],
    version: '',
    ready: false,
  }),

  getters: {
    frpcMissing: (state): boolean => state.binMissing.includes('frpc'),
    frpsMissing: (state): boolean => state.binMissing.includes('frps'),
  },

  actions: {
    async fetchReady(): Promise<void> {
      try {
        const info = await apiGetReady()
        this.initialized = info.initialized
        this.binMissing = info.binMissing ?? []
        this.version = info.version ?? ''
        this.ready = true
      } catch {
        this.ready = false
      }
    },
  },
})
