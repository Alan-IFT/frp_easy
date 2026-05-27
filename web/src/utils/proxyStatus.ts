// T-042 / proxy-runtime-status-merge · 02 § 3.2
//
// proxy runtime status → 视觉/文案映射 util。
//
// 决策（02 § 3.2 / 决策矩阵）：
//   - 大小写防御：toLowerCase 后判定（继承 T-041 GR C-5 范式）
//   - 空字符串 / null / undefined 归 'offline' 而非 'error'：
//     与"runtime 无此 proxy 名（Proxies.vue 行查不到 runtimeMap）"语义合并，
//     避免在 UI 上把"未注册"与"出错"两个语义混在同一红点。
//   - 颜色硬编码 hex：utils 是无 Vue setup 上下文的纯函数，
//     无法调 useThemeVars；调用方若需主题感知可在模板里覆盖 NTag type。
//
// 共享方：
//   - web/src/pages/ServerMonitor.vue (T-041 起；T-042 切到本 utils)
//   - web/src/pages/Proxies.vue (T-042 新增 runtime 列)

import type { TagProps } from 'naive-ui'

export type ProxyStatusTagType = NonNullable<TagProps['type']>

export interface ProxyStatusVisual {
  /** 给 NTag 的 type prop */
  type: ProxyStatusTagType
  /** 中文展示文本 */
  text: string
  /** 圆点颜色（语义化色，可用作内联 style 兜底） */
  dotColor: string
  /** 是否被识别为"在线" */
  online: boolean
}

const COLOR_SUCCESS = '#18a058'  // naive-ui 默认 success
const COLOR_DEFAULT = '#999999'  // 灰
const COLOR_ERROR = '#d03050'    // naive-ui 默认 error

export function getProxyStatusTag(raw: string | undefined | null): ProxyStatusVisual {
  const lower = (raw ?? '').toLowerCase()
  if (lower === 'online') {
    return { type: 'success', text: '在线', dotColor: COLOR_SUCCESS, online: true }
  }
  if (lower === 'offline' || lower === '') {
    return { type: 'default', text: '离线', dotColor: COLOR_DEFAULT, online: false }
  }
  return { type: 'error', text: raw || '未知', dotColor: COLOR_ERROR, online: false }
}
