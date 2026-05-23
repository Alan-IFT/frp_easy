import { describe, it, expect, vi, beforeEach } from 'vitest'
import { apiUploadBin, apiProbePorts } from '../system'
import type { UploadBinResponse, PortProbeResponse } from '../../types'

vi.mock('../client', () => ({
  default: {
    post: vi.fn(),
    get: vi.fn(),
    put: vi.fn(),
  },
  setCsrfTokenGetter: vi.fn(),
  extractApiError: vi.fn(),
  extractErrorMessage: vi.fn(),
}))

import apiClient from '../client'

describe('T-018 §A apiUploadBin', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('用 multipart/form-data 提交，字段 kind + file 齐备，且 Content-Type 显式 undefined 抵消 apiClient 实例 default（T-023 修复）', async () => {
    const mockRes: UploadBinResponse = {
      ok: true, kind: 'frpc',
      path: 'frp_linux/frpc',
      size: 12345,
      sha256: 'deadbeef',
    }
    vi.mocked(apiClient.post).mockResolvedValueOnce({ data: mockRes })

    const file = new File([new Uint8Array([0x7f, 0x45, 0x4c, 0x46])], 'frpc',
                          { type: 'application/octet-stream' })
    const result = await apiUploadBin('frpc', file)

    expect(apiClient.post).toHaveBeenCalledTimes(1)
    const [url, fd, opts] = vi.mocked(apiClient.post).mock.calls[0]
    expect(url).toBe('/api/v1/system/upload-bin')
    // FormData 实例
    expect(fd).toBeInstanceOf(FormData)
    const formData = fd as FormData
    expect(formData.get('kind')).toBe('frpc')
    expect(formData.get('file')).toBeInstanceOf(File)
    // **T-023 修复关键断言**：opts.headers 必须显式 Content-Type=undefined，否则
    // apiClient 实例 default 的 application/json 会污染 FormData 请求，axios 不再
    // 自动补 multipart boundary，服务端 multipart 解析直接 400。
    // 原 B-2 假设"不传 headers 就等于没设"是错的：实例 default 会注入。
    expect(opts).toBeDefined()
    const optsObj = opts as {
      headers?: Record<string, string | undefined>
      onUploadProgress?: unknown
    }
    expect(optsObj.headers).toBeDefined()
    // 关键：必须显式列出 Content-Type 键且值为 undefined，axios 1.x 才会抵消
    // 实例 default。`'Content-Type' in headers` 必须为 true。
    expect(optsObj.headers).toHaveProperty('Content-Type')
    expect(optsObj.headers!['Content-Type']).toBeUndefined()
    expect(optsObj.onUploadProgress).toBeDefined()
    expect(result).toEqual(mockRes)
  })

  it('onProgress 回调收到 0-100 整数百分比', async () => {
    vi.mocked(apiClient.post).mockImplementationOnce((_url, _data, opts) => {
      // 模拟 axios 触发 onUploadProgress
      const o = opts as { onUploadProgress?: (e: { loaded: number; total: number }) => void }
      o.onUploadProgress?.({ loaded: 50, total: 200 })
      o.onUploadProgress?.({ loaded: 200, total: 200 })
      return Promise.resolve({ data: { ok: true, path: 'p', size: 0, sha256: '' } })
    })
    const collected: number[] = []
    const file = new File(['x'], 'frpc')
    await apiUploadBin('frpc', file, (pct) => collected.push(pct))
    expect(collected).toEqual([25, 100])
  })

  it('提交 kind=frps 也工作', async () => {
    vi.mocked(apiClient.post).mockResolvedValueOnce({
      data: { ok: true, kind: 'frps', path: 'frp_linux/frps', size: 1, sha256: '' },
    })
    const file = new File(['x'], 'frps')
    const r = await apiUploadBin('frps', file)
    expect(r.kind).toBe('frps')
    const formData = vi.mocked(apiClient.post).mock.calls[0][1] as FormData
    expect(formData.get('kind')).toBe('frps')
  })
})

describe('T-018 §C.3 apiProbePorts', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('POST /api/v1/system/probe-ports 带 ports 数组', async () => {
    const mockRes: PortProbeResponse = {
      results: [
        { port: 22,   available: false, reason: 'privileged' },
        { port: 9999, available: true,  reason: '' },
      ],
    }
    vi.mocked(apiClient.post).mockResolvedValueOnce({ data: mockRes })

    const r = await apiProbePorts([22, 9999])
    expect(apiClient.post).toHaveBeenCalledWith('/api/v1/system/probe-ports', { ports: [22, 9999] })
    expect(r.results).toHaveLength(2)
    expect(r.results[0].reason).toBe('privileged')
  })

  it('空数组也允许（合法空批，与后端契约一致）', async () => {
    vi.mocked(apiClient.post).mockResolvedValueOnce({ data: { results: [] } })
    const r = await apiProbePorts([])
    expect(r.results).toEqual([])
  })
})
