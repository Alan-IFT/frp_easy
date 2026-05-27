// T-041 / server-monitor-page-ui · 02 §4.1
// useServerRuntime composable 单测：vi.useFakeTimers polling / visibility / 3 次失败自动停 / unmount 清理。

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h, nextTick } from 'vue'

// mock 必须先于 import 被测对象
vi.mock('../../api/serverRuntime', () => ({
  apiGetServerRuntimeInfo: vi.fn(),
  apiGetServerRuntimeProxies: vi.fn(),
  apiGetServerRuntimeTraffic: vi.fn(),
}))

import { useServerRuntime, type UseServerRuntimeReturn } from '../useServerRuntime'
import * as api from '../../api/serverRuntime'

const infoMock = vi.mocked(api.apiGetServerRuntimeInfo)
const proxiesMock = vi.mocked(api.apiGetServerRuntimeProxies)

// Holder：让 composable 跑在真正的 Vue setup 上下文，onUnmounted 才会触发。
// 通过 exposeHandle 让 spec 拿到 composable 返回值。
function mountHolder(opts: { intervalMs?: number; visibilityHidden?: () => boolean } = {}) {
  let handle!: UseServerRuntimeReturn
  const Holder = defineComponent({
    setup() {
      handle = useServerRuntime(opts.intervalMs ?? 5000, {
        visibilityHidden: opts.visibilityHidden,
      })
      return () => h('div')
    },
  })
  const wrapper = mount(Holder)
  return { wrapper, handle: handle! }
}

async function flush(n = 3) {
  for (let i = 0; i < n; i++) await nextTick()
}

beforeEach(() => {
  infoMock.mockReset()
  proxiesMock.mockReset()
  infoMock.mockResolvedValue({ clientCounts: 0, curConns: 0 })
  proxiesMock.mockResolvedValue({ proxies: {} })
})

afterEach(() => {
  vi.useRealTimers()
})

describe('useServerRuntime — start / stop / refresh', () => {
  it('初始：isPolling=false / info=null / proxies=null / error=null', () => {
    const { handle } = mountHolder()
    expect(handle.isPolling.value).toBe(false)
    expect(handle.info.value).toBeNull()
    expect(handle.proxies.value).toBeNull()
    expect(handle.error.value).toBeNull()
    expect(handle.consecutiveFailCount.value).toBe(0)
  })

  it('start → isPolling=true；setInterval 触发 → refresh 被调', async () => {
    vi.useFakeTimers()
    const { handle } = mountHolder({ intervalMs: 1000 })
    handle.start()
    expect(handle.isPolling.value).toBe(true)

    vi.advanceTimersByTime(1000)
    await flush()
    expect(infoMock).toHaveBeenCalledTimes(1)
    expect(proxiesMock).toHaveBeenCalledTimes(1)

    vi.advanceTimersByTime(1000)
    await flush()
    expect(infoMock).toHaveBeenCalledTimes(2)
  })

  it('stop → isPolling=false；后续 tick 不再触发 refresh', async () => {
    vi.useFakeTimers()
    const { handle } = mountHolder({ intervalMs: 1000 })
    handle.start()
    vi.advanceTimersByTime(1000)
    await flush()
    const callsBefore = infoMock.mock.calls.length

    handle.stop()
    expect(handle.isPolling.value).toBe(false)

    vi.advanceTimersByTime(5000)
    await flush()
    expect(infoMock.mock.calls.length).toBe(callsBefore)
  })

  it('refresh 成功 → info / proxies 写入 + lastUpdated 更新 + error null + 计数归零', async () => {
    infoMock.mockResolvedValueOnce({
      clientCounts: 5,
      curConns: 12,
      version: '0.58.1',
    })
    proxiesMock.mockResolvedValueOnce({
      proxies: { tcp: [{ name: 'ssh', status: 'online' }] },
    })
    const { handle } = mountHolder()
    handle.consecutiveFailCount.value = 2  // simulate prior failures
    await handle.refresh()

    expect(handle.info.value?.clientCounts).toBe(5)
    expect(handle.info.value?.curConns).toBe(12)
    expect(handle.info.value?.version).toBe('0.58.1')
    expect(handle.proxies.value?.proxies.tcp?.[0]?.name).toBe('ssh')
    expect(handle.lastUpdated.value).toBeGreaterThan(0)
    expect(handle.error.value).toBeNull()
    expect(handle.consecutiveFailCount.value).toBe(0)
  })

  it('refresh 失败 → error 写入 + 计数 + 1 + info / proxies 保留上一次', async () => {
    // 先成功一次让 info / proxies 有值
    infoMock.mockResolvedValueOnce({ clientCounts: 3, curConns: 7 })
    proxiesMock.mockResolvedValueOnce({ proxies: { tcp: [] } })
    const { handle } = mountHolder()
    await handle.refresh()
    expect(handle.info.value?.clientCounts).toBe(3)

    // 第二次失败
    infoMock.mockRejectedValueOnce(new Error('boom'))
    proxiesMock.mockRejectedValueOnce(new Error('boom'))
    await handle.refresh()

    expect(handle.error.value).toBe('boom')
    expect(handle.consecutiveFailCount.value).toBe(1)
    // F-5.6：保留上次数据
    expect(handle.info.value?.clientCounts).toBe(3)
    expect(handle.info.value?.curConns).toBe(7)
  })

  it('start 幂等：连续调 2 次不创建 2 个 timer', async () => {
    vi.useFakeTimers()
    const { handle } = mountHolder({ intervalMs: 1000 })
    handle.start()
    handle.start()  // 第二次应被忽略
    vi.advanceTimersByTime(1000)
    await flush()
    // 应该只调一次，不是两次
    expect(infoMock).toHaveBeenCalledTimes(1)
  })
})

describe('useServerRuntime — 3 次失败自动停（D-6 / AC-11）', () => {
  it('连续 3 次 reject → isPolling 切 false + count=3', async () => {
    infoMock.mockRejectedValue(new Error('upstream down'))
    proxiesMock.mockRejectedValue(new Error('upstream down'))
    const { handle } = mountHolder()
    handle.start()
    expect(handle.isPolling.value).toBe(true)

    await handle.refresh()
    expect(handle.consecutiveFailCount.value).toBe(1)
    expect(handle.isPolling.value).toBe(true)

    await handle.refresh()
    expect(handle.consecutiveFailCount.value).toBe(2)
    expect(handle.isPolling.value).toBe(true)

    await handle.refresh()
    expect(handle.consecutiveFailCount.value).toBe(3)
    expect(handle.isPolling.value).toBe(false)
  })

  it('restart() → 计数清零 + isPolling 切回 true + error null', async () => {
    infoMock.mockRejectedValue(new Error('x'))
    proxiesMock.mockRejectedValue(new Error('x'))
    const { handle } = mountHolder()
    handle.start()
    await handle.refresh()
    await handle.refresh()
    await handle.refresh()
    expect(handle.isPolling.value).toBe(false)
    expect(handle.consecutiveFailCount.value).toBe(3)

    // restart 后让 mock 改为成功
    infoMock.mockResolvedValue({ clientCounts: 1, curConns: 1 })
    proxiesMock.mockResolvedValue({ proxies: {} })
    handle.restart()
    expect(handle.consecutiveFailCount.value).toBe(0)
    expect(handle.error.value).toBeNull()
    expect(handle.isPolling.value).toBe(true)
  })
})

describe('useServerRuntime — visibilitychange 自动暂停 / 恢复（FR-5.3 / AC-7 / BC-7）', () => {
  it('hidden → polling 暂停；visible → 自动恢复 + 立即拉一次', async () => {
    vi.useFakeTimers()
    let hidden = false
    const { handle } = mountHolder({
      intervalMs: 1000,
      visibilityHidden: () => hidden,
    })
    handle.start()
    vi.advanceTimersByTime(1000)
    await flush()
    const before = infoMock.mock.calls.length
    expect(before).toBeGreaterThanOrEqual(1)

    // 模拟 tab 切后台
    hidden = true
    document.dispatchEvent(new Event('visibilitychange'))
    expect(handle.isPolling.value).toBe(false)

    vi.advanceTimersByTime(5000)
    await flush()
    // 后台期间 polling 不增长
    expect(infoMock.mock.calls.length).toBe(before)

    // 切回前台
    hidden = false
    document.dispatchEvent(new Event('visibilitychange'))
    await flush()
    expect(handle.isPolling.value).toBe(true)
    // 切回时立即拉一次
    expect(infoMock.mock.calls.length).toBeGreaterThan(before)
  })

  it('BC-7：用户显式 stop 后切后台 → 切回不自动恢复', async () => {
    vi.useFakeTimers()
    let hidden = false
    const { handle } = mountHolder({
      intervalMs: 1000,
      visibilityHidden: () => hidden,
    })
    handle.start()
    handle.stop()  // 用户显式暂停
    expect(handle.isPolling.value).toBe(false)

    hidden = true
    document.dispatchEvent(new Event('visibilitychange'))
    expect(handle.isPolling.value).toBe(false)

    hidden = false
    document.dispatchEvent(new Event('visibilitychange'))
    await flush()
    // 用户意图优先
    expect(handle.isPolling.value).toBe(false)
  })
})

describe('useServerRuntime — onUnmounted 清理（FR-5.4 / AC-10）', () => {
  it('unmount → setInterval 清除 + visibilitychange listener 解绑', async () => {
    vi.useFakeTimers()
    const removeSpy = vi.spyOn(document, 'removeEventListener')
    const { wrapper, handle } = mountHolder({ intervalMs: 1000 })
    handle.start()
    vi.advanceTimersByTime(1000)
    await flush()
    const callsBefore = infoMock.mock.calls.length

    wrapper.unmount()

    vi.advanceTimersByTime(5000)
    await flush()
    // unmount 后 timer 应被清除
    expect(infoMock.mock.calls.length).toBe(callsBefore)
    // visibilitychange listener 应被解绑
    const listenerRemoved = removeSpy.mock.calls.some(
      (c) => c[0] === 'visibilitychange',
    )
    expect(listenerRemoved).toBe(true)
    removeSpy.mockRestore()
  })

  it('BC-5 epoch race：unmount 后 in-flight 响应到达不写 ref', async () => {
    let resolveInfo!: (v: { clientCounts: number; curConns: number }) => void
    infoMock.mockImplementationOnce(
      () => new Promise((res) => { resolveInfo = res }),
    )
    proxiesMock.mockResolvedValueOnce({ proxies: {} })
    const { wrapper, handle } = mountHolder()
    const p = handle.refresh()
    // 在响应到来前 unmount
    wrapper.unmount()
    // unmount 自增 epoch；现在让响应到达
    resolveInfo({ clientCounts: 999, curConns: 999 })
    await p
    // 响应被丢弃 → info 仍为 null
    expect(handle.info.value).toBeNull()
  })
})

describe('useServerRuntime — extractErrorMessage 路径', () => {
  it('axios-like error → error 写入 message 字符串', async () => {
    // 模拟一个 plain Error，extractErrorMessage 走 fallback
    infoMock.mockRejectedValueOnce(new Error('network down'))
    proxiesMock.mockRejectedValueOnce(new Error('network down'))
    const { handle } = mountHolder()
    await handle.refresh()
    expect(typeof handle.error.value).toBe('string')
    // 至少落到 fallback 文案
    expect(handle.error.value!.length).toBeGreaterThan(0)
  })
})
