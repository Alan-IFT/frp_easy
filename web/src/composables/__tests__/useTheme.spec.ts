// T-066 / dark-theme-support · useTheme 状态层单测
//
// 测试策略（02 §3.1 / 03 §3 Q2 + C-1）：
//   - useTheme 是模块级单例：pref 初值在模块加载时由 readPref(localStorage) 一次性确定，
//     setPref 跨用例共享同一 ref → 测"不同 localStorage 预置态"必须用 vi.resetModules() +
//     动态 import 拿到全新模块实例（AC-5/6/7）。
//   - useOsTheme 须在 setup 上下文调用（onMounted/inject）→ 统一用 vi.mock('naive-ui')
//     提供受控的 useOsTheme（返回普通 ref），使 useTheme() 在非 setup 也安全调用，
//     并可把 OS 主题 mock 成 dark/light/null 测 auto 分支（AC-4/BC-5）。
//   - 断言全用可观察量：localStorage 值 / pref / activeTheme === darkTheme / isDark（insight L45）。
//
// 注：darkTheme 取真实 naive-ui 对象（importOriginal），故 activeTheme===darkTheme 引用相等可断言。

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { ref } from 'vue'

// 受控 osTheme：测试可改其 .value 模拟 OS 浅/暗/不支持。
const osThemeRef = ref<'light' | 'dark' | null>('light')

vi.mock('naive-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useOsTheme: () => osThemeRef,
  }
})

// 拿真实 darkTheme 对象用于引用相等断言。
async function realDarkTheme() {
  const naive = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return naive.darkTheme
}

// 动态 import 一份全新 useTheme 模块（重置模块单例 + 重读 localStorage）。
async function freshUseTheme() {
  vi.resetModules()
  return await import('../useTheme')
}

beforeEach(() => {
  localStorage.clear()
  osThemeRef.value = 'light' // 默认 OS 浅色，每用例显式覆盖
})

describe('useTheme — 默认与持久化（AC-1/2/3/5）', () => {
  it('AC-1：无持久化值时默认偏好 auto，OS 浅色 → 生效浅色（null），不回归', async () => {
    osThemeRef.value = 'light'
    const { useTheme, DEFAULT_PREF } = await freshUseTheme()
    const { pref, activeTheme, isDark } = useTheme()
    expect(DEFAULT_PREF).toBe('auto')
    expect(pref.value).toBe('auto')
    expect(activeTheme.value).toBeNull() // 浅色
    expect(isDark.value).toBe(false)
  })

  it('AC-2：setPref(dark) → 生效 darkTheme 且 localStorage 持久化为 dark', async () => {
    const dark = await realDarkTheme()
    const { useTheme, THEME_STORAGE_KEY } = await freshUseTheme()
    const { activeTheme, isDark, setPref, pref } = useTheme()
    setPref('dark')
    expect(pref.value).toBe('dark')
    expect(activeTheme.value).toBe(dark)
    expect(isDark.value).toBe(true)
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark')
  })

  it('AC-3：setPref(light) → 生效浅色（null），持久化 light，且不受 OS 暗色影响（BC-6）', async () => {
    osThemeRef.value = 'dark' // OS 是暗的
    const { useTheme, THEME_STORAGE_KEY } = await freshUseTheme()
    const { activeTheme, isDark, setPref } = useTheme()
    setPref('light')
    expect(activeTheme.value).toBeNull() // 用户显式 light 优先于 OS 暗
    expect(isDark.value).toBe(false)
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('light')
  })

  it('AC-5：localStorage 预置 dark（模拟重载）→ 新实例读回生效 darkTheme', async () => {
    localStorage.setItem('frpEasy.themePref', 'dark')
    const dark = await realDarkTheme()
    const { useTheme } = await freshUseTheme()
    const { pref, activeTheme } = useTheme()
    expect(pref.value).toBe('dark') // 持久化 round-trip
    expect(activeTheme.value).toBe(dark)
  })
})

describe('useTheme — auto 跟随系统（AC-4/BC-4/BC-5）', () => {
  it('AC-4：auto + OS 暗 → 生效 darkTheme', async () => {
    osThemeRef.value = 'dark'
    const dark = await realDarkTheme()
    const { useTheme } = await freshUseTheme()
    const { pref, activeTheme, isDark } = useTheme()
    expect(pref.value).toBe('auto')
    expect(activeTheme.value).toBe(dark)
    expect(isDark.value).toBe(true)
  })

  it('AC-4：auto + OS 浅 → 生效浅色（null）', async () => {
    osThemeRef.value = 'light'
    const { useTheme } = await freshUseTheme()
    const { activeTheme, isDark } = useTheme()
    expect(activeTheme.value).toBeNull()
    expect(isDark.value).toBe(false)
  })

  it('BC-4：auto 模式下 OS 主题运行时从浅切暗 → activeTheme 响应式跟随', async () => {
    osThemeRef.value = 'light'
    const dark = await realDarkTheme()
    const { useTheme } = await freshUseTheme()
    const { activeTheme } = useTheme()
    expect(activeTheme.value).toBeNull()
    osThemeRef.value = 'dark' // OS 切暗，无需重载/操作
    expect(activeTheme.value).toBe(dark)
  })

  it('BC-5：auto + useOsTheme 返回 null（环境不支持 matchMedia）→ 视为浅色不崩', async () => {
    osThemeRef.value = null
    const { useTheme } = await freshUseTheme()
    const { activeTheme, isDark } = useTheme()
    expect(activeTheme.value).toBeNull()
    expect(isDark.value).toBe(false)
  })
})

describe('useTheme — 边界降级（BC-1/BC-2/BC-3）', () => {
  it('BC-2：localStorage 预置非法值 purple → 降级默认 auto，不抛错', async () => {
    localStorage.setItem('frpEasy.themePref', 'purple')
    const { useTheme } = await freshUseTheme()
    const { pref } = useTheme()
    expect(pref.value).toBe('auto') // 非法值降级
  })

  it('BC-3：localStorage 无该 key → 默认 auto', async () => {
    const { useTheme } = await freshUseTheme()
    const { pref } = useTheme()
    expect(pref.value).toBe('auto')
  })

  it('BC-1：localStorage.setItem 抛错（quota/隐私模式）→ 仍可构造/切换/不抛错（内存降级）', async () => {
    // 探针 setItem 在 createSafeStorage 构造时抛 → 立即走内存
    const setSpy = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('QuotaExceededError')
    })
    const dark = await realDarkTheme()
    const { useTheme } = await freshUseTheme()
    const { pref, activeTheme, setPref } = useTheme()
    expect(pref.value).toBe('auto') // 默认（读不到任何持久化值）
    // 内存降级下切换仍生效（仅当次会话内，不抛错）
    expect(() => setPref('dark')).not.toThrow()
    expect(pref.value).toBe('dark')
    expect(activeTheme.value).toBe(dark)
    setSpy.mockRestore()
  })

  it('setPref 拒绝非法值（防御）：传 "xxx" 不改变当前偏好', async () => {
    const { useTheme } = await freshUseTheme()
    const { pref, setPref } = useTheme()
    setPref('dark')
    // @ts-expect-error 故意传非法值测防御
    setPref('xxx')
    expect(pref.value).toBe('dark') // 不被非法值覆盖
  })
})
