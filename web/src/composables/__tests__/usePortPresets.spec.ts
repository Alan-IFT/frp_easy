import { describe, it, expect } from 'vitest'
import { PORT_PRESETS, filterPresetsByType } from '../usePortPresets'

describe('T-018 §C.2 PORT_PRESETS 常用端口预设', () => {
  it('清单非空且每条字段齐备', () => {
    expect(PORT_PRESETS.length).toBeGreaterThan(0)
    for (const p of PORT_PRESETS) {
      expect(p.label).toBeTruthy()
      expect(typeof p.port).toBe('number')
      expect(p.port).toBeGreaterThanOrEqual(1)
      expect(p.port).toBeLessThanOrEqual(65535)
      expect(['tcp', 'udp', 'http', 'https']).toContain(p.type)
      expect(p.suggestedName).toMatch(/^[A-Za-z0-9_-]+$/)
    }
  })

  it('必含 SSH 22 / RDP 3389 / HTTP 80 / HTTPS 443 / MySQL 3306', () => {
    const ports = PORT_PRESETS.map((p) => p.port)
    expect(ports).toContain(22)
    expect(ports).toContain(3389)
    expect(ports).toContain(80)
    expect(ports).toContain(443)
    expect(ports).toContain(3306)
  })

  it('必含 PostgreSQL 5432 / Redis 6379 / MongoDB 27017 / SMB 445 / VNC 5900', () => {
    const ports = PORT_PRESETS.map((p) => p.port)
    expect(ports).toContain(5432)
    expect(ports).toContain(6379)
    expect(ports).toContain(27017)
    expect(ports).toContain(445)
    expect(ports).toContain(5900)
  })

  it('suggestedName 与 port 后缀对齐（便于派生 -<port> 命名）', () => {
    for (const p of PORT_PRESETS) {
      expect(p.suggestedName).toMatch(new RegExp(`-${p.port}$`))
    }
  })

  it('filterPresetsByType(tcp) 仅返回 TCP 项', () => {
    const tcp = filterPresetsByType(PORT_PRESETS, 'tcp')
    expect(tcp.length).toBeGreaterThan(0)
    expect(tcp.every((p) => p.type === 'tcp')).toBe(true)
  })

  it('filterPresetsByType(http) 仅返回 HTTP 项', () => {
    const http = filterPresetsByType(PORT_PRESETS, 'http')
    expect(http.every((p) => p.type === 'http')).toBe(true)
  })
})
