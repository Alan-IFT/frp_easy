import { describe, it, expect, vi, beforeEach } from 'vitest'
import { apiGetMode, apiPutMode } from '../mode'
import type { ModeState } from '../../types'

// apiClient をモック
vi.mock('../client', () => ({
  default: {
    get: vi.fn(),
    put: vi.fn(),
  },
  setCsrfTokenGetter: vi.fn(),
  extractApiError: vi.fn(),
  extractErrorMessage: vi.fn(),
}))

import apiClient from '../client'

describe('mode API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('apiGetMode', () => {
    it('GET /api/v1/mode の結果を返す', async () => {
      const mockState: ModeState = { frpc: true, frps: false }
      vi.mocked(apiClient.get).mockResolvedValueOnce({ data: mockState })

      const result = await apiGetMode()

      expect(apiClient.get).toHaveBeenCalledWith('/api/v1/mode')
      expect(result).toEqual(mockState)
    })

    it('frpc=false frps=true のレスポンスも正しく返す', async () => {
      const mockState: ModeState = { frpc: false, frps: true }
      vi.mocked(apiClient.get).mockResolvedValueOnce({ data: mockState })

      const result = await apiGetMode()

      expect(result.frpc).toBe(false)
      expect(result.frps).toBe(true)
    })

    it('API エラー時は例外を伝播する', async () => {
      vi.mocked(apiClient.get).mockRejectedValueOnce(new Error('Network Error'))

      await expect(apiGetMode()).rejects.toThrow('Network Error')
    })
  })

  describe('apiPutMode', () => {
    it('PUT /api/v1/mode にモード状態を送信する', async () => {
      const reqState: ModeState = { frpc: true, frps: false }
      const respState: ModeState = { frpc: true, frps: false }
      vi.mocked(apiClient.put).mockResolvedValueOnce({ data: respState })

      const result = await apiPutMode(reqState)

      expect(apiClient.put).toHaveBeenCalledWith('/api/v1/mode', reqState)
      expect(result).toEqual(respState)
    })

    it('frpc=false frps=false の無効化リクエストを正しく送る', async () => {
      const reqState: ModeState = { frpc: false, frps: false }
      vi.mocked(apiClient.put).mockResolvedValueOnce({ data: reqState })

      const result = await apiPutMode(reqState)

      expect(result.frpc).toBe(false)
      expect(result.frps).toBe(false)
    })

    it('API エラー時は例外を伝播する', async () => {
      const reqState: ModeState = { frpc: true, frps: true }
      vi.mocked(apiClient.put).mockRejectedValueOnce(new Error('Unauthorized'))

      await expect(apiPutMode(reqState)).rejects.toThrow('Unauthorized')
    })
  })
})
