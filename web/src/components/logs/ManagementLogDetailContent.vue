<script setup lang="ts">
import AppSkeleton from '@/components/AppSkeleton.vue'
import { computed } from 'vue'

import { useToastFeedback } from '@/adapter/feedback'
import ManagementContextActions from '@/components/ManagementContextActions.vue'
import { getLogLevelLabel, getLogProtocolLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import { buildLogContextActions } from '@/lib/management-links'
import { escapeUnsafeDisplayText, safeJsonStringify } from '@/lib/text-safety'
import { t } from '@/i18n'
import { usePluginsStore } from '@/stores/plugins'
import { usePluginDisplayName } from '@/lib/use-plugin-display-name'
import { correlatedRequestId, type LogScope } from '@/stores/log-state'
import type { LogDetailResponse, LogSummary } from '@/types/api'

const props = defineProps<{
  loading: boolean
  error: string | null
  summary: LogSummary | null
  detail: LogDetailResponse | null
  scope: LogScope
  // The floating window already shows time, level, source and protocol in its header.
  summaryInHeader?: boolean
}>()

const emit = defineEmits<{
  action: []
}>()
const pluginsStore = usePluginsStore()
usePluginDisplayName(() => props.summary?.plugin_id)

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

// Only facts the entry actually carries are listed; the reserved request ID of logs outside a request is left out.
const summaryFields = computed(() => {
  const summary = props.summary
  if (!summary) {
    return []
  }

  const requestId = correlatedRequestId(summary.request_id)
  return [
    ...(props.summaryInHeader ? [] : [
      { label: t('logs.fields.timestamp'), value: formatDateTime(summary.timestamp), mono: true },
      { label: t('logs.fields.level'), value: getLogLevelLabel(summary.level) },
      { label: t('logs.fields.source'), value: summary.source || t('display.empty'), mono: true },
      ...(summary.protocol ? [{ label: t('logs.filters.protocol'), value: getLogProtocolLabel(summary.protocol) }] : []),
    ]),
    ...(summary.plugin_id ? [{ label: t('logs.fields.plugin'), value: pluginsStore.getPluginLabel(summary.plugin_id) }] : []),
    ...(requestId ? [{ label: t('logs.fields.requestId'), value: requestId, mono: true }] : []),
  ]
})
</script>

<template>
  <AppSkeleton v-if="loading && !detail" :rows="5" />
  <template v-else>
    <div v-if="summary" class="log-detail-content">
      <section class="log-detail-section">
        <h3 class="log-detail-section__label">{{ t('logs.fields.message') }}</h3>
        <pre class="log-detail-section__inset log-detail-section__inset--message">{{ escapeUnsafeDisplayText(summary.message) }}</pre>
      </section>

      <dl v-if="summaryFields.length" class="log-detail-content__summary">
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
        @action="emit('action')"
      />

      <section class="log-detail-section">
        <h3 class="log-detail-section__label">{{ t('logs.detail.detailsJson') }}</h3>
        <pre class="log-detail-section__inset log-detail-section__inset--json">{{ detailJson }}</pre>
      </section>
    </div>
  </template>
</template>

<style lang="scss" scoped>
.log-detail-content {
  display: grid;
  gap: 16px;
}

.log-detail-section {
  display: grid;
  gap: 8px;
}

.log-detail-section__label {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
  font-weight: 600;
}

// Message and JSON sit on a borderless white inset, like plugin usage and console output.
.log-detail-section__inset {
  margin: 0;
  padding: 12px 14px;
  border-radius: var(--radius-md);
  background: var(--surface-raised);
  color: var(--text);
  font-family: var(--font-mono);
  font-size: 13px;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
  unicode-bidi: plaintext;
  overflow: auto;
}

.log-detail-section__inset--message {
  font-family: var(--font-sans);
  font-size: 14px;
}

.log-detail-content__summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 20px;
  margin: 0;
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

.log-detail-content__field-label {
  color: var(--muted);
  font-size: 13px;
}

.log-detail-content__field-value {
  color: var(--text);
  font-size: 14px;
  line-height: 1.5;
  word-break: break-word;
}

.log-detail-content__field-value.is-mono {
  font-family: var(--font-mono);
  font-size: 13px;
}

@media (forced-colors: active) {
  .log-detail-section__inset { border: 1px solid CanvasText; }
}
</style>
