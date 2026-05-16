import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { useAppStore } from './stores/app'
import { useAuthStore } from './stores/auth'
import { useWizardStore } from './stores/wizard'

const routes: RouteRecordRaw[] = [
  { path: '/setup', component: () => import('./pages/Setup.vue') },
  { path: '/login', component: () => import('./pages/Login.vue') },
  { path: '/wizard', component: () => import('./pages/Wizard.vue') },
  {
    path: '/',
    component: () => import('./components/AppLayout.vue'),
    children: [
      { path: '', redirect: '/dashboard' },
      { path: 'dashboard', component: () => import('./pages/Dashboard.vue') },
      { path: 'proxies', component: () => import('./pages/Proxies.vue') },
      { path: 'server', component: () => import('./pages/Server.vue') },
      { path: 'client', component: () => import('./pages/Client.vue') },
      { path: 'logs/frpc', component: () => import('./pages/Logs.vue'), props: { kind: 'frpc' } },
      { path: 'logs/frps', component: () => import('./pages/Logs.vue'), props: { kind: 'frps' } },
      { path: 'settings', component: () => import('./pages/Settings.vue') },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async (to) => {
  const app = useAppStore()
  const auth = useAuthStore()

  // 若尚未获取 system/ready，先获取
  if (!app.ready) {
    await app.fetchReady()
  }

  // 未初始化 → 跳转 /setup（/setup 本身除外）
  if (!app.initialized && to.path !== '/setup') {
    return '/setup'
  }

  // 已初始化但未登录 → 跳转 /login（/login 本身除外）
  if (app.initialized && to.path !== '/setup' && to.path !== '/login') {
    if (auth.user === null) {
      const loggedIn = await auth.checkMe()
      if (!loggedIn) {
        return '/login'
      }
    }
  }

  // 已登录时访问 /login 或 /setup → 跳转 /dashboard
  if (auth.user !== null && (to.path === '/login' || to.path === '/setup')) {
    return '/dashboard'
  }

  // Wizard 检查：已登录且正在导航到 /dashboard 且本 session 未检查过
  if (auth.user !== null && to.path === '/dashboard') {
    const wizard = useWizardStore()
    if (!wizard.checked) {
      await wizard.checkWizard()
      if (wizard.shouldShow) {
        return '/wizard'
      }
    }
  }

  return true
})

export default router
