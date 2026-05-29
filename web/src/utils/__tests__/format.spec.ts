// T-042 / proxy-runtime-status-merge · 02 § 3.5 / AC-7
// formatBytes + formatTime 边界值 unit test。

import { describe, it, expect } from 'vitest'
import { formatBytes, formatTime } from '../format'

describe('formatBytes', () => {
  it('0 → "0 B"', () => {
    expect(formatBytes(0)).toBe('0 B')
  })

  it('1 → "1 B"', () => {
    expect(formatBytes(1)).toBe('1 B')
  })

  it('1023 → "1023 B"', () => {
    expect(formatBytes(1023)).toBe('1023 B')
  })

  it('1024 → "1 KiB"', () => {
    expect(formatBytes(1024)).toBe('1 KiB')
  })

  it('1536 → "1.5 KiB"', () => {
    expect(formatBytes(1536)).toBe('1.5 KiB')
  })

  it('1 MiB → "1 MiB"', () => {
    expect(formatBytes(1024 * 1024)).toBe('1 MiB')
  })

  it('1 GiB → "1 GiB"', () => {
    expect(formatBytes(1024 * 1024 * 1024)).toBe('1 GiB')
  })

  it('1 TiB → "1 TiB"', () => {
    expect(formatBytes(1024 ** 4)).toBe('1 TiB')
  })

  it('Number.MAX_SAFE_INTEGER → 钳在 PiB 单位（不溢出）', () => {
    const s = formatBytes(Number.MAX_SAFE_INTEGER)
    expect(s).toMatch(/PiB$/)
  })

  it('undefined → "—"', () => {
    expect(formatBytes(undefined)).toBe('—')
  })

  it('null → "—"', () => {
    expect(formatBytes(null)).toBe('—')
  })

  it('NaN → "—"', () => {
    expect(formatBytes(Number.NaN)).toBe('—')
  })

  it('负数（防御）→ "—"', () => {
    expect(formatBytes(-1)).toBe('—')
    expect(formatBytes(-1024)).toBe('—')
  })

  it('小数 byte（如来自 frps int64 部分场景）保留 1 位精度并去除 .0 尾巴', () => {
    expect(formatBytes(2048)).toBe('2 KiB')  // 整数 KiB 不带小数
    expect(formatBytes(2560)).toBe('2.5 KiB')  // 半数 KiB 带 1 位
  })
})

describe('formatTime', () => {
  it('空字符串 → "—"', () => {
    expect(formatTime('')).toBe('—')
  })

  it('null → "—"', () => {
    expect(formatTime(null)).toBe('—')
  })

  it('undefined → "—"', () => {
    expect(formatTime(undefined)).toBe('—')
  })

  it('"0001-01-01 00:00:00" → "—"（frps 上游空值 sentinel）', () => {
    expect(formatTime('0001-01-01 00:00:00')).toBe('—')
  })

  it('"0001-..." 任意尾巴 → "—"', () => {
    expect(formatTime('0001-99')).toBe('—')
  })

  // T-048 C5：formatTime 现在统一本地化（toLocaleString('zh-CN', { hour12:false })），
  // 不再原样返回裸字符串。断言用稳定特征（含年份、不含裸 ISO 的 'T...Z'）避免时区脆弱。
  it('本地字符串 → 本地化（含年份，不再原样返回）', () => {
    const out = formatTime('2025-01-15 10:23:45')
    expect(out).toContain('2025')
    expect(out).not.toBe('—')
  })

  it('ISO 字符串 → 本地化（不再外泄裸 "T...Z" ISO）', () => {
    const out = formatTime('2026-05-28T01:00:00Z')
    expect(out).toContain('2026')
    // 关键对齐：不得把裸 ISO（含 'T' 和 'Z'）直接展示给用户
    expect(out).not.toMatch(/T\d{2}:\d{2}:\d{2}Z/)
  })

  it('无法解析的字符串（防御）→ "—"（不外泄 "Invalid Date"）', () => {
    expect(formatTime('not-a-real-date')).toBe('—')
    expect(formatTime('hello world')).toBe('—')
  })

  it('固定时间戳 → 稳定本地化（用 Date 构造规避时区脆弱）', () => {
    // 用一个本地构造的字符串，断言输出 = JS 自身的 toLocaleString（同一引擎/时区）
    const iso = '2026-05-28T01:00:00Z'
    const expected = new Date(iso).toLocaleString('zh-CN', { hour12: false })
    expect(formatTime(iso)).toBe(expected)
  })
})
