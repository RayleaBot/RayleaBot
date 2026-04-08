<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import RecoverySummaryDetails from '@/components/RecoverySummaryDetails.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { getRecoveryStatusLabel, getTaskStatusLabel, getTaskTypeLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import { apiDownload } from '@/lib/http'
import { t } from '@/i18n'
import type { RecoveryCompatibilitySummary, TaskSummary } from '@/types/api'
import { useTasksStore } from '@/stores/tasks'

const route = useRoute()
const router = useRouter()
const tasksStore = useTasksStore()
const { cancelPending, currentTask, detailLoading, error, loading, sortedItems } = storeToRefs(tasksStore)
const detailVisible = ref(false)
const previewImageSrc = ref('')
let previewImageLoadVersion = 0

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

  resetPreviewImage()
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

onBeforeUnmount(() => {
  resetPreviewImage()
})

function taskDetailEntries(details?: Record<string, unknown>) {
  return Object.entries(details ?? {}).filter(([key]) => key !== 'recovery_summary')
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

function isRecoverySummary(value: unknown): value is RecoveryCompatibilitySummary {
  return Boolean(
    value
    && typeof value === 'object'
    && !Array.isArray(value)
    && typeof (value as RecoveryCompatibilitySummary).status === 'string'
    && typeof (value as RecoveryCompatibilitySummary).phase === 'string'
    && typeof (value as RecoveryCompatibilitySummary).operation === 'string',
  )
}

const taskRecoverySummary = computed<RecoveryCompatibilitySummary | null>(() => {
  const recoverySummary = currentTask.value?.result?.details?.recovery_summary
  return isRecoverySummary(recoverySummary) ? recoverySummary : null
})

const taskRecoveryStatusLabel = computed(() => getRecoveryStatusLabel(taskRecoverySummary.value?.status))

async function openRecoveryPlugin(pluginId: string) {
  await router.push({ name: 'plugin-detail', params: { id: pluginId } })
}

function resetPreviewImage() {
  if (!previewImageSrc.value) {
    return
  }

  window.URL.revokeObjectURL(previewImageSrc.value)
  previewImageSrc.value = ''
}

watch(
  [detailVisible, () => previewImageUrl(currentTask.value)],
  async ([visible, imageUrl]) => {
    const requestVersion = ++previewImageLoadVersion

    if (!visible || !imageUrl) {
      resetPreviewImage()
      return
    }

    try {
      const { blob } = await apiDownload(imageUrl)
      if (requestVersion !== previewImageLoadVersion) {
        return
      }

      resetPreviewImage()
      previewImageSrc.value = window.URL.createObjectURL(blob)
    } catch {
      if (requestVersion !== previewImageLoadVersion) {
        return
      }
      resetPreviewImage()
    }
  },
  { immediate: true },
)
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

    <el-table
      v-else
      :data="sortedItems"
      style="width: 100%;"
      class="tasks-data-table"
      :empty-text="t('display.empty')"
    >
      <el-table-column :label="t('tasks.fields.type')" min-width="180">
        <template #default="{ row }">
          <div class="task-cell-identity">
            <strong class="task-type-label">{{ getTaskTypeLabel(row.task_type) }}</strong>
            <small class="task-type-id">{{ row.task_type }}</small>
          </div>
        </template>
      </el-table-column>

      <el-table-column :label="t('tasks.fields.status')" min-width="160">
        <template #default="{ row }">
          <div class="task-cell-status">
            <el-tag size="small" :type="row.status === 'succeeded' ? 'success' : (row.status === 'failed' ? 'danger' : 'info')" effect="light">
              {{ getTaskStatusLabel(row.status) }}
            </el-tag>
            <strong v-if="row.progress !== undefined" class="task-progress">{{ row.progress }}%</strong>
          </div>
        </template>
      </el-table-column>

      <el-table-column :label="t('tasks.fields.started')" min-width="200">
        <template #default="{ row }">
          <div class="task-cell-time">
            <div class="task-time-display">{{ formatDateTime(row.started_at) }}</div>
            <small class="task-id-mono">{{ row.task_id }}</small>
          </div>
        </template>
      </el-table-column>

      <el-table-column :label="t('tasks.fields.summary')" min-width="300">
        <template #default="{ row }">
          <p class="task-summary-text" :title="row.summary">{{ row.summary }}</p>
        </template>
      </el-table-column>

      <el-table-column fixed="right" width="120" align="right">
        <template #default="{ row }">
          <el-button size="small" plain @click="inspect(row.task_id)">
            {{ t('tasks.actions.detail') }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>

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
              v-if="previewImageSrc"
              :src="previewImageSrc"
              :alt="t('tasks.previewAlt')"
              class="task-preview-image"
            />
          </div>

          <div v-if="taskRecoverySummary" class="drawer-section">
            <div class="card-header">
              <span>{{ t('tasks.recoverySummary') }}</span>
            </div>
            <RecoverySummaryDetails
              :recovery-summary="taskRecoverySummary"
              :recovery-status-label="taskRecoveryStatusLabel"
              show-plugin-links
              @open-plugin="openRecoveryPlugin"
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

<style lang="scss" scoped>
.tasks-data-table {
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

.task-cell-identity {
  display: flex;
  flex-direction: column;
  gap: 4px;
  
  .task-type-label {
    font-size: 0.98rem;
    color: var(--text);
    font-weight: 600;
  }
  
  .task-type-id {
    font-family: "Cascadia Mono", "Consolas", monospace;
    font-size: 0.8rem;
    color: var(--muted);
  }
}

.task-cell-status {
  display: flex;
  align-items: center;
  gap: 10px;

  .task-progress {
    font-size: 0.9rem;
    color: var(--text);
  }
}

.task-cell-time {
  display: flex;
  flex-direction: column;
  gap: 4px;

  .task-time-display {
    font-size: 0.9rem;
    color: var(--text);
  }

  .task-id-mono {
    font-family: "Cascadia Mono", "Consolas", monospace;
    font-size: 0.75rem;
    color: var(--muted);
  }
}

.task-summary-text {
  margin: 0;
  font-size: 0.9rem;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 100%;
}

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
