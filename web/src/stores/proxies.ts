import { defineStore } from 'pinia'
import {
  apiListProxies,
  apiCreateProxy,
  apiUpdateProxy,
  apiDeleteProxy,
  apiBatchCreateProxies,
} from '../api/proxies'
import type {
  Proxy,
  ProxyInput,
  BatchProxiesRequest,
  BatchProxiesResponse,
} from '../types'

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

    /**
     * T-018 §C.1：批量创建多条代理规则。
     * 成功后用 fetchProxies 刷新（后端单事务，列表可能含其他用户的并发改动）。
     */
    async batchCreate(req: BatchProxiesRequest): Promise<BatchProxiesResponse> {
      const res = await apiBatchCreateProxies(req)
      // 用 fetchProxies 刷新整表，确保排序 / 其它字段一致；与单条 createProxy 的乐观更新策略
      // 不同（批量量大、乐观更新易错），权衡选择一次 round-trip 换取一致性。
      await this.fetchProxies()
      return res
    },
  },
})
