<template>
  <div>
    <n-page-header title="代理规则" subtitle="管理 frpc 端口转发规则">
      <template #extra>
        <n-button type="primary" @click="handleAdd">新增规则</n-button>
      </template>
    </n-page-header>

    <n-data-table
      :columns="columns"
      :data="proxiesStore.proxies"
      :loading="proxiesStore.loading"
      :row-key="(row: Proxy) => row.id"
      style="margin-top: 16px"
    >
      <template #empty>
        <n-empty description="暂无代理规则，点击右上角「新增规则」开始配置" />
      </template>
    </n-data-table>

    <!-- 防火墙提示（保存成功后展示） -->
    <firewall-hint :ports="firewallPorts" :proto="firewallProto" />

    <!-- 新增/编辑弹窗 -->
    <n-modal
      v-model:show="showForm"
      :title="editingProxy ? '编辑规则' : '新增规则'"
      preset="card"
      style="width: 640px"
      :mask-closable="false"
    >
      <proxy-form
        ref="proxyFormRef"
        :initial-value="formData"
        :edit-mode="!!editingProxy"
        :existing-proxy="editingProxy"
      />
      <template #action>
        <n-space justify="end">
          <n-button @click="showForm = false">取消</n-button>
          <n-button type="primary" :loading="submitting" @click="handleSubmit">
            保存
          </n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- 删除确认对话框 -->
    <confirm-dialog
      v-model:show="showDeleteConfirm"
      title="确认删除"
      :content="`确定要删除规则「${deletingProxy?.name}」吗？此操作会立即让 frpc 重新加载配置。`"
      @confirm="handleDeleteConfirm"
    />
  </div>
</template>

<script setup lang="ts">
// T-042 / proxy-runtime-status-merge · 02 § 3.3
// 在 T-037 基础上叠加 runtime 列（运行状态 / 流量），调 T-041 useServerRuntime 5s 轮询。
// 继承 T-032 单向数据流：Proxies.vue 只读 runtime ref，不 v-model 绑回。
// 降级：runtime 失败 → runtime 列灰点 + "监控不可用"；配置 CRUD 通路零关联。
// SFC 自检：script 段纯逻辑行数（去 import / 注释 / interface）目标 < 200（insight L31）。

import { ref, h, computed, onMounted } from 'vue'
import {
  NPageHeader, NButton, NDataTable, NModal, NSpace, NTag, NEmpty, NTooltip,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import ProxyForm from '../components/ProxyForm.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import FirewallHint from '../components/FirewallHint.vue'
import { useProxiesStore } from '../stores/proxies'
import { useServerRuntime } from '../composables/useServerRuntime'
import { formatBytes, formatTime } from '../utils/format'
import { getProxyStatusTag } from '../utils/proxyStatus'
import { extractErrorMessage } from '../api/client'
import type { Proxy, ProxyInput, ServerRuntimeProxyStatus } from '../types'

const proxiesStore = useProxiesStore()
const message = useMessage()

const showForm = ref(false)
const showDeleteConfirm = ref(false)
const submitting = ref(false)
const editingProxy = ref<Proxy | null>(null)
const deletingProxy = ref<Proxy | null>(null)
const proxyFormRef = ref<{
  validate: () => Promise<void>
  getProxyInput: () => ProxyInput
} | null>(null)
const firewallPorts = ref<number[]>([])
const firewallProto = ref<'tcp' | 'udp' | 'both'>('both')

// T-042：runtime polling 5s（与 ServerMonitor 同节拍；composable onUnmounted 自清）
const runtime = useServerRuntime(5000)

// runtime.proxies.value.proxies 是 Record<type, Status[]>；摊平为 Map<name, Status>
// 行 render 调用 O(1) 查找；polling 每 tick 触发 computed 重算一次
const runtimeMap = computed<Map<string, ServerRuntimeProxyStatus>>(() => {
  const m = new Map<string, ServerRuntimeProxyStatus>()
  const buckets = runtime.proxies.value?.proxies ?? {}
  for (const t of Object.keys(buckets)) {
    for (const r of buckets[t] ?? []) {
      m.set(r.name, r)
    }
  }
  return m
})

// 降级判定：从未拿到 runtime 数据 + 有 error → frps 不可达
const runtimeUnavailable = computed(
  () => runtime.proxies.value === null && runtime.error.value !== null,
)

const defaultFormData = (): ProxyInput => ({
  name: '',
  type: 'tcp',
  localIP: '127.0.0.1',
  localPort: 80,
  enabled: true,
})

/**
 * T-032：仅作为 ProxyForm 的初始种子；由 handleAdd / handleEdit 在打开模态框前写入。
 * 用户编辑期间它**不更新**——最终值用 proxyFormRef.value?.getProxyInput() 取（单向数据流）。
 */
const formData = ref<ProxyInput>(defaultFormData())

function handleAdd() {
  editingProxy.value = null
  formData.value = defaultFormData()
  showForm.value = true
}

function handleEdit(proxy: Proxy) {
  editingProxy.value = proxy
  formData.value = {
    name: proxy.name,
    type: proxy.type,
    localIP: proxy.localIP,
    localPort: proxy.localPort,
    remotePort: proxy.remotePort,
    customDomains: proxy.customDomains ? [...proxy.customDomains] : [],
    enabled: proxy.enabled,
    version: proxy.version,
  }
  showForm.value = true
}

function handleDeleteRequest(proxy: Proxy) {
  deletingProxy.value = proxy
  showDeleteConfirm.value = true
}

async function handleDeleteConfirm() {
  if (!deletingProxy.value) return
  try {
    await proxiesStore.deleteProxy(deletingProxy.value.id)
    message.success('规则已删除')
    firewallPorts.value = []
    firewallProto.value = 'both'
  } catch (e) {
    message.error(extractErrorMessage(e, '删除失败'))
  } finally {
    deletingProxy.value = null
  }
}

async function handleSubmit() {
  try {
    await proxyFormRef.value?.validate()
  } catch {
    return
  }

  submitting.value = true
  try {
    // T-032：从子组件主动拉取用户编辑后的最终值；formData 仅是种子，不再实时反映输入。
    const formValue = proxyFormRef.value?.getProxyInput()
    if (!formValue) {
      message.error('表单组件未就绪')
      return
    }

    let savedProxy: Proxy
    if (editingProxy.value) {
      savedProxy = await proxiesStore.updateProxy(editingProxy.value.id, formValue)
      message.success('规则已更新')
    } else {
      savedProxy = await proxiesStore.createProxy(formValue)
      message.success('规则已创建')
    }
    showForm.value = false

    // Show firewall hint for tcp/udp proxies with a remotePort
    if ((savedProxy.type === 'tcp' || savedProxy.type === 'udp') && savedProxy.remotePort) {
      firewallPorts.value = [savedProxy.remotePort]
      firewallProto.value = savedProxy.type === 'tcp' ? 'tcp' : 'udp'
    } else {
      firewallPorts.value = []
    }
  } catch (e) {
    message.error(extractErrorMessage(e, '保存失败'))
  } finally {
    submitting.value = false
  }
}

// T-042：runtime 状态列 render（提取为 fn 让 columns 数组保持紧凑）
function renderRuntimeStatus(row: Proxy) {
  if (runtimeUnavailable.value) {
    const vis = getProxyStatusTag(null)
    return h(NTooltip, { trigger: 'hover' }, {
      trigger: () => h(NTag, { type: vis.type, size: 'small', round: true },
        { default: () => '监控不可用' }),
      default: () => 'frps 未运行 / 监控暂不可达',
    })
  }
  const r = runtimeMap.value.get(row.name)
  const vis = getProxyStatusTag(r?.status)
  const lastStart = formatTime(r?.lastStartTime)
  const tooltipText = r
    ? `状态：${vis.text}\n上次启动：${lastStart}\n当前连接：${r.curConns ?? 0}`
    : '该 proxy 未在 frps 端注册（离线）'
  return h(NTooltip, { trigger: 'hover', style: 'white-space: pre-line' }, {
    trigger: () => h(NTag, { type: vis.type, size: 'small', round: true },
      { default: () => vis.text }),
    default: () => tooltipText,
  })
}

// T-042：runtime 流量列 render
function renderRuntimeTraffic(row: Proxy) {
  if (runtimeUnavailable.value) return '—'
  const r = runtimeMap.value.get(row.name)
  if (!r) return '—'
  const text = `${formatBytes(r.todayTrafficIn)} / ${formatBytes(r.todayTrafficOut)}`
  return h(NTooltip, { trigger: 'hover' }, {
    trigger: () => text,
    default: () => `当前连接：${r.curConns ?? 0}`,
  })
}

const columns: DataTableColumns<Proxy> = [
  {
    title: '名称',
    key: 'name',
    render: (row) => row.name,
  },
  {
    title: '类型',
    key: 'type',
    render: (row) => h(NTag, { type: 'info', size: 'small' },
      { default: () => row.type.toUpperCase() }),
  },
  {
    title: '本地地址',
    key: 'localAddr',
    render: (row) => `${row.localIP}:${row.localPort}`,
  },
  {
    title: '远程端口/域名',
    key: 'remote',
    render: (row) => {
      if (row.remotePort) return String(row.remotePort)
      if (row.customDomains?.length) return row.customDomains.join(', ')
      return '—'
    },
  },
  {
    title: '启用',
    key: 'enabled',
    render: (row) => h(NTag, {
      type: row.enabled ? 'success' : 'default',
      size: 'small',
    }, { default: () => row.enabled ? '启用' : '禁用' }),
  },
  // T-042：分两列展示 runtime（不与"启用"合并，避免 insight L29 同列名不同语义源陷阱）
  {
    title: '运行状态',
    key: 'runtimeStatus',
    render: renderRuntimeStatus,
  },
  {
    title: '流量（入 / 出）',
    key: 'runtimeTraffic',
    render: renderRuntimeTraffic,
  },
  {
    title: '操作',
    key: 'actions',
    render: (row) => h(NSpace, {}, {
      default: () => [
        h(NButton, {
          size: 'small',
          onClick: () => handleEdit(row),
        }, { default: () => '编辑' }),
        h(NButton, {
          size: 'small',
          type: 'error',
          onClick: () => handleDeleteRequest(row),
        }, { default: () => '删除' }),
      ],
    }),
  },
]

onMounted(() => {
  void proxiesStore.fetchProxies()
  // T-042：启动 runtime polling；不 await（让配置表先加载）
  runtime.start()
  void runtime.refresh()
})

// 暴露给测试的 handle（与 ServerMonitor.vue 同款）
defineExpose({
  __testing: {
    runtime,
    runtimeMap,
    runtimeUnavailable,
    renderRuntimeStatus,
    renderRuntimeTraffic,
    columns,
    handleAdd,
    handleEdit,
    handleDeleteRequest,
  },
})
</script>
