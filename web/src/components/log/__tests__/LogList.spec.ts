// T-064 menu-icons-and-a11y · IS-3 + AC-4/AC-5/AC-6
// LogList.vue 滚动容器键盘可访问性：
//   - IS-3：div.log-list-scroll 加 tabindex="0"（纯键盘用户可 Tab 聚焦后方向键滚动）
//     + role="log" + aria-label="日志输出"（屏幕阅读器识别为日志区域）
//   - 范式对齐同文件 paused-banner（:30-41 已有 role/tabindex/键盘支持）
//
// 关键模式（insight L45）：
//   - LogList 是纯展示组件，仅 props，无 store/router/message 依赖 → 直接 mount
//   - 断言全用 DOM 属性查询（find('.log-list-scroll').attributes()），零 naive-ui 组件名
//   - AC-6：错误态/加载态分支渲染 .log-empty 而非滚动容器 → 断言滚动容器不存在

import { describe, it, expect, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import LogList from '../LogList.vue'
import { parseLogLine } from '../../../composables/log/parseLogLine'
import type { VisibleLine } from '../../../composables/log/useLogSearch'

// 列表/空态/无命中分支（v-else：非 firstLoadError、非 loading）的最小 props
function baseProps(overrides: Record<string, unknown> = {}) {
  return {
    visibleLines: [] as VisibleLine[],
    bufferEmpty: true,
    noMatchHint: '无匹配结果',
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

function mountList(overrides: Record<string, unknown> = {}) {
  return mount(LogList, { props: baseProps(overrides) })
}

afterEach(() => {
  document.body.innerHTML = ''
})

describe('LogList.vue — 滚动容器键盘可聚焦 + ARIA（T-064 IS-3）', () => {
  it('列表分支：滚动容器带 tabindex="0"（AC-4 纯键盘可聚焦滚动）', () => {
    const w = mountList()
    const scroll = w.find('.log-list-scroll')
    expect(scroll.exists()).toBe(true)
    // 反向证伪：缺失或非 "0" 则 FAIL
    expect(scroll.attributes('tabindex')).toBe('0')
  })

  it('列表分支：滚动容器带 role（log 或 region）+ 非空 aria-label（AC-5）', () => {
    const w = mountList()
    const scroll = w.find('.log-list-scroll')
    const role = scroll.attributes('role')
    expect(role).toBeTruthy()
    expect(['log', 'region']).toContain(role)
    const ariaLabel = scroll.attributes('aria-label')
    expect(ariaLabel).toBeTruthy()
    expect(ariaLabel!.trim().length).toBeGreaterThan(0)
  })

  it('有日志行时滚动容器仍带 tabindex + role（用真 parseLogLine 构造合法行）', () => {
    const lines: VisibleLine[] = [
      {
        lineNumber: 1,
        parsed: parseLogLine('2026-05-31 12:00:00 INFO hello world'),
        searchHits: [],
      },
    ]
    const w = mountList({ visibleLines: lines, bufferEmpty: false })
    const scroll = w.find('.log-list-scroll')
    expect(scroll.exists()).toBe(true)
    expect(scroll.attributes('tabindex')).toBe('0')
    expect(scroll.attributes('role')).toBeTruthy()
  })
})

describe('LogList.vue — 状态分支保真（T-064 AC-6 未破坏既有三态）', () => {
  it('首次加载错误态：渲染错误区而非滚动容器（滚动容器不存在）', () => {
    const w = mountList({ firstLoadError: '连接被拒绝' })
    expect(w.find('.log-list-scroll').exists()).toBe(false)
    expect(w.text()).toContain('加载日志失败')
    expect(w.text()).toContain('连接被拒绝')
  })

  it('加载中态：渲染加载区而非滚动容器（滚动容器不存在）', () => {
    const w = mountList({ loading: true })
    expect(w.find('.log-list-scroll').exists()).toBe(false)
    expect(w.text()).toContain('正在加载日志')
  })

  it('空态：滚动容器存在且含"暂无日志输出"（IS-3 作用于内容分支）', () => {
    const w = mountList({ visibleLines: [], bufferEmpty: true })
    expect(w.find('.log-list-scroll').exists()).toBe(true)
    expect(w.text()).toContain('暂无日志输出')
  })
})
