// T-048 / frontend-consistency-cleanup · A4
// PublicIpDetector.vue — catch 改用 extractErrorMessage 透传后端精确原因。
//
// 关键模式：
//   - mock ../../api/system::apiGetPublicIP 让 mount 不触网
//   - API 失败用 apiError()（结构化错误，extractErrorMessage 才透传 message；
//     new Error 会走 fallback —— 见 test-utils/apiError.ts 注释）
//   - 句柄读 getExposed 不需要（直接断言渲染文本：n-alert "检测失败" 区域）

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { h, defineComponent, nextTick } from 'vue'
import { NMessageProvider } from 'naive-ui'
import { apiError } from '../../test-utils/apiError'

vi.mock('../../api/system', async (orig) => {
  const actual = await orig<typeof import('../../api/system')>()
  return {
    ...actual,
    apiGetPublicIP: vi.fn(),
  }
})

// T-058 (A)：useMessage 单例 spy（保留 naive-ui 其余真实实现，仅替换 useMessage），
// 以便断言 copyIp 的 message.success / message.error 调用。
vi.mock('naive-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  const messageSpies = {
    error: vi.fn(),
    success: vi.fn(),
    warning: vi.fn(),
    info: vi.fn(),
    loading: vi.fn(),
    destroyAll: vi.fn(),
  }
  return {
    ...actual,
    useMessage: () => messageSpies,
    __messageSpies: messageSpies,
  }
})

import PublicIpDetector from '../PublicIpDetector.vue'
import * as systemApi from '../../api/system'
import * as naive from 'naive-ui'

const getIpMock = vi.mocked(systemApi.apiGetPublicIP)
const messageSpies = (naive as unknown as { __messageSpies: {
  success: ReturnType<typeof vi.fn>
  error: ReturnType<typeof vi.fn>
} }).__messageSpies

const writeTextMock = vi.fn()
const execCommandMock = vi.fn()

function withProvider() {
  const Wrapper = defineComponent({
    setup() {
      return () => h(NMessageProvider, null, {
        default: () => h(PublicIpDetector),
      })
    },
  })
  return mount(Wrapper, { attachTo: document.body })
}

async function settle(n = 6): Promise<void> {
  for (let i = 0; i < n; i++) await nextTick()
}

beforeEach(() => {
  getIpMock.mockReset()
  messageSpies.success.mockReset()
  messageSpies.error.mockReset()
  writeTextMock.mockReset()
  execCommandMock.mockReset()
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText: writeTextMock },
    configurable: true,
    writable: true,
  })
  ;(document as unknown as { execCommand: typeof execCommandMock }).execCommand = execCommandMock
})

afterEach(() => {
  document.body.innerHTML = ''
})

// 找"复制"按钮（成功展示 IP 后出现）：按可见文本定位，不按组件名
function findCopyButton(w: ReturnType<typeof withProvider>) {
  return w.findAll('button').find((b) => b.text().includes('复制'))
}

describe('PublicIpDetector.vue — happy path', () => {
  it('成功 → 显示检测到的 IP', async () => {
    getIpMock.mockResolvedValue({ ip: '203.0.113.7' })
    const w = withProvider()
    await w.find('button').trigger('click')
    await settle()
    expect(w.text()).toContain('203.0.113.7')
  })
})

describe('PublicIpDetector.vue — 错误透传（A4）', () => {
  it('apiGetPublicIP reject（apiError）→ 展示后端精确原因，而非写死 fallback', async () => {
    getIpMock.mockRejectedValue(apiError('上游探测服务 503：稍后再试'))
    const w = withProvider()
    await w.find('button').trigger('click')
    await settle()
    // A4 关键对齐：不再丢弃后端原因
    expect(w.text()).toContain('上游探测服务 503：稍后再试')
    expect(w.text()).not.toContain('请求失败，请稍后重试')
  })
})

describe('PublicIpDetector.vue — copyIp 复制 IP（A）', () => {
  async function detectThenGetCopyBtn() {
    getIpMock.mockResolvedValue({ ip: '203.0.113.42' })
    const w = withProvider()
    await w.find('button').trigger('click') // 检测公网 IP
    await settle()
    return { w, btn: findCopyButton(w) }
  }

  it('writeText 成功 → message.success("已复制到剪贴板") + 按钮变"已复制 ✓"', async () => {
    writeTextMock.mockResolvedValue(undefined)
    const { w, btn } = await detectThenGetCopyBtn()
    expect(btn).toBeTruthy()
    await btn!.trigger('click')
    await settle()
    expect(writeTextMock).toHaveBeenCalledWith('203.0.113.42')
    expect(messageSpies.success).toHaveBeenCalledWith('已复制到剪贴板')
    expect(w.text()).toContain('已复制 ✓')
  })

  it('writeText reject + execCommand true（fallback 成功）→ message.success', async () => {
    writeTextMock.mockRejectedValue(new Error('insecure context'))
    execCommandMock.mockReturnValue(true)
    const { btn } = await detectThenGetCopyBtn()
    await btn!.trigger('click')
    await settle()
    expect(execCommandMock).toHaveBeenCalledWith('copy')
    expect(messageSpies.success).toHaveBeenCalledWith('已复制到剪贴板')
    expect(messageSpies.error).not.toHaveBeenCalled()
  })

  it('writeText reject + execCommand false（fallback 失败）→ message.error 不再静默', async () => {
    writeTextMock.mockRejectedValue(new Error('blocked'))
    execCommandMock.mockReturnValue(false)
    const { w, btn } = await detectThenGetCopyBtn()
    await btn!.trigger('click')
    await settle()
    expect(messageSpies.error).toHaveBeenCalledWith('复制失败：请手动选择文本复制')
    expect(messageSpies.success).not.toHaveBeenCalled()
    expect(w.text()).not.toContain('已复制 ✓')
  })
})

// ## Adversarial tests
describe('PublicIpDetector.vue — Adversarial', () => {
  it('clipboard reject 且 execCommand 抛异常（双重失败）→ 仍走 message.error 不抛未捕获错误', async () => {
    getIpMock.mockResolvedValue({ ip: '198.51.100.5' })
    writeTextMock.mockRejectedValue(new Error('NotAllowedError'))
    execCommandMock.mockImplementation(() => {
      throw new Error('execCommand unsupported')
    })
    const w = withProvider()
    await w.find('button').trigger('click')
    await settle()
    const btn = findCopyButton(w)
    await expect(btn!.trigger('click')).resolves.toBeUndefined()
    await settle()
    expect(messageSpies.error).toHaveBeenCalledWith('复制失败：请手动选择文本复制')
    expect(messageSpies.success).not.toHaveBeenCalled()
  })

  it('非结构化错误（无 response.data.error）→ 回落友好 fallback（不外泄裸 Error message）', async () => {
    // 普通 Error 不是 axios 结构化错误 → extractErrorMessage 走 fallback 分支
    getIpMock.mockRejectedValue(new Error('TypeError: undefined is not a function'))
    const w = withProvider()
    await w.find('button').trigger('click')
    await settle()
    expect(w.text()).toContain('请求失败，请稍后重试')
    // 不得把内部 Error 裸消息暴露给用户
    expect(w.text()).not.toContain('undefined is not a function')
  })

  it('成功后再失败：error 文案随当前结果更新，不残留旧 IP', async () => {
    getIpMock.mockResolvedValueOnce({ ip: '198.51.100.9' })
    const w = withProvider()
    await w.find('button').trigger('click')
    await settle()
    expect(w.text()).toContain('198.51.100.9')

    getIpMock.mockRejectedValueOnce(apiError('凭据失效'))
    await w.find('button').trigger('click')
    await settle()
    expect(w.text()).toContain('凭据失效')
    expect(w.text()).not.toContain('198.51.100.9')
  })
})
