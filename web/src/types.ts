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

/**
 * T-040: frps allowPorts 单条端口策略 entry。
 *
 * 互斥规则：每个 entry 必须含 single 或 start+end 之一（不能同时填）。
 * 后端 ValidateFrpsAllowPorts 守门；前端 AllowPortsEditor 实时镜像校验。
 */
export interface AllowPortRange {
  start?: number
  end?: number
  single?: number
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
  /** T-040: 端口策略白名单。留空 = 允许所有端口；最长 100 条。 */
  allowPorts?: AllowPortRange[]
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

export interface WizardStatus {
  handled: boolean
  shouldShow: boolean
}

export interface DownloadBinRequest {
  kind: 'frpc' | 'frps'
}

// ---------------------------------------------------------------
// T-038 boot-autostart-hardening：服务化状态 + autoRestore last-run
// ---------------------------------------------------------------

export interface AutoRestoreAttempt {
  index: number
  ok: boolean
  reason?: string
  at: string // RFC3339 UTC
}

export interface AutoRestoreLastRun {
  kind: 'frpc' | 'frps'
  timestamp: string
  outcome: 'ok' | 'exhausted' | 'user-initiated' | 'canceled' | 'binary-missing' | 'config-missing'
  attempts: AutoRestoreAttempt[]
}

export interface SystemServiceStatusResponse {
  supervised: boolean
  supervisor: 'systemd' | 'windows-service' | 'none'
  boot_autostart: boolean
  run_as: string
  probe_error?: string
  auto_restore: {
    enabled_kinds: Array<'frpc' | 'frps'>
    last_runs?: Record<string, AutoRestoreLastRun>
  }
}
