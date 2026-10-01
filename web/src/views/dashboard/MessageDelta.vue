<script setup lang="ts">
import { computed } from 'vue'
import { ArrowDownRightIcon, ArrowRightIcon, ArrowUpRightIcon } from '@lucide/vue'

import { t } from '@/i18n'
import { changeRatio } from '@/views/dashboard/message-stats'

const props = defineProps<{ value: number; previous?: number | null; title?: string }>()

// Message volume is neither good nor bad, so the change keeps a neutral chip and only the arrow shows its direction.
const ratio = computed(() => changeRatio(props.value, props.previous))
const icon = computed(() => (ratio.value === null || Math.abs(ratio.value) < 0.0005 ? ArrowRightIcon : ratio.value > 0 ? ArrowUpRightIcon : ArrowDownRightIcon))
const text = computed(() => (ratio.value === null ? t('dashboard.messages.noComparison') : `${Math.abs(ratio.value * 100).toFixed(1)}%`))
const label = computed(() => {
  if (ratio.value === null) return text.value
  const direction = Math.abs(ratio.value) < 0.0005 ? 'flat' : ratio.value > 0 ? 'up' : 'down'
  return t(`dashboard.messages.change.${direction}`, { percent: text.value })
})
</script>

<template>
  <span class="message-delta" :title="title" :aria-label="label">
    <component :is="icon" v-if="ratio !== null" aria-hidden="true" />{{ text }}
  </span>
</template>

<style scoped lang="scss">
.message-delta { display: inline-flex; flex: none; align-items: center; gap: 3px; padding: 2px 8px; border-radius: 999px; background: var(--surface-soft); color: var(--muted); font-size: var(--font-size-xs); font-weight: 700; font-variant-numeric: tabular-nums; white-space: nowrap; }
.message-delta svg { width: 13px; height: 13px; stroke-width: 2.4; }
@media (forced-colors: active) { .message-delta { border: 1px solid CanvasText; } }
</style>
