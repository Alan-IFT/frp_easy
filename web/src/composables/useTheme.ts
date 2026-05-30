// T-066 / dark-theme-support · 02 §3.1
// 全局主题状态层（模块级单例 composable）。
//
// 单一真相源：持有主题偏好（'light' | 'dark' | 'auto'）+ 派生"当前生效主题对象"
// （null=浅色 / darkTheme=暗色）+ localStorage 持久化（BC-13 内存降级）+ OS 跟随。
//
// 为何模块级单例 composable 而非 Pinia store（02 §3.1）：
//   - 偏好是纯 UI 局部状态，无需 Pinia devtools / SSR / 跨页 hydration。
//   - 模块作用域的 pref ref + useTheme() 返回它 → App.vue 与 AppLayout 共享同一状态，
//     无需 provide/inject。
//   - useThemeVars/useOsTheme 是组合式 hook，在组件 setup 内调用天然合规。
//
// useOsTheme() 约束（02 §3.1 / R-2）：它内部用 onMounted/inject 注册 matchMedia
// 监听，必须在某组件 setup 内同步调用。设计令 App.vue 在 setup 顶层调 useTheme()
// 触发首次 useOsTheme()（osThemeRef 惰性建立）。dark/light/setPref 分支不读 OS，
// osThemeRef 惰性保持 null 安全 —— 即便测试未经 setup 调 setPref('dark') 也不抛错。

import { ref, computed, type Ref, type ComputedRef } from 'vue'
import { darkTheme, useOsTheme, type GlobalTheme } from 'naive-ui'

export type ThemePref = 'light' | 'dark' | 'auto'
export const THEME_STORAGE_KEY = 'frpEasy.themePref'
export const DEFAULT_PREF: ThemePref = 'auto' // AC-1 / BC-3：首次缺失默认跟随系统

const VALID_PREFS: readonly ThemePref[] = ['light', 'dark', 'auto']

export interface UseThemeReturn {
  pref: Ref<ThemePref>
  activeTheme: ComputedRef<GlobalTheme | null> // null=浅色 / darkTheme=暗色
  isDark: ComputedRef<boolean>
  setPref: (p: ThemePref) => void
}

/**
 * BC-13 单点封装（复刻 useLogPrefs.ts:41-84 范式，受控副本——见 02 §R-4，不抽公共
 * util 以免反向耦合 log/theme 两域并触动 useLogPrefs 既有测试）。
 * localStorage 在隐私模式 / Safari ITP / quota 满 / 无 window 时 throw 或缺失，
 * 任何失败 → 静默降级到内存 Map，不报错、不弹 message、UI 仍可切换。
 */
function createSafeStorage(): {
  get: (key: string) => string | null
  set: (key: string, value: string) => void
} {
  const memory = new Map<string, string>()
  let useMemory = false
  try {
    if (typeof window === 'undefined' || !window.localStorage) {
      useMemory = true
    } else {
      const probe = '__themePref_probe__'
      window.localStorage.setItem(probe, '1')
      window.localStorage.removeItem(probe)
    }
  } catch {
    useMemory = true
  }
  return {
    get(key) {
      if (useMemory) return memory.get(key) ?? null
      try {
        return window.localStorage.getItem(key)
      } catch {
        useMemory = true
        return memory.get(key) ?? null
      }
    },
    set(key, value) {
      if (useMemory) {
        memory.set(key, value)
        return
      }
      try {
        window.localStorage.setItem(key, value)
      } catch {
        useMemory = true
        memory.set(key, value)
      }
    },
  }
}

function isThemePref(v: string | null): v is ThemePref {
  return v !== null && (VALID_PREFS as readonly string[]).includes(v)
}

function readPref(storage: ReturnType<typeof createSafeStorage>): ThemePref {
  const raw = storage.get(THEME_STORAGE_KEY)
  // BC-2 非法/损坏值降级 / BC-3 缺失降级 → DEFAULT_PREF
  return isThemePref(raw) ? raw : DEFAULT_PREF
}

// ── 模块级单例状态（App.vue 与 AppLayout 共享同一引用）──
const storage = createSafeStorage()
const pref = ref<ThemePref>(readPref(storage))
let osThemeRef: ReturnType<typeof useOsTheme> | null = null

const activeTheme = computed<GlobalTheme | null>(() => {
  if (pref.value === 'dark') return darkTheme
  if (pref.value === 'light') return null
  // 'auto'：跟随 OS。useOsTheme 返回 'dark' | 'light' | null。
  // null（环境不支持 matchMedia / 未在 setup 内建立）→ 视为浅色（BC-5）。
  return osThemeRef?.value === 'dark' ? darkTheme : null
})

const isDark = computed(() => activeTheme.value === darkTheme)

function setPref(p: ThemePref): void {
  if (!isThemePref(p)) return
  pref.value = p
  storage.set(THEME_STORAGE_KEY, p) // write-through 持久化（复刻 useLogPrefs setter 范式）
}

/**
 * 返回模块单例主题状态。首次由 App.vue 在 setup 顶层调用，惰性建立 osThemeRef
 * （useOsTheme 须在 setup 上下文）。后续调用方（AppLayout 等）复用已建立的 osThemeRef。
 */
export function useTheme(): UseThemeReturn {
  if (!osThemeRef) {
    // 仅在组件 setup 内安全（onMounted/inject）。App.vue 保证首次调用在 setup。
    osThemeRef = useOsTheme()
  }
  return { pref, activeTheme, isDark, setPref }
}
