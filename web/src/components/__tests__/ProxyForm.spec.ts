import { describe, it, expect } from 'vitest'
import { useProxyForm } from '../../composables/useProxyForm'
import type { ProxyInput } from '../../types'

describe('useProxyForm composable（ProxyForm ロジック）', () => {
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

  it('type=tcp のとき isTcpUdp が true になる', () => {
    const { isTcpUdp, isHttpHttps } = useProxyForm(tcpInput())
    expect(isTcpUdp.value).toBe(true)
    expect(isHttpHttps.value).toBe(false)
  })

  it('type=udp のとき isTcpUdp が true になる', () => {
    const udpInput: ProxyInput = { ...tcpInput(), type: 'udp' }
    const { isTcpUdp, isHttpHttps } = useProxyForm(udpInput)
    expect(isTcpUdp.value).toBe(true)
    expect(isHttpHttps.value).toBe(false)
  })

  it('type=http のとき isHttpHttps が true になる', () => {
    const { isTcpUdp, isHttpHttps } = useProxyForm(httpInput())
    expect(isTcpUdp.value).toBe(false)
    expect(isHttpHttps.value).toBe(true)
  })

  it('type=https のとき isHttpHttps が true になる', () => {
    const httpsInput: ProxyInput = { ...httpInput(), type: 'https' }
    const { isTcpUdp, isHttpHttps } = useProxyForm(httpsInput)
    expect(isTcpUdp.value).toBe(false)
    expect(isHttpHttps.value).toBe(true)
  })

  it('handleTypeChange でタイプ変更時に相互排他フィールドがクリアされる', () => {
    const { form, handleTypeChange } = useProxyForm(tcpInput())
    form.value.remotePort = 8080
    form.value.customDomains = ['test.com']

    handleTypeChange()

    expect(form.value.remotePort).toBeNull()
    expect(form.value.customDomains).toHaveLength(0)
  })

  it('toProxyInput が tcp 型の場合 remotePort を含む', () => {
    const { toProxyInput } = useProxyForm(tcpInput())
    const output = toProxyInput()
    expect(output.remotePort).toBe(6000)
    expect(output.customDomains).toBeUndefined()
  })

  it('toProxyInput が http 型の場合 customDomains を含む', () => {
    const { toProxyInput } = useProxyForm(httpInput())
    const output = toProxyInput()
    expect(output.customDomains).toEqual(['example.com'])
    expect(output.remotePort).toBeUndefined()
  })

  it('syncFromInput で外部からフォームを更新できる', () => {
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
