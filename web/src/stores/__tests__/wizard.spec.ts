// T-051 frontend-test-coverage · B-1
// wizard store 专属测试：checkWizard / completeWizard 状态流转。
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useWizardStore } from '../wizard'
import { apiError } from '../../test-utils/apiError'

vi.mock('../../api/wizard', () => ({
  apiGetWizardStatus: vi.fn(),
  apiWizardComplete: vi.fn(),
}))

import * as wizardApi from '../../api/wizard'

describe('useWizardStore — 初始状态', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('wizardHandled=false / shouldShow=false / checked=false', () => {
    const store = useWizardStore()
    expect(store.wizardHandled).toBe(false)
    expect(store.shouldShow).toBe(false)
    expect(store.checked).toBe(false)
  })
})

describe('useWizardStore.checkWizard', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('成功 handled=false shouldShow=true → 反映状态 + checked=true', async () => {
    vi.mocked(wizardApi.apiGetWizardStatus).mockResolvedValueOnce({
      handled: false,
      shouldShow: true,
    })
    const store = useWizardStore()
    await store.checkWizard()

    expect(store.wizardHandled).toBe(false)
    expect(store.shouldShow).toBe(true)
    expect(store.checked).toBe(true)
  })

  it('成功 handled=true shouldShow=false（已完成过）', async () => {
    vi.mocked(wizardApi.apiGetWizardStatus).mockResolvedValueOnce({
      handled: true,
      shouldShow: false,
    })
    const store = useWizardStore()
    await store.checkWizard()

    expect(store.wizardHandled).toBe(true)
    expect(store.shouldShow).toBe(false)
    expect(store.checked).toBe(true)
  })

  it('失败时 catch → shouldShow=false（不弹），但 finally 仍置 checked=true', async () => {
    vi.mocked(wizardApi.apiGetWizardStatus).mockRejectedValueOnce(
      apiError('状态接口不可用', 503),
    )
    const store = useWizardStore()
    await store.checkWizard()

    // catch 分支：出错时不显示向导
    expect(store.shouldShow).toBe(false)
    // finally 分支：无论成败都标记已检查
    expect(store.checked).toBe(true)
  })

  it('失败时不抛异常（吞掉错误，让 UI 正常进首屏）', async () => {
    vi.mocked(wizardApi.apiGetWizardStatus).mockRejectedValueOnce(new Error('network'))
    const store = useWizardStore()
    await expect(store.checkWizard()).resolves.toBeUndefined()
  })
})

describe('useWizardStore.completeWizard', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('成功 → wizardHandled=true + shouldShow=false', async () => {
    vi.mocked(wizardApi.apiWizardComplete).mockResolvedValueOnce(undefined)
    const store = useWizardStore()
    // 预置一个 shouldShow=true 的场景
    store.shouldShow = true
    await store.completeWizard()

    expect(wizardApi.apiWizardComplete).toHaveBeenCalledTimes(1)
    expect(store.wizardHandled).toBe(true)
    expect(store.shouldShow).toBe(false)
  })

  it('失败时异常传播，状态保持原样（不误标 handled）', async () => {
    vi.mocked(wizardApi.apiWizardComplete).mockRejectedValueOnce(
      apiError('保存失败', 500),
    )
    const store = useWizardStore()
    store.shouldShow = true

    await expect(store.completeWizard()).rejects.toBeTruthy()
    // await 在赋值前抛出 → 状态未变更
    expect(store.wizardHandled).toBe(false)
    expect(store.shouldShow).toBe(true)
  })
})
