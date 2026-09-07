<script setup lang="ts">
withDefaults(defineProps<{ tone?: 'neutral' | 'info' | 'success' | 'warning' | 'danger' }>(), { tone: 'neutral' })
</script>
<template><span class="app-badge" :data-tone="tone"><span class="app-badge__dot" aria-hidden="true" /><slot /></span></template>
<style scoped>
.app-badge { display: inline-flex; align-items: center; gap: 6px; flex-shrink: 0; padding: 3px 8px; border-radius: 6px; color: var(--muted); background: var(--surface-soft); font-size: 12px; line-height: 1.5; }
.app-badge__dot { width: 5px; height: 5px; border-radius: 50%; background: currentColor; }
.app-badge[data-tone=info] { color: var(--text-accent); background: var(--surface-accent); }
.app-badge[data-tone=success] { color: var(--text-success); background: var(--surface-success); }
.app-badge[data-tone=warning] { color: var(--text-warning); background: var(--surface-warning); }
.app-badge[data-tone=danger] { color: var(--text-danger); background: var(--surface-danger); }
</style>
