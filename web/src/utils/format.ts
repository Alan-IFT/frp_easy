// T-042 / proxy-runtime-status-merge · 02 § 3.1
//
// 通用字节 / 时间格式化 utils。
//
// formatBytes: T-041 ServerMonitor.vue 内联实现字节级搬运 + T-042 新增"负数 → '—'"防御
//   （上游 frps admin API 理论上不返回负数；防御性兜底避免 while 循环异常出口）
// formatTime:  T-041 ServerMonitor.vue 内联实现字节级搬运（空 / null / undefined / "0001-..." → '—'）
//   T-048 C5：统一本地化 —— 解析后 toLocaleString('zh-CN', { hour12:false }) 返回，
//   消除 ServerMonitor 显示裸 ISO 而 Dashboard / ServiceStatusCard 显示本地化时间的跨页不一致。
//   保留空 / null / undefined / "0001-..." / 无法解析 → '—' 的防御。
//
// 共享方：
//   - web/src/pages/ServerMonitor.vue (T-041 起；T-042 切到本 utils)
//   - web/src/pages/Proxies.vue (T-042 新增 runtime 列)
//   - web/src/pages/Dashboard.vue (T-048 起复用，消除本地 formatTime)
//   - web/src/components/ServiceStatusCard.vue (T-048 起复用，消除本地 formatTime)

export function formatBytes(n: number | undefined | null): string {
  if (n === undefined || n === null || Number.isNaN(n)) return '—'
  if (n === 0) return '0 B'
  if (n < 0) return '—'  // T-042 新增防御
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB']
  let v = n
  let u = 0
  while (v >= 1024 && u < units.length - 1) {
    v /= 1024
    u++
  }
  const s = u === 0 ? `${v}` : v.toFixed(1).replace(/\.0$/, '')
  return `${s} ${units[u]}`
}

export function formatTime(s: string | undefined | null): string {
  if (!s) return '—'
  if (s.startsWith('0001-')) return '—'  // frps 上游空值 sentinel
  const t = new Date(s).getTime()
  if (Number.isNaN(t)) return '—'  // 无法解析的字符串（防御 "Invalid Date" 裸字符串外泄）
  return new Date(t).toLocaleString('zh-CN', { hour12: false })
}
