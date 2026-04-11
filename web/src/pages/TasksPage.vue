<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import { notifyError, notifySuccess } from '@/adapter/feedback'
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
const hasRequestedTasks = ref(false)
let previewImageLoadVersion = 0

const tableColumns = computed(() => [
  { title: t('tasks.fields.type'), key: 'type', dataIndex: 'task_type', width: 220 },
  { title: t('tasks.fields.status'), key: 'status', dataIndex: 'status', width: 180 },
  { title: t('tasks.fields.started'), key: 'started', dataIndex: 'started_at', width: 220 },
  { title: t('tasks.fields.summary'), key: 'summary', dataIndex: 'summary' },
  { title: '', key: 'actions', dataIndex: 'actions', width: 120, fixed: 'right' as const },
])

async function loadTasks() {
  hasRequestedTasks.value = true
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
    notifyError(getDisplayErrorMessage(error))
  }
}

async function cancelCurrent() {
  if (!currentTask.value) {
    return
  }

  try {
    await tasksStore.cancelTask(currentTask.value.task_id)
    notifySuccess(t('tasks.cancelAccepted'))
  } catch (error) {
    notifyError(getDisplayErrorMessage(error))
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

function getStatusColor(status: string) {
  if (status === 'succeeded') return 'success'
  if (status === 'failed') return 'error'
  return 'processing'
}
</script>

<template>
  <div class="page-grid page-grid--viewport">
    <section class="hero-panel">
      <div>
        <h1>{{ t('tasks.title') }}</h1>
      </div>

      <a-button :loading="loading" @click="loadTasks()">
        {{ t('tasks.refresh') }}
      </a-button>
    </section>

    <RetryPanel
      v-if="error && sortedItems.length === 0"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading"
      @retry="loadTasks()"
    />

    <a-alert v-else-if="error" :message="t('errors.common.loadFailed')" type="error" :description="error" show-icon />

    <a-card v-else-if="(!hasRequestedTasks || loading) && sortedItems.length === 0" class="tasks-empty-card" :bordered="false">
      <a-skeleton active :paragraph="{ rows: 4 }" />
    </a-card>

    <a-card v-else-if="sortedItems.length === 0" class="tasks-empty-card" :bordered="false">
      <a-empty :description="t('display.empty')" />
    </a-card>

    <a-table
      v-else
      class="tasks-data-table"
      :columns="tableColumns"
      :data-source="sortedItems"
      :pagination="false"
      :row-key="(row) => row.task_id"
      :scroll="{ x: 980 }"
    >
      <template #emptyText>
        {{ t('display.empty') }}
      </template>

      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'type'">
          <div class="task-cell-identity">
            <strong class="task-type-label">{{ getTaskTypeLabel(record.task_type) }}</strong>
            <small class="task-type-id">{{ record.task_type }}</small>
          </div>
        </template>

        <template v-else-if="column.key === 'status'">
          <div class="task-cell-status">
            <a-tag :color="getStatusColor(record.status)">
              {{ getTaskStatusLabel(record.status) }}
            </a-tag>
            <strong v-if="record.progress !== undefined" class="task-progress">{{ record.progress }}%</strong>
          </div>
        </template>

        <template v-else-if="column.key === 'started'">
          <div class="task-cell-time">
            <div class="task-time-display">{{ formatDateTime(record.started_at) }}</div>
            <small class="task-id-mono">{{ record.task_id }}</small>
          </div>
        </template>

        <template v-else-if="column.key === 'summary'">
          <p class="task-summary-text" :title="record.summary">{{ record.summary }}</p>
        </template>

        <template v-else-if="column.key === 'actions'">
          <a-button size="small" @click="inspect(record.task_id)">
            {{ t('tasks.actions.detail') }}
          </a-button>
        </template>
      </template>
    </a-table>

    <a-drawer
      v-model:open="detailVisible"
      :get-container="false"
      :title="t('tasks.detailTitle')"
      placement="right"
      width="min(720px, 92vw)"
    >
      <a-skeleton :loading="detailLoading" active>
        <template v-if="currentTask">
          <a-descriptions :column="1" bordered size="small">
            <a-descriptions-item :label="t('tasks.fields.id')">{{ currentTask.task_id }}</a-descriptions-item>
            <a-descriptions-item :label="t('tasks.fields.type')">
              {{ getTaskTypeLabel(currentTask.task_type) }}
              <small> · {{ currentTask.task_type }}</small>
            </a-descriptions-item>
            <a-descriptions-item :label="t('tasks.fields.status')">
              {{ getTaskStatusLabel(currentTask.status) }}
              <small> · {{ currentTask.status }}</small>
            </a-descriptions-item>
            <a-descriptions-item :label="t('tasks.fields.progress')">{{ currentTask.progress ?? t('display.empty') }}</a-descriptions-item>
            <a-descriptions-item :label="t('tasks.fields.summary')">{{ currentTask.summary }}</a-descriptions-item>
            <a-descriptions-item :label="t('tasks.fields.started')">{{ formatDateTime(currentTask.started_at) }}</a-descriptions-item>
            <a-descriptions-item :label="t('tasks.fields.finished')">{{ formatDateTime(currentTask.finished_at) }}</a-descriptions-item>
          </a-descriptions>

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
            <a-alert
              :message="currentTask.error.code"
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

          <div class="drawer-section drawer-actions">
            <a-button danger :loading="cancelPending" @click="cancelCurrent">
              {{ t('tasks.actions.cancel') }}
            </a-button>
          </div>
        </template>
      </a-skeleton>
    </a-drawer>
  </div>
</template>

<style lang="scss" scoped>
.tasks-empty-card {
  border-radius: 22px;
  border: 1px solid rgba(22, 33, 39, 0.08);
  box-shadow: 0 14px 32px rgba(18, 32, 38, 0.06);
  background: rgba(247, 250, 246, 0.88);
}

.tasks-data-table {
  border-radius: 22px;
  overflow: hidden;
  box-shadow: 0 14px 32px rgba(18, 32, 38, 0.06);
  border: 1px solid rgba(22, 33, 39, 0.08);
}

.task-cell-identity {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

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

.task-cell-status {
  display: flex;
  align-items: center;
  gap: 10px;
}

.task-progress {
  font-size: 0.9rem;
  color: var(--text);
}

.task-cell-time {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.task-time-display {
  font-size: 0.9rem;
  color: var(--text);
}

.task-id-mono {
  font-family: "Cascadia Mono", "Consolas", monospace;
  font-size: 0.75rem;
  color: var(--muted);
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

.drawer-actions {
  display: flex;
  justify-content: flex-end;
}

.task-preview-image {
  display: block;
  width: 100%;
  margin-top: 16px;
  border-radius: 16px;
  border: 1px solid rgba(22, 33, 39, 0.08);
  background: rgba(247, 250, 246, 0.88);
}
</style>
