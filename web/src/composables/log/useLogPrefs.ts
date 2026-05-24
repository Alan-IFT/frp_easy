// T-036 / log-ui-ux-polish · 02 §3.6.6
// localStorage 持久化封装；BC-13：localStorage 不可用时静默降级到内存 Map。
// 唯一与 localStorage 对话的层；其他 composable 把 ref 当普通 reactive 消费。

import { ref, computed, type Ref, type ComputedRef } from 'vue'

export type LogHeight = 300 | 500 | 800
export const LOG_HEIGHTS: LogHeight[] = [300, 500, 800]
export const FONT_SIZE_MIN = 12
export const FONT_SIZE_MAX = 16
export const FONT_SIZE_DEFAULT = 13

const STORAGE_KEYS = {
  wrap: 'logViewer.wrap',
  height: 'logViewer.height',
  fontSize: 'logViewer.fontSize',
  followTail: 'logViewer.followTail',
  caseSensitive: 'logViewer.caseSensitive',
} as const

export interface UseLogPrefsReturn {
  wrap: Ref<boolean>
  height: Ref<LogHeight>
  heightPx: ComputedRef<number>
  fontSize: Ref<number>
  fontSizePx: ComputedRef<string>
  followTail: Ref<boolean>
  caseSensitive: Ref<boolean>
  setWrap: (v: boolean) => void
  setHeight: (v: LogHeight) => void
  setFontSize: (v: number) => void
  setFollowTail: (v: boolean) => void
  setCaseSensitive: (v: boolean) => void
  flush: () => void
}

/**
 * BC-13 单点封装：localStorage 在隐私模式 / Safari ITP 拦截 / quota 满时 throw。
 * 任何失败 → 静默降级到内存 Map，不报错、不弹 message、UI 仍可切换。
 */
function createSafeStorage(): {
  get: (key: string) => string | null
  set: (key: string, value: string) => void
} {
  const memory = new Map<string, string>()
  let useMemory = false
  // 启动时探测一次；浏览器无 localStorage（SSR / sandbox）走内存
  try {
    if (typeof window === 'undefined' || !window.localStorage) {
      useMemory = true
    } else {
      // 隐私模式可能让 setItem 立即 throw
      const probe = '__logPrefs_probe__'
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
        // quota throw / 其他失败：切到内存继续工作
        useMemory = true
        memory.set(key, value)
      }
    },
  }
}

function readBool(
  storage: ReturnType<typeof createSafeStorage>,
  key: string,
  dflt: boolean,
): boolean {
  const raw = storage.get(key)
  if (raw === null) return dflt
  return raw === 'true'
}

function readHeight(
  storage: ReturnType<typeof createSafeStorage>,
  dflt: LogHeight,
): LogHeight {
  const raw = storage.get(STORAGE_KEYS.height)
  if (raw === null) return dflt
  const n = Number.parseInt(raw, 10)
  if (n === 300 || n === 500 || n === 800) return n
  return dflt
}

function readFontSize(
  storage: ReturnType<typeof createSafeStorage>,
  dflt: number,
): number {
  const raw = storage.get(STORAGE_KEYS.fontSize)
  if (raw === null) return dflt
  const n = Number.parseInt(raw, 10)
  if (Number.isFinite(n) && n >= FONT_SIZE_MIN && n <= FONT_SIZE_MAX) return n
  return dflt
}

export function useLogPrefs(): UseLogPrefsReturn {
  const storage = createSafeStorage()

  const wrap = ref<boolean>(readBool(storage, STORAGE_KEYS.wrap, true))
  const height = ref<LogHeight>(readHeight(storage, 500))
  const fontSize = ref<number>(readFontSize(storage, FONT_SIZE_DEFAULT))
  const followTail = ref<boolean>(
    readBool(storage, STORAGE_KEYS.followTail, true),
  )
  const caseSensitive = ref<boolean>(
    readBool(storage, STORAGE_KEYS.caseSensitive, false),
  )

  const heightPx = computed(() => height.value)
  const fontSizePx = computed(() => `${fontSize.value}px`)

  function setWrap(v: boolean) {
    wrap.value = v
    storage.set(STORAGE_KEYS.wrap, v ? 'true' : 'false')
  }

  function setHeight(v: LogHeight) {
    height.value = v
    storage.set(STORAGE_KEYS.height, String(v))
  }

  function setFontSize(v: number) {
    if (!Number.isFinite(v)) return
    const clamped = Math.max(FONT_SIZE_MIN, Math.min(FONT_SIZE_MAX, Math.round(v)))
    fontSize.value = clamped
    storage.set(STORAGE_KEYS.fontSize, String(clamped))
  }

  function setFollowTail(v: boolean) {
    followTail.value = v
    storage.set(STORAGE_KEYS.followTail, v ? 'true' : 'false')
  }

  function setCaseSensitive(v: boolean) {
    caseSensitive.value = v
    storage.set(STORAGE_KEYS.caseSensitive, v ? 'true' : 'false')
  }

  function flush() {
    // 每个 setter 已 write-through；保留 hook 以便未来转 debounced 写
    storage.set(STORAGE_KEYS.wrap, wrap.value ? 'true' : 'false')
    storage.set(STORAGE_KEYS.height, String(height.value))
    storage.set(STORAGE_KEYS.fontSize, String(fontSize.value))
    storage.set(STORAGE_KEYS.followTail, followTail.value ? 'true' : 'false')
    storage.set(
      STORAGE_KEYS.caseSensitive,
      caseSensitive.value ? 'true' : 'false',
    )
  }

  return {
    wrap,
    height,
    heightPx,
    fontSize,
    fontSizePx,
    followTail,
    caseSensitive,
    setWrap,
    setHeight,
    setFontSize,
    setFollowTail,
    setCaseSensitive,
    flush,
  }
}
