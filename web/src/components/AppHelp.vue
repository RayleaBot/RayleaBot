<script setup lang="ts">
import { InfoIcon } from '@lucide/vue'
import AppPopover from './AppPopover.vue'

defineProps<{ label: string; description: string }>()
</script>

<template>
  <AppPopover :label="label">
    <button type="button" class="app-help" :aria-label="label"><InfoIcon :size="14" aria-hidden="true" /></button>
    <template #content><p class="app-help__description">{{ description }}</p></template>
  </AppPopover>
</template>

<style scoped>
.app-help { display: inline-flex; align-items: center; justify-content: center; flex: none; width: 24px; height: 24px; padding: 0; border-radius: 6px; color: var(--muted); }
.app-help:hover { color: var(--text); background: var(--surface-soft); }
.app-help__description { margin: 0; white-space: pre-line; overflow-wrap: anywhere; }
@media (max-width: 639px), (pointer: coarse) { .app-help { width: 44px; height: 44px; } }
</style>
