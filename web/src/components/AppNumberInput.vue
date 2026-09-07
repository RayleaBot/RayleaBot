<script setup lang="ts">
import { Input } from '@/components/ui/input'
import { useFieldContext } from './form-context'

const model = defineModel<number>({ required: true })
const field = useFieldContext()
</script>
<template>
  <Input
    :id="field?.id" :aria-invalid="Boolean(field?.error) || undefined"
    :aria-describedby="field?.error ? field.descriptionId : undefined"
    type="number" :model-value="Number.isFinite(model) ? model : ''" class="app-number-input"
    @update:model-value="model = $event === '' ? Number.NaN : Number($event)"
  />
</template>
<style scoped>
.app-number-input { height: 40px; background: var(--surface-strong); color: var(--text); }
@media (pointer: coarse) { .app-number-input { min-height: 44px; } }
</style>
