<script setup lang="ts">
defineProps<{
  kind: 'forbidden' | 'missing' | 'offline' | 'server'
}>()
</script>

<template>
  <svg aria-hidden="true" class="fallback-locator" focusable="false" viewBox="0 0 200 200">
    <rect class="fallback-locator__surface" x="20" y="20" width="160" height="160" rx="24" />
    <path class="fallback-locator__frame" d="M70 46H50a4 4 0 0 0-4 4v20M130 46h20a4 4 0 0 1 4 4v20M154 130v20a4 4 0 0 1-4 4h-20M70 154H50a4 4 0 0 1-4-4v-20" />

    <g v-if="kind === 'forbidden'" class="fallback-locator__symbol">
      <path d="M100 68 130 80v23c0 20-12 34-30 42-18-8-30-22-30-42V80l30-12Z" />
      <rect x="88" y="98" width="24" height="22" rx="5" />
      <path d="M93 98v-7a7 7 0 0 1 14 0v7" />
    </g>

    <g v-else-if="kind === 'missing'" class="fallback-locator__symbol">
      <circle cx="94" cy="94" r="25" />
      <path d="m112 112 20 20M94 83v12M94 105h.01" />
    </g>

    <g v-else-if="kind === 'server'" class="fallback-locator__symbol">
      <rect x="70" y="72" width="60" height="24" rx="6" />
      <rect x="70" y="104" width="60" height="24" rx="6" />
      <path d="M82 84h.01M82 116h.01M96 84h22M96 116h14M116 108l10 10M126 108l-10 10" />
    </g>

    <g v-else class="fallback-locator__symbol">
      <circle cx="73" cy="100" r="9" />
      <circle cx="127" cy="100" r="9" />
      <path d="M82 100h36M65 67l70 66" />
    </g>

    <circle class="fallback-locator__point" cx="100" cy="100" r="3" />
  </svg>
</template>

<style scoped>
.fallback-locator {
  display: block;
  width: 100%;
  max-width: 280px;
  color: var(--brand-foreground);
}

.fallback-locator__surface {
  fill: var(--surface-accent);
  stroke: var(--border);
  stroke-width: 1;
}

.fallback-locator__frame,
.fallback-locator__symbol {
  fill: none;
  stroke: currentColor;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 4;
}

.fallback-locator__symbol {
  color: var(--text);
  stroke-width: 3;
}

.fallback-locator__point {
  fill: var(--brand-foreground);
}
</style>
