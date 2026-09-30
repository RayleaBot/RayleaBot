<script setup lang="ts">
import {
  CheckIcon,
  ClockIcon,
  CopyIcon,
  EyeIcon,
  MessageSquareIcon,
  TriangleAlertIcon,
  ZapIcon,
} from '@lucide/vue'
import { computed } from 'vue'

import { copyText } from '@/adapter/clipboard'
import AppButton from '@/components/AppButton.vue'
import AppDataTable from '@/components/AppDataTable.vue'
import AppPopover from '@/components/AppPopover.vue'
import PluginIcon from '@/components/plugins/PluginIcon.vue'
import { t } from '@/i18n'
import { formatDateTime } from '@/lib/format'
import {
  conversationText,
  displayText,
  formatCronSchedule,
  formatDurationMs,
  formatNextRunRelative,
  getDurationClass,
  successRateText,
} from '@/lib/scheduler-job-display'
import { usePluginsStore } from '@/stores/plugins'
import type { SchedulerJobSummary } from '@/types/api'

// now: the page clock used for countdowns to the next run.
defineProps<{ jobs: SchedulerJobSummary[]; now: number; triggeringJobId: string | null }>()
const emit = defineEmits<{ view: [job: SchedulerJobSummary]; trigger: [job: SchedulerJobSummary] }>()

const pluginsStore = usePluginsStore()
const pluginMap = computed(() => new Map(pluginsStore.items.map(plugin => [plugin.id, plugin])))
const tableColumns = computed(() => [
  { label: `${t('scheduler.fields.plugin')} / ${t('scheduler.fields.task')}`, key: 'plugin', width: 300 },
  { label: `${t('scheduler.fields.label')} / ${t('scheduler.fields.conversation')}`, key: 'label', width: 250 },
  { label: `${t('scheduler.fields.cron')} / ${t('scheduler.fields.nextRun')}`, key: 'cron', width: 320 },
  { label: `${t('scheduler.fields.lastRun')} / ${t('scheduler.fields.duration')}`, key: 'lastRun', width: 240 },
  { label: `${t('scheduler.fields.stats')} / ${t('scheduler.fields.lastError')}`, key: 'stats', width: 280 },
  { label: t('scheduler.fields.actions'), key: 'actions', width: 180 },
])

function pluginName(job: SchedulerJobSummary) {
  return pluginsStore.getPluginDisplayName(job.plugin_id, job.plugin_name)
}

function schedulerRowKey(row: SchedulerJobSummary) {
  return row.job_id
}

function copyError(error: NonNullable<SchedulerJobSummary['last_error']>) {
  return copyText(`${error.code}: ${error.message}`, t('scheduler.errorCopied'))
}
</script>

<template>
  <div class="table-container-wrapper app-box">
    <AppDataTable
      class="scheduler-data-table app-data-table refactored-table"
      :columns="tableColumns"
      :rows="jobs"
      :row-key="schedulerRowKey"
      :min-width="1450"
      :label="t('scheduler.title')"
    >
      <template #empty>
        {{ t('display.empty') }}
      </template>

      <template #cell="{ column, row: record }">
        <!-- 1. 插件与任务合并列 -->
        <template v-if="column.key === 'plugin'">
          <div class="scheduler-cell-plugin-task">
            <PluginIcon :plugin-id="record.plugin_id" :icon="pluginMap.get(record.plugin_id)?.icon" :version="pluginMap.get(record.plugin_id)?.version" :refresh-key="pluginsStore.iconRevision" />
            <div class="meta-content">
              <div class="top-row">
                <strong class="plugin-name">{{ pluginName(record) }}</strong>
                <span class="task-tag">{{ record.task_name }}</span>
              </div>
              <div class="bottom-row">
                <span class="plugin-id" :title="t('scheduler.pluginId')">{{ record.plugin_id }}</span>
                <span class="divider">/</span>
                <span class="job-id" :title="t('scheduler.jobId')">{{ record.job_id }}</span>
              </div>
            </div>
          </div>
        </template>

        <!-- 2. 自定义内容与会话 ID 合并列 -->
        <template v-else-if="column.key === 'label'">
          <div class="scheduler-cell-label-conv">
            <div class="label-text" :title="record.log_label || record.payload_summary.content">
              {{ displayText(record.log_label || record.payload_summary.content) }}
            </div>
            <div class="conv-tag-row">
              <template v-if="conversationText(record)">
                <span class="conv-badge">
                  <MessageSquareIcon class="badge-icon" />
                  <span class="badge-text">{{ conversationText(record) }}</span>
                </span>
              </template>
              <template v-else>
                <span class="conv-badge global">{{ t('scheduler.globalConversation') }}</span>
              </template>
            </div>
          </div>
        </template>

        <!-- 3. 定时计划与下一次执行列 -->
        <template v-else-if="column.key === 'cron'">
          <div class="scheduler-cell-cron-next">
            <div class="cron-expr-row" :title="t('scheduler.expression', { expression: record.cron_expr, timeZone: record.timezone })">
              <span class="chinese-cron">{{ formatCronSchedule(record.cron_expr) }}</span>
              <span class="raw-cron">{{ record.cron_expr }}</span>
            </div>
            <div class="next-run-row">
              <ClockIcon class="clock-icon" />
              <span class="next-time" :title="formatDateTime(record.next_run)">
                {{ formatDateTime(record.next_run) }}
              </span>
              <span class="relative-time-pill" v-if="record.next_run">
                {{ formatNextRunRelative(record.next_run, now) }}
              </span>
            </div>
          </div>
        </template>

        <!-- 4. 最近执行与耗时列 -->
        <template v-else-if="column.key === 'lastRun'">
          <div class="scheduler-cell-run-duration">
            <div class="last-run-time">
              {{ record.last_run ? formatDateTime(record.last_run) : t('scheduler.notRun') }}
            </div>
            <div class="duration-row" v-if="record.last_run">
              <span class="duration-badge" :class="getDurationClass(record.last_duration_ms)">
                {{ formatDurationMs(record.last_duration_ms) }}
              </span>
            </div>
          </div>
        </template>

        <!-- 5. 执行情况与最近错误列 -->
        <template v-else-if="column.key === 'stats'">
          <div class="scheduler-cell-health-stats">
            <div class="stats-header">
              <span class="total-count">{{ t('scheduler.stats.total', { count: record.stats.total }) }}</span>
              <span class="success-rate-pct">{{ successRateText(record.stats) }}</span>
            </div>

            <!-- 运行结果占比 -->
            <div class="mini-stacked-bar" v-if="record.stats.total > 0">
              <div
                class="bar-success"
                :style="{ width: `${(record.stats.success / record.stats.total) * 100}%` }"
                :title="t('scheduler.stats.success', { count: record.stats.success })"
              ></div>
              <div
                class="bar-failed"
                :style="{ width: `${(record.stats.failed / record.stats.total) * 100}%` }"
                :title="t('scheduler.stats.failed', { count: record.stats.failed })"
              ></div>
              <div
                class="bar-other"
                :style="{ width: `${((record.stats.total - record.stats.success - record.stats.failed) / record.stats.total) * 100}%` }"
                :title="t('scheduler.stats.other', { count: record.stats.total - record.stats.success - record.stats.failed })"
              ></div>
            </div>

            <!-- 错误气泡 -->
            <div class="error-badge-row" v-if="record.last_error">
              <AppPopover :title="t('scheduler.recentError')" side="left">
                <template #content>
                  <div class="error-popover-content">
                    <div class="err-title">
                      <TriangleAlertIcon class="err-icon" />
                      <strong>{{ record.last_error.code }}</strong>
                    </div>
                    <div class="err-msg">{{ record.last_error.message }}</div>
                    <AppButton size="sm" variant="link" class="copy-err-btn" @click="copyError(record.last_error)">
                      <template #icon><CopyIcon /></template>
                      {{ t('scheduler.copyError') }}
                    </AppButton>
                  </div>
                </template>
                <button type="button" class="error-capsule">
                  {{ record.last_error.code }}
                </button>
              </AppPopover>
            </div>
            <div class="success-dot-row" v-else-if="record.stats.total > 0">
              <span class="success-dot"><CheckIcon class="ok-icon" /> {{ t('scheduler.healthy') }}</span>
            </div>
          </div>
        </template>

        <!-- 6. 操作列 -->
        <template v-else-if="column.key === 'actions'">
          <div class="scheduler-actions">
            <AppButton size="sm" class="action-btn view-btn" @click="emit('view', record)">
              <template #icon>
                <EyeIcon />
              </template>
              {{ t('scheduler.view') }}
            </AppButton>
            <AppButton
              size="sm"
              class="action-btn trigger-btn"
              :loading="triggeringJobId === record.job_id"
              @click="emit('trigger', record)"
            >
              <template #icon>
                <ZapIcon />
              </template>
              {{ t('scheduler.trigger') }}
            </AppButton>
          </div>
        </template>
      </template>
    </AppDataTable>
  </div>
</template>

<style lang="scss" scoped>
.lucide { width: 16px; height: 16px; flex-shrink: 0; }
.error-capsule:focus-visible { outline: 2px solid var(--focus); outline-offset: var(--focus-outline-offset); }
.table-container-wrapper {
  overflow: hidden;
}

.refactored-table :deep(th) { font-weight: 600; color: var(--text); }
.refactored-table :deep(th:last-child), .refactored-table :deep(td:last-child) { position: sticky; right: 0; z-index: 1; background: var(--surface-strong); border-left: 1px solid var(--border); }
.refactored-table :deep(th:last-child) { background: var(--surface-soft); }

/* 单元格布局 */
.scheduler-cell-plugin-task {
  display: flex;
  align-items: center;
  gap: 12px;
  white-space: nowrap;



  .meta-content {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }

  .top-row {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;

    .plugin-name {
      font-size: 14px;
      color: var(--text);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-weight: 600;
    }

    .task-tag {
      font-size: 13px;
      background: color-mix(in srgb, var(--accent) 8%, transparent);
      color: var(--accent);
      border: 1px solid color-mix(in srgb, var(--accent) 20%, transparent);
      padding: 1px 6px;
      border-radius: 4px;
      font-weight: 500;
      white-space: nowrap;
    }
  }

  .bottom-row {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 13px;
    color: var(--muted);
    font-family: var(--font-mono);

    span {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .divider {
      color: color-mix(in srgb, var(--border) 60%, transparent);
    }
  }
}

.scheduler-cell-label-conv {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
  white-space: nowrap;

  .label-text {
    font-size: 13px;
    color: var(--text);
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .conv-tag-row {
    display: flex;
    align-items: center;
  }

  .conv-badge {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: 13px;
    background: var(--surface-soft);
    color: var(--muted);
    padding: 1px 8px;
    border-radius: var(--radius-xs);
    max-width: 160px;
    font-family: var(--font-mono);

    .badge-icon {
      font-size: 12px;
    }
    .badge-text {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    // A global task is a kind of target, not a health state, so it stays neutral.
    &.global {
      font-family: var(--font-sans);
    }
  }
}

.scheduler-cell-cron-next {
  display: flex;
  flex-direction: column;
  gap: 5px;
  min-width: 0;
  white-space: nowrap;

  .cron-expr-row {
    display: flex;
    align-items: center;
    gap: 8px;
    white-space: nowrap;
    flex-shrink: 0;

    .chinese-cron {
      font-size: 13px;
      font-weight: 600;
      color: var(--text);
    }

    .raw-cron {
      font-size: 13px;
      font-family: var(--font-mono);
      color: var(--muted);
      background: color-mix(in srgb, var(--text) 4%, transparent);
      padding: 0 4px;
      border-radius: 3px;
    }
  }

  .next-run-row {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--muted);

    .clock-icon {
      font-size: 13px;
    }

    .next-time {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .relative-time-pill {
      font-size: 12px;
      background: var(--accent-soft);
      color: var(--accent);
      padding: 0px 5px;
      border-radius: 4px;
      font-weight: 600;
      white-space: nowrap;
    }
  }
}

.scheduler-cell-run-duration {
  display: flex;
  flex-direction: column;
  gap: 6px;
  white-space: nowrap;

  .last-run-time {
    font-size: 12px;
    color: var(--text);
  }

  .duration-row {
    display: flex;
  }

  .duration-badge {
    font-size: 13px;
    font-weight: 600;
    padding: 1px 6px;
    border-radius: 5px;
    font-family: var(--font-mono);
    border: 1px solid transparent;

    // Ordinary durations read as plain values; only a slow run is marked.
    &.duration-fast,
    &.duration-normal {
      background: var(--surface-soft);
      color: var(--muted);
    }

    &.duration-slow {
      background: color-mix(in srgb, var(--warning) 8%, transparent);
      border-color: color-mix(in srgb, var(--warning) 20%, transparent);
      color: var(--warning);
    }
  }
}

.scheduler-cell-health-stats {
  display: flex;
  flex-direction: column;
  gap: 5px;
  width: 100%;
  white-space: nowrap;

  .stats-header {
    display: flex;
    justify-content: flex-start;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    color: var(--muted);
    font-weight: 500;
    white-space: nowrap;
    padding-right: 0;
  }

  .mini-stacked-bar {
    display: flex;
    height: 6px;
    width: 140px;
    background: color-mix(in srgb, var(--text) 8%, transparent);
    border-radius: 3px;
    overflow: hidden;

    .bar-success {
      height: 100%;
      background: var(--success);
    }

    .bar-failed {
      height: 100%;
      background: var(--danger);
    }

    .bar-other {
      height: 100%;
      background: var(--warning);
    }
  }

  .error-badge-row {
    display: flex;
    margin-top: 2px;
  }

  .error-capsule {
    font-size: 12px;
    font-weight: 700;
    background: var(--surface-danger);
    border: 0;
    color: var(--text-danger);
    padding: 2px 8px;
    border-radius: var(--radius-xs);
    cursor: pointer;
    font-family: var(--font-mono);
    text-transform: uppercase;
    letter-spacing: 0.02em;
    transition: background-color 150ms ease, color 150ms ease;

    &:hover {
      background: var(--danger);
      color: var(--on-brand);
    }
  }

  .success-dot-row {
    display: flex;
    align-items: center;
    margin-top: 2px;
  }

  .success-dot {
    font-size: 13px;
    color: var(--success);
    font-weight: 600;
    display: inline-flex;
    align-items: center;
    gap: 4px;

    .ok-icon {
      font-size: 12px;
    }
  }
}

/* 气泡故障面板样式 */
.error-popover-content {
  max-width: 280px;
  padding: 4px;

  .err-title {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--danger);
    font-size: 13px;
    margin-bottom: 6px;

    .err-icon {
      font-size: 14px;
    }
  }

  .err-msg {
    font-size: 12px;
    color: var(--text);
    background: color-mix(in srgb, var(--text) 4%, transparent);
    padding: 6px 8px;
    border-radius: 6px;
    word-break: break-all;
    font-family: var(--font-mono);
    max-height: 120px;
    overflow-y: auto;
  }

  .copy-err-btn {
    padding: 0;
    height: auto;
    font-size: 13px;
    margin-top: 8px;
  }
}

.scheduler-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
</style>
