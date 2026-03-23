<script setup lang="ts">
import { onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'

import { usePluginsStore } from '@/stores/plugins'

const router = useRouter()
const pluginsStore = usePluginsStore()
const { actionPending, error, loading, sortedItems } = storeToRefs(pluginsStore)

onMounted(() => {
  void pluginsStore.fetchList()
})

function openDetail(id: string) {
  void router.push({ name: 'plugin-detail', params: { id } })
}
</script>

<template>
  <div class="page-grid">
    <section class="hero-panel">
      <div>
        <div class="page-eyebrow">Plugins</div>
        <h1>插件主流程</h1>
        <p>当前覆盖列表、详情与 `enable / disable / reload`，install / grants 后置。</p>
      </div>

      <el-button :loading="loading" @click="pluginsStore.fetchList()">
        刷新列表
      </el-button>
    </section>

    <el-alert v-if="error" title="插件列表读取失败" type="error" :description="error" show-icon />

    <el-table :data="sortedItems" stripe @row-click="(row) => openDetail(row.id)">
      <el-table-column prop="id" label="Plugin ID" min-width="180" />
      <el-table-column prop="registration_state" label="Registration" width="140" />
      <el-table-column prop="desired_state" label="Desired" width="140" />
      <el-table-column prop="runtime_state" label="Runtime" width="140" />
      <el-table-column prop="display_state" label="Display" width="160" />
      <el-table-column label="Actions" min-width="260">
        <template #default="{ row }">
          <div class="table-actions">
            <el-button size="small" type="success" :loading="actionPending[row.id] === 'enable'" @click.stop="pluginsStore.executeAction(row.id, 'enable')">
              Enable
            </el-button>
            <el-button size="small" type="warning" :loading="actionPending[row.id] === 'reload'" @click.stop="pluginsStore.executeAction(row.id, 'reload')">
              Reload
            </el-button>
            <el-button size="small" type="danger" plain :loading="actionPending[row.id] === 'disable'" @click.stop="pluginsStore.executeAction(row.id, 'disable')">
              Disable
            </el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>
