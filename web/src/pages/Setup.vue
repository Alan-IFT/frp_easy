<template>
  <div style="min-height: 100vh; display: flex; align-items: center; justify-content: center; background: #f5f5f5">
    <n-card title="初始化 FRP Easy" style="width: 400px">
      <n-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-placement="top"
      >
        <n-form-item label="管理员用户名" path="username">
          <n-input v-model:value="form.username" placeholder="admin" />
        </n-form-item>
        <n-form-item label="密码" path="password">
          <n-input
            v-model:value="form.password"
            type="password"
            show-password-on="click"
            placeholder="至少12位，含字母和数字"
          />
        </n-form-item>
        <n-form-item label="确认密码" path="confirmPassword">
          <n-input
            v-model:value="form.confirmPassword"
            type="password"
            show-password-on="click"
            placeholder="再次输入密码"
          />
        </n-form-item>
      </n-form>
      <template #action>
        <n-button
          type="primary"
          block
          :loading="loading"
          @click="handleSubmit"
        >
          完成初始化
        </n-button>
      </template>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { NCard, NForm, NFormItem, NInput, NButton, useMessage } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import { useAuthStore } from '../stores/auth'
import { useAppStore } from '../stores/app'
import { extractErrorMessage } from '../api/client'

const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()
const message = useMessage()

const formRef = ref<FormInst | null>(null)
const loading = ref(false)
const form = ref({
  username: 'admin',
  password: '',
  confirmPassword: '',
})

const rules: FormRules = {
  username: [
    { required: true, message: '用户名必填', trigger: 'blur' },
    {
      validator: (_rule, value: string) => {
        if (!/^[A-Za-z0-9_-]{1,64}$/.test(value)) {
          return new Error('只允许字母、数字、下划线、连字符')
        }
        return true
      },
      trigger: 'blur',
    },
  ],
  password: [
    { required: true, message: '密码必填', trigger: 'blur' },
    {
      validator: (_rule, value: string) => {
        if (value.length < 12) return new Error('密码至少12位')
        if (!/[A-Za-z]/.test(value)) return new Error('密码必须包含字母')
        if (!/[0-9]/.test(value)) return new Error('密码必须包含数字')
        return true
      },
      trigger: 'blur',
    },
  ],
  confirmPassword: [
    { required: true, message: '请再次输入密码', trigger: 'blur' },
    {
      validator: (_rule, value: string) => {
        if (value !== form.value.password) return new Error('两次密码不一致')
        return true
      },
      trigger: 'blur',
    },
  ],
}

async function handleSubmit() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  loading.value = true
  try {
    await authStore.setup(form.value.username, form.value.password)
    await appStore.fetchReady()
    message.success('初始化成功，欢迎使用 FRP Easy！')
    void router.push('/dashboard')
  } catch (e) {
    message.error(extractErrorMessage(e, '初始化失败'))
  } finally {
    loading.value = false
  }
}
</script>
