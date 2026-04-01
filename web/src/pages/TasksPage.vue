<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import RetryPanel from '@/components/RetryPanel.vue'
import VirtualDataViewport from '@/components/VirtualDataViewport.vue'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatDateTime } from '@/lib/format'
import { getTaskStatusLabel, getTaskTypeLabel } from '@/lib/display'
import { t } from '@/i18n'
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
    ElMessage.error(getDisplayErrorMessage(error))
  }
}

async function cancelCurrent() {
  if (!currentTask.value) {
    return
  }

  try {
    await tasksStore.cancelTask(currentTask.value.task_id)
    ElMessage.success(t('tasks.cancelAccepted'))
  } catch (error) {
    ElMessage.error(getDisplayErrorMessage(error))
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
    return t('display.empty')
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
  <div class="page-grid page-grid--viewport">
    <section class="hero-panel">
      <div>
        <h1>{{ t('tasks.title') }}</h1>
      </div>

      <el-button :loading="loading" @click="loadTasks()">
        {{ t('tasks.refresh') }}
      </el-button>
    </section>

    <RetryPanel
      v-if="error && sortedItems.length === 0"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading"
      @retry="loadTasks()"
    />

    <el-alert v-else-if="error" :title="t('errors.common.loadFailed')" type="error" :description="error" show-icon />

    <VirtualDataViewport
      :items="sortedItems"
      :item-height="156"
      :viewport-height="620"
      :get-item-key="(row) => row.task_id"
      :empty-label="t('display.empty')"
    >
      <template #header>
        <div class="data-panel-header task-summary-head">
          <span>{{ t('tasks.fields.id') }}</span>
          <span>{{ t('tasks.fields.type') }}</span>
          <span>{{ t('tasks.fields.status') }}</span>
          <span>{{ t('tasks.fields.summary') }}</span>
        </div>
      </template>

      <template #default="{ item: row }">
        <article class="task-summary-row">
          <div class="mono-list">
            <strong>{{ row.task_id }}</strong>
            <small>{{ formatDateTime(row.started_at) }}</small>
          </div>

          <div class="task-summary-main">
            <strong>{{ getTaskTypeLabel(row.task_type) }}</strong>
            <small>{{ row.task_type }}</small>
          </div>

          <div class="task-summary-status">
            <el-tag size="small">{{ getTaskStatusLabel(row.status) }}</el-tag>
            <strong>{{ row.progress ?? t('display.empty') }}</strong>
          </div>

          <div class="task-summary-copy">
            <p>{{ row.summary }}</p>
            <el-button size="small" plain @click="inspect(row.task_id)">
              {{ t('tasks.actions.detail') }}
            </el-button>
          </div>
        </article>
      </template>
    </VirtualDataViewport>

    <el-drawer v-model="detailVisible" :title="t('tasks.detailTitle')" size="clamp(320px, 92vw, 720px)" :modal="false">
      <el-skeleton :loading="detailLoading" animated>
        <template v-if="currentTask">
          <el-descriptions :column="1" border>
            <el-descriptions-item :label="t('tasks.fields.id')">{{ currentTask.task_id }}</el-descriptions-item>
            <el-descriptions-item :label="t('tasks.fields.type')">
              {{ getTaskTypeLabel(currentTask.task_type) }}
              <small> · {{ currentTask.task_type }}</small>
            </el-descriptions-item>
            <el-descriptions-item :label="t('tasks.fields.status')">
              {{ getTaskStatusLabel(currentTask.status) }}
              <small> · {{ currentTask.status }}</small>
            </el-descriptions-item>
            <el-descriptions-item :label="t('tasks.fields.progress')">{{ currentTask.progress ?? t('display.empty') }}</el-descriptions-item>
            <el-descriptions-item :label="t('tasks.fields.summary')">{{ currentTask.summary }}</el-descriptions-item>
            <el-descriptions-item :label="t('tasks.fields.started')">{{ formatDateTime(currentTask.started_at) }}</el-descriptions-item>
            <el-descriptions-item :label="t('tasks.fields.finished')">{{ formatDateTime(currentTask.finished_at) }}</el-descriptions-item>
          </el-descriptions>

          <div v-if="currentTask.result" class="drawer-section">
            <div class="card-header">
              <span>{{ t('tasks.fields.result') }}</span>
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
              :alt="t('tasks.previewAlt')"
              class="task-preview-image"
            />
          </div>

          <div v-if="currentTask.error" class="drawer-section">
            <div class="card-header">
              <span>{{ t('tasks.fields.error') }}</span>
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
          {{ t('tasks.actions.cancel') }}
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
