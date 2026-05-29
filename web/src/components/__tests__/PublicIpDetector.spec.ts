// T-048 / frontend-consistency-cleanup · A4
// PublicIpDetector.vue — catch 改用 extractErrorMessage 透传后端精确原因。
//
// 关键模式：
//   - mock ../../api/system::apiGetPublicIP 让 mount 不触网
//   - API 失败用 apiError()（结构化错误，extractErrorMessage 才透传 message；
//     new Error 会走 fallback —— 见 test-utils/apiError.ts 注释）
//   - 句柄读 getExposed 不需要（直接断言渲染文本：n-alert "检测失败" 区域）

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { h, defineComponent, nextTick } from 'vue'
import { NMessageProvider } from 'naive-ui'
import { apiError } from '../../test-utils/apiError'
import PublicIpDetector from '../PublicIpDetector.vue'
import * as systemApi from '../../api/system'

vi.mock('../../api/system', async (orig) => {
  const actual = await orig<typeof import('../../api/system')>()
  return {
    ...actual,
    apiGetPublicIP: vi.fn(),
  }
})

const getIpMock = vi.mocked(systemApi.apiGetPublicIP)

function withProvider() {
  const Wrapper = defineComponent({
    setup() {
      return () => h(NMessageProvider, null, {
        default: () => h(PublicIpDetector),
      })
    },
  })
  return mount(Wrapper, { attachTo: document.body })
}

async function settle(n = 6): Promise<void> {
  for (let i = 0; i < n; i++) await nextTick()
}

beforeEach(() => {
  getIpMock.mockReset()
})

describe('PublicIpDetector.vue — happy path', () => {
  it('成功 → 显示检测到的 IP', async () => {
    getIpMock.mockResolvedValue({ ip: '203.0.113.7' })
    const w = withProvider()
    await w.find('button').trigger('click')
    await settle()
    expect(w.text()).toContain('203.0.113.7')
  })
})

describe('PublicIpDetector.vue — 错误透传（A4）', () => {
  it('apiGetPublicIP reject（apiError）→ 展示后端精确原因，而非写死 fallback', async () => {
    getIpMock.mockRejectedValue(apiError('上游探测服务 503：稍后再试'))
    const w = withProvider()
    await w.find('button').trigger('click')
    await settle()
    // A4 关键对齐：不再丢弃后端原因
    expect(w.text()).toContain('上游探测服务 503：稍后再试')
    expect(w.text()).not.toContain('请求失败，请稍后重试')
  })
})

// ## Adversarial tests
describe('PublicIpDetector.vue — Adversarial', () => {
  it('非结构化错误（无 response.data.error）→ 回落友好 fallback（不外泄裸 Error message）', async () => {
    // 普通 Error 不是 axios 结构化错误 → extractErrorMessage 走 fallback 分支
    getIpMock.mockRejectedValue(new Error('TypeError: undefined is not a function'))
    const w = withProvider()
    await w.find('button').trigger('click')
    await settle()
    expect(w.text()).toContain('请求失败，请稍后重试')
    // 不得把内部 Error 裸消息暴露给用户
    expect(w.text()).not.toContain('undefined is not a function')
  })

  it('成功后再失败：error 文案随当前结果更新，不残留旧 IP', async () => {
    getIpMock.mockResolvedValueOnce({ ip: '198.51.100.9' })
    const w = withProvider()
    await w.find('button').trigger('click')
    await settle()
    expect(w.text()).toContain('198.51.100.9')

    getIpMock.mockRejectedValueOnce(apiError('凭据失效'))
    await w.find('button').trigger('click')
    await settle()
    expect(w.text()).toContain('凭据失效')
    expect(w.text()).not.toContain('198.51.100.9')
  })
})
