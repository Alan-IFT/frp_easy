// T-047 / frontend-honest-states · A2
// Dashboard.vue — 自动启动开关状态获取失败不静默。
//
// 关键模式（insight L1 / L2 / L14 + T-043 getExposed）：
//   - vi.mock('naive-ui') importOriginal + 6 方法 stub；useMessage 复用单例 spy（断言 warning）
//   - mock ../api/mode / ../api/proc / ../api/system 让 mount 不触网
//   - 读句柄用 getExposed<T>，API 失败用 apiError()

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { getExposed } from '../../test-utils/exposed'
import { apiError } from '../../test-utils/apiError'
import { defineComponent, h, nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { NConfigProvider, NMessageProvider } from 'naive-ui'

vi.mock('../../api/mode', () => ({
  apiGetMode: vi.fn(),
  apiPutMode: vi.fn(),
}))

vi.mock('../../api/proc', () => ({
  apiGetProcStatus: vi.fn().mockResolvedValue({
    frpc: { kind: 'frpc', state: 'stopped', pid: 0, lastErr: '', changedAt: '' },
    frps: { kind: 'frps', state: 'stopped', pid: 0, lastErr: '', changedAt: '' },
  }),
  apiStartProc: vi.fn(),
  apiStopProc: vi.fn(),
  apiRestartProc: vi.fn(),
}))

vi.mock('../../api/system', () => ({
  apiGetReady: vi.fn().mockResolvedValue({ binMissing: [] }),
  apiGetServiceStatus: vi.fn().mockResolvedValue({
    supervised: true,
    boot_autostart: true,
    run_as: 'alan',
    supervisor: 'systemd',
    auto_restore: { enabled_kinds: [], last_runs: {} },
  }),
}))

// useMessage 单例 spy：所有调用都打到同一组 spy 上，便于断言 warning
const messageSpies = {
  error: vi.fn(),
  success: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
  loading: vi.fn(),
  destroyAll: vi.fn(),
}

vi.mock('naive-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useMessage: () => messageSpies,
    useDialog: () => ({
      info: vi.fn(),
      success: vi.fn(),
      warning: vi.fn(),
      error: vi.fn(),
      create: vi.fn(),
      destroyAll: vi.fn(),
    }),
    useNotification: () => ({
      info: vi.fn(),
      success: vi.fn(),
      warning: vi.fn(),
      error: vi.fn(),
      destroyAll: vi.fn(),
    }),
    useLoadingBar: () => ({
      start: vi.fn(),
      finish: vi.fn(),
      error: vi.fn(),
    }),
    useModal: () => ({ create: vi.fn() }),
  }
})

import Dashboard from '../Dashboard.vue'
import * as modeApi from '../../api/mode'

const getModeMock = vi.mocked(modeApi.apiGetMode)

interface TestingHandle {
  modeState: { frpc: boolean; frps: boolean }
  modeLoading: { frpc: boolean; frps: boolean }
  modeFetchFailed: { value: boolean }
  fetchMode: () => Promise<void>
  retryFetchMode: () => void
  handleModeToggle: (kind: 'frpc' | 'frps', enabled: boolean) => Promise<void>
}

function mountPage() {
  const Holder = defineComponent({
    setup() {
      return () =>
        h(NConfigProvider, null, {
          default: () =>
            h(NMessageProvider, null, {
              default: () => h(Dashboard),
            }),
        })
    },
  })
  return mount(Holder, { attachTo: document.body })
}

function getTesting(wrapper: ReturnType<typeof mountPage>): TestingHandle {
  return getExposed<TestingHandle>(wrapper, Dashboard)
}

async function settle(n = 6): Promise<void> {
  for (let i = 0; i < n; i++) await nextTick()
}

beforeEach(() => {
  setActivePinia(createPinia())
  getModeMock.mockReset()
  Object.values(messageSpies).forEach((s) => s.mockReset())
  getModeMock.mockResolvedValue({ frpc: true, frps: false })
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('Dashboard.vue — fetchMode happy path（A2）', () => {
  it('成功 → modeState 写入真实值 / modeFetchFailed=false / 不弹 warning', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.modeState.frpc).toBe(true)
    expect(t.modeState.frps).toBe(false)
    expect(t.modeFetchFailed.value).toBe(false)
    expect(messageSpies.warning).not.toHaveBeenCalled()
  })
})

describe('Dashboard.vue — fetchMode 失败不静默（A2）', () => {
  it('apiGetMode reject（apiError）→ message.warning 被调用 + 透传消息', async () => {
    getModeMock.mockReset()
    getModeMock.mockRejectedValue(apiError('后端 503：自动启动状态读取失败'))
    mountPage()
    await settle()
    expect(messageSpies.warning).toHaveBeenCalled()
    expect(messageSpies.warning).toHaveBeenCalledWith('后端 503：自动启动状态读取失败')
  })

  it('失败 → modeFetchFailed=true（开关呈失败态信号）', async () => {
    getModeMock.mockReset()
    getModeMock.mockRejectedValue(apiError('x'))
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.modeFetchFailed.value).toBe(true)
  })

  it('失败 → 模板出现"刷新状态"重试入口', async () => {
    getModeMock.mockReset()
    getModeMock.mockRejectedValue(apiError('x'))
    const w = mountPage()
    await settle()
    expect(w.text()).toContain('刷新状态')
  })

  it('retryFetchMode 在失败后成功 → modeFetchFailed 回到 false', async () => {
    getModeMock.mockReset()
    getModeMock.mockRejectedValueOnce(apiError('临时失败'))
    getModeMock.mockResolvedValue({ frpc: true, frps: true })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.modeFetchFailed.value).toBe(true)
    t.retryFetchMode()
    await settle()
    expect(t.modeFetchFailed.value).toBe(false)
    expect(t.modeState.frps).toBe(true)
  })
})

// ## Adversarial tests
describe('Dashboard.vue — Adversarial', () => {
  it('获取失败时 UI 不撒谎：必有可见失败信号（modeFetchFailed=true 或 warning 被调用）', async () => {
    getModeMock.mockReset()
    getModeMock.mockRejectedValue(apiError('凭据失效'))
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    // 不能"静默"：失败标记与 warning 至少其一为真（实现两者都满足）
    const honest = t.modeFetchFailed.value || messageSpies.warning.mock.calls.length > 0
    expect(honest).toBe(true)
    // 且开关默认 false 时不得被当作"真实=关"无声展示
    expect(t.modeFetchFailed.value).toBe(true)
  })

  it('成功路径不误报失败态（不能把正常状态标成获取失败）', async () => {
    getModeMock.mockReset()
    getModeMock.mockResolvedValue({ frpc: false, frps: false })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.modeFetchFailed.value).toBe(false)
    expect(w.text()).not.toContain('刷新状态')
  })
})
