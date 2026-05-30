/**
 * T-064 / menu-icons-and-a11y · Stage 6 QA Tester 独立对抗测试
 *
 * 这是 QA 写的反向证伪 reproducer，**不复用 dev 的 spec**（独立从 01 的 AC 出发，
 * 自带挂载 + 自构造预期会失败的判据，验证实现是否抗住）。
 *
 * - QA-ADV-1（AC-1/AC-3 核心缺陷反向证伪）：折叠态仅图标时两个原本撞车的菜单项
 *   （"服务端配置" / "设置"）必须可由可视字形 **与** 无障碍名同时区分。
 *   假设："dev 可能只换了可视字形但漏改 aria-label（或反之），导致 AT 用户仍无法区分
 *   折叠态两项" → 同时断言字形不同 + aria-label 不同 + 历史撞车字形 ⚙ 全菜单只出现 1 次。
 * - QA-ADV-2（AC-4/AC-5 聚焦落点反向证伪）：tabindex 与 role 必须在**同一个真实可滚动
 *   元素**（overflow-y:auto 的 .log-list-scroll）上，否则键盘焦点会落在无标签的包裹层、
 *   或可滚区域无 ARIA 身份。假设："dev 可能把 tabindex 加在外层 .log-list-root 而非真正
 *   overflow 的 .log-list-scroll，导致聚焦元素不可滚 / role 与 tabindex 分离" → 断言两属性
 *   同元素 + 该元素带 overflow-y 滚动语义 class。
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h, nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { NConfigProvider, NMessageProvider } from 'naive-ui'

vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/dashboard' }),
  useRouter: () => ({ push: vi.fn() }),
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

import AppLayout from '../AppLayout.vue'
import LogList from '../log/LogList.vue'
import { parseLogLine } from '../../composables/log/parseLogLine'
import type { VisibleLine } from '../../composables/log/useLogSearch'

async function settle(n = 6): Promise<void> {
  for (let i = 0; i < n; i++) await nextTick()
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('T-064 QA-ADV-1 — 折叠态撞车两项必须字形+无障碍名双重可区分', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('"服务端配置"与"设置"字形不同 AND aria-label 不同 AND 齿轮 ⚙ 全菜单只出现一次', async () => {
    const Holder = defineComponent({
      setup() {
        return () =>
          h(NConfigProvider, null, {
            default: () => h(NMessageProvider, null, { default: () => h(AppLayout) }),
          })
      },
    })
    const w = mount(Holder, {
      attachTo: document.body,
      global: { stubs: { 'router-view': true, RouterView: true } },
    })
    await settle()

    const icons = w.findAll('span.n-icon').filter((s) => s.attributes('aria-label') !== undefined)
    const byName = (n: string) => icons.find((s) => s.attributes('aria-label') === n)
    const server = byName('服务端配置')
    const settings = byName('设置')
    expect(server, '应渲染"服务端配置"图标').toBeTruthy()
    expect(settings, '应渲染"设置"图标').toBeTruthy()

    // 预期失败假设：若 dev 只换字形漏改 label（或反之），下面任一断言会捕获。
    expect(server!.text().trim()).not.toBe(settings!.text().trim()) // 可视字形可区分
    expect(server!.attributes('aria-label')).not.toBe(settings!.attributes('aria-label')) // AT 可区分

    // 历史撞车根因反向证伪：齿轮 ⚙ 在整组顶层图标里只能出现一次（旧版出现两次）
    const gearCount = icons.filter((s) => s.text().trim() === '⚙').length
    expect(gearCount, '⚙ 在 7 个菜单图标中应恰好出现一次').toBe(1)
  })
})

describe('T-064 QA-ADV-2 — tabindex 与 role 必须在同一真实可滚动元素上', () => {
  function baseProps(overrides: Record<string, unknown> = {}) {
    const lines: VisibleLine[] = [
      { lineNumber: 1, parsed: parseLogLine('2026-05-31 12:00:00 INFO line'), searchHits: [] },
    ]
    return {
      visibleLines: lines,
      bufferEmpty: false,
      noMatchHint: '无匹配',
      wrap: false,
      heightPx: 500,
      fontSizePx: '13px',
      loading: false,
      firstLoadError: null,
      followTail: true,
      paused: false,
      ...overrides,
    }
  }

  it('可聚焦元素 = 带 overflow-y 滚动 class 的 .log-list-scroll，且 role 与 tabindex 同元素', () => {
    const w = mount(LogList, { props: baseProps() })

    // 真实可滚动容器（CSS overflow-y:auto 落在 .log-list-scroll）
    const scroll = w.find('.log-list-scroll')
    expect(scroll.exists()).toBe(true)

    // 预期失败假设：若 tabindex 被加到外层 .log-list-root（非 overflow 元素），
    // 则聚焦后键盘方向键无法滚动 → 下面断言要求二者同在 .log-list-scroll。
    expect(scroll.attributes('tabindex')).toBe('0')
    expect(scroll.attributes('role')).toBeTruthy()
    expect(scroll.attributes('aria-label')!.trim().length).toBeGreaterThan(0)

    // 外层 root 不应抢走 tabindex（否则焦点落在不可滚元素）
    const root = w.find('.log-list-root')
    expect(root.exists()).toBe(true)
    expect(root.attributes('tabindex')).toBeUndefined()
  })
})
