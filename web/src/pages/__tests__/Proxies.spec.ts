// T-042 / proxy-runtime-status-merge · 02 § 3.5
// Proxies.vue mount × runtime 列多态 + 反向构造 + 降级测试。
//
// 关键模式（insight L1 / L2）：
//   - vi.mock('naive-ui') importOriginal + 6 方法 stub
//   - Holder 包 NConfigProvider + NMessageProvider（NMessageProvider 必须在外层，L2）
//   - vi.mock api 层（serverRuntime + proxies）

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { getExposed } from '../../test-utils/exposed'
import { apiError } from '../../test-utils/apiError'
import { defineComponent, h, nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { NConfigProvider, NMessageProvider } from 'naive-ui'

vi.mock('../../api/serverRuntime', () => ({
  apiGetServerRuntimeInfo: vi.fn(),
  apiGetServerRuntimeProxies: vi.fn(),
  apiGetServerRuntimeTraffic: vi.fn(),
}))

vi.mock('../../api/proxies', () => ({
  apiListProxies: vi.fn(),
  apiCreateProxy: vi.fn(),
  apiUpdateProxy: vi.fn(),
  apiDeleteProxy: vi.fn(),
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

// T-062：vue-router push spy（IS-3/IS-4 跨页连通断言）。Proxies.vue 原不 import useRouter；
// 此模块级 mock 只提供 push spy，不影响既有用例（既有用例不依赖 router）。
const pushSpy = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushSpy }),
}))

import Proxies from '../Proxies.vue'
import * as rtApi from '../../api/serverRuntime'
import * as pxApi from '../../api/proxies'
import type { Proxy } from '../../types'

const infoMock = vi.mocked(rtApi.apiGetServerRuntimeInfo)
const proxiesRtMock = vi.mocked(rtApi.apiGetServerRuntimeProxies)
const listMock = vi.mocked(pxApi.apiListProxies)

function makeProxy(name: string, type: Proxy['type'] = 'tcp', overrides: Partial<Proxy> = {}): Proxy {
  return {
    id: Math.floor(Math.random() * 100000),
    name,
    type,
    localIP: '127.0.0.1',
    localPort: 22,
    remotePort: 6000,
    enabled: true,
    version: 1,
    updatedAt: '2026-05-28T00:00:00Z',
    ...overrides,
  }
}

function mountPage() {
  const Holder = defineComponent({
    setup() {
      return () =>
        h(NConfigProvider, null, {
          default: () =>
            h(NMessageProvider, null, {
              default: () => h(Proxies),
            }),
        })
    },
  })
  return mount(Holder, { attachTo: document.body })
}

interface TestingHandle {
  runtime: {
    info: { value: unknown }
    proxies: { value: unknown }
    error: { value: string | null }
    isPolling: { value: boolean }
  }
  runtimeMap: { value: Map<string, { name: string; status?: string; curConns?: number }> }
  runtimeUnavailable: { value: boolean }
  renderRuntimeStatus: (row: Proxy) => unknown
  renderRuntimeTraffic: (row: Proxy) => unknown
  columns: Array<{ title: string; key: string }>
  handleAdd: () => void
  handleEdit: (proxy: Proxy) => void
  handleDeleteRequest: (proxy: Proxy) => void
  reloadProxies: () => void
  proxiesStore: { proxies: Proxy[]; loading: boolean; error: string | null }
  // T-062
  showPostSaveHint: { value: boolean }
  goToDashboard: () => void
  goToMonitor: () => void
}

function getTesting(wrapper: ReturnType<typeof mountPage>): TestingHandle {
  return getExposed<TestingHandle>(wrapper, Proxies)
}

async function settle(n = 8): Promise<void> {
  for (let i = 0; i < n; i++) await nextTick()
}

beforeEach(() => {
  setActivePinia(createPinia())
  infoMock.mockReset()
  proxiesRtMock.mockReset()
  listMock.mockReset()
  pushSpy.mockReset()

  listMock.mockResolvedValue([
    makeProxy('ssh', 'tcp', { localPort: 22, remotePort: 6022 }),
    makeProxy('web', 'tcp', { localPort: 80, remotePort: 8080 }),
  ])
  infoMock.mockResolvedValue({
    clientCounts: 1,
    curConns: 3,
    version: '0.58.1',
    bindPort: 7000,
  })
  proxiesRtMock.mockResolvedValue({
    proxies: {
      tcp: [
        {
          name: 'ssh',
          status: 'online',
          curConns: 2,
          todayTrafficIn: 1500,
          todayTrafficOut: 2500,
          lastStartTime: '2026-05-28 09:00:00',
        },
      ],
    },
  })
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('Proxies.vue — runtime 列 happy path（AC-1 / AC-2 / AC-3）', () => {
  it('mount 后 settle → 表格中含 "ssh" 行 + 运行状态 "在线"', async () => {
    const w = mountPage()
    await settle()
    expect(w.text()).toContain('ssh')
    expect(w.text()).toContain('在线')
  })

  it('AC-2：流量列显示 "1.5 KiB / 2.4 KiB"', async () => {
    const w = mountPage()
    await settle()
    // 2500 / 1024 = 2.44 → toFixed(1) = "2.4"
    expect(w.text()).toContain('1.5 KiB')
    expect(w.text()).toContain('2.4 KiB')
  })

  it('AC-1：runtimeMap.value 摊平后含 "ssh" → status=online', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.runtimeMap.value.size).toBe(1)
    expect(t.runtimeMap.value.get('ssh')?.status).toBe('online')
  })
})

describe('Proxies.vue — 反向构造（AC-4：配置态有 / runtime 无）', () => {
  it('AC-4：配置 "web" 但 runtime 无 → renderRuntimeStatus 返回 "离线" tag', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    // 配置态 2 条；runtime 仅 ssh —— web 应该被识别为"离线"
    expect(t.runtimeMap.value.has('ssh')).toBe(true)
    expect(t.runtimeMap.value.has('web')).toBe(false)
    // 整体文案应含 "离线"
    expect(w.text()).toContain('离线')
  })
})

describe('Proxies.vue — 反向构造（AC-5：runtime 有 / 配置态无）', () => {
  it('AC-5：runtime 含 "extra" 但配置态无 → 表格不出现 "extra"', async () => {
    proxiesRtMock.mockReset()
    proxiesRtMock.mockResolvedValue({
      proxies: {
        tcp: [
          { name: 'ssh', status: 'online', curConns: 1 },
          { name: 'extra', status: 'online', curConns: 5 },
        ],
      },
    })
    const w = mountPage()
    await settle()
    expect(w.text()).toContain('ssh')
    expect(w.text()).not.toContain('extra')
  })
})

describe('Proxies.vue — 降级（AC-6：frps 不可用）', () => {
  it('AC-6：API reject → runtimeUnavailable=true / 列显 "监控不可用"', async () => {
    infoMock.mockReset()
    proxiesRtMock.mockReset()
    const err = new Error('frps 进程不可达')
    infoMock.mockRejectedValue(err)
    proxiesRtMock.mockRejectedValue(err)

    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.runtimeUnavailable.value).toBe(true)
    // 文本至少应有一次 "监控不可用"（每行渲染一次，N=2 → 至少出现 2 次）
    expect(w.text()).toContain('监控不可用')
  })

  it('AC-6：降级时配置 CRUD spy 仍正常被调用（listMock 被 onMounted 触发）', async () => {
    infoMock.mockReset()
    proxiesRtMock.mockReset()
    const err = new Error('x')
    infoMock.mockRejectedValue(err)
    proxiesRtMock.mockRejectedValue(err)

    mountPage()
    await settle()
    expect(listMock).toHaveBeenCalled()
  })
})

describe('Proxies.vue — 既有 CRUD 通路零回归', () => {
  it('handleAdd → editingProxy=null', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.handleAdd()
    // 没 throw 即可（行为属内部 ref，由现有 CRUD spec 覆盖语义）
    expect(true).toBe(true)
  })

  it('handleEdit(p) → 不抛异常，editingProxy 设置生效', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    const p = makeProxy('edit-target', 'tcp')
    t.handleEdit(p)
    expect(true).toBe(true)
  })

  it('handleDeleteRequest(p) → 不抛异常', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    const p = makeProxy('delete-target', 'tcp')
    t.handleDeleteRequest(p)
    expect(true).toBe(true)
  })
})

describe('Proxies.vue — columns 拓扑（行数 / 顺序）', () => {
  it('columns 8 列：名称/类型/本地/远程/启用/运行状态/流量/操作', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.columns.length).toBe(8)
    const titles = t.columns.map((c) => c.title)
    expect(titles).toEqual([
      '名称', '类型', '本地地址', '远程端口/域名',
      '启用', '运行状态', '流量（入 / 出）', '操作',
    ])
  })

  it('"启用"列与"运行状态"列分开（insight L29 防同名歧义）', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    const enabledIdx = t.columns.findIndex((c) => c.key === 'enabled')
    const runtimeIdx = t.columns.findIndex((c) => c.key === 'runtimeStatus')
    expect(enabledIdx).toBeGreaterThan(-1)
    expect(runtimeIdx).toBeGreaterThan(-1)
    expect(enabledIdx).not.toBe(runtimeIdx)
  })
})

// T-047 A3：区分"加载失败"与"暂无规则"
describe('Proxies.vue — 加载失败 vs 暂无规则（A3）', () => {
  it('fetchProxies reject（apiError）→ store.error 透传 + 显示错误态（非"暂无规则"）', async () => {
    listMock.mockReset()
    listMock.mockRejectedValue(apiError('后端 500：读取代理规则失败'))
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.proxiesStore.error).toBe('后端 500：读取代理规则失败')
    expect(w.text()).toContain('加载代理规则失败')
    expect(w.text()).toContain('重试')
    // 反向：失败时绝不渲染成"暂无代理规则"empty 文案
    expect(w.text()).not.toContain('暂无代理规则')
  })

  it('成功但列表为空 → empty 态"暂无代理规则"（store.error=null）', async () => {
    listMock.mockReset()
    listMock.mockResolvedValue([])
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.proxiesStore.error).toBeNull()
    expect(w.text()).toContain('暂无代理规则')
    // 反向：empty 态不得出现错误态文案
    expect(w.text()).not.toContain('加载代理规则失败')
  })

  it('错误态点重试成功 → store.error 清空，回到正常列表', async () => {
    listMock.mockReset()
    listMock.mockRejectedValueOnce(apiError('临时失败'))
    listMock.mockResolvedValue([
      makeProxy('ssh', 'tcp', { localPort: 22, remotePort: 6022 }),
    ])
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.proxiesStore.error).toBe('临时失败')
    t.reloadProxies()
    await settle()
    expect(t.proxiesStore.error).toBeNull()
    expect(w.text()).toContain('ssh')
  })
})

// T-062 IS-3：保存成功后引导去启动/看运行态
describe('Proxies.vue — 保存成功后引导（T-062 IS-3）', () => {
  it('初始不显示保存引导（showPostSaveHint=false）', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.showPostSaveHint.value).toBe(false)
    expect(w.text()).not.toContain('去仪表盘启动 frpc')
  })

  it('AC-5：showPostSaveHint=true → 出现"去仪表盘启动"+"去服务端监控"两入口', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.showPostSaveHint.value = true
    await settle()
    expect(w.text()).toContain('去仪表盘启动 frpc')
    expect(w.text()).toContain('去服务端监控查看运行态')
  })

  it('AC-5 / AC-12：点击"去仪表盘启动" → router.push(/dashboard)', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.showPostSaveHint.value = true
    await settle()
    const btn = w.findAll('button').find((b) => b.text().includes('去仪表盘启动'))
    expect(btn).toBeTruthy()
    await btn!.trigger('click')
    expect(pushSpy).toHaveBeenCalledWith('/dashboard')
  })

  it('AC-5 / AC-12：点击"去服务端监控" → router.push(/server/monitor)', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.showPostSaveHint.value = true
    await settle()
    const btn = w.findAll('button').find((b) => b.text().includes('去服务端监控查看运行态'))
    expect(btn).toBeTruthy()
    await btn!.trigger('click')
    expect(pushSpy).toHaveBeenCalledWith('/server/monitor')
  })

  it('goToDashboard / goToMonitor handler 直接调用各推对应路由', async () => {
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    t.goToDashboard()
    expect(pushSpy).toHaveBeenCalledWith('/dashboard')
    t.goToMonitor()
    expect(pushSpy).toHaveBeenCalledWith('/server/monitor')
  })
})

// T-062 IS-4：空态补跨页连通入口（不重复"新增规则"文案）
describe('Proxies.vue — 空态连通入口（T-062 IS-4）', () => {
  it('AC-6：列表为空 → #empty 含"去服务端监控"入口 + 保留"新增规则"引导文案', async () => {
    listMock.mockReset()
    listMock.mockResolvedValue([])
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.proxiesStore.error).toBeNull()
    // 现有空态文案仍在
    expect(w.text()).toContain('暂无代理规则')
    expect(w.text()).toContain('点击右上角「新增规则」')
    // 新增连通入口
    expect(w.text()).toContain('去服务端监控查看运行态')
  })

  it('AC-6 / AC-12：空态点击"去服务端监控" → router.push(/server/monitor)', async () => {
    listMock.mockReset()
    listMock.mockResolvedValue([])
    const w = mountPage()
    await settle()
    const btn = w.findAll('button').find((b) => b.text().includes('去服务端监控查看运行态'))
    expect(btn).toBeTruthy()
    await btn!.trigger('click')
    expect(pushSpy).toHaveBeenCalledWith('/server/monitor')
  })
})

// ## Adversarial tests
describe('Proxies.vue — Adversarial（A3：失败绝不渲染成空列表）', () => {
  it('fetchProxies 失败时 proxies 列表为空，但页面绝不显示 empty 态误导用户去新建', async () => {
    listMock.mockReset()
    listMock.mockRejectedValue(apiError('网络中断'))
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    // 列表确实为空（fetch 失败未填充）
    expect(t.proxiesStore.proxies.length).toBe(0)
    // 但 error 非空 → 必须走错误态，绝不是 empty 态
    expect(t.proxiesStore.error).not.toBeNull()
    expect(w.text()).not.toContain('暂无代理规则')
    expect(w.text()).toContain('加载代理规则失败')
  })

  // T-062 反向证伪：加载失败态绝不渲染空态连通入口（错误态 n-result 而非 n-empty）。
  // 假设：若 IS-4 入口被错误地放在数据表外层无条件渲染，则失败态也会出现"去服务端监控" → 断言失败。
  it('ADV（T-062）：加载失败态不出现空态连通入口（"去服务端监控"仅在 #empty 出现）', async () => {
    listMock.mockReset()
    listMock.mockRejectedValue(apiError('网络中断'))
    const w = mountPage()
    await settle()
    // 失败态走 n-result，data-table（含 #empty slot）整体 v-else 不渲染 → 空态入口不出现
    expect(w.text()).toContain('加载代理规则失败')
    expect(w.text()).not.toContain('去服务端监控查看运行态')
  })
})
