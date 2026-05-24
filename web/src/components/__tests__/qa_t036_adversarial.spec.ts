/**
 * T-036 / log-ui-ux-polish · Stage 6 QA Tester 独立对抗测试
 *
 * 这是 QA 写的反向证伪 reproducer，**不复用 dev 的 spec**，独立从
 * 01_REQUIREMENT_ANALYSIS.md 的 AC / NFR 出发构造预期会失败的输入，再验证实现是否抗住。
 *
 * - ADV-A：XSS payload `<script>alert(1)</script>` 搜索时验证 textContent 含 `<` 但
 *   DOM 中 `querySelectorAll('script').length === 0`（NFR-7）
 * - ADV-B：localStorage.setItem 强 throw → useLogPrefs setter 不崩 + UI 仍切换（BC-13）
 * - ADV-C：apiGetLogsIncremental 连续 3 次 reject → polling 自动停 + autoRefresh 切 false
 *   + message.error 仅一次（AC-12 / BC-6 / 2.4 §21）
 * - ADV-D：kindEpoch race（frpc → frps 切换中 in-flight 响应不污染新缓冲）（BC-5）
 *
 * 还附加 4 条 QA 独立想到的额外对抗：
 * - ADV-E：`<img onerror>` 类 XSS payload（不止 <script>，全 tag 一视同仁 escape）
 * - ADV-F：搜索关键字含 regex 元字符 `*` / `(` / `\` —— 应作字面子串匹配，不抛错
 * - ADV-G：空 needle 不死循环（useLogSearch findHits 边界）
 * - ADV-H：单行 5KB（巨长行） + 折行 → DOM 不抛错且行号仍 1
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick, h, defineComponent, ref } from 'vue'
import { NConfigProvider, NMessageProvider } from 'naive-ui'

// 顺序：先 mock，后 import
vi.mock('../../api/logs', () => ({
  apiGetLogsTail: vi.fn(),
  apiGetLogsIncremental: vi.fn(),
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
  }
})

import LogViewer from '../LogViewer.vue'
import * as logsApi from '../../api/logs'
import { useLogBuffer } from '../../composables/log/useLogBuffer'
import { useLogPrefs } from '../../composables/log/useLogPrefs'
import { useLogSearch } from '../../composables/log/useLogSearch'

const tailMock = vi.mocked(logsApi.apiGetLogsTail)
const incMock = vi.mocked(logsApi.apiGetLogsIncremental)

function mountInside(kind: string) {
  const Holder = defineComponent({
    props: { kind: { type: String, required: true } },
    setup(p) {
      return () =>
        h(NConfigProvider, { theme: null }, {
          default: () =>
            h(NMessageProvider, null, {
              default: () => h(LogViewer, { kind: p.kind }),
            }),
        })
    },
  })
  return mount(Holder, { props: { kind }, attachTo: document.body })
}

async function settle(times = 3) {
  for (let i = 0; i < times; i++) await nextTick()
}

interface TestingHandle {
  buf: {
    lines: { value: string[] }
    autoRefresh: { value: boolean }
    consecutiveFailCount: { value: number }
    setAutoRefresh: (v: boolean) => void
  }
  search: {
    setQuery: (q: string) => void
    visibleLines: { value: unknown[] }
  }
  prefs: {
    setWrap: (v: boolean) => void
    setHeight: (v: 300 | 500 | 800) => void
    wrap: { value: boolean }
    height: { value: number }
  }
}

function getTesting(wrapper: ReturnType<typeof mountInside>): TestingHandle {
  const lv = wrapper.findComponent(LogViewer)
  return (lv.vm as unknown as { __testing: TestingHandle }).__testing
}

beforeEach(() => {
  tailMock.mockReset()
  incMock.mockReset()
  window.localStorage.clear()
  tailMock.mockResolvedValue({ lines: [] })
  incMock.mockResolvedValue({ data: '', nextOffset: 0 })
})

afterEach(() => {
  vi.useRealTimers()
  document.body.innerHTML = ''
})

describe('QA-ADV-A：XSS escape — `<script>alert(1)</script>` 搜索后 DOM 中无 script 元素', () => {
  // Hypothesis: I expect failure if escape happens AFTER `<mark>` wrapping,
  // because then `<mark>` itself gets escaped to `&lt;mark&gt;` and the
  // `<script>` text would be interpreted as raw HTML by v-html.
  it('日志含 <script>alert(1)</script> + 搜索 "<script>" → 真实 DOM 0 个 script 元素 + textContent 含字面 `<script>alert(1)</script>`', async () => {
    tailMock.mockResolvedValueOnce({
      lines: ['attacker payload: <script>alert(1)</script> end'],
    })
    const w = mountInside('frpc')
    await settle()
    const t = getTesting(w)
    t.search.setQuery('<script>')
    await nextTick()

    // 反向证伪 1：整个 .log-viewer-root 内不存在任何真实 <script> 元素
    const rootEl = w.element as HTMLElement
    expect(rootEl.querySelectorAll('script').length).toBe(0)

    // 反向证伪 2：.line-message 元素的 textContent 包含完整字面文本（包含 `<` `>`）
    const msgEl = rootEl.querySelector('.line-message') as HTMLElement
    expect(msgEl).toBeTruthy()
    expect(msgEl.textContent).toContain('<script>alert(1)</script>')

    // 反向证伪 3：<mark> 节点是 mark.search-hit（说明高亮路径生效）
    const marks = msgEl.querySelectorAll('mark.search-hit')
    expect(marks.length).toBeGreaterThanOrEqual(1)
    // 第一个 mark 的 textContent 是字面 `<script>`，但 mark 内 0 个 script 子元素
    expect((marks[0] as HTMLElement).textContent).toBe('<script>')
    expect((marks[0] as HTMLElement).querySelectorAll('script').length).toBe(0)
  })
})

describe('QA-ADV-E：XSS escape — `<img src=x onerror=alert(1)>` 同型攻击全 tag 抗住', () => {
  // Hypothesis: I expect failure if escape only covers `<script>` specifically;
  // attacker may pivot to `<img onerror>` to dodge a naive blacklist.
  it('payload 含 <img src=x onerror=alert(1)> → 0 个 img 元素 + textContent 含字面文本', async () => {
    tailMock.mockResolvedValueOnce({
      lines: ['<img src=x onerror=alert(1)>'],
    })
    const w = mountInside('frpc')
    await settle()
    const rootEl = w.element as HTMLElement
    expect(rootEl.querySelectorAll('img').length).toBe(0)
    expect(rootEl.querySelectorAll('iframe').length).toBe(0)
    const msgEl = rootEl.querySelector('.line-message') as HTMLElement
    expect(msgEl.textContent).toContain('<img src=x onerror=alert(1)>')
  })

  it('payload 含 `<iframe src=javascript:alert(1)>` → 0 个 iframe', async () => {
    tailMock.mockResolvedValueOnce({
      lines: ['<iframe src="javascript:alert(1)"></iframe>'],
    })
    const w = mountInside('frpc')
    await settle()
    const rootEl = w.element as HTMLElement
    expect(rootEl.querySelectorAll('iframe').length).toBe(0)
    expect(rootEl.querySelectorAll('script').length).toBe(0)
  })
})

describe('QA-ADV-B：localStorage.setItem 强 throw 时 useLogPrefs 不崩', () => {
  // Hypothesis: I expect failure if setter does not try/catch around setItem;
  // Quota-exceeded or Safari ITP causes setItem to throw synchronously.
  let originalSetItem: typeof window.localStorage.setItem
  beforeEach(() => {
    originalSetItem = window.localStorage.setItem.bind(window.localStorage)
  })
  afterEach(() => {
    Object.defineProperty(window.localStorage, 'setItem', {
      configurable: true,
      writable: true,
      value: originalSetItem,
    })
  })

  it('setItem 始终 throw → setWrap / setHeight / setFollowTail 均不崩，UI value 仍生效', () => {
    const throwingSet = vi.fn(() => {
      throw new Error('QuotaExceededError simulated by QA')
    })
    Object.defineProperty(window.localStorage, 'setItem', {
      configurable: true,
      writable: true,
      value: throwingSet,
    })
    const p = useLogPrefs()
    expect(() => p.setWrap(false)).not.toThrow()
    expect(p.wrap.value).toBe(false)
    expect(() => p.setHeight(800)).not.toThrow()
    expect(p.height.value).toBe(800)
    expect(() => p.setFollowTail(false)).not.toThrow()
    expect(p.followTail.value).toBe(false)
    expect(() => p.setCaseSensitive(true)).not.toThrow()
    expect(p.caseSensitive.value).toBe(true)
    expect(() => p.flush()).not.toThrow()
  })

  it('mount 级 LogViewer 在 setItem throw 下 UI 仍可切换偏好', async () => {
    Object.defineProperty(window.localStorage, 'setItem', {
      configurable: true,
      writable: true,
      value: vi.fn(() => {
        throw new Error('QuotaExceededError')
      }),
    })
    const w = mountInside('frpc')
    await settle()
    const t = getTesting(w)
    expect(() => t.prefs.setWrap(false)).not.toThrow()
    expect(t.prefs.wrap.value).toBe(false)
    expect(() => t.prefs.setHeight(800)).not.toThrow()
    expect(t.prefs.height.value).toBe(800)
  })
})

describe('QA-ADV-C：连续 3 次 incremental reject → polling 自停 + autoRefresh false + message.error 一次', () => {
  // Hypothesis: I expect failure if the impl resets the failure counter at the
  // wrong time or fires message.error 3 times. Both would be regressions.
  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('3 次 reject → consecutiveFailCount ≥ 3 + autoRefresh=false + opts.message.error 调一次', async () => {
    tailMock.mockResolvedValue({ lines: [] })
    incMock.mockRejectedValue(new Error('QA simulated network down'))
    const errSpy = vi.fn()
    const buf = useLogBuffer(() => 'frpc', {
      pollIntervalMs: 50,
      message: { error: errSpy },
    })
    await buf.loadTail()
    buf.setAutoRefresh(true)
    for (let i = 0; i < 4; i++) {
      await vi.advanceTimersByTimeAsync(50)
    }
    // 让所有 microtask 收尾
    await Promise.resolve()
    await Promise.resolve()
    expect(buf.consecutiveFailCount.value).toBeGreaterThanOrEqual(3)
    expect(buf.autoRefresh.value).toBe(false)
    expect(errSpy).toHaveBeenCalledTimes(1)
    expect(errSpy.mock.calls[0][0]).toContain('连续 3 次')
  })

  it('继续 advanceTimers 5 个周期后 message.error 仍仅调用一次', async () => {
    tailMock.mockResolvedValue({ lines: [] })
    incMock.mockRejectedValue(new Error('persistent down'))
    const errSpy = vi.fn()
    const buf = useLogBuffer(() => 'frpc', {
      pollIntervalMs: 50,
      message: { error: errSpy },
    })
    await buf.loadTail()
    buf.setAutoRefresh(true)
    for (let i = 0; i < 10; i++) {
      await vi.advanceTimersByTimeAsync(50)
    }
    await Promise.resolve()
    // polling 已停 → 后续 5 个周期不会再触发 inc，error 仍 = 1
    expect(errSpy).toHaveBeenCalledTimes(1)
  })
})

describe('QA-ADV-D：kindEpoch race — in-flight loadIncremental 不污染新缓冲', () => {
  // Hypothesis: I expect failure if the impl doesn't compare epoch after await;
  // late frpc response would push stale lines into the now-cleared frps buffer.
  it('frpc inc 飞行中 bumpEpoch + clear → 迟到响应到达后 lines 仍空', async () => {
    tailMock.mockResolvedValue({ lines: ['init'] })
    let lateResolve: (
      v: { data: string; nextOffset: number },
    ) => void = () => {}
    incMock.mockReturnValueOnce(
      new Promise((res) => {
        lateResolve = res
      }) as unknown as ReturnType<typeof logsApi.apiGetLogsIncremental>,
    )
    const buf = useLogBuffer(() => 'frpc') as ReturnType<typeof useLogBuffer> & {
      __bumpEpoch: () => void
    }
    await buf.loadTail()
    expect(buf.lines.value).toEqual(['init'])

    const inFlight = buf.loadIncremental()
    buf.__bumpEpoch()
    buf.clear()
    expect(buf.lines.value).toEqual([])

    lateResolve({ data: 'STALE1\nSTALE2\nSTALE3', nextOffset: 9999 })
    await inFlight

    // 关键：迟到响应的 3 行 STALE 数据**不**进入清空后的缓冲
    expect(buf.lines.value).toEqual([])
  })

  it('mount 级：watch kind 切换 frpc → frps 期间 in-flight loadTail 不污染新缓冲', async () => {
    // 第一次 tail 是 in-flight；watch 切 kind 时 bumpEpoch；第二次 tail 立即返回
    let firstResolve: (v: { lines: string[] }) => void = () => {}
    tailMock.mockImplementationOnce(
      () =>
        new Promise((res) => {
          firstResolve = res
        }) as Promise<{ lines: string[] }>,
    )
    tailMock.mockResolvedValueOnce({ lines: ['fresh-frps-line'] })

    const w = mountInside('frpc')
    // 不 settle —— 让首次 tail 留在 in-flight
    await nextTick()

    await w.setProps({ kind: 'frps' })
    await settle(3)

    // 现在让 frpc 的第一次响应迟到归来
    firstResolve({ lines: ['STALE-FRPC-1', 'STALE-FRPC-2', 'STALE-FRPC-3'] })
    await settle(5)

    const t = getTesting(w)
    // 关键：不应被 frpc 的迟到响应污染；应当只有 frps 的新行
    for (const stale of ['STALE-FRPC-1', 'STALE-FRPC-2', 'STALE-FRPC-3']) {
      expect(t.buf.lines.value).not.toContain(stale)
    }
  })
})

describe('QA-ADV-F：搜索关键字含 regex 元字符 — 字面子串匹配不抛错', () => {
  // Hypothesis: I expect failure if impl uses new RegExp(needle) without
  // escape — `*` / `(` / `\` could throw or trigger catastrophic backtrack.
  it('搜索 "(*)" → 不抛错，命中行数 = 含字面 (*) 的行数', () => {
    const source = ref<{ raw: string; level: 'PLAIN'; message: string }[]>([
      { raw: 'foo (*) bar', level: 'PLAIN', message: 'foo (*) bar' },
      { raw: 'foo bar', level: 'PLAIN', message: 'foo bar' },
      { raw: '(*) at start', level: 'PLAIN', message: '(*) at start' },
    ])
    const cs = ref(false)
    const s = useLogSearch(source, cs)
    expect(() => s.setQuery('(*)')).not.toThrow()
    // computed 求值
    const vis = s.visibleLines.value
    expect(vis.length).toBe(2)
  })

  it('搜索反斜杠 `\\` 不抛错', () => {
    const source = ref<{ raw: string; level: 'PLAIN'; message: string }[]>([
      { raw: 'C:\\Programs\\frp_easy', level: 'PLAIN', message: 'C:\\Programs\\frp_easy' },
      { raw: 'plain', level: 'PLAIN', message: 'plain' },
    ])
    const cs = ref(false)
    const s = useLogSearch(source, cs)
    expect(() => s.setQuery('\\Programs\\')).not.toThrow()
    expect(s.visibleLines.value.length).toBe(1)
  })
})

describe('QA-ADV-G：空 needle / 空白 needle 不死循环', () => {
  // Hypothesis: I expect failure (infinite loop, hang) if indexOf("") returns 0
  // and impl doesn't guard.
  it('设空 query → visibleLines 包含全部源行', () => {
    const source = ref<{ raw: string; level: 'PLAIN'; message: string }[]>([
      { raw: 'a', level: 'PLAIN', message: 'a' },
      { raw: 'b', level: 'PLAIN', message: 'b' },
    ])
    const cs = ref(false)
    const s = useLogSearch(source, cs)
    s.setQuery('')
    expect(s.visibleLines.value.length).toBe(2)
  })

  it('设全空白 query → 等价空 needle', () => {
    const source = ref<{ raw: string; level: 'PLAIN'; message: string }[]>([
      { raw: 'a', level: 'PLAIN', message: 'a' },
    ])
    const cs = ref(false)
    const s = useLogSearch(source, cs)
    s.setQuery('   ')
    // trim 后空 → 全可见
    expect(s.visibleLines.value.length).toBe(1)
  })
})

describe('QA-ADV-H：5 KB 单行 + 折行模式 DOM 不抛错', () => {
  // Hypothesis: I expect failure (DOM error / overflow / lineNumber drift)
  // if impl naive-appends without controlling word-break behavior.
  it('单行 5000 字符 → DOM 渲染成功 + .log-line.wrap 存在 + 行号 = 1', async () => {
    const huge = 'X'.repeat(5000)
    tailMock.mockResolvedValueOnce({ lines: [huge] })
    const w = mountInside('frpc')
    await settle()
    expect(w.find('.log-line.wrap').exists()).toBe(true)
    const numEl = w.find('.log-line.wrap .line-number')
    expect(numEl.text().trim()).toBe('1')
    const msgEl = w.find('.log-line.wrap .line-message')
    expect((msgEl.element as HTMLElement).textContent?.length).toBe(5000)
  })
})

describe('QA-ADV：额外健壮性 — 极端 input', () => {
  it('lines 含 null 字节 \\u0000 → 不崩 + 显示', async () => {
    tailMock.mockResolvedValueOnce({
      lines: ['before after'],
    })
    const w = mountInside('frpc')
    await settle()
    expect(w.find('.log-line').exists()).toBe(true)
  })

  it('lines 含全角字符 / emoji → 渲染正常', async () => {
    tailMock.mockResolvedValueOnce({
      lines: ['2025/01/15 10:00:00 [I] 中文 测试 🚀 emoji'],
    })
    const w = mountInside('frpc')
    await settle()
    const msgEl = w.find('.line-message')
    expect((msgEl.element as HTMLElement).textContent).toContain('🚀')
    expect((msgEl.element as HTMLElement).textContent).toContain('中文')
  })
})
