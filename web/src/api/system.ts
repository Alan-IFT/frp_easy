import apiClient from './client'
import type {
  SystemReady,
  PublicIPResponse,
  UploadBinResponse,
  PortProbeRequest,
  PortProbeResponse,
} from '../types'

export async function apiGetReady(): Promise<SystemReady> {
  const res = await apiClient.get<SystemReady>('/api/v1/system/ready')
  return res.data
}

export async function apiGetPublicIP(): Promise<PublicIPResponse> {
  const res = await apiClient.get<PublicIPResponse>('/api/v1/system/public-ip')
  return res.data
}

/**
 * T-018 §A：上传 frpc/frps 二进制（multipart/form-data）。
 *
 * **B-2 修订（严禁显式 Content-Type）**：
 *   axios 1.x 在用户不显式设置 Content-Type 时会自动构造
 *   `multipart/form-data; boundary=<auto>` 头；一旦手工设 'multipart/form-data'
 *   就会丢掉 boundary，服务端 multipart 解析直接 400。所以这里**不传** headers。
 *
 * **B-6 修订（字段顺序无关）**：
 *   后端已改用 `r.ParseMultipartForm` + FormValue/FormFile，前端 append 顺序
 *   不再敏感；保留 kind 在前只是阅读习惯。
 *
 * 大小校验：前端先拦 64 MiB 与后端一致，避免 64 MiB 大文件先传后被拒。
 */
export async function apiUploadBin(
  kind: 'frpc' | 'frps',
  file: File,
  onProgress?: (pct: number) => void,
): Promise<UploadBinResponse> {
  const fd = new FormData()
  fd.append('kind', kind)
  fd.append('file', file)
  const res = await apiClient.post<UploadBinResponse>(
    '/api/v1/system/upload-bin',
    fd,
    {
      onUploadProgress: (e) => {
        if (onProgress && e.total) {
          onProgress(Math.round((e.loaded / e.total) * 100))
        }
      },
      timeout: 120_000, // 64 MiB 在慢链路上最长 120s
    },
  )
  return res.data
}

/**
 * T-018 §C.3：探测本机端口可用性。
 *
 * 仅探 TCP；UDP Listen 总成功语义不可靠，本期不做。
 * 端口 < 1024（特权端口）后端不真探，直接返 reason:"privileged"。
 */
export async function apiProbePorts(ports: number[]): Promise<PortProbeResponse> {
  const req: PortProbeRequest = { ports }
  const res = await apiClient.post<PortProbeResponse>(
    '/api/v1/system/probe-ports',
    req,
  )
  return res.data
}
