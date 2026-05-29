// T-051 frontend-test-coverage · B-3
// useLogLevelFilter 单测（同簇其他 4 个 log composable 已有 spec，这个漏了）。
// 覆盖：默认全 level 通过 / 选中子集过滤 / 空集 BC-9 无命中 / PLAIN 行参与过滤。
import { describe, it, expect } from 'vitest'
import { ref } from 'vue'
import { useLogLevelFilter } from '../../composables/log/useLogLevelFilter'
import {
  ALL_LEVELS,
  type LogLevelOrPlain,
  type ParsedLogLine,
} from '../../composables/log/parseLogLine'

function L(level: LogLevelOrPlain, message = level.toLowerCase()): ParsedLogLine {
  return { raw: `${level} ${message}`, level, message }
}

// 混合 ERROR/WARN/INFO/DEBUG/TRACE/PLAIN 行
function mixedSource() {
  return ref<ParsedLogLine[]>([
    L('ERROR', 'connect failed'),
    L('WARN', 'retrying'),
    L('INFO', 'started'),
    L('DEBUG', 'handshake'),
    L('TRACE', 'frame dump'),
    L('PLAIN', 'unparsed banner'),
  ])
}

describe('useLogLevelFilter — 默认全选', () => {
  it('初始 activeLevels = ALL_LEVELS，filteredLines 全通过', () => {
    const src = mixedSource()
    const f = useLogLevelFilter(src)
    expect(f.activeLevels.value).toEqual(ALL_LEVELS)
    expect(f.filteredLines.value).toHaveLength(6)
  })
})

describe('useLogLevelFilter — 选中子集', () => {
  it('只选 ERROR/WARN → 仅返回这两类行', () => {
    const src = mixedSource()
    const f = useLogLevelFilter(src)
    f.setActiveLevels(['ERROR', 'WARN'])

    const levels = f.filteredLines.value.map((l) => l.level)
    expect(levels).toEqual(['ERROR', 'WARN'])
  })

  it('只选 INFO → 仅 INFO 行', () => {
    const src = mixedSource()
    const f = useLogLevelFilter(src)
    f.setActiveLevels(['INFO'])

    expect(f.filteredLines.value).toHaveLength(1)
    expect(f.filteredLines.value[0].level).toBe('INFO')
    expect(f.filteredLines.value[0].message).toBe('started')
  })

  it('PLAIN 参与过滤：选 PLAIN 时未解析行通过', () => {
    const src = mixedSource()
    const f = useLogLevelFilter(src)
    f.setActiveLevels(['PLAIN'])

    expect(f.filteredLines.value).toHaveLength(1)
    expect(f.filteredLines.value[0].level).toBe('PLAIN')
  })

  it('不选 PLAIN → 未解析行被过滤掉', () => {
    const src = mixedSource()
    const f = useLogLevelFilter(src)
    f.setActiveLevels(['ERROR', 'WARN', 'INFO', 'DEBUG', 'TRACE'])

    expect(f.filteredLines.value.some((l) => l.level === 'PLAIN')).toBe(false)
    expect(f.filteredLines.value).toHaveLength(5)
  })
})

describe('useLogLevelFilter — BC-9 空选集', () => {
  it('activeLevels=[] → filteredLines 为空（无命中）', () => {
    const src = mixedSource()
    const f = useLogLevelFilter(src)
    f.setActiveLevels([])

    expect(f.filteredLines.value).toHaveLength(0)
  })
})

describe('useLogLevelFilter — 响应式', () => {
  it('source 变化后 filteredLines 重算', () => {
    const src = ref<ParsedLogLine[]>([L('ERROR')])
    const f = useLogLevelFilter(src)
    f.setActiveLevels(['ERROR'])
    expect(f.filteredLines.value).toHaveLength(1)

    // 追加一条 ERROR + 一条 INFO；当前只选 ERROR
    src.value = [L('ERROR', 'a'), L('ERROR', 'b'), L('INFO', 'c')]
    expect(f.filteredLines.value).toHaveLength(2)
  })

  it('setActiveLevels 改选集后 filteredLines 即时重算', () => {
    const src = mixedSource()
    const f = useLogLevelFilter(src)
    f.setActiveLevels(['DEBUG'])
    expect(f.filteredLines.value).toHaveLength(1)

    f.setActiveLevels(['DEBUG', 'TRACE'])
    expect(f.filteredLines.value).toHaveLength(2)
  })
})
