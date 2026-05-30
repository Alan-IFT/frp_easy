// T-067 / responsive-layout · 02 §3
// 模块级单例 composable：响应式视口宽度断点。
//
// 复刻 useTheme.ts 模块单例范式（App/AppLayout 共享同一引用，无需 provide/inject）。
// 用原生 window.matchMedia——零依赖、边界完全可控（不依赖 naive-ui useBreakpoint 的
// 固定分界点 640/1024，那些不对齐 768 且边界方向语义不透明）。
//
// 单一真相源：isNarrow（视口 < 768px 为 true）。AppLayout 据此驱动侧栏自动折叠默认态、
// 顶栏窄屏换行、内容区 padding。
//
// 监听不在组件 onUnmounted 清理：模块单例存活整个 app 生命周期（与 useTheme osThemeRef
// 同），initialized 守卫保证 matchMedia change 监听只注册一次，监听数恒为 1（NFR-4 无泄漏、
// 无重复；BC-5 抖动安全：listener 只更新 ref 值不递归）。

import { ref, type Ref } from 'vue'

// BC-1：< 768 折叠 / >= 768 展开。用 767.98px 上界让 (max-width:767.98px) 在 768 整数
// 边界为 false（即 768 判为"宽/展开"），消除整数 vs CSS px 边界歧义。
// 767.98 < 1280（e2e Desktop Chrome 视口）→ e2e 默认视口恒判展开，03-dashboard 零回归（BC-2）。
export const NARROW_MAX_WIDTH = 767.98
export const NARROW_QUERY = `(max-width: ${NARROW_MAX_WIDTH}px)`

// ── 模块级单例状态（App.vue / AppLayout.vue 共享同一引用）──
const isNarrow = ref<boolean>(false)
let initialized = false

function safeMatchMedia(): MediaQueryList | null {
  // BC-6：happy-dom 默认 / SSR / 老浏览器无 matchMedia → 返回 null，isNarrow 留 false
  // （退化为展开态，由用户手动控制，不抛错、不白屏）。
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return null
  try {
    return window.matchMedia(NARROW_QUERY)
  } catch {
    return null
  }
}

function init(): void {
  if (initialized) return
  initialized = true
  const mql = safeMatchMedia()
  if (!mql) return // BC-6 降级：保持 false
  isNarrow.value = mql.matches
  const onChange = (e: MediaQueryListEvent | MediaQueryList): void => {
    isNarrow.value = e.matches
  }
  // addEventListener('change') 是现代标准；老 Safari 仅有 addListener。两者都试。
  if (typeof mql.addEventListener === 'function') {
    mql.addEventListener('change', onChange as (e: MediaQueryListEvent) => void)
  } else if (
    typeof (mql as { addListener?: unknown }).addListener === 'function'
  ) {
    ;(mql as unknown as {
      addListener: (cb: (e: MediaQueryList) => void) => void
    }).addListener(onChange)
  }
}

export interface UseViewportReturn {
  /** 视口宽度 < 768px 时为 true（窄屏/移动端）。响应式。 */
  isNarrow: Ref<boolean>
}

/**
 * 返回模块单例视口断点状态。首次调用惰性建立 matchMedia 监听（幂等，initialized 守卫），
 * 后续调用方复用同一 isNarrow ref。AppLayout 是主要消费方。
 */
export function useViewport(): UseViewportReturn {
  init()
  return { isNarrow }
}
