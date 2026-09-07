<script setup lang="ts">
import AppBadge from '@/components/AppBadge.vue'
import { computed } from 'vue'

import { resolveStatusTone, type StatusTone } from '@/lib/status-tone'

const props = defineProps<{
  label: string
  status?: string | null
  tone?: StatusTone
}>()

const resolvedTone = computed(() => props.tone ?? resolveStatusTone(props.status))
</script>

<template><AppBadge class="app-status-tag" :tone="resolvedTone">{{ label }}</AppBadge></template>
