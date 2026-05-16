import apiClient from './client'
import type { Proxy, ProxyInput } from '../types'

export async function apiListProxies(): Promise<Proxy[]> {
  const res = await apiClient.get<Proxy[]>('/api/v1/proxies')
  return res.data
}

export async function apiCreateProxy(input: ProxyInput): Promise<Proxy> {
  const res = await apiClient.post<Proxy>('/api/v1/proxies', input)
  return res.data
}

export async function apiUpdateProxy(id: number, input: ProxyInput): Promise<Proxy> {
  const res = await apiClient.put<Proxy>(`/api/v1/proxies/${id}`, input)
  return res.data
}

export async function apiDeleteProxy(id: number): Promise<void> {
  await apiClient.delete(`/api/v1/proxies/${id}`)
}
