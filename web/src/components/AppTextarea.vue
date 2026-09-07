<script setup lang="ts">
import { Textarea } from '@/components/ui/textarea'
import { useFieldContext } from './form-context'
withDefaults(defineProps<{ rows?: number; maxRows?: number }>(), { rows: 3, maxRows: 10 })
const model = defineModel<string>({ default: '' })
const field = useFieldContext()
</script>
<template>
  <Textarea
    :id="field?.id"
    :aria-invalid="Boolean(field?.error) || undefined"
    :aria-describedby="field?.error ? field.descriptionId : undefined"
    :model-value="model"
    :rows="rows"
    class="app-textarea"
    :style="{ minHeight: (rows * 1.6 + 1.25) + 'em', maxHeight: (maxRows * 1.6 + 1.25) + 'em' }"
    @update:model-value="model = String($event)"
  />
</template>
<style scoped>
.app-textarea { resize: vertical; line-height: 1.6; background: var(--surface-strong); color: var(--text); }
</style>
