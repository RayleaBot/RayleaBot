<script setup lang="ts">
import { computed, onMounted, onUnmounted, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { storeToRefs } from 'pinia'
import { useRoute } from 'vue-router'

import { formatDateTime } from '@/lib/format'
import { usePluginsStore } from '@/stores/plugins'
import { useSocketStore } from '@/stores/sockets'

const route = useRoute()
const pluginsStore = usePluginsStore()
const socketStore = useSocketStore()

const { current } = storeToRefs(pluginsStore)

const pluginId = computed(() => String(route.params.id))
const consoleFrames = computed(() => pluginsStore.getConsole(pluginId.value))

async function loadDetail() {
  await pluginsStore.fetchDetail(pluginId.value)
  socketStore.setConsolePlugin(pluginId.value)
}

async function runAction(action: 'enable' | 'disable' | 'reload') {
  await pluginsStore.executeAction(pluginId.value, action)
  ElMessage.success(`${pluginId.value} ${action} accepted`)
}

watch(pluginId, () => {
  void loadDetail()
})

onMounted(() => {
  void loadDetail()
})

onUnmounted(() => {
  socketStore.setConsolePlugin(null)
})
</script>

<template>
  <div class="page-grid">
    <section class="hero-panel">
      <div>
        <div class="page-eyebrow">Plugin Detail</div>
        <h1>{{ pluginId }}</h1>
        <p>最小状态视图与 lifecycle 操作，console 走按需 WebSocket。</p>
      </div>

      <div class="table-actions">
        <el-button type="success" @click="runAction('enable')">Enable</el-button>
        <el-button type="warning" @click="runAction('reload')">Reload</el-button>
        <el-button type="danger" plain @click="runAction('disable')">Disable</el-button>
      </div>
    </section>

    <div class="content-grid">
      <el-card>
        <template #header>
          <div class="card-header">
            <span>Current Snapshot</span>
          </div>
        </template>

        <el-descriptions :column="1" border>
          <el-descriptions-item label="Registration">{{ current?.registration_state ?? '—' }}</el-descriptions-item>
          <el-descriptions-item label="Desired">{{ current?.desired_state ?? '—' }}</el-descriptions-item>
          <el-descriptions-item label="Runtime">{{ current?.runtime_state ?? '—' }}</el-descriptions-item>
          <el-descriptions-item label="Display">{{ current?.display_state ?? '—' }}</el-descriptions-item>
        </el-descriptions>
      </el-card>

      <el-card>
        <template #header>
          <div class="card-header">
            <span>Console</span>
            <el-tag size="small">{{ socketStore.snapshots.pluginConsole.status }}</el-tag>
          </div>
        </template>

        <el-empty v-if="consoleFrames.length === 0" description="等待 console 输出" />

        <div v-else class="console-feed">
          <div v-for="frame in consoleFrames" :key="`${frame.timestamp}-${frame.text}`" class="console-line">
            <span class="console-meta">{{ formatDateTime(frame.timestamp) }} · {{ frame.stream }}</span>
            <pre>{{ frame.text }}</pre>
          </div>
        </div>
      </el-card>
    </div>
  </div>
</template>
