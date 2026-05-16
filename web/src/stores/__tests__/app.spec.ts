import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAppStore } from '../app'

// system API をモック
vi.mock('../../api/system', () => ({
  apiGetReady: vi.fn(),
}))

import * as systemApi from '../../api/system'

describe('useAppStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('初期状態は未初期化・バイナリ欠損なし', () => {
    const store = useAppStore()
    expect(store.initialized).toBe(false)
    expect(store.binMissing).toEqual([])
    expect(store.version).toBe('')
    expect(store.ready).toBe(false)
  })

  it('fetchReady 成功後に initialized・binMissing・version が更新される', async () => {
    vi.mocked(systemApi.apiGetReady).mockResolvedValueOnce({
      initialized: true,
      binMissing: [],
      version: '1.2.3',
    })

    const store = useAppStore()
    await store.fetchReady()

    expect(store.initialized).toBe(true)
    expect(store.binMissing).toEqual([])
    expect(store.version).toBe('1.2.3')
    expect(store.ready).toBe(true)
  })

  it('fetchReady でバイナリ欠損リストが更新される', async () => {
    vi.mocked(systemApi.apiGetReady).mockResolvedValueOnce({
      initialized: true,
      binMissing: ['frpc', 'frps'],
      version: '1.0.0',
    })

    const store = useAppStore()
    await store.fetchReady()

    expect(store.binMissing).toEqual(['frpc', 'frps'])
  })

  it('fetchReady エラー時は ready=false のまま', async () => {
    vi.mocked(systemApi.apiGetReady).mockRejectedValueOnce(new Error('Network Error'))

    const store = useAppStore()
    await store.fetchReady()

    expect(store.ready).toBe(false)
    expect(store.initialized).toBe(false)
  })

  describe('frpcMissing getter', () => {
    it('binMissing に frpc が含まれる場合 true', async () => {
      vi.mocked(systemApi.apiGetReady).mockResolvedValueOnce({
        initialized: true,
        binMissing: ['frpc'],
        version: '1.0.0',
      })
      const store = useAppStore()
      await store.fetchReady()

      expect(store.frpcMissing).toBe(true)
    })

    it('binMissing に frpc が含まれない場合 false', async () => {
      vi.mocked(systemApi.apiGetReady).mockResolvedValueOnce({
        initialized: true,
        binMissing: ['frps'],
        version: '1.0.0',
      })
      const store = useAppStore()
      await store.fetchReady()

      expect(store.frpcMissing).toBe(false)
    })

    it('frpsMissing: binMissing に frps が含まれる場合 true', async () => {
      vi.mocked(systemApi.apiGetReady).mockResolvedValueOnce({
        initialized: true,
        binMissing: ['frps'],
        version: '1.0.0',
      })
      const store = useAppStore()
      await store.fetchReady()

      expect(store.frpsMissing).toBe(true)
    })
  })
})
