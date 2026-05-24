<template>
  <n-modal
    :show="show"
    preset="card"
    :title="`${kind} 日志（全屏）`"
    class="fullscreen-log-modal"
    @update:show="(v: boolean) => emit('update:show', v)"
  >
    <log-list
      :visible-lines="visibleLines"
      :buffer-empty="bufferEmpty"
      :no-match-hint="noMatchHint"
      :wrap="wrap"
      :height-px="heightPx"
      :font-size-px="fontSizePx"
      :loading="loading"
      :first-load-error="firstLoadError"
      :follow-tail="followTail"
      :paused="paused"
      @scroll="(p) => emit('scroll', p)"
      @retry="emit('retry')"
      @resume-follow="emit('resumeFollow')"
      @clear-filters="emit('clearFilters')"
      @scroll-el-ready="(el) => emit('scrollElReady', el)"
    />
  </n-modal>
</template>

<script setup lang="ts">
// T-036 / log-ui-ux-polish · 02 §3.5
// 全屏 Modal 包装一份 LogList。95vw / 90vh 通过 scoped CSS `:deep(.n-card)` 设定（C-4 落实）。
// 不用 inline style；不用浏览器 Fullscreen API（Q-e PM 决策）。

import { NModal } from 'naive-ui'
import LogList from './LogList.vue'
import type { VisibleLine } from '../../composables/log/useLogSearch'

defineProps<{
  show: boolean
  kind: string
  visibleLines: VisibleLine[]
  bufferEmpty: boolean
  noMatchHint: string
  wrap: boolean
  heightPx: number
  fontSizePx: string
  loading: boolean
  firstLoadError: string | null
  followTail: boolean
  paused: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', v: boolean): void
  (e: 'scroll', payload: {
    scrollTop: number
    scrollHeight: number
    clientHeight: number
  }): void
  (e: 'retry'): void
  (e: 'resumeFollow'): void
  (e: 'clearFilters'): void
  (e: 'scrollElReady', el: HTMLElement | null): void
}>()
</script>

<style scoped>
/* C-4：95vw / 90vh 通过 scoped :deep() 穿透到 Naive UI n-card 容器，
   避免任何 inline style；Q-e PM 决策实现。 */
.fullscreen-log-modal :deep(.n-card) {
  width: 95vw;
  height: 90vh;
  max-width: 95vw;
  max-height: 90vh;
}
.fullscreen-log-modal :deep(.n-card__content) {
  overflow: hidden;
}
</style>
