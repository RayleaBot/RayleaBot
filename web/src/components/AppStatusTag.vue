<script setup lang="ts">
import AppBadge from '@/components/AppBadge.vue'
import { computed } from 'vue'

import { resolveStatusTone } from '@/lib/status-tone'

const props = defineProps<{
  label: string
  status?: string | null
}>()

const resolvedTone = computed(() => resolveStatusTone(props.status))
</script>

<template><AppBadge class="app-status-tag" :tone="resolvedTone">{{ label }}</AppBadge></template>
