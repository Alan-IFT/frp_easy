<template>
  <n-alert
    v-if="ports.length > 0"
    type="info"
    :title="label"
    closable
    style="margin-top: 16px"
  >
    <div v-for="port in ports" :key="port" style="margin-bottom: 12px">
      <p style="margin: 0 0 6px; font-weight: 500">端口 {{ port }}</p>

      <div style="background: #f5f5f5; padding: 8px 12px; border-radius: 4px; font-family: monospace; font-size: 13px">
        <div v-for="cmd in getCommands(port)" :key="cmd" style="display: flex; align-items: center; gap: 8px; margin-bottom: 4px">
          <span style="flex: 1; word-break: break-all">{{ cmd }}</span>
          <n-button
            size="tiny"
            type="default"
            text
            @click="copyCmd(cmd)"
          >
            {{ copiedCmd === cmd ? '已复制 ✓' : '复制' }}
          </n-button>
        </div>
      </div>
    </div>

    <!-- 复制全部按钮 -->
    <div style="margin-top: 8px; text-align: right">
      <n-button size="small" @click="copyAll">
        {{ copiedAll ? '已复制全部 ✓' : '复制全部' }}
      </n-button>
    </div>
  </n-alert>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { NAlert, NButton } from 'naive-ui'

const props = withDefaults(
  defineProps<{
    ports: number[]
    label?: string
    proto?: 'tcp' | 'udp' | 'both'
  }>(),
  {
    label: '在 frps 服务器上执行以下命令开放端口：',
    proto: 'both',
  },
)

const copiedCmd = ref<string | null>(null)
const copiedAll = ref(false)

function getCommands(port: number): string[] {
  const protos: Array<'tcp' | 'udp'> =
    props.proto === 'both' ? ['tcp', 'udp'] :
    props.proto === 'tcp'  ? ['tcp']        : ['udp']

  const cmds: string[] = []
  // ufw rules first, then iptables rules (mirrors original four-command order)
  for (const p of protos) {
    cmds.push(`sudo ufw allow ${port}/${p}`)
  }
  for (const p of protos) {
    cmds.push(`sudo iptables -I INPUT -p ${p} --dport ${port} -j ACCEPT`)
  }
  return cmds
}

function getAllCommands(): string {
  return props.ports.flatMap(port => getCommands(port)).join('\n')
}

async function copyCmd(cmd: string) {
  try {
    await navigator.clipboard.writeText(cmd)
    copiedCmd.value = cmd
    setTimeout(() => {
      copiedCmd.value = null
    }, 2000)
  } catch {
    // clipboard not available
  }
}

async function copyAll() {
  try {
    await navigator.clipboard.writeText(getAllCommands())
    copiedAll.value = true
    setTimeout(() => {
      copiedAll.value = false
    }, 2000)
  } catch {
    // clipboard not available
  }
}
</script>
