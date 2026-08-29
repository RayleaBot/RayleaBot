<script setup lang="ts">
import { computed } from 'vue'

import { useToastFeedback } from '@/adapter/feedback'
import ManagementContextActions from '@/components/ManagementContextActions.vue'
import { getLogLevelLabel, getLogProtocolLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import { buildLogContextActions } from '@/lib/management-links'
import { escapeUnsafeDisplayText, safeJsonStringify } from '@/lib/text-safety'
import { t } from '@/i18n'
import type { LogScope } from '@/stores/log-state'
import type { LogDetailResponse, LogSummary } from '@/types/api'

const props = defineProps<{
  loading: boolean
  error: string | null
  summary: LogSummary | null
  detail: LogDetailResponse | null
  scope: LogScope
}>()

const emit = defineEmits<{
  action: []
}>()

const detailJson = computed(() => safeJsonStringify(props.detail?.details ?? {}))
const contextActions = computed(() => (
  props.summary ? buildLogContextActions(props.summary, props.scope) : []
))
const errorToast = computed(() => (
  props.error
    ? {
        key: `log-detail:${props.summary?.timestamp ?? ''}:${props.error}`,
        level: 'error' as const,
        message: props.error,
      }
    : null
))

useToastFeedback(errorToast)

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
      <section class="log-detail-card log-detail-card--message">
        <header class="log-detail-card__header">
          <span>{{ t('logs.fields.message') }}</span>
        </header>
        <pre class="log-detail-card__content log-detail-card__content--message">{{ escapeUnsafeDisplayText(summary.message) }}</pre>
      </section>

      <dl class="log-detail-content__summary">
        <div
          v-for="field in summaryFields"
          :key="field.label"
          class="log-detail-content__field"
        >
          <dt class="log-detail-content__field-label">{{ field.label }}</dt>
          <dd
            class="log-detail-content__field-value"
            :class="{ 'is-mono': field.mono }"
          >
            {{ field.value }}
          </dd>
        </div>
      </dl>

      <ManagementContextActions
        v-if="contextActions.length"
        :actions="contextActions"
        class="log-detail-content__actions"
        @action="emit('action')"
      />

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
.log-detail-content__summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 20px;
  margin: 16px 0 0;
}

.log-detail-content__actions {
  margin-top: 16px;
}

.log-detail-content__field {
  display: grid;
  grid-template-columns: 64px minmax(0, 1fr);
  align-items: baseline;
  gap: 10px;
  min-width: 0;
  padding: 10px 0;
  border-bottom: 1px solid var(--border);
}

.log-detail-content__field dt,
.log-detail-content__field dd { margin: 0; min-width: 0; }
.log-detail-card.log-detail-card--message { margin-top: 0; }

.log-detail-content__field-label {
  color: var(--muted);
  font-size: 13px;
  font-weight: 400;
  letter-spacing: 0;
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
  font-family: var(--font-mono);
}

.log-detail-card {
  display: grid;
  gap: 0;
  margin-top: 16px;
  border-radius: var(--radius-lg);
  border: 1px solid color-mix(in srgb, var(--border) 92%, transparent);
  background: var(--surface-strong);
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
  font-family: var(--font-sans);
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
