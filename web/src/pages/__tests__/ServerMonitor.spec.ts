// T-041 / server-monitor-page-ui · 02 §4.2
// ServerMonitor.vue mount × 多态测试。
//
// 关键模式（insight L4 / L14）：
//   - vi.mock('naive-ui') importOriginal + spread + 6 方法 stub
//   - mountInside(theme) 包 NConfigProvider + NMessageProvider 让 useMessage / useThemeVars 生效
//   - vi.mock('vue-router') → useRouter 桩，避免引入真路由

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { getExposed } from '../../test-utils/exposed'
import { apiError } from '../../test-utils/apiError'
import { defineComponent, h, nextTick } from 'vue'
import { NConfigProvider, NMessageProvider } from 'naive-ui'

// vi.mock 必须先于 import 被测组件
vi.mock('../../api/serverRuntime', () => ({
  apiGetServerRuntimeInfo: vi.fn(),
  apiGetServerRuntimeProxies: vi.fn(),
  apiGetServerRuntimeTraffic: vi.fn(),
}))

// insight L4 / L14：6 方法 stub 模式
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

// vue-router useRouter 桩
const pushSpy = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushSpy }),
}))

import ServerMonitor from '../ServerMonitor.vue'
import * as api from '../../api/serverRuntime'

const infoMock = vi.mocked(api.apiGetServerRuntimeInfo)
const proxiesMock = vi.mocked(api.apiGetServerRuntimeProxies)

interface TestingHandle {
  rt: {
    info: { value: unknown }
    proxies: { value: unknown }
    isPolling: { value: boolean }
    error: { value: string | null }
    consecutiveFailCount: { value: number }
    lastUpdated: { value: number }
  }
  allProxyTypes: { value: string[] }
  activeType: { value: string }
  firstLoading: { value: boolean }
  firstLoadFailed: { value: boolean }
  showFailureBanner: { value: boolean }
  showStaleBanner: { value: boolean }
  goServerHint: { value: boolean }
  lastUpdatedLabel: { value: string }
  formatBytes: (n: number | undefined) => string
  formatTime: (s: string | undefined) => string
  tabLabel: (t: string) => string
  onRefreshClick: () => Promise<void>
  onTogglePolling: () => void
  onRestartPolling: () => void
  goServerConfig: () => void
  isRefreshing: { value: boolean }
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
  // 默认 happy resolve
  infoMock.mockResolvedValue({
    clientCounts: 2,
    curConns: 5,
    version: '0.58.1',
    bindPort: 7000,
    totalTrafficIn: 1024,
    totalTrafficOut: 2048,
  })
  proxiesMock.mockResolvedValue({
    proxies: {
      tcp: [
        {
          name: 'ssh',
          status: 'online',
          curConns: 1,
          todayTrafficIn: 1500,
          todayTrafficOut: 2500,
          lastStartTime: '2025-01-15 10:23:45',
          lastCloseTime: '',
        },
      ],
    },
  })
})

afterEach(() => {
  vi.useRealTimers()
  document.body.innerHTML = ''
})

describe('ServerMonitor — mount 与首屏 happy path', () => {
  it('mount 后 settle → 服务端监控 标题可见', async () => {
    const w = mountPage()
    await settle()
    expect(w.text()).toContain('服务端监控')
  })

  it('AC-1：mount + refresh 完成 → info / proxies 写入', async () => {
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    expect(t.rt.info.value).not.toBeNull()
    expect(t.rt.proxies.value).not.toBeNull()
    expect(t.firstLoading.value).toBe(false)
    expect(t.firstLoadFailed.value).toBe(false)
  })

  it('AC-1：表格中含 proxy name "ssh" + 状态 "在线"', async () => {
    const w = mountPage()
    await settle(8)
    expect(w.text()).toContain('ssh')
    expect(w.text()).toContain('在线')
  })
})

describe('ServerMonitor — 首屏失败 NResult（AC-3 / AC-4 / AC-5）', () => {
  it('AC-4：API reject 含 "frps 进程不可达" → NResult + retry 按钮', async () => {
    infoMock.mockReset()
    proxiesMock.mockReset()
    const err = apiError('frps 进程不可达。请确认 frps 已启动。')
    infoMock.mockRejectedValue(err)
    proxiesMock.mockRejectedValue(err)
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    expect(t.firstLoadFailed.value).toBe(true)
    expect(w.text()).toContain('无法加载 frps 运行态')
    expect(w.text()).toContain('重试')
  })

  it('AC-3 / AC-5：错误含 "dashboard 未启用" → goServerHint=true + "前往服务端配置" 按钮可见', async () => {
    infoMock.mockReset()
    proxiesMock.mockReset()
    const err = apiError('frps dashboard 未启用。请到 Server 设置页打开 Dashboard 开关。')
    infoMock.mockRejectedValue(err)
    proxiesMock.mockRejectedValue(err)
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    expect(t.goServerHint.value).toBe(true)
    expect(w.text()).toContain('前往服务端配置')
  })

  it('AC-5：错误含 "凭据校验失败" → goServerHint=true', async () => {
    infoMock.mockReset()
    proxiesMock.mockReset()
    const err = apiError('frps dashboard 凭据校验失败（401）。')
    infoMock.mockRejectedValue(err)
    proxiesMock.mockRejectedValue(err)
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    expect(t.goServerHint.value).toBe(true)
  })

  it('点 "前往服务端配置" → router.push("/server")', async () => {
    infoMock.mockReset()
    proxiesMock.mockReset()
    const err = new Error('frps dashboard 未启用')
    infoMock.mockRejectedValue(err)
    proxiesMock.mockRejectedValue(err)
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    t.goServerConfig()
    expect(pushSpy).toHaveBeenCalledWith('/server')
  })
})

describe('ServerMonitor — 暂停 / 恢复 / 立即刷新（AC-8 / AC-9）', () => {
  it('AC-8：点 "暂停轮询" → isPolling false', async () => {
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    expect(t.rt.isPolling.value).toBe(true)
    t.onTogglePolling()
    await nextTick()
    expect(t.rt.isPolling.value).toBe(false)
    expect(w.text()).toContain('恢复轮询')
  })

  it('AC-9：点 "立即刷新" → infoMock 调用次数 + 1', async () => {
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    const calls = infoMock.mock.calls.length
    await t.onRefreshClick()
    await settle()
    expect(infoMock.mock.calls.length).toBe(calls + 1)
  })

  it('AC-11：3 次失败 → showFailureBanner=true', async () => {
    infoMock.mockReset()
    proxiesMock.mockReset()
    const err = new Error('x')
    infoMock.mockRejectedValue(err)
    proxiesMock.mockRejectedValue(err)

    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    // 已有 1 次（mount 时 refresh）
    await t.onRefreshClick()
    await t.onRefreshClick()
    await settle()
    expect(t.rt.consecutiveFailCount.value).toBe(3)
    expect(t.showFailureBanner.value).toBe(true)
    expect(w.text()).toContain('自动刷新已停止')
    expect(w.text()).toContain('重启轮询')
  })

  it('restart 按钮 onRestartPolling → 计数清零', async () => {
    infoMock.mockReset()
    proxiesMock.mockReset()
    const err = new Error('x')
    infoMock.mockRejectedValue(err)
    proxiesMock.mockRejectedValue(err)
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    await t.onRefreshClick()
    await t.onRefreshClick()
    await settle()
    expect(t.rt.consecutiveFailCount.value).toBe(3)

    // mock 恢复 happy
    infoMock.mockReset()
    proxiesMock.mockReset()
    infoMock.mockResolvedValue({ clientCounts: 1, curConns: 1 })
    proxiesMock.mockResolvedValue({ proxies: {} })

    t.onRestartPolling()
    await settle()
    expect(t.rt.consecutiveFailCount.value).toBe(0)
    expect(t.rt.isPolling.value).toBe(true)
  })
})

describe('ServerMonitor — empty 与 errors per-type（BC-1 / AC-12）', () => {
  it('BC-1：proxies map 全空 → "暂无连接的 proxy"', async () => {
    infoMock.mockReset()
    proxiesMock.mockReset()
    infoMock.mockResolvedValue({ clientCounts: 0, curConns: 0 })
    proxiesMock.mockResolvedValue({ proxies: {} })
    const w = mountPage()
    await settle(8)
    expect(w.text()).toContain('暂无连接的 proxy')
  })

  it('AC-12：tcp 有数据 + xtcp 错误 → 两个 type tab 各自渲染', async () => {
    infoMock.mockReset()
    proxiesMock.mockReset()
    infoMock.mockResolvedValue({ clientCounts: 1, curConns: 1 })
    proxiesMock.mockResolvedValue({
      proxies: {
        tcp: [{ name: 'svc1', status: 'online' }],
      },
      errors: {
        xtcp: 'xtcp endpoint failed: 502',
      },
    })
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    expect(t.allProxyTypes.value).toContain('tcp')
    expect(t.allProxyTypes.value).toContain('xtcp')
    // 顺序应是 allKnownTypes 固定（tcp 在 xtcp 之前）
    const idxTcp = t.allProxyTypes.value.indexOf('tcp')
    const idxXtcp = t.allProxyTypes.value.indexOf('xtcp')
    expect(idxTcp).toBeLessThan(idxXtcp)
  })
})

describe('ServerMonitor — formatBytes（AC-13 / BC-2）', () => {
  it('0 → "0 B"', async () => {
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    expect(t.formatBytes(0)).toBe('0 B')
  })

  it('undefined → "—"', async () => {
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    expect(t.formatBytes(undefined)).toBe('—')
  })

  it('1024 → "1 KiB"', async () => {
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    expect(t.formatBytes(1024)).toBe('1 KiB')
  })

  it('1536 → "1.5 KiB"', async () => {
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    expect(t.formatBytes(1536)).toBe('1.5 KiB')
  })

  it('1024 * 1024 → "1 MiB"', async () => {
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    expect(t.formatBytes(1024 * 1024)).toBe('1 MiB')
  })
})

describe('ServerMonitor — formatTime（BC-3）', () => {
  it('空字符串 → "—"', async () => {
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    expect(t.formatTime('')).toBe('—')
  })

  it('"0001-01-01 00:00:00" → "—"', async () => {
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    expect(t.formatTime('0001-01-01 00:00:00')).toBe('—')
  })

  it('正常字符串 → 原样返回', async () => {
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    expect(t.formatTime('2025-01-15 10:23:45')).toBe('2025-01-15 10:23:45')
  })
})

describe('ServerMonitor — status 大小写防御（GR C-5）', () => {
  it('status="Online"（大写）也归绿色', async () => {
    infoMock.mockReset()
    proxiesMock.mockReset()
    infoMock.mockResolvedValue({ clientCounts: 1, curConns: 1 })
    proxiesMock.mockResolvedValue({
      proxies: {
        tcp: [{ name: 'capsvc', status: 'Online' }],
      },
    })
    const w = mountPage()
    await settle(8)
    expect(w.text()).toContain('在线')
  })

  it('status="error" → "未知"/原文兜底', async () => {
    infoMock.mockReset()
    proxiesMock.mockReset()
    infoMock.mockResolvedValue({ clientCounts: 1, curConns: 1 })
    proxiesMock.mockResolvedValue({
      proxies: {
        tcp: [{ name: 'errsvc', status: 'error' }],
      },
    })
    const w = mountPage()
    await settle(8)
    // 字面 status="error" → toLowerCase="error" 不归 online/offline → text=row.status="error"
    expect(w.text()).toContain('error')
  })
})

describe('ServerMonitor — lastUpdatedLabel（FR-3.1）', () => {
  it('未刷新 → "尚未刷新"', async () => {
    // 让 mount 时 refresh reject，lastUpdated 保持 0
    infoMock.mockReset()
    proxiesMock.mockReset()
    infoMock.mockRejectedValue(new Error('x'))
    proxiesMock.mockRejectedValue(new Error('x'))
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    expect(t.lastUpdatedLabel.value).toBe('尚未刷新')
  })

  it('刚刚刷新 → "刚刚刷新"', async () => {
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    expect(t.lastUpdatedLabel.value).toBe('刚刚刷新')
  })
})

describe('ServerMonitor — tabLabel 计数（FR-2.2 视觉）', () => {
  it('tcp 含 1 条 → "TCP (1)"', async () => {
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    expect(t.tabLabel('tcp')).toBe('TCP (1)')
  })

  it('udp 不存在 → "UDP (0)"', async () => {
    const w = mountPage()
    await settle(8)
    const t = getTesting(w)
    expect(t.tabLabel('udp')).toBe('UDP (0)')
  })
})
