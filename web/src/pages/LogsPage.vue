<script setup lang="ts">
import { onMounted } from 'vue'
import { storeToRefs } from 'pinia'

import RetryPanel from '@/components/RetryPanel.vue'
import { formatDateTime } from '@/lib/format'
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
  <div class="page-grid">
    <section class="hero-panel">
      <div>
        <div class="page-eyebrow">Logs</div>
        <h1>管理日志</h1>
        <p>查看历史日志与最新记录。</p>
      </div>

      <el-button :loading="loading" @click="loadLogs()">
        刷新日志
      </el-button>
    </section>

    <el-card>
      <el-form :inline="true" class="filter-form">
        <el-form-item label="Level">
          <el-select v-model="filters.level" clearable placeholder="all" style="width: 120px">
            <el-option label="debug" value="debug" />
            <el-option label="info" value="info" />
            <el-option label="warn" value="warn" />
            <el-option label="error" value="error" />
          </el-select>
        </el-form-item>
        <el-form-item label="Source">
          <el-input v-model="filters.source" placeholder="runtime / adapter.onebot11" />
        </el-form-item>
        <el-form-item label="Plugin">
          <el-input v-model="filters.pluginId" placeholder="weather" />
        </el-form-item>
        <el-form-item label="Request ID">
          <el-input v-model="filters.requestId" placeholder="req_*" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="loadLogs()">应用筛选</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <RetryPanel
      v-if="error && items.length === 0"
      title="日志读取失败"
      :description="error"
      :loading="loading"
      @retry="loadLogs()"
    />

    <el-alert v-else-if="error" title="日志读取失败" type="error" :description="error" show-icon />

    <el-table class="desktop-table" :data="items" stripe>
      <el-table-column label="Timestamp" min-width="180">
        <template #default="{ row }">{{ formatDateTime(row.timestamp) }}</template>
      </el-table-column>
      <el-table-column prop="level" label="Level" width="100" />
      <el-table-column prop="source" label="Source" min-width="180" />
      <el-table-column prop="plugin_id" label="Plugin" min-width="140" />
      <el-table-column prop="request_id" label="Request ID" min-width="180" />
      <el-table-column prop="message" label="Message" min-width="320" />
    </el-table>

    <div class="mobile-card-list">
      <el-card v-for="row in items" :key="[row.timestamp, row.source, row.message].join('|')" class="mobile-data-card">
        <div class="mobile-data-header">
          <strong>{{ row.source }}</strong>
          <el-tag size="small">{{ row.level }}</el-tag>
        </div>
        <div class="mobile-data-grid">
          <div><span>时间</span><strong>{{ formatDateTime(row.timestamp) }}</strong></div>
          <div><span>插件</span><strong>{{ row.plugin_id ?? '—' }}</strong></div>
          <div><span>请求</span><strong>{{ row.request_id ?? '—' }}</strong></div>
        </div>
        <p class="mobile-data-copy">{{ row.message }}</p>
      </el-card>
    </div>
  </div>
</template>
