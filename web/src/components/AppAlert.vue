<script setup lang="ts">
import { CircleAlertIcon, CircleCheckIcon, InfoIcon } from '@lucide/vue'
withDefaults(defineProps<{ tone?: 'info' | 'success' | 'warning' | 'danger'; title: string; description?: string }>(), { tone: 'info' })
</script>
<template>
  <div class="app-alert" :data-tone="tone" :role="tone === 'danger' ? 'alert' : 'status'">
    <CircleCheckIcon v-if="tone === 'success'" :size="18" aria-hidden="true" />
    <CircleAlertIcon v-else-if="tone === 'danger' || tone === 'warning'" :size="18" aria-hidden="true" />
    <InfoIcon v-else :size="18" aria-hidden="true" />
    <div class="app-alert__content"><p>{{ title }}</p><p v-if="description" class="app-alert__description">{{ description }}</p><slot /></div>
    <div v-if="$slots.action" class="app-alert__action"><slot name="action" /></div>
  </div>
</template>
<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;
.app-alert { --alert-accent: var(--text-accent); display: grid; grid-template-columns: 18px minmax(0, 1fr) auto; align-items: start; gap: 10px; width: fit-content; max-width: 100%; padding: 10px 0; color: var(--text); font-size: 14px; line-height: 1.6; }
.app-alert[data-tone=warning] { --alert-accent: var(--text-warning); }
.app-alert[data-tone=danger] { --alert-accent: var(--text-danger); }
.app-alert[data-tone=success] { --alert-accent: var(--text-success); }
.app-alert > svg { margin-top: 3px; color: var(--alert-accent); }
.app-alert__content { min-width: 0; max-width: 72ch; overflow-wrap: anywhere; }
.app-alert p { margin: 0; }
.app-alert__content > p:first-child { font-weight: 500; }
.app-alert .app-alert__description { margin-top: 3px; color: var(--muted); font-size: 13px; }
.app-alert__action { align-self: center; }
@media (max-width: #{bp.$phone - 1px}) { .app-alert { grid-template-columns: 18px minmax(0, 1fr); } .app-alert__action { grid-column: 2; } }
</style>
