<template>
  <n-space :size="4" align="center" style="display: inline-flex">
    <n-tooltip trigger="hover" placement="bottom">
      <template #trigger>
        <n-button
          size="small"
          :type="uploading ? 'warning' : 'default'"
          :loading="uploading"
          :disabled="uploading || props.siblingDownloading"
          @click="triggerFilePick"
        >
          {{ uploading ? `上传中 ${progress}%` : `上传 ${kind}` }}
        </n-button>
      </template>
      <!-- T-027 FR-9 / F-1：tooltip + disabled 替代 dialog（PM 决策保留）。
           AppLayout banner 已有显性"取消"按钮在相邻位置，tooltip 引导用户先取消。 -->
      <template v-if="props.siblingDownloading">
        正在下载 {{ kind }}，请先点击左侧"取消"按钮再上传
      </template>
      <template v-else>
        本地选择已下载好的 {{ kind }} 二进制（适合 GitHub 不可达时使用）
      </template>
    </n-tooltip>
    <n-progress
      v-if="uploading"
      type="line"
      :percentage="progress"
      :height="4"
      :border-radius="2"
      :show-indicator="false"
      style="width: 100px"
    />
    <input
      ref="fileInputRef"
      type="file"
      style="display: none"
      @change="handleFileChange"
    />
  </n-space>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { NButton, NProgress, NSpace, NTooltip, useMessage } from 'naive-ui'
import { apiUploadBin } from '../api/system'
import { extractErrorMessage } from '../api/client'

const props = withDefaults(defineProps<{
  kind: 'frpc' | 'frps'
  // T-027 FR-9：父组件传入"同 kind 是否正在下载"，控制本按钮 disabled + tooltip 文案。
  siblingDownloading?: boolean
}>(), {
  siblingDownloading: false,
})

const emit = defineEmits<{
  (e: 'uploaded', payload: { sha256: string; size: number; kind: 'frpc' | 'frps' }): void
}>()

const message = useMessage()
const fileInputRef = ref<HTMLInputElement | null>(null)
const uploading = ref(false)
const progress = ref(0)

// 前端最大文件大小（与后端 uploadBinMaxBytes 一致）：64 MiB。
const MAX_BIN_BYTES = 64 * 1024 * 1024

function triggerFilePick() {
  if (uploading.value) return
  fileInputRef.value?.click()
}

async function handleFileChange(evt: Event) {
  const input = evt.target as HTMLInputElement
  const file = input.files?.[0]
  // 清空 input 让相同文件名可重新选；不依赖 reactive 更新
  if (input) input.value = ''
  if (!file) return

  // 前端预校验：大小（避免 64 MiB 大文件先传完再被拒）
  if (file.size === 0) {
    message.error('上传文件为空')
    return
  }
  if (file.size > MAX_BIN_BYTES) {
    message.error('文件超过 64 MiB 上限（请确认上传的是单 binary 而不是 .tar.gz / .zip）')
    return
  }

  uploading.value = true
  progress.value = 0
  try {
    const res = await apiUploadBin(props.kind, file, (pct) => {
      progress.value = pct
    })
    message.success(`已上传 ${props.kind}（${formatBytes(res.size)}）`)
    if (res.advisory) {
      message.info(res.advisory)
    }
    emit('uploaded', {
      sha256: res.sha256,
      size: res.size,
      kind: props.kind,
    })
  } catch (e) {
    message.error(extractErrorMessage(e, `上传 ${props.kind} 失败`))
  } finally {
    uploading.value = false
    progress.value = 0
  }
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KiB`
  return `${(n / 1024 / 1024).toFixed(1)} MiB`
}
</script>
