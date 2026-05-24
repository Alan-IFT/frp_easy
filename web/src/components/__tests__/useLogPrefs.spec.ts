import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { useLogPrefs } from '../../composables/log/useLogPrefs'

// T-036 · AC-8 / BC-13 / NFR-9：localStorage 持久化 + 不可用时静默降级到内存。

describe('useLogPrefs — 默认值', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  it('未持久化时默认 wrap=true / height=500 / fontSize=13 / followTail=true / caseSensitive=false', () => {
    const p = useLogPrefs()
    expect(p.wrap.value).toBe(true)
    expect(p.height.value).toBe(500)
    expect(p.heightPx.value).toBe(500)
    expect(p.fontSize.value).toBe(13)
    expect(p.fontSizePx.value).toBe('13px')
    expect(p.followTail.value).toBe(true)
    expect(p.caseSensitive.value).toBe(false)
  })
})

describe('useLogPrefs — localStorage 读写（AC-8）', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  it('setWrap(false) 后 localStorage 同步', () => {
    const p = useLogPrefs()
    p.setWrap(false)
    expect(p.wrap.value).toBe(false)
    expect(window.localStorage.getItem('logViewer.wrap')).toBe('false')
  })

  it('setHeight(800) 写入并读回', () => {
    const p = useLogPrefs()
    p.setHeight(800)
    expect(p.height.value).toBe(800)
    expect(window.localStorage.getItem('logViewer.height')).toBe('800')

    // 重新实例化（模拟刷新）应读到 800
    const p2 = useLogPrefs()
    expect(p2.height.value).toBe(800)
  })

  it('setFontSize(15) 在 [12, 16] 内', () => {
    const p = useLogPrefs()
    p.setFontSize(15)
    expect(p.fontSize.value).toBe(15)
    expect(p.fontSizePx.value).toBe('15px')
  })

  it('setFontSize 超出 [12,16] 被夹到边界', () => {
    const p = useLogPrefs()
    p.setFontSize(20)
    expect(p.fontSize.value).toBe(16)
    p.setFontSize(2)
    expect(p.fontSize.value).toBe(12)
  })

  it('setFontSize(NaN) 不动', () => {
    const p = useLogPrefs()
    p.setFontSize(Number.NaN)
    expect(p.fontSize.value).toBe(13)
  })

  it('setFollowTail / setCaseSensitive 各自写入', () => {
    const p = useLogPrefs()
    p.setFollowTail(false)
    p.setCaseSensitive(true)
    expect(window.localStorage.getItem('logViewer.followTail')).toBe('false')
    expect(window.localStorage.getItem('logViewer.caseSensitive')).toBe('true')
  })

  it('坏值（height 非 300/500/800）回退到默认 500', () => {
    window.localStorage.setItem('logViewer.height', '12345')
    const p = useLogPrefs()
    expect(p.height.value).toBe(500)
  })

  it('坏值（fontSize 非数字）回退默认 13', () => {
    window.localStorage.setItem('logViewer.fontSize', 'abc')
    const p = useLogPrefs()
    expect(p.fontSize.value).toBe(13)
  })
})

describe('useLogPrefs — BC-13 / ADV-B：localStorage 不可用降级（不崩 + 不弹 message）', () => {
  let originalSetItem: typeof window.localStorage.setItem
  beforeEach(() => {
    window.localStorage.clear()
    originalSetItem = window.localStorage.setItem.bind(window.localStorage)
  })
  afterEach(() => {
    // 还原 prototype 上的方法
    Object.defineProperty(window.localStorage, 'setItem', {
      configurable: true,
      writable: true,
      value: originalSetItem,
    })
  })

  it('setItem 始终 throw（quota） → setter 不崩 + value 仍生效（内存）', () => {
    Object.defineProperty(window.localStorage, 'setItem', {
      configurable: true,
      writable: true,
      value: vi.fn(() => {
        throw new Error('QuotaExceededError')
      }),
    })
    const p = useLogPrefs()
    expect(() => p.setHeight(800)).not.toThrow()
    expect(p.height.value).toBe(800)
    expect(() => p.setWrap(false)).not.toThrow()
    expect(p.wrap.value).toBe(false)
  })

  it('flush() 在 quota throw 下也不崩', () => {
    Object.defineProperty(window.localStorage, 'setItem', {
      configurable: true,
      writable: true,
      value: vi.fn(() => {
        throw new Error('QuotaExceededError')
      }),
    })
    const p = useLogPrefs()
    expect(() => p.flush()).not.toThrow()
  })
})

describe('useLogPrefs — flush()', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  it('flush 把当前所有值落到 localStorage', () => {
    const p = useLogPrefs()
    p.setWrap(false)
    p.setHeight(800)
    p.setFontSize(14)
    p.setFollowTail(false)
    p.setCaseSensitive(true)
    p.flush()
    expect(window.localStorage.getItem('logViewer.wrap')).toBe('false')
    expect(window.localStorage.getItem('logViewer.height')).toBe('800')
    expect(window.localStorage.getItem('logViewer.fontSize')).toBe('14')
    expect(window.localStorage.getItem('logViewer.followTail')).toBe('false')
    expect(window.localStorage.getItem('logViewer.caseSensitive')).toBe('true')
  })
})
