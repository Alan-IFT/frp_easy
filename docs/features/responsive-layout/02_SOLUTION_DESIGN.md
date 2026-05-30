# 02 方案设计 — T-067 · responsive-layout

> Harness Stage 2 · Solution Architect · 模式 full · 全中文 · 上游 01 verdict=READY

## 1. 架构摘要

纯前端布局响应式增强，零新依赖、零后端/store/路由/API 改动。新增一个**模块级单例 composable `web/src/composables/useViewport.ts`**（复刻 T-066 `useTheme.ts` 模块单例范式），用原生 `window.matchMedia('(max-width: 767.98px)')` 暴露响应式 `isNarrow: Ref<boolean>`（视口 < 768px 为 true），并在卸载安全的前提下监听媒体查询变化（NFR-4）。`AppLayout.vue` 消费 `isNarrow`：(a) 用一个 `watch(isNarrow)` 驱动侧栏 `collapsed` 默认态切换（窄→true 折叠 / 宽→false 展开），保留现有 `collapsed` ref 与手动 `@expand/@collapse`（FR-2 共存）；(b) 顶栏 `n-space` 加 `wrap` + 版本号窄屏隐藏（FR-4/FR-5）；(c) 内容区 padding 据 `isNarrow` 取 12/24px（FR-8 可选）。`Server.vue`/`Client.vue` 表单固定 px 宽改 `max-width + width:100%`（FR-6/FR-7，纯 CSS，不依赖 composable）。

## 2. 受影响模块

| 文件 | 改动 |
|---|---|
| `web/src/composables/useViewport.ts` | **新建**：模块单例 composable，`useViewport()` 返回 `{ isNarrow: Ref<boolean> }` |
| `web/src/components/AppLayout.vue` | 编辑：import useViewport；watch isNarrow 驱动 collapsed 默认态；顶栏 n-space 加 wrap + 版本号 v-if 窄屏隐藏；内容区 padding 响应式 |
| `web/src/pages/Server.vue` | 编辑：5 处 `style="width: Npx"` → `style="width: 100%; max-width: Npx"`（template 内联，纯 CSS） |
| `web/src/pages/Client.vue` | 编辑：3 处 `style="width: Npx"` → `style="width: 100%; max-width: Npx"` |
| `web/src/composables/__tests__/useViewport.spec.ts` | **新建**：composable 单测 |
| `web/src/components/__tests__/AppLayout.spec.ts` | 编辑：+ 自动折叠 / 共存 / 顶栏 wrap 用例（既有 8 例零回归） |
| `web/src/pages/__tests__/Server.spec.ts` | 编辑：+ 表单 max-width 用例 |
| `web/src/pages/__tests__/Client.spec.ts` | 编辑：+ 表单 max-width 用例 |
| `web/src/components/__tests__/qa_t067_adversarial.spec.ts` | **新建**：QA 独立对抗（stage 6 由 QA 补，列此处供 dev 预留断言契约） |
| `scripts/baseline.json` | 编辑：bump frontend_tests / test_count / version + notes |
| `docs/dev-map.md` | 编辑：composables 表 + AppLayout 行 + Server/Client 行响应式注 |

## 3. 模块分解（新建 useViewport.ts）

```ts
// web/src/composables/useViewport.ts
// T-067 / responsive-layout · 02 §3
// 模块级单例 composable：响应式视口宽度断点。
// 复刻 useTheme.ts 模块单例范式（App/AppLayout 共享同一引用，无需 provide/inject）。
//
// 单一真相源：isNarrow（视口 < NARROW_MAX_WIDTH 为 true）。
// 用原生 window.matchMedia——零依赖、边界完全可控（不依赖 naive-ui useBreakpoint 的
// 固定分界点 640/1024，那些不对齐 768 且边界方向语义不透明）。

import { ref, type Ref } from 'vue'

// BC-1：< 768 折叠 / >= 768 展开。用 767.98px 上界让 (max-width:767.98px) 在 768 整数
// 边界为 false（即 768 判为"宽/展开"），消除整数 vs CSS px 边界歧义。
export const NARROW_MAX_WIDTH = 767.98
export const NARROW_QUERY = `(max-width: ${NARROW_MAX_WIDTH}px)`

// 模块级单例（App.vue / AppLayout.vue 共享）
const isNarrow = ref<boolean>(false)
let initialized = false

function safeMatchMedia(): MediaQueryList | null {
  // BC-6：happy-dom 默认 / SSR / 老浏览器无 matchMedia → 返回 null，isNarrow 留 false
  // （退化为展开态，由用户手动控制，不抛错）。
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return null
  try {
    return window.matchMedia(NARROW_QUERY)
  } catch {
    return null
  }
}

function init(): void {
  if (initialized) return
  initialized = true
  const mql = safeMatchMedia()
  if (!mql) return // BC-6 降级：保持 false
  isNarrow.value = mql.matches
  const onChange = (e: MediaQueryListEvent | MediaQueryList) => {
    isNarrow.value = e.matches
  }
  // addEventListener('change') 是现代标准；老 Safari 用 addListener。两者都试，
  // 不在卸载时刻意 remove——模块单例存活整个 app 生命周期（与 useTheme osThemeRef 同），
  // 监听数恒为 1，无泄漏（NFR-4：单例不重复注册由 initialized 守卫保证）。
  if (typeof mql.addEventListener === 'function') {
    mql.addEventListener('change', onChange as (e: MediaQueryListEvent) => void)
  } else if (typeof (mql as { addListener?: unknown }).addListener === 'function') {
    ;(mql as unknown as { addListener: (cb: (e: MediaQueryList) => void) => void }).addListener(onChange)
  }
}

export interface UseViewportReturn {
  isNarrow: Ref<boolean>
}

export function useViewport(): UseViewportReturn {
  init() // 惰性首调初始化监听（幂等，initialized 守卫）
  return { isNarrow }
}
```

**设计要点**：
- 单例 + `initialized` 守卫 → matchMedia 监听全 app 仅注册一次（NFR-4 无泄漏、无重复，BC-5 抖动安全：listener 只更新 ref 值不递归）。
- `safeMatchMedia` null 降级（BC-6）。
- `NARROW_MAX_WIDTH=767.98` 让 768 整数判为展开（BC-1：>= 768 展开），且 1280（e2e）远大于 768 判为展开（BC-2 零回归）。

## 4. 数据模型 / API 变更

无。纯前端 UI 布局，无 schema / 无 API / 无 store 变更。

## 5. API 契约

无后端契约变更。

## 6. 序列 / 流程

```
App 启动 → App.vue setup（可选先调 useViewport，但 AppLayout setup 调即可）
AppLayout.vue setup:
  const { isNarrow } = useViewport()   // 首调 init() 建立 matchMedia 监听，读初值
  const collapsed = ref(isNarrow.value) // 初始默认态 = 当前断点默认（窄→true/宽→false）
  watch(isNarrow, (narrow) => {          // FR-3：跨阈值时重置默认态
    collapsed.value = narrow
  })
  // FR-2 共存：模板 @expand="collapsed=false" / @collapse="collapsed=true" 保留，
  //   用户手动操作直接改 collapsed，watch 只在 isNarrow 真正变化（跨阈值）时再赋值，
  //   同区间内手动展开不被覆盖（isNarrow 未变 → watch 不触发）。
窄屏运行时：用户手点 trigger 展开 → collapsed=false（watch 不触发，因 isNarrow 仍 true）→ 保持展开（不锁死）
视口宽→窄：isNarrow false→true → watch → collapsed=true（自动折叠）
视口窄→宽：isNarrow true→false → watch → collapsed=false（自动展开，桌面不回归）
```

**关键正确性论证（FR-2 不锁死）**：`watch(isNarrow)` 仅在 `isNarrow` 值变化（跨 768 阈值）时触发。用户在窄屏内手动展开（改 `collapsed=false`）时 `isNarrow` 仍为 true 未变 → watch 不触发 → `collapsed` 保持用户设定的 false → 侧栏保持展开。只有视口真正跨阈值才重置默认态（FR-3）。这是"自动折叠只是默认态、尊重用户手动操作"的精确实现。

## 7. Reuse audit

| 需求 | 既有代码 | 文件路径 | 决策 |
|---|---|---|---|
| 模块单例 composable 范式 | `useTheme.ts`（pref ref + 模块级 + 惰性 hook 初始化 + 单例守卫） | `web/src/composables/useTheme.ts` | 复刻范式（模块级 ref + 惰性 init + 单例）。**不抽公共 util**（避免反向耦合 theme/viewport 两域，与 T-066 R-4 同理）。 |
| matchMedia 监听安全模式 | useTheme 注释 L13-16 描述 useOsTheme 内部 matchMedia 监听须 setup 内 | `web/src/composables/useTheme.ts:13` | 借鉴 null 降级思路；但 useViewport 用原生 matchMedia 不依赖 setup（无 onMounted/inject），可在任意时机 init |
| AppLayout 既有 collapsed/trigger | `collapsed = ref(false)` + `@collapse/@expand` + `show-trigger` | `web/src/components/AppLayout.vue:84-93,132` | 复用：仅把初值与 watch 接上 isNarrow，手动 trigger 保留 |
| 响应式范本 | `n-grid cols="1 m:2" responsive="screen"` | `web/src/pages/Dashboard.vue:23` | 参考（确认 team 认可响应式）；本任务骨架用 matchMedia 不用 n-grid |
| AppLayout spec mount 范式 | 真 Pinia + vi.mock vue-router/naive-ui(useOsTheme/useMessage) + NConfigProvider wrap | `web/src/components/__tests__/AppLayout.spec.ts:21-67` | 复用；新增用例追加，既有 8 例不动 |
| 表单 max-width | (无现成范式，Settings.vue:4 已用 max-width:480px on card) | `web/src/pages/Settings.vue:4` | 借鉴 `max-width` 写法；表单控件改 `width:100% + max-width:Npx` |
| 断点机制候选 | naive-ui `useBreakpoint`（依赖 config-provider breakpoints 默认 640/1024…） | naive-ui 内置 | **不用**：分界点不对齐 768、边界方向语义不透明。改用原生 matchMedia（零依赖、边界可控）。理由见 §3。 |

## 8. 风险分析

- **R-1（e2e 误触发自动折叠 → 03-dashboard FAIL）**：若阈值 ≥ 1280px，e2e 默认视口（1280）会被判窄屏自动折叠，菜单文本隐藏，03-dashboard 断言菜单/退出文本 FAIL。**缓解**：阈值 `NARROW_MAX_WIDTH=767.98`（< 1280），1280 判展开（BC-2）；折叠态仅图标但 T-064 给了 aria-label/title，且 e2e 在 1280 根本不折叠。dev/QA 须静态核实 playwright 视口 + grep 03-dashboard 断言（insight L34）。
- **R-2（happy-dom matchMedia 行为不确定 → 测试不稳）**：happy-dom 的 matchMedia 默认基于 innerWidth、不自动响应、listener 行为可能与浏览器不同。**缓解**：测试不依赖 happy-dom 真实 matchMedia 行为，而是 `vi.stubGlobal('matchMedia', ...)` 注入受控 MediaQueryList（matches 可控 + 暴露注册的 onChange 供测试手动触发"视口变化"），与 useTheme.spec 用 vi.mock 受控 osThemeRef 同思路。useViewport 单例 + `vi.resetModules()` + 动态 import 拿全新单例规避跨用例泄漏（复刻 useTheme.spec C-1 范式）。
- **R-3（既有 AppLayout 8 例回归——T-064/T-066）**：新增 watch/wrap/padding 不能破坏 menuIcon 7 例 + 主题控件 3 例。**缓解**：AppLayout 既有 spec 不 mock matchMedia 时 useViewport 走 BC-6 降级（isNarrow=false，collapsed 初值 false=展开），与既有"默认展开"行为字节一致 → 既有 8 例零回归。dev 须确认既有 spec 未注入 matchMedia（默认 happy-dom innerWidth 1024 → (max-width:767.98) matches=false → 展开，与既有一致）。
- **R-4（表单 max-width 改动破坏 Server/Client 既有 spec）**：T-047/T-058/T-060/T-062 的 Server/Client spec 断言的是 DOM 文本/按钮/getExposed/apiGet 调用，**不**断言输入控件的 `width` style（已核 AppLayout.spec/既有范式断言可观察行为非像素宽）。**缓解**：dev 改 style 前 grep Server/Client spec 是否断言 `width:` style——若无（预期无），改动对既有 spec 透明（与 T-061 评估"抽取是否破坏测试须实读断言"insight L37 同源）。
- **R-5（顶栏 n-space wrap 影响主题控件/退出按钮位置 → e2e TC-05）**：03-dashboard TC-05 按 `getByRole name '退出登录'` 点击。**缓解**：wrap 只让窄屏换行，1280 视口下顶栏不换行（元素总宽 < 1280），退出按钮文本/role 不变、可点击；版本号 v-if 隐藏只在窄屏。dev 须确认 wrap 不改退出按钮的可访问名与可点击性。

## 9. 迁移 / rollout

- 无数据迁移、无 feature flag。纯前端布局增强，向后兼容（宽屏行为完全不变，NFR-1）。
- rollback：还原 4 个源文件 + 删 useViewport.ts 即可，无残留状态（localStorage 无新 key，无 DB）。

## 10. Out-of-scope 澄清（设计边界）

- 不做 drawer/抽屉式侧栏（OOS-6）；仅自动折叠到图标态。
- 不动 Proxies/Login/Setup/Wizard 表单宽（OOS-1）。
- 不引入新颜色（OOS-5）；padding 调整是数值非颜色。
- App.vue 是否也调 useViewport：**不需要**。AppLayout 是唯一消费方，AppLayout setup 调 useViewport 即触发 init。App.vue 不改（减小改动面）。
- 不为 `n-layout-content` 引入 scoped class——padding 用内联 `:style` 据 isNarrow 计算（与现有内联 style 风格一致）。

## 11. Partition assignment（分区模式 REQUIRED）

| 文件 | 分区 | 新建/编辑 | 依赖 |
|---|---|---|---|
| `web/src/composables/useViewport.ts` | dev-frontend | 新建 | — |
| `web/src/components/AppLayout.vue` | dev-frontend | 编辑 | useViewport |
| `web/src/pages/Server.vue` | dev-frontend | 编辑 | — |
| `web/src/pages/Client.vue` | dev-frontend | 编辑 | — |
| `web/src/composables/__tests__/useViewport.spec.ts` | dev-frontend | 新建 | useViewport |
| `web/src/components/__tests__/AppLayout.spec.ts` | dev-frontend | 编辑 | AppLayout |
| `web/src/pages/__tests__/Server.spec.ts` | dev-frontend | 编辑 | Server.vue |
| `web/src/pages/__tests__/Client.spec.ts` | dev-frontend | 编辑 | Client.vue |
| `scripts/baseline.json` | dev-frontend | 编辑 | 测试完成后 |
| `docs/dev-map.md` | dev-frontend | 编辑 | — |

### Dispatch order

1. dev-frontend（单分区覆盖全部）

### Parallelism

无——单分区，纯前端。

## 12. Verdict

**READY** — 设计完整、零新依赖、阈值边界（767.98 < 1280）确保 e2e 零回归、FR-2 不锁死有精确论证、既有 spec 零回归有 R-3/R-4 缓解路径。进入 Stage 3 Gate Review。
