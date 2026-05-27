<template>
  <div>
    <n-page-header title="服务端配置（frps）" subtitle="配置 FRP 服务端参数" />
    <n-card style="margin-top: 16px">
      <n-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-placement="left"
        label-width="140"
      >
        <!-- 公网 IP 检测 -->
        <n-form-item label=" " :show-feedback="false" style="margin-bottom: 8px">
          <public-ip-detector />
        </n-form-item>

        <n-form-item label="监听端口" path="bindPort">
          <n-input-number
            v-model:value="form.bindPort"
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
            placeholder="留空表示不启用 token 鉴权"
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

        <n-form-item label="启用 Dashboard">
          <n-switch v-model:value="form.dashboardEnabled" />
        </n-form-item>

        <template v-if="form.dashboardEnabled">
          <n-form-item label="Dashboard 端口" path="dashboardPort">
            <n-input-number
              v-model:value="form.dashboardPort"
              :min="1"
              :max="65535"
              style="width: 200px"
            />
          </n-form-item>
          <n-form-item label="Dashboard 用户名" path="dashboardUser">
            <n-input v-model:value="form.dashboardUser" style="width: 240px" />
          </n-form-item>
          <n-form-item label="Dashboard 密码" path="dashboardPass">
            <n-input
              v-model:value="form.dashboardPass"
              type="password"
              show-password-on="click"
              style="width: 240px"
            />
          </n-form-item>
        </template>

        <!-- T-040: 端口策略段 (allowPorts) -->
        <n-form-item label="端口策略" :show-feedback="false" style="margin-top: 8px">
          <allow-ports-editor :initial="initialAllowPorts" ref="allowPortsEditorRef" />
        </n-form-item>
      </n-form>
      <template #action>
        <n-space>
          <n-button type="primary" :loading="saving" @click="handleSave">保存配置</n-button>
          <n-button @click="() => void loadConfig()">重置</n-button>
        </n-space>
      </template>
    </n-card>

    <!-- 防火墙提示（保存成功后展示）；frps bindPort 和 dashboardPort 都是 TCP -->
    <firewall-hint :ports="savedPorts" proto="tcp" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  NPageHeader, NCard, NForm, NFormItem, NInputNumber, NInput, NSwitch,
  NSpace, NButton, useMessage,
} from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import { apiGetServer, apiPutServer } from '../api/server'
import { extractErrorMessage } from '../api/client'
import PublicIpDetector from '../components/PublicIpDetector.vue'
import FirewallHint from '../components/FirewallHint.vue'
import AllowPortsEditor from '../components/AllowPortsEditor.vue'
import type { AllowPortRange } from '../types'

const message = useMessage()
const formRef = ref<FormInst | null>(null)
const saving = ref(false)
const savedPorts = ref<number[]>([])

// T-040 单向数据流（insight L13）：
// initialAllowPorts 是父侧 ref，loadConfig 时写一次种子，AllowPortsEditor setup 读一次。
// 保存时通过 ref 拉子组件 getAllowPortsInput()，不引入 v-model 桥。
const allowPortsEditorRef = ref<InstanceType<typeof AllowPortsEditor> | null>(null)
const initialAllowPorts = ref<AllowPortRange[]>([])

const form = ref({
  bindPort: 7000,
  authToken: '',
  dashboardEnabled: false,
  dashboardPort: 7500,
  dashboardUser: 'admin',
  dashboardPass: '',
})

const rules: FormRules = {
  bindPort: [
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
    const cfg = await apiGetServer(reveal)
    form.value.bindPort = cfg.bindPort || 7000
    form.value.authToken = cfg.authToken ?? ''
    form.value.dashboardEnabled = cfg.dashboardEnabled ?? false
    form.value.dashboardPort = cfg.dashboardPort ?? 7500
    form.value.dashboardUser = cfg.dashboardUser ?? 'admin'
    form.value.dashboardPass = cfg.dashboardPass ?? ''
    // T-040 种子：AllowPortsEditor 在 setup 时读一次此 ref（不 watch）
    initialAllowPorts.value = cfg.allowPorts ?? []
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

  // T-040：端口策略前端守门。任一行非法 → 不发 PUT。
  // 后端 ValidateFrpsAllowPorts 仍是真值源（前端绕过场景由后端 422 兜底）。
  if (allowPortsEditorRef.value?.hasValidationError()) {
    message.error('端口策略存在非法项，请修复后再保存')
    return
  }
  const allowPorts = allowPortsEditorRef.value?.getAllowPortsInput() ?? []

  saving.value = true
  try {
    await apiPutServer({
      bindPort: form.value.bindPort,
      authToken: form.value.authToken || undefined,
      authMethod: form.value.authToken ? 'token' : undefined,
      dashboardEnabled: form.value.dashboardEnabled,
      dashboardPort: form.value.dashboardEnabled ? form.value.dashboardPort : undefined,
      dashboardUser: form.value.dashboardEnabled ? form.value.dashboardUser : undefined,
      dashboardPass: form.value.dashboardEnabled ? form.value.dashboardPass : undefined,
      allowPorts: allowPorts.length > 0 ? allowPorts : undefined,
    })
    message.success('服务端配置已保存（重启 frps 后生效）')

    // Build the list of ports to show firewall hint
    const ports: number[] = [form.value.bindPort]
    if (form.value.dashboardEnabled && form.value.dashboardPort) {
      ports.push(form.value.dashboardPort)
    }
    savedPorts.value = ports
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
