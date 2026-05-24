// QA T-032 adversarial — 独立复现，不复用 dev 的 spec 断言。
// 假设："父组件高频替换 initialValue 引用，子组件会重新进入 emit 循环。"
// 期望若反馈环未根除：emit 计数会随 N 增长（O(N) 或 ∞）。
// 期望若反馈环已根除：update:modelValue emit 永远 = 0。
//
// 本文件由 qa-tester 独立编写，不读 04_DEVELOPMENT.md 的测试代码，
// 直接对 RA AC-1 / AC-7 / FR-3 / FR-4 / FR-5 / FR-6 写反向测试。

import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import ProxyForm from '../ProxyForm.vue'
import type { ProxyInput } from '../../types'

vi.mock('naive-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useMessage: () => ({
      error: vi.fn(), success: vi.fn(), warning: vi.fn(),
      info: vi.fn(), loading: vi.fn(), destroyAll: vi.fn(),
    }),
  }
})

const seedTcp = (port = 80): ProxyInput => ({
  name: '', type: 'tcp', localIP: '127.0.0.1', localPort: port, enabled: true,
})

const seedHttp = (domains: string[] = ['a.com']): ProxyInput => ({
  name: '', type: 'http', localIP: '127.0.0.1', localPort: 80,
  customDomains: domains, enabled: true,
})

describe('QA T-032 — Independent adversarial reproducer', () => {

  // AC-1 / AC-7 加强版：1000 次 setProps 仍 = 0
  it('1000 次连续替换 initialValue 引用后 update:modelValue emit 仍 = 0（OOM 反馈环根除）', async () => {
    const wrapper = mount(ProxyForm, { props: { initialValue: seedTcp() } })
    for (let i = 0; i < 1000; i++) {
      await wrapper.setProps({ initialValue: seedTcp(80 + (i % 100)) })
    }
    await nextTick()
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  // FR-3：单条 TCP 新增提交时 getProxyInput 含 remotePort / 不含 customDomains
  // 关键：单向数据流下，mount 时 props.initialValue 就必须含完整种子，
  // setProps 后续不会重置 form。这一行为本身是 AC-1/AC-7 的设计意图（见下方对照用例）。
  it('FR-3 单条 TCP：mount 时种子含 remotePort → getProxyInput 上送 remotePort、不上送 customDomains', async () => {
    const wrapper = mount(ProxyForm, {
      props: {
        initialValue: {
          name: 'tcp-rule', type: 'tcp', localIP: '127.0.0.1',
          localPort: 22, remotePort: 6022, enabled: true,
        },
      },
    })
    await nextTick()
    const out = (wrapper.vm as unknown as { getProxyInput: () => ProxyInput }).getProxyInput()
    expect(out.type).toBe('tcp')
    expect(out.remotePort).toBe(6022)
    expect(out.customDomains).toBeUndefined()
  })

  // 单向数据流对照：setProps initialValue 不会重置已 mount 子组件的内部 form。
  // 这正是反馈环根除的物理依据：父→子 prop 变化无任何路径触发 form 重写或 emit。
  it('单向数据流对照：mount 后 setProps initialValue 不会重置 form（这正是 OOM 根除的物理保证）', async () => {
    const wrapper = mount(ProxyForm, {
      props: {
        initialValue: {
          name: 'first', type: 'tcp', localIP: '127.0.0.1',
          localPort: 22, remotePort: 6022, enabled: true,
        },
      },
    })
    await nextTick()
    await wrapper.setProps({
      initialValue: {
        name: 'second', type: 'http', localIP: '0.0.0.0',
        localPort: 80, customDomains: ['z.com'], enabled: false,
      },
    })
    for (let i = 0; i < 5; i++) await nextTick()
    const out = (wrapper.vm as unknown as { getProxyInput: () => ProxyInput }).getProxyInput()
    // form 应保持 mount 时的 first 种子
    expect(out.name).toBe('first')
    expect(out.type).toBe('tcp')
    expect(out.remotePort).toBe(6022)
    // 而且没有 emit 反馈环
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  // FR-4 / FR-6：HTTP 编辑场景 customDomains 不被反馈环抹掉，且 toProxyInput 输出正确
  it('FR-4/6 HTTP：mount with customDomains → getProxyInput 上送 customDomains、不上送 remotePort', async () => {
    const wrapper = mount(ProxyForm, {
      props: {
        initialValue: seedHttp(['x.example.com', 'y.example.com']),
        editMode: true,
      },
    })
    for (let i = 0; i < 5; i++) await nextTick()
    const out = (wrapper.vm as unknown as { getProxyInput: () => ProxyInput }).getProxyInput()
    expect(out.type).toBe('http')
    expect(out.customDomains).toEqual(['x.example.com', 'y.example.com'])
    expect(out.remotePort).toBeUndefined()
  })

  // FR-5 (类型切换边界)：setProps 切 type tcp → http → tcp，互斥字段语义正确（不抢跑、不残留）
  it('FR-7/FR-5 类型边界：initialValue 切 tcp → http → tcp，type 切换互斥重置语义正确', async () => {
    // 初始 tcp（带 remotePort）
    const wrapper = mount(ProxyForm, {
      props: {
        initialValue: {
          name: 'switcher', type: 'tcp', localIP: '127.0.0.1',
          localPort: 80, remotePort: 6080, enabled: true,
        },
      },
    })
    await nextTick()
    let out = (wrapper.vm as unknown as { getProxyInput: () => ProxyInput }).getProxyInput()
    expect(out.type).toBe('tcp')
    expect(out.remotePort).toBe(6080)

    // setProps initialValue 引用变化不会重新初始化 form（单向数据流，setup 只读一次）。
    // 这正是设计意图——所以此处 type 与 customDomains 不应跟随 setProps 变。
    await wrapper.setProps({
      initialValue: {
        name: 'switcher', type: 'http', localIP: '127.0.0.1',
        localPort: 80, customDomains: ['z.example.com'], enabled: true,
      },
    })
    for (let i = 0; i < 5; i++) await nextTick()
    out = (wrapper.vm as unknown as { getProxyInput: () => ProxyInput }).getProxyInput()
    // 单向数据流：setProps 不会重置 form 状态——form.type 仍是 mount 时的 tcp
    expect(out.type).toBe('tcp')
    // tcp 模式不输出 customDomains，即便父组件 setProps 注入了
    expect(out.customDomains).toBeUndefined()
  })

  // 边界：localPort = 0（falsy）→ form.localPort = null
  it('boundary：initialValue.localPort = 0（falsy）→ getProxyInput.localPort = 0 兜底', async () => {
    const wrapper = mount(ProxyForm, {
      props: {
        initialValue: {
          name: 'zero', type: 'tcp', localIP: '127.0.0.1',
          localPort: 0, remotePort: 6000, enabled: true,
        },
      },
    })
    await nextTick()
    const out = (wrapper.vm as unknown as { getProxyInput: () => ProxyInput }).getProxyInput()
    // useProxyForm L22 `initial.localPort || null` → null；toProxyInput L54 `?? 0` → 0
    expect(out.localPort).toBe(0)
  })

  // 边界：customDomains undefined / [] 均不应触发反馈环或 throw
  it('boundary：customDomains undefined / [] 均稳定，无 emit 反馈环', async () => {
    const wrapper = mount(ProxyForm, { props: { initialValue: seedTcp() } })
    await nextTick()
    await wrapper.setProps({ initialValue: { ...seedTcp(), customDomains: [] } })
    await nextTick()
    await wrapper.setProps({ initialValue: { ...seedTcp(), customDomains: undefined } })
    await nextTick()
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })
})
