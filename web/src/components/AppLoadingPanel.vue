<script setup lang="ts">
import AppSpinner from './AppSpinner.vue'
defineProps<{ busy?: boolean; label?: string }>()
</script>
<template>
  <div class="app-loading-panel" :aria-busy="busy || undefined">
    <div class="app-loading-panel__content" :inert="busy || undefined"><slot /></div>
    <div v-if="busy" class="app-loading-panel__indicator"><AppSpinner :tip="label" /></div>
  </div>
</template>
<style scoped>
.app-loading-panel { position: relative; min-width: 0; min-height: 100px; width: 100%; }
.app-loading-panel__content { width: 100%; min-width: 0; min-height: 0; }
.app-loading-panel__indicator { position: absolute; inset: 0; z-index: 1; display: flex; align-items: flex-start; justify-content: center; padding-top: 32px; background: color-mix(in srgb, var(--surface-strong) 80%, transparent); }
</style>
