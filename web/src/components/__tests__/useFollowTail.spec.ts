import { describe, it, expect } from 'vitest'
import { ref, nextTick } from 'vue'
import {
  useFollowTail,
  STICK_THRESHOLD_PX,
} from '../../composables/log/useFollowTail'

// T-036 · AC-4 / AC-5 / BC-7：跟随尾部状态机转移 + 32 px 阈值。

function makeScrollEl(scrollHeight: number, clientHeight: number) {
  const el = document.createElement('div')
  Object.defineProperty(el, 'scrollHeight', {
    configurable: true,
    get: () => scrollHeight,
  })
  Object.defineProperty(el, 'clientHeight', {
    configurable: true,
    get: () => clientHeight,
  })
  let st = 0
  Object.defineProperty(el, 'scrollTop', {
    configurable: true,
    get: () => st,
    set: (v: number) => {
      st = v
    },
  })
  return el
}

describe('useFollowTail — AC-4：onNewLines 把 scrollTop 拉到 scrollHeight-clientHeight', () => {
  it('enabled=true / paused=false → onNewLines 后 scrollTop = scrollHeight - clientHeight', async () => {
    const el = makeScrollEl(1000, 200)
    el.scrollTop = 0
    const initial = ref(true)
    const f = useFollowTail(initial)
    f.bindScrollEl(el)
    f.onNewLines()
    await nextTick()
    // 实现里直接 scrollTop = scrollHeight = 1000，但 DOM 浏览器自然会夹到
    // (scrollHeight - clientHeight)；这里 happy-dom 没有 clamp，所以是 1000。
    // AC-4 验证语义是"贴底"，断言距底 ≤ 1px。
    const dist = el.scrollHeight - el.scrollTop - el.clientHeight
    expect(Math.abs(dist)).toBeLessThanOrEqual(1)
  })

  it('enabled=false → onNewLines 不滚', async () => {
    const el = makeScrollEl(1000, 200)
    el.scrollTop = 0
    const initial = ref(false)
    const f = useFollowTail(initial)
    f.bindScrollEl(el)
    f.onNewLines()
    await nextTick()
    expect(el.scrollTop).toBe(0)
  })
})

describe('useFollowTail — AC-5 / BC-7：用户上滚 → paused = true（32 px 阈值）', () => {
  it('距底 > 32 px → paused 切 true', () => {
    const initial = ref(true)
    const f = useFollowTail(initial)
    // scrollHeight=1000, clientHeight=200, scrollTop=700 → 距底=100 > 32
    f.onScroll({ scrollTop: 700, scrollHeight: 1000, clientHeight: 200 })
    expect(f.paused.value).toBe(true)
  })

  it('距底 = 32 px（边界）→ 不 paused', () => {
    const initial = ref(true)
    const f = useFollowTail(initial)
    // 距底 = 1000 - 768 - 200 = 32
    f.onScroll({ scrollTop: 768, scrollHeight: 1000, clientHeight: 200 })
    expect(f.paused.value).toBe(false)
  })

  it('距底 > 32 但 enabled=false → 不切 paused', () => {
    const initial = ref(false)
    const f = useFollowTail(initial)
    f.onScroll({ scrollTop: 0, scrollHeight: 1000, clientHeight: 200 })
    expect(f.paused.value).toBe(false)
  })

  it('paused=true 后再回底（距底=0）→ 不自动反转（BC-7 避免抖动）', () => {
    const initial = ref(true)
    const f = useFollowTail(initial)
    f.onScroll({ scrollTop: 0, scrollHeight: 1000, clientHeight: 200 })
    expect(f.paused.value).toBe(true)
    f.onScroll({ scrollTop: 800, scrollHeight: 1000, clientHeight: 200 })
    expect(f.paused.value).toBe(true)
  })
})

describe('useFollowTail — onNewLines 在 paused 时不滚', () => {
  it('paused=true → onNewLines 不动 scrollTop', async () => {
    const el = makeScrollEl(1000, 200)
    el.scrollTop = 100
    const initial = ref(true)
    const f = useFollowTail(initial)
    f.bindScrollEl(el)
    // 触发 paused
    f.onScroll({ scrollTop: 0, scrollHeight: 1000, clientHeight: 200 })
    expect(f.paused.value).toBe(true)
    f.onNewLines()
    await nextTick()
    expect(el.scrollTop).toBe(100)
  })
})

describe('useFollowTail — E4 resume', () => {
  it('resume → paused=false + scrollToBottom', async () => {
    const el = makeScrollEl(1000, 200)
    el.scrollTop = 100
    const initial = ref(true)
    const f = useFollowTail(initial)
    f.bindScrollEl(el)
    f.onScroll({ scrollTop: 0, scrollHeight: 1000, clientHeight: 200 })
    expect(f.paused.value).toBe(true)
    f.resume()
    await nextTick()
    expect(f.paused.value).toBe(false)
    expect(el.scrollTop).toBeGreaterThan(100)
  })
})

describe('useFollowTail — E3 toggle', () => {
  it('toggle(false) → enabled=false + paused=false', () => {
    const initial = ref(true)
    const f = useFollowTail(initial)
    f.onScroll({ scrollTop: 0, scrollHeight: 1000, clientHeight: 200 })
    expect(f.paused.value).toBe(true)
    f.toggle(false)
    expect(f.enabled.value).toBe(false)
    expect(f.paused.value).toBe(false)
  })

  it('toggle(true) 从 false → enabled=true + scrollToBottom', async () => {
    const el = makeScrollEl(1000, 200)
    el.scrollTop = 0
    const initial = ref(false)
    const f = useFollowTail(initial)
    f.bindScrollEl(el)
    f.toggle(true)
    await nextTick()
    expect(f.enabled.value).toBe(true)
    expect(el.scrollTop).toBeGreaterThan(0)
  })
})

describe('useFollowTail — E5 scrollToBottom 不改状态', () => {
  it('scrollToBottom → 不改 enabled / paused', async () => {
    const el = makeScrollEl(1000, 200)
    el.scrollTop = 0
    const initial = ref(true)
    const f = useFollowTail(initial)
    f.bindScrollEl(el)
    f.onScroll({ scrollTop: 0, scrollHeight: 1000, clientHeight: 200 })
    const wasPaused = f.paused.value
    const wasEnabled = f.enabled.value
    f.scrollToBottom()
    await nextTick()
    expect(f.paused.value).toBe(wasPaused)
    expect(f.enabled.value).toBe(wasEnabled)
    expect(el.scrollTop).toBeGreaterThan(0)
  })
})

describe('useFollowTail — STICK_THRESHOLD_PX 常量', () => {
  it('STICK_THRESHOLD_PX = 32（D-3 决策矩阵）', () => {
    expect(STICK_THRESHOLD_PX).toBe(32)
  })
})
