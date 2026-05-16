import { ref, computed, watch } from 'vue'
import type { ProxyInput, Proxy } from '../types'

export type ProxyFormType = 'tcp' | 'udp' | 'http' | 'https'

export interface ProxyFormData {
  name: string
  type: ProxyFormType
  localIP: string
  localPort: number | null
  remotePort: number | null
  customDomains: string[]
  enabled: boolean
  version: number
}

export function useProxyForm(initial: ProxyInput, _existingProxy?: Proxy | null) {
  const form = ref<ProxyFormData>({
    name: initial.name,
    type: initial.type,
    localIP: initial.localIP ?? '127.0.0.1',
    localPort: initial.localPort || null,
    remotePort: initial.remotePort ?? null,
    customDomains: initial.customDomains ?? [],
    enabled: initial.enabled !== false,
    version: initial.version ?? 0,
  })

  const isTcpUdp = computed(() => form.value.type === 'tcp' || form.value.type === 'udp')
  const isHttpHttps = computed(() => form.value.type === 'http' || form.value.type === 'https')

  function handleTypeChange() {
    form.value.remotePort = null
    form.value.customDomains = []
  }

  function toProxyInput(): ProxyInput {
    const output: ProxyInput = {
      name: form.value.name,
      type: form.value.type,
      localIP: form.value.localIP || '127.0.0.1',
      localPort: form.value.localPort ?? 0,
      enabled: form.value.enabled,
      version: form.value.version,
    }
    if (isTcpUdp.value) {
      output.remotePort = form.value.remotePort ?? undefined
    } else {
      output.customDomains = form.value.customDomains.length > 0 ? form.value.customDomains : undefined
    }
    return output
  }

  function syncFromInput(val: ProxyInput) {
    form.value.name = val.name
    form.value.type = val.type
    form.value.localIP = val.localIP ?? '127.0.0.1'
    form.value.localPort = val.localPort || null
    form.value.remotePort = val.remotePort ?? null
    form.value.customDomains = val.customDomains ?? []
    form.value.enabled = val.enabled !== false
    form.value.version = val.version ?? 0
  }

  return {
    form,
    isTcpUdp,
    isHttpHttps,
    handleTypeChange,
    toProxyInput,
    syncFromInput,
    watch,
  }
}
