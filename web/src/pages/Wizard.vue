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
            <!-- T-058 (C)：原 v-if='both' / v-else 两分支文案完全相同，为冗余死分支。
                 外层 div v-if='frpc||both' 已控可见性，合并为单个无条件标题，零行为变化。 -->
            <n-text strong style="display: block; margin-bottom: 12px">
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

          <!-- T-062 IS-7：both 模式两端 token 均非空且不等 → 非阻断 warning。
               仅展示，不进入 handleNext 校验、不写 configError、不阻止推进（高级用户可能有意如此）。 -->
          <n-alert v-if="tokenMismatch" type="warning" style="margin-top: 12px">
            两端 token 不一致，frpc 将无法连接 frps（如非有意配置，请改为一致）
          </n-alert>

          <n-alert v-if="configError" type="error" style="margin-top: 12px">
            {{ configError }}
          </n-alert>
        </div>

        <!-- Step 3: Complete -->
        <div v-if="currentStep === 3" style="text-align: center; padding: 24px 0">
          <n-icon size="64" :color="binWarning.length > 0 ? '#f0a020' : '#18a058'">
            <svg viewBox="0 0 24 24" fill="currentColor">
              <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/>
            </svg>
          </n-icon>
          <div style="margin-top: 16px">
            <n-text strong style="font-size: 18px">配置完成！</n-text>
          </div>

          <!-- T-057：所选角色二进制全就绪 → 维持原"正在跳转"文案 + spin。 -->
          <template v-if="binWarning.length === 0">
            <div style="margin-top: 8px">
              <n-text depth="3">已保存配置并启用对应模式，现在跳转到仪表盘</n-text>
            </div>
            <n-spin v-if="completing" style="margin-top: 16px" />
            <!-- T-062 IS-1：正向下一步引导（仅 frpc/both 角色 + 二进制就绪分支）。
                 frps 纯服务端无 frpc 转发规则概念，故不展示（BC-1）。
                 缺失分支（v-else）不加此引导（BC-2，聚焦补二进制）。
                 全就绪分支会自动 push('/dashboard')；此按钮是附加的手动 push('/proxies')，
                 不阻断自动跳转（不破坏 T-057）。 -->
            <div v-if="selectedRole === 'frpc' || selectedRole === 'both'" style="margin-top: 12px">
              <n-button text type="primary" @click="goToProxies">
                下一步：前往「代理规则」添加要转发的端口 →
              </n-button>
            </div>
          </template>

          <!-- T-057：所选角色二进制缺失 → 配置已保存但不自动跳走，就地警告 + 引导 + 手动进入按钮。 -->
          <template v-else>
            <n-alert type="warning" title="配置已保存，但二进制尚未就绪" style="margin-top: 16px; text-align: left">
              所选角色的二进制（{{ binWarning.join('、') }}）尚未就绪。配置已保存并已开启自动启动，
              但二进制就绪后才能真正启动。进入仪表盘后，请用顶部横幅的「一键下载」或「手动上传」补齐，
              再启动对应进程。
            </n-alert>
            <n-button type="primary" style="margin-top: 16px" @click="goToDashboard">
              进入仪表盘
            </n-button>
          </template>
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
import { ref, computed } from 'vue'
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
import { useAppStore } from '../stores/app'
import { extractErrorMessage } from '../api/client'

const router = useRouter()
const message = useMessage()
const wizardStore = useWizardStore()
const appStore = useAppStore()

const currentStep = ref(1)
const selectedRole = ref<'frpc' | 'frps' | 'both' | ''>('')
const roleError = ref('')
const configError = ref('')
const submitting = ref(false)
const completing = ref(false)
// T-057：完成那一刻，所选角色对应但缺失的二进制 kind 列表（空 = 全就绪）。
// 用 ref 定格快照，不用 computed —— step3 已展示的警告不应随后续 binMissing 响应式漂移。
const binWarning = ref<string[]>([])

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

// T-062 IS-7：both 模式两端 token 一致性预警（非阻断）。
// 触发条件：角色为 both，且 frps/frpc token trim 后均非空，且二者不相等（BC-3 / BC-4：
// trim 后判空避免纯空白误报；与提交时 authToken || undefined 的语义对齐）。
const tokenMismatch = computed(() => {
  if (selectedRole.value !== 'both') return false
  const fs = frpsForm.value.authToken.trim()
  const fc = frpcForm.value.authToken.trim()
  return fs !== '' && fc !== '' && fs !== fc
})

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

      // Mark wizard as complete then redirect
      try {
        await wizardStore.completeWizard()
      } catch {
        // best effort
      }

      // T-057：保存配置 + 开启自动启动后，校验所选角色对应二进制是否就绪。
      // 进入向导前 router.beforeEach 已 fetch 过一次 binMissing；这里再刷新一次，
      // 覆盖境内用户在向导停留期间状态变化，并保证逻辑用最新值（fetchReady 内部吞错不抛）。
      await appStore.fetchReady()
      binWarning.value = missingForRole(selectedRole.value)

      if (binWarning.value.length > 0) {
        // 缺失：不阻断（配置已生效），但不自动跳走 —— 在 step3 就地展示警告 +
        // 让用户主动点「进入仪表盘」，避免错过提示后在仪表盘才通过红色错误态发现。
        completing.value = false
      } else {
        // 全就绪：维持原行为（success toast + 自动跳转）。
        completing.value = true
        message.success('配置已保存，正在跳转...')
        void router.push('/dashboard')
      }
    } catch (e) {
      configError.value = extractErrorMessage(e, '保存配置失败，请重试')
    } finally {
      submitting.value = false
    }
  }
}

// T-057：所选角色对应、且当前缺失的二进制 kind 集合。镜像 modePayload 的 frpc/frps/both 分支。
function missingForRole(role: 'frpc' | 'frps' | 'both' | ''): string[] {
  const need: string[] =
    role === 'both' ? ['frpc', 'frps'] : role === 'frpc' ? ['frpc'] : role === 'frps' ? ['frps'] : []
  return need.filter((k) => appStore.binMissing.includes(k))
}

// T-057：缺失分支由用户主动点「进入仪表盘」触发跳转（而非自动跳走错过提示）。
function goToDashboard(): void {
  void router.push('/dashboard')
}

// T-062 IS-1：正向下一步——前往代理规则页添加转发端口（SPA 内导航，insight L17）。
function goToProxies(): void {
  void router.push('/proxies')
}

async function handleSkip() {
  try {
    await wizardStore.completeWizard()
  } catch {
    // best effort
  }
  void router.push('/dashboard')
}

// 暴露给测试（getExposed 范式；禁用 wrapper.vm.__testing，详见 test-utils/exposed.ts / insight L45）。
defineExpose({
  __testing: {
    currentStep,
    selectedRole,
    completing,
    binWarning,
    configError,
    handleNext,
    handleSkip,
    missingForRole,
    goToDashboard,
    // T-062
    tokenMismatch,
    goToProxies,
    frpsForm,
    frpcForm,
  },
})
</script>
