<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import AppEmptyState from '@/components/AppEmptyState.vue'
import ManagementContextActions from '@/components/ManagementContextActions.vue'
import AppPage from '@/components/page/AppPage.vue'
import RecoverySummaryDetails from '@/components/RecoverySummaryDetails.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { getRecoveryStatusLabel, getTaskStatusLabel, getTaskTypeLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import { apiDownload } from '@/lib/http'
import { buildPluginDetailLocation, buildTaskContextActions, buildTaskLocation } from '@/lib/management-links'
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
let previewWatcherActive = true

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
    await router.replace(buildTaskLocation(taskId))
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
  if (route.name === 'tasks' && route.query.task_id) {
    await router.replace(buildTaskLocation(null))
  }
})

watch(
  () => route.query.task_id,
  async (taskId) => {
    if (route.name !== 'tasks') {
      return
    }

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
  previewWatcherActive = false
  previewImageLoadVersion += 1
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
const currentTaskActions = computed(() => (
  currentTask.value ? buildTaskContextActions(currentTask.value) : []
))

async function openRecoveryPlugin(pluginId: string) {
  await router.push(buildPluginDetailLocation(pluginId))
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
  async ([visible, imageUrl], _, onCleanup) => {
    const requestVersion = ++previewImageLoadVersion
    let cancelled = false
    const controller = new AbortController()

    onCleanup(() => {
      cancelled = true
      controller.abort()
    })

    if (!visible || !imageUrl) {
      resetPreviewImage()
      return
    }

    try {
      const { blob } = await apiDownload(imageUrl, { signal: controller.signal })
      if (cancelled || !previewWatcherActive || requestVersion !== previewImageLoadVersion) {
        return
      }

      const nextPreviewUrl = window.URL.createObjectURL(blob)
      if (cancelled || !previewWatcherActive || requestVersion !== previewImageLoadVersion) {
        window.URL.revokeObjectURL(nextPreviewUrl)
        return
      }

      resetPreviewImage()
      previewImageSrc.value = nextPreviewUrl
    } catch {
      if (cancelled || requestVersion !== previewImageLoadVersion) {
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
  <AppPage :title="t('tasks.title')" full-height>
    <template #extra>
      <a-button :loading="loading" :aria-label="t('tasks.refresh')" @click="loadTasks()">
        {{ t('tasks.refresh') }}
      </a-button>
    </template>

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

    <AppEmptyState
      v-else-if="sortedItems.length === 0"
      icon="box"
      :title="t('tasks.empty.title')"
      :description="t('tasks.empty.description')"
    />

    <a-table
      v-else
      class="tasks-data-table app-data-table"
      v-motion="{ initial: { opacity: 0, y: 12 }, enter: { opacity: 1, y: 0, transition: { duration: 300, delay: 0 } } }"
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
            <a-tag
              size="small"
              :color="getStatusColor(record.status)"
              :aria-label="t('tasks.statusAriaLabel', { status: getTaskStatusLabel(record.status) })"
            >
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
          <a-button size="small" :aria-label="`${t('tasks.actions.detail')} ${record.task_id}`" @click="inspect(record.task_id)">
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

          <div v-if="currentTaskActions.length" class="drawer-section">
            <div class="card-header">
              <span>{{ t('tasks.actions.related') }}</span>
            </div>
            <ManagementContextActions :actions="currentTaskActions" />
          </div>

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
            <a-button danger :loading="cancelPending" :aria-label="t('tasks.actions.cancel')" @click="cancelCurrent">
              {{ t('tasks.actions.cancel') }}
            </a-button>
          </div>
        </template>
      </a-skeleton>
    </a-drawer>
  </AppPage>
</template>

<style lang="scss" scoped>
.tasks-empty-card {
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-xs);
}

.tasks-data-table {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-xs);
}

.tasks-data-table :deep(.ant-spin-nested-loading),
.tasks-data-table :deep(.ant-spin-container) {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
}

.tasks-data-table :deep(.ant-table) {
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}

.tasks-data-table :deep(.ant-table-container) {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.tasks-data-table :deep(.ant-table-content) {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto !important;
}

.tasks-data-table :deep(.ant-table-body) {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
}

:deep(.ant-table-row:hover > td) {
  background: var(--surface-accent) !important;
}

:deep(.ant-tag-success) {
  color: #1f7a43;
  border-color: #1f7a43;
}

:deep(.ant-tag-error) {
  color: #b4232d;
  border-color: #b4232d;
}

:deep(.ant-tag-processing) {
  color: #8a5a00;
  border-color: #8a5a00;
}

.mono-list {
  font-family: var(--font-mono);
  color: var(--text);
}

.mobile-data-copy {
  font-family: var(--font-mono);
  color: var(--text);
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
  font-family: var(--font-mono);
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
  font-family: var(--font-mono);
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
  background: var(--surface);
  border-radius: var(--radius-md);
  padding: 16px;
  box-shadow: var(--shadow-card);
  border: 1px solid var(--border);
}

.drawer-actions {
  display: flex;
  justify-content: flex-end;
}

.task-preview-image {
  display: block;
  width: 100%;
  margin-top: 16px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: var(--surface-soft);
}
</style>
