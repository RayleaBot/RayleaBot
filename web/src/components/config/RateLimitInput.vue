<script setup lang="ts">
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
}>()

const emit = defineEmits<{
  'update:value': [value: string]
}>()

const count = ref<number | null>(null)
const windowValue = ref<number | null>(null)
const unit = ref<RateLimitUnit>('s')
const invalid = ref(false)

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

  count.value = null
  windowValue.value = null
  unit.value = 's'
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
  if (nextValue) {
    emit('update:value', nextValue)
  }
}
</script>

<template>
  <div class="rate-limit-input" :class="{ 'rate-limit-input--invalid': invalid }" role="group" :aria-label="ariaLabel">
    <div class="rate-limit-input__grid">
      <label class="rate-limit-input__field">
        <span>{{ t('config.rateLimit.count') }}</span>
        <a-input-number
          class="rate-limit-input__number"
          :value="count"
          :min="1"
          :precision="0"
          :step="1"
          :aria-label="`${ariaLabel} ${t('config.rateLimit.count')}`"
          @update:value="updateCount"
        />
      </label>

      <label class="rate-limit-input__field">
        <span>{{ t('config.rateLimit.window') }}</span>
        <a-input-number
          class="rate-limit-input__number"
          :value="windowValue"
          :min="1"
          :precision="0"
          :step="1"
          :aria-label="`${ariaLabel} ${t('config.rateLimit.window')}`"
          @update:value="updateWindow"
        />
      </label>

      <label class="rate-limit-input__field rate-limit-input__field--unit">
        <span>{{ t('config.rateLimit.unit') }}</span>
        <a-select
          class="rate-limit-input__unit"
          :value="unit"
          :options="unitOptions"
          :aria-label="`${ariaLabel} ${t('config.rateLimit.unit')}`"
          @update:value="updateUnit"
        />
      </label>
    </div>

    <p v-if="invalid" class="rate-limit-input__error">
      {{ t('config.rateLimit.invalid') }}
    </p>
  </div>
</template>

<style scoped lang="scss">
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
    font-size: 0.78rem;
    font-weight: 600;
    line-height: 1.35;
  }
}

.rate-limit-input__number,
.rate-limit-input__unit {
  width: 100%;
}

.rate-limit-input :deep(.ant-input-number),
.rate-limit-input :deep(.ant-select-selector) {
  border-radius: var(--radius-md);
}

.rate-limit-input--invalid :deep(.ant-input-number),
.rate-limit-input--invalid :deep(.ant-select-selector) {
  border-color: var(--danger);
}

.rate-limit-input__error {
  margin: 0;
  color: var(--danger);
  font-size: 0.8rem;
  line-height: 1.45;
}

@media (max-width: 640px) {
  .rate-limit-input__grid {
    grid-template-columns: 1fr;
  }
}
</style>
