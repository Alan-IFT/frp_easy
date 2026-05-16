import axios from 'axios'
import type { AxiosInstance } from 'axios'
import type { ApiErrorResponse } from '../types'

// axios 实例（baseURL 为空 = 同源）
const apiClient: AxiosInstance = axios.create({
  baseURL: '',
  withCredentials: true,
  headers: { 'Content-Type': 'application/json' },
})

// CSRF token 持有者（避免循环依赖，不直接引用 store）
let _csrfTokenGetter: (() => string) | null = null

export function setCsrfTokenGetter(fn: () => string): void {
  _csrfTokenGetter = fn
}

// 请求拦截器：附加 X-CSRF-Token 头
apiClient.interceptors.request.use((config) => {
  if (_csrfTokenGetter) {
    const token = _csrfTokenGetter()
    if (token) {
      config.headers['X-CSRF-Token'] = token
    }
  }
  return config
})

// 响应拦截器：401 → 跳转 /login
apiClient.interceptors.response.use(
  (response) => response,
  (error: unknown) => {
    if (axios.isAxiosError(error) && error.response?.status === 401) {
      // 登录页/设置页本身不做跳转
      const path = window.location.pathname
      if (path !== '/login' && path !== '/setup') {
        window.location.href = '/login'
      }
    }
    return Promise.reject(error)
  },
)

export default apiClient

// 从错误响应中提取消息的辅助函数
export function extractApiError(error: unknown): ApiErrorResponse | null {
  if (axios.isAxiosError(error) && error.response?.data) {
    return error.response.data as ApiErrorResponse
  }
  return null
}

export function extractErrorMessage(error: unknown, fallback = '操作失败'): string {
  const apiErr = extractApiError(error)
  if (apiErr?.error?.message) return apiErr.error.message
  return fallback
}
