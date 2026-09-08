<script setup lang="ts">
import { watchEffect } from 'vue'

import AppSpinner from '@/components/AppSpinner.vue'
import AppToastHost from '@/components/AppToastHost.vue'
import { t } from '@/i18n'
import { resolvePreferenceCssVariables } from '@/preferences/app'
import { useAppAvailabilityStore } from '@/stores/app-availability'
import { useUiShellStore } from '@/stores/ui-shell'

const uiShellStore = useUiShellStore()
const availabilityStore = useAppAvailabilityStore()

watchEffect(() => {
  if (typeof document === 'undefined') {
    return
  }

  const root = document.documentElement
  const cssVariables = resolvePreferenceCssVariables(uiShellStore.preferences)

  root.dataset.theme = uiShellStore.resolvedThemeMode
  root.dataset.themeMode = uiShellStore.preferences.themeMode
  root.dataset.density = uiShellStore.preferences.density
  root.dataset.contentWidth = uiShellStore.preferences.contentWidth

  for (const [key, value] of Object.entries(cssVariables)) {
    root.style.setProperty(key, value)
  }
})
</script>

<template>
  <div :class="['app-root', `app-root--${uiShellStore.resolvedThemeMode}`, `app-root--${uiShellStore.preferences.density}`]">
    <Transition name="connection-notice">
      <div
        v-if="availabilityStore.isConnectionInterrupted"
        class="connection-notice"
        role="status"
        aria-live="polite"
        data-testid="connection-reconnect-notice"
      >
        <AppSpinner />
        <span>{{ t('app.connectionInterrupted') }}</span>
      </div>
    </Transition>

    <RouterView v-slot="{ Component }">
      <component :is="Component" v-if="Component" />
      <div v-else class="app-startup" role="status" aria-live="polite">
        <AppSpinner :tip="t('app.loading')" />
      </div>
    </RouterView>
    <AppToastHost />
  </div>
</template>

<style scoped lang="scss">
.app-startup {
  display: grid;
  min-height: 100vh;
  place-items: center;
  background: var(--app-background);
}

.connection-notice {
  position: fixed;
  z-index: 1100;
  top: max(12px, env(safe-area-inset-top));
  left: 50%;
  display: flex;
  align-items: center;
  gap: 8px;
  max-width: min(440px, calc(100vw - 32px));
  padding: 8px 14px;
  border: 1px solid color-mix(in srgb, var(--warning) 36%, var(--border));
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--surface-warning) 94%, transparent);
  box-shadow: var(--shadow-floating);
  color: var(--text);
  font-size: 13px;
  line-height: 1.5;
  transform: translateX(-50%);
}

.connection-notice :deep(.app-loading__icon) {
  background-color: var(--warning);
}

.connection-notice-enter-active,
.connection-notice-leave-active {
  transition:
    opacity 180ms cubic-bezier(0.16, 1, 0.3, 1),
    transform 180ms cubic-bezier(0.16, 1, 0.3, 1);
}

.connection-notice-enter-from,
.connection-notice-leave-to {
  opacity: 0;
  transform: translate(-50%, -8px);
}

@media (prefers-reduced-motion: reduce) {
  .connection-notice-enter-active,
  .connection-notice-leave-active {
    transition: none;
  }
}
</style>
