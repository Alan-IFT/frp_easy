// T-047 / frontend-honest-states · A1
// Client.vue mount × 三态（loading / error+retry / loaded）。
//
// 关键模式（insight L1 / L2 / L14 + T-043 getExposed）。

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { getExposed } from '../../test-utils/exposed'
import { apiError } from '../../test-utils/apiError'
import { defineComponent, h, nextTick } from 'vue'
import { NConfigProvider, NMessageProvider } from 'naive-ui'

vi.mock('../../api/frpclient', () => ({
  apiGetClient: vi.fn(),
  apiPutClient: vi.fn(),
}))

vi.mock('naive-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useMessage: () => ({
      error: vi.fn(),
      success: vi.fn(),
      warning: vi.fn(),
      info: vi.fn(),
      loading: vi.fn(),
      destroyAll: vi.fn(),
    }),
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

import Client from '../Client.vue'
import * as api from '../../api/frpclient'

const getMock = vi.mocked(api.apiGetClient)

interface ClientForm {
  serverAddr: string
  serverPort: number
  authToken: string
}

interface TestingHandle {
  form: { value: ClientForm }
  loading: { value: boolean }
  loadError: { value: string | null }
  saving: { value: boolean }
  loadConfig: (reveal?: boolean) => Promise<void>
  handleSave: () => Promise<void>
}

function mountPage() {
  const Holder = defineComponent({
    setup() {
      return () =>
        h(NConfigProvider, null, {
          default: () =>
            h(NMessageProvider, null, {
              default: () => h(Client),
            }),
        })
    },
  })
  return mount(Holder, { attachTo: document.body })
}

function getTesting(wrapper: ReturnType<typeof mountPage>): TestingHandle {
  return getExposed<TestingHandle>(wrapper, Client)
}

async function settle(n = 6): Promise<void> {
  for (let i = 0; i < n; i++) await nextTick()
}

const HAPPY_CFG = {
  serverAddr: 'frps.example.com',
  serverPort: 7001,
  authToken: 'token-x',
}

beforeEach(() => {
  getMock.mockReset()
  getMock.mockResolvedValue({ ...HAPPY_CFG })
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('Client.vue — 三态：loading（A1）', () => {
  it('mount 立即 loading=true 且未渲染表单', async () => {
    getMock.mockReset()
    getMock.mockReturnValue(new Promise<typeof HAPPY_CFG>(() => {}))
    const w = mountPage()
    await nextTick()
    const t = getTesting(w)
    expect(t.loading.value).toBe(true)
    expect(t.loadError.value).toBeNull()
    expect(w.text()).not.toContain('服务端地址')
  })
})

describe('Client.vue — 三态：loaded 渲染真实值（A1）', () => {
  it('成功 → loading=false / loadError=null / 表单填入真实值', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.loading.value).toBe(false)
    expect(t.loadError.value).toBeNull()
    expect(t.form.value.serverAddr).toBe('frps.example.com')
    expect(t.form.value.serverPort).toBe(7001)
    expect(w.text()).toContain('服务端地址')
  })
})

describe('Client.vue — 三态：error + 重试（A1）', () => {
  it('apiGetClient reject（apiError）→ loadError 透传 + 错误文案 + 重试按钮', async () => {
    getMock.mockReset()
    getMock.mockRejectedValue(apiError('后端 500：读取 frpc 配置失败'))
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.loading.value).toBe(false)
    expect(t.loadError.value).toBe('后端 500：读取 frpc 配置失败')
    expect(w.text()).toContain('加载客户端配置失败')
    expect(w.text()).toContain('重试')
  })

  it('错误态重试成功 → 回到 loaded 并填入真实值', async () => {
    getMock.mockReset()
    getMock.mockRejectedValueOnce(apiError('临时失败'))
    getMock.mockResolvedValue({ ...HAPPY_CFG })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.loadError.value).toBe('临时失败')
    await t.loadConfig()
    await settle()
    expect(t.loadError.value).toBeNull()
    expect(t.form.value.serverAddr).toBe('frps.example.com')
  })
})

// ## Adversarial tests
describe('Client.vue — Adversarial', () => {
  it('加载失败时绝不渲染默认表单值（serverAddr 输入框不可见）', async () => {
    getMock.mockReset()
    getMock.mockRejectedValue(apiError('凭据失效'))
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.loadError.value).not.toBeNull()
    expect(w.text()).not.toContain('服务端地址')
    expect(w.text()).not.toContain('鉴权 Token')
    expect(w.text()).toContain('加载客户端配置失败')
  })

  it('三态互斥：error!=null 时 loading 必为 false', async () => {
    getMock.mockReset()
    getMock.mockRejectedValue(apiError('x'))
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.loadError.value).not.toBeNull()
    expect(t.loading.value).toBe(false)
  })
})
