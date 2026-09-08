<script setup lang="ts">
import AppHelp from '@/components/AppHelp.vue'
import AppTextarea from '@/components/AppTextarea.vue'
import AppSwitch from '@/components/AppSwitch.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppNumberInput from '@/components/AppNumberInput.vue'
import AppInput from '@/components/AppInput.vue'
import { computed } from 'vue'

import { composeFieldTooltip, type ConfigFieldDefinition } from '@/lib/config-form'
import { formatRateLimit, fromMultilineList, toMultilineList } from '@/lib/format'
import { t } from '@/i18n'
import RateLimitInput from './RateLimitInput.vue'

const props = defineProps<{
  field: ConfigFieldDefinition
  value: unknown
  disabled?: boolean
  layout?: 'stack' | 'row'
}>()

const emit = defineEmits<{
  'update:value': [value: unknown]
}>()

const tooltipContent = computed(() => composeFieldTooltip(props.field))
const hasTooltip = computed(() => Boolean(tooltipContent.value))
const placeholder = computed(() => {
  if (props.field.placeholder) return props.field.placeholder
  const value = props.field.defaultValue
  if (value === null || value === undefined || typeof value === 'boolean') return undefined
  const label = props.field.options?.find(option => option.value === value)?.label
  return (label ?? (Array.isArray(value) ? toMultilineList(value as string[]) : String(value))) || undefined
})

const fieldId = computed(() => `config-field-${props.field.path.replace(/\./g, '-')}`)
const descriptionId = computed(() => props.layout === 'row' && tooltipContent.value ? `${fieldId.value}-description` : undefined)

const textValue = computed(() => {
  if (props.value === null || props.value === undefined) {
    return ''
  }
  return String(props.value)
})

const numberValue = computed(() => (typeof props.value === 'number' ? props.value : null))

const selectValue = computed(() => typeof props.value === 'boolean' ? props.value : textValue.value)

const booleanValue = computed(() => Boolean(props.value))

const listValue = computed(() => {
  if (Array.isArray(props.value)) {
    return toMultilineList(props.value as string[])
  }
  return ''
})

const rateLimitPreview = computed(() => {
  if (props.field.type !== 'rateLimit') {
    return null
  }
  const raw = textValue.value.trim()
  if (!raw) {
    return null
  }
  const preview = formatRateLimit(raw)
  return preview !== raw ? preview : null
})

function emitText(value: unknown) {
  emit('update:value', String(value ?? ''))
}

function emitNumber(value: unknown) {
  if (value === null || value === undefined || value === '') {
    emit('update:value', undefined)
    return
  }
  const next = Number(value)
  emit('update:value', Number.isFinite(next) ? next : undefined)
}

function emitBoolean(value: unknown) {
  emit('update:value', Boolean(value))
}

function emitList(value: unknown) {
  emit('update:value', fromMultilineList(String(value ?? '')))
}

function emitSelect(value: unknown) {
  emit('update:value', value)
}

function emitRateLimit(value: string) {
  emit('update:value', value)
}

function handleTextareaUpdate(value: unknown) {
  if (props.field.type === 'list') {
    emitList(value)
  } else {
    emitText(value)
  }
}
</script>

<template>
  <div class="config-field" :class="{ 'config-field--row': layout === 'row' }" :data-field-path="field.path">
    <div class="config-field__header">
      <div class="config-field__heading">
        <label class="config-field__label" :for="fieldId">
          <span class="config-field__name">{{ field.label }}</span>
          <span v-if="field.unit && layout !== 'row'" class="config-field__unit">· {{ field.unit }}</span>
        </label>
        <AppHelp v-if="hasTooltip && layout !== 'row'" :label="`${field.label} · ${t('config.fieldHelp')}`" :description="tooltipContent || ''" />
      </div>
      <p v-if="descriptionId" :id="descriptionId" class="config-field__description">{{ tooltipContent }}</p>
    </div>

    <div class="config-field__control">
      <RateLimitInput
        v-if="field.type === 'rateLimit'"
        :value="textValue"
        :placeholder="placeholder"
        :ariaLabel="field.label"
        :aria-describedby="descriptionId"
        @update:value="emitRateLimit"
      />
      <AppInput
        v-else-if="field.type === 'text'"
        :id="fieldId"
        :model-value="textValue"
        :placeholder="placeholder"
        :disabled="disabled"
        :aria-label="field.label"
        :aria-describedby="descriptionId"
        @update:model-value="emitText"
      />
      <AppNumberInput nullable
        v-else-if="field.type === 'number'"
        :id="fieldId"
        class="config-field__number"
        :model-value="numberValue"
        :placeholder="placeholder"
        :min="field.min ?? 0"
        :max="field.max"
        :step="field.step ?? 1"
        :disabled="disabled"
        :aria-label="field.label"
        :aria-describedby="descriptionId"
        @update:model-value="emitNumber"
      />
      <div v-else-if="field.type === 'boolean'" class="config-field__switch">
        <AppSwitch
          :id="fieldId"
          :model-value="booleanValue"
          :disabled="disabled"
          :aria-label="field.label"
          :aria-describedby="descriptionId"
          @update:model-value="emitBoolean"
        />
      </div>
      <AppSelect
        v-else-if="field.type === 'select'"
        :id="fieldId"
        :model-value="selectValue"
        :options="field.options || []"
        :placeholder="placeholder"
        :disabled="disabled"
        :aria-label="field.label"
        :aria-describedby="descriptionId"
        @update:model-value="emitSelect"
      />
      <AppTextarea
        v-else
        :id="fieldId"
        :model-value="field.type === 'list' ? listValue : textValue"
        :rows="layout === 'row' ? 3 : 4" :max-rows="8"
        :placeholder="placeholder"
        :disabled="disabled"
        :aria-label="field.label"
        :aria-describedby="descriptionId"
        @update:model-value="handleTextareaUpdate"
      />
      <span v-if="layout === 'row' && field.unit" class="config-field__unit-end" aria-hidden="true">{{ field.unit }}</span>
    </div>

    <div v-if="rateLimitPreview" class="config-field__preview">
      <span class="config-field__preview-label">{{ t('config.hints.rateLimitPreview') }}</span>
      <strong class="config-field__preview-value">{{ rateLimitPreview }}</strong>
    </div>
  </div>
</template>

<style scoped lang="scss">
.config-field {
  display: grid;
  gap: 8px;
  padding: 6px;
  margin: -6px;
  border-radius: var(--radius-md);
}

.config-field__header {
  display: grid;
  gap: 6px;
  min-width: 0;
}

.config-field__heading {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 22px;
}

.config-field__label {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--text);
  line-height: 1.4;
  cursor: default;
}

.config-field__unit {
  color: var(--muted);
  font-weight: 500;
  font-size: 13px;
}

.config-field__control {
  min-width: 0;
}

.config-field__number {
  width: 100%;
}

.config-field__switch {
  display: flex;
  align-items: center;
  min-height: 32px;
}

.config-field__preview {
  display: inline-flex;
  align-items: baseline;
  gap: 6px;
  padding: 6px 10px;
  border-radius: var(--radius-md);
  background: var(--surface-soft);
  border: 1px solid var(--border);
  color: var(--muted);
  font-size: 13px;
  line-height: 1.4;
  width: max-content;
  max-width: 100%;
}

.config-field__preview-label {
  letter-spacing: 0.04em;
}

.config-field__preview-value {
  color: var(--text);
  font-weight: 600;
  font-family: var(--font-mono);
}

.config-field__description { margin: 0; color: var(--muted); font-size: 13px; line-height: 1.65; white-space: pre-line; overflow-wrap: anywhere; }
.config-field--row { grid-template-columns: minmax(0, 1fr) minmax(200px, 264px); gap: 8px 28px; padding: 18px 0; margin: 0; border-radius: 0; border-bottom: 1px solid var(--border); scroll-margin-block: 20px 120px; }
.config-field--row:last-child { border-bottom: 0; }
.config-field--row .config-field__label { font-size: 14px; font-weight: 500; }
.config-field--row .config-field__control { display: flex; align-items: center; align-self: center; gap: 8px; min-width: 0; }
.config-field--row .config-field__control > :first-child { flex: 1; width: 100%; min-width: 0; }
.config-field--row :deep(.rate-limit-input__grid) { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) 76px; gap: 8px; }
.config-field__unit-end { flex: none; color: var(--muted); font-size: 13px; }
.config-field--row .config-field__preview { grid-column: 2; padding: 0; border: 0; background: transparent; flex-wrap: wrap; }
:global([data-density=compact]) .config-field--row { padding-block: 12px; }
@container config-editor (max-width: 560px) {
  .config-field--row { grid-template-columns: minmax(0, 1fr); gap: 10px; }
  .config-field--row .config-field__preview { grid-column: 1; }
}
</style>
