<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { TagsInputInput, TagsInputItem, TagsInputItemDelete, TagsInputItemText, TagsInputRoot } from 'reka-ui'
import { XIcon } from '@lucide/vue'
import { useFieldContext } from './form-context'
defineOptions({ inheritAttrs: false })
const props = defineProps<{ disabled?: boolean; placeholder?: string; separators?: string[] }>()
const model = defineModel<string[]>({ required: true })
const field = useFieldContext()
const attrs = useAttrs()
const delimiter = computed(() => props.separators?.length
  ? new RegExp(props.separators.map(value => value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|'), 'u')
  : '')
</script>
<template>
  <TagsInputRoot
    :model-value="model"
    :disabled="disabled"
    :delimiter="delimiter"
    :add-on-paste="Boolean(separators?.length)"
    add-on-blur
    class="app-tags-input"
    :class="attrs.class"
    :data-testid="attrs['data-testid']"
    @update:model-value="model = $event.map(String)"
  >
    <TagsInputItem v-for="value in model" :key="value" :value="value" class="app-tags-input__item">
      <TagsInputItemText />
      <TagsInputItemDelete aria-labelledby="" :aria-label="'删除 ' + value" class="app-tags-input__delete"><XIcon :size="14" /></TagsInputItemDelete>
    </TagsInputItem>
    <TagsInputInput
      :id="attrs.id as string || field?.id"
      :aria-label="attrs['aria-label'] as string"
      :aria-invalid="Boolean(field?.error) || undefined"
      :aria-describedby="field?.error || field?.hint ? field.descriptionId : undefined"
      :aria-required="field?.required || undefined"
      :placeholder="placeholder"
      class="app-tags-input__entry"
    />
  </TagsInputRoot>
</template>
<style scoped>
.app-tags-input { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; min-height: 40px; padding: 6px 10px; border: 1px solid var(--border-strong); border-radius: 8px; background: var(--surface-strong); color: var(--text); }
.app-tags-input:focus-within { border-color: var(--focus); outline: 2px solid color-mix(in srgb, var(--focus) 50%, transparent); outline-offset: 1px; }
.app-tags-input__item { display: inline-flex; align-items: center; gap: 4px; padding: 2px 6px; border-radius: 4px; background: var(--surface-soft); font-size: 13px; }
.app-tags-input__item[data-state=active] { outline: 2px solid var(--focus); }
.app-tags-input__delete { display: grid; place-items: center; width: 24px; height: 24px; border-radius: 4px; }
.app-tags-input__delete:hover { background: var(--surface-accent); }
.app-tags-input__entry { flex: 1; width: 8ch; min-width: 8ch; border: 0; outline: none; background: transparent; font-size: 14px; }
.app-tags-input[data-disabled] { opacity: .5; }
@media (max-width: 639px), (pointer: coarse) { .app-tags-input { min-height: 44px; } .app-tags-input__delete { min-width: 44px; min-height: 44px; } }
@media (forced-colors: active) { .app-tags-input { border-color: CanvasText; } .app-tags-input:focus-within { outline-color: Highlight; } }
</style>
