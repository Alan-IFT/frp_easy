<template>
  <div class="log-toolbar">
    <!-- 第一行：搜索 + 等级 + 跟随 / 折行 / 高度 / 自动刷新 -->
    <n-space size="small" align="center" :wrap="true">
      <n-input
        :value="search"
        placeholder="搜索关键字…"
        clearable
        size="small"
        class="search-input"
        @update:value="(v: string) => emit('update:search', v)"
      />
      <n-button
        size="small"
        :type="caseSensitive ? 'primary' : 'default'"
        :ghost="caseSensitive"
        aria-label="切换大小写敏感"
        title="大小写敏感"
        @click="emit('update:caseSensitive', !caseSensitive)"
      >
        Aa
      </n-button>

      <n-select
        multiple
        size="small"
        class="level-select"
        :value="levels"
        :options="levelOptions"
        placeholder="日志等级"
        @update:value="(v: LogLevelOrPlain[]) => emit('update:levels', v)"
      />

      <n-space size="small" align="center" :wrap="false" class="switch-group">
        <n-text depth="3" class="switch-label">跟随</n-text>
        <n-switch
          size="small"
          :value="followTail"
          @update:value="(v: boolean) => emit('update:followTail', v)"
        />
        <n-text depth="3" class="switch-label">折行</n-text>
        <n-switch
          size="small"
          :value="wrap"
          @update:value="(v: boolean) => emit('update:wrap', v)"
        />
        <n-text depth="3" class="switch-label">自动刷新</n-text>
        <n-switch
          size="small"
          :value="autoRefresh"
          @update:value="(v: boolean) => emit('update:autoRefresh', v)"
        />
      </n-space>

      <n-select
        size="small"
        class="height-select"
        :value="height"
        :options="heightOptions"
        @update:value="onHeightSelect"
      />

      <n-button size="small" @click="emit('copy')">复制</n-button>
      <n-button size="small" @click="emit('clear')">清屏</n-button>
      <n-button size="small" @click="emit('scrollToBottom')">↓ 底部</n-button>
      <n-button size="small" @click="emit('fullscreen')">全屏</n-button>
    </n-space>

    <!-- 第二行：心跳 / 计数 / 失败小红点 -->
    <n-space size="small" align="center" class="meta-row">
      <n-text depth="3" class="meta">上次更新：{{ lastUpdatedLabel }}</n-text>
      <n-text depth="3" class="meta">{{ count }} / {{ maxCount }}</n-text>
      <n-tooltip v-if="failCount > 0" placement="top">
        <template #trigger>
          <span class="fail-dot" aria-label="自动刷新出错" />
        </template>
        最近一次错误：{{ lastError || '未知错误' }}（连续失败 {{ failCount }} 次）
      </n-tooltip>
    </n-space>
  </div>
</template>

<script setup lang="ts">
// T-036 / log-ui-ux-polish · 02 §3.2
// 工具条：搜索 / 等级 / 跟随 / 折行 / 高度 / 复制 / 清屏 / 全屏 / ↓底部 / 心跳 / 计数 / 失败小红点。
// props 只入；emit 用户意图（单向数据流，insight L28）。

import { computed } from 'vue'
import {
  NSpace,
  NInput,
  NButton,
  NSelect,
  NSwitch,
  NText,
  NTooltip,
} from 'naive-ui'
import {
  ALL_LEVELS,
  type LogLevelOrPlain,
} from '../../composables/log/parseLogLine'
import type { LogHeight } from '../../composables/log/useLogPrefs'

const props = defineProps<{
  search: string
  caseSensitive: boolean
  levels: LogLevelOrPlain[]
  followTail: boolean
  wrap: boolean
  height: LogHeight
  lastUpdated: number
  count: number
  maxCount: number
  autoRefresh: boolean
  failCount: number
  lastError: string | null
}>()

const emit = defineEmits<{
  (e: 'update:search', v: string): void
  (e: 'update:caseSensitive', v: boolean): void
  (e: 'update:levels', v: LogLevelOrPlain[]): void
  (e: 'update:followTail', v: boolean): void
  (e: 'update:wrap', v: boolean): void
  (e: 'update:height', v: LogHeight): void
  (e: 'update:autoRefresh', v: boolean): void
  (e: 'copy'): void
  (e: 'clear'): void
  (e: 'fullscreen'): void
  (e: 'scrollToBottom'): void
}>()

const levelOptions = ALL_LEVELS.map((l) => ({ label: l, value: l }))

const heightOptions = [
  { label: '300 px', value: 300 },
  { label: '500 px', value: 500 },
  { label: '800 px', value: 800 },
  { label: '全屏', value: -1 },
]

function onHeightSelect(v: number) {
  if (v === -1) {
    emit('fullscreen')
    return
  }
  if (v === 300 || v === 500 || v === 800) {
    emit('update:height', v as LogHeight)
  }
}

function pad2(n: number): string {
  return n < 10 ? `0${n}` : String(n)
}

const lastUpdatedLabel = computed(() => {
  if (!props.lastUpdated) return '—'
  const d = new Date(props.lastUpdated)
  return `${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(d.getSeconds())}`
})
</script>

<style scoped>
.log-toolbar {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-bottom: 8px;
}

.search-input {
  width: 220px;
}

.level-select {
  width: 240px;
}

.height-select {
  width: 110px;
}

.switch-label {
  font-size: 12px;
}

.switch-group {
  padding: 0 4px;
}

.meta-row {
  padding-left: 2px;
}

.meta {
  font-size: 12px;
}

.fail-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--log-error, #d03050);
}
</style>
