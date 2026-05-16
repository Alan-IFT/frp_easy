import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { useAppStore } from './stores/app'
import { useAuthStore } from './stores/auth'

const routes: RouteRecordRaw[] = [
  { path: '/setup', component: () => import('./pages/Setup.vue') },
  { path: '/login', component: () => import('./pages/Login.vue') },
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

  // system/ready を未取得の場合は取得する
  if (!app.ready) {
    await app.fetchReady()
  }

  // 未初期化 → /setup へ（/setup 自体は例外）
  if (!app.initialized && to.path !== '/setup') {
    return '/setup'
  }

  // 初期化済みかつ未認証 → /login へ（/login 自体は例外）
  if (app.initialized && to.path !== '/setup' && to.path !== '/login') {
    if (auth.user === null) {
      const loggedIn = await auth.checkMe()
      if (!loggedIn) {
        return '/login'
      }
    }
  }

  // 認証済みで /login や /setup にアクセス → /dashboard へ
  if (auth.user !== null && (to.path === '/login' || to.path === '/setup')) {
    return '/dashboard'
  }

  return true
})

export default router
