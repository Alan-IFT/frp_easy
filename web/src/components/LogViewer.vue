<template>
  <div>
    <n-space justify="space-between" align="center" style="margin-bottom: 8px">
      <n-text strong>{{ kindLabel }} 日志</n-text>
      <n-space align="center">
        <n-text depth="3" style="font-size: 12px">自动刷新</n-text>
        <n-switch v-model:value="autoRefresh" @update:value="handleAutoRefreshChange" />
        <n-button size="small" @click="loadTail">刷新</n-button>
      </n-space>
    </n-space>
    <n-code
      :code="logText"
      language="text"
      style="max-height: 500px; overflow-y: auto; background: #1a1a1a; padding: 12px; border-radius: 6px; font-size: 12px; font-family: monospace; white-space: pre-wrap; word-break: break-all"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { NSpace, NText, NSwitch, NButton, NCode } from 'naive-ui'
import { apiGetLogsTail, apiGetLogsIncremental } from '../api/logs'

const props = defineProps<{
  kind: string
}>()

const kindLabel = computed(() => props.kind === 'frpc' ? 'frpc' : 'frps')
const lines = ref<string[]>([])
const logText = computed(() => lines.value.join('\n') || '（暂无日志）')
const autoRefresh = ref(false)
const currentOffset = ref(0)
const pollingTimer = ref<ReturnType<typeof setInterval> | null>(null)

async function loadTail() {
  try {
    const res = await apiGetLogsTail(props.kind, 500)
    lines.value = res.lines
    // 次回のインクリメンタル取得のためオフセットをリセット（0 = ファイル末尾から再取得）
    currentOffset.value = 0
  } catch {
    // ignore
  }
}

async function loadIncremental() {
  try {
    const res = await apiGetLogsIncremental(props.kind, currentOffset.value)
    if (res.data) {
      const newLines = res.data.split('\n').filter((l) => l !== '')
      lines.value = [...lines.value, ...newLines].slice(-500)
    }
    currentOffset.value = res.nextOffset
  } catch {
    // ignore
  }
}

function handleAutoRefreshChange(val: boolean) {
  if (val) {
    startPolling()
  } else {
    stopPolling()
  }
}

function startPolling() {
  if (pollingTimer.value !== null) return
  pollingTimer.value = setInterval(() => {
    void loadIncremental()
  }, 2000)
}

function stopPolling() {
  if (pollingTimer.value !== null) {
    clearInterval(pollingTimer.value)
    pollingTimer.value = null
  }
}

watch(() => props.kind, () => {
  stopPolling()
  autoRefresh.value = false
  void loadTail()
})

onMounted(() => {
  void loadTail()
})

onUnmounted(() => {
  stopPolling()
})
</script>
