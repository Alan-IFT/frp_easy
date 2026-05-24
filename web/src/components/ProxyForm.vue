<template>
  <n-form
    ref="formRef"
    :model="form"
    :rules="rules"
    label-placement="left"
    label-width="100"
    require-mark-placement="right-hanging"
  >
    <!-- T-018 §C.1：批量模式开关；仅在新增模式下显示（编辑时无意义） -->
    <n-form-item v-if="!editMode" label="批量模式">
      <n-space :size="8" align="center">
        <n-switch v-model:value="batchMode" :disabled="!canUseBatch" />
        <n-text depth="3" style="font-size: 12px">
          {{ batchMode
            ? '批量：用端口表达式一次创建多条规则（仅 TCP/UDP）'
            : '单条：原有逐条新增模式' }}
        </n-text>
      </n-space>
    </n-form-item>

    <n-form-item :label="batchMode ? '规则前缀' : '规则名称'" path="name">
      <n-input
        v-model:value="form.name"
        :placeholder="batchMode
          ? '如 web（最终生成 web-6000、web-6001…，1-58 字符）'
          : '如 ssh-forward（字母/数字/下划线/连字符，1-64字符）'"
        :disabled="editMode"
      />
    </n-form-item>

    <n-form-item label="类型" path="type">
      <n-select
        v-model:value="form.type"
        :options="batchMode ? batchTypeOptions : typeOptions"
        @update:value="handleTypeChange"
      />
    </n-form-item>

    <!-- T-018 §C.2：常用端口预设 Tag 列表 -->
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

    <!-- 单条模式：本地端口 + 探测按钮 -->
    <n-form-item v-if="!batchMode" label="本地端口" path="localPort">
      <n-space :size="8" align="center" style="width: 100%">
        <n-input-number
          v-model:value="form.localPort"
          :min="1"
          :max="65535"
          placeholder="1-65535"
          style="width: 200px"
        />
        <n-button size="small" :loading="probing" :disabled="!form.localPort" @click="handleProbe">
          探测可用性
        </n-button>
        <n-tag
          v-if="probeStatus !== 'idle'"
          :type="probeStatus === 'ok' ? 'success' : 'error'"
          size="small"
        >
          {{ probeText }}
        </n-tag>
      </n-space>
    </n-form-item>

    <!-- 批量模式：端口表达式 -->
    <n-form-item v-if="batchMode" label="端口表达式" path="portsExpr">
      <n-space vertical :size="4" style="width: 100%">
        <n-input
          v-model:value="portsExpr"
          placeholder="如 6000-6010,7000（本地端口与远程端口 1:1）"
        />
        <n-text depth="3" style="font-size: 12px">
          支持范围（6000-6010）、列表（22,80,443）、混合（6000-6010,7000）；单次最多 32 个端口。
        </n-text>
      </n-space>
    </n-form-item>

    <!-- 单条模式 + tcp/udp：远程端口 -->
    <n-form-item
      v-if="!batchMode && isTcpUdp"
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

    <!-- 仅 http/https（不允许在批量模式启用） -->
    <n-form-item
      v-if="!batchMode && isHttpHttps"
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
 * 详见 docs/features/proxy-form-vmodel-oom-fix/02_SOLUTION_DESIGN.md §7（归档后路径 _archived/）；
 * 或直接 grep T-032 02 文档 §7 / §10 R-6。
 *
 * 保留 update:batchMode / update:portsExpr emit：它们是子→父单向通知（不构成
 * 反馈环），父组件按钮文案需要响应式镜像；非 modelValue 同类反模式。
 * 详见 02 §10.1 决策点 / §12 第 2 条。
 */
import { ref, watch, computed } from 'vue'
import {
  NForm, NFormItem, NInput, NInputNumber, NSelect, NSwitch, NDynamicTags,
  NSpace, NTag, NButton, NText,
  useMessage,
} from 'naive-ui'
import type { FormInst, FormRules, SelectOption } from 'naive-ui'
import type { Proxy, ProxyInput } from '../types'
import { useProxyForm } from '../composables/useProxyForm'
import { PORT_PRESETS, type PortPreset } from '../composables/usePortPresets'
import { apiProbePorts } from '../api/system'
import { extractErrorMessage } from '../api/client'

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

const emit = defineEmits<{
  (e: 'update:batchMode', val: boolean): void
  (e: 'update:portsExpr', val: string): void
}>()

const formRef = ref<FormInst | null>(null)
const message = useMessage()

const { form, isTcpUdp, isHttpHttps, handleTypeChange, toProxyInput } = useProxyForm(
  props.initialValue,
  props.existingProxy,
)

// -----------------------------------------------------------------------
// T-018 §C.1 批量模式相关本地状态
//   仅在新增模式有效；编辑模式下隐藏开关。
// -----------------------------------------------------------------------

const batchMode = ref(false)
const portsExpr = ref('')

// http/https 走域名，批量端口无意义；切到 http/https 时自动关闭 batch
const canUseBatch = computed(() => form.value.type === 'tcp' || form.value.type === 'udp')

watch(canUseBatch, (ok) => {
  if (!ok && batchMode.value) {
    batchMode.value = false
  }
})

watch(batchMode, (val) => {
  emit('update:batchMode', val)
})

// 暴露 portsExpr 也走 v-model:portsExpr 以便父组件读取
watch(portsExpr, (v) => {
  emit('update:portsExpr', v)
})

// portsExpr 用独立的 ref（不挂到 form 上）；template 内 v-model:value="portsExpr"
// 直接绑定本组件的 ref。原 form 字段维持现状，避免污染 useProxyForm 的契约。

// -----------------------------------------------------------------------
// T-018 §C.3 单端口探测
// -----------------------------------------------------------------------

const probing = ref(false)
const probeStatus = ref<'idle' | 'ok' | 'fail'>('idle')
const probeText = ref('')

async function handleProbe() {
  const port = form.value.localPort ?? 0
  if (!port || port < 1 || port > 65535) {
    message.error('请先填入合法的本地端口')
    return
  }
  probing.value = true
  probeStatus.value = 'idle'
  probeText.value = ''
  try {
    const res = await apiProbePorts([port])
    const r = res.results?.[0]
    if (!r) {
      probeStatus.value = 'fail'
      probeText.value = '未返回探测结果'
      return
    }
    if (r.available) {
      probeStatus.value = 'ok'
      probeText.value = `${r.port} 可用`
    } else {
      probeStatus.value = 'fail'
      probeText.value = `${r.port} ${reasonText(r.reason)}`
    }
  } catch (e) {
    probeStatus.value = 'fail'
    probeText.value = extractErrorMessage(e, '探测失败')
  } finally {
    probing.value = false
  }
}

function reasonText(reason: string): string {
  switch (reason) {
    case 'privileged': return '为特权端口（<1024），需 root/Admin 才能绑定'
    case 'in_use':     return '已被占用'
    case 'invalid':    return '非法'
    default:           return reason ? `不可用：${reason}` : '不可用'
  }
}

// 端口变化时清除上次探测结果
watch(() => form.value.localPort, () => {
  probeStatus.value = 'idle'
  probeText.value = ''
})

// -----------------------------------------------------------------------
// T-018 §C.2 预设 Tag 点击逻辑
// -----------------------------------------------------------------------

function applyPreset(preset: PortPreset) {
  if (batchMode.value) {
    // 批量模式：把 port 追加到 portsExpr，逗号拼接，去重
    const existing = portsExpr.value
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean)
    if (!existing.includes(String(preset.port))) {
      existing.push(String(preset.port))
    }
    portsExpr.value = existing.join(',')
  } else {
    // 单条模式：填 localPort + remotePort + 建议 name
    form.value.localPort = preset.port
    if (form.value.type === 'tcp' || form.value.type === 'udp') {
      form.value.remotePort = preset.port
    }
    if (!form.value.name) {
      form.value.name = preset.suggestedName
    }
  }
}

// -----------------------------------------------------------------------
// 类型选项
// -----------------------------------------------------------------------

const typeOptions: SelectOption[] = [
  { label: 'TCP', value: 'tcp' },
  { label: 'UDP', value: 'udp' },
  { label: 'HTTP', value: 'http' },
  { label: 'HTTPS', value: 'https' },
]

// 批量模式下 type 仅支持 tcp/udp（http/https 走域名，批量无意义）
const batchTypeOptions: SelectOption[] = [
  { label: 'TCP', value: 'tcp' },
  { label: 'UDP', value: 'udp' },
]

// -----------------------------------------------------------------------
// 校验规则
// -----------------------------------------------------------------------

const rules: FormRules = {
  name: [
    {
      required: true,
      message: () => batchMode.value ? '规则前缀必填' : '规则名称必填',
      trigger: 'blur',
    },
    {
      validator: (_rule, value: string) => {
        if (batchMode.value) {
          // batch basename：长度 ≤ 58（留 6 字符给 -65535 后缀）
          if (!/^[A-Za-z0-9_-]{1,58}$/.test(value)) {
            return new Error('前缀只允许字母/数字/下划线/连字符，1-58 字符')
          }
        } else {
          if (!/^[A-Za-z0-9_-]{1,64}$/.test(value)) {
            return new Error('只允许字母、数字、下划线、连字符，1-64字符')
          }
        }
        return true
      },
      trigger: 'blur',
    },
  ],
  localPort: [
    {
      validator: (_rule, value: number) => {
        if (batchMode.value) return true // 批量模式不用 localPort
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
        if (batchMode.value) return true
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
        if (batchMode.value) return true
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
  portsExpr: [
    {
      validator: (_rule, _value) => {
        if (!batchMode.value) return true
        const v = portsExpr.value.trim()
        if (!v) return new Error('端口表达式必填')
        // 简单语法预校验（详细校验由后端 portrange 包负责）
        if (!/^[\d,\s-]+$/.test(v)) {
          return new Error('端口表达式仅含数字、逗号、减号；如 6000-6010,7000')
        }
        return true
      },
      trigger: ['input', 'blur'],
    },
  ],
}

// 暴露 validate + 批量字段供父组件读取
defineExpose({
  validate: () => formRef.value?.validate(),
  isBatchMode: () => batchMode.value,
  getPortsExpr: () => portsExpr.value,
  // 让父组件在切换 add/edit 弹窗时重置批量状态
  resetBatchState: () => {
    batchMode.value = false
    portsExpr.value = ''
    probeStatus.value = 'idle'
    probeText.value = ''
  },
  // T-032：父组件提交时主动拉取用户编辑后的最终 ProxyInput（单向数据流）。
  // 必须在 await validate() 成功之后调用，确保 form 已稳定。
  getProxyInput: (): ProxyInput => toProxyInput(),
})
</script>
