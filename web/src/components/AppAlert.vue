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
    <div v-if="$slots.action"><slot name="action" /></div>
  </div>
</template>
<style scoped>
.app-alert { display: flex; align-items: flex-start; gap: 12px; padding: 16px; border-radius: 12px; background: var(--surface-accent); color: var(--text-accent); font-size: 14px; line-height: 1.6; }
.app-alert[data-tone=warning] { color: var(--text-warning); background: var(--surface-warning); }
.app-alert[data-tone=danger] { color: var(--text-danger); background: var(--surface-danger); }
.app-alert[data-tone=success] { color: var(--text-success); background: var(--surface-success); }
.app-alert > svg { margin-top: 3px; flex-shrink: 0; }
.app-alert__content { flex: 1; min-width: 0; overflow-wrap: anywhere; }
.app-alert p { margin: 0; }
.app-alert .app-alert__description { margin-top: 4px; font-size: 13px; }
</style>
