<script setup lang="ts">
import { computed, provide, useId } from 'vue'
import { Label } from '@/components/ui/label'
import { fieldContextKey } from './form-context'

const props = defineProps<{ label: string; for?: string; error?: string; hint?: string; required?: boolean; floating?: boolean }>()
const generatedId = useId()
const context = computed(() => ({
  id: props.for || generatedId,
  descriptionId: `${props.for || generatedId}-description`,
  error: props.error,
  hint: props.hint,
  required: props.required,
  floating: Boolean(props.floating && props.label),
}))
provide(fieldContextKey, context)
</script>

<template>
  <div class="app-field" :class="{ 'app-field--floating': context.floating }" :data-invalid="Boolean(error)">
    <div v-if="context.floating" class="app-field__floating-control">
      <slot />
      <Label :for="context.id" class="app-field__label" :data-required="required || undefined"><slot name="label">{{ label }}</slot></Label>
    </div>
    <template v-else>
      <Label :for="context.id" class="app-field__label" :data-required="required || undefined"><slot name="label">{{ label }}</slot></Label>
      <slot />
    </template>
    <p v-if="error || hint" :id="context.descriptionId" class="app-field__description" :class="{ 'text-danger': error }" :role="error ? 'alert' : undefined">{{ error || hint }}</p>
  </div>
</template>

<style scoped>
.app-field { display: grid; gap: 8px; margin-bottom: 24px; min-width: 0; }
.app-field__label { font-size: 14px; line-height: 1.5; font-weight: 500; }
.app-field__label[data-required]::after { content: '*'; color: var(--text-danger); }
.app-field__description { margin: 0; font-size: 13px; line-height: 1.6; color: var(--muted); }
.app-field[data-invalid=true] .app-field__description { color: var(--text-danger); }
.app-field__floating-control { position: relative; display: grid; gap: 8px; min-width: 0; --floating-label-start: 12px; --floating-label-end: 12px; }
.app-field__floating-control:has(.app-input-prefix) { --floating-label-start: 40px; }
.app-field__floating-control:has(.app-input-reveal), .app-field__floating-control:has([data-slot=select-trigger]) { --floating-label-end: 48px; }
.app-field--floating .app-field__label { position: absolute; z-index: 2; top: 18px; left: var(--floating-label-start); max-width: calc(100% - var(--floating-label-start) - var(--floating-label-end)); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; pointer-events: none; color: var(--floating-label-color, var(--muted)); font-size: 15px; line-height: 20px; font-weight: 400; transform-origin: left top; transition: transform 160ms cubic-bezier(.16, 1, .3, 1), color 160ms ease; }
.app-field__floating-control:has([data-slot=textarea])::before { content: ''; position: absolute; z-index: 1; top: 2px; left: 2px; right: 2px; height: 26px; border-radius: 10px 10px 0 0; background: var(--surface-strong); pointer-events: none; }
.app-field__floating-control:focus-within > .app-field__label,
.app-field__floating-control:has([data-slot=input]:not(:placeholder-shown), [data-slot=textarea]:not(:placeholder-shown), input:autofill, [data-slot=select-trigger]:not([data-placeholder]), [data-slot=select-trigger][data-state=open]) > .app-field__label { transform: translateY(-11px) scale(.8); }
.app-field__floating-control:focus-within > .app-field__label { color: var(--floating-label-active-color, var(--brand-foreground)); }
.app-field[data-invalid=true] .app-field__floating-control > .app-field__label { color: var(--floating-label-error-color, var(--text-danger)); }
.app-field__floating-control:not(:focus-within) :deep(input::placeholder),
.app-field__floating-control:not(:focus-within) :deep(textarea::placeholder) { color: transparent; }
.app-field__floating-control:not(:focus-within) :deep([data-slot=select-trigger][data-placeholder]:not([data-state=open]) [data-slot=select-value]) { visibility: hidden; }
@media (prefers-reduced-motion: reduce), (forced-colors: active) { .app-field--floating .app-field__label { transition: none; } }
@media (forced-colors: active) { .app-field__floating-control > .app-field__label { color: CanvasText; } .app-field__floating-control:focus-within > .app-field__label { color: Highlight; } }
@media (forced-colors: active) { .app-field__floating-control:has([data-slot=textarea])::before { background: Canvas; } }
</style>
