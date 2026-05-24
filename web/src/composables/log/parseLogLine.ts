// T-036 / log-ui-ux-polish · 02 §3.6.1
// 单行 frp 日志解析为 timestamp / level / message 三段；解析失败降级 PLAIN。
// frp 上游标准格式：`YYYY/MM/DD HH:MM:SS [I] [subsystem.go:123] message`
// 二次封装变体：    `YYYY-MM-DD HH:MM:SS [INFO] message`
// 兼容 ISO-T：       `YYYY-MM-DDTHH:MM:SS.sss [E] message`

export type LogLevel = 'ERROR' | 'WARN' | 'INFO' | 'DEBUG' | 'TRACE'
export type LogLevelOrPlain = LogLevel | 'PLAIN'

export const ALL_LEVELS: LogLevelOrPlain[] = [
  'ERROR',
  'WARN',
  'INFO',
  'DEBUG',
  'TRACE',
  'PLAIN',
]

export interface ParsedLogLine {
  raw: string
  timestamp?: string
  level: LogLevelOrPlain
  message: string
}

// 单条 regex 双格式 OR：日期支持 `/` 或 `-`，时间分隔支持空格或 `T`，可选毫秒；
// 等级段可选方括号包裹；短字母 I/W/E/D/T 与长全称 ERROR/WARN(ING)/INFO/DEBUG/TRACE 都支持。
const LOG_LINE_RE =
  /^(\d{4}[-/]\d{2}[-/]\d{2}[ T]\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+\[?(I|W|E|D|T|ERROR|WARN(?:ING)?|INFO|DEBUG|TRACE)\]?\s+(.*)$/i

const SHORT_TO_LEVEL: Record<string, LogLevel> = {
  I: 'INFO',
  W: 'WARN',
  E: 'ERROR',
  D: 'DEBUG',
  T: 'TRACE',
}

/**
 * 解析单行 frp 日志。
 *
 * 降级语义：任何不匹配 LOG_LINE_RE 的行 → `{ raw, level: 'PLAIN', message: raw }`，
 * 整行作为 message 渲染，无 timestamp / level 段，按默认前景着色。
 * **无任何抛错路径**（确保单行格式异常不污染整页渲染）。
 */
export function parseLogLine(raw: string): ParsedLogLine {
  const m = LOG_LINE_RE.exec(raw)
  if (!m) {
    return { raw, level: 'PLAIN', message: raw }
  }
  const rawLevelUpper = m[2].toUpperCase()
  // 优先匹配短字母映射，否则视作已经是长全称（可能包含 WARNING，需归一）
  const candidate: string = SHORT_TO_LEVEL[rawLevelUpper] ?? rawLevelUpper
  // WARNING 归一到 WARN
  const level: LogLevel =
    candidate === 'WARNING' ? 'WARN' : (candidate as LogLevel)
  return {
    raw,
    timestamp: m[1],
    level,
    message: m[3],
  }
}
