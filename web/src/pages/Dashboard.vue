<template>
  <div>
    <n-page-header title="仪表盘" subtitle="frpc / frps 进程状态与控制" />

    <!-- T-038 boot-autostart-hardening：服务化状态卡片（[boot-autostart-fix]） -->
    <ServiceStatusCard style="margin: 16px 0" />

    <!-- 二进制缺失警告 -->
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
      <!-- frpc 卡片 -->
      <n-gi>
        <n-card title="frpc（客户端）">
          <template #header-extra>
            <n-space align="center">
              <n-text depth="3" style="font-size: 13px">自动启动</n-text>
              <!-- T-047 A2：状态获取失败时禁用开关 + tooltip 提示 + 重试入口，不静默撒谎 -->
              <n-tooltip v-if="modeFetchFailed" trigger="hover">
                <template #trigger>
                  <n-switch :value="modeState.frpc" disabled />
                </template>
                状态获取失败，请点击刷新
              </n-tooltip>
              <n-switch
                v-else
                :value="modeState.frpc"
                :disabled="appStore.frpcMissing || modeLoading.frpc"
                :loading="modeLoading.frpc"
                @update:value="handleModeToggle('frpc', $event)"
              />
              <n-button
                v-if="modeFetchFailed"
                size="tiny"
                tertiary
                @click="retryFetchMode"
              >
                刷新状态
              </n-button>
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

          <!-- 错误详情（T-007 AC-7：lastErr 默认完整显示；word-break 防长 token 溢出） -->
          <n-alert
            v-if="procStore.frpcInfo.state === 'error' && procStore.frpcInfo.lastErr"
            type="error"
            style="margin-top: 12px"
          >
            <div style="font-family: monospace; font-size: 12px; white-space: pre-wrap; word-break: break-word">
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

      <!-- frps 卡片 -->
      <n-gi>
        <n-card title="frps（服务端）">
          <template #header-extra>
            <n-space align="center">
              <n-text depth="3" style="font-size: 13px">自动启动</n-text>
              <!-- T-047 A2：状态获取失败时禁用开关 + tooltip 提示 + 重试入口，不静默撒谎 -->
              <n-tooltip v-if="modeFetchFailed" trigger="hover">
                <template #trigger>
                  <n-switch :value="modeState.frps" disabled />
                </template>
                状态获取失败，请点击刷新
              </n-tooltip>
              <n-switch
                v-else
                :value="modeState.frps"
                :disabled="appStore.frpsMissing || modeLoading.frps"
                :loading="modeLoading.frps"
                @update:value="handleModeToggle('frps', $event)"
              />
              <n-button
                v-if="modeFetchFailed"
                size="tiny"
                tertiary
                @click="retryFetchMode"
              >
                刷新状态
              </n-button>
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
            <div style="font-family: monospace; font-size: 12px; white-space: pre-wrap; word-break: break-word">
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
import { reactive, ref, onMounted, onUnmounted } from 'vue'
import {
  NPageHeader, NCard, NGrid, NGi, NSpace, NButton, NAlert,
  NDescriptions, NDescriptionsItem, NText, NSwitch, NTooltip,
  useMessage,
} from 'naive-ui'
import StatusBadge from '../components/StatusBadge.vue'
import ServiceStatusCard from '../components/ServiceStatusCard.vue'
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
// T-047 A2：自动启动状态获取失败标记
const modeFetchFailed = ref(false)

// T-047 A2：自动启动开关获取失败不再静默。失败时给出可见信号（warning + 失败态开关），
// 否则两个 n-switch 停在 false 与真实状态不符 = UI 撒谎。
async function fetchMode() {
  modeFetchFailed.value = false
  try {
    const s = await apiGetMode()
    modeState.frpc = s.frpc
    modeState.frps = s.frps
  } catch (e) {
    modeFetchFailed.value = true
    message.warning(extractErrorMessage(e, '自动启动状态获取失败，请点击刷新重试'))
  }
}

// T-047 A2：开关状态获取失败标记。true 时禁用开关并展示重试入口。
function retryFetchMode(): void {
  void fetchMode()
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

// 暴露给测试的 handle（getExposed 范式；禁用 wrapper.vm.__testing）
defineExpose({
  __testing: {
    modeState,
    modeLoading,
    modeFetchFailed,
    fetchMode,
    retryFetchMode,
    handleModeToggle,
  },
})
</script>
