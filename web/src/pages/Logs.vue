<template>
  <div>
    <n-page-header
      :title="`${kindLabel} 日志`"
      :subtitle="`查看 ${kindLabel} 进程的运行日志`"
    />
    <n-card style="margin-top: 16px">
      <log-viewer :kind="kind" />
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { NPageHeader, NCard } from 'naive-ui'
import LogViewer from '../components/LogViewer.vue'

const props = defineProps<{
  kind: string
}>()

const kindLabel = computed(() => props.kind === 'frpc' ? 'frpc' : 'frps')
</script>
