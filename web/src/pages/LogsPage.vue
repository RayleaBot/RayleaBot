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

    <VirtualDataViewport
      :items="items"
      :item-height="130"
      :get-item-key="(row, index) => [row.timestamp, row.source, row.message, index].join('|')"
      :empty-label="t('display.empty')"
    >
      <template #default="{ item: row }">
        <article class="log-summary-row">
          <div class="log-summary-top">
            <div class="log-summary-primary">
              <strong>{{ formatDateTime(row.timestamp) }}</strong>
              <small>{{ row.request_id ?? t('display.empty') }}</small>
            </div>

            <div class="log-summary-tags">
              <el-tag size="small" effect="plain">{{ getLogLevelLabel(row.level) }}</el-tag>
              <el-tag size="small" effect="plain">{{ row.source }}</el-tag>
              <el-tag size="small" effect="plain">{{ row.plugin_id ?? t('display.empty') }}</el-tag>
            </div>
          </div>

          <div class="log-summary-bottom">
            <p class="summary-text-clamp" :title="row.message">{{ row.message }}</p>
            <small class="log-summary-request">{{ row.request_id ?? t('display.empty') }}</small>
          </div>
        </article>
      </template>
    </VirtualDataViewport>
  </div>
</template>
