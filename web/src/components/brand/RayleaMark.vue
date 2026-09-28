<script setup lang="ts">
import { rayleaMark } from '@/preferences/theme-tokens.generated'

withDefaults(defineProps<{
  variant?: 'neutral' | 'chrome' | 'monochrome'
}>(), {
  variant: 'monochrome',
})
</script>

<template>
  <svg
    aria-hidden="true"
    class="raylea-mark"
    :class="`raylea-mark--${variant}`"
    focusable="false"
    :viewBox="rayleaMark.viewBox"
  >
    <path :d="rayleaMark.paths[rayleaMark.outline.pathIndex].d" fill="none" :stroke="rayleaMark.outline.color" :stroke-width="rayleaMark.outline.width" stroke-linejoin="round" />
    <path v-for="part in rayleaMark.paths" :key="part.fill" :d="part.d" :fill="part.fill" :fill-rule="part.fillRule" :opacity="part.opacity" />
  </svg>
</template>

<style scoped>
.raylea-mark {
  display: block;
  width: 24px;
  height: 24px;
  flex: none;
  overflow: visible;
}
</style>
