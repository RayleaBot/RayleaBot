<script setup lang="ts">
import { useId } from 'vue'

defineProps<{ title: string; meta?: string }>()
const titleId = useId()
</script>

<template>
  <!-- A light gray box on the status page: a title row with optional meta and actions, then rows. -->
  <section class="status-section app-box" :aria-labelledby="titleId">
    <header class="status-section__header">
      <h2 :id="titleId" class="status-section__title">{{ title }}</h2>
      <span v-if="meta" class="status-section__meta">{{ meta }}</span>
      <div v-if="$slots.actions" class="status-section__actions">
        <slot name="actions" />
      </div>
    </header>
    <div class="status-section__body">
      <slot />
    </div>
  </section>
</template>

<style scoped lang="scss">
.status-section { display: flex; flex-direction: column; min-width: 0; padding: 16px 20px 10px; }
.status-section__header { display: flex; align-items: center; gap: 10px; min-height: 36px; margin-bottom: 4px; }
.status-section__title { margin: 0; color: var(--text); font-size: var(--font-size-lg); font-weight: 700; letter-spacing: -0.01em; }
.status-section__meta { color: var(--muted); font-size: var(--font-size-sm); font-variant-numeric: tabular-nums; }
.status-section__actions { display: flex; align-items: center; gap: 8px; margin-left: auto; }
.status-section__body { display: flex; flex: 1; flex-direction: column; min-height: 0; }
</style>
