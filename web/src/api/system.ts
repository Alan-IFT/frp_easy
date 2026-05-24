import apiClient from './client'
import type {
  SystemReady,
  PublicIPResponse,
  UploadBinResponse,
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
 * **Content-Type 处理（T-023 修复）**：
 *   `apiClient` 在 [client.ts](./client.ts) 设了实例级默认 `Content-Type: application/json`，
 *   FormData 请求会继承该 default → axios 1.x 将其视作"用户显式设置"，于是
 *   **不再**自动构造 `multipart/form-data; boundary=<auto>`，服务端报
 *   `请求不是合法的 multipart/form-data` 400。
 *
 *   修复：在本请求显式把 `Content-Type` 设为 `undefined`，抵消实例 default；
 *   axios 检测到 FormData + 无 Content-Type 后会自动补上正确的
 *   `multipart/form-data; boundary=<auto>` 头。
 *
 *   注意：不要写 `headers: { 'Content-Type': 'multipart/form-data' }` —— 缺
 *   boundary 等于把 multipart 标记空开，部分服务端 / axios 旧版会拒。`undefined`
 *   是文档化的"让 axios 自己来"信号。
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
      // 关键：抵消 apiClient 实例 default 的 application/json，让 axios 自动从
      // FormData 推导 multipart/form-data; boundary=<auto>。详见上方注释。
      headers: { 'Content-Type': undefined },
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
