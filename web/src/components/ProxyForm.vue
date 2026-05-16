<template>
  <n-form
    ref="formRef"
    :model="form"
    :rules="rules"
    label-placement="left"
    label-width="100"
    require-mark-placement="right-hanging"
  >
    <n-form-item label="规则名称" path="name">
      <n-input
        v-model:value="form.name"
        placeholder="如 ssh-forward（字母/数字/下划线/连字符，1-64字符）"
        :disabled="editMode"
      />
    </n-form-item>

    <n-form-item label="类型" path="type">
      <n-select
        v-model:value="form.type"
        :options="typeOptions"
        @update:value="handleTypeChange"
      />
    </n-form-item>

    <n-form-item label="本地 IP" path="localIP">
      <n-input v-model:value="form.localIP" placeholder="127.0.0.1" />
    </n-form-item>

    <n-form-item label="本地端口" path="localPort">
      <n-input-number
        v-model:value="form.localPort"
        :min="1"
        :max="65535"
        placeholder="1-65535"
        style="width: 100%"
      />
    </n-form-item>

    <!-- 仅 tcp/udp -->
    <n-form-item
      v-if="isTcpUdp"
      label="远程端口"
      path="remotePort"
    >
      <n-input-number
        v-model:value="form.remotePort"
        :min="1"
        :max="65535"
        placeholder="1-65535"
        style="width: 100%"
      />
    </n-form-item>

    <!-- 仅 http/https -->
    <n-form-item
      v-if="isHttpHttps"
      label="自定义域名"
      path="customDomains"
    >
      <n-dynamic-tags
        v-model:value="form.customDomains"
        :max="20"
      />
    </n-form-item>

    <n-form-item label="启用" path="enabled">
      <n-switch v-model:value="form.enabled" />
    </n-form-item>
  </n-form>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import {
  NForm, NFormItem, NInput, NInputNumber, NSelect, NSwitch, NDynamicTags,
} from 'naive-ui'
import type { FormInst, FormRules, SelectOption } from 'naive-ui'
import type { Proxy, ProxyInput } from '../types'
import { useProxyForm } from '../composables/useProxyForm'

const props = defineProps<{
  modelValue: ProxyInput
  editMode?: boolean
  existingProxy?: Proxy | null
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', val: ProxyInput): void
}>()

const formRef = ref<FormInst | null>(null)

const { form, isTcpUdp, isHttpHttps, handleTypeChange, toProxyInput, syncFromInput } = useProxyForm(
  props.modelValue,
  props.existingProxy,
)

// 通知父组件表单变更
watch(form, () => {
  emit('update:modelValue', toProxyInput())
}, { deep: true })

// 响应父组件的变更
watch(() => props.modelValue, (val) => {
  syncFromInput(val)
}, { deep: true })

const typeOptions: SelectOption[] = [
  { label: 'TCP', value: 'tcp' },
  { label: 'UDP', value: 'udp' },
  { label: 'HTTP', value: 'http' },
  { label: 'HTTPS', value: 'https' },
]

const rules: FormRules = {
  name: [
    { required: true, message: '规则名称必填', trigger: 'blur' },
    {
      validator: (_rule, value: string) => {
        if (!/^[A-Za-z0-9_-]{1,64}$/.test(value)) {
          return new Error('只允许字母、数字、下划线、连字符，1-64字符')
        }
        return true
      },
      trigger: 'blur',
    },
  ],
  localPort: [
    { required: true, type: 'number', message: '本地端口必填', trigger: ['input', 'blur'] },
    {
      type: 'number',
      validator: (_rule, value: number) => {
        if (!value || value < 1 || value > 65535) {
          return new Error('端口范围 1-65535')
        }
        return true
      },
      trigger: ['input', 'blur'],
    },
  ],
  remotePort: [
    {
      validator: (_rule, value: number | null) => {
        if (!isTcpUdp.value) return true
        if (!value || value < 1 || value > 65535) {
          return new Error('远程端口必填，范围 1-65535')
        }
        return true
      },
      trigger: ['input', 'blur'],
    },
  ],
  customDomains: [
    {
      validator: (_rule, value: string[]) => {
        if (!isHttpHttps.value) return true
        if (!value || value.length === 0) {
          return new Error('自定义域名至少填写 1 项')
        }
        for (const d of value) {
          if (!/^([A-Za-z0-9-]{1,63}\.)+[A-Za-z]{2,}$/.test(d)) {
            return new Error(`"${d}" 不是合法域名`)
          }
        }
        return true
      },
      trigger: 'change',
    },
  ],
}

// 暴露 validate 供父组件调用
defineExpose({
  validate: () => formRef.value?.validate(),
})
</script>
