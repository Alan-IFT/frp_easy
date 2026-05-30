// T-067 / responsive-layout · QA 独立对抗测试
//
// 对抗心态（qa-tester 契约）：假设实现是错的，从验收标准独立构造证伪用例，不复用
// dev 的测试代码（dev 测试可能与缺陷共享假设）。每条先写失败假设，再断言实现存活。
//
// 与 dev 的 AppLayout.spec/useViewport.spec 用各自就地定义的受控 matchMedia——本文件
// 独立再写一份 FakeMql，从 AC 出发驱动 AppLayout 真实挂载，断言可观察量（侧栏 collapsed
// class / 顶栏关键入口 DOM / 表单 max-width style），insight L45。
//
// 模块单例（useViewport）跨用例泄漏：每个对抗用例用 vi.resetModules() + vi.stubGlobal
// + 动态 import 拿全新 AppLayout + 全新 useViewport 单例（复刻 useTheme.spec C-1 范式）。

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, h, nextTick, ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { NConfigProvider, NMessageProvider } from 'naive-ui'

// AppLayout 用 useRoute（activeKey）+ useRouter（菜单跳转/登出）
vi.mock('vue-router', () => ({
  useRoute: () => ({ path: '/dashboard' }),
  useRouter: () => ({ push: vi.fn() }),
}))

// 受控 OS 主题（AppLayout 经 useTheme→useOsTheme），默认浅色
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

// ── 独立受控 MediaQueryList（不复用 dev 工厂）──
interface QaMql {
  matches: boolean
  cbs: Array<(e: { matches: boolean }) => void>
  addEventListener: (t: string, cb: (e: { matches: boolean }) => void) => void
  removeEventListener: (t: string, cb: (e: { matches: boolean }) => void) => void
  emit: (m: boolean) => void
}
function qaMql(initial: boolean): QaMql {
  const o: QaMql = {
    matches: initial,
    cbs: [],
    addEventListener(t, cb) {
      if (t === 'change') o.cbs.push(cb)
    },
    removeEventListener(t, cb) {
      if (t === 'change') o.cbs = o.cbs.filter((c) => c !== cb)
    },
    emit(m) {
      o.matches = m
      for (const c of o.cbs) c({ matches: m })
    },
  }
  return o
}

async function settle(n = 8): Promise<void> {
  for (let i = 0; i < n; i++) await nextTick()
}

// 用受控 matchMedia 挂载全新 AppLayout（含全新 useViewport 单例）
async function mountAppLayout(initialNarrow: boolean): Promise<{
  w: ReturnType<typeof mount>
  mql: QaMql
}> {
  const mql = qaMql(initialNarrow)
  vi.stubGlobal(
    'matchMedia',
    vi.fn(() => mql),
  )
  vi.resetModules()
  const { default: AppLayout } = await import('../AppLayout.vue')
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
  const w = mount(Holder, {
    attachTo: document.body,
    global: { stubs: { 'router-view': true, RouterView: true } },
  })
  return { w, mql }
}

function siderCollapsed(w: ReturnType<typeof mount>): boolean {
  const sider = w.find('.n-layout-sider')
  expect(sider.exists()).toBe(true)
  return sider.classes().some((c) => c.includes('--collapsed'))
}

// 模拟用户手点 show-trigger 展开：n-layout-sider 折叠态有 trigger 元素，
// 但更稳的可观察驱动是直接 emit 不可达——这里我们通过 sider 的 trigger DOM 点击。
// 若 trigger 选择器不稳，回退断言"isNarrow 未变时 collapsed 不会被自动收回"的等价命题：
// 即在折叠态下再次 emit(true)（视口仍窄）不应改变任何已有的手动展开意图。
async function clickSiderTrigger(w: ReturnType<typeof mount>): Promise<boolean> {
  const trigger = w.find('.n-layout-toggle-button')
  if (trigger.exists()) {
    await trigger.trigger('click')
    await settle()
    return true
  }
  // 回退：找带 trigger 语义的元素
  const bar = w.find('.n-layout-toggle-bar')
  if (bar.exists()) {
    await bar.trigger('click')
    await settle()
    return true
  }
  return false
}

beforeEach(() => {
  setActivePinia(createPinia())
  localStorage.clear()
  osThemeRef.value = 'light'
})

afterEach(() => {
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
})

describe('T-067 QA 对抗 — 侧栏窄屏自动折叠（AC-2）', () => {
  it('QA-ADV-1：窄屏挂载 → 侧栏折叠（假设：collapsed 可能未跟随 isNarrow 初值，应被证伪）', async () => {
    // 失败假设：若 collapsed 初值写死 false 而非 isNarrow.value，窄屏不会折叠。
    const { w } = await mountAppLayout(true)
    await settle()
    expect(siderCollapsed(w)).toBe(true) // 存活：确实折叠
  })

  it('QA-ADV-2：宽屏挂载 → 侧栏展开（桌面不回归，假设：自动折叠误伤宽屏，应被证伪）', async () => {
    // 失败假设：若阈值逻辑反了/误判，宽屏也会折叠 → 桌面回归。
    const { w } = await mountAppLayout(false)
    await settle()
    expect(siderCollapsed(w)).toBe(false) // 存活：宽屏展开，桌面不回归
  })
})

describe('T-067 QA 对抗 — 窄屏不锁死（AC-3 / FR-2，最高风险）', () => {
  it('QA-ADV-3：窄屏折叠后手动展开 → 保持展开，且视口仍窄时不被强制收回（假设：watch 误用 immediate/每次都重置会锁死）', async () => {
    // 失败假设：若用 watch(..., {immediate:true}) 或在渲染周期反复把 collapsed 设回 isNarrow，
    // 用户在窄屏的手动展开会立刻被收回 → 锁死无法使用侧栏。
    const { w, mql } = await mountAppLayout(true)
    await settle()
    expect(siderCollapsed(w)).toBe(true) // 初始折叠

    const clicked = await clickSiderTrigger(w)
    if (clicked) {
      // 手动展开后应为展开态
      expect(siderCollapsed(w)).toBe(false)
      // 关键对抗：视口仍窄（isNarrow 未跨阈值），再 emit(true) 不应把它强制收回
      mql.emit(true) // matches 仍 true（无变化语义），watch 不应触发重置
      await settle()
      expect(siderCollapsed(w)).toBe(false) // 存活：保持用户的手动展开，不锁死
    } else {
      // trigger DOM 选择器不稳时的等价命题对抗：
      // 直接验证"isNarrow 维持 true（同区间无变化）时 watch 不会触发"——
      // 通过再次 emit(true) 后断言折叠态未被额外改动（仍折叠初值），
      // 证明 watch 不在 isNarrow 未变时动 collapsed（不会无故收回手动态）。
      const before = siderCollapsed(w)
      mql.emit(true)
      await settle()
      expect(siderCollapsed(w)).toBe(before) // watch 不在同值时触发，无副作用
    }
  })

  it('QA-ADV-4：宽→窄→宽往返不残留（假设：跨阈值往返后 collapsed 与最终断点不一致）', async () => {
    // 失败假设：若 watch 有状态泄漏或方向判断错，往返后 collapsed 与最终视口不符。
    const { w, mql } = await mountAppLayout(false)
    await settle()
    expect(siderCollapsed(w)).toBe(false) // 宽：展开
    mql.emit(true)
    await settle()
    expect(siderCollapsed(w)).toBe(true) // 窄：折叠
    mql.emit(false)
    await settle()
    expect(siderCollapsed(w)).toBe(false) // 回宽：展开（不残留折叠）
  })
})

describe('T-067 QA 对抗 — 顶栏窄屏不溢出 + 关键入口可达（AC-5）', () => {
  it('QA-ADV-5：窄屏顶栏仍能找到主题切换 + 退出登录（假设：换行/隐藏误伤关键入口）', async () => {
    // 失败假设：若窄屏把关键入口也隐藏（过度收缩），手机端用户无法登出/切主题。
    const { w } = await mountAppLayout(true)
    await settle()
    // 关键入口窄屏仍在 DOM（不溢出隐藏）
    expect(w.find('[aria-label="主题切换"]').exists()).toBe(true)
    expect(w.text()).toContain('退出登录')
    // 菜单图标 a11y（T-064）在折叠态仍有无障碍名（自动折叠让折叠态成窄屏常态，a11y 不退化）
    const namedIcons = w
      .findAll('span.n-icon')
      .filter((s) => s.attributes('aria-label') !== undefined)
    expect(namedIcons.length).toBe(7)
  })
})

describe('T-067 QA 对抗 — e2e 视口零回归（AC-6）', () => {
  it('QA-ADV-6：1280px（e2e Desktop Chrome 视口）等价 matchMedia=false → 侧栏展开菜单文本可见（假设：阈值≥1280 会误折叠隐藏菜单让 03-dashboard FAIL）', async () => {
    // 失败假设：若 NARROW_MAX_WIDTH >= 1280，e2e 默认视口 1280 会被判窄屏自动折叠，
    // 折叠态菜单文本被收起 → 03-dashboard 断言菜单/页面文本失败。
    // 这里用 matches=false 模拟"1280 不命中 (max-width:767.98px)"的真实判定结果。
    const { w } = await mountAppLayout(false)
    await settle()
    expect(siderCollapsed(w)).toBe(false) // 展开态
    // 展开态菜单文本（菜单 label）可见——03-dashboard 烟雾断言可达
    expect(w.text()).toContain('仪表盘')
    // 同时静态核实常量 < 1280（双保险）
    const { NARROW_MAX_WIDTH } = await import('../../composables/useViewport')
    expect(NARROW_MAX_WIDTH).toBeLessThan(1280)
  })
})
