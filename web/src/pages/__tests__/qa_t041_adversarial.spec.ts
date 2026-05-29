// T-041 / server-monitor-page-ui · 06 QA stage adversarial
//
// 4 个反向构造场景，独立于主 spec：
//   ADV-1（AC-4）：frps 进程不可达 → 友好引导 + retry 按钮存在
//   ADV-2（AC-5）：dashboard 凭据校验失败 → "前往服务端配置" 按钮存在
//   ADV-3（AC-7）：tab 切后台 → polling 暂停（setInterval 不再触发）
//   ADV-4（D-6）：连续 3 次失败 → isPolling 切 false（用反向假设证伪：MAX_FAIL > 失败次数时 isPolling 应保持 true）

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { getExposed } from '../../test-utils/exposed'
import { apiError } from '../../test-utils/apiError'
import { defineComponent, h, nextTick } from 'vue'
import { NConfigProvider, NMessageProvider } from 'naive-ui'

vi.mock('../../api/serverRuntime', () => ({
  apiGetServerRuntimeInfo: vi.fn(),
  apiGetServerRuntimeProxies: vi.fn(),
  apiGetServerRuntimeTraffic: vi.fn(),
}))

vi.mock('naive-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useMessage: () => ({
      error: vi.fn(), success: vi.fn(), warning: vi.fn(),
      info: vi.fn(), loading: vi.fn(), destroyAll: vi.fn(),
    }),
    useDialog: () => ({
      info: vi.fn(), success: vi.fn(), warning: vi.fn(),
      error: vi.fn(), create: vi.fn(), destroyAll: vi.fn(),
    }),
    useNotification: () => ({
      info: vi.fn(), success: vi.fn(), warning: vi.fn(),
      error: vi.fn(), destroyAll: vi.fn(),
    }),
    useLoadingBar: () => ({ start: vi.fn(), finish: vi.fn(), error: vi.fn() }),
    useModal: () => ({ create: vi.fn() }),
  }
})

const pushSpy = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushSpy }),
}))

import ServerMonitor from '../ServerMonitor.vue'
import * as api from '../../api/serverRuntime'
import { useServerRuntime } from '../../composables/useServerRuntime'

const infoMock = vi.mocked(api.apiGetServerRuntimeInfo)
const proxiesMock = vi.mocked(api.apiGetServerRuntimeProxies)

interface TestingHandle {
  rt: {
    info: { value: unknown }
    proxies: { value: unknown }
    isPolling: { value: boolean }
    error: { value: string | null }
    consecutiveFailCount: { value: number }
  }
  firstLoadFailed: { value: boolean }
  goServerHint: { value: boolean }
  showFailureBanner: { value: boolean }
}

function mountPage() {
  const Holder = defineComponent({
    setup() {
      return () =>
        h(NConfigProvider, null, {
          default: () =>
            h(NMessageProvider, null, {
              default: () => h(ServerMonitor),
            }),
        })
    },
  })
  return mount(Holder, { attachTo: document.body })
}

function getTesting(wrapper: ReturnType<typeof mountPage>): TestingHandle {
  return getExposed<TestingHandle>(wrapper, ServerMonitor)
}

async function settle(n = 5) {
  for (let i = 0; i < n; i++) await nextTick()
}

beforeEach(() => {
  infoMock.mockReset()
  proxiesMock.mockReset()
  pushSpy.mockReset()
})

afterEach(() => {
  vi.useRealTimers()
  document.body.innerHTML = ''
})

describe('ADV-1（AC-4）frps 进程不可达 → 友好引导 + retry 按钮', () => {
  it('mount 时 API reject 含 "frps 进程不可达" → firstLoadFailed=true + 文案 + 重试按钮', async () => {
    const err = apiError('frps 进程不可达。请确认 frps 已启动且 dashboard 端口配置正确。')
    infoMock.mockRejectedValue(err)
    proxiesMock.mockRejectedValue(err)

    const w = mountPage()
    await settle(8)
    const t = getTesting(w)

    // 反向证伪 1：firstLoadFailed = true（说明判定逻辑命中）
    expect(t.firstLoadFailed.value).toBe(true)
    // 反向证伪 2：文案含错误细节（说明 NResult description 传入）
    expect(w.text()).toContain('frps 进程不可达')
    // 反向证伪 3：重试按钮可见
    expect(w.text()).toContain('重试')
    // 反向证伪 4：不应有"前往服务端配置"（因为这是 "frps 不可达" 不是 dashboard 配置错）
    expect(t.goServerHint.value).toBe(false)
    expect(w.text()).not.toContain('前往服务端配置')
  })
})

describe('ADV-2（AC-5）dashboard 凭据校验失败 → 前往服务端配置按钮', () => {
  it('错误含 "凭据" → goServerHint=true + 按钮可见 + 点击导航 /server', async () => {
    const err = apiError('frps dashboard 凭据校验失败（401）。请到 Server 设置页清空 user/pass 由 frp_easy 重新生成。')
    infoMock.mockRejectedValue(err)
    proxiesMock.mockRejectedValue(err)

    const w = mountPage()
    await settle(8)
    const t = getTesting(w)

    expect(t.goServerHint.value).toBe(true)
    expect(w.text()).toContain('前往服务端配置')

    // 找到按钮并 click → router.push('/server')
    const buttons = w.findAll('button')
    const goBtn = buttons.find((b) => b.text().includes('前往服务端配置'))
    expect(goBtn).toBeTruthy()
    await goBtn!.trigger('click')
    expect(pushSpy).toHaveBeenCalledWith('/server')
  })

  it('反向证伪：错误不含 "凭据" / "dashboard 未启用" → goServerHint=false 且按钮不可见', async () => {
    const err = apiError('一般网络错误')
    infoMock.mockRejectedValue(err)
    proxiesMock.mockRejectedValue(err)

    const w = mountPage()
    await settle(8)
    const t = getTesting(w)

    expect(t.goServerHint.value).toBe(false)
    expect(w.text()).not.toContain('前往服务端配置')
  })
})

describe('ADV-3（AC-7）tab 切后台 → polling 暂停（spy setInterval）', () => {
  it('hidden 后 setInterval 不再增长；恢复后立即拉一次', async () => {
    vi.useFakeTimers()
    infoMock.mockResolvedValue({ clientCounts: 0, curConns: 0 })
    proxiesMock.mockResolvedValue({ proxies: {} })

    // 直接用 composable + visibilityHidden inject，绕过 ServerMonitor template
    let hidden = false
    let handle!: ReturnType<typeof useServerRuntime>
    const Holder = defineComponent({
      setup() {
        handle = useServerRuntime(1000, { visibilityHidden: () => hidden })
        return () => h('div')
      },
    })
    const w = mount(Holder)
    handle.start()

    vi.advanceTimersByTime(1000)
    await nextTick()
    const callsBeforeHide = infoMock.mock.calls.length
    expect(callsBeforeHide).toBeGreaterThanOrEqual(1)

    // 模拟 hidden
    hidden = true
    document.dispatchEvent(new Event('visibilitychange'))
    expect(handle.isPolling.value).toBe(false)

    // 反向证伪 1：后台期间 advance 5s → setInterval 不再触发
    vi.advanceTimersByTime(5000)
    await nextTick()
    expect(infoMock.mock.calls.length).toBe(callsBeforeHide)

    // 反向证伪 2：恢复 visible → polling 重启 + 立即拉一次
    hidden = false
    document.dispatchEvent(new Event('visibilitychange'))
    await nextTick()
    expect(handle.isPolling.value).toBe(true)
    expect(infoMock.mock.calls.length).toBeGreaterThan(callsBeforeHide)

    w.unmount()
  })
})

describe('ADV-4（D-6）3 次失败自动停 — 反向证伪：单测可证 isPolling 真的因连续失败切 false', () => {
  it('正向：连续 3 次 reject → isPolling=false + showFailureBanner=true', async () => {
    const err = new Error('upstream')
    infoMock.mockRejectedValue(err)
    proxiesMock.mockRejectedValue(err)

    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    // 已 1 次（mount 时 refresh）
    expect(t.rt.consecutiveFailCount.value).toBe(1)
    expect(t.rt.isPolling.value).toBe(true)

    // 再手动触发 2 次让总失败数达到 3
    const handle = getExposed<TestingHandle & { onRefreshClick: () => Promise<void> }>(w, ServerMonitor)
    await handle.onRefreshClick()
    await handle.onRefreshClick()
    await settle()

    expect(t.rt.consecutiveFailCount.value).toBe(3)
    // 反向证伪：isPolling 应被 D-6 自动切 false
    expect(t.rt.isPolling.value).toBe(false)
    expect(t.showFailureBanner.value).toBe(true)
    expect(w.text()).toContain('自动刷新已停止')
  })

  it('反向证伪 2：只 2 次失败 → isPolling 仍 true（说明阈值真的是 3 不是 2）', async () => {
    const err = new Error('upstream')
    infoMock.mockRejectedValue(err)
    proxiesMock.mockRejectedValue(err)

    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    // 已 1 次
    expect(t.rt.consecutiveFailCount.value).toBe(1)

    const handle = getExposed<TestingHandle & { onRefreshClick: () => Promise<void> }>(w, ServerMonitor)
    await handle.onRefreshClick()  // 第 2 次
    await settle()

    expect(t.rt.consecutiveFailCount.value).toBe(2)
    // 阈值 3 → 2 次时仍 polling
    expect(t.rt.isPolling.value).toBe(true)
    expect(t.showFailureBanner.value).toBe(false)
  })
})
