<script setup lang="ts">
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
    viewBox="0 0 24 24"
  >
    <path d="M9 3H5a2 2 0 0 0-2 2v4M15 3h4a2 2 0 0 1 2 2v4M21 15v4a2 2 0 0 1-2 2h-4M9 21H5a2 2 0 0 1-2-2v-4" />
    <circle cx="12" cy="12" r="2.25" />
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
  fill: none;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 2;
}

.raylea-mark circle {
  fill: currentColor;
}

.raylea-mark--neutral {
  color: var(--brand-foreground);
}

.raylea-mark--chrome {
  color: var(--chrome-text);
}
</style>
