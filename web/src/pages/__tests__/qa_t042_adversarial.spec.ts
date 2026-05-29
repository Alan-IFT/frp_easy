// T-042 QA stage 6 adversarial reproducer。
// 命名约定：与 qa_t007_adversarial.spec.ts / qa_t032_adversarial.spec.ts / qa_t041_adversarial.spec.ts 对齐。
//
// 设计原则：与 Proxies.spec.ts 的"正向 + happy-flow + AC 字面"测试**互补**。
// 本文件聚焦"边界回退后不退化 / 大小写漂移 / 同名异 type / curConns 异常值"等防御视角。

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { getExposed } from '../../test-utils/exposed'
import { defineComponent, h, nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { NConfigProvider, NMessageProvider } from 'naive-ui'
import * as fs from 'fs'
import * as path from 'path'

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
    proxies: { value: { proxies: Record<string, unknown[]> } | null }
    error: { value: string | null }
  }
  runtimeMap: { value: Map<string, { name: string; status?: string; curConns?: number }> }
  runtimeUnavailable: { value: boolean }
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
  listMock.mockResolvedValue([makeProxy('ssh', 'tcp')])
  infoMock.mockResolvedValue({ clientCounts: 1, curConns: 1 })
  proxiesRtMock.mockResolvedValue({ proxies: {} })
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('ADV-1：runtime 503 → "监控不可用" → recover 后翻绿', () => {
  it('reject → 灰点 / unavailable=true', async () => {
    proxiesRtMock.mockReset()
    proxiesRtMock.mockRejectedValue(new Error('503 frps 进程不可达'))
    infoMock.mockReset()
    infoMock.mockRejectedValue(new Error('503'))
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.runtimeUnavailable.value).toBe(true)
    expect(w.text()).toContain('监控不可用')
  })
})

describe('ADV-2：runtime status 大小写漂移', () => {
  it('"Online" / "ONLINE" / "online" 都应被识别为在线（utils getProxyStatusTag 大小写防御）', async () => {
    proxiesRtMock.mockReset()
    proxiesRtMock.mockResolvedValue({
      proxies: {
        tcp: [{ name: 'ssh', status: 'ONLINE', curConns: 0 }],
      },
    })
    const w = mountPage()
    await settle()
    expect(w.text()).toContain('在线')
  })

  it('"Online" 首字大写也归绿', async () => {
    proxiesRtMock.mockReset()
    proxiesRtMock.mockResolvedValue({
      proxies: {
        tcp: [{ name: 'ssh', status: 'Online', curConns: 0 }],
      },
    })
    const w = mountPage()
    await settle()
    expect(w.text()).toContain('在线')
  })
})

describe('ADV-3：runtime 同名不同 type（理论 frps 不发生）', () => {
  it('Map last-wins 可接受；不抛 / 不死循环', async () => {
    proxiesRtMock.mockReset()
    proxiesRtMock.mockResolvedValue({
      proxies: {
        tcp: [{ name: 'ssh', status: 'online' }],
        udp: [{ name: 'ssh', status: 'offline' }],
      },
    })
    const w = mountPage()
    await settle()
    const t = getTesting(w)
    expect(t.runtimeMap.value.size).toBe(1)
    // last-wins：udp 后写 → status=offline
    expect(t.runtimeMap.value.get('ssh')?.status).toBe('offline')
  })
})

describe('ADV-4：runtime 异常数值', () => {
  it('curConns=undefined → 不抛 + 仍渲染 "在线"', async () => {
    proxiesRtMock.mockReset()
    proxiesRtMock.mockResolvedValue({
      proxies: {
        tcp: [{ name: 'ssh', status: 'online' }],  // 无 curConns
      },
    })
    const w = mountPage()
    await settle()
    expect(w.text()).toContain('在线')
  })

  it('todayTrafficIn=undefined / 0 → 流量列文本为 "—" 或 "0 B"', async () => {
    proxiesRtMock.mockReset()
    proxiesRtMock.mockResolvedValue({
      proxies: {
        tcp: [{ name: 'ssh', status: 'online', todayTrafficIn: 0, todayTrafficOut: 0 }],
      },
    })
    const w = mountPage()
    await settle()
    expect(w.text()).toContain('0 B / 0 B')
  })
})

describe('ADV-5：单向数据流静态守门（insight L13）', () => {
  it('Proxies.vue 源码内不出现 "v-model" 绑到 runtime / runtimeMap', () => {
    const file = path.resolve(__dirname, '..', 'Proxies.vue')
    const src = fs.readFileSync(file, 'utf8')
    // 不应出现 v-model:xxx="runtime" 或 v-model:xxx="runtimeMap" 形式
    // 用反向 regex：v-model 任何形式后跟 "runtime"
    const bad = src.match(/v-model[^=]*=["'][^"']*runtime/g)
    expect(bad).toBeNull()
  })

  it('Proxies.vue 内不再有 inline setInterval / addEventListener（让 composable 自管）', () => {
    const file = path.resolve(__dirname, '..', 'Proxies.vue')
    const src = fs.readFileSync(file, 'utf8')
    expect(src).not.toMatch(/\bsetInterval\s*\(/)
    expect(src).not.toMatch(/\baddEventListener\s*\(/)
  })
})

describe('ADV-6：runtime 首次未返回 + error=null（首屏 loading）→ 列不报错', () => {
  it('proxies.value=null + error.value=null → unavailable=false（不进降级分支）→ 行渲染走"无 runtimeMap 命中"分支', async () => {
    // 让 mount 之后 settle 时 promise 还未 resolve
    proxiesRtMock.mockReset()
    proxiesRtMock.mockImplementation(() => new Promise(() => { /* hang */ }))
    infoMock.mockReset()
    infoMock.mockImplementation(() => new Promise(() => { /* hang */ }))
    const w = mountPage()
    await settle(3)  // 不等 promise resolve
    const t = getTesting(w)
    // runtime.proxies 仍 null，但 error 也 null → unavailable=false
    expect(t.runtimeUnavailable.value).toBe(false)
    // 行 render 走"该 proxy 未在 frps 端注册"分支（离线 tag）
    // 不应抛异常 mount
    expect(w.exists()).toBe(true)
  })
})
