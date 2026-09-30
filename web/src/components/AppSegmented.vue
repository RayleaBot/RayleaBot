<script setup lang="ts" generic="T extends string">
import { RadioGroupItem, RadioGroupRoot } from 'reka-ui'
defineProps<{ options: { label: string; value: T; disabled?: boolean }[]; label?: string }>()
const model = defineModel<T>({ required: true })
</script>
<template>
  <RadioGroupRoot v-model="model" class="app-segmented" :aria-label="label" orientation="horizontal">
    <RadioGroupItem v-for="option in options" :key="option.value" :value="option.value" :disabled="option.disabled" class="app-segmented__item">{{ option.label }}</RadioGroupItem>
  </RadioGroupRoot>
</template>
<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;
// A pill track whose checked segment is lit, like the workspace tabs.
.app-segmented { display: flex; gap: 2px; padding: 3px; border-radius: 999px; background: color-mix(in srgb, var(--text) 7%, transparent); }
.app-segmented__item { flex: 1; min-height: 34px; padding: 6px 14px; border: 0; border-radius: 999px; color: var(--muted); font-size: 13px; white-space: nowrap; cursor: pointer; transition: background-color var(--motion-fast), color var(--motion-fast); }
.app-segmented__item:not([data-state=checked]):not(:disabled):hover { color: var(--text); }
.app-segmented__item[data-state=checked] { background: var(--surface-lit); color: var(--on-lit); font-weight: 600; box-shadow: inset 0 1px 0 var(--glass-rim), 0 3px 10px -5px color-mix(in srgb, var(--text) 35%, transparent); }
.app-segmented__item:focus-visible { outline: 2px solid var(--focus); outline-offset: var(--focus-outline-offset); }
.app-segmented__item[data-state=checked]:focus-visible { outline-color: var(--on-lit); }
.app-segmented__item:disabled { opacity: .5; cursor: not-allowed; }
@media (max-width: #{bp.$phone - 1px}), (pointer: coarse) { .app-segmented__item { min-width: 44px; min-height: 44px; } }
@media (prefers-reduced-motion: reduce) { .app-segmented__item { transition: none; } }
@media (forced-colors: active) { .app-segmented { border: 1px solid CanvasText; } .app-segmented__item[data-state=checked] { outline: 2px solid Highlight; outline-offset: -2px; } }
</style>
