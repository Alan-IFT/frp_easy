import apiClient from './client'
import type { ProcessInfo } from '../types'

interface ProcStatusAll {
  frpc: ProcessInfo
  frps: ProcessInfo
}

export async function apiStartProc(kind: string): Promise<ProcessInfo> {
  const res = await apiClient.post<ProcessInfo>(`/api/v1/proc/${kind}/start`)
  return res.data
}

export async function apiStopProc(kind: string): Promise<ProcessInfo> {
  const res = await apiClient.post<ProcessInfo>(`/api/v1/proc/${kind}/stop`)
  return res.data
}

export async function apiRestartProc(kind: string): Promise<ProcessInfo> {
  const res = await apiClient.post<ProcessInfo>(`/api/v1/proc/${kind}/restart`)
  return res.data
}

export async function apiGetProcStatus(): Promise<ProcStatusAll> {
  const res = await apiClient.get<ProcStatusAll>('/api/v1/proc/status')
  return res.data
}
