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
    <path v-for="part in rayleaMark.paths" :key="part.d" :d="part.d" :opacity="part.opacity" />
  </svg>
</template>

<style scoped>
.raylea-mark {
  display: block;
  width: 24px;
  height: 24px;
  flex: none;
  color: currentColor;
  overflow: visible;
}

.raylea-mark path {
  fill: currentColor;
}

.raylea-mark--neutral {
  color: var(--brand-foreground);
}

.raylea-mark--chrome {
  color: var(--brand-foreground);
}
</style>
