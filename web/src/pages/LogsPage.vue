<script setup lang="ts">
import { onMounted } from 'vue'
import { storeToRefs } from 'pinia'

import RetryPanel from '@/components/RetryPanel.vue'
import VirtualDataViewport from '@/components/VirtualDataViewport.vue'
import { getLogLevelLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import { t } from '@/i18n'
import { useLogsStore } from '@/stores/logs'

const logsStore = useLogsStore()
const { error, filters, items, loading } = storeToRefs(logsStore)

async function loadLogs() {
  try {
    await logsStore.fetchList()
  } catch {
    // store error state drives the page
  }
}

onMounted(() => {
  void loadLogs()
})
</script>

<template>
  <div class="page-grid page-grid--viewport">
    <section class="hero-panel">
      <div>
        <h1>{{ t('logs.title') }}</h1>
      </div>

      <el-button :loading="loading" @click="loadLogs()">
        {{ t('logs.refresh') }}
      </el-button>
    </section>

    <el-card class="logs-filter-toolbar">
      <el-form label-position="top" class="logs-filter-grid">
        <el-form-item :label="t('logs.filters.level')">
          <el-select v-model="filters.level" clearable :placeholder="t('logs.filters.all')">
            <el-option :label="t('display.logLevels.debug')" value="debug" />
            <el-option :label="t('display.logLevels.info')" value="info" />
            <el-option :label="t('display.logLevels.warn')" value="warn" />
            <el-option :label="t('display.logLevels.error')" value="error" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('logs.filters.source')">
          <el-input v-model="filters.source" :placeholder="t('logs.filters.sourcePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('logs.filters.plugin')">
          <el-input v-model="filters.pluginId" :placeholder="t('logs.filters.pluginPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('logs.filters.requestId')">
          <el-input v-model="filters.requestId" :placeholder="t('logs.filters.requestPlaceholder')" />
        </el-form-item>
      </el-form>

      <div class="logs-filter-actions">
        <el-button type="primary" @click="loadLogs()">{{ t('logs.filters.apply') }}</el-button>
      </div>
    </el-card>

    <RetryPanel
      v-if="error && items.length === 0"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading"
      @retry="loadLogs()"
    />

    <el-alert v-else-if="error" :title="t('errors.common.loadFailed')" type="error" :description="error" show-icon />

    <el-table
      v-else
      :data="items"
      style="width: 100%;"
      class="logs-data-table"
      :empty-text="t('display.empty')"
    >
      <el-table-column :label="t('logs.fields.timestamp')" width="200">
        <template #default="{ row }">
          <div class="log-cell-time">
            <div class="log-time-display">{{ formatDateTime(row.timestamp) }}</div>
            <small class="log-request-id">{{ row.request_id ?? t('display.empty') }}</small>
          </div>
        </template>
      </el-table-column>

      <el-table-column :label="t('logs.fields.level')" width="100">
        <template #default="{ row }">
          <el-tag size="small" :type="row.level === 'error' ? 'danger' : (row.level === 'warn' ? 'warning' : 'info')" effect="plain">
            {{ getLogLevelLabel(row.level) }}
          </el-tag>
        </template>
      </el-table-column>

      <el-table-column :label="t('logs.fields.source')" width="140">
        <template #default="{ row }">
          <div class="log-cell-source">
            <div class="log-source-text">{{ row.source }}</div>
            <small v-if="row.plugin_id" class="log-plugin-id">{{ row.plugin_id }}</small>
          </div>
        </template>
      </el-table-column>

      <el-table-column :label="t('logs.fields.message')" min-width="400">
        <template #default="{ row }">
          <p class="log-message-text" :title="row.message">{{ row.message }}</p>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<style lang="scss" scoped>
.logs-data-table {
  border-radius: 22px;
  overflow: hidden;
  box-shadow: 0 14px 32px rgba(18, 32, 38, 0.06);
  border: 1px solid rgba(22, 33, 39, 0.08);

  :deep(.el-table__inner-wrapper) {
    background: rgba(247, 250, 246, 0.88);
  }
  
  :deep(.el-table__header-wrapper th) {
    background-color: transparent !important;
    border-bottom: 1px solid rgba(22, 33, 39, 0.08);
    color: var(--muted);
    font-size: 0.85rem;
    font-weight: 600;
    padding: 16px 8px;
  }

  :deep(.el-table__row) {
    background-color: transparent;
    transition: background-color 150ms ease;
    
    td {
      border-bottom: 1px solid rgba(22, 33, 39, 0.04);
      padding: 12px 8px;
    }

    &:hover {
      background-color: rgba(255, 255, 255, 0.6);
      td {
        background-color: transparent !important;
      }
    }
  }

  :deep(.el-table__body-wrapper) {
    background-color: transparent;
  }
}

.log-cell-time {
  display: flex;
  flex-direction: column;
  gap: 4px;
  
  .log-time-display {
    font-size: 0.9rem;
    color: var(--text);
  }
  
  .log-request-id {
    font-family: "Cascadia Mono", "Consolas", monospace;
    font-size: 0.75rem;
    color: var(--muted);
  }
}

.log-cell-source {
  display: flex;
  flex-direction: column;
  gap: 4px;

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
}

.log-message-text {
  margin: 0;
  font-size: 0.9rem;
  line-height: 1.5;
  color: var(--text);
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
