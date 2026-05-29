/**
 * 构造一个与真实后端响应同形状的 axios 错误，供测试模拟 API 失败。
 *
 * 真实链路：后端 4xx/5xx 返回 `{ error: { code, message } }` body → axios 抛出
 * 一个 `isAxiosError === true` 且 `response.data` 为该 body 的错误对象。
 * `extractErrorMessage`（src/api/client.ts）只透传这种**结构化**错误的 message，
 * 对普通 `new Error('xxx')` 一律走友好 fallback。因此测试若用 `new Error('凭据失效')`
 * reject，UI 实际拿到的是 fallback 而非该消息 —— 必须用本 helper 才能正确模拟
 * "后端返回带具体原因的错误"这一生产路径（曾因此让 4 个 adversarial 测试误判）。
 *
 * `axios.isAxiosError` 的判定仅为 `isObject(payload) && payload.isAxiosError === true`
 * （见 node_modules/axios/dist/node/axios.cjs），故返回带该 brand 的普通对象即可。
 */
export function apiError(message: string, status = 502, code = 'INTERNAL'): unknown {
  return {
    isAxiosError: true,
    message,
    response: {
      data: { error: { code, message } },
      status,
    },
  }
}
