// T-058 (A) / frontend-interaction-polish
// FirewallHint.vue — 剪贴板写入失败不再静默吞错。
//
// 关键模式（insight L14 / L45 + T-057 Wizard.spec messageSpies 单例范式）：
//   - vi.mock('naive-ui') importOriginal + useMessage 返回**单例** messageSpies（否则每次
//     useMessage() 返回新对象无法断言 message.success/error 调用）
//   - mock navigator.clipboard.writeText（resolve / reject 两态）+ document.execCommand
//   - 断言用 wrapper.text() / wrapper.find('button')（DOM），**禁** findComponent({name:'NAlert'})
//     （naive-ui 组件名查询不可靠，T-057 已踩此坑 B.3 FAIL）

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h, nextTick } from 'vue'
import { NConfigProvider, NMessageProvider } from 'naive-ui'

// useMessage 单例 spy（mock 工厂内联定义以规避 vitest hoist 限制）
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

import FirewallHint from '../FirewallHint.vue'
import * as naive from 'naive-ui'

// 取回 mock 工厂内的单例 spy
const messageSpies = (naive as unknown as { __messageSpies: {
  success: ReturnType<typeof vi.fn>
  error: ReturnType<typeof vi.fn>
} }).__messageSpies

const writeTextMock = vi.fn()
const execCommandMock = vi.fn()

function mountHint(ports = [7000]) {
  const Holder = defineComponent({
    setup() {
      return () =>
        h(NConfigProvider, null, {
          default: () =>
            h(NMessageProvider, null, {
              default: () => h(FirewallHint, { ports, proto: 'tcp' }),
            }),
        })
    },
  })
  return mount(Holder, { attachTo: document.body })
}

async function settle(n = 6): Promise<void> {
  for (let i = 0; i < n; i++) await nextTick()
}

// 找到"复制"按钮（单条命令）：按可见文本定位，不按组件名
function findCopyButton(w: ReturnType<typeof mountHint>) {
  return w.findAll('button').find((b) => b.text().includes('复制') && !b.text().includes('全部'))
}
function findCopyAllButton(w: ReturnType<typeof mountHint>) {
  return w.findAll('button').find((b) => b.text().includes('全部'))
}

beforeEach(() => {
  messageSpies.success.mockReset()
  messageSpies.error.mockReset()
  writeTextMock.mockReset()
  execCommandMock.mockReset()
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText: writeTextMock },
    configurable: true,
    writable: true,
  })
  // document.execCommand 在 jsdom 默认不存在，需显式装上
  ;(document as unknown as { execCommand: typeof execCommandMock }).execCommand = execCommandMock
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('FirewallHint.vue — copyCmd 复制单条命令（A）', () => {
  it('clipboard.writeText 成功 → message.success("已复制到剪贴板") + 按钮文案变"已复制 ✓"', async () => {
    writeTextMock.mockResolvedValue(undefined)
    const w = mountHint()
    await settle()
    const btn = findCopyButton(w)
    expect(btn).toBeTruthy()
    await btn!.trigger('click')
    await settle()
    expect(writeTextMock).toHaveBeenCalled()
    expect(messageSpies.success).toHaveBeenCalledWith('已复制到剪贴板')
    expect(messageSpies.error).not.toHaveBeenCalled()
    // 短暂态视觉反馈仍保留
    expect(w.text()).toContain('已复制 ✓')
  })

  it('writeText reject + execCommand 返回 true（fallback 成功）→ message.success', async () => {
    writeTextMock.mockRejectedValue(new Error('clipboard unavailable (insecure context)'))
    execCommandMock.mockReturnValue(true)
    const w = mountHint()
    await settle()
    await findCopyButton(w)!.trigger('click')
    await settle()
    expect(execCommandMock).toHaveBeenCalledWith('copy')
    expect(messageSpies.success).toHaveBeenCalledWith('已复制到剪贴板')
    expect(messageSpies.error).not.toHaveBeenCalled()
  })

  it('writeText reject + execCommand 返回 false（fallback 失败）→ message.error 且不再静默', async () => {
    writeTextMock.mockRejectedValue(new Error('clipboard unavailable'))
    execCommandMock.mockReturnValue(false)
    const w = mountHint()
    await settle()
    await findCopyButton(w)!.trigger('click')
    await settle()
    expect(messageSpies.error).toHaveBeenCalledWith('复制失败：请手动选择文本复制')
    expect(messageSpies.success).not.toHaveBeenCalled()
    // 失败时不应给"已复制 ✓"假反馈
    expect(w.text()).not.toContain('已复制 ✓')
  })
})

describe('FirewallHint.vue — copyAll 复制全部（A）', () => {
  it('writeText 成功 → message.success + 按钮文案"已复制全部 ✓"', async () => {
    writeTextMock.mockResolvedValue(undefined)
    const w = mountHint([7000, 7500])
    await settle()
    await findCopyAllButton(w)!.trigger('click')
    await settle()
    expect(messageSpies.success).toHaveBeenCalledWith('已复制到剪贴板')
    expect(w.text()).toContain('已复制全部 ✓')
  })

  it('writeText reject + execCommand false → message.error', async () => {
    writeTextMock.mockRejectedValue(new Error('blocked'))
    execCommandMock.mockReturnValue(false)
    const w = mountHint([7000, 7500])
    await settle()
    await findCopyAllButton(w)!.trigger('click')
    await settle()
    expect(messageSpies.error).toHaveBeenCalledWith('复制失败：请手动选择文本复制')
  })
})

// ## Adversarial tests
describe('FirewallHint.vue — Adversarial', () => {
  it('clipboard reject 且 execCommand 抛异常（双重失败）→ 仍走 message.error，不抛出未捕获错误', async () => {
    writeTextMock.mockRejectedValue(new Error('NotAllowedError'))
    execCommandMock.mockImplementation(() => {
      throw new Error('execCommand is not supported in this jsdom')
    })
    const w = mountHint()
    await settle()
    // 点击不应抛出未捕获异常
    await expect(findCopyButton(w)!.trigger('click')).resolves.toBeUndefined()
    await settle()
    expect(messageSpies.error).toHaveBeenCalledWith('复制失败：请手动选择文本复制')
    expect(messageSpies.success).not.toHaveBeenCalled()
    expect(w.text()).not.toContain('已复制 ✓')
  })

  it('fallback 失败后临时 textarea 必须从 DOM 移除（不残留隐藏节点）', async () => {
    writeTextMock.mockRejectedValue(new Error('blocked'))
    execCommandMock.mockReturnValue(false)
    const w = mountHint()
    await settle()
    await findCopyButton(w)!.trigger('click')
    await settle()
    // finally 块 removeChild → 不残留 aria-hidden 临时 textarea
    expect(document.querySelectorAll('textarea[aria-hidden="true"]').length).toBe(0)
  })
})
