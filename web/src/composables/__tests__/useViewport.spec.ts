// T-067 / responsive-layout · useViewport 单测
//
// 测试策略（02 §R-2 / 03 §3 C-1）：
//   - useViewport 是模块级单例：isNarrow 初值在首次 useViewport()→init() 时由 matchMedia
//     一次性确定，initialized 守卫使监听只注册一次 → 测"不同视口初值/降级态"必须用
//     vi.resetModules() + 动态 import 拿全新模块实例。
//   - happy-dom 的 matchMedia 行为不可控（默认基于 innerWidth、不响应宽度变化），故用
//     vi.stubGlobal('matchMedia', ...) 注入受控 MediaQueryList：matches 可控 + 暴露注册的
//     change 监听供测试手动触发"视口跨阈值变化"（与 useTheme.spec 受控 osThemeRef 同思路）。
//   - 断言全用可观察量：isNarrow.value 布尔 + listener 注册次数 + 常量值（insight L45）。

import { describe, it, expect, afterEach, vi } from 'vitest'

// ── 受控 MediaQueryList 工厂 ──
// 记录注册的 change 回调；fire(matches) 模拟视口跨阈值，同步更新 matches 并触发回调。
interface FakeMql {
  matches: boolean
  listeners: Array<(e: { matches: boolean }) => void>
  addEventListener: (type: string, cb: (e: { matches: boolean }) => void) => void
  removeEventListener: (type: string, cb: (e: { matches: boolean }) => void) => void
  fire: (matches: boolean) => void
}

function makeFakeMql(initialMatches: boolean): FakeMql {
  const mql: FakeMql = {
    matches: initialMatches,
    listeners: [],
    addEventListener(type, cb) {
      if (type === 'change') mql.listeners.push(cb)
    },
    removeEventListener(type, cb) {
      if (type === 'change') mql.listeners = mql.listeners.filter((l) => l !== cb)
    },
    fire(matches) {
      mql.matches = matches
      for (const l of mql.listeners) l({ matches })
    },
  }
  return mql
}

// 装一个受控 matchMedia，返回单一 FakeMql（NARROW_QUERY 只会被查询一次）。
function stubMatchMedia(initialMatches: boolean): FakeMql {
  const mql = makeFakeMql(initialMatches)
  vi.stubGlobal(
    'matchMedia',
    vi.fn(() => mql),
  )
  return mql
}

// 动态 import 一份全新 useViewport 模块（重置模块单例 isNarrow + initialized）。
async function freshUseViewport() {
  vi.resetModules()
  return await import('../useViewport')
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('useViewport — 断点初值（FR-1 / AC-2）', () => {
  it('窄屏（matchMedia.matches=true）→ isNarrow 初值 true（侧栏将折叠）', async () => {
    stubMatchMedia(true)
    const { useViewport } = await freshUseViewport()
    const { isNarrow } = useViewport()
    expect(isNarrow.value).toBe(true)
  })

  it('宽屏（matchMedia.matches=false）→ isNarrow 初值 false（侧栏将展开，桌面不回归）', async () => {
    stubMatchMedia(false)
    const { useViewport } = await freshUseViewport()
    const { isNarrow } = useViewport()
    expect(isNarrow.value).toBe(false)
  })
})

describe('useViewport — 视口跨阈值响应式（FR-3 / AC-2）', () => {
  it('宽→窄：matchMedia change matches=true → isNarrow 变 true', async () => {
    const mql = stubMatchMedia(false)
    const { useViewport } = await freshUseViewport()
    const { isNarrow } = useViewport()
    expect(isNarrow.value).toBe(false)
    mql.fire(true) // 模拟视口缩到 < 768
    expect(isNarrow.value).toBe(true)
  })

  it('窄→宽：matchMedia change matches=false → isNarrow 变 false', async () => {
    const mql = stubMatchMedia(true)
    const { useViewport } = await freshUseViewport()
    const { isNarrow } = useViewport()
    expect(isNarrow.value).toBe(true)
    mql.fire(false) // 模拟视口放大到 >= 768
    expect(isNarrow.value).toBe(false)
  })
})

describe('useViewport — 单例 + 监听守卫（NFR-4 / BC-5）', () => {
  it('同一模块内多次 useViewport() 复用同一 isNarrow 引用（模块单例）', async () => {
    stubMatchMedia(false)
    const { useViewport } = await freshUseViewport()
    const a = useViewport().isNarrow
    const b = useViewport().isNarrow
    expect(a).toBe(b) // 同一 ref 引用
  })

  it('多次 useViewport() 不重复注册 change 监听（initialized 守卫，监听数恒为 1）', async () => {
    const mql = stubMatchMedia(false)
    const { useViewport } = await freshUseViewport()
    useViewport()
    useViewport()
    useViewport()
    // init 幂等：matchMedia change 监听只注册一次（NFR-4 无泄漏）
    expect(mql.listeners.length).toBe(1)
  })
})

describe('useViewport — 降级安全（BC-6）', () => {
  it('环境无 matchMedia → isNarrow 留 false 不抛错（退化为展开态）', async () => {
    vi.stubGlobal('matchMedia', undefined)
    const { useViewport } = await freshUseViewport()
    let result: { isNarrow: { value: boolean } } | null = null
    expect(() => {
      result = useViewport()
    }).not.toThrow()
    expect(result!.isNarrow.value).toBe(false)
  })

  it('matchMedia 抛错 → 捕获后 isNarrow 留 false 不崩', async () => {
    vi.stubGlobal(
      'matchMedia',
      vi.fn(() => {
        throw new Error('matchMedia not supported')
      }),
    )
    const { useViewport } = await freshUseViewport()
    let result: { isNarrow: { value: boolean } } | null = null
    expect(() => {
      result = useViewport()
    }).not.toThrow()
    expect(result!.isNarrow.value).toBe(false)
  })
})

describe('useViewport — 常量边界（BC-1 / BC-2）', () => {
  it('NARROW_MAX_WIDTH < 1280（e2e Desktop Chrome 视口），确保 e2e 默认视口判展开零回归', async () => {
    const { NARROW_MAX_WIDTH, NARROW_QUERY } = await freshUseViewport()
    expect(NARROW_MAX_WIDTH).toBeLessThan(1280)
    // 反向证伪 BC-2：若阈值 >= 1280，e2e 1280 视口会被判窄屏自动折叠隐藏菜单 → 03-dashboard FAIL
    expect(NARROW_QUERY).toBe(`(max-width: ${NARROW_MAX_WIDTH}px)`)
  })
})
