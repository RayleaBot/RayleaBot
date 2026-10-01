<script setup lang="ts">
import { computed, watch, watchEffect } from 'vue'

import AppSpinner from '@/components/AppSpinner.vue'
import AppToastHost from '@/components/AppToastHost.vue'
import { t } from '@/i18n'
import { resolvePreferenceCssVariables } from '@/preferences/app'
import { useAppAvailabilityStore } from '@/stores/app-availability'
import { useSystemStore } from '@/stores/system'
import { useUiShellStore } from '@/stores/ui-shell'

const uiShellStore = useUiShellStore()
const availabilityStore = useAppAvailabilityStore()
const systemStore = useSystemStore()

// The notice says why the connection is gone: an announced stop stays down until someone starts the service again,
// a restart or an update comes back by itself, and anything else is a lost connection that keeps retrying. Once
// the service answers again, a later interruption is an ordinary one.
watch(() => availabilityStore.isConnectionInterrupted, (interrupted) => {
  if (!interrupted) systemStore.stopIntent = null
})
const connectionNotice = computed(() => {
  switch (systemStore.stopIntent) {
    case 'stop': return { state: 'stopped', text: t('app.serviceStopped') }
    case 'restart': return { state: 'restarting', text: t('app.serviceRestarting') }
    case 'update': return { state: 'updating', text: t('app.serviceUpdating') }
    default: return { state: 'retrying', text: t('app.connectionInterrupted') }
  }
})

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

  const favicon = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  const faviconPath = uiShellStore.resolvedThemeMode === 'dark' ? '/favicon-dark.svg' : '/favicon.svg'
  if (favicon && favicon.getAttribute('href') !== faviconPath) favicon.href = faviconPath

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
        :data-state="connectionNotice.state"
        role="status"
        aria-live="polite"
        data-testid="connection-reconnect-notice"
      >
        <span class="connection-notice__signal" aria-hidden="true" />
        <span>{{ connectionNotice.text }}</span>
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

// A status pill in the toast family: raised surface, hairline edge and the floating shadow.
.connection-notice {
  --notice-tone: var(--warning);
  --notice-tone-soft: var(--surface-warning);

  position: fixed;
  z-index: 1100;
  top: max(14px, env(safe-area-inset-top));
  left: 50%;
  display: flex;
  align-items: center;
  gap: 10px;
  max-width: min(520px, calc(100vw - 32px));
  padding: 6px 16px 6px 6px;
  border: 1px solid var(--border);
  border-radius: 999px;
  background: var(--surface-raised);
  box-shadow: var(--shadow-floating);
  color: var(--text);
  font-size: 13px;
  font-weight: 500;
  line-height: 1.5;
  transform: translateX(-50%);
}

.connection-notice[data-state='stopped'] {
  --notice-tone: var(--muted);
  --notice-tone-soft: var(--surface-soft);
}

// A restart or an update is planned and comes back by itself, so it pulses in the calm info tone.
.connection-notice:is([data-state='restarting'], [data-state='updating']) {
  --notice-tone: var(--info);
  --notice-tone-soft: var(--surface-info);
}

.connection-notice__signal {
  position: relative;
  z-index: 0;
  display: grid;
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  place-items: center;
  border-radius: 50%;
  background: var(--notice-tone-soft);
}

.connection-notice__signal::before {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--notice-tone);
  content: '';
}

// While the service is expected back, a soft disc keeps spreading out from behind the dot; a stopped service keeps
// the dot still.
.connection-notice:not([data-state='stopped']) .connection-notice__signal::after {
  position: absolute;
  z-index: -1;
  inset: 0;
  border-radius: 50%;
  background: var(--notice-tone-soft);
  animation: connection-notice-ping 1.6s cubic-bezier(0, 0, 0.2, 1) infinite;
  content: '';
}

@keyframes connection-notice-ping {
  from {
    opacity: 1;
    transform: scale(1);
  }

  to {
    opacity: 0;
    transform: scale(1.9);
  }
}

.connection-notice-enter-active {
  transition:
    opacity 220ms var(--motion-easing),
    transform 220ms var(--motion-easing);
}

.connection-notice-leave-active {
  transition:
    opacity 160ms cubic-bezier(0.4, 0, 1, 1),
    transform 160ms cubic-bezier(0.4, 0, 1, 1);
}

.connection-notice-enter-from,
.connection-notice-leave-to {
  opacity: 0;
  transform: translate(-50%, -10px) scale(0.96);
}

@media (prefers-reduced-motion: reduce), (forced-colors: active) {
  .connection-notice-enter-active,
  .connection-notice-leave-active {
    transition: none;
  }

  .connection-notice:not([data-state='stopped']) .connection-notice__signal::after {
    animation: none;
    opacity: 0;
  }
}

@media (forced-colors: active) {
  .connection-notice {
    border-color: CanvasText;
    box-shadow: none;
  }
}
</style>
