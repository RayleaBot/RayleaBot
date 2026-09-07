<script setup lang="ts">
import type { StatusTone } from '@/lib/status-tone'
import AppBadge from './AppBadge.vue'
defineProps<{ tone?: StatusTone }>()
</script>
<template><AppBadge :tone="tone" :dot="false" class="app-tag"><slot /></AppBadge></template>
