<script setup lang="ts">
import { computed, provide, useId } from 'vue'
import { Label } from '@/components/ui/label'
import { fieldContextKey } from './form-context'

const props = defineProps<{ label: string; for?: string; error?: string; hint?: string; required?: boolean }>()
const generatedId = useId()
const context = computed(() => ({
  id: props.for || generatedId,
  descriptionId: `${props.for || generatedId}-description`,
  error: props.error,
  hint: props.hint,
  required: props.required,
}))
provide(fieldContextKey, context)
</script>

<template>
  <div class="app-field" :data-invalid="Boolean(error)">
    <Label :for="context.id" class="app-field__label" :data-required="required || undefined"><slot name="label">{{ label }}</slot></Label>
    <slot />
    <p v-if="error || hint" :id="context.descriptionId" class="app-field__description" :class="{ 'text-danger': error }" :role="error ? 'alert' : undefined">{{ error || hint }}</p>
  </div>
</template>

<style scoped>
.app-field { display: grid; gap: 8px; margin-bottom: 24px; min-width: 0; }
.app-field__label { font-size: 14px; line-height: 1.5; font-weight: 500; }
.app-field__label[data-required]::after { content: '*'; color: var(--text-danger); }
.app-field__description { margin: 0; font-size: 13px; line-height: 1.6; color: var(--muted); }
.app-field[data-invalid=true] .app-field__description { color: var(--text-danger); }
</style>
