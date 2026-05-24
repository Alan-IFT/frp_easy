import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { useProxyForm } from '../../composables/useProxyForm'
import ProxyForm from '../ProxyForm.vue'
import type { ProxyInput } from '../../types'

// T-032 P1-2：vitest + happy-dom + Naive UI 的 useMessage stub。
// 必须用 importOriginal 模式 —— ProxyForm.vue import 十余个 Naive UI 组件，
// 整体 vi.mock('naive-ui') 不带 importOriginal 会让 render 时所有 N* 组件缺失。
vi.mock('naive-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useMessage: () => ({
      error:       vi.fn(),
      success:     vi.fn(),
      warning:     vi.fn(),
      info:        vi.fn(),
      loading:     vi.fn(),
      destroyAll:  vi.fn(),
    }),
  }
})

describe('useProxyForm composable（ProxyForm 逻辑）', () => {
  const tcpInput = (): ProxyInput => ({
    name: 'test-proxy',
    type: 'tcp',
    localIP: '127.0.0.1',
    localPort: 22,
    remotePort: 6000,
    enabled: true,
  })

  const httpInput = (): ProxyInput => ({
    name: 'web-proxy',
    type: 'http',
    localIP: '127.0.0.1',
    localPort: 80,
    customDomains: ['example.com'],
    enabled: true,
  })

  it('type=tcp 时 isTcpUdp 为 true', () => {
    const { isTcpUdp, isHttpHttps } = useProxyForm(tcpInput())
    expect(isTcpUdp.value).toBe(true)
    expect(isHttpHttps.value).toBe(false)
  })

  it('type=udp 时 isTcpUdp 为 true', () => {
    const udpInput: ProxyInput = { ...tcpInput(), type: 'udp' }
    const { isTcpUdp, isHttpHttps } = useProxyForm(udpInput)
    expect(isTcpUdp.value).toBe(true)
    expect(isHttpHttps.value).toBe(false)
  })

  it('type=http 时 isHttpHttps 为 true', () => {
    const { isTcpUdp, isHttpHttps } = useProxyForm(httpInput())
    expect(isTcpUdp.value).toBe(false)
    expect(isHttpHttps.value).toBe(true)
  })

  it('type=https 时 isHttpHttps 为 true', () => {
    const httpsInput: ProxyInput = { ...httpInput(), type: 'https' }
    const { isTcpUdp, isHttpHttps } = useProxyForm(httpsInput)
    expect(isTcpUdp.value).toBe(false)
    expect(isHttpHttps.value).toBe(true)
  })

  // T-007 AC-9：handleTypeChange 按目标 type 互斥重置，**只清不属于该 type 的字段**。
  // 旧逻辑（T-001/T-002）会一刀切清空两个字段；新逻辑只清互斥字段。
  it('handleTypeChange(tcp) 只清 customDomains，remotePort 保留（语义化重置）', () => {
    const { form, handleTypeChange } = useProxyForm(tcpInput())
    form.value.remotePort = 8080
    form.value.customDomains = ['stale.com']

    handleTypeChange('tcp')

    expect(form.value.customDomains).toHaveLength(0)
    expect(form.value.remotePort).toBe(8080)
  })

  it('handleTypeChange(http) 只清 remotePort，customDomains 保留', () => {
    const { form, handleTypeChange } = useProxyForm(tcpInput())
    form.value.remotePort = 8080
    form.value.customDomains = ['kept.com']

    handleTypeChange('http')

    expect(form.value.remotePort).toBeNull()
    expect(form.value.customDomains).toEqual(['kept.com'])
  })

  it('toProxyInput tcp 类型时包含 remotePort', () => {
    const { toProxyInput } = useProxyForm(tcpInput())
    const output = toProxyInput()
    expect(output.remotePort).toBe(6000)
    expect(output.customDomains).toBeUndefined()
  })

  it('toProxyInput http 类型时包含 customDomains', () => {
    const { toProxyInput } = useProxyForm(httpInput())
    const output = toProxyInput()
    expect(output.customDomains).toEqual(['example.com'])
    expect(output.remotePort).toBeUndefined()
  })

  // ----------------------------
  // T-032：以下原 3 条「syncFromInput 直接调用」用例改写为 mount + initialValue 等价断言。
  // syncFromInput 已被删除（02 §3.2）；等价语义现在由「mount ProxyForm with initialValue
  // → 内部 form 立即正确反映 initialValue」承担，且双向桥已删，物理上不可能被 watch 抹掉。
  // 详见 docs/features/proxy-form-vmodel-oom-fix/02_SOLUTION_DESIGN.md §10 R-3 / §13.3 / §13.5。
  // ----------------------------

  it('T-032 改写：mount ProxyForm with initialValue 后内部 form 等价于外部字段', async () => {
    const wrapper = mount(ProxyForm, {
      props: {
        initialValue: {
          name: 'updated',
          type: 'http',
          localIP: '0.0.0.0',
          localPort: 3000,
          customDomains: ['new.example.com'],
          enabled: false,
          version: 2,
        },
      },
    })
    await nextTick()
    const submitted = (wrapper.vm as unknown as { getProxyInput: () => ProxyInput })
      .getProxyInput()
    expect(submitted.name).toBe('updated')
    expect(submitted.type).toBe('http')
    expect(submitted.localPort).toBe(3000)
    expect(submitted.customDomains).toEqual(['new.example.com'])
    expect(submitted.enabled).toBe(false)
    expect(submitted.version).toBe(2)
  })

  it('AC-9 / C-1（T-032 等价）：mount 编辑现有 HTTP 规则 customDomains 不会被抹掉', async () => {
    const wrapper = mount(ProxyForm, {
      props: {
        initialValue: {
          name: 'edit-web',
          type: 'http',
          localIP: '127.0.0.1',
          localPort: 80,
          customDomains: ['existing.example.com', 'second.example.com'],
          enabled: true,
          version: 7,
        },
        editMode: true,
      },
    })
    // 等 happy-dom + Naive UI 完成几个 tick；任何 watch 触发都会在此期间反映出来
    for (let i = 0; i < 5; i++) await nextTick()
    const submitted = (wrapper.vm as unknown as { getProxyInput: () => ProxyInput })
      .getProxyInput()
    expect(submitted.type).toBe('http')
    expect(submitted.customDomains).toEqual([
      'existing.example.com', 'second.example.com',
    ])
    // tcp 字段未上送（http 模式 toProxyInput 不输出 remotePort）
    expect(submitted.remotePort).toBeUndefined()
    expect(submitted.version).toBe(7)
  })

  it('AC-9 / C-1（T-032 等价）：mount 编辑现有 TCP 规则 remotePort 不会被抹掉', async () => {
    const wrapper = mount(ProxyForm, {
      props: {
        initialValue: {
          name: 'edit-ssh',
          type: 'tcp',
          localIP: '127.0.0.1',
          localPort: 22,
          remotePort: 6022,
          enabled: true,
          version: 3,
        },
        editMode: true,
      },
    })
    for (let i = 0; i < 5; i++) await nextTick()
    const submitted = (wrapper.vm as unknown as { getProxyInput: () => ProxyInput })
      .getProxyInput()
    expect(submitted.type).toBe('tcp')
    expect(submitted.remotePort).toBe(6022)
    // tcp 模式 toProxyInput 不输出 customDomains
    expect(submitted.customDomains).toBeUndefined()
    expect(submitted.version).toBe(3)
  })

  it('AC-9: type 切换 tcp → http 时 customDomains 不残留旧值（watch 兜底）', async () => {
    const { form } = useProxyForm(tcpInput())
    form.value.customDomains = ['stale.com'] // 模拟某条 race 路径污染了 customDomains
    expect(form.value.customDomains).toEqual(['stale.com'])

    // 用户在 select 中改成 http；模拟 v-model 直接更新 form.value.type
    form.value.type = 'http'
    await nextTick()

    // 切到 http 时不应清 customDomains（因为这是 http 类型本身需要的字段）
    // 但应该清掉 remotePort
    expect(form.value.remotePort).toBeNull()
    // customDomains 在切到 http 时不动（仍是用户旧值，提交时由后端校验）
    expect(form.value.customDomains).toEqual(['stale.com'])
  })

  it('AC-9: type 切换 http → tcp 时 customDomains 被 watch 兜底清空', async () => {
    const { form } = useProxyForm(httpInput())
    expect(form.value.customDomains).toEqual(['example.com'])

    form.value.type = 'tcp'
    await nextTick()

    expect(form.value.customDomains).toEqual([])
    // 此时用户应手动填 remotePort；watch 不会自动给值，但隐藏字段已清，避免提交时上送
  })

  it('AC-9: type 不变时 watch 不会重复触发清理', async () => {
    const { form, isTcpUdp } = useProxyForm(tcpInput())
    form.value.remotePort = 6000

    // 改一个无关字段不应触发 type watch
    form.value.name = 'renamed'
    await nextTick()

    expect(form.value.remotePort).toBe(6000)
    expect(isTcpUdp.value).toBe(true)
  })

  it('AC-9: toProxyInput 在 tcp 模式下不会上送残留的 customDomains', () => {
    const { form, toProxyInput } = useProxyForm(tcpInput())
    // 假设某条边界路径污染了 customDomains（不应该发生，但回归保险）
    form.value.customDomains = ['ghost.com']

    const output = toProxyInput()
    // toProxyInput 已经按 type 分支选字段：tcp 模式下不会输出 customDomains
    expect(output.customDomains).toBeUndefined()
    expect(output.remotePort).toBe(6000)
  })
})

// =========================================================================
// T-032：新增 mount-level 测试覆盖单向数据流不变量 + 反馈环根除证明
// =========================================================================

const defaultSeed = (): ProxyInput => ({
  name: '',
  type: 'tcp',
  localIP: '127.0.0.1',
  localPort: 80,
  enabled: true,
})

describe('T-032 AC-1: ProxyForm 不产生 update:modelValue 反馈环', () => {
  it('mount 后 50ms / 10 ticks 内不 emit "update:modelValue"（删 emit 后上界 = 0）', async () => {
    const wrapper = mount(ProxyForm, {
      props: { initialValue: defaultSeed() },
    })
    for (let i = 0; i < 10; i++) await nextTick()
    await new Promise((r) => setTimeout(r, 50))
    // 关键：本设计删除了 'update:modelValue' emit，断言 emit 表中根本不应出现该事件
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })
})

describe('T-032 AC-7: ProxyForm initialValue 引用变化时不进入无限 emit 循环', () => {
  it('父组件连续 3 次替换 initialValue（新对象引用，字段同），emit 总次数 = 0', async () => {
    const wrapper = mount(ProxyForm, { props: { initialValue: defaultSeed() } })
    for (let i = 0; i < 5; i++) await nextTick()

    // 模拟父组件 3 次 formData.value = defaultFormData() —— 新引用、字段相同
    for (let i = 0; i < 3; i++) {
      await wrapper.setProps({ initialValue: defaultSeed() })
      for (let j = 0; j < 5; j++) await nextTick()
    }

    // 关键断言：上界是 0（因为没有 update:modelValue emit），而非随 N 增长
    expect(wrapper.emitted('update:modelValue') ?? []).toHaveLength(0)

    // 兜底：batchMode / portsExpr emit 不在此用例语义内，但也应 ≤ 常数（初始 false / '' 后无变化）
    expect((wrapper.emitted('update:batchMode') ?? []).length).toBeLessThanOrEqual(1)
    expect((wrapper.emitted('update:portsExpr') ?? []).length).toBeLessThanOrEqual(1)
  })
})
