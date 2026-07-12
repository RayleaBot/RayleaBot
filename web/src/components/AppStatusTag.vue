<script setup lang="ts">
import { computed } from 'vue'

import { resolveStatusTone, type StatusTone } from '@/lib/status-tone'

const props = defineProps<{
  label: string
  status?: string | null
  tone?: StatusTone
}>()

const resolvedTone = computed(() => props.tone ?? resolveStatusTone(props.status))
</script>

<template>
  <a-tag class="app-status-tag" :data-tone="resolvedTone">
    <span class="app-status-tag__dot" aria-hidden="true" />
    {{ label }}
  </a-tag>
</template>

<style scoped lang="scss">
.app-status-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-inline-end: 0;
  color: var(--muted);
  background: var(--surface-soft);
  border-color: var(--border);
  font-size: 12px;
}

.app-status-tag__dot {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: currentColor;
}

.app-status-tag[data-tone='info'] {
  color: var(--text-accent);
  background: var(--surface-accent);
  border-color: var(--border-accent);
}

.app-status-tag[data-tone='success'] {
  color: var(--text-success);
  background: var(--surface-success);
  border-color: var(--border-success);
}

.app-status-tag[data-tone='warning'] {
  color: var(--text-warning);
  background: var(--surface-warning);
  border-color: var(--border-warning);
}

.app-status-tag[data-tone='attention'] {
  color: var(--text-attention);
  background: var(--surface-attention);
  border-color: var(--border-attention);
}

.app-status-tag[data-tone='danger'] {
  color: var(--text-danger);
  background: var(--surface-danger);
  border-color: var(--border-danger);
}
</style>
