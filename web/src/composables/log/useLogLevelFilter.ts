// T-036 / log-ui-ux-polish · 02 §3.6.4
// 等级多选过滤；activeLevels=[] 等价于 BC-9 "无命中"。

import { ref, computed, type ComputedRef, type Ref } from 'vue'
import {
  ALL_LEVELS,
  type LogLevelOrPlain,
  type ParsedLogLine,
} from './parseLogLine'

export interface UseLogLevelFilterReturn {
  activeLevels: Ref<LogLevelOrPlain[]>
  setActiveLevels: (l: LogLevelOrPlain[]) => void
  filteredLines: ComputedRef<ParsedLogLine[]>
}

export function useLogLevelFilter(
  source: ComputedRef<ParsedLogLine[]> | Ref<ParsedLogLine[]>,
): UseLogLevelFilterReturn {
  const activeLevels = ref<LogLevelOrPlain[]>([...ALL_LEVELS])

  function setActiveLevels(l: LogLevelOrPlain[]) {
    activeLevels.value = l
  }

  const filteredLines = computed<ParsedLogLine[]>(() => {
    const set = new Set(activeLevels.value)
    return source.value.filter((l) => set.has(l.level))
  })

  return { activeLevels, setActiveLevels, filteredLines }
}
