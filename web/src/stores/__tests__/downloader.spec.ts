// T-027 downloader store cancel action 测试。
// 覆盖 01 §5.4 AC-store-cancel-action 等。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useDownloaderStore } from '../downloader'

vi.mock('../../api/downloader', () => ({
  apiDownloadBin: vi.fn(),
  apiDownloadStatus: vi.fn(),
  apiCancelDownload: vi.fn(),
}))

import * as downloaderApi from '../../api/downloader'

describe('T-027 useDownloaderStore.cancelDownload', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('cancel 成功后 store.frpc 切到 canceled，timer 被清', async () => {
    vi.mocked(downloaderApi.apiCancelDownload).mockResolvedValueOnce({
      status: 'canceled',
      progress: 42,
      error: '用户取消下载',
    })
    const store = useDownloaderStore()
    // 模拟一个 timer 已经在跑
    store._timers['frpc'] = setInterval(() => {}, 1000)

    await store.cancelDownload('frpc')

    expect(downloaderApi.apiCancelDownload).toHaveBeenCalledWith('frpc')
    expect(store.frpc.status).toBe('canceled')
    expect(store.frpc.progress).toBe(42)
    expect(store._timers['frpc']).toBeUndefined() // stopPolling 触发
  })

  it('cancel idle 时返 idle state，不改变 store 状态', async () => {
    vi.mocked(downloaderApi.apiCancelDownload).mockResolvedValueOnce({
      status: 'idle',
      progress: 0,
    })
    const store = useDownloaderStore()
    await store.cancelDownload('frps')
    expect(store.frps.status).toBe('idle')
  })

  it('cancel 失败时仍然 stopPolling（finally 保证）', async () => {
    vi.mocked(downloaderApi.apiCancelDownload).mockRejectedValueOnce(new Error('network'))
    const store = useDownloaderStore()
    store._timers['frpc'] = setInterval(() => {}, 1000)

    await expect(store.cancelDownload('frpc')).rejects.toThrow()
    // finally 触发的 stopPolling 仍然要把 timer 清掉
    expect(store._timers['frpc']).toBeUndefined()
  })

  it('canceled 状态不算 isDownloading', () => {
    const store = useDownloaderStore()
    store.frpc = { status: 'canceled', progress: 30 }
    expect(store.isDownloading('frpc')).toBe(false)
  })
})
