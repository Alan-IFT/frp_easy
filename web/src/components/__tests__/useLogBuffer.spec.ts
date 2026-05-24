import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

// 必须先 mock，再 import 被测模块
vi.mock('../../api/logs', () => ({
  apiGetLogsTail: vi.fn(),
  apiGetLogsIncremental: vi.fn(),
}))

import { useLogBuffer } from '../../composables/log/useLogBuffer'
import * as logsApi from '../../api/logs'

const tailMock = vi.mocked(logsApi.apiGetLogsTail)
const incMock = vi.mocked(logsApi.apiGetLogsIncremental)

describe('useLogBuffer — loadTail 基本路径', () => {
  beforeEach(() => {
    tailMock.mockReset()
    incMock.mockReset()
  })

  it('loadTail 成功 → lines / lastUpdatedAt / firstLoading 状态正确', async () => {
    tailMock.mockResolvedValueOnce({ lines: ['a', 'b', 'c'] })
    const buf = useLogBuffer(() => 'frpc')
    await buf.loadTail()
    expect(buf.lines.value).toEqual(['a', 'b', 'c'])
    expect(buf.firstLoading.value).toBe(false)
    expect(buf.firstLoadError.value).toBeNull()
    expect(buf.lastUpdatedAt.value).toBeGreaterThan(0)
  })

  it('loadTail 失败（首次）→ firstLoadError 非空', async () => {
    tailMock.mockRejectedValueOnce(new Error('boom'))
    const buf = useLogBuffer(() => 'frpc')
    await buf.loadTail()
    expect(buf.firstLoadError.value).toBe('boom')
    expect(buf.lines.value).toEqual([])
  })

  it('AC-16：首次失败后再 loadTail 调用次数 = 2', async () => {
    tailMock
      .mockRejectedValueOnce(new Error('first fail'))
      .mockResolvedValueOnce({ lines: ['ok'] })
    const buf = useLogBuffer(() => 'frpc')
    await buf.loadTail()
    expect(buf.firstLoadError.value).toBe('first fail')
    await buf.loadTail()
    expect(buf.firstLoadError.value).toBeNull()
    expect(buf.lines.value).toEqual(['ok'])
    expect(tailMock).toHaveBeenCalledTimes(2)
  })
})

describe('useLogBuffer — BC-3 slice(-max)', () => {
  beforeEach(() => {
    tailMock.mockReset()
    incMock.mockReset()
  })

  it('500 满载 + 增量 3 行 → 总数 = 500，最早 3 行被裁掉', async () => {
    const initial = Array.from({ length: 500 }, (_, i) => `line ${i}`)
    tailMock.mockResolvedValueOnce({ lines: initial })
    incMock.mockResolvedValueOnce({ data: 'new1\nnew2\nnew3', nextOffset: 999 })
    const buf = useLogBuffer(() => 'frpc')
    await buf.loadTail()
    await buf.loadIncremental()
    expect(buf.lines.value.length).toBe(500)
    expect(buf.lines.value[0]).toBe('line 3')
    expect(buf.lines.value[499]).toBe('new3')
  })

  it('小 max=10 也工作', async () => {
    tailMock.mockResolvedValueOnce({ lines: ['a', 'b', 'c'] })
    incMock.mockResolvedValueOnce({
      data: 'd\ne\nf\ng\nh\ni\nj\nk\nl',
      nextOffset: 100,
    })
    const buf = useLogBuffer(() => 'frpc', { max: 10 })
    await buf.loadTail()
    await buf.loadIncremental()
    expect(buf.lines.value.length).toBe(10)
    // a/b/c + d-l (9 行) = 12 → 裁到 10
    expect(buf.lines.value[0]).toBe('c')
    expect(buf.lines.value[9]).toBe('l')
  })
})

describe('useLogBuffer — AC-7 clear 不调后端', () => {
  beforeEach(() => {
    tailMock.mockReset()
    incMock.mockReset()
  })

  it('clear() → lines=[] + currentOffset 重置 + 后端 0 调用', async () => {
    tailMock.mockResolvedValueOnce({ lines: ['a', 'b'] })
    const buf = useLogBuffer(() => 'frpc')
    await buf.loadTail()
    expect(buf.lines.value).toEqual(['a', 'b'])

    const callsBefore = tailMock.mock.calls.length + incMock.mock.calls.length
    buf.clear()
    expect(buf.lines.value).toEqual([])
    const callsAfter = tailMock.mock.calls.length + incMock.mock.calls.length
    expect(callsAfter).toBe(callsBefore)
  })
})

describe('useLogBuffer — AC-12 / ADV-C：连续 3 次轮询失败停 polling', () => {
  beforeEach(() => {
    tailMock.mockReset()
    incMock.mockReset()
    vi.useFakeTimers()
  })

  it('3 次 reject → stopPolling + autoRefresh=false + message.error 仅调一次', async () => {
    tailMock.mockResolvedValue({ lines: [] })
    incMock.mockRejectedValue(new Error('network'))
    const errSpy = vi.fn()
    const buf = useLogBuffer(() => 'frpc', {
      pollIntervalMs: 100,
      message: { error: errSpy },
    })
    await buf.loadTail()
    buf.setAutoRefresh(true)

    // 触发 3 次 polling
    await vi.advanceTimersByTimeAsync(100)
    await vi.advanceTimersByTimeAsync(100)
    await vi.advanceTimersByTimeAsync(100)
    // 让所有 microtask 收尾
    await Promise.resolve()
    await Promise.resolve()

    expect(buf.consecutiveFailCount.value).toBeGreaterThanOrEqual(3)
    expect(buf.autoRefresh.value).toBe(false)
    expect(errSpy).toHaveBeenCalledTimes(1)
    expect(errSpy).toHaveBeenCalledWith(
      '自动刷新已停止：连续 3 次拉取失败',
    )
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('成功一次后失败计数归零', async () => {
    tailMock.mockResolvedValue({ lines: [] })
    incMock
      .mockRejectedValueOnce(new Error('x'))
      .mockResolvedValueOnce({ data: '', nextOffset: 0 })
    const buf = useLogBuffer(() => 'frpc')
    await buf.loadTail()
    await buf.loadIncremental()
    expect(buf.consecutiveFailCount.value).toBe(1)
    await buf.loadIncremental()
    expect(buf.consecutiveFailCount.value).toBe(0)
  })
})

describe('useLogBuffer — BC-5 / ADV-D kindEpoch race', () => {
  beforeEach(() => {
    tailMock.mockReset()
    incMock.mockReset()
  })

  it('in-flight loadIncremental 在 epoch++ 后丢弃响应', async () => {
    // tail 立即返回；inc 用可控 Promise
    tailMock.mockResolvedValue({ lines: ['init'] })
    let resolveInc: (v: { data: string; nextOffset: number }) => void = () => {}
    incMock.mockReturnValueOnce(
      new Promise((res) => {
        resolveInc = res
      }) as unknown as ReturnType<typeof logsApi.apiGetLogsIncremental>,
    )

    const buf = useLogBuffer(() => 'frpc')
    await buf.loadTail()
    expect(buf.lines.value).toEqual(['init'])

    const incPromise = buf.loadIncremental()
    // 在 in-flight 期间 bump epoch（模拟切 kind）
    const ext = buf as unknown as { __bumpEpoch: () => void }
    ext.__bumpEpoch()
    buf.clear()
    expect(buf.lines.value).toEqual([])

    // 现在迟到的响应解析回来
    resolveInc({ data: 'stale1\nstale2', nextOffset: 500 })
    await incPromise
    // 仍然空，不污染新缓冲
    expect(buf.lines.value).toEqual([])
  })
})

describe('useLogBuffer — parsedLines memoization', () => {
  beforeEach(() => {
    tailMock.mockReset()
    incMock.mockReset()
  })

  it('parsedLines 与 lines 等长 + level 正确', async () => {
    tailMock.mockResolvedValueOnce({
      lines: [
        '2025/01/15 10:23:45 [I] info line',
        '2025/01/15 10:23:46 [E] err line',
        'plain text',
      ],
    })
    const buf = useLogBuffer(() => 'frpc')
    await buf.loadTail()
    const p = buf.parsedLines.value
    expect(p.length).toBe(3)
    expect(p[0].level).toBe('INFO')
    expect(p[1].level).toBe('ERROR')
    expect(p[2].level).toBe('PLAIN')
  })
})

describe('useLogBuffer — stopPolling 清 timer', () => {
  beforeEach(() => {
    tailMock.mockReset()
    incMock.mockReset()
    vi.useFakeTimers()
  })

  it('setAutoRefresh(true) 后 setAutoRefresh(false) → 后续不再触发 incremental', async () => {
    tailMock.mockResolvedValue({ lines: [] })
    incMock.mockResolvedValue({ data: '', nextOffset: 0 })
    const buf = useLogBuffer(() => 'frpc', { pollIntervalMs: 50 })
    buf.setAutoRefresh(true)
    await vi.advanceTimersByTimeAsync(50)
    const callsAfterOne = incMock.mock.calls.length
    expect(callsAfterOne).toBeGreaterThanOrEqual(1)

    buf.setAutoRefresh(false)
    await vi.advanceTimersByTimeAsync(500)
    // 关闭后 inc 不应再被调用（容许容差：等于关闭前）
    expect(incMock.mock.calls.length).toBe(callsAfterOne)
  })

  afterEach(() => {
    vi.useRealTimers()
  })
})
