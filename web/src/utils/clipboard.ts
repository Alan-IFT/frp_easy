// T-061 / clipboard-util-extract · 02 §3
//
// 跨组件共享的"复制文本到剪贴板"底层操作。
//
// 抽取自 T-058 三处逐字重复的内联实现（偿还 T-058 (A) 决策 D1 记录的 backlog）：
//   - web/src/components/LogViewer.vue::onCopy（catch 分支）
//   - web/src/components/FirewallHint.vue::copyText
//   - web/src/components/PublicIpDetector.vue::copyText
//
// 设计要点（insight L37 / L42）：
//   - 内网 http 非安全上下文 navigator.clipboard.writeText 必 reject → 必须配
//     document.execCommand('copy') + 临时 textarea fallback。
//   - 纯函数：不调用 message / useMessage（useMessage 是组合式 hook，只能在组件
//     setup 用）。util 只返回成功布尔，UI 反馈（toast）留各组件 setup 层。
//   - 1:1 行为搬运，无新增防御（OOS-5）。
//
// 共享方：LogViewer.vue / FirewallHint.vue / PublicIpDetector.vue

/**
 * 把 text 写入系统剪贴板。
 * 首选 navigator.clipboard.writeText（安全上下文）；失败回落临时离屏 textarea +
 * document.execCommand('copy')。
 *
 * @returns true=复制成功；false=两条路径都失败（调用方据此决定 UI 反馈）。
 *   不抛错、不弹 toast。
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.setAttribute('aria-hidden', 'true')
    ta.style.position = 'fixed'
    ta.style.left = '-9999px'
    document.body.appendChild(ta)
    ta.select()
    let ok = false
    try {
      ok = document.execCommand('copy')
    } catch {
      ok = false
    } finally {
      document.body.removeChild(ta)
    }
    return ok
  }
}
