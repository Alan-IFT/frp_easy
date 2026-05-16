<template>
  <div>
    <n-page-header title="客户端配置（frpc）" subtitle="配置 frpc 连接到服务端的参数" />
    <n-card style="margin-top: 16px">
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
  NSpace, NButton, useMessage,
} from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import { apiGetClient, apiPutClient } from '../api/frpclient'
import { extractErrorMessage } from '../api/client'

const message = useMessage()
const formRef = ref<FormInst | null>(null)
const saving = ref(false)

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
  try {
    const cfg = await apiGetClient(reveal)
    form.value.serverAddr = cfg.serverAddr ?? ''
    form.value.serverPort = cfg.serverPort || 7000
    form.value.authToken = cfg.authToken ?? ''
  } catch (e) {
    message.error(extractErrorMessage(e, '加载配置失败'))
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
</script>
