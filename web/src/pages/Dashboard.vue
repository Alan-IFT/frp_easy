<template>
  <div>
    <n-page-header title="仪表盘" subtitle="frpc / frps 进程状态与控制" />

    <!-- バイナリ欠損警告 -->
    <n-alert
      v-if="appStore.binMissing.length > 0"
      type="warning"
      title="二进制文件缺失"
      style="margin: 16px 0"
    >
      以下进程的二进制文件未找到：{{ appStore.binMissing.join(', ') }}。
      请将对应文件放置到 <n-text code>frp_win/</n-text> 或 <n-text code>frp_linux/</n-text> 目录下后重启。
    </n-alert>

    <n-grid :cols="2" :x-gap="16" :y-gap="16" style="margin-top: 16px">
      <!-- frpc カード -->
      <n-gi>
        <n-card title="frpc（客户端）">
          <template #header-extra>
            <n-space align="center">
              <n-text depth="3" style="font-size: 13px">自动启动</n-text>
              <n-switch
                :value="modeState.frpc"
                :disabled="appStore.frpcMissing || modeLoading.frpc"
                :loading="modeLoading.frpc"
                @update:value="handleModeToggle('frpc', $event)"
              />
              <status-badge :state="procStore.frpcInfo.state" />
            </n-space>
          </template>
          <n-descriptions :column="1" size="small">
            <n-descriptions-item label="状态">
              <status-badge :state="procStore.frpcInfo.state" />
            </n-descriptions-item>
            <n-descriptions-item label="PID">
              {{ procStore.frpcInfo.pid > 0 ? procStore.frpcInfo.pid : '—' }}
            </n-descriptions-item>
            <n-descriptions-item label="最后变更">
              {{ formatTime(procStore.frpcInfo.changedAt) }}
            </n-descriptions-item>
          </n-descriptions>

          <!-- エラー時の最後のエラーメッセージ -->
          <n-alert
            v-if="procStore.frpcInfo.state === 'error' && procStore.frpcInfo.lastErr"
            type="error"
            style="margin-top: 12px"
          >
            <div style="font-family: monospace; font-size: 12px; white-space: pre-wrap">
              {{ procStore.frpcInfo.lastErr }}
            </div>
            <n-button text tag="a" href="/logs/frpc" style="margin-top: 4px">
              查看完整日志 →
            </n-button>
          </n-alert>

          <template #action>
            <n-space>
              <n-button
                type="primary"
                :disabled="!canStart('frpc') || appStore.frpcMissing"
                :loading="loadingMap['frpc-start']"
                @click="handleStart('frpc')"
              >
                启动
              </n-button>
              <n-button
                type="error"
                :disabled="!canStop('frpc')"
                :loading="loadingMap['frpc-stop']"
                @click="handleStop('frpc')"
              >
                停止
              </n-button>
              <n-button
                :disabled="procStore.frpcInfo.state === 'stopped' || appStore.frpcMissing"
                :loading="loadingMap['frpc-restart']"
                @click="handleRestart('frpc')"
              >
                重启
              </n-button>
            </n-space>
          </template>
        </n-card>
      </n-gi>

      <!-- frps カード -->
      <n-gi>
        <n-card title="frps（服务端）">
          <template #header-extra>
            <n-space align="center">
              <n-text depth="3" style="font-size: 13px">自动启动</n-text>
              <n-switch
                :value="modeState.frps"
                :disabled="appStore.frpsMissing || modeLoading.frps"
                :loading="modeLoading.frps"
                @update:value="handleModeToggle('frps', $event)"
              />
              <status-badge :state="procStore.frpsInfo.state" />
            </n-space>
          </template>
          <n-descriptions :column="1" size="small">
            <n-descriptions-item label="状态">
              <status-badge :state="procStore.frpsInfo.state" />
            </n-descriptions-item>
            <n-descriptions-item label="PID">
              {{ procStore.frpsInfo.pid > 0 ? procStore.frpsInfo.pid : '—' }}
            </n-descriptions-item>
            <n-descriptions-item label="最后变更">
              {{ formatTime(procStore.frpsInfo.changedAt) }}
            </n-descriptions-item>
          </n-descriptions>

          <n-alert
            v-if="procStore.frpsInfo.state === 'error' && procStore.frpsInfo.lastErr"
            type="error"
            style="margin-top: 12px"
          >
            <div style="font-family: monospace; font-size: 12px; white-space: pre-wrap">
              {{ procStore.frpsInfo.lastErr }}
            </div>
            <n-button text tag="a" href="/logs/frps" style="margin-top: 4px">
              查看完整日志 →
            </n-button>
          </n-alert>

          <template #action>
            <n-space>
              <n-button
                type="primary"
                :disabled="!canStart('frps') || appStore.frpsMissing"
                :loading="loadingMap['frps-start']"
                @click="handleStart('frps')"
              >
                启动
              </n-button>
              <n-button
                type="error"
                :disabled="!canStop('frps')"
                :loading="loadingMap['frps-stop']"
                @click="handleStop('frps')"
              >
                停止
              </n-button>
              <n-button
                :disabled="procStore.frpsInfo.state === 'stopped' || appStore.frpsMissing"
                :loading="loadingMap['frps-restart']"
                @click="handleRestart('frps')"
              >
                重启
              </n-button>
            </n-space>
          </template>
        </n-card>
      </n-gi>
    </n-grid>
  </div>
</template>

<script setup lang="ts">
import { reactive, onMounted, onUnmounted } from 'vue'
import {
  NPageHeader, NCard, NGrid, NGi, NSpace, NButton, NAlert,
  NDescriptions, NDescriptionsItem, NText, NSwitch,
  useMessage,
} from 'naive-ui'
import StatusBadge from '../components/StatusBadge.vue'
import { useProcStore } from '../stores/proc'
import { useAppStore } from '../stores/app'
import { extractErrorMessage } from '../api/client'
import { apiGetMode, apiPutMode } from '../api/mode'
import type { ProcessState } from '../types'

const procStore = useProcStore()
const appStore = useAppStore()
const message = useMessage()

const loadingMap = reactive<Record<string, boolean>>({})
const modeState = reactive({ frpc: false, frps: false })
const modeLoading = reactive({ frpc: false, frps: false })

async function fetchMode() {
  try {
    const s = await apiGetMode()
    modeState.frpc = s.frpc
    modeState.frps = s.frps
  } catch {
    // 非致命性错误，静默忽略
  }
}

async function handleModeToggle(kind: 'frpc' | 'frps', enabled: boolean) {
  modeLoading[kind] = true
  try {
    const next = { ...modeState, [kind]: enabled }
    const result = await apiPutMode(next)
    modeState.frpc = result.frpc
    modeState.frps = result.frps
    message.success(`${kind} 自动启动已${enabled ? '开启' : '关闭'}`)
  } catch (e) {
    message.error(extractErrorMessage(e, `${kind} 模式切换失败`))
  } finally {
    modeLoading[kind] = false
  }
}

function canStart(kind: string): boolean {
  const state: ProcessState = kind === 'frpc' ? procStore.frpcInfo.state : procStore.frpsInfo.state
  return state === 'stopped' || state === 'error'
}

function canStop(kind: string): boolean {
  const state: ProcessState = kind === 'frpc' ? procStore.frpcInfo.state : procStore.frpsInfo.state
  return state === 'running' || state === 'starting'
}

async function handleStart(kind: string) {
  const key = `${kind}-start`
  loadingMap[key] = true
  try {
    await procStore.startProc(kind)
    message.success(`${kind} 启动指令已发送`)
  } catch (e) {
    message.error(extractErrorMessage(e, `${kind} 启动失败`))
  } finally {
    loadingMap[key] = false
  }
}

async function handleStop(kind: string) {
  const key = `${kind}-stop`
  loadingMap[key] = true
  try {
    await procStore.stopProc(kind)
    message.success(`${kind} 停止指令已发送`)
  } catch (e) {
    message.error(extractErrorMessage(e, `${kind} 停止失败`))
  } finally {
    loadingMap[key] = false
  }
}

async function handleRestart(kind: string) {
  const key = `${kind}-restart`
  loadingMap[key] = true
  try {
    await procStore.restartProc(kind)
    message.success(`${kind} 重启指令已发送`)
  } catch (e) {
    message.error(extractErrorMessage(e, `${kind} 重启失败`))
  } finally {
    loadingMap[key] = false
  }
}

function formatTime(iso: string): string {
  if (!iso) return '—'
  try {
    return new Date(iso).toLocaleString('zh-CN')
  } catch {
    return iso
  }
}

onMounted(() => {
  procStore.startPolling()
  fetchMode()
})

onUnmounted(() => {
  procStore.stopPolling()
})
</script>
