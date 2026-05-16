import axios from 'axios'
import type { AxiosInstance } from 'axios'
import type { ApiErrorResponse } from '../types'

// axios インスタンス（baseURL は空 = 同一オリジン）
const apiClient: AxiosInstance = axios.create({
  baseURL: '',
  withCredentials: true,
  headers: { 'Content-Type': 'application/json' },
})

// CSRF トークンホルダー（循環依存を避けるため store 直接参照しない）
let _csrfTokenGetter: (() => string) | null = null

export function setCsrfTokenGetter(fn: () => string): void {
  _csrfTokenGetter = fn
}

// リクエストインターセプター: X-CSRF-Token ヘッダを付与
apiClient.interceptors.request.use((config) => {
  if (_csrfTokenGetter) {
    const token = _csrfTokenGetter()
    if (token) {
      config.headers['X-CSRF-Token'] = token
    }
  }
  return config
})

// レスポンスインターセプター: 401 → /login リダイレクト
apiClient.interceptors.response.use(
  (response) => response,
  (error: unknown) => {
    if (axios.isAxiosError(error) && error.response?.status === 401) {
      // ログインページ自身 / セットアップページには行かない
      const path = window.location.pathname
      if (path !== '/login' && path !== '/setup') {
        window.location.href = '/login'
      }
    }
    return Promise.reject(error)
  },
)

export default apiClient

// エラーレスポンスからメッセージを取得するヘルパー
export function extractApiError(error: unknown): ApiErrorResponse | null {
  if (axios.isAxiosError(error) && error.response?.data) {
    return error.response.data as ApiErrorResponse
  }
  return null
}

export function extractErrorMessage(error: unknown, fallback = '操作に失敗しました'): string {
  const apiErr = extractApiError(error)
  if (apiErr?.error?.message) return apiErr.error.message
  return fallback
}
