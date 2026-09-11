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
.app-segmented { display: flex; gap: 3px; padding: 4px; border-radius: 10px; background: var(--surface-soft); }
.app-segmented__item { flex: 1; min-height: 36px; padding: 6px 8px; border: 1px solid transparent; border-radius: 7px; color: var(--muted); font-size: 13px; white-space: nowrap; cursor: pointer; transition: background-color var(--motion-fast), color var(--motion-fast); }
.app-segmented__item[data-state=checked] { background: var(--surface-strong); border-color: var(--border); color: var(--text); box-shadow: var(--shadow-sm); }
.app-segmented__item:focus-visible { outline: 2px solid var(--focus); outline-offset: var(--focus-outline-offset); }
.app-segmented__item:disabled { opacity: .5; cursor: not-allowed; }
@media (max-width: #{bp.$phone - 1px}), (pointer: coarse) { .app-segmented__item { min-width: 44px; min-height: 44px; } }
@media (prefers-reduced-motion: reduce) { .app-segmented__item { transition: none; } }
</style>
