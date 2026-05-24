import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick, h, defineComponent } from 'vue'
import { NConfigProvider, NMessageProvider, darkTheme } from 'naive-ui'

// 必须先 mock，再 import 被测组件
vi.mock('../../api/logs', () => ({
  apiGetLogsTail: vi.fn(),
  apiGetLogsIncremental: vi.fn(),
}))

// insight L29：importOriginal + 6 方法 stub mock 模式。
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

const tailMock = vi.mocked(logsApi.apiGetLogsTail)
const incMock = vi.mocked(logsApi.apiGetLogsIncremental)

// 让 LogViewer 嵌在 NConfigProvider + NMessageProvider 下，
// 但 props.kind 通过外层 ref 提供，外层暴露给测试 setProps 切 kind。
function mountInside(kind: string, theme: 'light' | 'dark' = 'light') {
  const Holder = defineComponent({
    props: { kind: { type: String, required: true } },
    setup(p) {
      return () =>
        h(
          NConfigProvider,
          { theme: theme === 'dark' ? darkTheme : null },
          {
            default: () =>
              h(NMessageProvider, null, {
                default: () => h(LogViewer, { kind: p.kind }),
              }),
          },
        )
    },
  })
  return mount(Holder, { props: { kind }, attachTo: document.body })
}

async function settle(times = 3) {
  for (let i = 0; i < times; i++) await nextTick()
}

interface TestingHandle {
  prefs: {
    setWrap: (v: boolean) => void
    setHeight: (v: 300 | 500 | 800) => void
    setFollowTail: (v: boolean) => void
    setCaseSensitive: (v: boolean) => void
    wrap: { value: boolean }
    height: { value: number }
    heightPx: { value: number }
    caseSensitive: { value: boolean }
  }
  buf: {
    lines: { value: string[] }
    autoRefresh: { value: boolean }
    consecutiveFailCount: { value: number }
    firstLoadError: { value: string | null }
    setAutoRefresh: (v: boolean) => void
  }
  filter: {
    activeLevels: { value: string[] }
    setActiveLevels: (l: string[]) => void
  }
  search: {
    query: { value: string }
    setQuery: (q: string) => void
    visibleLines: { value: unknown[] }
  }
  follow: { enabled: { value: boolean }; paused: { value: boolean } }
  rootCssVars: { value: Record<string, string> }
  fullscreenOpen: { value: boolean }
  onCopy: () => Promise<void>
  onClear: () => void
  onClearFilters: () => void
  onRetry: () => void
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

describe('LogViewer — mount & 基本渲染', () => {
  it('mount 成功，工具条 / 列表容器存在', async () => {
    const w = mountInside('frpc')
    await settle()
    const t = getTesting(w)
    expect(t.filter.activeLevels.value.length).toBe(6)
    expect(w.find('.log-viewer-root').exists()).toBe(true)
    expect(w.find('.log-list-root').exists()).toBe(true)
  })

  it('AC-15：空缓冲 → "暂无日志输出" 文案可见 + 不渲染 log-line', async () => {
    tailMock.mockResolvedValueOnce({ lines: [] })
    const w = mountInside('frpc')
    await settle()
    expect(w.text()).toContain('暂无日志输出')
    expect(w.findAll('.log-line').length).toBe(0)
  })

  it('AC-1：fixture 含 ERROR / INFO / PLAIN 三行 → DOM 有对应 level-error class', async () => {
    tailMock.mockResolvedValueOnce({
      lines: [
        '2025/01/15 10:23:45 [E] connection refused',
        '2025/01/15 10:23:46 [I] info line',
        'plain text',
      ],
    })
    const w = mountInside('frpc')
    await settle()
    expect(w.find('.log-line.level-error').exists()).toBe(true)
    expect(w.find('.log-line.level-info').exists()).toBe(true)
    expect(w.find('.log-line.level-plain').exists()).toBe(true)
  })
})

describe('LogViewer — AC-2 搜索过滤', () => {
  it('搜索 "refused" → visibleLines.length=1', async () => {
    tailMock.mockResolvedValueOnce({
      lines: [
        '2025/01/15 10:23:45 [E] connection refused',
        '2025/01/15 10:23:46 [I] all good',
        '2025/01/15 10:23:47 [I] also good',
      ],
    })
    const w = mountInside('frpc')
    await settle()
    const t = getTesting(w)
    t.search.setQuery('refused')
    await nextTick()
    expect(t.search.visibleLines.value.length).toBe(1)
  })
})

describe('LogViewer — AC-3 等级筛选 + BC-9 全去勾', () => {
  it('去掉 INFO → INFO 行隐藏', async () => {
    tailMock.mockResolvedValueOnce({
      lines: [
        '2025/01/15 10:23:45 [E] err',
        '2025/01/15 10:23:46 [I] info',
        '2025/01/15 10:23:47 [W] warn',
      ],
    })
    const w = mountInside('frpc')
    await settle()
    const t = getTesting(w)
    t.filter.setActiveLevels(['ERROR', 'WARN'])
    await nextTick()
    expect(t.search.visibleLines.value.length).toBe(2)
  })

  it('BC-9：全去勾 → "请至少选择一个日志等级" 提示', async () => {
    tailMock.mockResolvedValueOnce({ lines: ['2025/01/15 10:23:45 [I] x'] })
    const w = mountInside('frpc')
    await settle()
    const t = getTesting(w)
    t.filter.setActiveLevels([])
    await nextTick()
    expect(t.search.visibleLines.value.length).toBe(0)
    expect(w.text()).toContain('请至少选择一个日志等级')
  })
})

describe('LogViewer — AC-6 复制全部', () => {
  it('onCopy → navigator.clipboard.writeText 收到拼接 raw 字符串', async () => {
    tailMock.mockResolvedValueOnce({
      lines: [
        '2025/01/15 10:23:45 [I] line a',
        '2025/01/15 10:23:46 [W] line b',
      ],
    })
    const writeSpy = vi.fn(async () => undefined)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText: writeSpy },
    })
    const w = mountInside('frpc')
    await settle()
    const t = getTesting(w)
    await t.onCopy()
    expect(writeSpy).toHaveBeenCalledTimes(1)
    const calls = writeSpy.mock.calls as unknown as Array<[string]>
    const arg: string = calls[0][0]
    expect(arg).toContain('line a')
    expect(arg).toContain('line b')
    expect(arg).not.toContain('<')
    expect(arg).not.toContain('mark')
  })
})

describe('LogViewer — AC-7 清屏不调后端', () => {
  it('onClear → lines=[] 且后端调用次数不增长', async () => {
    tailMock.mockResolvedValueOnce({ lines: ['x', 'y'] })
    const w = mountInside('frpc')
    await settle()
    const t = getTesting(w)
    expect(t.buf.lines.value.length).toBe(2)
    const calls = tailMock.mock.calls.length + incMock.mock.calls.length
    t.onClear()
    await nextTick()
    expect(t.buf.lines.value.length).toBe(0)
    expect(tailMock.mock.calls.length + incMock.mock.calls.length).toBe(calls)
  })
})

describe('LogViewer — AC-10 全屏 Modal 打开 / 关闭后缓冲不丢', () => {
  it('fullscreenOpen 切 true → 再切 false → lines 保留', async () => {
    tailMock.mockResolvedValueOnce({ lines: ['keep1', 'keep2'] })
    const w = mountInside('frpc')
    await settle()
    const t = getTesting(w)
    expect(t.fullscreenOpen.value).toBe(false)
    t.fullscreenOpen.value = true
    await nextTick()
    expect(t.fullscreenOpen.value).toBe(true)
    t.fullscreenOpen.value = false
    await nextTick()
    expect(t.buf.lines.value).toEqual(['keep1', 'keep2'])
  })
})

describe('LogViewer — AC-11 切 kind（BC-4）', () => {
  it('frpc → frps：缓冲清 + autoRefresh false + height 偏好保留', async () => {
    tailMock.mockResolvedValue({ lines: ['frpc-line'] })
    const w = mountInside('frpc')
    await settle()
    const t = getTesting(w)
    t.buf.setAutoRefresh(true)
    t.prefs.setHeight(800)
    expect(t.buf.autoRefresh.value).toBe(true)
    expect(t.prefs.height.value).toBe(800)

    tailMock.mockResolvedValue({ lines: ['frps-line'] })
    await w.setProps({ kind: 'frps' })
    await settle(5)

    expect(t.buf.autoRefresh.value).toBe(false)
    expect(t.prefs.height.value).toBe(800)
  })
})

describe('LogViewer — AC-13 暗 / 亮主题不同（A-2 / C-2 spike 验证）', () => {
  it('light vs dark：rootCssVars 至少 1 个 token 值不同', async () => {
    tailMock.mockResolvedValue({ lines: [] })
    const wLight = mountInside('frpc', 'light')
    await settle()
    const tLight = getTesting(wLight)
    const lightVars = { ...tLight.rootCssVars.value }

    const wDark = mountInside('frpc', 'dark')
    await settle()
    const tDark = getTesting(wDark)
    const darkVars = { ...tDark.rootCssVars.value }

    const differs =
      lightVars['--log-text'] !== darkVars['--log-text'] ||
      lightVars['--log-bg'] !== darkVars['--log-bg']
    expect(differs).toBe(true)
  })
})

describe('LogViewer — AC-16 首次 loadTail 失败 → 重试', () => {
  it('首次 reject → 文案 "加载日志失败" 可见；onRetry 后再次调 apiGetLogsTail', async () => {
    tailMock
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce({ lines: ['ok'] })
    const w = mountInside('frpc')
    await settle()
    const t = getTesting(w)
    expect(t.buf.firstLoadError.value).toBe('boom')
    expect(w.text()).toContain('加载日志失败')

    t.onRetry()
    await settle(3)
    expect(tailMock).toHaveBeenCalledTimes(2)
    expect(t.buf.firstLoadError.value).toBeNull()
  })
})

describe('LogViewer — ADV-A：XSS escape（先 escape 后 mark，NFR-7）', () => {
  it('日志含 <script>alert(1)</script>：DOM 内不存在真实 script 元素，textContent 含完整字面文本', async () => {
    tailMock.mockResolvedValueOnce({
      lines: ['attacker tried <script>alert(1)</script>'],
    })
    const w = mountInside('frpc')
    await settle()
    const t = getTesting(w)
    t.search.setQuery('<script>')
    await nextTick()

    const lineMsgEl = w.element.querySelector('.line-message') as HTMLElement
    expect(lineMsgEl).toBeTruthy()

    // 反向证伪 1：textContent 含完整字面文本（说明 escape 已发生 →
    // v-html 接收的是 &lt;script&gt; → 浏览器 decode 成 text node "<script>"）。
    expect(lineMsgEl.textContent).toContain('<script>alert(1)</script>')

    // 反向证伪 2：mark 标签存在
    const marks = lineMsgEl.querySelectorAll('mark.search-hit')
    expect(marks.length).toBeGreaterThanOrEqual(1)
    // mark 内 textContent 是 "<script>"（字面文本）
    expect((marks[0] as HTMLElement).textContent).toContain('<script>')
    // 但 mark 内不应有真实 script 子元素
    expect((marks[0] as HTMLElement).querySelectorAll('script').length).toBe(0)
  })
})

describe('LogViewer — AC-8 useLogPrefs 持久化', () => {
  it('setWrap(false) → localStorage 同步', async () => {
    const w = mountInside('frpc')
    await settle()
    const t = getTesting(w)
    t.prefs.setWrap(false)
    await nextTick()
    expect(window.localStorage.getItem('logViewer.wrap')).toBe('false')
  })
})

describe('LogViewer — AC-9 height 档位', () => {
  it('setHeight(800) → heightPx=800', async () => {
    const w = mountInside('frpc')
    await settle()
    const t = getTesting(w)
    t.prefs.setHeight(800)
    await nextTick()
    expect(t.prefs.heightPx.value).toBe(800)
  })
})

describe('LogViewer — onClearFilters', () => {
  it('清空筛选 → query="" + activeLevels=6 等级', async () => {
    tailMock.mockResolvedValueOnce({ lines: ['2025/01/15 10:23:45 [I] x'] })
    const w = mountInside('frpc')
    await settle()
    const t = getTesting(w)
    t.search.setQuery('zzz')
    t.filter.setActiveLevels([])
    await nextTick()
    t.onClearFilters()
    await nextTick()
    expect(t.search.query.value).toBe('')
    expect(t.filter.activeLevels.value.length).toBe(6)
  })
})

describe('LogViewer — AC-14：超长单行不溢出（pre-wrap 模式）', () => {
  it('单行 2000 字符 + wrap=true → DOM .log-line 不抛错且 wrap class 命中', async () => {
    const long = 'x'.repeat(2000)
    tailMock.mockResolvedValueOnce({ lines: [long] })
    const w = mountInside('frpc')
    await settle()
    expect(w.find('.log-line.wrap').exists()).toBe(true)
    const msgEl = w.find('.log-line.wrap .line-message')
    expect(msgEl.exists()).toBe(true)
    expect((msgEl.element as HTMLElement).textContent?.length).toBe(2000)
  })
})

describe('LogViewer — paused banner 文案（11 / BC-7）', () => {
  it('follow.paused = true → 提示条 "已暂停跟随" 显示', async () => {
    tailMock.mockResolvedValueOnce({ lines: ['a', 'b'] })
    const w = mountInside('frpc')
    await settle()
    const t = getTesting(w)
    // 模拟用户向上滚 → onScroll 让 paused = true
    // 直接修改 paused.value 简单触发 UI
    ;(t.follow as unknown as { paused: { value: boolean } }).paused.value = true
    await nextTick()
    expect(w.text()).toContain('已暂停跟随')
  })
})
