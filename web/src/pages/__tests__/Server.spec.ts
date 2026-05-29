// T-047 / frontend-honest-states · A1 + B1
// Server.vue mount × 三态（loading / error+retry / loaded）+ Dashboard 字段校验。
//
// 关键模式（insight L1 / L2 / L14 + T-043 getExposed）：
//   - vi.mock('naive-ui') importOriginal + 6 方法 stub
//   - Holder 包 NConfigProvider + NMessageProvider（NMessageProvider 必须在外层）
//   - 读 defineExpose 句柄用 getExposed<T>，禁 wrapper.vm.__testing
//   - API 失败用 apiError() 构造 axios 形状错误，禁 new Error()（extractErrorMessage 才透传）

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { getExposed } from '../../test-utils/exposed'
import { apiError } from '../../test-utils/apiError'
import { defineComponent, h, nextTick } from 'vue'
import { NConfigProvider, NMessageProvider } from 'naive-ui'
import type { FormInst } from 'naive-ui'

vi.mock('../../api/server', () => ({
  apiGetServer: vi.fn(),
  apiPutServer: vi.fn(),
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

import Server from '../Server.vue'
import * as api from '../../api/server'

const getMock = vi.mocked(api.apiGetServer)

interface ServerForm {
  bindPort: number
  authToken: string
  dashboardEnabled: boolean
  dashboardPort: number
  dashboardUser: string
  dashboardPass: string
}

interface TestingHandle {
  form: { value: ServerForm }
  loading: { value: boolean }
  loadError: { value: string | null }
  saving: { value: boolean }
  loadConfig: (reveal?: boolean) => Promise<void>
  handleSave: () => Promise<void>
  formRef: { value: FormInst | null }
}

function mountPage() {
  const Holder = defineComponent({
    setup() {
      return () =>
        h(NConfigProvider, null, {
          default: () =>
            h(NMessageProvider, null, {
              default: () => h(Server),
            }),
        })
    },
  })
  return mount(Holder, { attachTo: document.body })
}

function getTesting(wrapper: ReturnType<typeof mountPage>): TestingHandle {
  return getExposed<TestingHandle>(wrapper, Server)
}

async function settle(n = 6): Promise<void> {
  for (let i = 0; i < n; i++) await nextTick()
}

const HAPPY_CFG = {
  bindPort: 7100,
  authToken: 'sekret',
  dashboardEnabled: true,
  dashboardPort: 7500,
  dashboardUser: 'realuser',
  dashboardPass: 'realpass',
  allowPorts: [],
}

beforeEach(() => {
  getMock.mockReset()
  getMock.mockResolvedValue({ ...HAPPY_CFG })
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('Server.vue — 三态：loading（A1）', () => {
  it('mount 立即（settle 前）loading=true 且未渲染表单', async () => {
    // getMock 返回一个永不 resolve 的 promise，定格在 loading 态
    getMock.mockReset()
    getMock.mockReturnValue(new Promise<typeof HAPPY_CFG>(() => {}))
    const w = mountPage()
    await nextTick()
    const t = getTesting(w)
    expect(t.loading.value).toBe(true)
    expect(t.loadError.value).toBeNull()
    // loading 态不应渲染"监听端口"表单项
    expect(w.text()).not.toContain('监听端口')
  })
})

describe('Server.vue — 三态：loaded 渲染真实值（A1）', () => {
  it('成功 → loading=false / loadError=null / 表单填入真实值', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.loading.value).toBe(false)
    expect(t.loadError.value).toBeNull()
    expect(t.form.value.bindPort).toBe(7100)
    expect(t.form.value.dashboardEnabled).toBe(true)
    expect(t.form.value.dashboardUser).toBe('realuser')
    expect(w.text()).toContain('监听端口')
  })
})

describe('Server.vue — 三态：error + 重试（A1）', () => {
  it('apiGetServer reject（apiError）→ loadError 透传消息 + 错误态文案 + 重试按钮', async () => {
    getMock.mockReset()
    getMock.mockRejectedValue(apiError('后端 500：读取 frps 配置失败'))
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.loading.value).toBe(false)
    expect(t.loadError.value).toBe('后端 500：读取 frps 配置失败')
    expect(w.text()).toContain('加载服务端配置失败')
    expect(w.text()).toContain('重试')
  })

  it('错误态点重试 → loadConfig 再次调用；成功后回到 loaded', async () => {
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
    expect(t.loading.value).toBe(false)
    expect(t.form.value.bindPort).toBe(7100)
  })
})

describe('Server.vue — Dashboard 字段校验（B1）', () => {
  it('启用 dashboard + 空密码 → validate 失败（阻止提交）', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.form.value.dashboardEnabled = true
    t.form.value.dashboardUser = 'admin'
    t.form.value.dashboardPass = ''
    await nextTick()
    await expect(t.formRef.value!.validate()).rejects.toBeTruthy()
  })

  it('启用 dashboard + 空用户名 → validate 失败', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.form.value.dashboardEnabled = true
    t.form.value.dashboardUser = ''
    t.form.value.dashboardPass = 'somepass'
    await nextTick()
    await expect(t.formRef.value!.validate()).rejects.toBeTruthy()
  })

  it('启用 dashboard + 非法端口（0）→ validate 失败', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.form.value.dashboardEnabled = true
    t.form.value.dashboardUser = 'admin'
    t.form.value.dashboardPass = 'somepass'
    t.form.value.dashboardPort = 0
    await nextTick()
    await expect(t.formRef.value!.validate()).rejects.toBeTruthy()
  })

  it('启用 dashboard + 越界端口（70000）→ validate 失败', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.form.value.dashboardEnabled = true
    t.form.value.dashboardUser = 'admin'
    t.form.value.dashboardPass = 'somepass'
    t.form.value.dashboardPort = 70000
    await nextTick()
    await expect(t.formRef.value!.validate()).rejects.toBeTruthy()
  })

  it('启用 dashboard + 合法 user/pass/port → validate 通过', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.form.value.dashboardEnabled = true
    t.form.value.dashboardUser = 'admin'
    t.form.value.dashboardPass = 'goodpass'
    t.form.value.dashboardPort = 7500
    await nextTick()
    await expect(t.formRef.value!.validate()).resolves.toBeTruthy()
  })

  it('未启用 dashboard → 空 user/pass 不阻塞 validate（规则仅在启用时生效）', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.form.value.dashboardEnabled = false
    t.form.value.dashboardUser = ''
    t.form.value.dashboardPass = ''
    await nextTick()
    await expect(t.formRef.value!.validate()).resolves.toBeTruthy()
  })
})

// ## Adversarial tests
describe('Server.vue — Adversarial', () => {
  it('加载失败时绝不渲染默认表单值（不能让用户把默认值误当真实配置）', async () => {
    getMock.mockReset()
    getMock.mockRejectedValue(apiError('凭据失效'))
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    // 失败态：表单（"监听端口" / "鉴权 Token"）必须不可见
    expect(t.loadError.value).not.toBeNull()
    expect(w.text()).not.toContain('监听端口')
    expect(w.text()).not.toContain('鉴权 Token')
    // 错误态文案可见
    expect(w.text()).toContain('加载服务端配置失败')
  })

  it('三态互斥：error!=null 时 loading 必为 false（不能同时转圈+报错）', async () => {
    getMock.mockReset()
    getMock.mockRejectedValue(apiError('x'))
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.loadError.value).not.toBeNull()
    expect(t.loading.value).toBe(false)
  })

  it('启用 dashboard 但空 pass 时 handleSave 不调用 apiPutServer', async () => {
    const putMock = vi.mocked(api.apiPutServer)
    putMock.mockReset()
    putMock.mockResolvedValue({ ...HAPPY_CFG })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.form.value.dashboardEnabled = true
    t.form.value.dashboardUser = 'admin'
    t.form.value.dashboardPass = ''
    await nextTick()
    await t.handleSave()
    await settle()
    expect(putMock).not.toHaveBeenCalled()
  })
})
