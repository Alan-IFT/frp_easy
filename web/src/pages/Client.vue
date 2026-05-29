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
          <n-button @click="loadConfig()">重置</n-button>
        </n-space>
      </template>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  NPageHeader, NCard, NForm, NFormItem, NInputNumber, NInput,
  NSpace, NButton, NSkeleton, NResult, useMessage,
} from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import { apiGetClient, apiPutClient } from '../api/frpclient'
import { extractErrorMessage } from '../api/client'

const message = useMessage()
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
  } catch (e) {
    message.error(extractErrorMessage(e, '保存失败'))
  } finally {
    saving.value = false
  }
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
  },
})
</script>
