// T-036 / log-ui-ux-polish · 02 §3.6.3
// 搜索（默认大小写不敏感子串匹配）+ 命中区间 → 喂给 LogLine 做 <mark> 包裹。
// 算法：String.indexOf 循环；不支持 regex；NFR-3 / D-2 决策。

import { computed, type ComputedRef, type Ref } from 'vue'
import type { ParsedLogLine } from './parseLogLine'

export interface SearchHit {
  start: number
  end: number
}

export interface VisibleLine {
  lineNumber: number
  parsed: ParsedLogLine
  searchHits: SearchHit[]
}

export interface UseLogSearchReturn {
  query: Ref<string>
  setQuery: (q: string) => void
  visibleLines: ComputedRef<VisibleLine[]>
}

import { ref } from 'vue'

export function useLogSearch(
  source: Ref<ParsedLogLine[]> | ComputedRef<ParsedLogLine[]>,
  caseSensitiveRef: Ref<boolean>,
): UseLogSearchReturn {
  const query = ref<string>('')

  function setQuery(q: string) {
    query.value = q
  }

  function findHits(message: string, needle: string, cs: boolean): SearchHit[] {
    if (needle === '') return []
    const hay = cs ? message : message.toLowerCase()
    const nee = cs ? needle : needle.toLowerCase()
    const hits: SearchHit[] = []
    let from = 0
    // 防御：空 needle 上 indexOf 总返回 0 → 死循环。已在上方拦截，但再加一层保险。
    if (nee.length === 0) return []
    while (true) {
      const idx = hay.indexOf(nee, from)
      if (idx === -1) break
      hits.push({ start: idx, end: idx + nee.length })
      from = idx + nee.length
    }
    return hits
  }

  const visibleLines = computed<VisibleLine[]>(() => {
    const raw = source.value
    const q = query.value.trim()
    const cs = caseSensitiveRef.value
    const out: VisibleLine[] = []
    if (q === '') {
      for (let i = 0; i < raw.length; i++) {
        out.push({
          lineNumber: i + 1,
          parsed: raw[i],
          searchHits: [],
        })
      }
      return out
    }
    for (let i = 0; i < raw.length; i++) {
      const line = raw[i]
      const hits = findHits(line.message, q, cs)
      if (hits.length > 0) {
        out.push({
          lineNumber: i + 1,
          parsed: line,
          searchHits: hits,
        })
      }
    }
    return out
  })

  return { query, setQuery, visibleLines }
}
