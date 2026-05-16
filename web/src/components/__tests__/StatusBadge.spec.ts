import { describe, it, expect } from 'vitest'
import { getTagType, getStateLabel } from '../../composables/statusUtils'
import type { ProcessState } from '../../types'

describe('StatusBadge 逻辑', () => {
  describe('getTagType', () => {
    it('running → success', () => {
      expect(getTagType('running' as ProcessState)).toBe('success')
    })

    it('error → error', () => {
      expect(getTagType('error' as ProcessState)).toBe('error')
    })

    it('stopped → default', () => {
      expect(getTagType('stopped' as ProcessState)).toBe('default')
    })

    it('starting → warning', () => {
      expect(getTagType('starting' as ProcessState)).toBe('warning')
    })

    it('stopping → warning', () => {
      expect(getTagType('stopping' as ProcessState)).toBe('warning')
    })
  })

  describe('getStateLabel', () => {
    it('running → 运行中', () => {
      expect(getStateLabel('running' as ProcessState)).toBe('运行中')
    })

    it('stopped → 已停止', () => {
      expect(getStateLabel('stopped' as ProcessState)).toBe('已停止')
    })

    it('error → 错误', () => {
      expect(getStateLabel('error' as ProcessState)).toBe('错误')
    })

    it('starting → 启动中', () => {
      expect(getStateLabel('starting' as ProcessState)).toBe('启动中')
    })

    it('stopping → 停止中', () => {
      expect(getStateLabel('stopping' as ProcessState)).toBe('停止中')
    })
  })
})
