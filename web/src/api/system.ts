import apiClient from './client'
import type { SystemReady, PublicIPResponse } from '../types'

export async function apiGetReady(): Promise<SystemReady> {
  const res = await apiClient.get<SystemReady>('/api/v1/system/ready')
  return res.data
}

export async function apiGetPublicIP(): Promise<PublicIPResponse> {
  const res = await apiClient.get<PublicIPResponse>('/api/v1/system/public-ip')
  return res.data
}
