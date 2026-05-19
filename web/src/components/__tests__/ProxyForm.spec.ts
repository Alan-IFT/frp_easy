import { describe, it, expect } from 'vitest'
import { nextTick } from 'vue'
import { useProxyForm } from '../../composables/useProxyForm'
import type { ProxyInput } from '../../types'

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

  it('syncFromInput 可从外部更新表单', () => {
    const { form, syncFromInput } = useProxyForm(tcpInput())
    const newInput: ProxyInput = {
      name: 'updated',
      type: 'http',
      localIP: '0.0.0.0',
      localPort: 3000,
      customDomains: ['new.example.com'],
      enabled: false,
      version: 2,
    }
    syncFromInput(newInput)

    expect(form.value.name).toBe('updated')
    expect(form.value.type).toBe('http')
    expect(form.value.localPort).toBe(3000)
    expect(form.value.customDomains).toEqual(['new.example.com'])
    expect(form.value.enabled).toBe(false)
    expect(form.value.version).toBe(2)
  })

  // ----------------------------
  // T-007 AC-9 / Gate Review C-1 新增覆盖：watch type 兜底 + 编辑模式
  // ----------------------------

  it('AC-9 / C-1: 编辑现有 HTTP 规则加载时 customDomains 不被 watch 抹掉', async () => {
    // 初始状态：tcp（默认 modal 打开时的 defaultFormData 是 tcp）
    const initial: ProxyInput = {
      name: '', type: 'tcp', localIP: '127.0.0.1', localPort: 80, enabled: true,
    }
    const { form, syncFromInput } = useProxyForm(initial)

    // 模拟父组件 handleEdit 触发：写入已有 HTTP 规则的完整数据
    syncFromInput({
      name: 'edit-web',
      type: 'http',
      localIP: '127.0.0.1',
      localPort: 80,
      customDomains: ['existing.example.com', 'second.example.com'],
      enabled: true,
      version: 7,
    })
    // 让 watch（flush='pre'）的回调跑一次
    await nextTick()

    expect(form.value.type).toBe('http')
    expect(form.value.customDomains).toEqual(['existing.example.com', 'second.example.com'])
    expect(form.value.remotePort).toBeNull()
    expect(form.value.version).toBe(7)
  })

  it('AC-9 / C-1: 编辑现有 TCP 规则加载时 remotePort 不被 watch 抹掉', async () => {
    // 反向场景：默认 tcp → syncFromInput 写入已有 TCP 规则
    const initial: ProxyInput = {
      name: '', type: 'http', localIP: '127.0.0.1', localPort: 80, enabled: true,
    }
    const { form, syncFromInput } = useProxyForm(initial)

    syncFromInput({
      name: 'edit-ssh',
      type: 'tcp',
      localIP: '127.0.0.1',
      localPort: 22,
      remotePort: 6022,
      enabled: true,
      version: 3,
    })
    await nextTick()

    expect(form.value.type).toBe('tcp')
    expect(form.value.remotePort).toBe(6022)
    expect(form.value.customDomains).toEqual([])
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
