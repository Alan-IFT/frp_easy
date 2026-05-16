<template>
  <n-layout style="min-height: 100vh">
    <n-layout-header bordered style="padding: 0 16px; display: flex; align-items: center; height: 56px">
      <n-space align="center" style="width: 100%">
        <n-text strong style="font-size: 18px; color: #18a058">FRP Easy</n-text>
        <n-text v-if="appStore.version" depth="3" style="font-size: 12px">
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
            <span>二进制缺失: {{ appStore.binMissing.join(', ') }}。</span>
        <template v-for="kind in (appStore.binMissing as Array<'frpc' | 'frps'>)" :key="kind">
              <n-space vertical :size="4" style="align-items: flex-start">
                <n-button
                  size="small"
                  :type="getDownloadBtnType(kind)"
                  :loading="downloaderStore.isDownloading(kind)"
                  :disabled="downloaderStore.isDownloading(kind) || downloaderStore.downloadState(kind).status === 'success'"
                  @click="handleDownload(kind)"
                >
                  {{ getDownloadBtnLabel(kind) }}
                </n-button>
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

      <n-layout-content style="padding: 24px">
        <router-view />
      </n-layout-content>
    </n-layout>
  </n-layout>
</template>

<script setup lang="ts">
import { ref, computed, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NLayout, NLayoutHeader, NLayoutSider, NLayoutContent,
  NMenu, NSpace, NText, NButton, NAlert, NProgress,
  useMessage,
} from 'naive-ui'
import type { MenuOption } from 'naive-ui'
import { useAuthStore } from '../stores/auth'
import { useAppStore } from '../stores/app'
import { useDownloaderStore } from '../stores/downloader'

const authStore = useAuthStore()
const appStore = useAppStore()
const downloaderStore = useDownloaderStore()
const route = useRoute()
const router = useRouter()
const message = useMessage()
const collapsed = ref(false)

const activeKey = computed(() => {
  const path = route.path
  if (path.startsWith('/logs/')) return path
  return path.replace(/^\//, '') || 'dashboard'
})

const menuOptions: MenuOption[] = [
  {
    label: '仪表盘',
    key: 'dashboard',
    icon: () => h('span', { class: 'n-icon' }, '⊙'),
  },
  {
    label: '代理规则',
    key: 'proxies',
    icon: () => h('span', { class: 'n-icon' }, '⇌'),
  },
  {
    label: '服务端配置',
    key: 'server',
    icon: () => h('span', { class: 'n-icon' }, '⚙'),
  },
  {
    label: '客户端配置',
    key: 'client',
    icon: () => h('span', { class: 'n-icon' }, '↗'),
  },
  {
    label: '日志',
    key: 'logs',
    icon: () => h('span', { class: 'n-icon' }, '≡'),
    children: [
      { label: 'frpc 日志', key: '/logs/frpc' },
      { label: 'frps 日志', key: '/logs/frps' },
    ],
  },
  {
    label: '设置',
    key: 'settings',
    icon: () => h('span', { class: 'n-icon' }, '⚙'),
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
    return '下载中...'
  }
  if (state.status === 'success') {
    return '已下载'
  }
  if (state.status === 'failed') {
    return '重试'
  }
  return `一键下载 ${kind}`
}

function getDownloadBtnType(kind: 'frpc' | 'frps'): 'default' | 'primary' | 'success' | 'error' {
  const state = downloaderStore.downloadState(kind)
  if (state.status === 'success') return 'success'
  if (state.status === 'failed') return 'error'
  return 'primary'
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
