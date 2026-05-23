<template>
  <div>
    <n-page-header title="代理规则" subtitle="管理 frpc 端口转发规则">
      <template #extra>
        <n-button type="primary" @click="handleAdd">新增规则 / 批量新增</n-button>
      </template>
    </n-page-header>

    <n-data-table
      :columns="columns"
      :data="groupedRows"
      :loading="proxiesStore.loading"
      :row-key="(row: TableRow) => row.key"
      style="margin-top: 16px"
    >
      <!-- T-007 AC-8(b)：空状态文案，与「新增规则」按钮文本对齐 -->
      <template #empty>
        <n-empty description="暂无代理规则，点击右上角「新增规则 / 批量新增」开始配置" />
      </template>
    </n-data-table>

    <!-- 防火墙提示（保存成功后展示） -->
    <firewall-hint :ports="firewallPorts" :proto="firewallProto" />

    <!-- 新增/编辑弹窗 -->
    <n-modal
      v-model:show="showForm"
      :title="editingProxy ? '编辑规则' : '新增规则 / 批量新增'"
      preset="card"
      style="width: 640px"
      :mask-closable="false"
    >
      <proxy-form
        ref="proxyFormRef"
        v-model="formData"
        :edit-mode="!!editingProxy"
        :existing-proxy="editingProxy"
        @update:batch-mode="(v: boolean) => (batchMode = v)"
        @update:ports-expr="(v: string) => (portsExpr = v)"
      />
      <template #action>
        <n-space justify="end">
          <n-button @click="showForm = false">取消</n-button>
          <n-button type="primary" :loading="submitting" @click="handleSubmit">
            {{ batchMode ? '批量创建' : '保存' }}
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
import { ref, h, onMounted, computed } from 'vue'
import {
  NPageHeader, NButton, NDataTable, NModal, NSpace, NTag, NEmpty,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import ProxyForm from '../components/ProxyForm.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import FirewallHint from '../components/FirewallHint.vue'
import { useProxiesStore } from '../stores/proxies'
import { extractErrorMessage } from '../api/client'
import { groupProxiesByPrefix, type GroupedProxyRow } from '../composables/useProxyGrouping'
import type { Proxy, ProxyInput, BatchProxiesRequest } from '../types'

const proxiesStore = useProxiesStore()
const message = useMessage()

const showForm = ref(false)
const showDeleteConfirm = ref(false)
const submitting = ref(false)
const editingProxy = ref<Proxy | null>(null)
const deletingProxy = ref<Proxy | null>(null)
const proxyFormRef = ref<{
  validate: () => Promise<void>
  isBatchMode: () => boolean
  getPortsExpr: () => string
  resetBatchState: () => void
} | null>(null)
const firewallPorts = ref<number[]>([])
const firewallProto = ref<'tcp' | 'udp' | 'both'>('both')

// 从子组件回传的批量状态镜像（用于按钮文案）
const batchMode = ref(false)
const portsExpr = ref('')

const defaultFormData = (): ProxyInput => ({
  name: '',
  type: 'tcp',
  localIP: '127.0.0.1',
  localPort: 80,
  enabled: true,
})

const formData = ref<ProxyInput>(defaultFormData())

function handleAdd() {
  editingProxy.value = null
  formData.value = defaultFormData()
  batchMode.value = false
  portsExpr.value = ''
  proxyFormRef.value?.resetBatchState()
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
  batchMode.value = false
  proxyFormRef.value?.resetBatchState()
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
    // T-018 §C.1：批量分支
    if (!editingProxy.value && proxyFormRef.value?.isBatchMode()) {
      const expr = proxyFormRef.value.getPortsExpr().trim()
      const req: BatchProxiesRequest = {
        basename: formData.value.name,
        type: formData.value.type,
        localIP: formData.value.localIP || '127.0.0.1',
        portsExpr: expr,
        enabled: formData.value.enabled !== false,
      }
      const res = await proxiesStore.batchCreate(req)
      message.success(`批量创建 ${res.created} 条规则成功`)
      showForm.value = false
      // 批量场景的防火墙提示：取本次创建的所有远程端口
      const ports = res.items
        .map((p) => p.remotePort)
        .filter((p): p is number => typeof p === 'number')
      if (ports.length > 0) {
        firewallPorts.value = ports
        firewallProto.value =
          req.type === 'tcp' ? 'tcp' :
          req.type === 'udp' ? 'udp' : 'both'
      }
      return
    }

    // 原有：单条新增 / 编辑
    let savedProxy: Proxy
    if (editingProxy.value) {
      savedProxy = await proxiesStore.updateProxy(editingProxy.value.id, formData.value)
      message.success('规则已更新')
    } else {
      savedProxy = await proxiesStore.createProxy(formData.value)
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

// -----------------------------------------------------------------------
// T-018 §C 折叠分组：纯视图层
// -----------------------------------------------------------------------

type TableRow = GroupedProxyRow

const groupedRows = computed<TableRow[]>(() => {
  return groupProxiesByPrefix(proxiesStore.proxies)
})

const columns = computed<DataTableColumns<TableRow>>(() => [
  {
    title: '名称',
    key: 'name',
    render: (row) => {
      if (row.kind === 'group') {
        return h('span', { style: 'font-weight: 600' },
          `${row.basename}（${row.count} 条 ${row.proto.toUpperCase()}）`)
      }
      return row.proxy.name
    },
  },
  {
    title: '类型',
    key: 'type',
    render: (row) => {
      const type = row.kind === 'group' ? row.proto : row.proxy.type
      return h(NTag, { type: 'info', size: 'small' },
        { default: () => type.toUpperCase() })
    },
  },
  {
    title: '本地地址',
    key: 'localAddr',
    render: (row) => {
      if (row.kind === 'group') {
        return `${row.localIP}:${row.portRangeText}`
      }
      return `${row.proxy.localIP}:${row.proxy.localPort}`
    },
  },
  {
    title: '远程端口/域名',
    key: 'remote',
    render: (row) => {
      if (row.kind === 'group') {
        return row.portRangeText
      }
      if (row.proxy.remotePort) return String(row.proxy.remotePort)
      if (row.proxy.customDomains?.length) return row.proxy.customDomains.join(', ')
      return '—'
    },
  },
  {
    title: '启用',
    key: 'enabled',
    render: (row) => {
      if (row.kind === 'group') {
        const allEnabled = row.proxies.every((p) => p.enabled)
        const anyEnabled = row.proxies.some((p) => p.enabled)
        const label =
          allEnabled ? '启用' :
          anyEnabled ? '部分启用' : '禁用'
        return h(NTag, {
          type: allEnabled ? 'success' : anyEnabled ? 'warning' : 'default',
          size: 'small',
        }, { default: () => label })
      }
      return h(NTag, {
        type: row.proxy.enabled ? 'success' : 'default',
        size: 'small',
      }, { default: () => row.proxy.enabled ? '启用' : '禁用' })
    },
  },
  {
    title: '操作',
    key: 'actions',
    render: (row) => {
      if (row.kind === 'group') {
        return h(NSpace, {}, {
          default: () => [
            h(NButton, {
              size: 'small',
              onClick: () => { row.expanded = !row.expanded },
            }, { default: () => row.expanded ? '收起明细' : '展开明细' }),
          ],
        })
      }
      return h(NSpace, {}, {
        default: () => [
          h(NButton, {
            size: 'small',
            onClick: () => handleEdit(row.proxy),
          }, { default: () => '编辑' }),
          h(NButton, {
            size: 'small',
            type: 'error',
            onClick: () => handleDeleteRequest(row.proxy),
          }, { default: () => '删除' }),
        ],
      })
    },
  },
])

onMounted(() => {
  void proxiesStore.fetchProxies()
})
</script>
