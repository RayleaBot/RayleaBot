<script setup lang="ts">
import AppSpinner from './AppSpinner.vue'
defineProps<{ busy?: boolean; label?: string }>()
</script>
<template>
  <div class="app-loading-panel" :aria-busy="busy || undefined">
    <div class="app-loading-panel__content" :inert="busy || undefined"><slot /></div>
    <div v-if="busy" class="app-loading-panel__indicator"><AppSpinner class="app-loading-panel__pill" :tip="label" /></div>
  </div>
</template>
<style scoped>
.app-loading-panel { position: relative; min-width: 0; min-height: 100px; width: 100%; }
.app-loading-panel__content { width: 100%; min-width: 0; min-height: 0; }
/* The content stays in place and inert while busy; an opaque pill in the toast family says so instead of a veil. */
.app-loading-panel__indicator { position: absolute; inset: 0; z-index: 1; display: flex; align-items: flex-start; justify-content: center; padding-top: 32px; pointer-events: none; }
.app-loading-panel__pill { padding: 8px 16px 8px 12px; border: 1px solid var(--border); border-radius: 999px; background: var(--surface-raised); box-shadow: var(--shadow-floating); font-size: 13px; font-weight: 500; }
.app-loading-panel__pill :deep(.app-loading__icon) { width: 16px; height: 16px; color: var(--muted); }
@media (forced-colors: active) { .app-loading-panel__pill { border-color: CanvasText; box-shadow: none; } }
</style>
