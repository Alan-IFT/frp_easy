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
import type { AllowPortRange } from '../../types'

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

// T-062：vue-router push spy（IS-5 查看运行态跳转断言）。Server.vue 原不 import useRouter；
// 此模块级 mock 只提供 push spy，不影响既有用例（既有用例不依赖 router）。
const pushSpy = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushSpy }),
}))

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
  // T-060
  loadedAllowPortsSnapshot: { value: string | null }
  normalizeAllowPorts: (ranges: AllowPortRange[]) => string
  allowPortsEditorRef: {
    value: {
      getAllowPortsInput: () => AllowPortRange[]
      hasValidationError: () => boolean
    } | null
  }
  // T-062 IS-5
  goToMonitor: () => void
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
  pushSpy.mockReset()
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

// ── T-067 responsive-layout · 表单 max-width 化（FR-6 / AC-4）──
// 固定像素宽输入控件改 width:100% + max-width:Npx，窄屏自适应不溢出、宽屏维持上限观感。
// 断言可观察量：渲染后控件元素的 inline style 含 max-width（不再裸 width:200px），
// 且 width 为 100%（不查 naive-ui 组件名，查渲染后元素 attributes，insight L45）。
describe('Server.vue — 表单 max-width 化（T-067 FR-6 / AC-4）', () => {
  it('loaded 态：输入控件 inline style 含 max-width 且 width:100%（窄屏不溢出，宽屏维持上限）', async () => {
    const w = mountPage()
    await settle()
    // n-input-number / n-input 把 style 透传到根元素；收集带 max-width 的控件
    const styled = [
      ...w.findAll('.n-input-number'),
      ...w.findAll('.n-input'),
    ]
      .map((el) => el.attributes('style') ?? '')
      .filter((s) => s.includes('max-width'))
    // 至少 bindPort(200) / authToken(360) / dashboardPort(200) / dashboardUser(240) / dashboardPass(240)
    // 中的若干在 loaded(dashboardEnabled=true) 态渲染并带 max-width
    expect(styled.length).toBeGreaterThan(0)
    for (const s of styled) {
      // 反向证伪：每个带 max-width 的控件同时 width:100%（不再裸固定 px 宽）
      expect(s).toMatch(/width:\s*100%/)
      expect(s).toMatch(/max-width:\s*\d+px/)
    }
  })

  it('AC-4 反向证伪：不再存在裸固定像素宽（width:Npx 不配 max-width）的目标控件', async () => {
    const w = mountPage()
    await settle()
    const styledControls = [
      ...w.findAll('.n-input-number'),
      ...w.findAll('.n-input'),
    ].map((el) => el.attributes('style') ?? '')
    // 任何含像素 width 的目标控件都必须同时含 max-width（即不再有裸 width:200px 旧写法）
    for (const s of styledControls) {
      if (/width:\s*\d+px/.test(s)) {
        // 出现裸像素 width 即回归（应为 width:100% + max-width:Npx）
        expect(s).toContain('max-width')
      }
    }
  })
})

// T-062 IS-5：查看运行态链接（loaded 态 + 双向连通）
describe('Server.vue — 查看运行态链接（T-062 IS-5）', () => {
  it('AC-7：loaded 态 → 出现"查看运行态"链接', async () => {
    const w = mountPage()
    await settle()
    expect(w.text()).toContain('查看运行态')
  })

  it('AC-7 / AC-12：点击"查看运行态" → router.push(/server/monitor)', async () => {
    const w = mountPage()
    await settle()
    const btn = w.findAll('button').find((b) => b.text().includes('查看运行态'))
    expect(btn).toBeTruthy()
    await btn!.trigger('click')
    expect(pushSpy).toHaveBeenCalledWith('/server/monitor')
  })

  it('goToMonitor handler 直接调用 → push(/server/monitor)', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.goToMonitor()
    expect(pushSpy).toHaveBeenCalledWith('/server/monitor')
  })

  it('AC-7 / BC-8：加载失败态（loadError）不出现"查看运行态"链接', async () => {
    getMock.mockReset()
    getMock.mockRejectedValue(apiError('后端 500：加载配置失败'))
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.loadError.value).not.toBeNull()
    expect(w.text()).toContain('加载服务端配置失败')
    // 失败态 card（v-if=loadError）不含 loaded 态 #action 区的"查看运行态"按钮
    expect(w.text()).not.toContain('查看运行态')
  })

  it('AC-7 / BC-8：加载中态（loading）不出现"查看运行态"链接', async () => {
    getMock.mockReset()
    getMock.mockReturnValue(new Promise<typeof HAPPY_CFG>(() => {}))
    const w = mountPage()
    await nextTick()
    const t = getTesting(w)
    expect(t.loading.value).toBe(true)
    expect(w.text()).not.toContain('查看运行态')
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

// T-060：dirty 检测纳入 AllowPortsEditor 端口策略
// 消除"只改端口策略 → 点重新加载 → 无确认 → 静默丢弃端口编辑"的数据丢失路径。
// 驱动方式：用真 AllowPortsEditor（本 spec 未 mock 它），通过 DOM 按钮文本（"添加单端口"/"删除"）
// 改变编辑器 rows，或直接通过 allowPortsEditorRef 句柄读 getAllowPortsInput。
// 断言不按 naive-ui 组件名查询（insight L45 / T-057 教训）。
describe('Server.vue — normalizeAllowPorts 稳定性（T-060 AC-4）', () => {
  it('空列表 → 空串；single/range 各产生确定且互不相同的字符串', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    const n = t.normalizeAllowPorts
    expect(n([])).toBe('')
    expect(n([{ single: 8080 }])).toBe('s:8080')
    expect(n([{ start: 1000, end: 2000 }])).toBe('r:1000-2000')
    // single vs range 同端口形态敏感（互不相同）
    expect(n([{ single: 8080 }])).not.toBe(n([{ start: 8080, end: 8080 }]))
    expect(n([{ start: 8080, end: 8080 }])).toBe('r:8080-8080')
  })

  it('顺序敏感：[s:1,s:2] 与 [s:2,s:1] 规范化不同', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    const n = t.normalizeAllowPorts
    expect(n([{ single: 1 }, { single: 2 }])).toBe('s:1|s:2')
    expect(n([{ single: 2 }, { single: 1 }])).toBe('s:2|s:1')
    expect(n([{ single: 1 }, { single: 2 }])).not.toBe(n([{ single: 2 }, { single: 1 }]))
  })

  it('混合 single + range 按用户顺序 join，未填值退化为 0', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    const n = t.normalizeAllowPorts
    expect(n([{ single: 80 }, { start: 1000, end: 2000 }])).toBe('s:80|r:1000-2000')
    // 未填行（start/end 缺失）→ r:0-0（与"加了空行"语义对应）
    expect(n([{ start: undefined, end: undefined }])).toBe('r:0-0')
  })
})

describe('Server.vue — 端口策略纳入 dirty（T-060）', () => {
  const CFG_WITH_PORTS = {
    ...HAPPY_CFG,
    allowPorts: [{ single: 8080 }, { start: 1000, end: 2000 }] as AllowPortRange[],
  }

  it('AC-5 round-trip：加载带端口策略后未改动 → isDirty()=false（不误判脏）', async () => {
    getMock.mockReset()
    getMock.mockResolvedValue({ ...CFG_WITH_PORTS })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    // 快照 = 加载值规范化；编辑器 seed→output 对合法值是 identity
    expect(t.loadedAllowPortsSnapshot.value).toBe('s:8080|r:1000-2000')
    // 编辑器当前输出规范化应与快照相等 → 非脏（round-trip identity 锁死）
    expect(t.normalizeAllowPorts(t.allowPortsEditorRef.value!.getAllowPortsInput())).toBe(
      's:8080|r:1000-2000',
    )
    expect(t.isDirty()).toBe(false)
    expect(t.reloadConfirmShow.value).toBe(false)
  })

  it('AC-1 只改端口策略（DOM 添加单端口行，标量不动）→ isDirty()=true 且 handleReloadClick 弹确认、不调 apiGetServer', async () => {
    getMock.mockReset()
    getMock.mockResolvedValue({ ...CFG_WITH_PORTS })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.isDirty()).toBe(false)
    // DOM 驱动：点编辑器"添加单端口"按钮（按文本，非组件名）→ 真实改 rows
    const addBtn = w.findAll('button').find((b) => b.text().includes('添加单端口'))
    expect(addBtn).toBeTruthy()
    await addBtn!.trigger('click')
    await settle()
    // 标量未动；仅端口策略多了一行（未填 → s:0）→ 脏
    expect(t.isDirty()).toBe(true)
    const callsBefore = getMock.mock.calls.length
    t.handleReloadClick()
    await nextTick()
    // 弹确认，且确认前绝不重载（防"点了就丢"）
    expect(t.reloadConfirmShow.value).toBe(true)
    expect(getMock.mock.calls.length).toBe(callsBefore)
  })

  it('AC-2 既不改标量也不改端口策略 → handleReloadClick 不弹确认、直接重载', async () => {
    getMock.mockReset()
    getMock.mockResolvedValue({ ...CFG_WITH_PORTS })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.isDirty()).toBe(false)
    const callsBefore = getMock.mock.calls.length
    t.handleReloadClick()
    await settle()
    expect(t.reloadConfirmShow.value).toBe(false)
    expect(getMock.mock.calls.length).toBe(callsBefore + 1)
  })

  // AC-6：改端口策略 + 确认 → 触发重载、快照刷新回真实值基准。
  // 注意 D-1（QA 发现）：AllowPortsEditor 单向数据流不 watch props.initial（setup 只读一次），
  // 故 confirmReload→loadConfig 重写 initialAllowPorts 后编辑器 rows 不复位——用户加的行仍在。
  // 这是端口策略（独立组件、不复位）与标量字段（form 由 loadConfig 直接重赋、复位）的本质差异。
  // 因此本用例断言可观测的真实行为：apiGet 再调一次 + loadedAllowPortsSnapshot 刷新回真实值，
  // 而非 isDirty 归零（编辑器不复位 → 端口策略侧仍判脏，是已知且可接受的范式约束）。
  it('AC-6 改端口策略 + 确认 → 触发重载（apiGet +1）且快照刷新回真实值基准', async () => {
    getMock.mockReset()
    getMock.mockResolvedValue({ ...CFG_WITH_PORTS })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    // 加一行使脏
    const addBtn = w.findAll('button').find((b) => b.text().includes('添加单端口'))
    await addBtn!.trigger('click')
    await settle()
    expect(t.isDirty()).toBe(true)
    t.handleReloadClick()
    await nextTick()
    expect(t.reloadConfirmShow.value).toBe(true)
    const callsBefore = getMock.mock.calls.length
    t.confirmReload()
    await settle()
    // 重载：apiGetServer 再调一次 + 端口策略快照刷新回真实值基准
    expect(getMock.mock.calls.length).toBe(callsBefore + 1)
    expect(t.loadedAllowPortsSnapshot.value).toBe('s:8080|r:1000-2000')
  })

  // AC-6 配套：标量侧的 dirty + 确认 → 重载后标量复位 → isDirty 归零（无回归，与 T-058 一致）。
  // 标量字段经 loadConfig 直接重赋 form.value，故确认重载后标量侧确实复位、isDirty 归零；
  // 此用例与上面端口策略侧形成对照，锁死"标量复位 vs 端口策略不复位"的差异语义。
  it('AC-6 配套：改标量字段 + 确认 → 重载后标量复位、isDirty 归零（无回归）', async () => {
    getMock.mockReset()
    getMock.mockResolvedValue({ ...CFG_WITH_PORTS })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.form.value.bindPort = 9999 // 仅改标量
    await nextTick()
    expect(t.isDirty()).toBe(true)
    t.handleReloadClick()
    await nextTick()
    expect(t.reloadConfirmShow.value).toBe(true)
    const callsBefore = getMock.mock.calls.length
    t.confirmReload()
    await settle()
    expect(getMock.mock.calls.length).toBe(callsBefore + 1)
    expect(t.form.value.bindPort).toBe(HAPPY_CFG.bindPort) // 标量复位回真实值
    expect(t.isDirty()).toBe(false) // 标量复位 + 端口策略未动 → 归零
  })

  it('改标量字段仍弹确认（无回归）—— 与端口策略纳入互不干扰', async () => {
    getMock.mockReset()
    getMock.mockResolvedValue({ ...CFG_WITH_PORTS })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.form.value.bindPort = 9999 // 仅改标量，端口策略不动
    await nextTick()
    expect(t.isDirty()).toBe(true)
    const callsBefore = getMock.mock.calls.length
    t.handleReloadClick()
    await nextTick()
    expect(t.reloadConfirmShow.value).toBe(true)
    expect(getMock.mock.calls.length).toBe(callsBefore)
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

  // T-060：反向证伪 —— 只删一行端口（标量原封不动）→ dirty 检测必须捕获 → 确认必须出现。
  // 若 isDirty 漏掉 allowPorts 比较（退回 T-058 已知局限），此断言会 FAIL（弹不出确认、静默丢弃）。
  it('只删一行端口策略（标量不动）→ isDirty 捕获 → handleReloadClick 弹确认、不静默重载丢弃', async () => {
    getMock.mockReset()
    getMock.mockResolvedValue({
      ...HAPPY_CFG,
      allowPorts: [{ single: 8080 }, { start: 1000, end: 2000 }] as AllowPortRange[],
    })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    // 加载后未改动 → 非脏（前提）
    expect(t.isDirty()).toBe(false)
    expect(t.loadedAllowPortsSnapshot.value).toBe('s:8080|r:1000-2000')
    // DOM 驱动：点第一个"删除"按钮删掉一行端口（按文本，非组件名查询）
    const delBtn = w.findAll('button').find((b) => b.text().includes('删除'))
    expect(delBtn).toBeTruthy()
    await delBtn!.trigger('click')
    await settle()
    // 端口策略少了一行（标量原封不动）→ 必须判脏
    expect(t.form.value.bindPort).toBe(HAPPY_CFG.bindPort) // 标量确实未动
    expect(t.isDirty()).toBe(true)
    const callsBefore = getMock.mock.calls.length
    t.handleReloadClick()
    await nextTick()
    // 反向证伪：若漏掉 allowPorts 比较 → 会走"直接 loadConfig"静默丢弃 → 此两断言 FAIL
    expect(t.reloadConfirmShow.value).toBe(true)
    expect(getMock.mock.calls.length).toBe(callsBefore)
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
