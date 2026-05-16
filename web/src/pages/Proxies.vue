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
      style="margin-top: 16px"
    />

    <!-- 新増/編集モーダル -->
    <n-modal
      v-model:show="showForm"
      :title="editingProxy ? '编辑规则' : '新增规则'"
      preset="card"
      style="width: 560px"
      :mask-closable="false"
    >
      <proxy-form
        ref="proxyFormRef"
        v-model="formData"
        :edit-mode="!!editingProxy"
        :existing-proxy="editingProxy"
      />
      <template #action>
        <n-space justify="end">
          <n-button @click="showForm = false">取消</n-button>
          <n-button type="primary" :loading="submitting" @click="handleSubmit">保存</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- 削除確認ダイアログ -->
    <confirm-dialog
      v-model:show="showDeleteConfirm"
      title="确认删除"
      :content="`确定要删除规则「${deletingProxy?.name}」吗？此操作会立即让 frpc 重新加载配置。`"
      @confirm="handleDeleteConfirm"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, h, onMounted } from 'vue'
import {
  NPageHeader, NButton, NDataTable, NModal, NSpace, NTag,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import ProxyForm from '../components/ProxyForm.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import { useProxiesStore } from '../stores/proxies'
import { extractErrorMessage } from '../api/client'
import type { Proxy, ProxyInput } from '../types'

const proxiesStore = useProxiesStore()
const message = useMessage()

const showForm = ref(false)
const showDeleteConfirm = ref(false)
const submitting = ref(false)
const editingProxy = ref<Proxy | null>(null)
const deletingProxy = ref<Proxy | null>(null)
const proxyFormRef = ref<{ validate: () => Promise<void> } | null>(null)

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
    if (editingProxy.value) {
      await proxiesStore.updateProxy(editingProxy.value.id, formData.value)
      message.success('规则已更新')
    } else {
      await proxiesStore.createProxy(formData.value)
      message.success('规则已创建')
    }
    showForm.value = false
  } catch (e) {
    message.error(extractErrorMessage(e, '保存失败'))
  } finally {
    submitting.value = false
  }
}

const columns: DataTableColumns<Proxy> = [
  { title: '名称', key: 'name' },
  {
    title: '类型',
    key: 'type',
    render: (row) => h(NTag, { type: 'info', size: 'small' }, { default: () => row.type.toUpperCase() }),
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
})
</script>
