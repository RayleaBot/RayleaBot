<script setup lang="ts" generic="Nullable extends boolean = false">
import { Input } from '@/components/ui/input'
import { useFieldContext } from './form-context'

defineOptions({ inheritAttrs: false })
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
    :aria-describedby="field?.error || field?.hint ? field.descriptionId : undefined"
    :aria-required="field?.required || undefined"
    type="number" :model-value="typeof model === 'number' && Number.isFinite(model) ? model : ''" class="app-number-input"
    :class="{ 'app-number-input--floating': field?.floating }"
    v-bind="$attrs"
    :placeholder="field?.floating ? ($attrs.placeholder as string) || ' ' : $attrs.placeholder as string | undefined"
    @update:model-value="update"
  />
</template>
<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;
.app-number-input { height: 40px; background: var(--surface-strong); color: var(--text); }
.app-number-input--floating { height: 56px; min-height: 56px; padding-top: 24px; padding-bottom: 8px; line-height: 22px; }
@media (max-width: #{bp.$phone - 1px}), (pointer: coarse) { .app-number-input { min-height: 44px; } }
</style>
