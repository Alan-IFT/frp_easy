// -----------------------------------------------------------------------
// T-018 §C.2：常用端口预设清单（前端 hardcode，FR-C.2.1 / NF-C.3）。
//
// 设计要求：
//   - 一行 Tag 列表渲染在 ProxyForm 的 type 选择下方；
//   - 点击单条 Tag → 填入 localPort + remotePort + 建议 name（如 "ssh-22"）；
//   - 不强制 type 联动（FR-C.2.3），但 type=tcp/udp 时 hint 友好；
//   - batch 模式下复用 port 值拼成逗号表达式（FR-C.2.4，由调用方处理）。
//
// 预设清单来自 01 §FR-C.2.2（PM-DECIDED）。
// 后续要追加新预设只改这一个数组，不散在多个组件。
// -----------------------------------------------------------------------

export interface PortPreset {
  /** 在 Tag 上显示的中文/英文 label，例 "SSH 22"。 */
  label: string
  /** 端口号。 */
  port: number
  /** 建议的代理类型；UI 可仅作 hint，不强制覆盖用户已选 type。 */
  type: 'tcp' | 'udp' | 'http' | 'https'
  /** 建议规则名，例 "ssh-22"。点击 preset 时自动填入 name。 */
  suggestedName: string
}

export const PORT_PRESETS: PortPreset[] = [
  // 远程登录
  { label: 'SSH 22',       port: 22,    type: 'tcp',   suggestedName: 'ssh-22' },
  { label: 'RDP 3389',     port: 3389,  type: 'tcp',   suggestedName: 'rdp-3389' },
  { label: 'VNC 5900',     port: 5900,  type: 'tcp',   suggestedName: 'vnc-5900' },
  // Web
  { label: 'HTTP 80',      port: 80,    type: 'http',  suggestedName: 'http-80' },
  { label: 'HTTPS 443',    port: 443,   type: 'https', suggestedName: 'https-443' },
  // 数据库
  { label: 'MySQL 3306',     port: 3306,  type: 'tcp', suggestedName: 'mysql-3306' },
  { label: 'PostgreSQL 5432', port: 5432, type: 'tcp', suggestedName: 'postgres-5432' },
  { label: 'Redis 6379',     port: 6379,  type: 'tcp', suggestedName: 'redis-6379' },
  { label: 'MongoDB 27017',  port: 27017, type: 'tcp', suggestedName: 'mongo-27017' },
  // 文件共享
  { label: 'SMB 445',      port: 445,   type: 'tcp',   suggestedName: 'smb-445' },
]

/**
 * 工具：按指定的 type 过滤预设（批量模式下 type 仅支持 tcp/udp，需要过滤 http/https 项）。
 */
export function filterPresetsByType(
  presets: PortPreset[],
  type: 'tcp' | 'udp' | 'http' | 'https',
): PortPreset[] {
  // 简化：tcp/udp 接受同 type 的预设；http/https 仅展示同 type 的预设。
  return presets.filter((p) => p.type === type)
}
