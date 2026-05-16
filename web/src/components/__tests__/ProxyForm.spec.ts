import { describe, it, expect } from 'vitest'
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

  it('handleTypeChange 切换类型时互斥字段被清空', () => {
    const { form, handleTypeChange } = useProxyForm(tcpInput())
    form.value.remotePort = 8080
    form.value.customDomains = ['test.com']

    handleTypeChange()

    expect(form.value.remotePort).toBeNull()
    expect(form.value.customDomains).toHaveLength(0)
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
})
