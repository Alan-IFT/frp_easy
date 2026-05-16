// frpc 客户端配置 API
// 避免文件名冲突，故命名为 frpclient.ts
import apiClient from './client'
import type { FrpcServerConn } from '../types'

export async function apiGetClient(reveal = false): Promise<FrpcServerConn> {
  const params = reveal ? { reveal: '1' } : {}
  const res = await apiClient.get<FrpcServerConn>('/api/v1/client', { params })
  return res.data
}

export async function apiPutClient(config: FrpcServerConn): Promise<FrpcServerConn> {
  const res = await apiClient.put<FrpcServerConn>('/api/v1/client', config)
  return res.data
}
