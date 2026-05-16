import type { TagProps } from 'naive-ui'
import type { ProcessState } from '../types'

export type StatusTagType = TagProps['type']

export function getTagType(state: ProcessState): StatusTagType {
  switch (state) {
    case 'running':  return 'success'
    case 'error':    return 'error'
    case 'starting':
    case 'stopping': return 'warning'
    case 'stopped':
    default:         return 'default'
  }
}

export function getStateLabel(state: ProcessState): string {
  const labels: Record<ProcessState, string> = {
    stopped:  '已停止',
    starting: '启动中',
    running:  '运行中',
    stopping: '停止中',
    error:    '错误',
  }
  return labels[state] ?? state
}
