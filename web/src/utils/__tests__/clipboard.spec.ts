// T-061 / clipboard-util-extract · 02 §3 · AC-2 / AC-5
//
// copyToClipboard 纯函数单测。
//
// 关键模拟范式（insight L37）：
//   - Object.defineProperty(navigator, 'clipboard', { value: { writeText: mock }, configurable: true })
//   - document.execCommand 在 jsdom/happy-dom 默认不存在 → 显式装 mock
//   - util 无 UI（不调 message/useMessage），故无需 naive-ui mock，断言纯布尔 +
//     DOM textarea 残留检查，零 naive-ui 组件名查询（L45 风险不适用）

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { copyToClipboard } from '../clipboard'

const writeTextMock = vi.fn()
const execCommandMock = vi.fn()

beforeEach(() => {
  writeTextMock.mockReset()
  execCommandMock.mockReset()
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText: writeTextMock },
    configurable: true,
    writable: true,
  })
  // document.execCommand 在 jsdom/happy-dom 默认不存在，需显式装上
  ;(document as unknown as { execCommand: typeof execCommandMock }).execCommand = execCommandMock
})

afterEach(() => {
  document.body.innerHTML = ''
})

// 残留临时 textarea 计数（fallback 用 aria-hidden 离屏 textarea）
function strayTextareas(): number {
  return document.querySelectorAll('textarea[aria-hidden="true"]').length
}

describe('copyToClipboard — 首选路径（navigator.clipboard.writeText）', () => {
  it('writeText resolve → 返回 true，且未走 fallback（execCommand 未调用）', async () => {
    writeTextMock.mockResolvedValue(undefined)
    const ok = await copyToClipboard('hello world')
    expect(ok).toBe(true)
    expect(writeTextMock).toHaveBeenCalledTimes(1)
    expect(writeTextMock).toHaveBeenCalledWith('hello world')
    // 未走 fallback
    expect(execCommandMock).not.toHaveBeenCalled()
    expect(strayTextareas()).toBe(0)
  })
})

describe('copyToClipboard — fallback 路径（writeText reject → execCommand）', () => {
  it('writeText reject + execCommand 返回 true → 返回 true', async () => {
    writeTextMock.mockRejectedValue(new Error('clipboard unavailable (insecure context)'))
    execCommandMock.mockReturnValue(true)
    const ok = await copyToClipboard('fallback ok')
    expect(ok).toBe(true)
    expect(execCommandMock).toHaveBeenCalledWith('copy')
    // 临时 textarea 已被 finally 清理
    expect(strayTextareas()).toBe(0)
  })

  it('writeText reject + execCommand 返回 false → 返回 false', async () => {
    writeTextMock.mockRejectedValue(new Error('blocked'))
    execCommandMock.mockReturnValue(false)
    const ok = await copyToClipboard('fallback fail')
    expect(ok).toBe(false)
    expect(execCommandMock).toHaveBeenCalledWith('copy')
    expect(strayTextareas()).toBe(0)
  })

  it('writeText reject + execCommand 抛错 → 返回 false（无未捕获异常）', async () => {
    writeTextMock.mockRejectedValue(new Error('NotAllowedError'))
    execCommandMock.mockImplementation(() => {
      throw new Error('execCommand is not supported in this environment')
    })
    // 不应抛出未捕获异常
    await expect(copyToClipboard('throw path')).resolves.toBe(false)
    // 即便 execCommand 抛错，finally 仍移除临时 textarea
    expect(strayTextareas()).toBe(0)
  })
})

describe('copyToClipboard — 边界', () => {
  it('BC-1：空字符串照常写入，不抛错（首选路径）', async () => {
    writeTextMock.mockResolvedValue(undefined)
    const ok = await copyToClipboard('')
    expect(ok).toBe(true)
    expect(writeTextMock).toHaveBeenCalledWith('')
  })

  it('fallback 路径下临时 textarea 被赋值并提交 select（值正确进入 DOM 后清理）', async () => {
    writeTextMock.mockRejectedValue(new Error('insecure'))
    let capturedValue: string | null = null
    execCommandMock.mockImplementation(() => {
      // execCommand('copy') 触发时，离屏 textarea 应已在 DOM 且持有目标文本
      const ta = document.querySelector('textarea[aria-hidden="true"]') as HTMLTextAreaElement | null
      capturedValue = ta?.value ?? null
      return true
    })
    const ok = await copyToClipboard('captured text')
    expect(ok).toBe(true)
    expect(capturedValue).toBe('captured text')
    // 提交后清理
    expect(strayTextareas()).toBe(0)
  })
})

// ## Adversarial tests
describe('copyToClipboard — Adversarial', () => {
  it('clipboard reject 且 execCommand reject（抛错，双重失败）→ 返回 false 且无未捕获异常 + textarea 清理', async () => {
    writeTextMock.mockRejectedValue(new Error('NotAllowedError: clipboard write denied'))
    execCommandMock.mockImplementation(() => {
      throw new Error('execCommand unsupported / threw')
    })
    // 反向证伪：双重失败既不返回 true 也不外抛，且不残留隐藏节点
    await expect(copyToClipboard('double failure')).resolves.toBe(false)
    expect(strayTextareas()).toBe(0)
  })

  // QA 独立反向证伪（不复用 dev 用例假设）：抽取后首选路径失败时，util 不得"重试/二次调用"
  // navigator.clipboard.writeText（抽函数误把整块包进循环/递归的典型回归）——writeText 恰 1 次。
  it('writeText reject 时 writeText 恰被调用 1 次（不重试、不递归），随后唯一一次走 execCommand', async () => {
    writeTextMock.mockRejectedValue(new Error('insecure context'))
    execCommandMock.mockReturnValue(true)
    const ok = await copyToClipboard('no-retry')
    expect(ok).toBe(true)
    expect(writeTextMock).toHaveBeenCalledTimes(1)
    expect(execCommandMock).toHaveBeenCalledTimes(1)
  })

  // QA 独立反向证伪：util 无共享可变状态——前一次 fallback 失败不得"污染"后一次 fallback 成功
  // （证伪误用模块级 textarea/标志位的回归）。连续两次调用结果各自独立。
  it('连续两次 fallback 调用结果相互独立（无残留状态），且每次都清理 textarea', async () => {
    writeTextMock.mockRejectedValue(new Error('always reject'))
    execCommandMock.mockReturnValueOnce(false).mockReturnValueOnce(true)
    const first = await copyToClipboard('first')
    const second = await copyToClipboard('second')
    expect(first).toBe(false)
    expect(second).toBe(true)
    expect(strayTextareas()).toBe(0)
  })
})
