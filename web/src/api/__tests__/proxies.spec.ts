import { describe, it, expect, vi, beforeEach } from 'vitest'
import { apiBatchCreateProxies } from '../proxies'
import type { BatchProxiesRequest, BatchProxiesResponse } from '../../types'

vi.mock('../client', () => ({
  default: {
    post: vi.fn(),
    get: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
  setCsrfTokenGetter: vi.fn(),
  extractApiError: vi.fn(),
  extractErrorMessage: vi.fn(),
}))

import apiClient from '../client'

describe('T-018 §C.1 apiBatchCreateProxies', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('POST /api/v1/proxies/batch 提交 BatchProxiesRequest，返回 BatchProxiesResponse', async () => {
    const req: BatchProxiesRequest = {
      basename: 'web',
      type: 'tcp',
      localIP: '127.0.0.1',
      portsExpr: '6000-6002',
      enabled: true,
    }
    const mockRes: BatchProxiesResponse = {
      created: 3,
      items: [
        { id: 10, name: 'web-6000', type: 'tcp', localIP: '127.0.0.1',
          localPort: 6000, remotePort: 6000, enabled: true,
          version: 1, updatedAt: '2026-05-23T00:00:00Z' },
        { id: 11, name: 'web-6001', type: 'tcp', localIP: '127.0.0.1',
          localPort: 6001, remotePort: 6001, enabled: true,
          version: 1, updatedAt: '2026-05-23T00:00:00Z' },
        { id: 12, name: 'web-6002', type: 'tcp', localIP: '127.0.0.1',
          localPort: 6002, remotePort: 6002, enabled: true,
          version: 1, updatedAt: '2026-05-23T00:00:00Z' },
      ],
    }
    vi.mocked(apiClient.post).mockResolvedValueOnce({ data: mockRes })

    const result = await apiBatchCreateProxies(req)

    expect(apiClient.post).toHaveBeenCalledWith('/api/v1/proxies/batch', req)
    expect(result.created).toBe(3)
    expect(result.items).toHaveLength(3)
  })

  it('字段名是 portsExpr（不是 portSpec），与后端契约一致', async () => {
    vi.mocked(apiClient.post).mockResolvedValueOnce({ data: { created: 0, items: [] } })
    const req: BatchProxiesRequest = {
      basename: 'svc',
      type: 'udp',
      portsExpr: '5000,5001,5002',
    }
    await apiBatchCreateProxies(req)
    const body = vi.mocked(apiClient.post).mock.calls[0][1] as BatchProxiesRequest
    // 关键断言：字段名 portsExpr 而非 portSpec
    expect(body.portsExpr).toBe('5000,5001,5002')
    expect((body as unknown as Record<string, unknown>).portSpec).toBeUndefined()
    expect(body.basename).toBe('svc')
  })

  it('提交时透传 type 与 localIP', async () => {
    vi.mocked(apiClient.post).mockResolvedValueOnce({ data: { created: 0, items: [] } })
    await apiBatchCreateProxies({
      basename: 'a',
      type: 'udp',
      localIP: '0.0.0.0',
      portsExpr: '7000',
    })
    const body = vi.mocked(apiClient.post).mock.calls[0][1] as BatchProxiesRequest
    expect(body.type).toBe('udp')
    expect(body.localIP).toBe('0.0.0.0')
  })

  it('API 出错时传播异常（让调用方在 UI 层用 extractErrorMessage 提示）', async () => {
    vi.mocked(apiClient.post).mockRejectedValueOnce(new Error('422'))
    await expect(apiBatchCreateProxies({
      basename: 'x',
      type: 'tcp',
      portsExpr: 'badexpr',
    })).rejects.toThrow('422')
  })
})
