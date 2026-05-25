<script setup lang="ts">
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
  ExclamationCircleOutlined,
  EyeOutlined,
  ReloadOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons-vue'
import { computed, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'

import { notifyError, notifySuccess } from '@/adapter/feedback'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatDateTime } from '@/lib/format'
import { t } from '@/i18n'
import { useSchedulerJobsStore } from '@/stores/scheduler-jobs'
import type { SchedulerJobRunStats, SchedulerJobSummary } from '@/types/api'

const schedulerStore = useSchedulerJobsStore()
const { error, loading, sortedItems, triggeringJobId } = storeToRefs(schedulerStore)
const detailVisible = ref(false)
const currentJob = ref<SchedulerJobSummary | null>(null)

const tableColumns = computed(() => [
  { title: t('scheduler.fields.plugin'), key: 'plugin', dataIndex: 'plugin_name', width: 220 },
  { title: t('scheduler.fields.task'), key: 'task', dataIndex: 'task_name', width: 220 },
  { title: t('scheduler.fields.conversation'), key: 'conversation', dataIndex: 'payload_summary', width: 180 },
  { title: t('scheduler.fields.label'), key: 'label', dataIndex: 'log_label', width: 180 },
  { title: t('scheduler.fields.cron'), key: 'cron', dataIndex: 'cron_expr', width: 180 },
  { title: t('scheduler.fields.lastRun'), key: 'lastRun', dataIndex: 'last_run', width: 180 },
  { title: t('scheduler.fields.duration'), key: 'duration', dataIndex: 'last_duration_ms', width: 100 },
  { title: t('scheduler.fields.nextRun'), key: 'nextRun', dataIndex: 'next_run', width: 180 },
  { title: t('scheduler.fields.lastError'), key: 'lastError', dataIndex: 'last_error', width: 260 },
  { title: t('scheduler.fields.stats'), key: 'stats', dataIndex: 'stats', width: 260 },
  { title: t('scheduler.fields.actions'), key: 'actions', dataIndex: 'actions', width: 210, fixed: 'right' as const },
])

async function loadSchedulerJobs() {
  try {
    await schedulerStore.fetchList()
  } catch {
    // store error state drives the page
  }
}

async function triggerJob(job: SchedulerJobSummary) {
  try {
    await schedulerStore.trigger(job.job_id)
    notifySuccess(t('scheduler.triggerAccepted'))
  } catch (err) {
    notifyError(getDisplayErrorMessage(err))
  }
}

function showJobDetail(job: SchedulerJobSummary) {
  currentJob.value = job
  detailVisible.value = true
}

onMounted(() => {
  void loadSchedulerJobs()
})

function formatDurationMs(value: number) {
  if (!Number.isFinite(value) || value <= 0) {
    return t('display.empty')
  }
  if (value < 1000) {
    return `${value} ms`
  }
  return `${(value / 1000).toFixed(value < 10_000 ? 1 : 0)} s`
}

function displayText(value?: string | null) {
  return value?.trim() || t('display.empty')
}

function conversationText(job: SchedulerJobSummary) {
  const payload = job.payload_summary
  if (payload.conversation_id) {
    return payload.conversation_id
  }
  if (payload.target_type && payload.target_id) {
    return `${payload.target_type}:${payload.target_id}`
  }
  return ''
}

function statsItems(stats: SchedulerJobRunStats) {
  return [
    { key: 'success', icon: CheckCircleOutlined, value: stats.success, className: 'is-success', label: t('scheduler.stats.success', { count: stats.success }), aria: t('scheduler.aria.success') },
    { key: 'failed', icon: CloseCircleOutlined, value: stats.failed, className: 'is-failed', label: t('scheduler.stats.failed', { count: stats.failed }), aria: t('scheduler.aria.failed') },
    { key: 'timeout', icon: ClockCircleOutlined, value: stats.timeout, className: 'is-timeout', label: t('scheduler.stats.timeout', { count: stats.timeout }), aria: t('scheduler.aria.timeout') },
    { key: 'retry', icon: ReloadOutlined, value: stats.retry, className: 'is-retry', label: t('scheduler.stats.retry', { count: stats.retry }), aria: t('scheduler.aria.retry') },
    { key: 'other', icon: ExclamationCircleOutlined, value: stats.other, className: 'is-other', label: t('scheduler.stats.other', { count: stats.other }), aria: t('scheduler.aria.other') },
  ].filter((item) => item.value > 0 || item.key === 'success')
}
</script>

<template>
  <AppPage :title="t('scheduler.title')" full-height>
    <template #extra>
      <a-button :loading="loading" :aria-label="t('scheduler.refresh')" @click="loadSchedulerJobs">
        <template #icon>
          <ReloadOutlined />
        </template>
        {{ t('scheduler.refresh') }}
      </a-button>
    </template>

    <RetryPanel
      v-if="error && sortedItems.length === 0"
      :title="t('errors.common.loadFailed')"
      :description="error"
      :loading="loading"
      @retry="loadSchedulerJobs"
    />

    <a-card v-else-if="loading && sortedItems.length === 0" class="scheduler-empty-card" :bordered="false">
      <a-skeleton active :paragraph="{ rows: 4 }" />
    </a-card>

    <AppEmptyState
      v-else-if="sortedItems.length === 0"
      icon="box"
      :title="t('scheduler.empty.title')"
      :description="t('scheduler.empty.description')"
    />

    <a-table
      v-else
      class="scheduler-data-table app-data-table"
      :columns="tableColumns"
      :data-source="sortedItems"
      :pagination="false"
      :row-key="(row) => row.job_id"
      :scroll="{ x: 2060 }"
    >
      <template #emptyText>
        {{ t('display.empty') }}
      </template>

      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'plugin'">
          <div class="scheduler-cell-identity">
            <strong>{{ record.plugin_name }}</strong>
            <small>{{ record.plugin_id }}</small>
          </div>
        </template>

        <template v-else-if="column.key === 'task'">
          <div class="scheduler-cell-identity">
            <strong>{{ record.task_name }}</strong>
            <small>{{ record.job_id }}</small>
          </div>
        </template>

        <template v-else-if="column.key === 'conversation'">
          <span class="scheduler-mono">{{ displayText(conversationText(record)) }}</span>
        </template>

        <template v-else-if="column.key === 'label'">
          <div class="scheduler-cell-copy">
            <span>{{ displayText(record.log_label || record.payload_summary.content) }}</span>
          </div>
        </template>

        <template v-else-if="column.key === 'cron'">
          <div class="scheduler-cell-identity">
            <span class="scheduler-mono">{{ record.cron_expr }}</span>
            <small>{{ record.timezone }}</small>
          </div>
        </template>

        <template v-else-if="column.key === 'lastRun'">
          {{ formatDateTime(record.last_run) }}
        </template>

        <template v-else-if="column.key === 'duration'">
          {{ formatDurationMs(record.last_duration_ms) }}
        </template>

        <template v-else-if="column.key === 'nextRun'">
          {{ formatDateTime(record.next_run) }}
        </template>

        <template v-else-if="column.key === 'lastError'">
          <a-alert
            v-if="record.last_error"
            class="scheduler-error-alert"
            type="error"
            :message="record.last_error.code"
            :description="record.last_error.message"
            show-icon
          />
          <span v-else>{{ t('display.empty') }}</span>
        </template>

        <template v-else-if="column.key === 'stats'">
          <div class="scheduler-stats">
            <span class="scheduler-stats__total">{{ t('scheduler.stats.total', { count: record.stats.total }) }}</span>
            <span
              v-for="item in statsItems(record.stats)"
              :key="item.key"
              class="scheduler-stats__item"
              :class="item.className"
              :aria-label="`${item.aria}: ${item.value}`"
              :title="item.label"
            >
              <component :is="item.icon" />
              <span>{{ item.value }}</span>
            </span>
          </div>
        </template>

        <template v-else-if="column.key === 'actions'">
          <div class="scheduler-actions">
            <a-button size="small" :aria-label="`${t('scheduler.view')} ${record.job_id}`" @click="showJobDetail(record)">
              <template #icon>
                <EyeOutlined />
              </template>
              {{ t('scheduler.view') }}
            </a-button>
            <a-button
              size="small"
              :loading="triggeringJobId === record.job_id"
              :aria-label="`${t('scheduler.trigger')} ${record.job_id}`"
              @click="triggerJob(record)"
            >
              <template #icon>
                <ThunderboltOutlined />
              </template>
              {{ t('scheduler.trigger') }}
            </a-button>
          </div>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="detailVisible"
      :title="t('scheduler.detailTitle')"
      :footer="null"
      width="720px"
    >
      <a-descriptions v-if="currentJob" :column="1" bordered size="small">
        <a-descriptions-item :label="t('scheduler.fields.plugin')">
          {{ currentJob.plugin_name }} / {{ currentJob.plugin_id }}
        </a-descriptions-item>
        <a-descriptions-item :label="t('scheduler.fields.task')">
          {{ currentJob.task_name }} / {{ currentJob.job_id }}
        </a-descriptions-item>
        <a-descriptions-item :label="t('scheduler.fields.conversation')">
          {{ displayText(conversationText(currentJob)) }}
        </a-descriptions-item>
        <a-descriptions-item :label="t('scheduler.fields.label')">
          {{ displayText(currentJob.log_label || currentJob.payload_summary.content) }}
        </a-descriptions-item>
        <a-descriptions-item :label="t('scheduler.fields.cron')">
          {{ currentJob.cron_expr }} / {{ currentJob.timezone }}
        </a-descriptions-item>
        <a-descriptions-item :label="t('scheduler.fields.lastRun')">
          {{ formatDateTime(currentJob.last_run) }}
        </a-descriptions-item>
        <a-descriptions-item :label="t('scheduler.fields.duration')">
          {{ formatDurationMs(currentJob.last_duration_ms) }}
        </a-descriptions-item>
        <a-descriptions-item :label="t('scheduler.fields.nextRun')">
          {{ formatDateTime(currentJob.next_run) }}
        </a-descriptions-item>
        <a-descriptions-item :label="t('scheduler.fields.lastError')">
          <template v-if="currentJob.last_error">
            {{ currentJob.last_error.code }}: {{ currentJob.last_error.message }}
          </template>
          <template v-else>{{ t('display.empty') }}</template>
        </a-descriptions-item>
        <a-descriptions-item :label="t('scheduler.fields.stats')">
          <div class="scheduler-stats">
            <span class="scheduler-stats__total">{{ t('scheduler.stats.total', { count: currentJob.stats.total }) }}</span>
            <span
              v-for="item in statsItems(currentJob.stats)"
              :key="item.key"
              class="scheduler-stats__item"
              :class="item.className"
              :aria-label="`${item.aria}: ${item.value}`"
              :title="item.label"
            >
              <component :is="item.icon" />
              <span>{{ item.value }}</span>
            </span>
          </div>
        </a-descriptions-item>
      </a-descriptions>
    </a-modal>
  </AppPage>
</template>

<style lang="scss" scoped>
.scheduler-empty-card {
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-xs);
}

.scheduler-data-table {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-xs);
}

.scheduler-data-table :deep(.ant-spin-nested-loading),
.scheduler-data-table :deep(.ant-spin-container),
.scheduler-data-table :deep(.ant-table) {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
}

.scheduler-data-table :deep(.ant-table-container) {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

.scheduler-data-table :deep(.ant-table-content) {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto !important;
}

.scheduler-cell-identity {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.scheduler-cell-identity strong,
.scheduler-cell-copy span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.scheduler-cell-identity small,
.scheduler-mono {
  color: var(--text-tertiary);
  font-family: var(--font-mono);
  font-size: 12px;
}

.scheduler-cell-copy {
  max-width: 220px;
  min-width: 0;
}

.scheduler-error-alert {
  width: 240px;
}

.scheduler-error-alert :deep(.ant-alert-message),
.scheduler-error-alert :deep(.ant-alert-description) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.scheduler-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 8px;
  align-items: center;
}

.scheduler-stats__total {
  flex: 0 0 100%;
  color: var(--text-secondary);
  font-size: 12px;
}

.scheduler-stats__item {
  display: inline-flex;
  gap: 4px;
  align-items: center;
  font-size: 12px;
  font-weight: 600;
}

.scheduler-stats__item.is-success {
  color: #1f7a43;
}

.scheduler-stats__item.is-failed {
  color: #b42318;
}

.scheduler-stats__item.is-timeout {
  color: #ad6800;
}

.scheduler-stats__item.is-retry {
  color: #0958d9;
}

.scheduler-stats__item.is-other {
  color: #595959;
}

.scheduler-actions {
  display: inline-flex;
  gap: 8px;
  align-items: center;
}

:deep(.ant-table-row:hover > td) {
  background: var(--surface-accent) !important;
}
</style>
