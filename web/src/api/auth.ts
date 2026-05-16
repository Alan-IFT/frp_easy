import apiClient from './client'
import type { LoginResponse, MeResponse, CsrfResponse } from '../types'

export async function apiSetup(username: string, password: string): Promise<LoginResponse> {
  const res = await apiClient.post<LoginResponse>('/api/v1/setup', { username, password })
  return res.data
}

export async function apiLogin(username: string, password: string): Promise<LoginResponse> {
  const res = await apiClient.post<LoginResponse>('/api/v1/auth/login', { username, password })
  return res.data
}

export async function apiLogout(): Promise<void> {
  await apiClient.post('/api/v1/auth/logout')
}

export async function apiGetMe(): Promise<MeResponse> {
  const res = await apiClient.get<MeResponse>('/api/v1/auth/me')
  return res.data
}

export async function apiGetCsrf(): Promise<CsrfResponse> {
  const res = await apiClient.get<CsrfResponse>('/api/v1/auth/csrf')
  return res.data
}

export async function apiChangePassword(oldPassword: string, newPassword: string): Promise<void> {
  await apiClient.post('/api/v1/auth/password', { oldPassword, newPassword })
}
