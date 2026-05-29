// T-051 frontend-test-coverage · B-3
// statusUtils 纯函数 table-driven 全 ProcessState 枚举覆盖。
import { describe, it, expect } from 'vitest'
import { getTagType, getStateLabel } from '../statusUtils'
import type { ProcessState } from '../../types'

describe('getTagType — 全 ProcessState 枚举', () => {
  const cases: Array<[ProcessState, string]> = [
    ['running', 'success'],
    ['error', 'error'],
    ['starting', 'warning'],
    ['stopping', 'warning'],
    ['stopped', 'default'],
  ]

  it.each(cases)('state=%s → tagType=%s', (state, expected) => {
    expect(getTagType(state)).toBe(expected)
  })

  it('未知 state（越界）→ default 兜底', () => {
    // 故意传越界值测 switch default 分支
    expect(getTagType('frozen' as ProcessState)).toBe('default')
  })
})

describe('getStateLabel — 全 ProcessState 枚举中文标签', () => {
  const cases: Array<[ProcessState, string]> = [
    ['stopped', '已停止'],
    ['starting', '启动中'],
    ['running', '运行中'],
    ['stopping', '停止中'],
    ['error', '错误'],
  ]

  it.each(cases)('state=%s → label=%s', (state, expected) => {
    expect(getStateLabel(state)).toBe(expected)
  })

  it('未知 state → 原样返回（?? state 兜底）', () => {
    expect(getStateLabel('mystery' as ProcessState)).toBe('mystery')
  })
})
