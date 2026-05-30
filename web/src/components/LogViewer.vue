<template>
  <div class="log-viewer-root" :style="rootCssVars">
    <log-toolbar
      :search="search.query.value"
      :case-sensitive="prefs.caseSensitive.value"
      :levels="filter.activeLevels.value"
      :follow-tail="follow.enabled.value"
      :wrap="prefs.wrap.value"
      :height="prefs.height.value"
      :last-updated="buf.lastUpdatedAt.value"
      :count="buf.lines.value.length"
      :max-count="MAX_LINES"
      :auto-refresh="buf.autoRefresh.value"
      :fail-count="buf.consecutiveFailCount.value"
      :last-error="buf.lastError.value"
      @update:search="search.setQuery"
      @update:case-sensitive="prefs.setCaseSensitive"
      @update:levels="filter.setActiveLevels"
      @update:follow-tail="onToggleFollow"
      @update:wrap="prefs.setWrap"
      @update:height="prefs.setHeight"
      @update:auto-refresh="onSetAutoRefresh"
      @copy="onCopy"
      @clear="onClear"
      @fullscreen="onOpenFullscreen"
      @scroll-to-bottom="follow.scrollToBottom"
    />

    <log-list
      v-show="!fullscreenOpen"
      :visible-lines="search.visibleLines.value"
      :buffer-empty="buf.lines.value.length === 0"
      :no-match-hint="noMatchHint"
      :wrap="prefs.wrap.value"
      :height-px="prefs.heightPx.value"
      :font-size-px="prefs.fontSizePx.value"
      :loading="buf.firstLoading.value"
      :first-load-error="buf.firstLoadError.value"
      :follow-tail="follow.enabled.value"
      :paused="follow.paused.value"
      @scroll="follow.onScroll"
      @retry="onRetry"
      @resume-follow="follow.resume"
      @clear-filters="onClearFilters"
      @scroll-el-ready="onScrollElReady"
    />

    <fullscreen-log-modal
      v-if="fullscreenOpen"
      :show="fullscreenOpen"
      :kind="kindLabel"
      :visible-lines="search.visibleLines.value"
      :buffer-empty="buf.lines.value.length === 0"
      :no-match-hint="noMatchHint"
      :wrap="prefs.wrap.value"
      :height-px="fullscreenHeightPx"
      :font-size-px="prefs.fontSizePx.value"
      :loading="buf.firstLoading.value"
      :first-load-error="buf.firstLoadError.value"
      :follow-tail="follow.enabled.value"
      :paused="follow.paused.value"
      @update:show="fullscreenOpen = $event"
      @scroll="follow.onScroll"
      @retry="onRetry"
      @resume-follow="follow.resume"
      @clear-filters="onClearFilters"
      @scroll-el-ready="onScrollElReady"
    />
  </div>
</template>

<script setup lang="ts">
// T-036 / log-ui-ux-polish · 02 §3.1
// 壳组件：持有全部 composable 实例 + 子组件协调；< 200 行。
// :style="rootCssVars" 是 NFR-4 单点豁免（C-3）：把 useThemeVars 投到 CSS 变量，
// 子组件全部走 var(--log-error) 等读取 → 切主题 0 额外代码即跟随（AC-13）。

import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useMessage, useThemeVars } from 'naive-ui'
import LogToolbar from './log/LogToolbar.vue'
import LogList from './log/LogList.vue'
import FullscreenLogModal from './log/FullscreenLogModal.vue'
import { useLogPrefs } from '../composables/log/useLogPrefs'
import { useLogBuffer } from '../composables/log/useLogBuffer'
import { useLogLevelFilter } from '../composables/log/useLogLevelFilter'
import { useLogSearch } from '../composables/log/useLogSearch'
import { useFollowTail } from '../composables/log/useFollowTail'
import { copyToClipboard } from '../utils/clipboard'

const MAX_LINES = 500

const props = defineProps<{
  kind: string
}>()

const themeVars = useThemeVars()
const message = useMessage()

const prefs = useLogPrefs()
const buf = useLogBuffer(() => props.kind, {
  max: MAX_LINES,
  message,
}) as ReturnType<typeof useLogBuffer> & { __bumpEpoch: () => void }
const filter = useLogLevelFilter(buf.parsedLines)
const search = useLogSearch(filter.filteredLines, prefs.caseSensitive)
const follow = useFollowTail(prefs.followTail)

const fullscreenOpen = ref(false)

const kindLabel = computed(() => (props.kind === 'frpc' ? 'frpc' : 'frps'))

const fullscreenHeightPx = computed(() => {
  if (typeof window === 'undefined') return 800
  return Math.max(400, Math.floor(window.innerHeight * 0.78))
})

const noMatchHint = computed(() => {
  if (filter.activeLevels.value.length === 0) {
    return '请至少选择一个日志等级'
  }
  return '无匹配日志（已应用筛选 / 搜索）'
})

const rootCssVars = computed<Record<string, string>>(() => {
  const t = themeVars.value
  return {
    '--log-error': t.errorColor,
    '--log-warn': t.warningColor,
    '--log-text': t.textColor1,
    '--log-text-3': t.textColor3,
    '--log-divider': t.dividerColor,
    '--log-bg': (t.codeColor as string | undefined) ?? t.cardColor,
    '--log-mark-bg': t.primaryColorSuppl,
  }
})

function onToggleFollow(v: boolean) {
  prefs.setFollowTail(v)
  follow.toggle(v)
}

function onSetAutoRefresh(v: boolean) {
  buf.setAutoRefresh(v)
}

async function onCopy() {
  // T-061：剪贴板 + execCommand fallback 逻辑抽到 utils/clipboard.ts（消除三处重复）。
  // 可观察行为字节不变：成功 message.success / 失败 message.error（各一次）。
  const text = search.visibleLines.value.map((v) => v.parsed.raw).join('\n')
  const ok = await copyToClipboard(text)
  if (ok) {
    message.success('已复制到剪贴板')
  } else {
    message.error('复制失败：请手动选择文本复制')
  }
}

function onClear() {
  buf.clear()
}

function onOpenFullscreen() {
  fullscreenOpen.value = true
}

function onRetry() {
  void buf.loadTail()
}

function onClearFilters() {
  search.setQuery('')
  filter.setActiveLevels(['ERROR', 'WARN', 'INFO', 'DEBUG', 'TRACE', 'PLAIN'])
}

function onScrollElReady(el: HTMLElement | null) {
  follow.bindScrollEl(el)
}

watch(
  () => props.kind,
  () => {
    buf.stopPolling()
    buf.setAutoRefresh(false)
    buf.__bumpEpoch()
    buf.clear()
    void buf.loadTail()
  },
)

watch(
  () => buf.lines.value.length,
  () => {
    follow.onNewLines()
  },
  { flush: 'post' },
)

onMounted(() => {
  void buf.loadTail()
})

onUnmounted(() => {
  buf.stopPolling()
  prefs.flush()
})

defineExpose({
  __testing: {
    prefs,
    buf,
    filter,
    search,
    follow,
    rootCssVars,
    fullscreenOpen,
    onCopy,
    onClear,
    onClearFilters,
    onRetry,
  },
})
</script>

<style scoped>
.log-viewer-root {
  width: 100%;
}
</style>
