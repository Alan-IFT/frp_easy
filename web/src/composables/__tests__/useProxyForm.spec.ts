// T-051 frontend-test-coverage · B-2
// useProxyForm composable 专属测试（T-037 insight L20 点名的"同列名不同语义"高发 bug 区：
// remotePort vs customDomains 是 tcp/udp 与 http/https 两路互斥字段，切 type 时若残留
// 旧值会被错误上送）。
//
// 说明：components/__tests__/ProxyForm.spec.ts 已从组件挂载侧覆盖部分等价行为；
// 本 spec 直接针对 composable 单测：1) watch(type) 触发 handleTypeChange 清残留字段
// （需 await nextTick）；2) toProxyInput 按 isTcpUdp 决定输出 remotePort vs customDomains。
import { describe, it, expect } from 'vitest'
import { nextTick } from 'vue'
import { useProxyForm } from '../useProxyForm'
import type { ProxyInput } from '../../types'

const tcpInput = (): ProxyInput => ({
  name: 'ssh',
  type: 'tcp',
  localIP: '127.0.0.1',
  localPort: 22,
  remotePort: 6000,
  enabled: true,
})

const httpInput = (): ProxyInput => ({
  name: 'web',
  type: 'http',
  localIP: '127.0.0.1',
  localPort: 80,
  customDomains: ['example.com'],
  enabled: true,
})

describe('useProxyForm — 初始化映射', () => {
  it('tcp 输入：form 字段与 initial 一致，isTcpUdp=true', () => {
    const { form, isTcpUdp, isHttpHttps } = useProxyForm(tcpInput())
    expect(form.value.type).toBe('tcp')
    expect(form.value.remotePort).toBe(6000)
    expect(form.value.customDomains).toEqual([])
    expect(isTcpUdp.value).toBe(true)
    expect(isHttpHttps.value).toBe(false)
  })

  it('localIP 缺省 → 回落 127.0.0.1；localPort=0/缺省 → null', () => {
    const { form } = useProxyForm({
      name: 'x',
      type: 'tcp',
      localPort: 0,
    })
    expect(form.value.localIP).toBe('127.0.0.1')
    expect(form.value.localPort).toBeNull()
  })

  it('enabled 未显式 false → 默认 true；version 缺省 → 0', () => {
    const { form } = useProxyForm({ name: 'x', type: 'tcp', localPort: 22 })
    expect(form.value.enabled).toBe(true)
    expect(form.value.version).toBe(0)
  })
})

describe('useProxyForm — watch(type) 切换清残留互斥字段（await nextTick）', () => {
  it('tcp → http：watch 兜底清 remotePort（http 不需要 remotePort）', async () => {
    const { form } = useProxyForm(tcpInput())
    expect(form.value.remotePort).toBe(6000)

    form.value.type = 'http'
    await nextTick()

    // http 路：handleTypeChange 清 remotePort，避免把残留的 6000 上送给后端
    expect(form.value.remotePort).toBeNull()
  })

  it('http → tcp：watch 兜底清 customDomains（tcp 不需要 customDomains）', async () => {
    const { form } = useProxyForm(httpInput())
    expect(form.value.customDomains).toEqual(['example.com'])

    form.value.type = 'tcp'
    await nextTick()

    // tcp 路：handleTypeChange 清 customDomains，根除"同列名不同语义"残留 bug
    expect(form.value.customDomains).toEqual([])
  })

  it('tcp → udp：仍属 isTcpUdp，customDomains 维持空、remotePort 保留', async () => {
    const { form, isTcpUdp } = useProxyForm(tcpInput())
    form.value.type = 'udp'
    await nextTick()

    expect(isTcpUdp.value).toBe(true)
    expect(form.value.remotePort).toBe(6000)
    expect(form.value.customDomains).toEqual([])
  })

  it('http → https：仍属 isHttpHttps，customDomains 保留', async () => {
    const { form, isHttpHttps } = useProxyForm(httpInput())
    form.value.type = 'https'
    await nextTick()

    expect(isHttpHttps.value).toBe(true)
    // https 路同样清 remotePort（与 http 一致），customDomains 该 type 需要，保留
    expect(form.value.customDomains).toEqual(['example.com'])
  })

  it('type 不变（写同值）→ watch 早退，不触发清理', async () => {
    const { form } = useProxyForm(tcpInput())
    form.value.remotePort = 7000
    // 写入相同 type，watch newType===oldType 早退
    form.value.type = 'tcp'
    await nextTick()
    expect(form.value.remotePort).toBe(7000)
  })
})

describe('useProxyForm — handleTypeChange 直接调用（语义化重置）', () => {
  it('handleTypeChange("tcp") 只清 customDomains，remotePort 保留', () => {
    const { form, handleTypeChange } = useProxyForm(tcpInput())
    form.value.remotePort = 8080
    form.value.customDomains = ['stale.com']

    handleTypeChange('tcp')

    expect(form.value.customDomains).toEqual([])
    expect(form.value.remotePort).toBe(8080)
  })

  it('handleTypeChange("http") 只清 remotePort，customDomains 保留', () => {
    const { form, handleTypeChange } = useProxyForm(tcpInput())
    form.value.remotePort = 8080
    form.value.customDomains = ['kept.com']

    handleTypeChange('http')

    expect(form.value.remotePort).toBeNull()
    expect(form.value.customDomains).toEqual(['kept.com'])
  })

  it('handleTypeChange() 不传参 → 用当前 form.type 决定清哪一列', () => {
    const { form, handleTypeChange } = useProxyForm(httpInput())
    form.value.remotePort = 9000
    // 当前 type=http → 应清 remotePort
    handleTypeChange()
    expect(form.value.remotePort).toBeNull()
  })
})

describe('useProxyForm — toProxyInput 按 isTcpUdp 分流输出', () => {
  it('tcp：输出 remotePort，不输出 customDomains', () => {
    const { toProxyInput } = useProxyForm(tcpInput())
    const out = toProxyInput()
    expect(out.remotePort).toBe(6000)
    expect(out.customDomains).toBeUndefined()
    expect(out.localPort).toBe(22)
    expect(out.type).toBe('tcp')
  })

  it('http：输出 customDomains，不输出 remotePort', () => {
    const { toProxyInput } = useProxyForm(httpInput())
    const out = toProxyInput()
    expect(out.customDomains).toEqual(['example.com'])
    expect(out.remotePort).toBeUndefined()
    expect(out.type).toBe('http')
  })

  it('tcp 模式即便 customDomains 被污染也不上送（分支隔离）', () => {
    const { form, toProxyInput } = useProxyForm(tcpInput())
    form.value.customDomains = ['ghost.com']
    const out = toProxyInput()
    expect(out.customDomains).toBeUndefined()
    expect(out.remotePort).toBe(6000)
  })

  it('http 且 customDomains 为空 → customDomains 输出 undefined（不空数组上送）', () => {
    const { form, toProxyInput } = useProxyForm(httpInput())
    form.value.customDomains = []
    const out = toProxyInput()
    expect(out.customDomains).toBeUndefined()
  })

  it('tcp 且 remotePort=null → remotePort 输出 undefined', () => {
    const { form, toProxyInput } = useProxyForm(tcpInput())
    form.value.remotePort = null
    const out = toProxyInput()
    expect(out.remotePort).toBeUndefined()
  })

  it('localPort=null → toProxyInput 回落 0；localIP 空 → 回落 127.0.0.1', () => {
    const { form, toProxyInput } = useProxyForm(tcpInput())
    form.value.localPort = null
    form.value.localIP = ''
    const out = toProxyInput()
    expect(out.localPort).toBe(0)
    expect(out.localIP).toBe('127.0.0.1')
  })
})
