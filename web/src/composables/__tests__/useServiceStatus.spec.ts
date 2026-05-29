// T-051 frontend-test-coverage · B-3
// useServiceStatus 单测：mock api，测正常 / 失败 / loading 流转 + needsFix 联合判定。
// composable 用 onMounted 自动 refresh，故用 setup-host 组件挂载触发生命周期。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h, nextTick } from 'vue'

vi.mock('../../api/system', () => ({
  apiGetServiceStatus: vi.fn(),
}))

import { useServiceStatus } from '../useServiceStatus'
import * as systemApi from '../../api/system'
import type { SystemServiceStatusResponse } from '../../types'

type Handle = ReturnType<typeof useServiceStatus>

// Holder：让 composable 在真正的 Vue setup 上下文里跑，onMounted 才触发。
function mountHolder() {
  let handle!: Handle
  const Holder = defineComponent({
    setup() {
      handle = useServiceStatus()
      return () => h('div')
    },
  })
  const wrapper = mount(Holder)
  return { wrapper, handle: handle! }
}

function svcStatus(over: Partial<SystemServiceStatusResponse> = {}): SystemServiceStatusResponse {
  return {
    supervised: true,
    supervisor: 'systemd',
    boot_autostart: true,
    run_as: 'root',
    auto_restore: { enabled_kinds: [] },
    ...over,
  }
}

async function flush(n = 3) {
  for (let i = 0; i < n; i++) await nextTick()
}

const statusMock = vi.mocked(systemApi.apiGetServiceStatus)

beforeEach(() => {
  vi.clearAllMocks()
})

describe('useServiceStatus — onMounted 自动 refresh', () => {
  it('mount 即拉取一次 status', async () => {
    statusMock.mockResolvedValueOnce(svcStatus())
    mountHolder()
    await flush()
    expect(statusMock).toHaveBeenCalledTimes(1)
  })
})

describe('useServiceStatus — 成功流转', () => {
  it('成功 → status 写入 / error=null / loading 收尾 false', async () => {
    const data = svcStatus({ supervisor: 'windows-service' })
    statusMock.mockResolvedValueOnce(data)
    const { handle } = mountHolder()
    await flush()

    expect(handle.status.value).toEqual(data)
    expect(handle.error.value).toBeNull()
    expect(handle.loading.value).toBe(false)
  })

  it('refresh() 期间 loading=true，完成后回 false', async () => {
    let resolveStatus!: (v: SystemServiceStatusResponse) => void
    statusMock.mockImplementationOnce(
      () => new Promise<SystemServiceStatusResponse>((res) => { resolveStatus = res }),
    )
    const { handle } = mountHolder()
    // onMounted 触发的 refresh 仍 pending
    expect(handle.loading.value).toBe(true)

    resolveStatus(svcStatus())
    await flush()
    expect(handle.loading.value).toBe(false)
  })
})

describe('useServiceStatus — 失败流转', () => {
  it('失败 → error 写入 message / status 保持 null', async () => {
    statusMock.mockReset()
    // useServiceStatus 用 e instanceof Error ? e.message : '加载失败'，
    // 这里刻意用普通 Error 测取 message 分支（非 extractErrorMessage 路径）。
    statusMock.mockRejectedValueOnce(new Error('probe timeout'))
    const { handle } = mountHolder()
    await flush()

    expect(handle.error.value).toBe('probe timeout')
    expect(handle.status.value).toBeNull()
    expect(handle.loading.value).toBe(false)
  })

  it('非 Error 抛出 → error 落到 "加载失败" fallback', async () => {
    statusMock.mockReset()
    statusMock.mockRejectedValueOnce('string error')
    const { handle } = mountHolder()
    await flush()

    expect(handle.error.value).toBe('加载失败')
  })

  it('refresh 失败后再成功 → error 被清空', async () => {
    statusMock.mockReset()
    statusMock.mockRejectedValueOnce(new Error('boom'))
    const { handle } = mountHolder()
    await flush()
    expect(handle.error.value).toBe('boom')

    statusMock.mockResolvedValueOnce(svcStatus())
    await handle.refresh()
    expect(handle.error.value).toBeNull()
  })
})

describe('useServiceStatus — needsFix 联合判定', () => {
  it('status=null → needsFix=false', async () => {
    statusMock.mockRejectedValueOnce(new Error('x'))
    const { handle } = mountHolder()
    await flush()
    expect(handle.status.value).toBeNull()
    expect(handle.needsFix.value).toBe(false)
  })

  it('supervised=true && boot_autostart=true → needsFix=false', async () => {
    statusMock.mockResolvedValueOnce(svcStatus({ supervised: true, boot_autostart: true }))
    const { handle } = mountHolder()
    await flush()
    expect(handle.needsFix.value).toBe(false)
  })

  it('supervised=false → needsFix=true', async () => {
    statusMock.mockResolvedValueOnce(
      svcStatus({ supervised: false, supervisor: 'none', boot_autostart: true }),
    )
    const { handle } = mountHolder()
    await flush()
    expect(handle.needsFix.value).toBe(true)
  })

  it('boot_autostart=false → needsFix=true', async () => {
    statusMock.mockResolvedValueOnce(svcStatus({ supervised: true, boot_autostart: false }))
    const { handle } = mountHolder()
    await flush()
    expect(handle.needsFix.value).toBe(true)
  })
})
