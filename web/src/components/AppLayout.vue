<template>
  <n-layout style="min-height: 100vh">
    <!-- T-067：顶栏窄屏不溢出——n-space wrap 允许优雅换行（FR-4），高度 auto + 最小 56px
         让换行时不裁切（min-height 取代固定 height）。 -->
    <n-layout-header bordered style="padding: 8px 16px; display: flex; align-items: center; min-height: 56px">
      <n-space align="center" :wrap="true" style="width: 100%">
        <n-text strong :style="{ fontSize: '18px', color: themeVars.primaryColor }">FRP Easy</n-text>
        <!-- T-067：版本号是非关键元素，窄屏（isNarrow）隐藏以省空间不溢出（FR-4）。 -->
        <n-text v-if="appStore.version && !isNarrow" depth="3" style="font-size: 12px">
          v{{ appStore.version }}
        </n-text>
        <div style="flex: 1" />
        <!-- 二进制缺失横幅 -->
        <n-alert
          v-if="appStore.binMissing.length > 0"
          type="warning"
          :show-icon="true"
          style="padding: 4px 12px"
        >
          <n-space align="center" :size="8">
            <span>二进制缺失: {{ appStore.binMissing.join(', ') }}。网络不便时可手动上传：</span>
        <template v-for="kind in (appStore.binMissing as Array<'frpc' | 'frps'>)" :key="kind">
              <n-space vertical :size="4" style="align-items: flex-start">
                <n-space :size="4" align="center">
                  <n-tooltip trigger="hover" placement="bottom">
                    <template #trigger>
                      <n-button
                        size="small"
                        :type="getDownloadBtnType(kind)"
                        :loading="downloaderStore.isDownloading(kind)"
                        :disabled="downloaderStore.isDownloading(kind) || downloaderStore.downloadState(kind).status === 'success'"
                        @click="handleDownload(kind)"
                      >
                        {{ getDownloadBtnLabel(kind) }}
                      </n-button>
                    </template>
                    从 GitHub Releases 自动拉取最新版（境内可能失败）
                  </n-tooltip>
                  <!-- T-027：取消下载按钮，仅在 downloading 状态显示 -->
                  <n-button
                    v-if="downloaderStore.isDownloading(kind)"
                    size="small"
                    type="error"
                    ghost
                    @click="handleCancel(kind)"
                  >
                    ✕ 取消
                  </n-button>
                  <!-- T-018 §A.3：手动上传入口，与一键下载并列；B-4 修订仅挂 AppLayout banner -->
                  <!-- T-027 FR-9：传 sibling-downloading 让 UploadBinButton 在同 kind 下载时 disabled + tooltip 引导 -->
                  <upload-bin-button
                    :kind="kind"
                    :sibling-downloading="downloaderStore.isDownloading(kind)"
                    @uploaded="handleUploaded"
                  />
                </n-space>
                <n-progress
                  v-if="downloaderStore.downloadState(kind).status === 'downloading'"
                  type="line"
                  :percentage="downloaderStore.downloadState(kind).progress"
                  :height="4"
                  :border-radius="2"
                  :show-indicator="false"
                  style="width: 100px"
                />
              </n-space>
            </template>
          </n-space>
        </n-alert>
        <n-text depth="3" style="font-size: 13px">{{ authStore.user }}</n-text>
        <!-- T-066：主题切换三态下拉（跟随系统/浅色/深色）。放"退出登录"按钮之前，
             不改其文本/位置（e2e 03-dashboard TC-05 按 name '退出登录' 点击，AC-13 保护）。
             aria-label 给无障碍名（延续 T-064 a11y 风格）。 -->
        <n-select
          :value="themePref"
          :options="themeOptions"
          size="small"
          style="width: 110px"
          aria-label="主题切换"
          @update:value="onThemeChange"
        />
        <n-button size="small" @click="handleLogout">退出登录</n-button>
      </n-space>
    </n-layout-header>

    <n-layout has-sider>
      <n-layout-sider
        bordered
        collapse-mode="width"
        :collapsed-width="64"
        :width="200"
        :collapsed="collapsed"
        show-trigger
        @collapse="collapsed = true"
        @expand="collapsed = false"
      >
        <n-menu
          v-model:value="activeKey"
          :collapsed="collapsed"
          :collapsed-width="64"
          :collapsed-icon-size="22"
          :options="menuOptions"
          @update:value="handleMenuSelect"
        />
      </n-layout-sider>

      <!-- T-067：内容区 padding 窄屏减小（24→12px）增加可用宽度（FR-8 可选优化）。 -->
      <n-layout-content :style="{ padding: isNarrow ? '12px' : '24px' }">
        <router-view />
      </n-layout-content>
    </n-layout>
  </n-layout>
</template>

<script setup lang="ts">
import { ref, computed, h, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NLayout, NLayoutHeader, NLayoutSider, NLayoutContent,
  NMenu, NSpace, NText, NButton, NAlert, NProgress, NTooltip, NSelect,
  useMessage, useThemeVars,
} from 'naive-ui'
import type { MenuOption, SelectOption } from 'naive-ui'
import { useAuthStore } from '../stores/auth'
import { useAppStore } from '../stores/app'
import { useDownloaderStore } from '../stores/downloader'
import { useTheme } from '../composables/useTheme'
import { useViewport } from '../composables/useViewport'
import UploadBinButton from './UploadBinButton.vue'

const authStore = useAuthStore()
const appStore = useAppStore()
const downloaderStore = useDownloaderStore()
const route = useRoute()
const router = useRouter()
const message = useMessage()

// T-067 responsive-layout · 02 §6
// 侧栏窄屏自动折叠：collapsed 初值 = 当前断点默认态（窄→true 折叠 / 宽→false 展开，FR-1）。
// watch(isNarrow)（非 immediate）仅在视口跨 768px 阈值时重置默认态（FR-3）。
// 手动 show-trigger（@expand/@collapse 改 collapsed）与之共存：用户在同一断点区间内手动
// 展开时 isNarrow 未变 → watch 不触发 → collapsed 保持用户设定值，不被强制收回（FR-2 不锁死）。
const { isNarrow } = useViewport()
const collapsed = ref(isNarrow.value)
watch(isNarrow, (narrow) => {
  collapsed.value = narrow
})

// T-066：主题状态层（与 App.vue 共享同一模块单例）+ themeVars 供品牌色 token 化。
const themeVars = useThemeVars()
const { pref: themePref, setPref: setThemePref } = useTheme()
const themeOptions: SelectOption[] = [
  { label: '跟随系统', value: 'auto' },
  { label: '浅色', value: 'light' },
  { label: '深色', value: 'dark' },
]
function onThemeChange(v: string | number | Array<string | number> | null) {
  // NSelect @update:value 联合类型收口到 ThemePref；setPref 自带非法值守卫。
  if (v === 'light' || v === 'dark' || v === 'auto') setThemePref(v)
}

const activeKey = computed(() => {
  const path = route.path
  if (path.startsWith('/logs/')) return path
  // T-041：/server/monitor 与 /server 同根但不同 menu item，需精确匹配
  if (path === '/server/monitor') return 'server/monitor'
  return path.replace(/^\//, '') || 'dashboard'
})

// T-064 menu-icons-and-a11y · 02 §3.1
// 折叠态（:collapsed-width="64"）仅显示图标，故 (a) 7 个顶层项字形两两互不相同
// （此前"服务端配置"与"设置"同用 ⚙ 折叠态撞车 → 误点；"设置"改用 ⚒ 消除重复），
// (b) 每个 icon span 挂 aria-label + title + role="img" 给出无障碍名，使折叠态 +
// 屏幕阅读器可区分（aria-label 主路径 / title 悬停 tooltip / role="img" 让 AT 当
// 有名图像而非逐字朗读裸字形）。取值 = 该项 label，故 server≠settings 可访问名不同。
// 不改菜单结构 / 路由 key / activeKey 计算（:122-128 /server/monitor 特判不变）。
const menuIcon = (glyph: string, name: string) =>
  h('span', { class: 'n-icon', role: 'img', 'aria-label': name, title: name }, glyph)

const menuOptions: MenuOption[] = [
  {
    label: '仪表盘',
    key: 'dashboard',
    icon: () => menuIcon('⊙', '仪表盘'),
  },
  {
    label: '代理规则',
    key: 'proxies',
    icon: () => menuIcon('⇌', '代理规则'),
  },
  {
    label: '服务端配置',
    key: 'server',
    icon: () => menuIcon('⚙', '服务端配置'),
  },
  {
    // T-041 server-monitor-page-ui：frps 运行态监控入口
    label: '服务端监控',
    key: 'server/monitor',
    icon: () => menuIcon('◉', '服务端监控'),
  },
  {
    label: '客户端配置',
    key: 'client',
    icon: () => menuIcon('↗', '客户端配置'),
  },
  {
    label: '日志',
    key: 'logs',
    icon: () => menuIcon('≡', '日志'),
    children: [
      { label: 'frpc 日志', key: '/logs/frpc' },
      { label: 'frps 日志', key: '/logs/frps' },
    ],
  },
  {
    label: '设置',
    key: 'settings',
    // T-064：原 ⚙ 与"服务端配置"重复，折叠态视觉撞车 → 改 ⚒（工具/设置语义，形态不同）
    icon: () => menuIcon('⚒', '设置'),
  },
]

function handleMenuSelect(key: string) {
  if (key.startsWith('/')) {
    void router.push(key)
  } else {
    void router.push('/' + key)
  }
}

async function handleLogout() {
  await authStore.logout()
  message.success('已退出登录')
  void router.push('/login')
}

function getDownloadBtnLabel(kind: 'frpc' | 'frps'): string {
  const state = downloaderStore.downloadState(kind)
  if (state.status === 'downloading') {
    return `下载中... ${state.progress}%`
  }
  if (state.status === 'success') {
    return '已下载'
  }
  if (state.status === 'failed') {
    return '重试'
  }
  // T-027：新增 canceled 状态显式文案
  if (state.status === 'canceled') {
    return '已取消，点击重试'
  }
  return `一键下载 ${kind}`
}

function getDownloadBtnType(kind: 'frpc' | 'frps'): 'default' | 'primary' | 'success' | 'error' | 'warning' {
  const state = downloaderStore.downloadState(kind)
  if (state.status === 'success') return 'success'
  if (state.status === 'failed') return 'error'
  // T-027：canceled 用 warning 与 failed 视觉区分（用户主动 vs 系统错误）
  if (state.status === 'canceled') return 'warning'
  return 'primary'
}

// T-027：取消下载 handler
async function handleCancel(kind: 'frpc' | 'frps') {
  try {
    await downloaderStore.cancelDownload(kind)
    message.info(`已取消 ${kind} 下载`)
  } catch {
    message.error(`取消 ${kind} 失败，请稍后再试`)
  }
}

// T-018 §A：手动上传成功后刷新 systemReady，让 binMissing 重新计算
function handleUploaded(payload: { kind: 'frpc' | 'frps' }) {
  void appStore.fetchReady()
  message.success(`${payload.kind} 上传成功，二进制状态已刷新`)
}

async function handleDownload(kind: 'frpc' | 'frps') {
  try {
    await downloaderStore.downloadBin(kind)
  } catch {
    message.error(`启动下载 ${kind} 失败`)
  }

  // Watch for completion to refresh binMissing
  const timer = setInterval(() => {
    const state = downloaderStore.downloadState(kind)
    if (state.status === 'success') {
      clearInterval(timer)
      void appStore.fetchReady()
    } else if (state.status === 'failed') {
      clearInterval(timer)
      message.error(() =>
        h('span', null, [
          `下载 ${kind} 失败：${state.error ?? '未知错误'}。请访问 `,
          h('a', {
            href: 'https://github.com/fatedier/frp/releases',
            target: '_blank',
            style: 'color: inherit; text-decoration: underline; cursor: pointer',
          }, '手动下载'),
          '。',
        ]),
      )
    }
  }, 500)
}
</script>
