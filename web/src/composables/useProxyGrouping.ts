// -----------------------------------------------------------------------
// T-018 §C / B-12 修订：代理规则按 name 前缀折叠分组（纯视图层）。
//
// 规则：
//   - name 形如 "<basename>-<port>"，其中 <port> 是 1-5 位数字尾段；
//   - 同一 basename 且同 type 的连续多条规则合并为一行（"组行"）；
//   - "连续" 定义：localPort 升序排序后，组内端口要么是连续整数，要么是同样的 1:1 映射；
//   - 只折叠 TCP/UDP（HTTP/HTTPS 走域名不适用）；
//   - 单条规则（basename 不重复 / 类型不同 / 端口不连续）保持原 row 形态。
//
// B-12 修订正则：`^(.+)-(\d{1,5})$` —— greedy 取最后一段数字尾。
//   覆盖用例：
//     web-6000        → basename="web", port=6000        ✓
//     my-web-6000     → basename="my-web", port=6000     ✓
//     a-b-c-22        → basename="a-b-c", port=22        ✓
//     web-notaport    → 匹配失败 → 不折叠                ✗
//     abc             → 匹配失败 → 不折叠                ✗
// -----------------------------------------------------------------------

import { reactive } from 'vue'
import type { Proxy } from '../types'

/** B-12 修订正则：greedy 匹配最后一段 1-5 位数字尾。 */
export const PROXY_NAME_RE = /^(.+)-(\d{1,5})$/

export interface ParsedProxyName {
  basename: string
  port: number
}

/**
 * 把 name 拆为 (basename, port)；匹配失败返回 null（调用方按"单条"处理）。
 */
export function parseProxyName(name: string): ParsedProxyName | null {
  const m = PROXY_NAME_RE.exec(name)
  if (!m) return null
  const port = Number.parseInt(m[2], 10)
  if (!Number.isFinite(port) || port < 1 || port > 65535) return null
  return { basename: m[1], port }
}

// -----------------------------------------------------------------------

export interface ProxySingleRow {
  kind: 'single'
  key: string
  proxy: Proxy
}

export interface ProxyGroupRow {
  kind: 'group'
  key: string
  basename: string
  proto: 'tcp' | 'udp'
  localIP: string
  count: number
  /** "6000-6010" 或 "6000-6005, 7000-7002"。 */
  portRangeText: string
  proxies: Proxy[]
  expanded: boolean
}

export type GroupedProxyRow = ProxySingleRow | ProxyGroupRow

/**
 * 按 basename + type 折叠。HTTP/HTTPS 不折叠。
 * 同 basename 的多条规则被合并为单个 "组行"；端口连续段用 "a-b" 表示，多段用逗号分隔。
 *
 * 折叠仅展示层：删除/编辑仍按单条（由调用方对 expanded 状态做处理）。
 */
export function groupProxiesByPrefix(proxies: Proxy[]): GroupedProxyRow[] {
  // 第一步：按 (basename, type) 桶
  type Bucket = {
    basename: string
    proto: 'tcp' | 'udp'
    items: Proxy[]
  }
  const buckets = new Map<string, Bucket>()
  const standalone: Proxy[] = []

  for (const p of proxies) {
    if (p.type !== 'tcp' && p.type !== 'udp') {
      standalone.push(p)
      continue
    }
    const parsed = parseProxyName(p.name)
    if (!parsed) {
      standalone.push(p)
      continue
    }
    const key = `${parsed.basename}::${p.type}`
    let bucket = buckets.get(key)
    if (!bucket) {
      bucket = { basename: parsed.basename, proto: p.type, items: [] }
      buckets.set(key, bucket)
    }
    bucket.items.push(p)
  }

  const rows: GroupedProxyRow[] = []

  // 第二步：每个桶若条数 ≥ 2，且端口数字尾段去重后多于 1 → 折叠成组行
  for (const bucket of buckets.values()) {
    if (bucket.items.length < 2) {
      // 单条不折叠，回退为 single row
      for (const p of bucket.items) {
        rows.push(makeSingleRow(p))
      }
      continue
    }
    // 端口按 localPort 升序
    const sorted = [...bucket.items].sort((a, b) => a.localPort - b.localPort)
    const portRangeText = compressPorts(sorted.map((p) => p.localPort))
    const localIP = sorted[0]?.localIP ?? '127.0.0.1'
    rows.push(reactive<ProxyGroupRow>({
      kind: 'group',
      key: `group::${bucket.basename}::${bucket.proto}`,
      basename: bucket.basename,
      proto: bucket.proto,
      localIP,
      count: sorted.length,
      portRangeText,
      proxies: sorted,
      expanded: false,
    }))
  }

  // 第三步：单条规则（含 http/https / 无后缀）
  for (const p of standalone) {
    rows.push(makeSingleRow(p))
  }

  // 排序：组行在前（按 basename 字母），单条在后（按 id）
  rows.sort((a, b) => {
    if (a.kind !== b.kind) return a.kind === 'group' ? -1 : 1
    if (a.kind === 'group' && b.kind === 'group') {
      return a.basename.localeCompare(b.basename)
    }
    if (a.kind === 'single' && b.kind === 'single') {
      return a.proxy.id - b.proxy.id
    }
    return 0
  })

  // 展开后：把组内成员作为 single row 紧跟在组行之后插入
  const out: GroupedProxyRow[] = []
  for (const row of rows) {
    out.push(row)
    if (row.kind === 'group' && row.expanded) {
      for (const p of row.proxies) {
        out.push({
          kind: 'single',
          key: `expanded::${row.key}::${p.id}`,
          proxy: p,
        })
      }
    }
  }
  return out
}

function makeSingleRow(p: Proxy): ProxySingleRow {
  return {
    kind: 'single',
    key: `single::${p.id}`,
    proxy: p,
  }
}

/**
 * 把端口数组压缩为人类可读区间，如 [6000,6001,6002,7000] → "6000-6002, 7000"。
 * 输入要求：去重 + 升序。
 */
export function compressPorts(ports: number[]): string {
  if (ports.length === 0) return ''
  const segments: string[] = []
  let start = ports[0]
  let prev = ports[0]
  for (let i = 1; i < ports.length; i++) {
    const cur = ports[i]
    if (cur === prev + 1) {
      prev = cur
      continue
    }
    segments.push(start === prev ? `${start}` : `${start}-${prev}`)
    start = cur
    prev = cur
  }
  segments.push(start === prev ? `${start}` : `${start}-${prev}`)
  return segments.join(', ')
}
