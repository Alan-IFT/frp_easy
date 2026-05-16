import apiClient from './client'
import type { ModeState } from '../types'

export async function apiGetMode(): Promise<ModeState> {
  const res = await apiClient.get<ModeState>('/api/v1/mode')
  return res.data
}

export async function apiPutMode(state: ModeState): Promise<ModeState> {
  const res = await apiClient.put<ModeState>('/api/v1/mode', state)
  return res.data
}
