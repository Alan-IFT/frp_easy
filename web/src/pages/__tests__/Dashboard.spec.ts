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

// T-048 D4：vue-router useRouter 桩 —— "查看完整日志"改 router.push（不再 href 整页刷新）。
const pushSpy = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushSpy }),
}))

import Dashboard from '../Dashboard.vue'
import * as modeApi from '../../api/mode'
import * as procApi from '../../api/proc'

const getModeMock = vi.mocked(modeApi.apiGetMode)

interface TestingHandle {
  modeState: { frpc: boolean; frps: boolean }
  modeLoading: { frpc: boolean; frps: boolean }
  modeFetchFailed: { value: boolean }
  fetchMode: () => Promise<void>
  retryFetchMode: () => void
  handleModeToggle: (kind: 'frpc' | 'frps', enabled: boolean) => Promise<void>
  // T-048 E1 / D4
  labelOf: (kind: string) => string
  actionResultMsg: (kind: string, state: string, fallbackVerb: string) => string
  handleStart: (kind: string) => Promise<void>
  handleStop: (kind: string) => Promise<void>
  handleRestart: (kind: string) => Promise<void>
  router: { push: (p: string) => void }
  // T-056：停止/重启二次确认状态机
  pendingAction: { value: { kind: 'frpc' | 'frps'; type: 'stop' | 'restart' } | null }
  showConfirm: { value: boolean }
  confirmTitle: { value: string }
  confirmContent: { value: string }
  requestStop: (kind: 'frpc' | 'frps') => void
  requestRestart: (kind: 'frpc' | 'frps') => void
  confirmPending: () => void
  cancelPending: () => void
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
  pushSpy.mockReset()
  Object.values(messageSpies).forEach((s) => s.mockReset())
  getModeMock.mockResolvedValue({ frpc: true, frps: false })
  // T-048：重置 proc 桩并恢复默认 stopped 状态，避免上一用例的 mockResolvedValue 跨用例泄漏
  vi.mocked(procApi.apiGetProcStatus).mockReset()
  vi.mocked(procApi.apiStartProc).mockReset()
  vi.mocked(procApi.apiStopProc).mockReset()
  vi.mocked(procApi.apiRestartProc).mockReset()
  vi.mocked(procApi.apiGetProcStatus).mockResolvedValue({
    frpc: { kind: 'frpc', state: 'stopped', pid: 0, lastErr: '', changedAt: '' },
    frps: { kind: 'frps', state: 'stopped', pid: 0, lastErr: '', changedAt: '' },
  })
  // T-056 C-2：给 stop/restart/start 桩默认 resolved，避免确认后调用走 reject 错误分支干扰断言
  vi.mocked(procApi.apiStopProc).mockResolvedValue({
    kind: 'frpc', state: 'stopped', pid: 0, lastErr: '', changedAt: '',
  })
  vi.mocked(procApi.apiRestartProc).mockResolvedValue({
    kind: 'frpc', state: 'running', pid: 1, lastErr: '', changedAt: '',
  })
  vi.mocked(procApi.apiStartProc).mockResolvedValue({
    kind: 'frpc', state: 'running', pid: 1, lastErr: '', changedAt: '',
  })
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

// T-048 E1：进程操作文案统一命名（kindLabel）+ 依据真实新状态措辞。
describe('Dashboard.vue — 进程操作文案统一（E1）', () => {
  it('labelOf 把裸 kind 映射为"客户端 frpc / 服务端 frps"', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.labelOf('frpc')).toBe('客户端 frpc')
    expect(t.labelOf('frps')).toBe('服务端 frps')
    // 未知 kind 回落原值（不抛）
    expect(t.labelOf('unknown')).toBe('unknown')
  })

  it('actionResultMsg 用真实 state 给出明确动词（running→已启动 / stopped→已停止）', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.actionResultMsg('frpc', 'running', '已启动')).toBe('客户端 frpc已启动')
    expect(t.actionResultMsg('frps', 'stopped', '已停止')).toBe('服务端 frps已停止')
    expect(t.actionResultMsg('frpc', 'error', '已启动')).toBe('客户端 frpc启动失败')
  })

  it('handleStart 成功 → success 文案含"客户端 frpc"且不含裸 "frpc 启动指令已发送"', async () => {
    vi.mocked(procApi.apiStartProc).mockResolvedValueOnce({
      kind: 'frpc', state: 'running', pid: 123, lastErr: '', changedAt: '2026-05-28T01:00:00Z',
    })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    await t.handleStart('frpc')
    expect(messageSpies.success).toHaveBeenCalledWith('客户端 frpc已启动')
    // 不能再出现旧的裸标识 + 含糊"指令已发送"
    const allMsgs = messageSpies.success.mock.calls.flat().join('|')
    expect(allMsgs).not.toContain('frpc 启动指令已发送')
  })

  it('handleStop 成功 → 文案"服务端 frps已停止"（按 store 返回 state）', async () => {
    vi.mocked(procApi.apiStopProc).mockResolvedValueOnce({
      kind: 'frps', state: 'stopped', pid: 0, lastErr: '', changedAt: '2026-05-28T01:00:00Z',
    })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    await t.handleStop('frps')
    expect(messageSpies.success).toHaveBeenCalledWith('服务端 frps已停止')
  })

  it('handleStart 失败 → error 文案统一命名"客户端 frpc 启动失败"', async () => {
    vi.mocked(procApi.apiStartProc).mockRejectedValueOnce(apiError('端口被占用'))
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    await t.handleStart('frpc')
    // 透传后端精确原因（extractErrorMessage 走结构化分支）
    expect(messageSpies.error).toHaveBeenCalledWith('端口被占用')
  })
})

// T-048 D4：查看完整日志改 router.push（SPA 内跳转，不丢 Pinia 状态）。
describe('Dashboard.vue — 日志链接 router.push（D4）', () => {
  it('frpc error 态点"查看完整日志"→ router.push("/logs/frpc")，模板不再含 href', async () => {
    vi.mocked(procApi.apiGetProcStatus).mockResolvedValue({
      frpc: { kind: 'frpc', state: 'error', pid: 0, lastErr: 'boom', changedAt: '2026-05-28T01:00:00Z' },
      frps: { kind: 'frps', state: 'stopped', pid: 0, lastErr: '', changedAt: '' },
    })
    const w = mountPage()
    await settle()
    // 找到含"查看完整日志"文案的按钮并点击
    const btns = w.findAll('button')
    const logBtn = btns.find((b) => b.text().includes('查看完整日志'))
    expect(logBtn).toBeTruthy()
    await logBtn!.trigger('click')
    expect(pushSpy).toHaveBeenCalledWith('/logs/frpc')
    // D4 关键对齐：不再用 <a href> 触发整页刷新
    expect(w.find('a[href="/logs/frpc"]').exists()).toBe(false)
  })
})

// T-056：停止/重启破坏性操作二次确认（happy path，AC-1~AC-6）。
describe('Dashboard.vue — 停止/重启二次确认（T-056）', () => {
  it('AC-1：点 frpc 停止 → 对话框打开（showConfirm=true）且未调用 stop API', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.requestStop('frpc')
    await settle()
    expect(t.showConfirm.value).toBe(true)
    expect(t.pendingAction.value).toEqual({ kind: 'frpc', type: 'stop' })
    expect(vi.mocked(procApi.apiStopProc)).not.toHaveBeenCalled()
  })

  it('AC-2：确认后 → stopProc(frpc) 恰好 1 次，未触碰 restart/start', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.requestStop('frpc')
    await settle()
    t.confirmPending()
    await settle()
    expect(vi.mocked(procApi.apiStopProc)).toHaveBeenCalledTimes(1)
    expect(vi.mocked(procApi.apiStopProc)).toHaveBeenCalledWith('frpc')
    expect(vi.mocked(procApi.apiRestartProc)).not.toHaveBeenCalled()
    expect(vi.mocked(procApi.apiStartProc)).not.toHaveBeenCalled()
    // 确认后对话框关闭 + pending 清空
    expect(t.showConfirm.value).toBe(false)
    expect(t.pendingAction.value).toBeNull()
  })

  it('AC-3：点 frps 重启后确认 → restartProc(frps) 恰好 1 次', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.requestRestart('frps')
    await settle()
    expect(vi.mocked(procApi.apiRestartProc)).not.toHaveBeenCalled()
    t.confirmPending()
    await settle()
    expect(vi.mocked(procApi.apiRestartProc)).toHaveBeenCalledTimes(1)
    expect(vi.mocked(procApi.apiRestartProc)).toHaveBeenCalledWith('frps')
  })

  it('AC-4：点取消 → 对应 API 零调用，对话框关闭', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.requestStop('frps')
    await settle()
    t.cancelPending()
    await settle()
    expect(vi.mocked(procApi.apiStopProc)).not.toHaveBeenCalled()
    expect(vi.mocked(procApi.apiRestartProc)).not.toHaveBeenCalled()
    expect(t.showConfirm.value).toBe(false)
    expect(t.pendingAction.value).toBeNull()
  })

  it('AC-5：启动不弹确认 → handleStart 直接调 startProc 1 次，无 pendingAction', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    await t.handleStart('frpc')
    await settle()
    expect(vi.mocked(procApi.apiStartProc)).toHaveBeenCalledTimes(1)
    expect(vi.mocked(procApi.apiStartProc)).toHaveBeenCalledWith('frpc')
    // 启动不经状态机
    expect(t.pendingAction.value).toBeNull()
    expect(t.showConfirm.value).toBe(false)
  })

  it('AC-6：动态文案随 pendingAction 切换（停止 frps/frpc 后果不同；重启文案固定）', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)

    t.requestStop('frps')
    await settle()
    expect(t.confirmTitle.value).toBe('停止服务端 frps？')
    expect(t.confirmContent.value).toBe('将立即中断所有正在穿透的远程连接。')

    t.requestStop('frpc')
    await settle()
    expect(t.confirmTitle.value).toBe('停止客户端 frpc？')
    expect(t.confirmContent.value).toBe('将断开本机所有正在转发的连接。')

    t.requestRestart('frps')
    await settle()
    expect(t.confirmTitle.value).toBe('重启服务端 frps？')
    expect(t.confirmContent.value).toBe('将短暂中断当前所有连接后重新建立。')

    t.requestRestart('frpc')
    await settle()
    expect(t.confirmTitle.value).toBe('重启客户端 frpc？')
    expect(t.confirmContent.value).toBe('将短暂中断当前所有连接后重新建立。')
  })

  it('确认对话框组件存在且初始不可见（DOM 不渲染确认文案）', async () => {
    const w = mountPage()
    await settle()
    // 初始无 pending，不应出现任何确认文案
    expect(w.text()).not.toContain('将立即中断所有正在穿透的远程连接')
    expect(w.text()).not.toContain('将断开本机所有正在转发的连接')
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

  // E1 反向：start 返回非预期 state（如 starting）也不能漏措辞 / 不能退回裸 kind
  it('E1：start 返回 state="starting" → 文案"客户端 frpc正在启动"（不裸 kind、不含糊）', async () => {
    vi.mocked(procApi.apiStartProc).mockResolvedValueOnce({
      kind: 'frpc', state: 'starting', pid: 0, lastErr: '', changedAt: '2026-05-28T01:00:00Z',
    })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    await t.handleStart('frpc')
    expect(messageSpies.success).toHaveBeenCalledWith('客户端 frpc正在启动')
  })

  // D4 反向：点击日志链接走 SPA push，绝不通过 <a href> 触发整页刷新（丢 Pinia 状态）
  it('D4：error 态日志链接是 router.push 而非 href 跳转（DOM 无 a[href=/logs/*]）', async () => {
    vi.mocked(procApi.apiGetProcStatus).mockResolvedValue({
      frpc: { kind: 'frpc', state: 'error', pid: 0, lastErr: 'x', changedAt: '2026-05-28T01:00:00Z' },
      frps: { kind: 'frps', state: 'error', pid: 0, lastErr: 'y', changedAt: '2026-05-28T01:00:00Z' },
    })
    const w = mountPage()
    await settle()
    expect(w.find('a[href="/logs/frpc"]').exists()).toBe(false)
    expect(w.find('a[href="/logs/frps"]').exists()).toBe(false)
    const logBtns = w.findAll('button').filter((b) => b.text().includes('查看完整日志'))
    expect(logBtns.length).toBeGreaterThan(0)
    await logBtns[1].trigger('click')
    expect(pushSpy).toHaveBeenCalledWith('/logs/frps')
  })

  // T-056 ADV-A（核心证伪）：点"停止"→点"取消" → stop API 必须零调用。
  // 假设：若 requestStop 直接调了 stop（确认无效），此处 apiStopProc 会被调 → 断言失败。
  it('T-056：点停止→点取消 → apiStopProc 零调用（确认门确实拦住了破坏性操作）', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.requestStop('frps')
    await settle()
    // 取消前已确认未调（确认门在前）
    expect(vi.mocked(procApi.apiStopProc)).not.toHaveBeenCalled()
    t.cancelPending()
    await settle()
    // 取消后仍零调用：误点被彻底拦截
    expect(vi.mocked(procApi.apiStopProc)).toHaveBeenCalledTimes(0)
    expect(vi.mocked(procApi.apiRestartProc)).toHaveBeenCalledTimes(0)
  })

  // T-056 ADV-B（并发待确认串扰证伪）：先 requestStop('frpc') 再 requestRestart('frps')，
  // 确认 → 必须只执行最后记录的操作（restart frps），且 frpc 既不被 stop 也不被 restart。
  // 假设：若状态机记多个 pending / 用错快照，会串到 frpc-stop → 断言失败。
  it('T-056：并发待确认 last-wins → 只执行最后操作(restart frps)，不串到 frpc-stop', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.requestStop('frpc')
    await settle()
    t.requestRestart('frps')
    await settle()
    expect(t.pendingAction.value).toEqual({ kind: 'frps', type: 'restart' })
    t.confirmPending()
    await settle()
    // 只 restart frps 一次
    expect(vi.mocked(procApi.apiRestartProc)).toHaveBeenCalledTimes(1)
    expect(vi.mocked(procApi.apiRestartProc)).toHaveBeenCalledWith('frps')
    // frpc 既没被 stop 也没被 restart（前一个待确认被覆盖，未泄漏执行）
    expect(vi.mocked(procApi.apiStopProc)).not.toHaveBeenCalled()
    expect(vi.mocked(procApi.apiRestartProc)).not.toHaveBeenCalledWith('frpc')
  })

  // T-056 ADV-C（重复确认幂等证伪）：confirmPending 后 pendingAction 已清空，
  // 再 confirmPending 一次不得二次触发 API（防双击连发两次中断指令）。
  // 假设：若 confirmPending 未清 pendingAction / 未判 null，会二次调 stop → 断言失败。
  it('T-056：确认后再点确认 → 不二次触发 API（幂等）', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.requestStop('frpc')
    await settle()
    t.confirmPending()
    await settle()
    expect(vi.mocked(procApi.apiStopProc)).toHaveBeenCalledTimes(1)
    // 二次确认（pendingAction 已 null）
    t.confirmPending()
    await settle()
    expect(vi.mocked(procApi.apiStopProc)).toHaveBeenCalledTimes(1)
  })

  // T-056 ADV-D（启动绝不被加确认证伪）：点"启动"按钮（DOM trigger）→ startProc 立即调，
  // 且不出现确认文案。假设：若误把 handleStart 也改向 requestX，启动会卡在确认 → 断言失败。
  it('T-056：DOM 点"启动"按钮 → startProc 立即调用、无确认文案（启动非破坏性）', async () => {
    const w = mountPage()
    await settle()
    const startBtns = w.findAll('button').filter((b) => b.text().trim() === '启动')
    expect(startBtns.length).toBeGreaterThanOrEqual(1)
    // frpc 卡片"启动"（stopped 态可点）
    await startBtns[0].trigger('click')
    await settle()
    expect(vi.mocked(procApi.apiStartProc)).toHaveBeenCalledTimes(1)
    // 启动不弹确认：不出现停止/重启的后果文案
    expect(w.text()).not.toContain('将立即中断所有正在穿透的远程连接')
    expect(w.text()).not.toContain('将短暂中断当前所有连接后重新建立')
  })
})
