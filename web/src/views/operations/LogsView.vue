<script setup lang="ts">
import { computed, onActivated, onDeactivated, onMounted } from 'vue'
import { storeToRefs } from 'pinia'

import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import VirtualDataViewport from '@/components/VirtualDataViewport.vue'
import { getLogLevelLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import { escapeUnsafeDisplayText } from '@/lib/text-safety'
import { t } from '@/i18n'
import { LOG_PAGE_SIZE_OPTIONS } from '@/stores/log-state'
import { useLogsStore } from '@/stores/logs'

const logsStore = useLogsStore()
const {
  canLoadNewer,
  canLoadOlder,
  error,
  filters,
  isLatestPage,
  items,
  loading,
  needsLatestRefresh,
  pendingNewCount,
} = storeToRefs(logsStore)

const pageSizeOptions = LOG_PAGE_SIZE_OPTIONS.map((value) => ({
  label: t('logs.page.sizeOption', { count: value }),
  value,
}))

const pageStateLabel = computed(() => (
  isLatestPage.value
    ? t('logs.page.latestState')
    : t('logs.page.historyState')
))

const pageNoticeLabel = computed(() => {
  if (pendingNewCount.value > 0) {
    return t('logs.page.newLogsNotice', { count: pendingNewCount.value })
  }

  return isLatestPage.value
    ? t('logs.page.latestHint')
    : t('logs.page.historyHint')
})

async function loadLatestLogs() {
  try {
    await logsStore.goToLatestPage()
  } catch {
    // store error state drives the page
  }
}

async function restoreLatestLogs() {
  try {
    await logsStore.restoreLatestPage()
  } catch {
    // store error state drives the page
  }
}

async function applyFilters() {
  try {
    await logsStore.fetchList()
  } catch {
    // store error state drives the page
  }
}

async function handlePageSizeChange(value: number) {
  filters.value = {
    ...filters.value,
    limit: value,
  }
  await loadLatestLogs()
}

async function goToOlderPage() {
  try {
    await logsStore.goToOlderPage()
  } catch {
    // store error state drives the page
  }
}

async function goToNewerPage() {
  try {
    await logsStore.goToNewerPage()
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

function activatePage() {
  logsStore.activate()

  if (loading.value) {
    return
  }

  if (items.value.length === 0) {
    void applyFilters()
    return
  }

  if (needsLatestRefresh.value) {
    void restoreLatestLogs()
  }
}

onMounted(() => {
  activatePage()
})

onActivated(() => {
  activatePage()
})

onDeactivated(() => {
  logsStore.deactivate()
})
</script>

<template>
  <AppPage :title="t('logs.title')" full-height>
    <template #extra>
      <a-button :loading="loading" @click="loadLatestLogs()">
        {{ t('logs.refresh') }}
      </a-button>
    </template>

    <template #toolbar>
      <a-card :bordered="false" class="app-view-card logs-filter-toolbar">
        <a-form layout="vertical" class="logs-filter-grid">
          <a-form-item :label="t('logs.filters.level')">
            <a-select
              v-model:value="filters.level"
              allow-clear
              :options="levelOptions()"
              :placeholder="t('logs.filters.all')"
            />
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
          <a-button type="primary" @click="applyFilters">{{ t('logs.filters.apply') }}</a-button>
        </div>
      </a-card>
    </template>

    <RetryPanel
      v-if="error && items.length === 0"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading"
      @retry="applyFilters()"
    />

    <a-alert v-else-if="error" :message="t('errors.common.loadFailed')" type="error" :description="error" show-icon />

    <div v-else class="logs-page">
      <section class="app-view-card logs-history-toolbar">
        <div class="logs-history-toolbar__meta">
          <a-tag :color="isLatestPage ? 'success' : 'default'">{{ pageStateLabel }}</a-tag>
          <span class="logs-history-toolbar__hint">{{ pageNoticeLabel }}</span>
        </div>

        <div class="logs-history-toolbar__actions">
          <div class="logs-history-toolbar__size">
            <span>{{ t('logs.page.sizeLabel') }}</span>
            <a-select
              :value="filters.limit"
              class="logs-page-size-select"
              :options="pageSizeOptions"
              @change="handlePageSizeChange"
            />
          </div>
          <a-button :disabled="!canLoadNewer" @click="goToNewerPage()">
            {{ t('logs.page.newer') }}
          </a-button>
          <a-button :disabled="!canLoadOlder" @click="goToOlderPage()">
            {{ t('logs.page.older') }}
          </a-button>
          <a-button type="primary" ghost :disabled="isLatestPage" @click="loadLatestLogs()">
            {{ t('logs.page.backToLatest') }}
          </a-button>
        </div>
      </section>

      <section class="logs-data-table">
        <header class="logs-table-header data-panel-header">
          <span>{{ t('logs.fields.timestamp') }}</span>
          <span>{{ t('logs.fields.level') }}</span>
          <span>{{ t('logs.fields.source') }}</span>
          <span>{{ t('logs.fields.message') }}</span>
        </header>

        <VirtualDataViewport
          class="logs-data-viewport"
          :items="items"
          :item-height="64"
          dynamic-item-height
          :overscan="6"
          :empty-label="t('display.empty')"
          :get-item-key="(item) => item.log_id"
        >
          <template #default="{ item }">
            <article class="logs-table-row">
              <div class="logs-table-cell log-cell-time">
                <div class="log-time-display">{{ formatDateTime(item.timestamp) }}</div>
                <small class="log-request-id">{{ item.request_id ?? t('display.empty') }}</small>
              </div>

              <div class="logs-table-cell">
                <a-tag size="small" :color="getLevelColor(item.level)">
                  {{ getLogLevelLabel(item.level) }}
                </a-tag>
              </div>

              <div class="logs-table-cell log-cell-source">
                <div class="log-source-text">{{ item.source }}</div>
                <small v-if="item.plugin_id" class="log-plugin-id">{{ item.plugin_id }}</small>
              </div>

              <div class="logs-table-cell">
                <p class="log-message-text" :title="escapeUnsafeDisplayText(item.message)">
                  {{ escapeUnsafeDisplayText(item.message) }}
                </p>
              </div>
            </article>
          </template>
        </VirtualDataViewport>
      </section>
    </div>
  </AppPage>
</template>

<style lang="scss" scoped>
.logs-page {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  gap: 12px;
}

.logs-history-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
}

.logs-history-toolbar__meta,
.logs-history-toolbar__actions,
.logs-history-toolbar__size {
  display: flex;
  align-items: center;
  gap: 10px;
}

.logs-history-toolbar__meta {
  min-width: 0;
}

.logs-history-toolbar__hint {
  min-width: 0;
  color: var(--muted);
  font-size: 0.84rem;
  line-height: 1.4;
}

.logs-page-size-select {
  min-width: 132px;
}

.logs-data-table {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
}

.logs-table-header,
.logs-table-row {
  display: grid;
  grid-template-columns: 190px 100px 180px minmax(0, 1fr);
  gap: 12px;
}

.logs-data-viewport {
  flex: 1 1 auto;
  min-height: 0;
  border-top-left-radius: 0;
  border-top-right-radius: 0;
  border-top-width: 0;
}

.logs-table-row {
  min-height: 100%;
  height: auto;
  align-items: center;
  padding: 0 16px;
  border-bottom: 1px solid color-mix(in srgb, var(--border) 80%, transparent);
  background: var(--surface-strong);
}

.logs-table-cell {
  min-width: 0;
}

.log-cell-time,
.log-cell-source {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.log-time-display,
.log-source-text {
  font-size: 0.9rem;
  color: var(--text);
}

.log-source-text {
  font-weight: 600;
}

.log-request-id,
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

@media (max-width: 960px) {
  .logs-history-toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .logs-history-toolbar__actions {
    flex-wrap: wrap;
  }

  .logs-table-header,
  .logs-table-row {
    grid-template-columns: 170px 90px 150px minmax(220px, 1fr);
  }
}

@media (max-width: 720px) {
  .logs-page {
    overflow: auto;
  }

  .logs-data-table {
    min-height: 520px;
  }

  .logs-table-header {
    display: none;
  }

  .logs-table-row {
    grid-template-columns: 1fr;
    align-items: flex-start;
    gap: 8px;
    padding-block: 12px;
  }
}
</style>
