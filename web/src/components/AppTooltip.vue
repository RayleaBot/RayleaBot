<script setup lang="ts">
import { inject } from 'vue'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { overlayLayerKey } from './overlay-layer'
defineProps<{ title: string }>()
const layer = inject(overlayLayerKey, undefined)
</script>
<template><TooltipProvider :delay-duration="450"><Tooltip><TooltipTrigger as-child><slot /></TooltipTrigger><TooltipContent :side-offset="8" :style="{ zIndex: (layer || 1100) + 8 }">{{ title }}</TooltipContent></Tooltip></TooltipProvider></template>
