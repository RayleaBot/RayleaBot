<script setup lang="ts">
import { inject } from 'vue'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { overlayLayerKey } from './overlay-layer'
defineProps<{ title?: string }>()
const layer = inject(overlayLayerKey, undefined)
</script>
<template>
  <Popover><PopoverTrigger as-child><slot /></PopoverTrigger><PopoverContent side="bottom" align="start" :side-offset="8" class="app-popover" :style="{ zIndex: (layer || 1100) + 5 }"><strong v-if="title">{{ title }}</strong><slot name="content" /></PopoverContent></Popover>
</template>
<style scoped>
.app-popover { display: grid; gap: 8px; width: min(360px, calc(100vw - 24px)); padding: 16px; font-size: 13px; line-height: 1.7; background: var(--surface-strong); color: var(--text); }
</style>
