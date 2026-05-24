// T-036 / log-ui-ux-polish · 02 §3.6.5
// 跟随尾部状态机：autoFollow / paused 两轴；32 px 距底阈值；不自动反转 paused（BC-7）。

import { ref, nextTick, type Ref } from 'vue'

export interface ScrollPayload {
  scrollTop: number
  scrollHeight: number
  clientHeight: number
}

export interface UseFollowTailReturn {
  enabled: Ref<boolean>
  paused: Ref<boolean>
  /** E3：用户切换跟随开关 */
  toggle: (v: boolean) => void
  /** E4：用户点击 "已暂停跟随" 提示条 */
  resume: () => void
  /** E5：用户点击 ↓ 底部 按钮（不改 autoFollow / paused） */
  scrollToBottom: () => void
  /** E2：滚动事件 */
  onScroll: (p: ScrollPayload) => void
  /** E1：新数据到达；shouldStickToBottom 时滚到底 */
  onNewLines: () => void
  /** 绑定 DOM scroll 容器 */
  bindScrollEl: (el: HTMLElement | null) => void
}

export const STICK_THRESHOLD_PX = 32

export function useFollowTail(initial: Ref<boolean>): UseFollowTailReturn {
  const enabled = ref<boolean>(initial.value)
  const paused = ref<boolean>(false)
  let scrollEl: HTMLElement | null = null

  function bindScrollEl(el: HTMLElement | null) {
    scrollEl = el
  }

  function doScrollToBottom() {
    if (scrollEl) {
      // 浏览器会自然 clamp 到 (scrollHeight - clientHeight)，但为了在测试环境
      // （happy-dom 不会 clamp）也得到一致语义，这里显式 clamp。
      // AC-4：scrollTop 与 (scrollHeight - clientHeight) 误差 ≤ 1px。
      const target = Math.max(0, scrollEl.scrollHeight - scrollEl.clientHeight)
      scrollEl.scrollTop = target
    }
  }

  function scrollToBottom() {
    void nextTick(() => doScrollToBottom())
  }

  function toggle(v: boolean) {
    enabled.value = v
    paused.value = false
    if (v) {
      void nextTick(() => doScrollToBottom())
    }
  }

  function resume() {
    paused.value = false
    void nextTick(() => doScrollToBottom())
  }

  function onScroll(p: ScrollPayload) {
    const distFromBot = p.scrollHeight - p.scrollTop - p.clientHeight
    if (distFromBot > STICK_THRESHOLD_PX && enabled.value && !paused.value) {
      paused.value = true
    }
    // 注意：不在距底 ≤ 32 时自动反转 paused → false（BC-7 避免抖动）
  }

  function onNewLines() {
    if (enabled.value && !paused.value) {
      void nextTick(() => doScrollToBottom())
    }
  }

  return {
    enabled,
    paused,
    toggle,
    resume,
    scrollToBottom,
    onScroll,
    onNewLines,
    bindScrollEl,
  }
}
