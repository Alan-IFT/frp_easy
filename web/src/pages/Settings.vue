<template>
  <div>
    <n-page-header title="设置" subtitle="修改登录密码" />
    <n-card title="修改密码" style="margin-top: 16px; max-width: 480px">
      <n-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-placement="top"
      >
        <n-form-item label="当前密码" path="oldPassword">
          <n-input
            v-model:value="form.oldPassword"
            type="password"
            show-password-on="click"
            placeholder="输入当前密码"
          />
        </n-form-item>
        <n-form-item label="新密码" path="newPassword">
          <n-input
            v-model:value="form.newPassword"
            type="password"
            show-password-on="click"
            placeholder="至少12位，含字母和数字"
          />
        </n-form-item>
        <n-form-item label="确认新密码" path="confirmPassword">
          <n-input
            v-model:value="form.confirmPassword"
            type="password"
            show-password-on="click"
            placeholder="再次输入新密码"
          />
        </n-form-item>
      </n-form>
      <template #action>
        <n-button type="primary" :loading="saving" @click="handleSave">修改密码</n-button>
      </template>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { NPageHeader, NCard, NForm, NFormItem, NInput, NButton, useMessage } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import { apiChangePassword } from '../api/auth'
import { extractErrorMessage } from '../api/client'

const message = useMessage()
const formRef = ref<FormInst | null>(null)
const saving = ref(false)

const form = ref({
  oldPassword: '',
  newPassword: '',
  confirmPassword: '',
})

const rules: FormRules = {
  oldPassword: [
    { required: true, message: '当前密码必填', trigger: 'blur' },
  ],
  newPassword: [
    { required: true, message: '新密码必填', trigger: 'blur' },
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
    { required: true, message: '请确认新密码', trigger: 'blur' },
    {
      validator: (_rule, value: string) => {
        if (value !== form.value.newPassword) return new Error('两次密码不一致')
        return true
      },
      trigger: 'blur',
    },
  ],
}

async function handleSave() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  saving.value = true
  try {
    await apiChangePassword(form.value.oldPassword, form.value.newPassword)
    message.success('密码修改成功')
    form.value = { oldPassword: '', newPassword: '', confirmPassword: '' }
  } catch (e) {
    message.error(extractErrorMessage(e, '密码修改失败'))
  } finally {
    saving.value = false
  }
}
</script>
