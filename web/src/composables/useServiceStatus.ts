// T-038 boot-autostart-hardening：服务化状态 composable。
//
// 用途：让 ServiceStatusCard.vue 在 mount 时拉取一次 / 暴露 refresh()，并把
// "supervised + boot_autostart" 联合判定的 needsFix 标志暴露给模板。
//
// 不做轮询：服务化状态变化的触发是用户跑了 install-service.{sh,ps1}，需要重新
// 加载页面（或显式点刷新）才有意义；持续轮询徒增负担。

import { computed, ref, onMounted } from 'vue'
import { apiGetServiceStatus } from '../api/system'
import type { SystemServiceStatusResponse } from '../types'

export function useServiceStatus() {
  const status = ref<SystemServiceStatusResponse | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function refresh() {
    loading.value = true
    error.value = null
    try {
      status.value = await apiGetServiceStatus()
    } catch (e: unknown) {
      error.value = e instanceof Error ? e.message : '加载失败'
    } finally {
      loading.value = false
    }
  }

  // needsFix = 服务化未完成（前台运行 或 unit 已注册但未 enabled）
  // ⇒ 卡片高亮 + 展开"如何修复"折叠区。
  const needsFix = computed(() => {
    const s = status.value
    if (!s) return false
    return !s.supervised || !s.boot_autostart
  })

  onMounted(() => {
    refresh()
  })

  return { status, loading, error, refresh, needsFix }
}
