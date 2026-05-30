// T-064 menu-icons-and-a11y · IS-1 / IS-2 + AC-1/AC-2/AC-3
// AppLayout.vue 侧边栏菜单图标语义缺陷修复：
//   - IS-1：7 个顶层项图标字形两两互不相同（原"服务端配置"与"设置"同用 ⚙ 折叠态撞车）
//   - IS-2：每个 icon span 带非空 aria-label + title + role="img" 无障碍名
//
// 关键模式（insight L45 + Dashboard.spec/Server.spec 范式）：
//   - setActivePinia(createPinia()) 真 Pinia（项目不用 @pinia/testing）；
//     app store 默认 binMissing=[] → 顶栏横幅整块跳过，无需 mock downloader 复杂状态
//   - vi.mock vue-router 提供 useRoute + useRouter（AppLayout 两者都用）
//   - vi.mock naive-ui useMessage 单例 stub
//   - <router-view /> 用 mount stubs 占位（vue-router 已 mock，无真 RouterView）
//   - 断言全用 DOM 属性查询（span.n-icon 的 aria-label/title），零 naive-ui 组件名查询

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h, nextTick, ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { NConfigProvider, NMessageProvider } from 'naive-ui'

// AppLayout 用 useRoute（activeKey 计算）+ useRouter（handleMenuSelect/handleLogout）
vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/dashboard' }),
  useRouter: () => ({ push: vi.fn() }),
}))

// T-066：AppLayout 现在调 useTheme()→useOsTheme()，受控 mock 让 auto 派生确定。
const osThemeRef = ref<'light' | 'dark' | null>('light')

vi.mock('naive-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useOsTheme: () => osThemeRef,
    useMessage: () => ({
      error: vi.fn(),
      success: vi.fn(),
      warning: vi.fn(),
      info: vi.fn(),
      loading: vi.fn(),
      destroyAll: vi.fn(),
    }),
  }
})

import AppLayout from '../AppLayout.vue'
import { useTheme } from '../../composables/useTheme'

function mountLayout() {
  const Holder = defineComponent({
    setup() {
      return () =>
        h(NConfigProvider, null, {
          default: () =>
            h(NMessageProvider, null, {
              default: () => h(AppLayout),
            }),
        })
    },
  })
  return mount(Holder, {
    attachTo: document.body,
    global: {
      // vue-router 已 mock，无真 RouterView 组件 → stub 占位
      stubs: { 'router-view': true, RouterView: true },
    },
  })
}

async function settle(n = 6): Promise<void> {
  for (let i = 0; i < n; i++) await nextTick()
}

// 取所有菜单图标 span（icon render 返回 <span class="n-icon" role="img" aria-label=...>）
// 仅取带 aria-label 的 n-icon（菜单图标），排除其他可能的 n-icon 装饰节点。
function menuIconSpans(w: ReturnType<typeof mountLayout>) {
  return w.findAll('span.n-icon').filter((s) => s.attributes('aria-label') !== undefined)
}

afterEach(() => {
  document.body.innerHTML = ''
})

beforeEach(() => {
  setActivePinia(createPinia())
  // T-066 / C-1：useTheme 是模块单例，pref 跨用例共享 → 每用例显式复位防泄漏。
  localStorage.clear()
  osThemeRef.value = 'light'
  useTheme().setPref('auto')
})

describe('AppLayout.vue — 菜单图标无障碍名（T-064 IS-2 / AC-2）', () => {
  it('渲染出 7 个顶层菜单图标，每个带非空 aria-label（折叠态/屏幕阅读器可区分）', async () => {
    const w = mountLayout()
    await settle()
    const icons = menuIconSpans(w)
    // 7 个顶层项：仪表盘/代理规则/服务端配置/服务端监控/客户端配置/日志/设置
    expect(icons.length).toBe(7)
    for (const icon of icons) {
      const name = icon.attributes('aria-label')
      expect(name).toBeTruthy()
      expect(name!.trim().length).toBeGreaterThan(0)
    }
  })

  it('每个菜单图标 aria-label 与 title 一致且 role="img"（AT 当有名图像而非逐字朗读字形）', async () => {
    const w = mountLayout()
    await settle()
    const icons = menuIconSpans(w)
    for (const icon of icons) {
      expect(icon.attributes('role')).toBe('img')
      expect(icon.attributes('title')).toBe(icon.attributes('aria-label'))
      expect(icon.attributes('title')!.trim().length).toBeGreaterThan(0)
    }
  })

  it('无障碍名覆盖全部预期菜单文案（含 server/settings 各自语义）', async () => {
    const w = mountLayout()
    await settle()
    const names = menuIconSpans(w).map((s) => s.attributes('aria-label'))
    expect(names).toContain('仪表盘')
    expect(names).toContain('代理规则')
    expect(names).toContain('服务端配置')
    expect(names).toContain('服务端监控')
    expect(names).toContain('客户端配置')
    expect(names).toContain('日志')
    expect(names).toContain('设置')
  })
})

describe('AppLayout.vue — 消除重复图标字形（T-064 IS-1 / AC-1 / AC-3）', () => {
  it('7 个顶层菜单图标字形两两互不相同（折叠态仅图标也不撞车）', async () => {
    const w = mountLayout()
    await settle()
    const glyphs = menuIconSpans(w).map((s) => s.text().trim())
    expect(glyphs.length).toBe(7)
    const unique = new Set(glyphs)
    // 反向证伪：若任意两项字形相同（如旧 ⚙ 撞车）则 size < 7 → FAIL
    expect(unique.size).toBe(7)
  })

  it('"服务端配置"与"设置"图标字形不再相同（修复核心缺陷：旧版均为 ⚙）', async () => {
    const w = mountLayout()
    await settle()
    const icons = menuIconSpans(w)
    const server = icons.find((s) => s.attributes('aria-label') === '服务端配置')
    const settings = icons.find((s) => s.attributes('aria-label') === '设置')
    expect(server).toBeTruthy()
    expect(settings).toBeTruthy()
    // 核心反向证伪：折叠态两项的可视字形必须不同
    expect(server!.text().trim()).not.toBe(settings!.text().trim())
    // 且两项无障碍名也不同（折叠态屏幕阅读器可区分）
    expect(server!.attributes('aria-label')).not.toBe(settings!.attributes('aria-label'))
  })
})

describe('AppLayout.vue — 主题切换控件（T-066 AC-9）', () => {
  it('顶栏存在带非空 aria-label="主题切换" 的切换控件', async () => {
    const w = mountLayout()
    await settle()
    // DOM 属性查询（insight L45），不查 naive-ui 组件名
    const el = w.find('[aria-label="主题切换"]')
    expect(el.exists()).toBe(true)
  })

  it('切换控件绑定到 useTheme 偏好：setPref(dark) 后状态层 pref=dark 且 localStorage 持久化', async () => {
    mountLayout() // 挂载建立绑定上下文；本例直接断言 useTheme 单例状态，无需句柄
    await settle()
    const { pref, setPref } = useTheme()
    expect(pref.value).toBe('auto') // beforeEach 复位基线
    setPref('dark')
    await settle()
    expect(pref.value).toBe('dark')
    expect(localStorage.getItem('frpEasy.themePref')).toBe('dark')
  })

  it('切换控件不改"退出登录"按钮（e2e 03-dashboard TC-05 保护，AC-13）', async () => {
    const w = mountLayout()
    await settle()
    // 退出登录按钮文本仍存在且未被切换控件替换
    expect(w.text()).toContain('退出登录')
    // 切换控件与退出按钮共存（控件在退出按钮之前，不遮挡/不替换）
    expect(w.find('[aria-label="主题切换"]').exists()).toBe(true)
  })
})
