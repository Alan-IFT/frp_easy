// T-066 / dark-theme-support · App.vue 接线结构测试（AC-8）
//
// 验证 App.vue：(a) <n-config-provider :theme> 绑定到 useTheme 的 activeTheme，
// (b) 含 <n-global-style />。断言以"切换主题后 NConfigProvider 收到的 theme prop"
// 为主（可观察量），辅以结构存在性（NGlobalStyle）。少用组件名查询（insight L45）：
// 此处对 NConfigProvider/NGlobalStyle 的结构断言是 AC-8 的本质（接线点），不可避免。

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref, nextTick } from 'vue'
import { NConfigProvider, NGlobalStyle, darkTheme } from 'naive-ui'

const osThemeRef = ref<'light' | 'dark' | null>('light')

vi.mock('naive-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useOsTheme: () => osThemeRef,
  }
})

async function settle(n = 4) {
  for (let i = 0; i < n; i++) await nextTick()
}

beforeEach(() => {
  localStorage.clear()
  osThemeRef.value = 'light'
})

async function mountApp() {
  vi.resetModules()
  const App = (await import('../App.vue')).default
  const useThemeMod = await import('../composables/useTheme')
  const w = mount(App, {
    attachTo: document.body,
    global: {
      stubs: { 'router-view': true, RouterView: true },
    },
  })
  return { w, useTheme: useThemeMod.useTheme }
}

describe('App.vue — 主题接线（AC-8）', () => {
  it('含 <n-global-style />（让 body 背景随主题切换）', async () => {
    const { w } = await mountApp()
    await settle()
    expect(w.findComponent(NGlobalStyle).exists()).toBe(true)
    w.unmount()
  })

  it('默认 auto + OS 浅色 → NConfigProvider theme 为 null（浅色，不回归）', async () => {
    osThemeRef.value = 'light'
    const { w } = await mountApp()
    await settle()
    const cp = w.findComponent(NConfigProvider)
    expect(cp.exists()).toBe(true)
    // 浅色 = theme prop 为 null/undefined
    expect(cp.props('theme') ?? null).toBeNull()
    w.unmount()
  })

  it('切换偏好到 dark → NConfigProvider theme 变 darkTheme（响应式）', async () => {
    const { w, useTheme } = await mountApp()
    await settle()
    const { setPref } = useTheme()
    setPref('dark')
    await settle()
    const cp = w.findComponent(NConfigProvider)
    expect(cp.props('theme')).toBe(darkTheme)
    w.unmount()
  })

  it('auto + OS 暗 → NConfigProvider theme 为 darkTheme（跟随系统）', async () => {
    osThemeRef.value = 'dark'
    const { w } = await mountApp()
    await settle()
    const cp = w.findComponent(NConfigProvider)
    expect(cp.props('theme')).toBe(darkTheme)
    w.unmount()
  })
})
