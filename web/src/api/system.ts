import apiClient from './client'
import type { SystemReady } from '../types'

export async function apiGetReady(): Promise<SystemReady> {
  const res = await apiClient.get<SystemReady>('/api/v1/system/ready')
  return res.data
}
