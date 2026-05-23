import apiClient from './client'
import type {
  Proxy,
  ProxyInput,
  BatchProxiesRequest,
  BatchProxiesResponse,
} from '../types'

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

/**
 * T-018 §C.1：批量创建代理规则。
 *
 * 后端单事务执行；任一条违反约束（name 重复 / (type,remote_port) 冲突 / 总数超限）
 * → 全部回滚，响应 422/409 + 标准 ApiErrorResponse。前端调用方应捕获错误并提示用户。
 * 成功后 201 + `{created, items[]}`，items 是与单条 ProxyResponse 同构的对象数组。
 */
export async function apiBatchCreateProxies(
  req: BatchProxiesRequest,
): Promise<BatchProxiesResponse> {
  const res = await apiClient.post<BatchProxiesResponse>(
    '/api/v1/proxies/batch',
    req,
  )
  return res.data
}
