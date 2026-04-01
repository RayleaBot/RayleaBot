<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import RetryPanel from '@/components/RetryPanel.vue'
import { formatDateTime } from '@/lib/format'
import type { TaskSummary } from '@/types/api'
import { useTasksStore } from '@/stores/tasks'

const route = useRoute()
const router = useRouter()
const tasksStore = useTasksStore()
const { cancelPending, currentTask, detailLoading, error, loading, sortedItems } = storeToRefs(tasksStore)
const detailVisible = ref(false)

async function loadTasks() {
  try {
    await tasksStore.fetchList()
  } catch {
    // store error state drives the page
  }
}

async function inspect(taskId: string) {
  try {
    await tasksStore.fetchDetail(taskId)
    detailVisible.value = true
    await router.replace({ name: 'tasks', query: { ...route.query, task_id: taskId } })
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : 'task detail load failed')
  }
}

async function cancelCurrent() {
  if (!currentTask.value) {
    return
  }

  try {
    await tasksStore.cancelTask(currentTask.value.task_id)
    ElMessage.success('取消请求已发送')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : 'cancel failed')
  }
}

watch(detailVisible, async (visible) => {
  if (visible) {
    return
  }

  tasksStore.clearCurrentTask()
  if (route.query.task_id) {
    await router.replace({ name: 'tasks', query: { ...route.query, task_id: undefined } })
  }
})

watch(
  () => route.query.task_id,
  async (taskId) => {
    if (typeof taskId === 'string' && taskId) {
      await inspect(taskId)
    } else {
      detailVisible.value = false
    }
  },
  { immediate: true },
)

onMounted(() => {
  void loadTasks()
})

function taskDetailEntries(details?: Record<string, unknown>) {
  return Object.entries(details ?? {})
}

function formatTaskDetailValue(value: unknown) {
  if (value === null || value === undefined || value === '') {
    return '—'
  }
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
    return String(value)
  }
  return JSON.stringify(value)
}

function previewImageUrl(task: TaskSummary | null) {
  const imageUrl = task?.result?.details?.image_url
  return typeof imageUrl === 'string' && imageUrl ? imageUrl : ''
}
</script>

<template>
  <div class="page-grid">
    <section class="hero-panel">
      <div>
        <div class="page-eyebrow">Tasks</div>
        <h1>后台任务</h1>
        <p>查看后台任务列表与最新状态。</p>
      </div>

      <el-button :loading="loading" @click="loadTasks()">
        刷新任务
      </el-button>
    </section>

    <RetryPanel
      v-if="error && sortedItems.length === 0"
      title="任务列表读取失败"
      :description="error"
      :loading="loading"
      @retry="loadTasks()"
    />

    <el-alert v-else-if="error" title="任务列表读取失败" type="error" :description="error" show-icon />

    <el-table class="desktop-table" :data="sortedItems" stripe @row-click="(row) => inspect(row.task_id)">
      <el-table-column prop="task_id" label="Task ID" min-width="220" />
      <el-table-column prop="task_type" label="Type" min-width="150" />
      <el-table-column prop="status" label="Status" width="130" />
      <el-table-column prop="progress" label="Progress" width="120" />
      <el-table-column prop="summary" label="Summary" min-width="260" />
    </el-table>

    <div class="mobile-card-list">
      <el-card
        v-for="row in sortedItems"
        :key="row.task_id"
        class="mobile-data-card"
        @click="inspect(row.task_id)"
      >
        <div class="mobile-data-header">
          <strong>{{ row.task_type }}</strong>
          <el-tag size="small">{{ row.status }}</el-tag>
        </div>
        <div class="mobile-data-grid">
          <div><span>任务</span><strong>{{ row.task_id }}</strong></div>
          <div><span>进度</span><strong>{{ row.progress ?? '—' }}</strong></div>
        </div>
        <p class="mobile-data-copy">{{ row.summary }}</p>
      </el-card>
    </div>

    <el-drawer v-model="detailVisible" title="任务详情" size="clamp(320px, 92vw, 720px)" :modal="false">
      <el-skeleton :loading="detailLoading" animated>
        <template v-if="currentTask">
          <el-descriptions :column="1" border>
            <el-descriptions-item label="Task ID">{{ currentTask.task_id }}</el-descriptions-item>
            <el-descriptions-item label="Type">{{ currentTask.task_type }}</el-descriptions-item>
            <el-descriptions-item label="Status">{{ currentTask.status }}</el-descriptions-item>
            <el-descriptions-item label="Progress">{{ currentTask.progress ?? '—' }}</el-descriptions-item>
            <el-descriptions-item label="Summary">{{ currentTask.summary }}</el-descriptions-item>
            <el-descriptions-item label="Started">{{ formatDateTime(currentTask.started_at) }}</el-descriptions-item>
            <el-descriptions-item label="Finished">{{ formatDateTime(currentTask.finished_at) }}</el-descriptions-item>
          </el-descriptions>

          <div v-if="currentTask.result" class="drawer-section">
            <div class="card-header">
              <span>Result</span>
            </div>
            <p class="mobile-data-copy">{{ currentTask.result.summary }}</p>
            <div v-if="taskDetailEntries(currentTask.result.details).length" class="mono-list">
              <div v-for="[key, value] in taskDetailEntries(currentTask.result.details)" :key="key">
                {{ key }} = {{ formatTaskDetailValue(value) }}
              </div>
            </div>
            <img
              v-if="previewImageUrl(currentTask)"
              :src="previewImageUrl(currentTask)"
              alt="render preview"
              class="task-preview-image"
            />
          </div>

          <div v-if="currentTask.error" class="drawer-section">
            <div class="card-header">
              <span>Error</span>
            </div>
            <el-alert
              :title="currentTask.error.code"
              type="error"
              :description="currentTask.error.message"
              show-icon
            />
            <div v-if="taskDetailEntries(currentTask.error.details).length" class="mono-list">
              <div v-for="[key, value] in taskDetailEntries(currentTask.error.details)" :key="key">
                {{ key }} = {{ formatTaskDetailValue(value) }}
              </div>
            </div>
          </div>
        </template>
      </el-skeleton>

      <template #footer>
        <el-button type="danger" plain :loading="cancelPending" @click="cancelCurrent">
          请求取消
        </el-button>
      </template>
    </el-drawer>
  </div>
</template>

<style scoped>
.drawer-section {
  margin-top: 20px;
}

.task-preview-image {
  display: block;
  width: 100%;
  margin-top: 16px;
  border-radius: 16px;
  border: 1px solid var(--el-border-color);
  background: var(--el-bg-color-page);
}
</style>
