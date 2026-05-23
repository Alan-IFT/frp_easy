import apiClient from './client'
import type { DownloadState } from '../types'

export async function apiDownloadBin(kind: 'frpc' | 'frps'): Promise<void> {
  await apiClient.post('/api/v1/system/download-bin', { kind })
}

export async function apiDownloadStatus(kind: 'frpc' | 'frps'): Promise<DownloadState> {
  const res = await apiClient.get<DownloadState>(`/api/v1/system/download-status/${kind}`)
  return res.data
}

// T-027：取消正在进行的下载；返回取消后的最新 state（后端 FR-7 保证返回时已是终态）。
// 注意：不传 body（chi 不读 body），不显式指定 Content-Type 避免 insight L37 axios default
// 污染陷阱的镜像问题（这里我们要的是空 POST 不要 multipart 之类的胡乱伪造）。
export async function apiCancelDownload(kind: 'frpc' | 'frps'): Promise<DownloadState> {
  const res = await apiClient.post<DownloadState>(`/api/v1/system/download-cancel/${kind}`)
  return res.data
}
