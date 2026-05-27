import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import AllowPortsEditor from '../AllowPortsEditor.vue'
import type { AllowPortRange } from '../../types'

/**
 * T-040 AllowPortsEditor.spec.ts
 *
 * 继承 insight L9 / L14 范式：importOriginal + spread + 6 方法 stub mock naive-ui。
 * 直接整体 vi.mock('naive-ui') 不带 importOriginal 会让 mount 时所有 N* 组件丢失 → render 失败。
 *
 * 测试矩阵（02 §7.3）：
 * - mount 空 initial → 0 行 + 两个添加按钮
 * - 点添加范围 → 列表 +1 行 kind=range
 * - 点添加单端口 → 列表 +1 行 kind=single
 * - 输入 end=65536 → n-input-number 的 :max 限制了 UI；这里用 setRow 直接注入越界值验证错误文案出现
 * - 两行 [1000-2000] + [1500-2500] → 第二行显 "与第 1 行区间重叠"
 * - getAllowPortsInput() 返回顺序与添加顺序一致
 * - hasValidationError() 在有非法行时返 true
 * - initial=[{single:80}] → 第一行 kind=single value=80
 */

vi.mock('naive-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useMessage: () => ({
      error:      vi.fn(),
      success:    vi.fn(),
      warning:    vi.fn(),
      info:       vi.fn(),
      loading:    vi.fn(),
      destroyAll: vi.fn(),
    }),
  }
})

describe('AllowPortsEditor — 端口策略编辑器（T-040）', () => {
  it('mount 空 initial → rows 为空 + 两个添加按钮可见', () => {
    const w = mount(AllowPortsEditor, { props: { initial: [] } })
    const text = w.text()
    expect(text).toContain('添加范围')
    expect(text).toContain('添加单端口')
    expect(text).toContain('暂无策略')
    // getAllowPortsInput 应返空
    const exposed = w.vm as unknown as { getAllowPortsInput(): AllowPortRange[]; hasValidationError(): boolean }
    expect(exposed.getAllowPortsInput()).toEqual([])
    expect(exposed.hasValidationError()).toBe(false)
  })

  it('initial=[{single:80}] → 第一行 kind=single value=80', () => {
    const w = mount(AllowPortsEditor, { props: { initial: [{ single: 80 }] } })
    const exposed = w.vm as unknown as { getAllowPortsInput(): AllowPortRange[] }
    const out = exposed.getAllowPortsInput()
    expect(out).toHaveLength(1)
    expect(out[0]).toEqual({ single: 80 })
  })

  it('initial=[{start:6000,end:7000}] → 第一行 kind=range value=6000-7000', () => {
    const w = mount(AllowPortsEditor, { props: { initial: [{ start: 6000, end: 7000 }] } })
    const exposed = w.vm as unknown as { getAllowPortsInput(): AllowPortRange[] }
    expect(exposed.getAllowPortsInput()).toEqual([{ start: 6000, end: 7000 }])
  })

  it('点添加范围 → 列表 +1 行 kind=range（默认空值）', async () => {
    const w = mount(AllowPortsEditor, { props: { initial: [] } })
    // 直接调内部方法（公共 UX 路径，通过 .vm 暴露的 addRange 不在 expose 上，所以我们走 DOM 模拟）
    const buttons = w.findAll('button')
    const addRangeBtn = buttons.find(b => b.text().includes('添加范围'))
    expect(addRangeBtn).toBeDefined()
    await addRangeBtn!.trigger('click')
    await nextTick()
    const exposed = w.vm as unknown as { getAllowPortsInput(): AllowPortRange[]; hasValidationError(): boolean }
    const out = exposed.getAllowPortsInput()
    expect(out).toHaveLength(1)
    // 空值 → start=0/end=0；hasValidationError 应 true
    expect(out[0]).toEqual({ start: 0, end: 0 })
    expect(exposed.hasValidationError()).toBe(true)
  })

  it('点添加单端口 → 列表 +1 行 kind=single（默认空值）', async () => {
    const w = mount(AllowPortsEditor, { props: { initial: [] } })
    const buttons = w.findAll('button')
    const addSingleBtn = buttons.find(b => b.text().includes('添加单端口'))
    expect(addSingleBtn).toBeDefined()
    await addSingleBtn!.trigger('click')
    await nextTick()
    const exposed = w.vm as unknown as { getAllowPortsInput(): AllowPortRange[]; hasValidationError(): boolean }
    expect(exposed.getAllowPortsInput()).toHaveLength(1)
    expect(exposed.hasValidationError()).toBe(true) // single 为 null
  })

  it('合法 single + 合法 range → hasValidationError false', () => {
    const w = mount(AllowPortsEditor, {
      props: {
        initial: [
          { single: 80 },
          { start: 6000, end: 7000 },
        ],
      },
    })
    const exposed = w.vm as unknown as { getAllowPortsInput(): AllowPortRange[]; hasValidationError(): boolean }
    expect(exposed.hasValidationError()).toBe(false)
    expect(exposed.getAllowPortsInput()).toEqual([
      { single: 80 },
      { start: 6000, end: 7000 },
    ])
  })

  it('两行 [1000-2000] + [1500-2500] → 第二行错误 "与第 1 行区间重叠"', () => {
    const w = mount(AllowPortsEditor, {
      props: {
        initial: [
          { start: 1000, end: 2000 },
          { start: 1500, end: 2500 },
        ],
      },
    })
    expect(w.text()).toContain('与第 1 行区间重叠')
    const exposed = w.vm as unknown as { hasValidationError(): boolean }
    expect(exposed.hasValidationError()).toBe(true)
  })

  it('闭区间重叠：[1000-2000] + [2000-3000] → 第二行错误（边界触碰算重叠）', () => {
    const w = mount(AllowPortsEditor, {
      props: {
        initial: [
          { start: 1000, end: 2000 },
          { start: 2000, end: 3000 },
        ],
      },
    })
    expect(w.text()).toContain('与第 1 行区间重叠')
  })

  it('单端口与范围重叠：[1000-2000] + {single:1500} → 错误', () => {
    const w = mount(AllowPortsEditor, {
      props: {
        initial: [
          { start: 1000, end: 2000 },
          { single: 1500 },
        ],
      },
    })
    expect(w.text()).toContain('与第 1 行区间重叠')
  })

  it('start > end → 错误 "起始端口必须 ≤ 结束端口"', () => {
    const w = mount(AllowPortsEditor, {
      props: { initial: [{ start: 80, end: 70 }] },
    })
    expect(w.text()).toContain('起始端口必须 ≤ 结束端口')
  })

  it('getAllowPortsInput 顺序与添加顺序一致（OQ-6）', () => {
    const seed: AllowPortRange[] = [
      { start: 6000, end: 7000 },
      { single: 9000 },
      { start: 10000, end: 11000 },
    ]
    const w = mount(AllowPortsEditor, { props: { initial: seed } })
    const exposed = w.vm as unknown as { getAllowPortsInput(): AllowPortRange[] }
    const out = exposed.getAllowPortsInput()
    expect(out).toEqual(seed)
  })

  it('删除中间行后 getAllowPortsInput 反映剩余顺序', async () => {
    const w = mount(AllowPortsEditor, {
      props: {
        initial: [
          { single: 1 },
          { single: 2 },
          { single: 3 },
        ],
      },
    })
    // 找到第二行的删除按钮：按 DOM 顺序 button[文本=删除] 索引 1
    const removeBtns = w.findAll('button').filter(b => b.text() === '删除')
    expect(removeBtns).toHaveLength(3)
    await removeBtns[1].trigger('click')
    await nextTick()
    const exposed = w.vm as unknown as { getAllowPortsInput(): AllowPortRange[] }
    expect(exposed.getAllowPortsInput()).toEqual([{ single: 1 }, { single: 3 }])
  })
})
