<template>
  <div>
    <n-page-header title="服务端配置（frps）" subtitle="配置 FRP 服务端参数" />

    <!-- T-047 A1：加载失败态（与 loading / loaded 三态互斥）。
         失败时显示 n-result + 重试，绝不留默认值让用户误当真实配置而误操作覆盖。 -->
    <n-card v-if="loadError" style="margin-top: 16px">
      <n-result
        status="error"
        title="加载服务端配置失败"
        :description="loadError"
      >
        <template #footer>
          <n-button @click="() => void loadConfig()">重试</n-button>
        </template>
      </n-result>
    </n-card>

    <!-- T-047 A1：加载中骨架（避免渲染默认值假装是真实配置） -->
    <n-card v-else-if="loading" style="margin-top: 16px">
      <n-skeleton text :repeat="6" />
    </n-card>

    <n-card v-else style="margin-top: 16px">
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
          <!-- T-058 (B)：原文案"重置" + 直接 loadConfig 会静默丢弃未保存编辑。
               改文案"重新加载"让用户预期正确；dirty 时弹确认防误丢，不 dirty 直接重载不打扰。 -->
          <n-button @click="handleReloadClick">重新加载</n-button>
          <!-- T-062 IS-5：跳服务端运行态监控页，与 ServerMonitor→Server（goServerConfig）形成双向连通。
               仅在 loaded 态 card 内（加载失败 / 加载中 card 不含此按钮，BC-8）。SPA 内 router.push（insight L17）。 -->
          <n-button text type="primary" @click="goToMonitor">查看运行态 →</n-button>
        </n-space>
      </template>
    </n-card>

    <!-- 防火墙提示（保存成功后展示）；frps bindPort 和 dashboardPort 都是 TCP -->
    <firewall-hint :ports="savedPorts" proto="tcp" />

    <!-- T-058 (B)：dirty 时确认放弃未保存编辑（复用 T-056 ConfirmDialog 范式） -->
    <confirm-dialog
      v-model:show="reloadConfirmShow"
      title="重新加载配置"
      content="将放弃当前未保存的修改并重新加载配置，确定？"
      @confirm="confirmReload"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  NPageHeader, NCard, NForm, NFormItem, NInputNumber, NInput, NSwitch,
  NSpace, NButton, NSkeleton, NResult, useMessage,
} from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import { apiGetServer, apiPutServer } from '../api/server'
import { extractErrorMessage } from '../api/client'
import PublicIpDetector from '../components/PublicIpDetector.vue'
import FirewallHint from '../components/FirewallHint.vue'
import AllowPortsEditor from '../components/AllowPortsEditor.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import type { AllowPortRange } from '../types'

const router = useRouter()
const message = useMessage()

// T-062 IS-5：跳服务端运行态监控页（SPA 内 router.push，insight L17）
function goToMonitor(): void {
  void router.push('/server/monitor')
}
const formRef = ref<FormInst | null>(null)
const saving = ref(false)
const savedPorts = ref<number[]>([])
// T-047 A1：三态。loading 初始 true（onMounted 立即拉取）；loadError 非 null = 失败态。
// loaded = !loading && !loadError。三态互斥由模板 v-if / v-else-if / v-else 保证。
const loading = ref(true)
const loadError = ref<string | null>(null)

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

// T-058 (B)：dirty 检测 = 加载时存标量快照 + 浅比较当前表单。
// T-060：dirty 已纳入 AllowPortsEditor 端口策略（消除"只改端口策略→点重新加载→静默丢弃"
// 的数据丢失路径）。比较基于规范化字符串快照，保留单向数据流范式（不引入 v-model 桥，
// insight L13）：加载时从 cfg.allowPorts 派生快照，比较时拉编辑器当前输出再规范化。
type ServerScalarForm = typeof form.value
const loadedSnapshot = ref<ServerScalarForm | null>(null)
// T-060：端口策略规范化快照（与 loadedSnapshot 同步在 loadConfig 末尾刷新）
const loadedAllowPortsSnapshot = ref<string | null>(null)
const reloadConfirmShow = ref(false)

// T-060：把端口策略列表映射成稳定字符串，消除 JSON key 顺序/格式歧义。
// single 行 → 's:N'；range 行 → 'r:A-B'；按用户顺序 join('|')。顺序+形态敏感
// （重排 / single↔range 切换均视为脏，保守判脏优于漏判丢数据）。
// 双侧（加载值 cfg.allowPorts / 编辑器 getAllowPortsInput()）用同一函数 → round-trip identity。
function normalizeAllowPorts(ranges: AllowPortRange[]): string {
  return ranges
    .map((r) => (typeof r.single === 'number' ? `s:${r.single}` : `r:${r.start ?? 0}-${r.end ?? 0}`))
    .join('|')
}

function isDirty(): boolean {
  const snap = loadedSnapshot.value
  if (snap == null) return false
  const f = form.value
  const scalarDirty =
    f.bindPort !== snap.bindPort ||
    f.authToken !== snap.authToken ||
    f.dashboardEnabled !== snap.dashboardEnabled ||
    f.dashboardPort !== snap.dashboardPort ||
    f.dashboardUser !== snap.dashboardUser ||
    f.dashboardPass !== snap.dashboardPass
  if (scalarDirty) return true
  // T-060：端口策略比较。ref 未挂载（loading/error 态）时退化为空策略比较，不抛错。
  const currentAllowPorts = normalizeAllowPorts(allowPortsEditorRef.value?.getAllowPortsInput() ?? [])
  return currentAllowPorts !== loadedAllowPortsSnapshot.value
}

function handleReloadClick() {
  // dirty 才打扰用户；不 dirty 直接重载（不弹确认）
  if (isDirty()) {
    reloadConfirmShow.value = true
  } else {
    void loadConfig()
  }
}

function confirmReload() {
  // ConfirmDialog 自身 emit update:show false 关闭弹窗；这里只负责重载
  void loadConfig()
}

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
  // T-047 B1：Dashboard 三字段补校验（启用 Dashboard 后才生效；与 bindPort 严谨度对齐）
  dashboardPort: [
    {
      type: 'number',
      validator: (_rule, value: number) => {
        if (!form.value.dashboardEnabled) return true
        if (value == null || !Number.isInteger(value) || value < 1 || value > 65535) {
          return new Error('Dashboard 端口需为 1-65535 的整数')
        }
        return true
      },
      trigger: ['input', 'blur'],
    },
  ],
  dashboardUser: [
    {
      validator: (_rule, value: string) => {
        if (!form.value.dashboardEnabled) return true
        if (!value || !value.trim()) return new Error('启用 Dashboard 时用户名必填')
        return true
      },
      trigger: ['input', 'blur'],
    },
  ],
  dashboardPass: [
    {
      validator: (_rule, value: string) => {
        if (!form.value.dashboardEnabled) return true
        if (!value) return new Error('启用 Dashboard 时密码必填')
        return true
      },
      trigger: ['input', 'blur'],
    },
  ],
}

async function loadConfig(reveal = false) {
  // T-047 A1：进入加载态。失败时不再仅弹 toast 后留默认值，而是切到 loadError 错误态。
  loading.value = true
  loadError.value = null
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
    // T-058 (B)：在 6 个标量字段赋值之后存快照，作为后续 dirty 比较基准
    loadedSnapshot.value = { ...form.value }
    // T-060：端口策略规范化快照（从 cfg.allowPorts 派生，与 initialAllowPorts 同源）。
    // 因 round-trip identity，未改动时编辑器输出规范化值应与此相等 → isDirty 判非脏。
    loadedAllowPortsSnapshot.value = normalizeAllowPorts(cfg.allowPorts ?? [])
  } catch (e) {
    loadError.value = extractErrorMessage(e, '加载配置失败')
  } finally {
    loading.value = false
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

// 暴露给测试的 handle（getExposed 范式；禁用 wrapper.vm.__testing）
defineExpose({
  __testing: {
    form,
    rules,
    loading,
    loadError,
    saving,
    loadConfig,
    handleSave,
    formRef,
    // T-058 (B)
    loadedSnapshot,
    reloadConfirmShow,
    isDirty,
    handleReloadClick,
    confirmReload,
    // T-060
    loadedAllowPortsSnapshot,
    normalizeAllowPorts,
    allowPortsEditorRef,
    // T-062 IS-5
    goToMonitor,
  },
})
</script>
