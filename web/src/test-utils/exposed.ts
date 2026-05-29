import type { VueWrapper } from '@vue/test-utils'

/**
 * 健壮读取一个子组件 `defineExpose` 暴露的对象（测试用，默认键 `__testing`）。
 *
 * ⚠️ 不要直接写 `wrapper.findComponent(C).vm.<key>`：那依赖 Vue Test Utils 的
 * `createVMProxy` 把 `vm.$.exposed` 透传到 `vm` 上，而该透传只在 Vue 已为该实例
 * 创建 `exposeProxy` 时才生效（`vue-test-utils.cjs.js` 里 `key in vm.$.exposeProxy`
 * 的判定）。`exposeProxy` 是否存在取决于实例是否被父级 ref 访问过 —— 不可靠：
 * 同样的 `defineExpose({ __testing: {...} })`，LogViewer 能取到、ServerMonitor /
 * Proxies 取到 `undefined`，曾让整条前端测试基线变红却没被发现。
 *
 * 规范做法是直接读 `vm.$.exposed`，它在 `defineExpose` 调用后必然存在。
 * 本 helper 先试 `vm[key]`（兼容已能透传的旧路径），再回落 `vm.$.exposed[key]`。
 * 详见 .harness/insight-index.md（T-043）。
 */
export function getExposed<T>(
  wrapper: VueWrapper,
  component: unknown,
  key = '__testing',
): T {
  const found = wrapper.findComponent(component as never) as VueWrapper
  const vm = found.vm as unknown as Record<string, unknown> & {
    $?: { exposed?: Record<string, unknown> | null }
  }
  const viaProxy = vm[key]
  if (viaProxy !== undefined) return viaProxy as T
  return vm.$?.exposed?.[key] as T
}
