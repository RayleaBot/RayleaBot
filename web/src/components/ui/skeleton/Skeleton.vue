<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/ui'

interface SkeletonProps {
  class?: HTMLAttributes['class']
}

const props = defineProps<SkeletonProps>()
</script>

<template>
  <div
    data-slot="skeleton"
    :class="cn('bg-surface-soft rounded-md animate-pulse', props.class)"
  />
</template>
