<script setup lang="ts">
import { Switch } from '@/components/ui/switch'
import { useFieldContext } from './form-context'
const model = defineModel<boolean>({ default: false })
const field = useFieldContext()
defineProps<{ disabled?: boolean }>()
</script>
<template>
  <Switch :id="field?.id" :aria-describedby="field?.error || field?.hint ? field.descriptionId : undefined" :aria-invalid="Boolean(field?.error) || undefined" :aria-required="field?.required || undefined" :model-value="model" :disabled="disabled" class="app-switch" @update:model-value="model = $event" />
</template>
<style lang="scss">
@use '@/styles/breakpoints.generated' as bp;
// Ink switch: the checked track takes the text colour and the knob the surface colour, so both
// themes keep a high-contrast state; the unchecked track uses the control boundary (3:1 on surfaces).
.app-switch { width: 40px; height: 24px; padding: 2px; border: 0; transition: background-color var(--motion-fast, 160ms); }
.app-switch[data-state=unchecked] { background: var(--border-strong); }
.app-switch[data-state=checked] { background: var(--text); }
.app-switch [data-slot=switch-thumb] { width: 20px; height: 20px; background: var(--surface-strong); box-shadow: inset 0 1px 0 rgb(255 255 255 / 55%), 0 1px 3px rgb(0 0 0 / 28%); transition: translate var(--motion-fast, 160ms) cubic-bezier(.16, 1, .3, 1); }
.app-switch [data-state=checked] { translate: 16px 0; }
@media (max-width: #{bp.$phone - 1px}), (pointer: coarse) {
  .app-switch { margin: 10px 2px; }
  .app-switch::after { inset: -10px -2px; }
}
@media (prefers-reduced-motion: reduce), (forced-colors: active) {
  .app-switch, .app-switch [data-slot=switch-thumb] { transition: none; }
}
@media (forced-colors: active) {
  .app-switch { border: 1px solid CanvasText; }
  .app-switch[data-state=checked] { background: Highlight; }
  .app-switch [data-slot=switch-thumb] { background: CanvasText; box-shadow: none; }
}
</style>
