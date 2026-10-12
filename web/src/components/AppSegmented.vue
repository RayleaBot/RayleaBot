<script setup lang="ts" generic="T extends string">
import { RadioGroupItem, RadioGroupRoot } from 'reka-ui'
withDefaults(defineProps<{ options: { label: string; value: T; disabled?: boolean; title?: string }[]; label?: string; fit?: 'equal' | 'content' }>(), { fit: 'equal' })
const model = defineModel<T>({ required: true })
</script>
<template>
  <RadioGroupRoot v-model="model" class="app-segmented" :class="{ 'app-segmented--content': fit === 'content' }" :aria-label="label" orientation="horizontal">
    <RadioGroupItem v-for="option in options" :key="option.value" :value="option.value" :disabled="option.disabled" :title="option.title" class="app-segmented__item">{{ option.label }}</RadioGroupItem>
  </RadioGroupRoot>
</template>
<style scoped lang="scss">
// A recessed pill track whose checked segment is a white raised pill.
.app-segmented { display: flex; gap: 2px; padding: 3px; border-radius: 999px; background: var(--surface-soft); }
.app-segmented__item { flex: 1; min-height: 34px; padding: 6px 14px; border: 0; border-radius: 999px; color: var(--muted); font-size: 13px; white-space: nowrap; cursor: pointer; transition: background-color var(--motion-fast), color var(--motion-fast); }
// Long labels such as account names keep their own width and shorten with an ellipsis instead of equal columns.
.app-segmented--content { display: inline-flex; max-width: 100%; }
.app-segmented--content .app-segmented__item { flex: 0 1 auto; min-width: 0; max-width: 18rem; overflow: hidden; text-overflow: ellipsis; }
.app-segmented__item:not([data-state=checked]):not(:disabled):hover { color: var(--text); }
.app-segmented__item[data-state=checked] { background: var(--surface-raised); color: var(--text); font-weight: 700; box-shadow: var(--shadow-xs); }
.app-segmented__item:focus-visible { outline: 2px solid var(--focus); outline-offset: var(--focus-outline-offset); }
.app-segmented__item:disabled { opacity: .5; cursor: not-allowed; }
@media (pointer: coarse) { .app-segmented__item { min-width: 44px; min-height: 44px; } }
@media (prefers-reduced-motion: reduce) { .app-segmented__item { transition: none; } }
@media (forced-colors: active) { .app-segmented { border: 1px solid CanvasText; } .app-segmented__item[data-state=checked] { outline: 2px solid Highlight; outline-offset: -2px; } }
</style>
