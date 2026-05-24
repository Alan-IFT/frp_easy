import { describe, it, expect } from 'vitest'
import {
  parseLogLine,
  ALL_LEVELS,
} from '../../composables/log/parseLogLine'

// T-036 / GR §7 C-1：parseLogLine 必须能解析真实 frp 日志的两种主流格式
// + 短字母 I/W/E/D/T 映射 + PLAIN 兜底。

describe('parseLogLine — frp 短字母格式（上游标准）', () => {
  it('YYYY/MM/DD HH:MM:SS [I] [subsystem.go:123] message → INFO', () => {
    const r = parseLogLine(
      '2025/01/15 10:23:45 [I] [proxy_manager.go:108] proxy added: [ssh]',
    )
    expect(r.timestamp).toBe('2025/01/15 10:23:45')
    expect(r.level).toBe('INFO')
    expect(r.message).toBe('[proxy_manager.go:108] proxy added: [ssh]')
  })

  it('短字母 W → WARN', () => {
    const r = parseLogLine('2025/01/15 10:23:45 [W] auth failed')
    expect(r.level).toBe('WARN')
    expect(r.message).toBe('auth failed')
  })

  it('短字母 E → ERROR', () => {
    const r = parseLogLine('2025/01/15 10:23:45 [E] connection refused')
    expect(r.level).toBe('ERROR')
  })

  it('短字母 D → DEBUG', () => {
    const r = parseLogLine('2025/01/15 10:23:45 [D] tick')
    expect(r.level).toBe('DEBUG')
  })

  it('短字母 T → TRACE', () => {
    const r = parseLogLine('2025/01/15 10:23:45 [T] trace event')
    expect(r.level).toBe('TRACE')
  })

  it('带毫秒：2025/01/15 10:23:45.123 [I] foo started', () => {
    const r = parseLogLine('2025/01/15 10:23:45.123 [I] foo started')
    expect(r.timestamp).toBe('2025/01/15 10:23:45.123')
    expect(r.level).toBe('INFO')
    expect(r.message).toBe('foo started')
  })
})

describe('parseLogLine — 长全称格式（二次封装变体）', () => {
  it('YYYY-MM-DD HH:MM:SS [INFO] message', () => {
    const r = parseLogLine('2025-01-15 10:23:45 [INFO] foo started')
    expect(r.timestamp).toBe('2025-01-15 10:23:45')
    expect(r.level).toBe('INFO')
    expect(r.message).toBe('foo started')
  })

  it('[ERROR] 全大写命中', () => {
    const r = parseLogLine('2025-01-15 10:23:45 [ERROR] panic')
    expect(r.level).toBe('ERROR')
  })

  it('[WARN] 命中', () => {
    const r = parseLogLine('2025-01-15 10:23:45 [WARN] degraded')
    expect(r.level).toBe('WARN')
  })

  it('[WARNING] 归一到 WARN', () => {
    const r = parseLogLine('2025-01-15 10:23:45 [WARNING] degraded')
    expect(r.level).toBe('WARN')
  })

  it('[DEBUG] / [TRACE] 命中', () => {
    expect(parseLogLine('2025-01-15 10:23:45 [DEBUG] tick').level).toBe('DEBUG')
    expect(parseLogLine('2025-01-15 10:23:45 [TRACE] step').level).toBe('TRACE')
  })

  it('ISO-T 分隔 + 毫秒：2025-01-15T10:23:45.456 [E] msg', () => {
    const r = parseLogLine('2025-01-15T10:23:45.456 [E] msg')
    expect(r.timestamp).toBe('2025-01-15T10:23:45.456')
    expect(r.level).toBe('ERROR')
    expect(r.message).toBe('msg')
  })

  it('大小写不敏感：[info] 也命中', () => {
    const r = parseLogLine('2025-01-15 10:23:45 [info] x')
    expect(r.level).toBe('INFO')
  })

  it('无方括号也命中：YYYY/MM/DD HH:MM:SS I message', () => {
    const r = parseLogLine('2025/01/15 10:23:45 I started')
    expect(r.level).toBe('INFO')
    expect(r.message).toBe('started')
  })
})

describe('parseLogLine — PLAIN 兜底', () => {
  it('panic stack（无时间戳）→ PLAIN', () => {
    const r = parseLogLine('goroutine 1 [running]:')
    expect(r.level).toBe('PLAIN')
    expect(r.message).toBe('goroutine 1 [running]:')
    expect(r.timestamp).toBeUndefined()
  })

  it('空字符串 → PLAIN', () => {
    const r = parseLogLine('')
    expect(r.level).toBe('PLAIN')
    expect(r.message).toBe('')
  })

  it('随机文本 → PLAIN', () => {
    const r = parseLogLine('hello world')
    expect(r.level).toBe('PLAIN')
    expect(r.message).toBe('hello world')
  })

  it('错误日期格式（YYYY/M/D 单位数）→ PLAIN', () => {
    const r = parseLogLine('2025/1/5 10:23:45 [I] msg')
    expect(r.level).toBe('PLAIN')
  })

  it('解析失败时 raw === message', () => {
    const raw = 'something weird'
    const r = parseLogLine(raw)
    expect(r.raw).toBe(raw)
    expect(r.message).toBe(raw)
  })
})

describe('parseLogLine — ALL_LEVELS 列表', () => {
  it('6 个等级（含 PLAIN）', () => {
    expect(ALL_LEVELS).toEqual([
      'ERROR',
      'WARN',
      'INFO',
      'DEBUG',
      'TRACE',
      'PLAIN',
    ])
  })
})
