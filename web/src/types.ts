// -----------------------------------------------------------------------
// frp_easy 前端公共类型定义
// 与后端 API 契约 (02 §5.2) 严格保持一致
// -----------------------------------------------------------------------

export type ProcessState = 'stopped' | 'starting' | 'running' | 'stopping' | 'error'

export interface ProcessInfo {
  kind: string
  state: ProcessState
  pid: number
  lastErr?: string
  changedAt: string
}

export interface ProxyInput {
  name: string
  type: 'tcp' | 'udp' | 'http' | 'https'
  localIP?: string
  localPort: number
  remotePort?: number
  customDomains?: string[]
  enabled?: boolean
  version?: number  // PUT 时必填（乐观锁）
}

export interface Proxy {
  id: number
  name: string
  type: 'tcp' | 'udp' | 'http' | 'https'
  localIP: string
  localPort: number
  remotePort?: number
  customDomains?: string[]
  enabled: boolean
  version: number
  updatedAt: string
}

export interface SystemReady {
  initialized: boolean
  binMissing: string[]
  version: string
}

export interface ModeState {
  frpc: boolean
  frps: boolean
}

export interface FrpsConfig {
  bindPort: number
  authMethod?: string
  authToken?: string
  dashboardEnabled?: boolean
  dashboardAddr?: string
  dashboardPort?: number
  dashboardUser?: string
  dashboardPass?: string
}

export interface FrpcServerConn {
  serverAddr: string
  serverPort: number
  authMethod?: string
  authToken?: string
}

export interface ApiErrorDetail {
  code: string
  message: string
  field?: string
}

export interface ApiErrorResponse {
  error: ApiErrorDetail
}

export interface LoginResponse {
  ok: boolean
}

export interface MeResponse {
  username: string
}

export interface CsrfResponse {
  csrfToken: string
}

export interface LogsTailResponse {
  lines: string[]
}

export interface LogsIncrementalResponse {
  data: string
  nextOffset: number
}

export interface DownloadState {
  // T-027：新增 'canceled'，与后端 downloader.StatusCanceled / openapi.yaml DownloadState.status enum 一致。
  status: 'idle' | 'downloading' | 'success' | 'failed' | 'canceled'
  progress: number
  error?: string
}

export interface PublicIPResponse {
  ip?: string
  error?: string
  advisory?: string
  // T-018 §B.1：来源标识，可选展示。例 "ipify" / "ip.cn" / "env"
  source?: string
}

// ---------------------------------------------------------------
// T-018 §A：二进制上传
// ---------------------------------------------------------------

export interface UploadBinResponse {
  ok: boolean
  kind?: 'frpc' | 'frps'
  /** 后端返回的相对仓库 root 的子路径，例 "frp_linux/frpc"（不暴露绝对路径）。 */
  path: string
  /** 落盘字节数（后端契约字段名 size）。 */
  size: number
  /** 落盘后的 sha256 hex；用户可与官方 release 校验。 */
  sha256: string
  /** 上传时 frpc/frps 正在运行时返回的提示；可选。 */
  advisory?: string
}

// ---------------------------------------------------------------
// T-018 §C.1：批量端口创建
// ---------------------------------------------------------------

export interface BatchProxiesRequest {
  /** 派生 name 的前缀，^[A-Za-z0-9_-]{1,58}$。 */
  basename: string
  /** 仅 tcp / udp 支持批量（http/https 走域名，无意义）。 */
  type: 'tcp' | 'udp' | 'http' | 'https'
  /** 可选，默认 127.0.0.1。 */
  localIP?: string
  /** 端口表达式，例 "6000-6010,7000"。本地与远程端口 1:1。 */
  portsExpr: string
  /** 可选，默认 true。 */
  enabled?: boolean
}

export interface BatchProxiesResponse {
  /** 实际创建条数。 */
  created: number
  /** 新建条目，结构与单条 ProxyResponse 一致。 */
  items: Proxy[]
}

// ---------------------------------------------------------------
// T-018 §C.3：端口可用性探测
// ---------------------------------------------------------------

export interface PortProbeRequest {
  /** 1-32 个端口（后端硬限制；超过 → 422）。 */
  ports: number[]
}

export interface PortProbeResult {
  port: number
  available: boolean
  /**
   * 可能值：
   * - ""（空字符串） : available=true 时
   * - "privileged"    : 端口 < 1024
   * - "in_use"        : 端口已被占用
   * - "invalid"       : 端口非法（0 或 > 65535，后端校验通常先行拒）
   */
  reason: string
}

export interface PortProbeResponse {
  results: PortProbeResult[]
}

export interface WizardStatus {
  handled: boolean
  shouldShow: boolean
}

export interface DownloadBinRequest {
  kind: 'frpc' | 'frps'
}
