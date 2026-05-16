import apiClient from './client'
import type { DownloadState } from '../types'

export async function apiDownloadBin(kind: 'frpc' | 'frps'): Promise<void> {
  await apiClient.post('/api/v1/system/download-bin', { kind })
}

export async function apiDownloadStatus(kind: 'frpc' | 'frps'): Promise<DownloadState> {
  const res = await apiClient.get<DownloadState>(`/api/v1/system/download-status/${kind}`)
  return res.data
}
