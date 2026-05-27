// T-041 / server-monitor-page-ui · 02 §3.2
// 消费 T-039 后端 REST：/api/v1/server/runtime/{info,proxies,traffic/{name}}
//
// 后端 handler 错误映射（与 T-039 handlers_server_runtime.go 对齐）：
//   - 503 + "frps dashboard 未启用..."         → 用户未在 Server 页启用 dashboard
//   - 503 + "frps 进程不可达..."               → frps 没跑 / dashboard 端口不通
//   - 502 + "frps dashboard 凭据校验失败..."   → frps.toml 凭据与 KV 不一致
//   - 404                                      → 单条 proxy / traffic name 不存在（仅 detail / traffic）
//
// 调用方（useServerRuntime composable）通过 extractErrorMessage 统一提取文案，
// 无须本层做 sentinel 类型断言。

import apiClient from './client'
import type {
  ServerRuntimeInfo,
  ServerRuntimeProxiesResponse,
  ServerRuntimeTraffic,
} from '../types'

/** GET /api/v1/server/runtime/info — frps 全局元信息 + 总流量 */
export async function apiGetServerRuntimeInfo(): Promise<ServerRuntimeInfo> {
  const res = await apiClient.get<ServerRuntimeInfo>('/api/v1/server/runtime/info')
  return res.data
}

/**
 * GET /api/v1/server/runtime/proxies — 聚合 N 个 type 的 proxy 列表。
 * 部分 type 失败时整体仍 200，errors[type] 透传给 UI 分 tab 展示；
 * 全 type 失败（凭据 / 连接）后端走 5xx，由 axios 抛出 → composable catch。
 */
export async function apiGetServerRuntimeProxies(): Promise<ServerRuntimeProxiesResponse> {
  const res = await apiClient.get<ServerRuntimeProxiesResponse>('/api/v1/server/runtime/proxies')
  return res.data
}

/**
 * GET /api/v1/server/runtime/traffic/{name} — 单条 proxy 流量时序。
 *
 * 本任务（T-041）暂不消费；导出供 T-042 / 后续 detail 抽屉使用。
 * name 经过 encodeURIComponent 包装，允许包含特殊字符的 proxy 名。
 */
export async function apiGetServerRuntimeTraffic(name: string): Promise<ServerRuntimeTraffic> {
  const encoded = encodeURIComponent(name)
  const res = await apiClient.get<ServerRuntimeTraffic>(`/api/v1/server/runtime/traffic/${encoded}`)
  return res.data
}
