<template>
  <div :class="['log-line', `level-${levelLower}`, wrap ? 'wrap' : 'nowrap']">
    <span class="line-number" aria-hidden="true">{{ lineNumber }}</span>
    <span v-if="parsed.timestamp" class="line-timestamp">
      {{ parsed.timestamp }}
    </span>
    <span v-if="parsed.level !== 'PLAIN'" class="line-level">
      {{ parsed.level }}
    </span>
    <!-- eslint-disable vue/no-v-html -->
    <span class="line-message" v-html="renderedMessage" />
    <!-- eslint-enable vue/no-v-html -->
  </div>
</template>

<script setup lang="ts">
// T-036 / log-ui-ux-polish · 02 §3.4
// 单行视觉；纯展示，无状态。
// 严格 "先 escape 后 mark" 顺序（NFR-7 XSS 防御 / ADV-A 反向测试覆盖）。

import { computed } from 'vue'
import type { ParsedLogLine } from '../../composables/log/parseLogLine'
import type { SearchHit } from '../../composables/log/useLogSearch'

const props = defineProps<{
  lineNumber: number
  parsed: ParsedLogLine
  searchHits: SearchHit[]
  wrap: boolean
}>()

const levelLower = computed(() => props.parsed.level.toLowerCase())

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

/**
 * 先 escape 整段 message → 然后按"原始 message 字符索引"切片重组并在命中段外包 <mark>。
 * 切片用 escape 后的字符串重新构建，避免 hits 坐标错位（因为 escape 后字符串变长）。
 *
 * 策略：对原始 message 按 hits 区间切成 [前导/命中/后续/命中/...] 段，每段单独 escape，
 * 命中段 escape 后用 <mark> 包裹。这样既保证 escape 完整，又保证 <mark> 不被自身 escape。
 */
const renderedMessage = computed(() => {
  const msg = props.parsed.message
  const hits = props.searchHits
  if (!hits || hits.length === 0) {
    return escapeHtml(msg)
  }
  // 按 start 排序（防御性 —— 调用方理论上已有序）
  const sorted = [...hits].sort((a, b) => a.start - b.start)
  const parts: string[] = []
  let cursor = 0
  for (const h of sorted) {
    if (h.start > cursor) {
      parts.push(escapeHtml(msg.slice(cursor, h.start)))
    }
    parts.push(
      `<mark class="search-hit">${escapeHtml(msg.slice(h.start, h.end))}</mark>`,
    )
    cursor = h.end
  }
  if (cursor < msg.length) {
    parts.push(escapeHtml(msg.slice(cursor)))
  }
  return parts.join('')
})
</script>

<style scoped>
.log-line {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 2px 8px;
  font-family: ui-monospace, SFMono-Regular, Consolas, Menlo, Monaco, monospace;
  font-size: var(--log-font-size, 13px);
  line-height: 1.55;
  color: var(--log-text);
}

.line-number {
  flex: 0 0 auto;
  min-width: 3em;
  text-align: right;
  color: var(--log-text-3);
  user-select: none;
  -webkit-user-select: none;
}

.line-timestamp {
  flex: 0 0 auto;
  color: var(--log-text-3);
  white-space: nowrap;
}

.line-level {
  flex: 0 0 auto;
  min-width: 4em;
  font-weight: 600;
  text-align: left;
}

.line-message {
  flex: 1 1 auto;
  min-width: 0;
}

.log-line.wrap .line-message {
  white-space: pre-wrap;
  word-break: break-all;
}

.log-line.nowrap .line-message {
  white-space: pre;
  overflow-x: auto;
}

/* 等级着色 —— 全部走主题 token CSS variable */
.log-line.level-error .line-message,
.log-line.level-error .line-level {
  color: var(--log-error);
}
.log-line.level-warn .line-message,
.log-line.level-warn .line-level {
  color: var(--log-warn);
}
.log-line.level-info .line-message,
.log-line.level-info .line-level {
  color: var(--log-text);
}
.log-line.level-debug .line-message,
.log-line.level-debug .line-level,
.log-line.level-trace .line-message,
.log-line.level-trace .line-level {
  color: var(--log-text-3);
}
.log-line.level-plain .line-message {
  color: var(--log-text);
}

/* 搜索高亮 */
.line-message :deep(.search-hit) {
  background: var(--log-mark-bg);
  color: inherit;
  padding: 0 1px;
  border-radius: 2px;
}
</style>
