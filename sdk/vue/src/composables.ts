import { computed, onBeforeUnmount, readonly, ref, shallowRef } from 'vue'

import { PluginUIBridgeClient } from './client'
import type { HostInitPayload } from './contract.generated'

export function usePluginHost(client = new PluginUIBridgeClient()) {
  const init = shallowRef<HostInitPayload | null>(null)
  const loading = ref(true)
  const error = shallowRef<Error | null>(null)

  const applyHostInit = (payload: HostInitPayload) => {
    init.value = payload
    applyTheme(payload.theme.mode, payload.theme.tokens)
  }

  const stopHostInit = client.on('host.init', (message) => {
    applyHostInit(message.payload as HostInitPayload)
  })

  const ready = client.connect()
    .then((payload) => {
      applyHostInit(payload)
      client.observeDocumentHeight()
      return payload
    })
    .catch((cause: unknown) => {
      error.value = cause instanceof Error ? cause : new Error(String(cause))
      throw error.value
    })
    .finally(() => {
      loading.value = false
    })

  onBeforeUnmount(() => {
    stopHostInit()
    client.close()
  })

  return {
    client,
    ready,
    init: readonly(init),
    loading: readonly(loading),
    error: readonly(error),
    config: computed(() => init.value?.config ?? {}),
    secretsConfigured: computed(() => init.value?.secrets_configured ?? {}),
  }
}

export function applyTheme(mode: 'light' | 'dark', tokens: Record<string, string>): void {
  const root = document.documentElement
  for (const [name, value] of Object.entries(tokens)) {
    if (/^[a-z0-9-]+$/.test(name) && typeof value === 'string') {
      root.style.setProperty(`--raylea-${name}`, value)
    }
  }
  root.dataset.theme = mode
}
