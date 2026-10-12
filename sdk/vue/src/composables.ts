import { computed, onUnmounted, readonly, ref, shallowRef } from 'vue'

import { hostDocument, PluginUIClient, readHostTheme } from './client'
import type { BotAccount, HostInitPayload } from './client'

export function usePluginHost(client = new PluginUIClient()) {
  const init = shallowRef<HostInitPayload | null>(null)
  const loading = ref(true)
  const error = shallowRef<Error | null>(null)

  const ready = client.loadContext()
    .then((payload) => {
      init.value = payload
      applyTheme(payload.theme.mode, payload.theme.tokens)
      return payload
    })
    .catch((cause: unknown) => {
      error.value = cause instanceof Error ? cause : new Error(String(cause))
      throw error.value
    })
    .finally(() => {
      loading.value = false
    })

  const stopThemeSync = observeHostTheme(() => {
    const theme = readHostTheme()
    applyTheme(theme.mode, theme.tokens)
  })
  onUnmounted(stopThemeSync)

  // refreshBot rereads the selected account's profile and connection; the
  // account itself stays fixed for the lifetime of the page.
  async function refreshBot(): Promise<BotAccount> {
    const bot = await client.reloadBotAccount()
    if (init.value) {
      init.value = { ...init.value, bot }
    }
    return bot
  }

  return {
    client,
    ready,
    init: readonly(init),
    loading: readonly(loading),
    error: readonly(error),
    page: computed(() => init.value?.page ?? null),
    bot: computed(() => init.value?.bot ?? null),
    refreshBot,
    config: computed(() => init.value?.config ?? {}),
    secretsConfigured: computed(() => init.value?.secrets_configured ?? {}),
  }
}

export function observeHostTheme(onChange: () => void): () => void {
  const root = hostDocument()?.documentElement
  if (!root || root === document.documentElement || typeof MutationObserver === 'undefined') {
    return () => undefined
  }
  const observer = new MutationObserver(onChange)
  observer.observe(root, { attributes: true, attributeFilter: ['data-theme', 'class', 'style'] })
  return () => observer.disconnect()
}

export function applyTheme(mode: 'light' | 'dark', tokens: Record<string, string>): void {
  const root = document.documentElement
  for (const [name, value] of Object.entries(tokens)) {
    if (/^[a-z0-9-]+$/.test(name) && typeof value === 'string' && value) {
      root.style.setProperty(`--raylea-${name}`, value)
    }
  }
  root.dataset.theme = mode
}
