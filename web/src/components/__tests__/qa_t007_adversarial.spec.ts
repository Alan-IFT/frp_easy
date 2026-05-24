import { describe, it, expect } from 'vitest'
import { nextTick } from 'vue'
import { useProxyForm } from '../../composables/useProxyForm'

describe('QA T-007 Adversarial — ProxyForm AC-9', () => {
  // AC-9 对抗：syncFromInput 顺序写 type 然后 customDomains/remotePort，
  // 在 sync 期间如果 watch 用 flush:'sync' 会"在 type 写完之后立即清空"customDomains，
  // 后续赋值会修复。当前 flush:'pre' 应该让整个 sync 完成后才触发 watch。
  // 测试：连续多次切换 + 立即提交（同 tick），验证 hidden 字段不会泄露。
  it('Adversarial: 同 tick 内多次 type 切换 + toProxyInput → 无残留', async () => {
    const { form, toProxyInput } = useProxyForm({
      name: 'x', type: 'tcp', localIP: '127.0.0.1', localPort: 80,
      remotePort: 8080, enabled: true,
    })
    form.value.type = 'http'
    form.value.customDomains = ['test.com']
    form.value.type = 'tcp'
    await nextTick()
    const out = toProxyInput()
    // 关键断言：切换链尾是 tcp，不应上送 customDomains
    expect(out.customDomains).toBeUndefined()
    // remotePort 不会被 watch 抹（tcp 切到 http 再回 tcp 后 remotePort 一开始就被 http 清掉了）
    // 真实场景需用户手动重新填 remotePort
  })

  // AC-9 对抗：oldType==newType 短路逻辑是否真工作？
  // watch 回调里的 `if (newType === oldType) return` 应让无 type 变化的 set 操作不触发清理
  it('Adversarial: 给 type 赋同一值不触发清理（oldType==newType 短路）', async () => {
    const { form } = useProxyForm({
      name: 'x', type: 'http', localIP: '127.0.0.1', localPort: 80,
      customDomains: ['keep.com'], enabled: true,
    })
    form.value.customDomains = ['final.com']
    // 假装某个外部代码无意义地把 type 重新赋为同一个值
    form.value.type = 'http'
    await nextTick()
    expect(form.value.customDomains).toEqual(['final.com'])
  })

  // AC-9 对抗：watch 的 oldType 在初次回调时是 undefined（Vue 3 watch 首次触发时）。
  // 但本测试用法（普通 watch、非 immediate）—— 首次 mount 不会触发；只有 type 后续变化才触发。
  // 验证 mount 后 form.type 与 initial.type 一致，没有被 watch 提前清掉。
  it('Adversarial: mount 后 form.type 与 initial 一致（watch immediate=false）', () => {
    const { form } = useProxyForm({
      name: 'x', type: 'http', localIP: '127.0.0.1', localPort: 80,
      customDomains: ['kept.com'], enabled: true,
    })
    expect(form.value.type).toBe('http')
    expect(form.value.customDomains).toEqual(['kept.com'])
  })

  // T-032：原 "Adversarial: syncFromInput 是原子的 — watch 触发后看到完整新状态" 用例已删除。
  // 原因：T-032 移除了 useProxyForm.syncFromInput 函数（02 §3.2）以及 ProxyForm.vue 的
  // 双向 watch + emit 反馈环（02 §3.1）。"sync 期间 watch flush:'pre' 不抢跑"的前提
  // 已不存在；等价语义改由「mount 完整组件传 initialValue → 内部 form 立即反映新值」
  // 在 ProxyForm.spec.ts 中以更强方式守护（参见该文件 T-032 改写 + AC-1 / AC-7 用例）。
  // 详见 docs/features/proxy-form-vmodel-oom-fix/02_SOLUTION_DESIGN.md §10 R-3。

  // AC-9 对抗：连续 5 次 type 切换 — 验证没有无限循环 / 栈溢出
  it('Adversarial: 多次 type 切换不会栈溢出（watch 不形成循环）', async () => {
    const { form } = useProxyForm({
      name: 'x', type: 'tcp', localIP: '127.0.0.1', localPort: 80,
      remotePort: 8080, enabled: true,
    })
    for (let i = 0; i < 50; i++) {
      form.value.type = i % 2 === 0 ? 'http' : 'tcp'
    }
    await nextTick()
    expect(['http', 'tcp']).toContain(form.value.type)
    // 没有抛错就算 PASS
  })
})
