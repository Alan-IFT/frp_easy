<template>
  <div class="log-list-root">
    <!-- 状态分支 1：首次加载错误 → 红字 + 重试 -->
    <div v-if="firstLoadError" class="log-empty error">
      <n-text type="error">加载日志失败：{{ firstLoadError }}</n-text>
      <n-button size="small" type="primary" @click="$emit('retry')">
        重试
      </n-button>
    </div>

    <!-- 状态分支 2：首次加载中 -->
    <div v-else-if="loading" class="log-empty">
      <n-spin size="small" />
      <n-text depth="3">正在加载日志…</n-text>
    </div>

    <!-- 状态分支 3 / 4 / 5：列表 / 空态 / 无命中 -->
    <div
      v-else
      ref="scrollEl"
      class="log-list-scroll"
      :style="{ '--log-list-height': heightPx + 'px', '--log-font-size': fontSizePx }"
      @scroll="onScrollNative"
    >
      <!-- justify-inline-style: 单一动态 CSS 变量赋值（max-height + font-size），
           无法走静态 class（高度 300/500/800/全屏 任意切换 + 字号未来扩展）；
           NFR-4 self-check 02 §10 PM/SA 已签字接受（C-3 落实位置）。 -->

      <!-- paused 提示条 sticky 顶部 -->
      <div
        v-if="paused"
        class="paused-banner"
        role="button"
        tabindex="0"
        aria-label="已暂停跟随，点击回到底部"
        @click="$emit('resumeFollow')"
        @keydown.enter="$emit('resumeFollow')"
        @keydown.space.prevent="$emit('resumeFollow')"
      >
        已暂停跟随；点击此处回到底部
      </div>

      <!-- 空态（缓冲为 0） -->
      <div v-if="visibleLines.length === 0 && bufferEmpty" class="log-empty inline">
        <n-text depth="3">暂无日志输出</n-text>
      </div>

      <!-- 无命中（已应用筛选 / 搜索） -->
      <div
        v-else-if="visibleLines.length === 0 && !bufferEmpty"
        class="log-empty inline"
      >
        <n-text depth="3">{{ noMatchHint }}</n-text>
        <n-button text type="primary" @click="$emit('clearFilters')">
          清空筛选
        </n-button>
      </div>

      <!-- 列表 -->
      <template v-else>
        <log-line
          v-for="vl in visibleLines"
          :key="vl.lineNumber + ':' + vl.parsed.raw"
          :line-number="vl.lineNumber"
          :parsed="vl.parsed"
          :search-hits="vl.searchHits"
          :wrap="wrap"
        />
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
// T-036 / log-ui-ux-polish · 02 §3.3
// 滚动容器 + 状态分支（空态 / 加载态 / 错误 / 无命中 / 正常列表）+ 暂停跟随提示条。
// 不持有滚动状态机，事件原样向上 emit。

import { ref, watch } from 'vue'
import { NText, NButton, NSpin } from 'naive-ui'
import LogLine from './LogLine.vue'
import type { VisibleLine } from '../../composables/log/useLogSearch'

defineProps<{
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

const scrollEl = ref<HTMLElement | null>(null)

watch(scrollEl, (el) => {
  emit('scrollElReady', el)
})

function onScrollNative(ev: Event) {
  const t = ev.target as HTMLElement
  emit('scroll', {
    scrollTop: t.scrollTop,
    scrollHeight: t.scrollHeight,
    clientHeight: t.clientHeight,
  })
}

defineExpose({ scrollEl })
</script>

<style scoped>
.log-list-root {
  position: relative;
  width: 100%;
}

.log-list-scroll {
  max-height: var(--log-list-height);
  overflow-y: auto;
  overflow-x: hidden;
  background: var(--log-bg);
  border-radius: 6px;
  padding: 8px 0;
  position: relative;
}

.log-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 32px 12px;
  min-height: 80px;
}

.log-empty.inline {
  min-height: 60px;
  padding: 16px;
}

.log-empty.error {
  flex-direction: column;
  gap: 12px;
}

.paused-banner {
  position: sticky;
  top: 0;
  z-index: 2;
  background: var(--log-mark-bg);
  color: var(--log-text);
  text-align: center;
  padding: 6px 12px;
  font-size: 12px;
  cursor: pointer;
  user-select: none;
  border-bottom: 1px solid var(--log-divider);
}
.paused-banner:hover {
  filter: brightness(1.05);
}
</style>
