// T-051 frontend-test-coverage · B-5
// api/client.ts 是所有请求的公共层（最关键）。本 spec 不接真实网络：用 axios 的
// per-instance adapter（apiClient.defaults.adapter）合成 200 / 4xx / 5xx / 401 响应，
// 从而走完真实拦截器链（请求侧 CSRF 注入 + 响应侧 401 跳转），并覆盖纯函数
// extractApiError / extractErrorMessage 的 axios 错误 vs 普通 Error 两条分支（T-043 契约）。
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import axios from 'axios'
import type { AxiosAdapter, AxiosResponse } from 'axios'
import apiClient, {
  setCsrfTokenGetter,
  extractApiError,
  extractErrorMessage,
} from '../client'

// 合成一个成功响应（绕过网络）
function ok200(data: unknown): AxiosAdapter {
  return (config) =>
    Promise.resolve<AxiosResponse>({
      data,
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
      request: {},
    })
}

// 合成一个 axios 错误响应（4xx/5xx）：自定义 adapter 必须自行 reject，
// 否则 axios 不会把非 2xx 自动转 AxiosError（settle 是内建 adapter 的职责）。
function errStatus(status: number, body: unknown): AxiosAdapter {
  return (config) =>
    Promise.reject(
      new axios.AxiosError(
        `Request failed with status code ${status}`,
        status >= 500 ? 'ERR_BAD_RESPONSE' : 'ERR_BAD_REQUEST',
        config,
        {},
        {
          data: body,
          status,
          statusText: String(status),
          headers: {},
          config,
          request: {},
        } as AxiosResponse,
      ),
    )
}

const originalAdapter = apiClient.defaults.adapter

afterEach(() => {
  apiClient.defaults.adapter = originalAdapter
  // 还原 CSRF getter（避免泄漏到其它 spec）
  setCsrfTokenGetter(() => '')
})

describe('apiClient — 成功 200 解 JSON', () => {
  it('GET 200 → res.data 为后端 JSON', async () => {
    apiClient.defaults.adapter = ok200({ hello: 'world', n: 42 })
    const res = await apiClient.get('/api/v1/whatever')
    expect(res.status).toBe(200)
    expect(res.data).toEqual({ hello: 'world', n: 42 })
  })
})

describe('apiClient — CSRF token 注入请求头', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('setCsrfTokenGetter 返非空 → 请求头带 X-CSRF-Token', async () => {
    setCsrfTokenGetter(() => 'tok-abc123')
    let seenHeader: string | undefined
    apiClient.defaults.adapter = (config) => {
      seenHeader = config.headers?.['X-CSRF-Token'] as string | undefined
      return ok200({})(config)
    }
    await apiClient.post('/api/v1/proxies', { name: 'x' })
    expect(seenHeader).toBe('tok-abc123')
  })

  it('getter 返空串 → 不注入该头', async () => {
    setCsrfTokenGetter(() => '')
    let hasHeader = true
    apiClient.defaults.adapter = (config) => {
      hasHeader = config.headers?.['X-CSRF-Token'] !== undefined
      return ok200({})(config)
    }
    await apiClient.get('/api/v1/proxies')
    expect(hasHeader).toBe(false)
  })
})

describe('apiClient — 4xx/5xx 抛错且 extractErrorMessage 能取后端 message', () => {
  it('500 携带 { error: { message } } → 抛错，extractErrorMessage 透传后端 message', async () => {
    apiClient.defaults.adapter = errStatus(500, {
      error: { code: 'INTERNAL', message: '后端炸了' },
    })
    let caught: unknown
    try {
      await apiClient.get('/api/v1/proxies')
    } catch (e) {
      caught = e
    }
    expect(caught).toBeTruthy()
    expect(axios.isAxiosError(caught)).toBe(true)
    expect(extractErrorMessage(caught)).toBe('后端炸了')
  })

  it('409 冲突 message 也能取出', async () => {
    apiClient.defaults.adapter = errStatus(409, {
      error: { code: 'CONFLICT', message: '端口已占用' },
    })
    await expect(apiClient.post('/api/v1/proxies', {})).rejects.toBeTruthy()
    try {
      await apiClient.post('/api/v1/proxies', {})
    } catch (e) {
      expect(extractErrorMessage(e)).toBe('端口已占用')
    }
  })
})

describe('apiClient — 401 触发重定向逻辑（响应拦截器）', () => {
  let hrefSink: string
  let savedLocation: Location

  beforeEach(() => {
    savedLocation = window.location
    hrefSink = ''
  })

  afterEach(() => {
    // 还原 location
    Object.defineProperty(window, 'location', {
      configurable: true,
      writable: true,
      value: savedLocation,
    })
  })

  function stubLocation(pathname: string) {
    Object.defineProperty(window, 'location', {
      configurable: true,
      writable: true,
      value: {
        pathname,
        get href() {
          return hrefSink
        },
        set href(v: string) {
          hrefSink = v
        },
      } as unknown as Location,
    })
  }

  it('401 且当前不在 /login → href 被设为 /login', async () => {
    stubLocation('/proxies')
    apiClient.defaults.adapter = errStatus(401, { error: { code: 'UNAUTH', message: '未登录' } })
    await expect(apiClient.get('/api/v1/proxies')).rejects.toBeTruthy()
    expect(hrefSink).toBe('/login')
  })

  it('401 但当前已在 /login → 不再跳转（避免循环）', async () => {
    stubLocation('/login')
    apiClient.defaults.adapter = errStatus(401, { error: { code: 'UNAUTH', message: '未登录' } })
    await expect(apiClient.get('/api/v1/auth/me')).rejects.toBeTruthy()
    expect(hrefSink).toBe('')
  })

  it('401 但当前在 /setup → 不跳转', async () => {
    stubLocation('/setup')
    apiClient.defaults.adapter = errStatus(401, { error: { code: 'UNAUTH', message: '未登录' } })
    await expect(apiClient.get('/api/v1/system/ready')).rejects.toBeTruthy()
    expect(hrefSink).toBe('')
  })

  it('500（非 401）不触发跳转', async () => {
    stubLocation('/proxies')
    apiClient.defaults.adapter = errStatus(500, { error: { code: 'X', message: 'boom' } })
    await expect(apiClient.get('/api/v1/proxies')).rejects.toBeTruthy()
    expect(hrefSink).toBe('')
  })

  it('401 错误仍向调用方 reject（拦截器不吞错）', async () => {
    stubLocation('/proxies')
    apiClient.defaults.adapter = errStatus(401, { error: { code: 'UNAUTH', message: '未登录' } })
    await expect(apiClient.get('/api/v1/proxies')).rejects.toBeTruthy()
  })
})

describe('extractApiError — axios 错误 vs 普通 Error 分支', () => {
  it('结构化 axios 错误（带 response.data）→ 返回 body', () => {
    const err = new axios.AxiosError('failed', 'ERR', undefined, {}, {
      data: { error: { code: 'X', message: '具体原因' } },
      status: 500,
      statusText: '500',
      headers: {},
      config: {} as never,
    } as AxiosResponse)
    const out = extractApiError(err)
    expect(out).toEqual({ error: { code: 'X', message: '具体原因' } })
  })

  it('普通 Error（无 response）→ null', () => {
    expect(extractApiError(new Error('plain'))).toBeNull()
  })

  it('非对象（字符串）→ null', () => {
    expect(extractApiError('oops')).toBeNull()
  })

  it('axios 错误但无 response → null', () => {
    const err = new axios.AxiosError('no response', 'ERR_NETWORK')
    expect(extractApiError(err)).toBeNull()
  })
})

describe('extractErrorMessage — 透传 vs fallback（T-043 契约）', () => {
  it('结构化错误 → 透传后端 error.message', () => {
    const err = new axios.AxiosError('failed', 'ERR', undefined, {}, {
      data: { error: { code: 'X', message: '凭据失效' } },
      status: 401,
      statusText: '401',
      headers: {},
      config: {} as never,
    } as AxiosResponse)
    expect(extractErrorMessage(err)).toBe('凭据失效')
  })

  it('普通 Error → 默认 fallback "操作失败"', () => {
    // 刻意测 fallback：普通 Error 不带 response.data.error.message。
    expect(extractErrorMessage(new Error('凭据失效'))).toBe('操作失败')
  })

  it('普通 Error + 自定义 fallback → 用自定义文案', () => {
    expect(extractErrorMessage(new Error('x'), '加载代理规则失败')).toBe('加载代理规则失败')
  })

  it('axios 错误但 body 无 error.message → fallback', () => {
    const err = new axios.AxiosError('failed', 'ERR', undefined, {}, {
      data: { somethingElse: true },
      status: 500,
      statusText: '500',
      headers: {},
      config: {} as never,
    } as AxiosResponse)
    expect(extractErrorMessage(err, '兜底文案')).toBe('兜底文案')
  })
})
