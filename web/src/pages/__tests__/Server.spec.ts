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
  // T-058 (B)
  loadedSnapshot: { value: ServerForm | null }
  reloadConfirmShow: { value: boolean }
  isDirty: () => boolean
  handleReloadClick: () => void
  confirmReload: () => void
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

// T-058 (B)：重置 → 重新加载，dirty 防误丢
describe('Server.vue — 重新加载防误丢未保存编辑（B）', () => {
  it('文案：渲染"重新加载"而非旧"重置"', async () => {
    const w = mountPage()
    await settle()
    // 用渲染文本断言（不按 naive-ui 组件名查询 —— insight L45 / T-057 教训）
    expect(w.text()).toContain('重新加载')
    expect(w.text()).not.toContain('重置')
  })

  it('加载后未改 → isDirty()=false；handleReloadClick 直接重载（不弹确认）', async () => {
    getMock.mockReset()
    getMock.mockResolvedValue({ ...HAPPY_CFG })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.isDirty()).toBe(false)
    expect(t.reloadConfirmShow.value).toBe(false)
    const callsBefore = getMock.mock.calls.length
    t.handleReloadClick()
    await settle()
    // 不弹确认 + apiGetServer 再被调用一次（直接重载）
    expect(t.reloadConfirmShow.value).toBe(false)
    expect(getMock.mock.calls.length).toBe(callsBefore + 1)
  })

  it('改了字段使 dirty → handleReloadClick 弹确认，此刻 apiGetServer 未再调用', async () => {
    getMock.mockReset()
    getMock.mockResolvedValue({ ...HAPPY_CFG })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.form.value.bindPort = 9999 // 与快照 7100 不同 → dirty
    await nextTick()
    expect(t.isDirty()).toBe(true)
    const callsBefore = getMock.mock.calls.length
    t.handleReloadClick()
    await nextTick()
    expect(t.reloadConfirmShow.value).toBe(true)
    // 确认前绝不重载（防止"点了就丢"）
    expect(getMock.mock.calls.length).toBe(callsBefore)
  })

  it('dirty + 确认（confirmReload）→ apiGetServer 再被调用并覆盖回真实值', async () => {
    getMock.mockReset()
    getMock.mockResolvedValue({ ...HAPPY_CFG })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.form.value.bindPort = 9999
    await nextTick()
    t.handleReloadClick()
    await nextTick()
    const callsBefore = getMock.mock.calls.length
    t.confirmReload()
    await settle()
    expect(getMock.mock.calls.length).toBe(callsBefore + 1)
    expect(t.form.value.bindPort).toBe(7100) // 被重载覆盖回真实值
    expect(t.isDirty()).toBe(false)
  })

  it('dirty + 取消（不调 confirmReload）→ apiGetServer 不再调用，编辑保留', async () => {
    getMock.mockReset()
    getMock.mockResolvedValue({ ...HAPPY_CFG })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.form.value.bindPort = 9999
    await nextTick()
    t.handleReloadClick()
    await nextTick()
    const callsBefore = getMock.mock.calls.length
    // 取消 = 关闭弹窗但不触发 confirmReload（模拟用户点"取消"）
    t.reloadConfirmShow.value = false
    await settle()
    expect(getMock.mock.calls.length).toBe(callsBefore)
    expect(t.form.value.bindPort).toBe(9999) // 编辑未被丢弃
  })

  it('loadedSnapshot 在每次成功加载后刷新 → 重载后 isDirty 归零', async () => {
    getMock.mockReset()
    getMock.mockResolvedValue({ ...HAPPY_CFG })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.loadedSnapshot.value?.bindPort).toBe(7100)
    t.form.value.authToken = 'changed'
    await nextTick()
    expect(t.isDirty()).toBe(true)
    await t.loadConfig()
    await settle()
    expect(t.isDirty()).toBe(false)
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

  // T-058 (B)：反向证伪 —— dirty 表单点"重新加载"绝不静默丢弃（必须经确认）
  it('dirty 时 handleReloadClick 不得静默重载丢弃编辑（只置确认标志，不调 apiGetServer）', async () => {
    getMock.mockReset()
    getMock.mockResolvedValue({ ...HAPPY_CFG })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    // 改多个字段
    t.form.value.bindPort = 8001
    t.form.value.authToken = 'half-typed-secret'
    await nextTick()
    expect(t.isDirty()).toBe(true)
    const callsBefore = getMock.mock.calls.length
    t.handleReloadClick()
    await settle()
    // 反向证伪：若实现退回"直接 loadConfig" → 此断言会 FAIL
    expect(getMock.mock.calls.length).toBe(callsBefore)
    expect(t.form.value.authToken).toBe('half-typed-secret')
    expect(t.reloadConfirmShow.value).toBe(true)
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
