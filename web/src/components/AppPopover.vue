<script setup lang="ts">
import { inject } from 'vue'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { overlayLayerKey } from './overlay-layer'
withDefaults(defineProps<{ title?: string; label?: string; side?: 'top' | 'right' | 'bottom' | 'left'; align?: 'start' | 'center' | 'end'; width?: number }>(), { side: 'bottom', align: 'start', width: 360 })
const open = defineModel<boolean>('open', { default: false })
const layer = inject(overlayLayerKey, undefined)
</script>
<template>
  <Popover v-model:open="open"><PopoverTrigger as-child><slot /></PopoverTrigger><PopoverContent :side="side" :align="align" :aria-label="label || title" :side-offset="8" class="app-popover" :style="{ zIndex: (layer || 1100) + 5, width: `min(${width}px, calc(100vw - 24px))` }"><strong v-if="title">{{ title }}</strong><slot name="content" /></PopoverContent></Popover>
</template>
<style scoped>
.app-popover { display: grid; gap: 8px; width: min(360px, calc(100vw - 24px)); padding: 16px; font-size: 13px; line-height: 1.7; background: var(--surface-strong); color: var(--text); }
</style>
