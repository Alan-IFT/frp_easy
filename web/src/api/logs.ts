import apiClient from './client'
import type { LogsTailResponse, LogsIncrementalResponse } from '../types'

export async function apiGetLogsTail(kind: string, lines = 500): Promise<LogsTailResponse> {
  const res = await apiClient.get<LogsTailResponse>(`/api/v1/logs/${kind}`, {
    params: { lines },
  })
  return res.data
}

export async function apiGetLogsIncremental(kind: string, offset: number): Promise<LogsIncrementalResponse> {
  const res = await apiClient.get<LogsIncrementalResponse>(`/api/v1/logs/${kind}`, {
    params: { offset },
  })
  return res.data
}
