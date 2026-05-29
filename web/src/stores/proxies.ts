import { defineStore } from 'pinia'
import {
  apiListProxies,
  apiCreateProxy,
  apiUpdateProxy,
  apiDeleteProxy,
} from '../api/proxies'
import { extractErrorMessage } from '../api/client'
import type { Proxy, ProxyInput } from '../types'

interface ProxiesState {
  proxies: Proxy[]
  loading: boolean
  // T-047 A3：加载失败信息。null = 无错误（区分"暂无规则" vs "加载失败"）。
  error: string | null
}

export const useProxiesStore = defineStore('proxies', {
  state: (): ProxiesState => ({
    proxies: [],
    loading: false,
    error: null,
  }),

  actions: {
    // T-047 A3：捕获 fetch 失败并暴露 error，避免 void 吞 promise 让失败渲染成空列表。
    // 失败时保留旧 proxies（不清空），由页面据 error!=null 显示错误态而非 empty 态。
    async fetchProxies(): Promise<void> {
      this.loading = true
      try {
        this.proxies = await apiListProxies()
        this.error = null
      } catch (e) {
        this.error = extractErrorMessage(e, '加载代理规则失败')
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
