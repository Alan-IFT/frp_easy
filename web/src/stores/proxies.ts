import { defineStore } from 'pinia'
import { apiListProxies, apiCreateProxy, apiUpdateProxy, apiDeleteProxy } from '../api/proxies'
import type { Proxy, ProxyInput } from '../types'

interface ProxiesState {
  proxies: Proxy[]
  loading: boolean
}

export const useProxiesStore = defineStore('proxies', {
  state: (): ProxiesState => ({
    proxies: [],
    loading: false,
  }),

  actions: {
    async fetchProxies(): Promise<void> {
      this.loading = true
      try {
        this.proxies = await apiListProxies()
      } finally {
        this.loading = false
      }
    },

    async createProxy(input: ProxyInput): Promise<Proxy> {
      const proxy = await apiCreateProxy(input)
      this.proxies.push(proxy)
      return proxy
    },

    async updateProxy(id: number, input: ProxyInput): Promise<Proxy> {
      const proxy = await apiUpdateProxy(id, input)
      const idx = this.proxies.findIndex((p) => p.id === id)
      if (idx >= 0) this.proxies[idx] = proxy
      return proxy
    },

    async deleteProxy(id: number): Promise<void> {
      await apiDeleteProxy(id)
      this.proxies = this.proxies.filter((p) => p.id !== id)
    },
  },
})
