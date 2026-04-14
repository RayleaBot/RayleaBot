<script setup lang="ts">
import { computed, nextTick, onActivated, onDeactivated, onMounted, onUnmounted, ref } from 'vue'
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
const pageRoot = ref<HTMLElement | null>(null)
const tableScrollY = ref<number>()
const desktopTableBottomGap = 12
let layoutObserver: ResizeObserver | null = null

const tableColumns = computed(() => [
  { title: t('logs.fields.timestamp'), key: 'timestamp', dataIndex: 'timestamp', width: 200 },
  { title: t('logs.fields.level'), key: 'level', dataIndex: 'level', width: 110 },
  { title: t('logs.fields.source'), key: 'source', dataIndex: 'source', width: 180 },
  { title: t('logs.fields.message'), key: 'message', dataIndex: 'message' },
])
const tableScroll = computed(() => (
  tableScrollY.value && tableScrollY.value > 0
    ? { x: 980, y: tableScrollY.value }
    : { x: 980 }
))

async function loadLogs() {
  try {
    await logsStore.fetchList()
  } catch {
    // store error state drives the page
  }
  updateTableScrollHeight()
  startLayoutObserver()
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
  updateTableScrollHeight()
  startLayoutObserver()
  void loadLogs()
})

onActivated(() => {
  updateTableScrollHeight()
  startLayoutObserver()
})

onDeactivated(() => {
  stopLayoutObserver()
})

onUnmounted(() => {
  stopLayoutObserver()
})

function updateTableScrollHeight() {
  if (typeof window === 'undefined') {
    return
  }

  void nextTick(() => {
    const root = pageRoot.value
    if (!root) {
      tableScrollY.value = undefined
      return
    }

    const mobileQuery = typeof window.matchMedia === 'function'
      ? window.matchMedia('(max-width: 900px)')
      : null
    if (mobileQuery?.matches) {
      tableScrollY.value = undefined
      return
    }

    const shellMain = root.closest('.admin-layout__content') as HTMLElement | null
    const rootRect = root.getBoundingClientRect()
    const shellRect = shellMain?.getBoundingClientRect()
    const rootStyles = window.getComputedStyle(root)
    const shellStyles = shellMain ? window.getComputedStyle(shellMain) : null
    const visibleBottom = Math.min(shellRect?.bottom ?? window.innerHeight, window.innerHeight)
    const availableHeight = Math.floor(
      visibleBottom
      - rootRect.top
      - getInsetValue(rootStyles.paddingBottom)
      - getInsetValue(shellStyles?.paddingBottom)
      - desktopTableBottomGap
    )

    tableScrollY.value = availableHeight > 180 ? availableHeight : undefined
  })
}

function startLayoutObserver() {
  if (typeof window === 'undefined') {
    return
  }

  const root = pageRoot.value
  const shellMain = root?.closest('.admin-layout__content') as HTMLElement | null
  if (!root || typeof window.ResizeObserver !== 'function' || layoutObserver) {
    return
  }

  layoutObserver = new window.ResizeObserver(() => {
    updateTableScrollHeight()
  })

  layoutObserver.observe(root)
  if (shellMain) {
    layoutObserver.observe(shellMain)
  }
}

function stopLayoutObserver() {
  layoutObserver?.disconnect()
  layoutObserver = null
}

function getInsetValue(value?: string) {
  return Number.parseFloat(value || '0') || 0
}
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

    <div v-else ref="pageRoot" class="logs-page">
      <a-table
        class="logs-data-table app-data-table"
        :columns="tableColumns"
        :data-source="items"
        :pagination="false"
        :row-key="(row) => row.log_id"
        :scroll="tableScroll"
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
    </div>
  </AppPage>
</template>

<style lang="scss" scoped>
.logs-page {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}

.logs-data-table {
  flex: 1 1 auto;
  min-height: 0;
  border-radius: 10px;
  overflow: hidden;
}

.logs-data-table :deep(.ant-spin-nested-loading),
.logs-data-table :deep(.ant-spin-container),
.logs-data-table :deep(.ant-table-wrapper),
.logs-data-table :deep(.ant-table),
.logs-data-table :deep(.ant-table-container) {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
}

.logs-data-table :deep(.ant-table-header),
.logs-data-table :deep(.ant-table-body) {
  flex-shrink: 0;
}

.logs-data-table :deep(.ant-table-body) {
  overscroll-behavior: contain;
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

@media (max-width: 900px) {
  .logs-page {
    overflow: auto;
  }

  .logs-data-table {
    flex: 0 0 auto;
  }
}
</style>
