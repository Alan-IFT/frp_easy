import apiClient from './client'
import type { FrpsConfig } from '../types'

export async function apiGetServer(reveal = false): Promise<FrpsConfig> {
  const params = reveal ? { reveal: '1' } : {}
  const res = await apiClient.get<FrpsConfig>('/api/v1/server', { params })
  return res.data
}

export async function apiPutServer(config: FrpsConfig): Promise<FrpsConfig> {
  const res = await apiClient.put<FrpsConfig>('/api/v1/server', config)
  return res.data
}
