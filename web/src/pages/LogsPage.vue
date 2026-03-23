<script setup lang="ts">
import { onMounted } from 'vue'
import { storeToRefs } from 'pinia'

import { formatDateTime } from '@/lib/format'
import { useLogsStore } from '@/stores/logs'

const logsStore = useLogsStore()
const { error, filters, items, loading } = storeToRefs(logsStore)

onMounted(() => {
  void logsStore.fetchList()
})
</script>

<template>
  <div class="page-grid">
    <section class="hero-panel">
      <div>
        <div class="page-eyebrow">Logs</div>
        <h1>管理日志</h1>
        <p>先回放 `/api/logs`，再接 `/ws/logs` 追加。</p>
      </div>

      <el-button :loading="loading" @click="logsStore.fetchList()">
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
          <el-button type="primary" @click="logsStore.fetchList()">应用筛选</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-alert v-if="error" title="日志读取失败" type="error" :description="error" show-icon />

    <el-table :data="items" stripe>
      <el-table-column label="Timestamp" min-width="180">
        <template #default="{ row }">{{ formatDateTime(row.timestamp) }}</template>
      </el-table-column>
      <el-table-column prop="level" label="Level" width="100" />
      <el-table-column prop="source" label="Source" min-width="180" />
      <el-table-column prop="plugin_id" label="Plugin" min-width="140" />
      <el-table-column prop="request_id" label="Request ID" min-width="180" />
      <el-table-column prop="message" label="Message" min-width="320" />
    </el-table>
  </div>
</template>
