<template>
  <div>
    <n-button
      :loading="loading"
      :disabled="loading"
      @click="detect"
    >
      检测公网 IP
    </n-button>

    <div v-if="result" style="margin-top: 12px">
      <n-alert
        v-if="result.ip"
        type="success"
        title="检测到公网 IP"
        :show-icon="true"
      >
        <div style="display: flex; align-items: center; gap: 8px">
          <span style="font-family: monospace; font-size: 15px">{{ result.ip }}</span>
          <n-button size="tiny" type="default" text @click="copyIp">
            {{ copied ? '已复制 ✓' : '复制' }}
          </n-button>
        </div>
        <div v-if="result.advisory" style="margin-top: 6px; font-size: 13px; color: #888">
          {{ result.advisory }}
        </div>
      </n-alert>

      <n-alert
        v-else-if="result.error"
        type="warning"
        title="检测失败"
        :show-icon="true"
      >
        {{ result.error }}
      </n-alert>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { NButton, NAlert, useMessage } from 'naive-ui'
import { apiGetPublicIP } from '../api/system'
import { extractErrorMessage } from '../api/client'
import type { PublicIPResponse } from '../types'

const message = useMessage()
const loading = ref(false)
const result = ref<PublicIPResponse | null>(null)
const copied = ref(false)

async function detect() {
  loading.value = true
  result.value = null
  try {
    result.value = await apiGetPublicIP()
  } catch (e) {
    // A4：透传后端精确原因（与全站 extractErrorMessage 一致），无结构化消息时回落友好文案。
    result.value = { error: extractErrorMessage(e, '请求失败，请稍后重试') }
  } finally {
    loading.value = false
  }
}

// T-058 (A)：剪贴板写入失败不再静默吞错。1:1 搬运 LogViewer.vue onCopy 范式。
async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    message.success('已复制到剪贴板')
    return true
  } catch {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.setAttribute('aria-hidden', 'true')
    ta.style.position = 'fixed'
    ta.style.left = '-9999px'
    document.body.appendChild(ta)
    ta.select()
    let ok = false
    try {
      ok = document.execCommand('copy')
    } catch {
      ok = false
    } finally {
      document.body.removeChild(ta)
    }
    if (ok) {
      message.success('已复制到剪贴板')
    } else {
      message.error('复制失败：请手动选择文本复制')
    }
    return ok
  }
}

async function copyIp() {
  if (!result.value?.ip) return
  // 仅在复制成功时给"已复制 ✓"短暂态视觉反馈（失败已弹 message.error）
  if (await copyText(result.value.ip)) {
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  }
}
</script>
