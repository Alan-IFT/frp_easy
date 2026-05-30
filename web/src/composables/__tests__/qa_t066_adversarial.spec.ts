// T-066 / dark-theme-support · QA 独立对抗测试（reproducer 从 AC 重新派生，
// 与 dev 的 useTheme.spec 假设独立——QA 红线 2）。
//
// 每条对抗用例先写失败假设（"I expect failure when…"），再断言实现存活。
// 断言全用可观察量（activeTheme===真实 darkTheme 引用 / localStorage / themeVars
// 派生值），少用 naive-ui 组件名查询（insight L45）。
//
// 注：QA 与 dev 共用受控 osThemeRef mock 范式是不可避免的（useOsTheme 须 setup 内
// 调用，唯一可控注入点），但 reproducer 的预置态/断言路径独立从 AC 派生。

import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h, nextTick, ref } from 'vue'
import { NConfigProvider, useThemeVars } from 'naive-ui'

const osThemeRef = ref<'light' | 'dark' | null>('light')

vi.mock('naive-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useOsTheme: () => osThemeRef,
  }
})

async function realDarkTheme() {
  const naive = await vi.importActual<typeof import('naive-ui')>('naive-ui')
  return naive.darkTheme
}

async function freshUseTheme() {
  vi.resetModules()
  return await import('../useTheme')
}

async function settle(n = 4) {
  for (let i = 0; i < n; i++) await nextTick()
}

beforeEach(() => {
  localStorage.clear()
  osThemeRef.value = 'light'
})

describe('QA-ADV T-066 — 反向证伪', () => {
  // AC-1：假设 = "默认偏好会意外变成暗色让浅色回归"
  it('QA-ADV-1：默认（无持久化、OS 浅）绝不生效暗色——浅色不回归', async () => {
    // I expect failure if DEFAULT_PREF 误设 dark 或 auto 分支误判 OS。
    osThemeRef.value = 'light'
    const { useTheme } = await freshUseTheme()
    const { activeTheme, isDark, pref } = useTheme()
    expect(pref.value).toBe('auto')
    expect(activeTheme.value).toBeNull() // 绝不是 darkTheme
    expect(isDark.value).toBe(false)
    // 反向证伪：即便 OS 后续报 null（不支持）也不该塌成暗色
    osThemeRef.value = null
    expect(useTheme().activeTheme.value).toBeNull()
  })

  // AC-4：假设 = "auto 模式不真正跟随 OS（写死浅色）"
  it('QA-ADV-2：auto 模式真跟随 OS——mock OS 暗必须生效 darkTheme，浅则回浅', async () => {
    // I expect failure if activeTheme 忽略 osThemeRef（写死 null）。
    const dark = await realDarkTheme()
    osThemeRef.value = 'dark'
    const { useTheme } = await freshUseTheme()
    const { activeTheme } = useTheme()
    expect(activeTheme.value).toBe(dark) // 跟随 OS 暗
    osThemeRef.value = 'light'
    expect(activeTheme.value).toBeNull() // 跟随 OS 回浅（响应式）
    osThemeRef.value = 'dark'
    expect(activeTheme.value).toBe(dark) // 再切暗，无残留
  })

  // AC-5：假设 = "手选暗色重载后丢失（退回默认）"
  it('QA-ADV-3：手选 dark 写盘后，全新实例（模拟重载）仍读回 dark 不退默认', async () => {
    // I expect failure if setPref 未持久化 或 readPref 未读回。
    const dark = await realDarkTheme()
    const first = await freshUseTheme()
    first.useTheme().setPref('dark')
    expect(localStorage.getItem('frpEasy.themePref')).toBe('dark')
    // 模拟整页重载：全新模块实例重读 localStorage
    const second = await freshUseTheme()
    const { pref, activeTheme } = second.useTheme()
    expect(pref.value).toBe('dark') // 重载保持，不退 auto
    expect(activeTheme.value).toBe(dark)
  })

  // AC-7：假设 = "localStorage 不可用时崩溃 / 切换失效"
  it('QA-ADV-4：localStorage 全程抛错（隐私模式）——构造+切换均不崩，内存降级生效', async () => {
    // I expect failure if 任一 storage 调用未被 try/catch 包裹。
    const getSpy = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('SecurityError')
    })
    const setSpy = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('SecurityError')
    })
    const dark = await realDarkTheme()
    const { useTheme } = await freshUseTheme()
    const { pref, activeTheme, setPref } = useTheme()
    expect(pref.value).toBe('auto') // 读不到→默认，不崩
    expect(() => setPref('dark')).not.toThrow() // 写抛错→内存降级，不崩
    expect(pref.value).toBe('dark')
    expect(activeTheme.value).toBe(dark) // 当次会话内切换仍生效
    getSpy.mockRestore()
    setSpy.mockRestore()
  })

  // OOS-1 边界：假设 = "顶级路由页（/login /setup /wizard）不在 AppLayout 内 →
  // 误以为它们不跟随全局主题"。验证机制：被 NConfigProvider{darkTheme} 包裹的任意
  // 子树（顶级路由页同样被 App.vue 的 NConfigProvider 包裹）useThemeVars 派生暗色值。
  it('QA-ADV-5：顶级路由页等价场景——NConfigProvider{darkTheme} 包裹的子树确实拿到暗色 themeVars', async () => {
    // I expect failure if 顶级路由页脱离全局 NConfigProvider 主题 context。
    const dark = await realDarkTheme()
    let lightBody = ''
    let darkBody = ''
    const Probe = defineComponent({
      setup() {
        const tv = useThemeVars()
        return () => h('span', { class: 'probe' }, tv.value.bodyColor)
      },
    })
    // 浅色 context
    const wLight = mount(
      defineComponent({ setup: () => () => h(NConfigProvider, { theme: null }, { default: () => h(Probe) }) }),
      { attachTo: document.body },
    )
    await settle()
    lightBody = wLight.find('.probe').text()
    wLight.unmount()
    // 暗色 context（顶级路由页被 App.vue NConfigProvider 包裹的等价场景）
    const wDark = mount(
      defineComponent({ setup: () => () => h(NConfigProvider, { theme: dark }, { default: () => h(Probe) }) }),
      { attachTo: document.body },
    )
    await settle()
    darkBody = wDark.find('.probe').text()
    wDark.unmount()
    // 反向证伪：暗色 context 下 bodyColor 必须 ≠ 浅色 context（证明主题 context 穿透子树）
    expect(darkBody).toBeTruthy()
    expect(lightBody).toBeTruthy()
    expect(darkBody).not.toBe(lightBody)
  })
})
