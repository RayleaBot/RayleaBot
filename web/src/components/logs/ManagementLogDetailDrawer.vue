<script setup lang="ts">
import { computed } from 'vue'

import { getLogLevelLabel, getLogProtocolLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import { escapeUnsafeDisplayText, safeJsonStringify } from '@/lib/text-safety'
import { t } from '@/i18n'
import type { LogDetailResponse, LogSummary } from '@/types/api'

const props = defineProps<{
  open: boolean
  loading: boolean
  error: string | null
  summary: LogSummary | null
  detail: LogDetailResponse | null
}>()

const emit = defineEmits<{
  close: []
}>()

const detailJson = computed(() => safeJsonStringify(props.detail?.details ?? {}))
</script>

<template>
  <a-drawer
    :open="open"
    :get-container="false"
    placement="right"
    width="min(720px, 92vw)"
    :title="t('logs.detail.title')"
    @close="emit('close')"
  >
    <a-skeleton :loading="loading && !detail" active>
      <template v-if="summary">
        <a-alert
          v-if="error"
          :message="t('errors.common.loadFailed')"
          type="error"
          :description="error"
          show-icon
          class="log-detail-alert"
        />

        <a-descriptions :column="1" bordered size="small">
          <a-descriptions-item :label="t('logs.fields.timestamp')">{{ formatDateTime(summary.timestamp) }}</a-descriptions-item>
          <a-descriptions-item :label="t('logs.fields.level')">{{ getLogLevelLabel(summary.level) }}</a-descriptions-item>
          <a-descriptions-item :label="t('logs.fields.source')">{{ summary.source }}</a-descriptions-item>
          <a-descriptions-item :label="t('logs.filters.protocol')">{{ getLogProtocolLabel(summary.protocol) }}</a-descriptions-item>
          <a-descriptions-item :label="t('logs.fields.plugin')">{{ summary.plugin_id || t('display.empty') }}</a-descriptions-item>
          <a-descriptions-item :label="t('logs.fields.requestId')">{{ summary.request_id || t('display.empty') }}</a-descriptions-item>
          <a-descriptions-item :label="t('logs.fields.message')">
            <pre class="log-detail-message">{{ escapeUnsafeDisplayText(summary.message) }}</pre>
          </a-descriptions-item>
        </a-descriptions>

        <div class="log-detail-json">
          <div class="log-detail-json__title">{{ t('logs.detail.detailsJson') }}</div>
          <pre class="log-detail-json__content">{{ detailJson }}</pre>
        </div>
      </template>
    </a-skeleton>
  </a-drawer>
</template>

<style lang="scss" scoped>
.log-detail-alert {
  margin-bottom: 16px;
}

.log-detail-message,
.log-detail-json__content {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  unicode-bidi: plaintext;
  font-family: "Cascadia Mono", "Consolas", monospace;
}

.log-detail-json {
  margin-top: 20px;
}

.log-detail-json__title {
  margin-bottom: 8px;
  font-weight: 600;
  color: var(--text);
}

.log-detail-json__content {
  padding: 12px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
  line-height: 1.6;
}
</style>
