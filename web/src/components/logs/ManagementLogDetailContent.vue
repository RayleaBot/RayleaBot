<script setup lang="ts">
import { computed } from 'vue'

import { getLogLevelLabel, getLogProtocolLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import { escapeUnsafeDisplayText, safeJsonStringify } from '@/lib/text-safety'
import { t } from '@/i18n'
import type { LogDetailResponse, LogSummary } from '@/types/api'

const props = defineProps<{
  loading: boolean
  error: string | null
  summary: LogSummary | null
  detail: LogDetailResponse | null
}>()

const detailJson = computed(() => safeJsonStringify(props.detail?.details ?? {}))

const summaryFields = computed(() => {
  if (!props.summary) {
    return []
  }

  return [
    {
      label: t('logs.fields.timestamp'),
      value: formatDateTime(props.summary.timestamp),
    },
    {
      label: t('logs.fields.level'),
      value: getLogLevelLabel(props.summary.level),
    },
    {
      label: t('logs.fields.source'),
      value: props.summary.source || t('display.empty'),
      mono: true,
    },
    {
      label: t('logs.filters.protocol'),
      value: getLogProtocolLabel(props.summary.protocol),
    },
    {
      label: t('logs.fields.plugin'),
      value: props.summary.plugin_id || t('display.empty'),
      mono: true,
    },
    {
      label: t('logs.fields.requestId'),
      value: props.summary.request_id || t('display.empty'),
      mono: true,
    },
  ]
})
</script>

<template>
  <a-skeleton :loading="loading && !detail" active>
    <template v-if="summary">
      <a-alert
        v-if="error"
        :message="t('errors.common.loadFailed')"
        type="error"
        :description="error"
        show-icon
        class="log-detail-content__alert"
      />

      <section class="log-detail-content__summary">
        <div
          v-for="field in summaryFields"
          :key="field.label"
          class="log-detail-content__field"
        >
          <div class="log-detail-content__field-label">{{ field.label }}</div>
          <div
            class="log-detail-content__field-value"
            :class="{ 'is-mono': field.mono }"
          >
            {{ field.value }}
          </div>
        </div>
      </section>

      <section class="log-detail-card">
        <header class="log-detail-card__header">
          <span>{{ t('logs.fields.message') }}</span>
        </header>
        <pre class="log-detail-card__content log-detail-card__content--message">{{ escapeUnsafeDisplayText(summary.message) }}</pre>
      </section>

      <section class="log-detail-card">
        <header class="log-detail-card__header">
          <span>{{ t('logs.detail.detailsJson') }}</span>
        </header>
        <pre class="log-detail-card__content log-detail-card__content--json">{{ detailJson }}</pre>
      </section>
    </template>
  </a-skeleton>
</template>

<style lang="scss" scoped>
.log-detail-content__alert {
  margin-bottom: 16px;
}

.log-detail-content__summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.log-detail-content__field {
  display: grid;
  gap: 6px;
  padding: 12px 14px;
  border-radius: 14px;
  border: 1px solid color-mix(in srgb, var(--border) 92%, transparent);
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--surface-soft) 96%, transparent), transparent 100%),
    color-mix(in srgb, var(--surface-strong) 96%, transparent);
}

.log-detail-content__field-label {
  color: var(--muted);
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.log-detail-content__field-value {
  color: var(--text);
  font-size: 0.92rem;
  line-height: 1.5;
  word-break: break-word;
}

.log-detail-content__field-value.is-mono,
.log-detail-card__content {
  font-family: "Cascadia Mono", "Consolas", monospace;
}

.log-detail-card {
  display: grid;
  gap: 0;
  margin-top: 16px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--border) 92%, transparent);
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--surface-soft) 96%, transparent), transparent 44%),
    color-mix(in srgb, var(--surface-strong) 96%, transparent);
  overflow: hidden;
}

.log-detail-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  border-bottom: 1px solid color-mix(in srgb, var(--border) 92%, transparent);
  color: var(--text);
  font-size: 0.82rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.log-detail-card__content {
  margin: 0;
  padding: 14px;
  color: var(--text);
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
  unicode-bidi: plaintext;
  overflow: auto;
}

.log-detail-card__content--message {
  max-height: min(28vh, 240px);
}

.log-detail-card__content--json {
  max-height: none;
  overflow: visible;
}

@media (max-width: 640px) {
  .log-detail-content__summary {
    grid-template-columns: 1fr;
  }
}
</style>
