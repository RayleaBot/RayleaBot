<script setup lang="ts">
import { t } from '@/i18n'
import { Skeleton } from '@/components/ui/skeleton'
withDefaults(defineProps<{ rows?: number }>(), { rows: 3 })
</script>
<template><div class="app-skeleton" aria-busy="true" :aria-label="t('ui.loading')"><Skeleton v-for="row in rows" :key="row" class="app-skeleton__line" /></div></template>
<style scoped>
.app-skeleton { display: grid; gap: 12px; padding: 12px 0; }
.app-skeleton__line { height: 16px; border-radius: 4px; }
.app-skeleton__line:last-child { width: 65%; }
@media (prefers-reduced-motion: reduce), (forced-colors: active) { .app-skeleton__line { animation: none; } }
</style>
