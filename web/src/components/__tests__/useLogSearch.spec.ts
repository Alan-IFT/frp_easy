import { describe, it, expect } from 'vitest'
import { ref } from 'vue'
import { useLogSearch } from '../../composables/log/useLogSearch'
import type { ParsedLogLine } from '../../composables/log/parseLogLine'

// T-036 · AC-2 / NFR-7 ADV-A：搜索 + 大小写敏感 + escape 边界。

function P(message: string): ParsedLogLine {
  return { raw: message, level: 'PLAIN', message }
}

describe('useLogSearch — 空 query 全可见 / 命中 0 行 → 隐藏', () => {
  it('空 query → 所有行可见，hits=[]', () => {
    const src = ref<ParsedLogLine[]>([P('a'), P('b'), P('c')])
    const cs = ref(false)
    const s = useLogSearch(src, cs)
    expect(s.visibleLines.value.length).toBe(3)
    expect(s.visibleLines.value[0].searchHits).toEqual([])
  })

  it('query 命中 0 行 → visible=[]', () => {
    const src = ref<ParsedLogLine[]>([P('alpha'), P('beta')])
    const cs = ref(false)
    const s = useLogSearch(src, cs)
    s.setQuery('zzz')
    expect(s.visibleLines.value.length).toBe(0)
  })

  it('query 只命中 1 行 → 仅返回该行', () => {
    const src = ref<ParsedLogLine[]>([
      P('alpha line'),
      P('beta line'),
      P('gamma line'),
    ])
    const cs = ref(false)
    const s = useLogSearch(src, cs)
    s.setQuery('beta')
    expect(s.visibleLines.value.length).toBe(1)
    expect(s.visibleLines.value[0].parsed.message).toBe('beta line')
  })
})

describe('useLogSearch — AC-2 大小写不敏感（默认）', () => {
  it('默认 caseSensitive=false：CONNECTION 与 connection 都命中', () => {
    const src = ref<ParsedLogLine[]>([
      P('CONNECTION refused'),
      P('connection ok'),
      P('Connection lost'),
      P('unrelated'),
    ])
    const cs = ref(false)
    const s = useLogSearch(src, cs)
    s.setQuery('connection')
    expect(s.visibleLines.value.length).toBe(3)
    // hits 区间正确：CONNECTION refused → start=0, end=10
    expect(s.visibleLines.value[0].searchHits[0]).toEqual({
      start: 0,
      end: 10,
    })
  })
})

describe('useLogSearch — AC-2 大小写敏感切换', () => {
  it('caseSensitive=true：CONNECTION 与 connection 仅大写命中', () => {
    const src = ref<ParsedLogLine[]>([P('CONNECTION refused'), P('connection ok')])
    const cs = ref(true)
    const s = useLogSearch(src, cs)
    s.setQuery('connection')
    expect(s.visibleLines.value.length).toBe(1)
    expect(s.visibleLines.value[0].parsed.message).toBe('connection ok')
  })
})

describe('useLogSearch — 多次命中', () => {
  it('一行内多次出现命中 → searchHits 长度 ≥ 2', () => {
    const src = ref<ParsedLogLine[]>([P('foo bar foo baz foo')])
    const cs = ref(false)
    const s = useLogSearch(src, cs)
    s.setQuery('foo')
    expect(s.visibleLines.value.length).toBe(1)
    expect(s.visibleLines.value[0].searchHits.length).toBe(3)
    expect(s.visibleLines.value[0].searchHits[0]).toEqual({
      start: 0,
      end: 3,
    })
    expect(s.visibleLines.value[0].searchHits[2]).toEqual({
      start: 16,
      end: 19,
    })
  })
})

describe('useLogSearch — NFR-7 / ADV-A：XSS 字符不会让 useLogSearch 自身崩 / 区间正确', () => {
  it('搜索 "<script>" 关键字 → 命中行的 hits 指向 message 内 "<script>" 子串', () => {
    const src = ref<ParsedLogLine[]>([
      P('attacker tried <script>alert(1)</script> injection'),
      P('safe line'),
    ])
    const cs = ref(false)
    const s = useLogSearch(src, cs)
    s.setQuery('<script>')
    expect(s.visibleLines.value.length).toBe(1)
    expect(s.visibleLines.value[0].searchHits.length).toBe(1)
    const hit = s.visibleLines.value[0].searchHits[0]
    expect(
      s.visibleLines.value[0].parsed.message.slice(hit.start, hit.end),
    ).toBe('<script>')
  })

  it('regex 特殊字符（*, (, ?） 不会触发 throw', () => {
    const src = ref<ParsedLogLine[]>([P('a*b'), P('(c)'), P('?d?')])
    const cs = ref(false)
    const s = useLogSearch(src, cs)
    expect(() => {
      s.setQuery('*')
      // 强行触发 computed
      void s.visibleLines.value
    }).not.toThrow()
    expect(() => {
      s.setQuery('(')
      void s.visibleLines.value
    }).not.toThrow()
  })
})

describe('useLogSearch — lineNumber 基于源缓冲（BC-3）', () => {
  it('命中行的 lineNumber = 在原 source 中的 1-based 序号', () => {
    const src = ref<ParsedLogLine[]>([
      P('aaa'),
      P('bbb'),
      P('ccc'),
      P('ddd'),
      P('eee bbb'), // 命中
    ])
    const cs = ref(false)
    const s = useLogSearch(src, cs)
    s.setQuery('bbb')
    expect(s.visibleLines.value.length).toBe(2)
    expect(s.visibleLines.value[0].lineNumber).toBe(2)
    expect(s.visibleLines.value[1].lineNumber).toBe(5)
  })
})
