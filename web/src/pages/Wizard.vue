<template>
  <div style="min-height: 100vh; background: #f0f2f5; display: flex; justify-content: center; align-items: flex-start; padding: 40px 16px">
    <div style="width: 100%; max-width: 680px">
      <div style="text-align: center; margin-bottom: 32px">
        <n-text strong style="font-size: 24px; color: #18a058">FRP Easy</n-text>
        <div style="margin-top: 8px">
          <n-text depth="3">快速部署向导 — 几步完成初始配置</n-text>
        </div>
      </div>

      <n-card>
        <n-steps :current="currentStep" style="margin-bottom: 32px">
          <n-step title="选择角色" />
          <n-step title="填写配置" />
          <n-step title="完成" />
        </n-steps>

        <!-- Step 1: Role selection -->
        <div v-if="currentStep === 1">
          <n-text strong style="display: block; margin-bottom: 16px">请选择您的部署角色：</n-text>
          <n-radio-group v-model:value="selectedRole" name="role">
            <n-space vertical>
              <n-radio value="frpc">
                <div>
                  <div style="font-weight: 500">仅配置 frpc（客户端）</div>
                  <div style="font-size: 13px; color: #888">我需要穿透到 frps 服务器，本机没有公网 IP</div>
                </div>
              </n-radio>
              <n-radio value="frps">
                <div>
                  <div style="font-weight: 500">仅配置 frps（服务端）</div>
                  <div style="font-size: 13px; color: #888">我的机器有公网 IP，作为穿透服务端</div>
                </div>
              </n-radio>
              <n-radio value="both">
                <div>
                  <div style="font-weight: 500">两者都配置</div>
                  <div style="font-size: 13px; color: #888">同一台机器兼做服务端和客户端</div>
                </div>
              </n-radio>
            </n-space>
          </n-radio-group>

          <n-alert v-if="roleError" type="error" style="margin-top: 12px">
            {{ roleError }}
          </n-alert>
        </div>

        <!-- Step 2: Config form -->
        <div v-if="currentStep === 2">
          <!-- frps config -->
          <div v-if="selectedRole === 'frps' || selectedRole === 'both'">
            <n-text strong style="display: block; margin-bottom: 12px">
              frps 服务端配置
            </n-text>
            <n-form
              ref="frpsFormRef"
              :model="frpsForm"
              :rules="frpsRules"
              label-placement="left"
              label-width="120"
            >
              <n-form-item label="监听端口" path="bindPort">
                <n-input-number
                  v-model:value="frpsForm.bindPort"
                  :min="1"
                  :max="65535"
                  style="width: 200px"
                />
              </n-form-item>
              <n-form-item label="鉴权 Token" path="authToken">
                <n-input
                  v-model:value="frpsForm.authToken"
                  type="password"
                  show-password-on="click"
                  placeholder="可选，留空表示不启用 token 鉴权"
                  style="width: 280px"
                />
              </n-form-item>
            </n-form>
          </div>

          <!-- frpc config -->
          <div v-if="selectedRole === 'frpc' || selectedRole === 'both'" :style="selectedRole === 'both' ? 'margin-top: 24px' : ''">
            <n-text v-if="selectedRole === 'both'" strong style="display: block; margin-bottom: 12px">
              frpc 客户端配置
            </n-text>
            <n-text v-else strong style="display: block; margin-bottom: 12px">
              frpc 客户端配置
            </n-text>
            <n-form
              ref="frpcFormRef"
              :model="frpcForm"
              :rules="frpcRules"
              label-placement="left"
              label-width="120"
            >
              <n-form-item label="服务器地址" path="serverAddr">
                <n-input
                  v-model:value="frpcForm.serverAddr"
                  placeholder="frps 服务器的 IP 或主机名"
                  style="width: 320px"
                />
              </n-form-item>
              <n-form-item label="服务器端口" path="serverPort">
                <n-input-number
                  v-model:value="frpcForm.serverPort"
                  :min="1"
                  :max="65535"
                  style="width: 200px"
                />
              </n-form-item>
              <n-form-item label="鉴权 Token" path="authToken">
                <n-input
                  v-model:value="frpcForm.authToken"
                  type="password"
                  show-password-on="click"
                  placeholder="可选，与服务端 token 一致"
                  style="width: 280px"
                />
              </n-form-item>
            </n-form>
          </div>

          <n-alert v-if="configError" type="error" style="margin-top: 12px">
            {{ configError }}
          </n-alert>
        </div>

        <!-- Step 3: Complete -->
        <div v-if="currentStep === 3" style="text-align: center; padding: 24px 0">
          <n-icon size="64" color="#18a058">
            <svg viewBox="0 0 24 24" fill="currentColor">
              <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/>
            </svg>
          </n-icon>
          <div style="margin-top: 16px">
            <n-text strong style="font-size: 18px">配置完成！</n-text>
          </div>
          <div style="margin-top: 8px">
            <n-text depth="3">已保存配置并启用对应模式，现在跳转到仪表盘</n-text>
          </div>
          <n-spin v-if="completing" style="margin-top: 16px" />
        </div>

        <!-- Actions -->
        <div style="margin-top: 24px; display: flex; justify-content: space-between; align-items: center">
          <n-button @click="handleSkip" :disabled="completing">
            跳过，直接进入
          </n-button>

          <n-space>
            <n-button
              v-if="currentStep < 3"
              type="primary"
              :loading="submitting"
              @click="handleNext"
            >
              {{ currentStep === 2 ? '完成配置' : '下一步' }}
            </n-button>
          </n-space>
        </div>
      </n-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  NCard, NSteps, NStep, NRadioGroup, NRadio, NSpace, NText,
  NForm, NFormItem, NInput, NInputNumber, NButton, NAlert, NIcon, NSpin,
  useMessage,
} from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import { apiPutClient } from '../api/frpclient'
import { apiPutServer } from '../api/server'
import { apiPutMode } from '../api/mode'
import { useWizardStore } from '../stores/wizard'
import { extractErrorMessage } from '../api/client'

const router = useRouter()
const message = useMessage()
const wizardStore = useWizardStore()

const currentStep = ref(1)
const selectedRole = ref<'frpc' | 'frps' | 'both' | ''>('')
const roleError = ref('')
const configError = ref('')
const submitting = ref(false)
const completing = ref(false)

const frpsForm = ref({
  bindPort: 7000,
  authToken: '',
})

const frpcForm = ref({
  serverAddr: '',
  serverPort: 7000,
  authToken: '',
})

const frpsFormRef = ref<FormInst | null>(null)
const frpcFormRef = ref<FormInst | null>(null)

const frpsRules: FormRules = {
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

const frpcRules: FormRules = {
  serverAddr: [
    { required: true, message: '服务器地址必填', trigger: ['input', 'blur'] },
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

async function handleNext() {
  if (currentStep.value === 1) {
    roleError.value = ''
    if (!selectedRole.value) {
      roleError.value = '请先选择部署角色'
      return
    }
    currentStep.value = 2
    return
  }

  if (currentStep.value === 2) {
    configError.value = ''

    // Validate forms
    try {
      if (selectedRole.value === 'frps' || selectedRole.value === 'both') {
        await frpsFormRef.value?.validate()
      }
      if (selectedRole.value === 'frpc' || selectedRole.value === 'both') {
        await frpcFormRef.value?.validate()
      }
    } catch {
      return
    }

    submitting.value = true
    try {
      // Save configs
      if (selectedRole.value === 'frps' || selectedRole.value === 'both') {
        await apiPutServer({
          bindPort: frpsForm.value.bindPort,
          authToken: frpsForm.value.authToken || undefined,
          authMethod: frpsForm.value.authToken ? 'token' : undefined,
        })
      }
      if (selectedRole.value === 'frpc' || selectedRole.value === 'both') {
        await apiPutClient({
          serverAddr: frpcForm.value.serverAddr,
          serverPort: frpcForm.value.serverPort,
          authToken: frpcForm.value.authToken || undefined,
          authMethod: frpcForm.value.authToken ? 'token' : undefined,
        })
      }

      // Enable mode
      const modePayload = {
        frpc: selectedRole.value === 'frpc' || selectedRole.value === 'both',
        frps: selectedRole.value === 'frps' || selectedRole.value === 'both',
      }
      await apiPutMode(modePayload)

      currentStep.value = 3
      completing.value = true

      // Mark wizard as complete then redirect
      try {
        await wizardStore.completeWizard()
      } catch {
        // best effort
      }
      message.success('配置已保存，正在跳转...')
      void router.push('/dashboard')
    } catch (e) {
      configError.value = extractErrorMessage(e, '保存配置失败，请重试')
    } finally {
      submitting.value = false
    }
  }
}

async function handleSkip() {
  try {
    await wizardStore.completeWizard()
  } catch {
    // best effort
  }
  void router.push('/dashboard')
}
</script>
