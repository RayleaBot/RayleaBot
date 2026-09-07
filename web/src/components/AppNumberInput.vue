<script setup lang="ts" generic="Nullable extends boolean = false">
import { Input } from '@/components/ui/input'
import { useFieldContext } from './form-context'

const props = defineProps<{ nullable?: Nullable & boolean }>()
const model = defineModel<Nullable extends true ? number | null : number>({ required: true })
const field = useFieldContext()
function update(value: string | number) {
  model.value = (value === '' ? props.nullable ? null : Number.NaN : Number(value)) as Nullable extends true ? number | null : number
}
</script>
<template>
  <Input
    :id="field?.id" :aria-invalid="Boolean(field?.error) || undefined"
    :aria-describedby="field?.error ? field.descriptionId : undefined"
    type="number" :model-value="typeof model === 'number' && Number.isFinite(model) ? model : ''" class="app-number-input"
    @update:model-value="update"
  />
</template>
<style scoped>
.app-number-input { height: 40px; background: var(--surface-strong); color: var(--text); }
@media (pointer: coarse) { .app-number-input { min-height: 44px; } }
</style>
