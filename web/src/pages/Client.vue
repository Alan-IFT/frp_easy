<template>
  <div>
    <n-page-header title="客户端配置（frpc）" subtitle="配置 frpc 连接到服务端的参数" />

    <!-- T-047 A1：加载失败态（与 loading / loaded 三态互斥）。
         失败时显示 n-result + 重试，绝不留默认值让用户误当真实配置而误操作覆盖。 -->
    <n-card v-if="loadError" style="margin-top: 16px">
      <n-result
        status="error"
        title="加载客户端配置失败"
        :description="loadError"
      >
        <template #footer>
          <n-button @click="() => void loadConfig()">重试</n-button>
        </template>
      </n-result>
    </n-card>

    <!-- T-047 A1：加载中骨架（避免渲染默认值假装是真实配置） -->
    <n-card v-else-if="loading" style="margin-top: 16px">
      <n-skeleton text :repeat="4" />
    </n-card>

    <n-card v-else style="margin-top: 16px">
      <n-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-placement="left"
        label-width="140"
      >
        <n-form-item label="服务端地址" path="serverAddr">
          <n-input
            v-model:value="form.serverAddr"
            placeholder="如 example.com 或 1.2.3.4"
            style="width: 300px"
          />
        </n-form-item>

        <n-form-item label="服务端端口" path="serverPort">
          <n-input-number
            v-model:value="form.serverPort"
            :min="1"
            :max="65535"
            style="width: 200px"
          />
        </n-form-item>

        <n-form-item label="鉴权 Token" path="authToken">
          <n-input
            v-model:value="form.authToken"
            type="password"
            show-password-on="click"
            placeholder="与服务端保持一致，留空则不启用"
            style="width: 360px"
          />
          <n-button
            size="small"
            style="margin-left: 8px"
            @click="loadReveal"
          >
            查看明文
          </n-button>
        </n-form-item>
      </n-form>
      <template #action>
        <n-space>
          <n-button type="primary" :loading="saving" @click="handleSave">保存配置</n-button>
          <!-- T-058 (B)：原文案"重置" + 直接 loadConfig 会静默丢弃未保存编辑。
               改文案"重新加载"；dirty 时弹确认防误丢，不 dirty 直接重载不打扰。 -->
          <n-button @click="handleReloadClick">重新加载</n-button>
        </n-space>
      </template>
    </n-card>

    <!-- T-062 IS-2：保存成功后正向下一步引导。仅 handleSave 成功置 showNextStepHint=true，
         失败（catch）不显示（BC-7）。SPA 内导航 router.push（insight L17）。 -->
    <n-alert
      v-if="showNextStepHint"
      type="success"
      title="客户端配置已保存"
      style="margin-top: 16px"
    >
      <n-button text type="primary" @click="goToProxies">
        下一步：前往「代理规则」添加要转发的端口 →
      </n-button>
    </n-alert>

    <!-- T-058 (B)：dirty 时确认放弃未保存编辑（复用 T-056 ConfirmDialog 范式） -->
    <confirm-dialog
      v-model:show="reloadConfirmShow"
      title="重新加载配置"
      content="将放弃当前未保存的修改并重新加载配置，确定？"
      @confirm="confirmReload"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  NPageHeader, NCard, NForm, NFormItem, NInputNumber, NInput,
  NSpace, NButton, NSkeleton, NResult, NAlert, useMessage,
} from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import { apiGetClient, apiPutClient } from '../api/frpclient'
import { extractErrorMessage } from '../api/client'
import ConfirmDialog from '../components/ConfirmDialog.vue'

const router = useRouter()
const message = useMessage()
// T-062 IS-2：保存成功后显示「前往代理规则」引导（仅 handleSave 成功置 true）
const showNextStepHint = ref(false)
const formRef = ref<FormInst | null>(null)
const saving = ref(false)
// T-047 A1：三态。loading 初始 true；loadError 非 null = 失败态；loaded = !loading && !loadError。
const loading = ref(true)
const loadError = ref<string | null>(null)

const form = ref({
  serverAddr: '',
  serverPort: 7000,
  authToken: '',
})

// T-058 (B)：dirty 检测用"加载时存一份标量字段快照 + 浅比较当前表单"。
type ClientScalarForm = typeof form.value
const loadedSnapshot = ref<ClientScalarForm | null>(null)
const reloadConfirmShow = ref(false)

function isDirty(): boolean {
  const snap = loadedSnapshot.value
  if (snap == null) return false
  const f = form.value
  return (
    f.serverAddr !== snap.serverAddr ||
    f.serverPort !== snap.serverPort ||
    f.authToken !== snap.authToken
  )
}

function handleReloadClick() {
  // dirty 才打扰用户；不 dirty 直接重载（不弹确认）
  if (isDirty()) {
    reloadConfirmShow.value = true
  } else {
    void loadConfig()
  }
}

function confirmReload() {
  void loadConfig()
}

const rules: FormRules = {
  serverAddr: [
    { required: true, message: '服务端地址必填', trigger: 'blur' },
  ],
  serverPort: [
    {
      type: 'number',
      validator: (_rule, value: number) => {
        if (!value || value < 1 || value > 65535) return new Error('端口范围 1-65535')
        return true
      },
      trigger: ['input', 'blur'],
    },
  ],
}

async function loadConfig(reveal = false) {
  // T-047 A1：进入加载态。失败切到 loadError 错误态，不再仅弹 toast 后留默认值。
  loading.value = true
  loadError.value = null
  try {
    const cfg = await apiGetClient(reveal)
    form.value.serverAddr = cfg.serverAddr ?? ''
    form.value.serverPort = cfg.serverPort || 7000
    form.value.authToken = cfg.authToken ?? ''
    // T-058 (B)：在 3 个标量字段赋值之后存快照，作为后续 dirty 比较基准
    loadedSnapshot.value = { ...form.value }
  } catch (e) {
    loadError.value = extractErrorMessage(e, '加载配置失败')
  } finally {
    loading.value = false
  }
}

async function loadReveal() {
  await loadConfig(true)
}

async function handleSave() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  saving.value = true
  try {
    await apiPutClient({
      serverAddr: form.value.serverAddr,
      serverPort: form.value.serverPort,
      authToken: form.value.authToken || undefined,
      authMethod: form.value.authToken ? 'token' : undefined,
    })
    message.success('客户端配置已保存（重启 frpc 后生效）')
    // T-062 IS-2：保存成功后展示正向下一步引导（失败 catch 不置，BC-7）
    showNextStepHint.value = true
  } catch (e) {
    message.error(extractErrorMessage(e, '保存失败'))
  } finally {
    saving.value = false
  }
}

// T-062 IS-2：前往代理规则页（SPA 内导航 router.push，insight L17）
function goToProxies(): void {
  void router.push('/proxies')
}

onMounted(() => {
  void loadConfig()
})

// 暴露给测试的 handle（getExposed 范式；禁用 wrapper.vm.__testing）
defineExpose({
  __testing: {
    form,
    loading,
    loadError,
    saving,
    loadConfig,
    handleSave,
    formRef,
    // T-058 (B)
    loadedSnapshot,
    reloadConfirmShow,
    isDirty,
    handleReloadClick,
    confirmReload,
    // T-062 IS-2
    showNextStepHint,
    goToProxies,
  },
})
</script>
