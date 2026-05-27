<script setup lang="ts">
import { computed } from 'vue'

import { composeFieldTooltip, type ConfigFieldDefinition } from '@/lib/config-form'
import { formatRateLimit, fromMultilineList, toMultilineList } from '@/lib/format'
import { t } from '@/i18n'
import RateLimitInput from './RateLimitInput.vue'

const props = defineProps<{
  field: ConfigFieldDefinition
  value: unknown
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:value': [value: unknown]
}>()

const tooltipContent = computed(() => composeFieldTooltip(props.field))
const hasTooltip = computed(() => Boolean(tooltipContent.value))

const fieldId = computed(() => `config-field-${props.field.path.replace(/\./g, '-')}`)

const textValue = computed(() => {
  if (props.value === null || props.value === undefined) {
    return ''
  }
  return String(props.value)
})

const numberValue = computed(() => (typeof props.value === 'number' ? props.value : null))

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
  <div class="config-field">
    <div class="config-field__header">
      <label class="config-field__label" :for="fieldId">
        <span class="config-field__name">{{ field.label }}</span>
        <span v-if="field.unit" class="config-field__unit">· {{ field.unit }}</span>
      </label>
      <a-tooltip
        v-if="hasTooltip"
        placement="top"
        :mouse-enter-delay="0.15"
        :mouse-leave-delay="0.1"
        :overlay-style="{ maxWidth: '320px' }"
        :overlay-inner-style="{ whiteSpace: 'pre-line', fontSize: '12.5px', lineHeight: '1.55' }"
      >
        <template #title>{{ tooltipContent }}</template>
        <button
          type="button"
          class="config-field__info"
          :aria-label="`${field.label} · ${t('config.fieldHelp')}`"
        >
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" aria-hidden="true" focusable="false">
            <circle cx="8" cy="8" r="6.5" stroke="currentColor" stroke-width="1.2" />
            <circle cx="8" cy="5" r="0.85" fill="currentColor" />
            <path d="M8 7.5v4" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" />
          </svg>
        </button>
      </a-tooltip>
    </div>

    <div class="config-field__control">
      <RateLimitInput
        v-if="field.type === 'rateLimit'"
        :value="textValue"
        :aria-label="field.label"
        @update:value="emitRateLimit"
      />
      <a-input
        v-else-if="field.type === 'text'"
        :id="fieldId"
        :value="textValue"
        :placeholder="field.placeholder"
        :disabled="disabled"
        :aria-label="field.label"
        @update:value="emitText"
      />
      <a-input-number
        v-else-if="field.type === 'number'"
        :id="fieldId"
        class="config-field__number"
        :value="numberValue"
        :min="field.min ?? 0"
        :max="field.max"
        :step="field.step ?? 1"
        :disabled="disabled"
        :aria-label="field.label"
        @update:value="emitNumber"
      />
      <div v-else-if="field.type === 'boolean'" class="config-field__switch">
        <a-switch
          :id="fieldId"
          :checked="booleanValue"
          :disabled="disabled"
          :aria-label="field.label"
          @update:checked="emitBoolean"
        />
      </div>
      <a-select
        v-else-if="field.type === 'select'"
        :id="fieldId"
        :value="textValue"
        :options="field.options"
        :disabled="disabled"
        :aria-label="field.label"
        @update:value="emitSelect"
      />
      <a-textarea
        v-else
        :id="fieldId"
        :value="field.type === 'list' ? listValue : textValue"
        :auto-size="{ minRows: 4, maxRows: 8 }"
        :placeholder="field.placeholder"
        :disabled="disabled"
        :aria-label="field.label"
        @update:value="handleTextareaUpdate"
      />
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
  transition: box-shadow 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.config-field:focus-within {
  box-shadow: 0 0 0 3px var(--accent-soft);
}

.config-field__header {
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
  font-size: 0.78rem;
}

.config-field__info {
  appearance: none;
  border: none;
  padding: 0;
  background: transparent;
  color: var(--muted);
  cursor: help;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: var(--radius-sm);
  opacity: 0.65;
  transition: color 0.15s ease, opacity 0.15s ease;
}

.config-field__info:hover {
  color: var(--accent);
  opacity: 1;
}

.config-field__info:focus-visible {
  color: var(--accent);
  opacity: 1;
  outline: 2px solid var(--accent);
  outline-offset: 2px;
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
  font-size: 0.78rem;
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
</style>
