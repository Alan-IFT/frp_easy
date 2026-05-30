// T-057 / binary-missing-onboarding-ux
// Wizard.vue — 完成配置后校验所选角色二进制是否就绪。
//
// 关键模式（insight L14 / L45 + T-056 Dashboard.spec 范式）：
//   - vi.mock('naive-ui') importOriginal + useMessage 单例 spy（断言 success / 不发"正在跳转"）
//   - vue-router useRouter push spy（断言"未自动跳转" / "手动跳转"）
//   - mock api 层（server/frpclient/mode/wizard/system）让 mount 不触网
//   - appStore.fetchReady 真实执行，但 api/system.apiGetReady 被 mock 控制 binMissing
//   - 读句柄用 getExposed<T>，API 失败用 apiError()

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { getExposed } from '../../test-utils/exposed'
import { apiError } from '../../test-utils/apiError'
import { defineComponent, h, nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { NConfigProvider, NMessageProvider } from 'naive-ui'

vi.mock('../../api/server', () => ({ apiPutServer: vi.fn() }))
vi.mock('../../api/frpclient', () => ({ apiPutClient: vi.fn() }))
vi.mock('../../api/mode', () => ({ apiPutMode: vi.fn() }))
vi.mock('../../api/wizard', () => ({
  apiGetWizardStatus: vi.fn().mockResolvedValue({ handled: false, shouldShow: true }),
  apiWizardComplete: vi.fn().mockResolvedValue(undefined),
}))
vi.mock('../../api/system', () => ({
  apiGetReady: vi.fn().mockResolvedValue({ initialized: true, binMissing: [], version: '1.0.0' }),
}))

// useMessage 单例 spy
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
      info: vi.fn(), success: vi.fn(), warning: vi.fn(), error: vi.fn(),
      create: vi.fn(), destroyAll: vi.fn(),
    }),
    useNotification: () => ({
      info: vi.fn(), success: vi.fn(), warning: vi.fn(), error: vi.fn(), destroyAll: vi.fn(),
    }),
    useLoadingBar: () => ({ start: vi.fn(), finish: vi.fn(), error: vi.fn() }),
    useModal: () => ({ create: vi.fn() }),
  }
})

const pushSpy = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushSpy }),
}))

import Wizard from '../Wizard.vue'
import * as serverApi from '../../api/server'
import * as clientApi from '../../api/frpclient'
import * as modeApi from '../../api/mode'
import * as systemApi from '../../api/system'

const getReadyMock = vi.mocked(systemApi.apiGetReady)

interface TestingHandle {
  currentStep: { value: number }
  selectedRole: { value: 'frpc' | 'frps' | 'both' | '' }
  completing: { value: boolean }
  binWarning: { value: string[] }
  configError: { value: string }
  handleNext: () => Promise<void>
  handleSkip: () => Promise<void>
  missingForRole: (role: 'frpc' | 'frps' | 'both' | '') => string[]
  goToDashboard: () => void
  // T-062
  tokenMismatch: { value: boolean }
  goToProxies: () => void
  frpsForm: { value: { bindPort: number; authToken: string } }
  frpcForm: { value: { serverAddr: string; serverPort: number; authToken: string } }
}

function mountPage() {
  const Holder = defineComponent({
    setup() {
      return () =>
        h(NConfigProvider, null, {
          default: () => h(NMessageProvider, null, { default: () => h(Wizard) }),
        })
    },
  })
  return mount(Holder, { attachTo: document.body })
}

function getTesting(wrapper: ReturnType<typeof mountPage>): TestingHandle {
  return getExposed<TestingHandle>(wrapper, Wizard)
}

async function settle(n = 8): Promise<void> {
  for (let i = 0; i < n; i++) await nextTick()
}

// 驱动完成流程：设角色 → 直接调 handleNext（绕过 step1/2 表单交互，逻辑分支一致）。
// step2 完成分支只在表单校验通过后到达；这里通过把 currentStep 推到 2 并填好表单值后调
// handleNext。对 frpc/both 需 serverAddr 必填，故按角色预填。
async function completeAsRole(
  w: ReturnType<typeof mountPage>,
  role: 'frpc' | 'frps' | 'both',
): Promise<TestingHandle> {
  const t = getTesting(w)
  t.selectedRole.value = role
  // 推进到 step2
  t.currentStep.value = 2
  await settle()
  // frpc/both 需要 serverAddr 才能过校验：用 DOM 填入
  if (role === 'frpc' || role === 'both') {
    const addrInput = w.find('input[placeholder="frps 服务器的 IP 或主机名"]')
    if (addrInput.exists()) {
      await addrInput.setValue('1.2.3.4')
    }
  }
  await settle()
  await t.handleNext()
  await settle()
  return t
}

beforeEach(() => {
  setActivePinia(createPinia())
  pushSpy.mockReset()
  Object.values(messageSpies).forEach((s) => s.mockReset())
  vi.mocked(serverApi.apiPutServer).mockReset().mockResolvedValue(undefined as never)
  vi.mocked(clientApi.apiPutClient).mockReset().mockResolvedValue(undefined as never)
  vi.mocked(modeApi.apiPutMode).mockReset().mockResolvedValue({ frpc: true, frps: true } as never)
  getReadyMock.mockReset()
  getReadyMock.mockResolvedValue({ initialized: true, binMissing: [], version: '1.0.0' } as never)
})

afterEach(() => {
  document.body.innerHTML = ''
})

// missingForRole 纯函数语义（单测，不经完成流程）。
describe('Wizard.vue — missingForRole 角色→缺失交集（T-057）', () => {
  it('both + binMissing=[frps] → [frps]（所选之一缺失即缺失）', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    const { useAppStore } = await import('../../stores/app')
    useAppStore().binMissing = ['frps']
    expect(t.missingForRole('both')).toEqual(['frps'])
  })

  it('frpc + binMissing=[frps] → []（与所选无关的缺失不算）', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    const { useAppStore } = await import('../../stores/app')
    useAppStore().binMissing = ['frps']
    expect(t.missingForRole('frpc')).toEqual([])
  })

  it('both + binMissing=[frpc,frps] → [frpc,frps]（全缺失）', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    const { useAppStore } = await import('../../stores/app')
    useAppStore().binMissing = ['frpc', 'frps']
    expect(t.missingForRole('both')).toEqual(['frpc', 'frps'])
  })

  it('binMissing=[] → 任何角色都返回 []', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.missingForRole('both')).toEqual([])
    expect(t.missingForRole('frpc')).toEqual([])
    expect(t.missingForRole('frps')).toEqual([])
  })
})

// AC-4 / AC-6：不缺失分支维持原行为（success toast + 自动跳转 + fetchReady 被调）。
describe('Wizard.vue — 二进制全就绪 → 维持自动跳转（T-057 AC-4）', () => {
  it('both 全就绪：success("正在跳转") + 自动 router.push(/dashboard) + binWarning=[]', async () => {
    getReadyMock.mockResolvedValue({ initialized: true, binMissing: [], version: '1.0.0' } as never)
    const w = mountPage()
    await settle()
    const t = await completeAsRole(w, 'both')
    expect(t.binWarning.value).toEqual([])
    expect(messageSpies.success).toHaveBeenCalledWith('配置已保存，正在跳转...')
    expect(pushSpy).toHaveBeenCalledWith('/dashboard')
    // AC-6：完成流程在校验前刷新了 binMissing（apiGetReady 被调）
    expect(getReadyMock).toHaveBeenCalled()
  })

  it('AC-5：frpc 但缺的是 frps（与所选无关）→ 仍走自动跳转（不误报警告）', async () => {
    getReadyMock.mockResolvedValue({ initialized: true, binMissing: ['frps'], version: '1.0.0' } as never)
    const w = mountPage()
    await settle()
    const t = await completeAsRole(w, 'frpc')
    expect(t.binWarning.value).toEqual([])
    expect(messageSpies.success).toHaveBeenCalledWith('配置已保存，正在跳转...')
    expect(pushSpy).toHaveBeenCalledWith('/dashboard')
  })
})

// AC-3：缺失分支 —— 不自动跳、不发"正在跳转"toast、就地警告、手动按钮。
describe('Wizard.vue — 所选角色二进制缺失 → 不静默跳走（T-057 AC-3）', () => {
  it('both + frps 缺失：binWarning=[frps]、不发 success、未自动 push、出现警告与手动按钮', async () => {
    getReadyMock.mockResolvedValue({ initialized: true, binMissing: ['frps'], version: '1.0.0' } as never)
    const w = mountPage()
    await settle()
    const t = await completeAsRole(w, 'both')

    expect(t.binWarning.value).toEqual(['frps'])
    // 不发"正在跳转"成功提示
    expect(messageSpies.success).not.toHaveBeenCalledWith('配置已保存，正在跳转...')
    // 未自动跳转
    expect(pushSpy).not.toHaveBeenCalledWith('/dashboard')
    // 就地警告可见，列出缺失的 frps
    const txt = w.text()
    expect(txt).toContain('尚未就绪')
    expect(txt).toContain('frps')
    expect(txt).toContain('顶部横幅')
    // 配置仍已保存（PUT 调用发生）
    expect(vi.mocked(serverApi.apiPutServer)).toHaveBeenCalled()
    expect(vi.mocked(clientApi.apiPutClient)).toHaveBeenCalled()
  })

  it('点「进入仪表盘」按钮才跳转（手动而非自动）', async () => {
    getReadyMock.mockResolvedValue({ initialized: true, binMissing: ['frpc', 'frps'], version: '1.0.0' } as never)
    const w = mountPage()
    await settle()
    await completeAsRole(w, 'both')
    expect(pushSpy).not.toHaveBeenCalledWith('/dashboard')
    const btn = w.findAll('button').find((b) => b.text().includes('进入仪表盘'))
    expect(btn).toBeTruthy()
    await btn!.trigger('click')
    expect(pushSpy).toHaveBeenCalledWith('/dashboard')
  })

  it('frps 角色 + frps 缺失：binWarning=[frps]、不自动跳', async () => {
    getReadyMock.mockResolvedValue({ initialized: true, binMissing: ['frps'], version: '1.0.0' } as never)
    const w = mountPage()
    await settle()
    const t = await completeAsRole(w, 'frps')
    expect(t.binWarning.value).toEqual(['frps'])
    expect(pushSpy).not.toHaveBeenCalledWith('/dashboard')
  })
})

// BC-6：保存失败优先于二进制校验（不进缺失分支）。
describe('Wizard.vue — 保存失败不进二进制校验（T-057 BC-6）', () => {
  it('apiPutServer reject → configError 被设、不进 step3 完成态、不发跳转 toast', async () => {
    vi.mocked(serverApi.apiPutServer).mockRejectedValue(apiError('后端 500：保存失败'))
    const w = mountPage()
    await settle()
    const t = await completeAsRole(w, 'frps')
    expect(t.configError.value).toContain('保存失败')
    expect(messageSpies.success).not.toHaveBeenCalledWith('配置已保存，正在跳转...')
    expect(t.binWarning.value).toEqual([])
  })
})

// T-058 (C)：frpc 客户端配置标题死分支清理（原 v-if='both' / v-else 两分支文案相同）
describe('Wizard.vue — frpc 客户端配置标题（C，死分支清理后无回归）', () => {
  function countOccurrences(haystack: string, needle: string): number {
    let n = 0
    let i = haystack.indexOf(needle)
    while (i !== -1) {
      n++
      i = haystack.indexOf(needle, i + needle.length)
    }
    return n
  }

  it("selectedRole='frpc' → step2 恰显示一次「frpc 客户端配置」标题", async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.selectedRole.value = 'frpc'
    t.currentStep.value = 2
    await settle()
    expect(countOccurrences(w.text(), 'frpc 客户端配置')).toBe(1)
    // frpc-only 时不应渲染 frps 段标题
    expect(w.text()).not.toContain('frps 服务端配置')
  })

  it("selectedRole='both' → step2 恰显示一次「frpc 客户端配置」标题（且含 frps 服务端配置）", async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.selectedRole.value = 'both'
    t.currentStep.value = 2
    await settle()
    expect(countOccurrences(w.text(), 'frpc 客户端配置')).toBe(1)
    expect(w.text()).toContain('frps 服务端配置')
  })

  it("selectedRole='frps' → step2 不渲染「frpc 客户端配置」标题（外层 v-if 仍正确隐藏）", async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.selectedRole.value = 'frps'
    t.currentStep.value = 2
    await settle()
    expect(countOccurrences(w.text(), 'frpc 客户端配置')).toBe(0)
    expect(w.text()).toContain('frps 服务端配置')
  })
})

// T-062 IS-7：both 模式两端 token 一致性预警（非阻断）
describe('Wizard.vue — both token 一致性预警（T-062 IS-7）', () => {
  async function setupStep2Both(
    w: ReturnType<typeof mountPage>,
    frpsToken: string,
    frpcToken: string,
  ): Promise<TestingHandle> {
    const t = getTesting(w)
    t.selectedRole.value = 'both'
    t.currentStep.value = 2
    await settle()
    t.frpsForm.value.authToken = frpsToken
    t.frpcForm.value.authToken = frpcToken
    await settle()
    return t
  }

  it('AC-9：两端 token 均非空且不等 → tokenMismatch=true + step2 出现不一致 warning', async () => {
    const w = mountPage()
    await settle()
    const t = await setupStep2Both(w, 'aaa', 'bbb')
    expect(t.tokenMismatch.value).toBe(true)
    expect(w.text()).toContain('两端 token 不一致')
  })

  it('AC-10：两端 token 相等 → 不报不一致警告', async () => {
    const w = mountPage()
    await settle()
    const t = await setupStep2Both(w, 'same-token', 'same-token')
    expect(t.tokenMismatch.value).toBe(false)
    expect(w.text()).not.toContain('两端 token 不一致')
  })

  it('AC-10：仅一端有 token（另一端空）→ 不报警告（BC-3）', async () => {
    const w = mountPage()
    await settle()
    const t = await setupStep2Both(w, 'only-frps', '')
    expect(t.tokenMismatch.value).toBe(false)
    expect(w.text()).not.toContain('两端 token 不一致')
  })

  it('AC-10：角色非 both（frpc）→ 即使两 token 不等也不报（tokenMismatch 仅 both 生效）', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.selectedRole.value = 'frpc'
    t.currentStep.value = 2
    await settle()
    t.frpsForm.value.authToken = 'aaa'
    t.frpcForm.value.authToken = 'bbb'
    await settle()
    expect(t.tokenMismatch.value).toBe(false)
  })

  it('AC-9：token 不一致非阻断 —— 完成配置仍能推进到 step3、不进 configError', async () => {
    getReadyMock.mockResolvedValue({ initialized: true, binMissing: [], version: '1.0.0' } as never)
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.selectedRole.value = 'both'
    t.currentStep.value = 2
    await settle()
    // 填 serverAddr 过 frpc 校验 + 两端不等 token
    const addrInput = w.find('input[placeholder="frps 服务器的 IP 或主机名"]')
    if (addrInput.exists()) await addrInput.setValue('1.2.3.4')
    t.frpsForm.value.authToken = 'aaa'
    t.frpcForm.value.authToken = 'bbb'
    await settle()
    expect(t.tokenMismatch.value).toBe(true)
    await t.handleNext()
    await settle()
    // 非阻断：推进到完成态、configError 仍空
    expect(t.currentStep.value).toBe(3)
    expect(t.configError.value).toBe('')
  })
})

// T-062 IS-1：完成页加规则引导（仅 frpc/both + 就绪分支）
describe('Wizard.vue — 完成页加规则引导（T-062 IS-1）', () => {
  it('AC-1：both 全就绪完成 → step3 出现「前往代理规则」引导 + 点击 push(/proxies)', async () => {
    getReadyMock.mockResolvedValue({ initialized: true, binMissing: [], version: '1.0.0' } as never)
    const w = mountPage()
    await settle()
    const t = await completeAsRole(w, 'both')
    expect(t.binWarning.value).toEqual([])
    // 引导按钮存在（C-1：验收点是存在+可点击，不要求阻止自动跳转）
    const btn = w.findAll('button').find((b) => b.text().includes('前往「代理规则」'))
    expect(btn).toBeTruthy()
    // 点击触发 push('/proxies')（与既有自动 push('/dashboard') 并存）
    await btn!.trigger('click')
    expect(pushSpy).toHaveBeenCalledWith('/proxies')
  })

  it('AC-1：frpc 全就绪完成 → 也出现加规则引导', async () => {
    getReadyMock.mockResolvedValue({ initialized: true, binMissing: [], version: '1.0.0' } as never)
    const w = mountPage()
    await settle()
    await completeAsRole(w, 'frpc')
    const btn = w.findAll('button').find((b) => b.text().includes('前往「代理规则」'))
    expect(btn).toBeTruthy()
    await btn!.trigger('click')
    expect(pushSpy).toHaveBeenCalledWith('/proxies')
  })

  it('AC-2：frps 角色完成 → 不出现加规则引导（纯服务端无转发规则概念，BC-1）', async () => {
    getReadyMock.mockResolvedValue({ initialized: true, binMissing: [], version: '1.0.0' } as never)
    const w = mountPage()
    await settle()
    await completeAsRole(w, 'frps')
    const btn = w.findAll('button').find((b) => b.text().includes('前往「代理规则」'))
    expect(btn).toBeFalsy()
    expect(w.text()).not.toContain('前往「代理规则」')
  })

  it('AC-3：both 但 frps 缺失（binWarning>0）→ 缺失警告在、加规则引导不在（BC-2）', async () => {
    getReadyMock.mockResolvedValue({ initialized: true, binMissing: ['frps'], version: '1.0.0' } as never)
    const w = mountPage()
    await settle()
    const t = await completeAsRole(w, 'both')
    expect(t.binWarning.value).toEqual(['frps'])
    // T-057 缺失警告仍在
    expect(w.text()).toContain('尚未就绪')
    // 加规则引导不在缺失分支出现
    expect(w.text()).not.toContain('前往「代理规则」')
  })

  it('AC-11：全就绪自动跳转既有行为不变（T-057 护栏）—— success toast + push(/dashboard)', async () => {
    getReadyMock.mockResolvedValue({ initialized: true, binMissing: [], version: '1.0.0' } as never)
    const w = mountPage()
    await settle()
    await completeAsRole(w, 'both')
    // 既有 T-057 行为：新引导是附加，不改这两条
    expect(messageSpies.success).toHaveBeenCalledWith('配置已保存，正在跳转...')
    expect(pushSpy).toHaveBeenCalledWith('/dashboard')
  })
})

// ## Adversarial tests
describe('Wizard.vue — Adversarial（T-057）', () => {
  // 核心反向证伪：所选 both 但 frps 缺失 → 警告出现、未静默跳。
  // 假设：若完成分支无视 binMissing 直接 success+push（旧行为），此处 pushSpy 会被以
  // '/dashboard' 调用且 binWarning 为空 → 断言失败。
  it('ADV：both 选中但 frps 缺失 → 警告出现且未静默自动跳（绝不悄悄跳到仪表盘）', async () => {
    getReadyMock.mockResolvedValue({ initialized: true, binMissing: ['frps'], version: '1.0.0' } as never)
    const w = mountPage()
    await settle()
    const t = await completeAsRole(w, 'both')
    // 必须有可见缺失结论
    expect(t.binWarning.value).toEqual(['frps'])
    expect(w.text()).toContain('尚未就绪')
    // 绝不自动跳：完成那一刻 push 未被以 /dashboard 调用
    expect(pushSpy).not.toHaveBeenCalledWith('/dashboard')
    // 也绝不发"正在跳转"误导文案
    expect(messageSpies.success).not.toHaveBeenCalledWith('配置已保存，正在跳转...')
  })

  // 反向：fetchReady 失败（apiGetReady reject）时 fetchReady 吞错，binMissing 维持原值（空）→
  // 不崩、不阻断；按已知值（空）走自动跳转，不会因为 fetch 失败误报缺失警告。
  it('ADV：fetchReady 失败被吞 → 不崩、按已知 binMissing(空) 走自动跳转，不误报警告', async () => {
    getReadyMock.mockRejectedValue(apiError('ready 探测失败'))
    const w = mountPage()
    await settle()
    const t = await completeAsRole(w, 'both')
    // binMissing 维持初始空 → binWarning 空 → 自动跳转
    expect(t.binWarning.value).toEqual([])
    expect(pushSpy).toHaveBeenCalledWith('/dashboard')
    expect(t.configError.value).toBe('')
  })

  // 反向：完成那一刻定格快照 —— binWarning 是 ref 快照，完成后即使 store.binMissing
  // 再变化（如后台下载完成）也不应让 step3 已展示的警告凭空消失（语义：定格）。
  it('ADV：binWarning 定格快照，完成后 store.binMissing 清空也不抹掉已展示警告', async () => {
    getReadyMock.mockResolvedValue({ initialized: true, binMissing: ['frpc', 'frps'], version: '1.0.0' } as never)
    const w = mountPage()
    await settle()
    const t = await completeAsRole(w, 'both')
    expect(t.binWarning.value).toEqual(['frpc', 'frps'])
    // 模拟后续 store 变化
    const { useAppStore } = await import('../../stores/app')
    useAppStore().binMissing = []
    await settle()
    // 警告快照不变（ref 不随 store 漂移）
    expect(t.binWarning.value).toEqual(['frpc', 'frps'])
    expect(w.text()).toContain('尚未就绪')
  })

  // T-062 反向证伪：token 不一致预警必须真的"非阻断"。
  // 假设：若 tokenMismatch 被误接进 handleNext 的校验（阻断），则 currentStep 会停在 2、
  // 不会到 3 → 断言失败。此用例锁死"预警不阻断推进"。
  it('ADV（T-062）：token 不一致绝不阻断完成 —— 不一致状态下仍推进到 step3', async () => {
    getReadyMock.mockResolvedValue({ initialized: true, binMissing: [], version: '1.0.0' } as never)
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.selectedRole.value = 'both'
    t.currentStep.value = 2
    await settle()
    const addrInput = w.find('input[placeholder="frps 服务器的 IP 或主机名"]')
    if (addrInput.exists()) await addrInput.setValue('1.2.3.4')
    t.frpsForm.value.authToken = 'token-A'
    t.frpcForm.value.authToken = 'token-B-different'
    await settle()
    expect(t.tokenMismatch.value).toBe(true)
    await t.handleNext()
    await settle()
    expect(t.currentStep.value).toBe(3)
    expect(t.configError.value).toBe('')
    // 配置照常 PUT（不一致不阻止保存）
    expect(vi.mocked(serverApi.apiPutServer)).toHaveBeenCalled()
    expect(vi.mocked(clientApi.apiPutClient)).toHaveBeenCalled()
  })

  // T-062 反向证伪：纯空白 token 不应被当作"非空 token"误报不一致（BC-4 trim 防御）。
  // 假设：若漏 trim，则 '   '（空白）与 'real' 会被判不等且都"非空"→ 误报警告 → 断言失败。
  it('ADV（T-062）：一端纯空白 token 经 trim 视为空 → 不误报不一致', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.selectedRole.value = 'both'
    t.currentStep.value = 2
    await settle()
    t.frpsForm.value.authToken = '   '
    t.frpcForm.value.authToken = 'real-token'
    await settle()
    expect(t.tokenMismatch.value).toBe(false)
    expect(w.text()).not.toContain('两端 token 不一致')
  })

  // T-062 QA 独立反向证伪（从 AC-3 重写，不复用 dev 测试假设）：
  // IS-1 加规则引导与 T-057 缺失分支必须互斥——缺失态时引导按钮元素必须不存在，
  // 且缺失分支的「进入仪表盘」按钮必须存在（证明确在 v-else 分支而非误判分支）。
  // 假设：若 IS-1 按钮被错误放在 step3 外层（非 binWarning===0 template 内），
  // 则缺失态也会出现该按钮 → 第一条断言失败。
  it('ADV（T-062 QA）：缺失态 IS-1 引导按钮元素不存在，且「进入仪表盘」按钮存在（分支互斥）', async () => {
    getReadyMock.mockResolvedValue({ initialized: true, binMissing: ['frpc', 'frps'], version: '1.0.0' } as never)
    const w = mountPage()
    await settle()
    const t = await completeAsRole(w, 'both')
    expect(t.binWarning.value).toEqual(['frpc', 'frps'])
    // 引导按钮元素绝不在缺失分支渲染
    const guideBtn = w.findAll('button').find((b) => b.text().includes('前往「代理规则」'))
    expect(guideBtn).toBeUndefined()
    // 缺失分支独有的「进入仪表盘」按钮必须存在（确认确实在 v-else）
    const dashBtn = w.findAll('button').find((b) => b.text().includes('进入仪表盘'))
    expect(dashBtn).toBeTruthy()
  })

  // T-062 QA 独立反向证伪（从 AC-1 重写）：就绪态点引导按钮恰触发一次 push('/proxies')，
  // 且 push('/proxies') 与既有自动 push('/dashboard') 是两次独立调用（不串扰、不互相抑制）。
  it('ADV（T-062 QA）：就绪态 push(/dashboard) 与点击 push(/proxies) 两次独立共存', async () => {
    getReadyMock.mockResolvedValue({ initialized: true, binMissing: [], version: '1.0.0' } as never)
    const w = mountPage()
    await settle()
    await completeAsRole(w, 'both')
    // 完成那一刻已自动 push('/dashboard')（T-057 既有）
    expect(pushSpy).toHaveBeenCalledWith('/dashboard')
    const before = pushSpy.mock.calls.filter((c) => c[0] === '/proxies').length
    expect(before).toBe(0)
    const guideBtn = w.findAll('button').find((b) => b.text().includes('前往「代理规则」'))
    expect(guideBtn).toBeTruthy()
    await guideBtn!.trigger('click')
    const afterProxies = pushSpy.mock.calls.filter((c) => c[0] === '/proxies').length
    expect(afterProxies).toBe(1)
    // /dashboard 调用次数未被点击 /proxies 影响（独立）
    expect(pushSpy).toHaveBeenCalledWith('/dashboard')
  })
})
