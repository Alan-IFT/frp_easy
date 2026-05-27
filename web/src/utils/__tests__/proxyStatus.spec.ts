// T-042 / proxy-runtime-status-merge · 02 § 3.5 / AC-8
// getProxyStatusTag 大小写防御 + 语义合并 unit test。

import { describe, it, expect } from 'vitest'
import { getProxyStatusTag } from '../proxyStatus'

describe('getProxyStatusTag — 在线状态', () => {
  it('"online" → success / "在线" / online=true', () => {
    const v = getProxyStatusTag('online')
    expect(v.type).toBe('success')
    expect(v.text).toBe('在线')
    expect(v.online).toBe(true)
    expect(v.dotColor).toBe('#18a058')
  })

  it('"Online"（首字大写）→ success / "在线"', () => {
    const v = getProxyStatusTag('Online')
    expect(v.type).toBe('success')
    expect(v.text).toBe('在线')
    expect(v.online).toBe(true)
  })

  it('"ONLINE"（全大写）→ success / "在线"', () => {
    const v = getProxyStatusTag('ONLINE')
    expect(v.type).toBe('success')
    expect(v.text).toBe('在线')
  })
})

describe('getProxyStatusTag — 离线 / 空 / null / undefined 语义合并', () => {
  it('"offline" → default / "离线" / online=false', () => {
    const v = getProxyStatusTag('offline')
    expect(v.type).toBe('default')
    expect(v.text).toBe('离线')
    expect(v.online).toBe(false)
    expect(v.dotColor).toBe('#999999')
  })

  it('"Offline" → default / "离线"', () => {
    const v = getProxyStatusTag('Offline')
    expect(v.type).toBe('default')
    expect(v.text).toBe('离线')
  })

  it('空字符串 → default / "离线"（与"无此 proxy"语义合并）', () => {
    const v = getProxyStatusTag('')
    expect(v.type).toBe('default')
    expect(v.text).toBe('离线')
  })

  it('null → default / "离线"', () => {
    const v = getProxyStatusTag(null)
    expect(v.type).toBe('default')
    expect(v.text).toBe('离线')
  })

  it('undefined → default / "离线"', () => {
    const v = getProxyStatusTag(undefined)
    expect(v.type).toBe('default')
    expect(v.text).toBe('离线')
  })
})

describe('getProxyStatusTag — 其它状态 → error 兜底（保留原文）', () => {
  it('"error" → error / 原文 "error"', () => {
    const v = getProxyStatusTag('error')
    expect(v.type).toBe('error')
    expect(v.text).toBe('error')
    expect(v.online).toBe(false)
    expect(v.dotColor).toBe('#d03050')
  })

  it('"unknown_state" → error / 原文', () => {
    const v = getProxyStatusTag('unknown_state')
    expect(v.type).toBe('error')
    expect(v.text).toBe('unknown_state')
  })

  it('"中文状态" → error / 原文（不丢失上游字面）', () => {
    const v = getProxyStatusTag('启动失败')
    expect(v.type).toBe('error')
    expect(v.text).toBe('启动失败')
  })
})
