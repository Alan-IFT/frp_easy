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
          二进制缺失: {{ appStore.binMissing.join(', ') }}。请将 frpc/frps 放到 frp_win/ 或 frp_linux/ 目录下。
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
  NMenu, NSpace, NText, NButton, NAlert,
  useMessage,
} from 'naive-ui'
import type { MenuOption } from 'naive-ui'
import { useAuthStore } from '../stores/auth'
import { useAppStore } from '../stores/app'

const authStore = useAuthStore()
const appStore = useAppStore()
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
</script>
