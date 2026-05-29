// T-051 frontend-test-coverage · B-1
// proxies store 专属测试：CRUD 本地数组维护 + fetchProxies loading + T-047 error ref。
// store 范式参考 app/proc/downloader spec：setActivePinia(createPinia()) + vi.mock api。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useProxiesStore } from '../proxies'
import { apiError } from '../../test-utils/apiError'
import type { Proxy, ProxyInput } from '../../types'

vi.mock('../../api/proxies', () => ({
  apiListProxies: vi.fn(),
  apiCreateProxy: vi.fn(),
  apiUpdateProxy: vi.fn(),
  apiDeleteProxy: vi.fn(),
}))

import * as proxiesApi from '../../api/proxies'

function makeProxy(over: Partial<Proxy> = {}): Proxy {
  return {
    id: 1,
    name: 'ssh',
    type: 'tcp',
    localIP: '127.0.0.1',
    localPort: 22,
    remotePort: 6000,
    enabled: true,
    version: 1,
    updatedAt: '2026-05-30T00:00:00Z',
    ...over,
  }
}

const tcpInput = (): ProxyInput => ({
  name: 'ssh',
  type: 'tcp',
  localIP: '127.0.0.1',
  localPort: 22,
  remotePort: 6000,
  enabled: true,
})

describe('useProxiesStore — 初始状态', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('proxies=[] / loading=false / error=null', () => {
    const store = useProxiesStore()
    expect(store.proxies).toEqual([])
    expect(store.loading).toBe(false)
    expect(store.error).toBeNull()
  })
})

describe('useProxiesStore.fetchProxies', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('成功：写入列表 + error 置 null + loading 收尾为 false', async () => {
    const list = [makeProxy(), makeProxy({ id: 2, name: 'web', type: 'http' })]
    vi.mocked(proxiesApi.apiListProxies).mockResolvedValueOnce(list)

    const store = useProxiesStore()
    // 预置一个旧 error，验证成功后被清空
    store.error = '上次失败遗留'
    await store.fetchProxies()

    expect(store.proxies).toEqual(list)
    expect(store.error).toBeNull()
    expect(store.loading).toBe(false)
  })

  it('loading 在请求中为 true、finally 后回 false', async () => {
    let resolveList!: (v: Proxy[]) => void
    vi.mocked(proxiesApi.apiListProxies).mockImplementationOnce(
      () => new Promise<Proxy[]>((res) => { resolveList = res }),
    )
    const store = useProxiesStore()
    const p = store.fetchProxies()
    // 请求未完成 → loading 应为 true
    expect(store.loading).toBe(true)

    resolveList([makeProxy()])
    await p
    expect(store.loading).toBe(false)
  })

  it('T-047 A3：失败时用 extractErrorMessage 写 error 且保留旧列表（不清空）', async () => {
    const store = useProxiesStore()
    // 先成功一次让列表有内容
    vi.mocked(proxiesApi.apiListProxies).mockResolvedValueOnce([makeProxy()])
    await store.fetchProxies()
    expect(store.proxies).toHaveLength(1)

    // 第二次失败：结构化 axios 错误 → extractErrorMessage 透传后端 message
    vi.mocked(proxiesApi.apiListProxies).mockRejectedValueOnce(
      apiError('数据库连接中断', 503, 'DB_DOWN'),
    )
    await store.fetchProxies()

    expect(store.error).toBe('数据库连接中断')
    // F：失败保留旧 proxies，避免失败渲染成空列表
    expect(store.proxies).toHaveLength(1)
    expect(store.loading).toBe(false)
  })

  it('失败为普通 Error（非结构化）→ 走友好 fallback 文案', async () => {
    const store = useProxiesStore()
    // 刻意用 new Error 测 extractErrorMessage 的 fallback 分支：
    // 普通 Error 不带 response.data.error.message，应落到 store 内 fallback。
    vi.mocked(proxiesApi.apiListProxies).mockRejectedValueOnce(new Error('boom'))
    await store.fetchProxies()

    expect(store.error).toBe('加载代理规则失败')
  })
})

describe('useProxiesStore.createProxy', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('成功后 push 到本地数组并返回新对象', async () => {
    const created = makeProxy({ id: 42, name: 'new' })
    vi.mocked(proxiesApi.apiCreateProxy).mockResolvedValueOnce(created)

    const store = useProxiesStore()
    store.proxies = [makeProxy({ id: 1 })]
    const ret = await store.createProxy(tcpInput())

    expect(proxiesApi.apiCreateProxy).toHaveBeenCalledWith(tcpInput())
    expect(ret).toEqual(created)
    expect(store.proxies).toHaveLength(2)
    expect(store.proxies[1]).toEqual(created)
  })

  it('失败时异常传播，本地数组不变', async () => {
    vi.mocked(proxiesApi.apiCreateProxy).mockRejectedValueOnce(
      apiError('端口已占用', 409, 'PORT_CONFLICT'),
    )
    const store = useProxiesStore()
    store.proxies = [makeProxy({ id: 1 })]

    await expect(store.createProxy(tcpInput())).rejects.toBeTruthy()
    expect(store.proxies).toHaveLength(1)
  })
})

describe('useProxiesStore.updateProxy', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('命中 id → findIndex 替换该项', async () => {
    const updated = makeProxy({ id: 2, name: 'renamed', version: 2 })
    vi.mocked(proxiesApi.apiUpdateProxy).mockResolvedValueOnce(updated)

    const store = useProxiesStore()
    store.proxies = [makeProxy({ id: 1 }), makeProxy({ id: 2, name: 'old' })]
    const ret = await store.updateProxy(2, tcpInput())

    expect(proxiesApi.apiUpdateProxy).toHaveBeenCalledWith(2, tcpInput())
    expect(ret).toEqual(updated)
    expect(store.proxies[1]).toEqual(updated)
    // 其它项不动
    expect(store.proxies[0].id).toBe(1)
    expect(store.proxies).toHaveLength(2)
  })

  it('未命中 id（idx<0）→ 不 push、列表长度不变，仍返回服务端对象', async () => {
    const updated = makeProxy({ id: 999, name: 'ghost' })
    vi.mocked(proxiesApi.apiUpdateProxy).mockResolvedValueOnce(updated)

    const store = useProxiesStore()
    store.proxies = [makeProxy({ id: 1 })]
    const ret = await store.updateProxy(999, tcpInput())

    expect(ret).toEqual(updated)
    // idx<0 分支：既不替换也不追加
    expect(store.proxies).toHaveLength(1)
    expect(store.proxies[0].id).toBe(1)
  })
})

describe('useProxiesStore.deleteProxy', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('成功后 filter 掉对应 id', async () => {
    vi.mocked(proxiesApi.apiDeleteProxy).mockResolvedValueOnce(undefined)

    const store = useProxiesStore()
    store.proxies = [makeProxy({ id: 1 }), makeProxy({ id: 2 }), makeProxy({ id: 3 })]
    await store.deleteProxy(2)

    expect(proxiesApi.apiDeleteProxy).toHaveBeenCalledWith(2)
    expect(store.proxies.map((p) => p.id)).toEqual([1, 3])
  })

  it('删除不存在的 id → 列表无变化（filter 不误删）', async () => {
    vi.mocked(proxiesApi.apiDeleteProxy).mockResolvedValueOnce(undefined)

    const store = useProxiesStore()
    store.proxies = [makeProxy({ id: 1 }), makeProxy({ id: 2 })]
    await store.deleteProxy(99)

    expect(store.proxies.map((p) => p.id)).toEqual([1, 2])
  })

  it('失败时异常传播，本地数组不变', async () => {
    vi.mocked(proxiesApi.apiDeleteProxy).mockRejectedValueOnce(
      apiError('删除失败', 500),
    )
    const store = useProxiesStore()
    store.proxies = [makeProxy({ id: 1 })]

    await expect(store.deleteProxy(1)).rejects.toBeTruthy()
    expect(store.proxies).toHaveLength(1)
  })
})
