<script setup lang="ts">
import AppSelect from '@/components/AppSelect.vue'
import AppNumberInput from '@/components/AppNumberInput.vue'
import { computed, ref, watch } from 'vue'

import { t } from '@/i18n'
import {
  buildRateLimitValue,
  normalizePositiveInteger,
  parseRateLimitValue,
  type RateLimitUnit,
} from '@/lib/rate-limit'

const props = defineProps<{
  ariaLabel: string
  value?: string | null
  placeholder?: string
}>()

const emit = defineEmits<{
  'update:value': [value: string]
}>()

const count = ref<number | null>(null)
const windowValue = ref<number | null>(null)
const unit = ref<RateLimitUnit>('s')
const invalid = ref(false)
const defaultParts = computed(() => parseRateLimitValue(props.placeholder ?? ''))

const unitOptions = computed(() => [
  { label: t('config.rateLimit.seconds'), value: 's' },
  { label: t('config.rateLimit.minutes'), value: 'm' },
  { label: t('config.rateLimit.hours'), value: 'h' },
])

watch(() => props.value, (value) => {
  const parsed = parseRateLimitValue(value)
  if (parsed) {
    count.value = parsed.count
    windowValue.value = parsed.windowValue
    unit.value = parsed.unit
    invalid.value = false
    return
  }

  // Incomplete edits live in the parent draft too, so filtering or switching
  // categories cannot silently restore the last valid configuration.
  const partial = /^(\d*)\/(\d*)(s|m|h)$/.exec(value?.trim() ?? '')
  count.value = normalizePositiveInteger(partial?.[1])
  windowValue.value = normalizePositiveInteger(partial?.[2])
  unit.value = partial ? partial[3] as RateLimitUnit : 's'
  invalid.value = Boolean(value?.trim())
}, { immediate: true })

function updateCount(value: unknown) {
  count.value = normalizePositiveInteger(value)
  emitIfValid()
}

function updateWindow(value: unknown) {
  windowValue.value = normalizePositiveInteger(value)
  emitIfValid()
}

function updateUnit(value: unknown) {
  if (value === 's' || value === 'm' || value === 'h') {
    unit.value = value
  }
  emitIfValid()
}

function emitIfValid() {
  const nextValue = buildRateLimitValue({
    count: count.value ?? undefined,
    windowValue: windowValue.value ?? undefined,
    unit: unit.value,
  })

  invalid.value = !nextValue
  emit('update:value', nextValue ?? `${count.value ?? ''}/${windowValue.value ?? ''}${unit.value}`)
}
</script>

<template>
  <div class="rate-limit-input" :class="{ 'rate-limit-input--invalid': invalid }" role="group" :aria-label="ariaLabel">
    <div class="rate-limit-input__grid">
      <label class="rate-limit-input__field">
        <span>{{ t('config.rateLimit.count') }}</span>
        <AppNumberInput nullable
          class="rate-limit-input__number"
          :model-value="count"
          :placeholder="defaultParts ? String(defaultParts.count) : undefined"
          :min="1"
          :step="1"
          :aria-label="`${ariaLabel} ${t('config.rateLimit.count')}`"
          @update:model-value="updateCount"
        />
      </label>

      <label class="rate-limit-input__field">
        <span>{{ t('config.rateLimit.window') }}</span>
        <AppNumberInput nullable
          class="rate-limit-input__number"
          :model-value="windowValue"
          :placeholder="defaultParts?.unit === unit ? String(defaultParts.windowValue) : undefined"
          :min="1"
          :step="1"
          :aria-label="`${ariaLabel} ${t('config.rateLimit.window')}`"
          @update:model-value="updateWindow"
        />
      </label>

      <label class="rate-limit-input__field rate-limit-input__field--unit">
        <span>{{ t('config.rateLimit.unit') }}</span>
        <AppSelect
          class="rate-limit-input__unit"
          :model-value="unit"
          :options="unitOptions"
          :aria-label="`${ariaLabel} ${t('config.rateLimit.unit')}`"
          @update:model-value="updateUnit"
        />
      </label>
    </div>

    <p v-if="invalid" class="rate-limit-input__error">
      {{ t('config.rateLimit.invalid') }}
    </p>
  </div>
</template>

<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;
.rate-limit-input {
  display: grid;
  gap: 8px;
}

.rate-limit-input__grid {
  display: grid;
  grid-template-columns: minmax(96px, 1fr) minmax(112px, 1fr) minmax(96px, 0.8fr);
  gap: 10px;
}

.rate-limit-input__field {
  display: grid;
  gap: 6px;
  min-width: 0;

  span {
    color: var(--muted);
    font-size: 13px;
    font-weight: 600;
    line-height: 1.35;
  }
}

.rate-limit-input__number,
.rate-limit-input__unit {
  width: 100%;
}

.rate-limit-input :deep(.app-number-input),
.rate-limit-input :deep(.app-select) {
  border-radius: var(--radius-md);
}

.rate-limit-input--invalid :deep(.app-number-input),
.rate-limit-input--invalid :deep(.app-select) {
  border-color: var(--danger);
}

.rate-limit-input__error {
  margin: 0;
  color: var(--danger);
  font-size: 13px;
  line-height: 1.45;
}

@media (max-width: #{bp.$phone}) {
  .rate-limit-input__grid {
    grid-template-columns: 1fr;
  }
}
</style>
