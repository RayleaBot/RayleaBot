<script setup lang="ts" generic="T extends string | number | boolean, M extends boolean = false">
import { t } from '@/i18n'
import { computed, inject, ref, type HTMLAttributes } from 'vue'
import { XIcon } from '@lucide/vue'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useFieldContext } from './form-context'
import { overlayLayerKey } from './overlay-layer'

const props = defineProps<{
  options: readonly { value: T; label: string; disabled?: boolean }[]
  multiple?: M & boolean; placeholder?: string; disabled?: boolean; id?: string; clearable?: boolean; wrapperClass?: HTMLAttributes['class']
}>()
const model = defineModel<M extends true ? T[] : T>({ required: true })
defineEmits<{ open: [value: boolean] }>()
const field = useFieldContext()
const layer = inject(overlayLayerKey, computed(() => 1100))
const control = ref<HTMLElement | null>(null)
defineOptions({ inheritAttrs: false })
// Reka uses string values internally; keep domain values and their primitive types at the product boundary.
const selectOptions = computed(() => {
  const options = [...props.options]
  const selected = Array.isArray(model.value) ? model.value : [model.value]
  for (const value of selected as T[]) {
    if (value !== null && value !== undefined && value !== '' && !options.some(option => option.value === value)) options.push({ value, label: String(value) })
  }
  return options
})
function encode(value: T) { return typeof value + ':' + String(value) }
const selection = computed(() => Array.isArray(model.value)
  ? (model.value as T[]).map(encode)
  : model.value === '' && !props.options.some(option => option.value === '') ? undefined : encode(model.value as T))
const selectedLabels = computed(() => Array.isArray(model.value)
  ? selectOptions.value.filter((option) => (model.value as T[]).includes(option.value)).map((option) => option.label).join('、')
  : selectOptions.value.find(option => option.value === model.value)?.label)
function update(value: unknown) {
  const decode = (key: unknown) => selectOptions.value.find(option => encode(option.value) === key)
  if (Array.isArray(value)) model.value = value.flatMap(key => { const option = decode(key); return option ? [option.value] : [] }) as M extends true ? T[] : T
  else { const option = decode(value); if (option) model.value = option.value as M extends true ? T[] : T }
}
function clearSelection() {
  model.value = [] as unknown as M extends true ? T[] : T
  control.value?.querySelector<HTMLButtonElement>('[data-slot=select-trigger]')?.focus()
}
</script>
<template>
  <div ref="control" class="app-select-wrap" :class="wrapperClass">
    <Select :model-value="selection" :multiple="multiple" :disabled="disabled" :required="field?.required" @update:model-value="update" @update:open="$emit('open', $event)">
      <SelectTrigger :id="id || field?.id" :aria-invalid="Boolean(field?.error) || undefined" :aria-describedby="field?.error || field?.hint ? field.descriptionId : undefined" :aria-required="field?.required || undefined" v-bind="$attrs" class="app-select" :class="{ 'app-select--clearable': multiple && clearable && selectedLabels, 'app-select--floating': field?.floating }">
        <span v-if="selectedLabels" class="app-select__value">{{ selectedLabels }}</span>
        <SelectValue v-else :placeholder="placeholder || t('ui.select')" />
      </SelectTrigger>
      <button v-if="multiple && clearable && selectedLabels" type="button" class="app-select-clear" :aria-label="t('ui.clearSelection')" :disabled="disabled" @click="clearSelection"><XIcon :size="15" /></button>
    <SelectContent position="popper" class="app-select-content p-1" :style="{ zIndex: layer + 5 }" :side-offset="4">
      <SelectItem v-for="option in selectOptions" :key="encode(option.value)" :value="encode(option.value)" :disabled="option.disabled" class="min-h-9 px-3 pr-9">{{ option.label }}</SelectItem>
    </SelectContent>
    </Select>
  </div>
</template>
<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;
.app-select { min-height: 40px; height: auto; width: 100%; padding: 9px 12px; background: var(--surface-strong); }
.app-select--floating { min-height: 56px; padding-top: 24px; padding-bottom: 8px; padding-right: 40px; }
.app-select--floating :deep(svg:last-child) { position: absolute; right: 12px; top: 50%; translate: 0 -50%; }
.app-select-wrap { position: relative; min-width: 0; width: 100%; }
.app-select--clearable { position: relative; padding-right: 72px; }
.app-select--clearable :deep(svg:last-child) { position: absolute; right: 12px; top: 50%; translate: 0 -50%; }
.app-select-clear { position: absolute; right: 32px; top: 50%; translate: 0 -50%; display: grid; place-items: center; width: 32px; height: 36px; border-radius: 6px; color: var(--muted); }
.app-select-clear:hover { color: var(--text); background: var(--surface-soft); }
.app-select__value { text-align: left; white-space: normal; overflow-wrap: anywhere; line-height: 1.5; }
@media (max-width: #{bp.$phone - 1px}), (pointer: coarse) { .app-select { min-height: 44px; } .app-select--clearable { padding-right: 82px; } .app-select-clear { width: 44px; height: 44px; } .app-select-content :deep([role=option]) { min-height: 44px; } }
</style>
