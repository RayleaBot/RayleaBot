<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'

import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getLogLevelLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import { escapeUnsafeDisplayText } from '@/lib/text-safety'
import { t } from '@/i18n'
import { useLogsStore } from '@/stores/logs'

const logsStore = useLogsStore()
const { error, filters, items, loading } = storeToRefs(logsStore)

const tableColumns = computed(() => [
  { title: t('logs.fields.timestamp'), key: 'timestamp', dataIndex: 'timestamp', width: 200 },
  { title: t('logs.fields.level'), key: 'level', dataIndex: 'level', width: 110 },
  { title: t('logs.fields.source'), key: 'source', dataIndex: 'source', width: 180 },
  { title: t('logs.fields.message'), key: 'message', dataIndex: 'message' },
])

async function loadLogs() {
  try {
    await logsStore.fetchList()
  } catch {
    // store error state drives the page
  }
}

function levelOptions() {
  return [
    { label: t('display.logLevels.debug'), value: 'debug' },
    { label: t('display.logLevels.info'), value: 'info' },
    { label: t('display.logLevels.warn'), value: 'warn' },
    { label: t('display.logLevels.error'), value: 'error' },
  ]
}

function getLevelColor(level: string) {
  if (level === 'error') return 'error'
  if (level === 'warn') return 'warning'
  if (level === 'info') return 'blue'
  return 'default'
}

onMounted(() => {
  void loadLogs()
})
</script>

<template>
  <AppPage :title="t('logs.title')" full-height>
    <template #extra>
      <a-button :loading="loading" @click="loadLogs()">
        {{ t('logs.refresh') }}
      </a-button>
    </template>

    <template #toolbar>
      <a-card :bordered="false" class="app-view-card logs-filter-toolbar">
        <a-form layout="vertical" class="logs-filter-grid">
          <a-form-item :label="t('logs.filters.level')">
            <a-select v-model:value="filters.level" allow-clear :options="levelOptions()" :placeholder="t('logs.filters.all')" />
          </a-form-item>
          <a-form-item :label="t('logs.filters.source')">
            <a-input v-model:value="filters.source" :placeholder="t('logs.filters.sourcePlaceholder')" />
          </a-form-item>
          <a-form-item :label="t('logs.filters.plugin')">
            <a-input v-model:value="filters.pluginId" :placeholder="t('logs.filters.pluginPlaceholder')" />
          </a-form-item>
          <a-form-item :label="t('logs.filters.requestId')">
            <a-input v-model:value="filters.requestId" :placeholder="t('logs.filters.requestPlaceholder')" />
          </a-form-item>
        </a-form>

        <div class="logs-filter-actions">
          <a-button type="primary" @click="loadLogs()">{{ t('logs.filters.apply') }}</a-button>
        </div>
      </a-card>
    </template>

    <RetryPanel
      v-if="error && items.length === 0"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading"
      @retry="loadLogs()"
    />

    <a-alert v-else-if="error" :message="t('errors.common.loadFailed')" type="error" :description="error" show-icon />

    <a-table
      v-else
      class="logs-data-table app-data-table"
      :columns="tableColumns"
      :data-source="items"
      :pagination="false"
      :row-key="(row) => row.log_id"
      :scroll="{ x: 980 }"
    >
      <template #emptyText>
        {{ t('display.empty') }}
      </template>

      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'timestamp'">
          <div class="log-cell-time">
            <div class="log-time-display">{{ formatDateTime(record.timestamp) }}</div>
            <small class="log-request-id">{{ record.request_id ?? t('display.empty') }}</small>
          </div>
        </template>

        <template v-else-if="column.key === 'level'">
          <a-tag size="small" :color="getLevelColor(record.level)">
            {{ getLogLevelLabel(record.level) }}
          </a-tag>
        </template>

        <template v-else-if="column.key === 'source'">
          <div class="log-cell-source">
            <div class="log-source-text">{{ record.source }}</div>
            <small v-if="record.plugin_id" class="log-plugin-id">{{ record.plugin_id }}</small>
          </div>
        </template>

        <template v-else-if="column.key === 'message'">
          <p class="log-message-text" :title="escapeUnsafeDisplayText(record.message)">
            {{ escapeUnsafeDisplayText(record.message) }}
          </p>
        </template>
      </template>
    </a-table>
  </AppPage>
</template>

<style lang="scss" scoped>
.logs-data-table {
  border-radius: 10px;
  overflow: hidden;
}

.log-cell-time {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.log-time-display {
  font-size: 0.9rem;
  color: var(--text);
}

.log-request-id {
  font-family: "Cascadia Mono", "Consolas", monospace;
  font-size: 0.75rem;
  color: var(--muted);
}

.log-cell-source {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.log-source-text {
  font-size: 0.9rem;
  color: var(--text);
  font-weight: 600;
}

.log-plugin-id {
  font-family: "Cascadia Mono", "Consolas", monospace;
  font-size: 0.75rem;
  color: var(--muted);
}

.log-message-text {
  margin: 0;
  font-size: 0.9rem;
  line-height: 1.5;
  color: var(--text);
  white-space: pre-wrap;
  word-break: break-all;
  unicode-bidi: plaintext;
}
</style>
