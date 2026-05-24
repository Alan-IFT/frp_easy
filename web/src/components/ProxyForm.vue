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

    <!-- 常用端口预设（快速填充，非自动探测） -->
    <n-form-item v-if="!editMode" label="快速选择">
      <n-space :size="4" :wrap="true">
        <n-tag
          v-for="preset in PORT_PRESETS"
          :key="preset.label"
          checkable
          size="small"
          style="cursor: pointer"
          @click="applyPreset(preset)"
        >
          {{ preset.label }}
        </n-tag>
      </n-space>
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
/**
 * T-032: 单向数据流（initialValue prop + defineExpose getProxyInput()）。
 * 不要恢复 v-model / defineModel 双向桥——toProxyInput() 每次返回新对象引用，
 * defineModel 的循环检测对此场景无效，会再次触发 OOM 反馈环。
 * 详见 docs/features/_archived/proxy-form-vmodel-oom-fix/02_SOLUTION_DESIGN.md §7。
 */
import { ref } from 'vue'
import {
  NForm, NFormItem, NInput, NInputNumber, NSelect, NSwitch, NDynamicTags,
  NSpace, NTag,
} from 'naive-ui'
import type { FormInst, FormRules, SelectOption } from 'naive-ui'
import type { Proxy, ProxyInput } from '../types'
import { useProxyForm } from '../composables/useProxyForm'
import { PORT_PRESETS, type PortPreset } from '../composables/usePortPresets'

const props = defineProps<{
  /**
   * T-032: 仅作为「表单初始种子」使用——子组件在 setup() 阶段读 1 次，
   * 后续不再 watch；用户编辑只写本地 form，不回流到父组件。
   * 父组件需要用户最终输入时通过 defineExpose 的 getProxyInput() 主动拉取。
   */
  initialValue: ProxyInput
  editMode?: boolean
  existingProxy?: Proxy | null
}>()

const formRef = ref<FormInst | null>(null)

const { form, isTcpUdp, isHttpHttps, handleTypeChange, toProxyInput } = useProxyForm(
  props.initialValue,
  props.existingProxy,
)

// 端口预设：点击 Tag 把 port 填到 localPort / remotePort，名称留空时建议预设的 suggestedName。
function applyPreset(preset: PortPreset) {
  form.value.localPort = preset.port
  if (form.value.type === 'tcp' || form.value.type === 'udp') {
    form.value.remotePort = preset.port
  }
  if (!form.value.name) {
    form.value.name = preset.suggestedName
  }
}

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
    {
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

defineExpose({
  validate: () => formRef.value?.validate(),
  // T-032：父组件提交时主动拉取用户编辑后的最终 ProxyInput（单向数据流）。
  // 必须在 await validate() 成功之后调用，确保 form 已稳定。
  getProxyInput: (): ProxyInput => toProxyInput(),
})
</script>
