<!--
  ServiceStatusCard.vue — T-038 boot-autostart-hardening

  Dashboard 顶部"服务化状态"卡片。展示：
    · 监管方式（systemd / Windows Service / 未监管）
    · 开机自启（是 / 否）
    · 运行用户
    · 重启后自动恢复（哪些 kind 启用，按 kind 列出 last_run.outcome）

  当 supervised=false 或 boot_autostart=false 时高亮 warning，展开"如何修复"
  折叠区给出对应平台精确命令（含 [boot-autostart-fix] 锚字串供 verify_all I.3 守门）。

  设计依据：T-038 02 §3.5-§3.6 / 03 §3 Q-6。
-->
<template>
  <n-card
    title="服务化状态"
    size="small"
    :bordered="true"
    :class="['svc-status-card', needsFix ? 'svc-status-card--warn' : 'svc-status-card--ok']"
  >
    <template #header-extra>
      <n-button
        size="tiny"
        quaternary
        :loading="loading"
        :disabled="loading"
        @click="refresh"
      >
        刷新
      </n-button>
    </template>

    <n-text v-if="loading && !status" depth="3" style="font-size: 13px">
      加载中…
    </n-text>
    <n-text v-else-if="error" type="error" style="font-size: 13px">
      加载失败：{{ error }}
    </n-text>
    <div v-else-if="status">
      <n-descriptions :column="2" size="small" label-placement="left" bordered>
        <n-descriptions-item label="监管方式">
          <n-tag :type="supervisorTagType" size="small">{{ supervisorLabel }}</n-tag>
        </n-descriptions-item>
        <n-descriptions-item label="开机自启">
          <n-tag :type="status.boot_autostart ? 'success' : 'warning'" size="small">
            {{ status.boot_autostart ? '是' : '否' }}
          </n-tag>
        </n-descriptions-item>
        <n-descriptions-item label="运行用户">
          {{ status.run_as || '—' }}
        </n-descriptions-item>
        <n-descriptions-item label="启用自动恢复">
          <template v-if="status.auto_restore.enabled_kinds.length === 0">
            <n-text depth="3">无</n-text>
          </template>
          <template v-else>
            <n-tag
              v-for="k in status.auto_restore.enabled_kinds"
              :key="k"
              size="small"
              style="margin-right: 4px"
            >
              {{ k }}
            </n-tag>
          </template>
        </n-descriptions-item>
      </n-descriptions>

      <!-- 上次自动恢复结果（按 kind 拆分展示） -->
      <n-collapse v-if="hasLastRuns" style="margin-top: 12px" :default-expanded-names="needsFix ? ['last-run'] : []">
        <n-collapse-item name="last-run" title="上次自动恢复结果">
          <div
            v-for="[kind, run] in (Object.entries(status.auto_restore.last_runs ?? {}) as Array<[string, AutoRestoreLastRun]>)"
            :key="kind"
            style="margin-bottom: 8px; font-size: 12px"
          >
            <div>
              <strong>{{ kind }}</strong>
              <n-tag :type="outcomeTagType(run.outcome)" size="tiny" style="margin-left: 6px">
                {{ outcomeLabel(run.outcome) }}
              </n-tag>
              <n-text depth="3" style="margin-left: 6px">
                {{ formatTime(run.timestamp) }}
              </n-text>
            </div>
            <div v-if="run.attempts.length > 0" style="margin-top: 4px; padding-left: 16px; font-family: monospace; font-size: 11px">
              <div v-for="a in run.attempts" :key="a.index">
                <n-text depth="2">#{{ a.index }} {{ a.ok ? '✓' : '✗' }} {{ a.reason || '' }}</n-text>
                <n-text depth="3" style="margin-left: 4px">{{ formatTime(a.at) }}</n-text>
              </div>
            </div>
          </div>
        </n-collapse-item>
      </n-collapse>

      <!-- 如何修复折叠区（仅在 needsFix 时显示）。锚字串 [boot-autostart-fix] -->
      <n-collapse v-if="needsFix" style="margin-top: 12px" :default-expanded-names="['fix']">
        <n-collapse-item name="fix" title="[boot-autostart-fix] 如何修复">
          <div style="font-size: 13px; line-height: 1.6">
            <p style="margin: 0 0 8px">
              当前 frp-easy <strong v-if="!status.supervised">未被系统服务管理（前台运行）</strong>
              <strong v-else-if="!status.boot_autostart">已注册但未启用开机自启</strong>。
              关机/重启后远程连接将不会自动恢复，需手动启动；
              注册为系统服务后，<strong>设备开机即可远程使用，不依赖任何用户登录</strong>。
            </p>
            <div :style="cmdBlockStyle">
              <n-text depth="2" tag="div" style="margin-bottom: 4px; font-size: 12px">Linux：</n-text>
              <code style="font-size: 12px">sudo /opt/frp-easy/scripts/install-service.sh</code>
            </div>
            <div :style="cmdBlockStyle">
              <n-text depth="2" tag="div" style="margin-bottom: 4px; font-size: 12px">Windows（管理员 PowerShell）：</n-text>
              <code style="font-size: 12px">&amp; "C:\Program Files\frp-easy\scripts\install-service.ps1"</code>
            </div>
            <n-text depth="3" tag="p" style="margin: 6px 0 0; font-size: 12px">
              注册成功后脚本会自动跑自检（systemctl is-active+is-enabled / sc.exe qc+query），
              失败 exit 4 立即可见。本卡片"刷新"按钮可重新探测。
            </n-text>
          </div>
        </n-collapse-item>
      </n-collapse>
    </div>
  </n-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  NCard, NDescriptions, NDescriptionsItem, NTag, NText,
  NCollapse, NCollapseItem, NButton, useThemeVars,
} from 'naive-ui'
import { useServiceStatus } from '../composables/useServiceStatus'
import { formatTime } from '../utils/format'
import type { AutoRestoreLastRun } from '../types'

const { status, loading, error, refresh, needsFix } = useServiceStatus()

// C2：命令块背景用主题色 code 背景，避免在浅色主题上 rgba(0,0,0,0.3) 偏暗/对比失衡。
const themeVars = useThemeVars()
const cmdBlockStyle = computed(() => ({
  background: themeVars.value.codeColor,
  padding: '8px 12px',
  borderRadius: '4px',
  margin: '6px 0',
}))

const supervisorLabel = computed(() => {
  switch (status.value?.supervisor) {
    case 'systemd':         return 'systemd'
    case 'windows-service': return 'Windows Service'
    default:                return '未监管（前台进程）'
  }
})

const supervisorTagType = computed(() => {
  return status.value?.supervised ? 'success' : 'warning'
})

const hasLastRuns = computed(() => {
  const lr = status.value?.auto_restore.last_runs
  return lr && Object.keys(lr).length > 0
})

function outcomeLabel(outcome: string): string {
  const m: Record<string, string> = {
    ok: '成功',
    exhausted: '5 次重试全失败',
    'user-initiated': '用户介入中止',
    canceled: '关停取消',
    'binary-missing': '二进制缺失',
    'config-missing': '配置缺失',
  }
  return m[outcome] || outcome
}

function outcomeTagType(outcome: string): 'success' | 'warning' | 'error' | 'default' {
  if (outcome === 'ok') return 'success'
  if (outcome === 'exhausted' || outcome === 'binary-missing' || outcome === 'config-missing') return 'error'
  return 'warning'
}
</script>

<style scoped>
.svc-status-card--warn {
  border-color: #f0a020;
  box-shadow: 0 0 0 1px #f0a020 inset;
}
.svc-status-card--ok {
  /* 默认边框即可 */
}
</style>
