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
.app-switch { width: 40px; height: 24px; padding: 2px; }
.app-switch [data-slot=switch-thumb] { width: 18px; height: 18px; background: var(--on-brand); }
.app-switch [data-state=checked] { translate: 16px 0; }
@media (max-width: #{bp.$phone - 1px}), (pointer: coarse) {
  .app-switch { margin: 10px 2px; }
  .app-switch::after { inset: -10px -2px; }
}
@media (prefers-reduced-motion: reduce), (forced-colors: active) {
  .app-switch, .app-switch [data-slot=switch-thumb] { transition: none; }
}
</style>
