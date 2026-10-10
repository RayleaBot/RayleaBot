<script setup lang="ts">
import { rayleaMarkSources } from '@/preferences/brand-assets.generated'

withDefaults(defineProps<{
  variant?: 'neutral' | 'chrome' | 'monochrome'
  size?: 32 | 40 | 128
}>(), {
  variant: 'monochrome',
  size: 32,
})
</script>

<template>
  <span
    aria-hidden="true"
    class="raylea-mark"
    :class="`raylea-mark--${variant}`"
    :style="{ width: `${size}px`, height: `${size}px` }"
  >
    <img
      :src="rayleaMarkSources[size].src"
      :srcset="rayleaMarkSources[size].srcset"
      :sizes="`${size}px`"
      :width="size"
      :height="size"
      alt=""
      draggable="false"
    >
  </span>
</template>

<style scoped>
.raylea-mark {
  display: grid;
  flex: none;
}

.raylea-mark img {
  grid-area: 1 / 1;
  width: 100%;
  height: 100%;
  object-fit: contain;
}
</style>
