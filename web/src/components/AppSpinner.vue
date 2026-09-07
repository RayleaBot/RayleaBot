<script setup lang="ts">
import { LoaderCircleIcon } from '@lucide/vue'
defineProps<{ tip?: string }>()
</script>
<template><span class="app-loading" role="status"><LoaderCircleIcon class="app-loading__icon" aria-hidden="true" /><span v-if="tip">{{ tip }}</span><span v-else class="sr-only">加载中</span></span></template>
<style scoped>
.app-loading { display: inline-flex; align-items: center; justify-content: center; gap: 10px; color: var(--muted); }
.app-loading__icon { width: 20px; height: 20px; animation: app-loading-spin .8s linear infinite; }
@keyframes app-loading-spin { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce), (forced-colors: active) { .app-loading__icon { animation: none; } }
</style>
