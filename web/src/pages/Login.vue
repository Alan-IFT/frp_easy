<template>
  <div style="min-height: 100vh; display: flex; align-items: center; justify-content: center; background: #f5f5f5">
    <n-card title="登录 FRP Easy" style="width: 360px">
      <n-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-placement="top"
        @keyup.enter="handleLogin"
      >
        <n-form-item label="用户名" path="username">
          <n-input v-model:value="form.username" placeholder="admin" />
        </n-form-item>
        <n-form-item label="密码" path="password">
          <n-input
            v-model:value="form.password"
            type="password"
            show-password-on="click"
            placeholder="密码"
          />
        </n-form-item>
      </n-form>

      <!-- 速率限制カウントダウン -->
      <n-alert v-if="retryAfter > 0" type="error" style="margin-bottom: 12px">
        登录尝试过多，请 {{ retryAfter }} 秒后重试
      </n-alert>

      <template #action>
        <n-button
          type="primary"
          block
          :loading="loading"
          :disabled="retryAfter > 0"
          @click="handleLogin"
        >
          登录
        </n-button>
      </template>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { NCard, NForm, NFormItem, NInput, NButton, NAlert, useMessage } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import axios from 'axios'
import { useAuthStore } from '../stores/auth'
import { useAppStore } from '../stores/app'

const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()
const message = useMessage()

const formRef = ref<FormInst | null>(null)
const loading = ref(false)
const retryAfter = ref(0)
let countdownTimer: ReturnType<typeof setInterval> | null = null

const form = ref({ username: '', password: '' })

const rules: FormRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
}

function startCountdown(seconds: number) {
  retryAfter.value = seconds
  if (countdownTimer) clearInterval(countdownTimer)
  countdownTimer = setInterval(() => {
    retryAfter.value -= 1
    if (retryAfter.value <= 0) {
      retryAfter.value = 0
      if (countdownTimer) clearInterval(countdownTimer)
    }
  }, 1000)
}

async function handleLogin() {
  if (retryAfter.value > 0) return
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  loading.value = true
  try {
    await authStore.login(form.value.username, form.value.password)
    await appStore.fetchReady()
    message.success('登录成功')
    void router.push('/dashboard')
  } catch (e: unknown) {
    if (axios.isAxiosError(e)) {
      if (e.response?.status === 429) {
        const retryAfterHeader = e.response.headers['retry-after']
        const seconds = retryAfterHeader ? parseInt(String(retryAfterHeader), 10) : 60
        startCountdown(isNaN(seconds) ? 60 : seconds)
        message.error('登录尝试过多，请稍后再试')
      } else {
        message.error(e.response?.data?.error?.message ?? '用户名或密码错误')
      }
    } else {
      message.error('登录失败，请稍后重试')
    }
  } finally {
    loading.value = false
  }
}

onUnmounted(() => {
  if (countdownTimer) clearInterval(countdownTimer)
})
</script>
