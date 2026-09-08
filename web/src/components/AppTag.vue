<script setup lang="ts">
import type { StatusTone } from '@/lib/status-tone'
import AppBadge from './AppBadge.vue'
defineProps<{ tone?: StatusTone; size?: 'small' }>()
</script>
<template><AppBadge :tone="tone" :dot="false" class="app-tag" :data-size="size"><slot /></AppBadge></template>
<style scoped>
.app-tag[data-size=small] { padding-block: 2px; }
</style>
