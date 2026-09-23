<script setup lang="ts">
import { CircleCheckIcon, CircleXIcon, ClockIcon, CopyIcon, InfoIcon, TriangleAlertIcon } from '@lucide/vue'

import { copyText } from '@/adapter/clipboard'
import AppButton from '@/components/AppButton.vue'
import AppDialog from '@/components/AppDialog.vue'
import { t } from '@/i18n'
import { formatDateTime } from '@/lib/format'
import {
  conversationText,
  displayText,
  formatCronSchedule,
  formatDurationMs,
  getHealthRingStyle,
  getSuccessRate,
} from '@/lib/scheduler-job-display'
import type { SchedulerJobSummary } from '@/types/api'

// The job stays set while the dialog closes so the content remains during the exit animation.
defineProps<{ open: boolean; job: SchedulerJobSummary | null; pluginName: string; nextRunText: string }>()
defineEmits<{ close: []; afterClose: [] }>()

function copyError(error: NonNullable<SchedulerJobSummary['last_error']>) {
  return copyText(`${error.code}: ${error.message}`, t('scheduler.errorCopied'))
}
</script>

<template>
  <AppDialog :open="open" :title="t('scheduler.detailTitle')" :width="800" @close="$emit('close')" @after-close="$emit('afterClose')">
    <div v-if="job" class="modal-console-layout">
      <!-- 左侧：系统参数面板 -->
      <div class="console-pane-left">
        <div class="pane-group">
          <div class="pane-group-title">{{ t('scheduler.identitySection') }}</div>
          <div class="info-block">
            <span class="label">{{ t('scheduler.pluginModule') }}</span>
            <span class="value bold">{{ pluginName }}<span class="sr-only"> / </span></span>
            <span class="sub-val">{{ job.plugin_id }}</span>
          </div>
          <div class="info-block">
            <span class="label">{{ t('scheduler.taskName') }}</span>
            <span class="value bold">{{ job.task_name }}<span class="sr-only"> / </span></span>
            <span class="sub-val">{{ job.job_id }}</span>
          </div>
        </div>

        <div class="pane-group">
          <div class="pane-group-title">{{ t('scheduler.scheduleSection') }}</div>
          <div class="info-block inline">
            <div>
              <span class="label">{{ t('scheduler.cronRule') }}</span>
              <span class="value code">{{ job.cron_expr }}</span>
            </div>
            <div>
              <span class="label">{{ t('scheduler.timeZone') }}</span>
              <span class="value code">{{ job.timezone }}</span>
            </div>
          </div>
          <div class="info-block">
            <span class="label">{{ t('scheduler.scheduleMeaning') }}</span>
            <span class="value highlight">{{ formatCronSchedule(job.cron_expr) }}</span>
          </div>
        </div>

        <div class="pane-group">
          <div class="pane-group-title">{{ t('scheduler.contextSection') }}</div>
          <div class="info-block inline">
            <div>
              <span class="label">{{ t('scheduler.fields.conversation') }}</span>
              <span class="value code">{{ conversationText(job) || t('scheduler.globalTask') }}</span>
            </div>
          </div>
          <div class="info-block">
            <span class="label">{{ t('scheduler.contentLabel') }}</span>
            <span class="value">{{ displayText(job.log_label || job.payload_summary.content) }}</span>
          </div>
        </div>
      </div>

      <!-- 右侧：健康运行分析仪 -->
      <div class="console-pane-right">
        <div class="pane-group-title">{{ t('scheduler.healthSection') }}</div>

        <div class="health-instrument">
          <!-- 仪表圆环 -->
          <div class="health-gauge" :style="getHealthRingStyle(job.stats)">
            <div class="gauge-center">
              <span class="gauge-pct">{{ job.stats.total ? `${getSuccessRate(job.stats)}%` : '-' }}</span>
              <span class="gauge-desc">{{ t('scheduler.health') }}</span>
            </div>
          </div>

          <!-- 数据列项 -->
          <div class="gauge-stats-list">
            <div class="stat-item success">
              <CircleCheckIcon />
              <span class="lbl">{{ t('scheduler.successRuns') }}</span>
              <span class="val">{{ t('scheduler.runCount', { count: job.stats.success }) }}</span>
            </div>
            <div class="stat-item failed">
              <CircleXIcon />
              <span class="lbl">{{ t('scheduler.failedRuns') }}</span>
              <span class="val">{{ t('scheduler.runCount', { count: job.stats.failed }) }}</span>
            </div>
            <div class="stat-item warning">
              <ClockIcon />
              <span class="lbl">{{ t('scheduler.timeoutRuns') }}</span>
              <span class="val">{{ t('scheduler.runCount', { count: job.stats.timeout }) }}</span>
            </div>
            <div class="stat-item other">
              <InfoIcon />
              <span class="lbl">{{ t('scheduler.retryRuns') }}</span>
              <span class="val">{{ t('scheduler.runCount', { count: job.stats.retry }) }}</span>
            </div>
          </div>
        </div>

        <div class="pane-group margin-top">
          <div class="pane-group-title">{{ t('scheduler.performanceSection') }}</div>
          <div class="info-block inline">
            <div>
              <span class="label">{{ t('scheduler.previousRun') }}</span>
              <span class="value small-text">{{ job.last_run ? formatDateTime(job.last_run) : t('scheduler.notStarted') }}</span>
            </div>
            <div>
              <span class="label">{{ t('scheduler.runDuration') }}</span>
              <span class="value highlight">{{ formatDurationMs(job.last_duration_ms) }}</span>
            </div>
          </div>
          <div class="info-block">
            <span class="label">{{ t('scheduler.scheduledNextRun') }}</span>
            <span class="value small-text">
              {{ job.next_run ? formatDateTime(job.next_run) : t('scheduler.notScheduled') }}
              <span class="rel-time" v-if="job.next_run">({{ nextRunText }})</span>
            </span>
          </div>
        </div>

        <!-- 最近运行错误报告区 -->
        <div class="console-error-report" v-if="job.last_error">
          <div class="report-head">
            <TriangleAlertIcon />
            <span>{{ t('scheduler.errorReport') }}</span>
          </div>
          <div class="report-body">
            <div class="err-code">{{ t('scheduler.errorCode') }} <code>{{ job.last_error.code }}</code></div>
            <p class="err-msg">{{ job.last_error.message }}</p>
            <AppButton size="sm" class="copy-console-err-btn" @click="copyError(job.last_error)" variant="destructive">
              <template #icon><CopyIcon /></template>
              {{ t('scheduler.copyDiagnosis') }}
            </AppButton>
          </div>
        </div>
      </div>
    </div>
  </AppDialog>
</template>

<style lang="scss" scoped>
.lucide { width: 16px; height: 16px; flex-shrink: 0; }
.modal-console-layout {
  display: grid;
  grid-template-columns: 1.1fr 1fr;
  gap: 20px;
}

.console-pane-left {
  border-right: 1px solid var(--border);
  padding-right: 20px;
}

.console-pane-left,
.console-pane-right {
  display: flex;
  flex-direction: column;
  gap: 16px;

  .pane-group-title {
    font-size: 13px;
    font-weight: 700;
    color: var(--accent);
    letter-spacing: 0;
    margin-bottom: 8px;
  }

  .pane-group {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .info-block {
    display: flex;
    flex-direction: column;
    background: color-mix(in srgb, var(--text) 3%, transparent);
    padding: 8px 12px;
    border-radius: 8px;
    border: 1px solid var(--border);

    .label {
      font-size: 13px;
      color: var(--muted);
      margin-bottom: 3px;
    }

    .value {
      font-size: 13px;
      font-weight: 600;
      color: var(--text);

      &.bold {
        font-weight: 700;
        font-size: 14px;
      }
      &.code {
        font-family: var(--font-mono);
        color: var(--text);
        background: color-mix(in srgb, var(--text) 5%, transparent);
        padding-inline: 4px;
        border-radius: 3px;
        font-size: 12px;
        width: fit-content;
      }
      &.highlight {
        color: var(--accent);
        font-weight: 700;
      }
    }

    .sub-val {
      font-size: 13px;
      color: var(--muted);
      font-family: var(--font-mono);
      margin-top: 2px;
    }

    &.inline {
      flex-direction: row;
      justify-content: space-between;
      gap: 12px;

      & > div {
        display: flex;
        flex-direction: column;
        flex: 1 1 0%;
      }
    }
  }
}

.console-pane-right {
  .margin-top {
    margin-top: 4px;
  }

  .small-text {
    font-size: 12px !important;
  }

  .rel-time {
    color: var(--accent);
    font-weight: 600;
    margin-left: 4px;
  }

  /* 运行健康仪表盘 */
  .health-instrument {
    display: flex;
    align-items: center;
    gap: 16px;
    background: color-mix(in srgb, var(--text) 3%, transparent);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 14px;
  }

  .health-gauge {
    width: 90px;
    height: 90px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 8px; // 圆环粗细
    flex-shrink: 0;

    .gauge-center {
      width: 100%;
      height: 100%;
      border-radius: 50%;
      background: var(--surface) !important;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      box-shadow: var(--shadow-sm);
    }

    .gauge-pct {
      font-size: 18px;
      font-weight: 800;
      color: var(--text);
      line-height: 1;
    }

    .gauge-desc {
      font-size: 12px;
      color: var(--muted);
      margin-top: 2px;
    }
  }

  .gauge-stats-list {
    display: flex;
    flex-direction: column;
    gap: 6px;
    flex: 1 1 auto;

    .stat-item {
      display: flex;
      align-items: center;
      font-size: 12px;
      font-weight: 500;

      span.lbl {
        margin-left: 6px;
        color: var(--muted);
      }

      span.val {
        margin-left: auto;
        font-family: var(--font-mono);
        font-weight: 700;
      }

      &.success { color: var(--success); }
      &.failed { color: var(--danger); }
      &.warning { color: var(--warning); }
      &.other { color: var(--muted); }
    }
  }

  /* 故障诊断 */
  .console-error-report {
    background: color-mix(in srgb, var(--danger) 8%, transparent);
    border: 1px solid color-mix(in srgb, var(--danger) 22%, transparent);
    border-radius: 10px;
    padding: 12px 14px;
    display: flex;
    flex-direction: column;
    gap: 8px;

    .report-head {
      display: flex;
      align-items: center;
      gap: 6px;
      font-size: 13px;
      font-weight: 700;
      color: var(--danger);
    }

    .report-body {
      display: flex;
      flex-direction: column;
      gap: 6px;

      .err-code {
        font-size: 12px;
        color: var(--text);

        code {
          font-family: var(--font-mono);
          background: color-mix(in srgb, var(--danger) 15%, transparent);
          padding: 1px 5px;
          border-radius: 4px;
          color: var(--danger);
          font-weight: 700;
        }
      }

      .err-msg {
        font-size: 13px;
        color: var(--text);
        font-family: var(--font-mono);
        background: color-mix(in srgb, var(--surface) 60%, transparent);
        padding: 8px;
        border-radius: 6px;
        max-height: 80px;
        overflow-y: auto;
        word-break: break-all;
        margin: 0;
        border: 1px solid color-mix(in srgb, var(--danger) 10%, transparent);
      }

      .copy-console-err-btn {
        margin-top: 4px;
        font-size: 13px;
        height: 28px;
        width: fit-content;
        align-self: flex-end;
      }
    }
  }
}
</style>
