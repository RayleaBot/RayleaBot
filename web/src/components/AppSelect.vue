<script setup lang="ts" generic="T extends string | number, M extends boolean = false">
import { computed, inject } from 'vue'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useFieldContext } from './form-context'
import { overlayLayerKey } from './overlay-layer'

const props = defineProps<{
  options: readonly { value: T; label: string; disabled?: boolean }[]
  multiple?: M; placeholder?: string; disabled?: boolean; id?: string
}>()
const model = defineModel<M extends true ? T[] : T>({ required: true })
const field = useFieldContext()
const layer = inject(overlayLayerKey, computed(() => 1100))
defineOptions({ inheritAttrs: false })
const selectedLabels = computed(() => Array.isArray(model.value)
  ? props.options.filter((option) => (model.value as T[]).includes(option.value)).map((option) => option.label).join('、')
  : undefined)
function update(value: unknown) { model.value = value as M extends true ? T[] : T }
</script>
<template>
  <Select :model-value="model" :multiple="multiple" :disabled="disabled" @update:model-value="update">
    <SelectTrigger :id="id || field?.id" v-bind="$attrs" class="app-select" :aria-invalid="Boolean(field?.error) || undefined" :aria-describedby="field?.error ? field.descriptionId : undefined">
      <span v-if="multiple && selectedLabels" class="app-select__value">{{ selectedLabels }}</span>
      <SelectValue v-else :placeholder="placeholder || '请选择'" />
    </SelectTrigger>
    <SelectContent position="popper" class="app-select-content p-1" :style="{ zIndex: layer + 5 }" :side-offset="4">
      <SelectItem v-for="option in options" :key="option.value" :value="option.value" :disabled="option.disabled" class="min-h-9 px-3 pr-9">{{ option.label }}</SelectItem>
    </SelectContent>
  </Select>
</template>
<style scoped>
.app-select { min-height: 40px; height: auto; width: 100%; padding: 9px 12px; background: var(--surface-strong); }
.app-select__value { text-align: left; white-space: normal; overflow-wrap: anywhere; line-height: 1.5; }
@media (pointer: coarse) { .app-select { min-height: 44px; } }
</style>
