<script setup lang="ts">
import { CopyIcon } from '@lucide/vue'

import { copyText } from '@/adapter/clipboard'
import AppAlert from '@/components/AppAlert.vue'
import AppButton from '@/components/AppButton.vue'
import AppDetailItem from '@/components/AppDetailItem.vue'
import AppDetails from '@/components/AppDetails.vue'
import AppDialog from '@/components/AppDialog.vue'
import { t } from '@/i18n'
import { formatDateTime } from '@/lib/format'
import {
  conversationText,
  displayText,
  formatCronSchedule,
  formatDurationMs,
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
  <AppDialog :open="open" :title="t('scheduler.detailTitle')" :width="720" @close="$emit('close')" @after-close="$emit('afterClose')">
    <!-- Facts sit in divided rows on the dialog itself: no gauge, no boxes within the box. -->
    <div v-if="job" class="scheduler-job-detail">
      <AppAlert v-if="job.last_error" tone="danger" :title="t('scheduler.recentError')" :description="job.last_error.message">
        <code class="scheduler-job-detail__error-code">{{ job.last_error.code }}</code>
        <template #action>
          <AppButton size="sm" @click="copyError(job.last_error)">
            <template #icon><CopyIcon /></template>
            {{ t('scheduler.copyError') }}
          </AppButton>
        </template>
      </AppAlert>

      <section class="scheduler-job-detail__group">
        <h3>{{ t('scheduler.identitySection') }}</h3>
        <AppDetails>
          <AppDetailItem :label="t('scheduler.fields.plugin')">
            {{ pluginName }}<span class="sr-only"> / </span><span class="scheduler-job-detail__id">{{ job.plugin_id }}</span>
          </AppDetailItem>
          <AppDetailItem :label="t('scheduler.taskName')">
            {{ job.task_name }}<span class="sr-only"> / </span><span class="scheduler-job-detail__id">{{ job.job_id }}</span>
          </AppDetailItem>
          <AppDetailItem :label="t('scheduler.fields.conversation')">
            <span v-if="conversationText(job)" class="scheduler-job-detail__mono">{{ conversationText(job) }}</span>
            <template v-else>{{ t('scheduler.globalTask') }}</template>
          </AppDetailItem>
          <AppDetailItem :label="t('scheduler.fields.label')">{{ displayText(job.log_label || job.payload_summary.content) }}</AppDetailItem>
        </AppDetails>
      </section>

      <section class="scheduler-job-detail__group">
        <h3>{{ t('scheduler.scheduleSection') }}</h3>
        <AppDetails>
          <AppDetailItem :label="t('scheduler.scheduleMeaning')">{{ formatCronSchedule(job.cron_expr) }}</AppDetailItem>
          <AppDetailItem :label="t('scheduler.cronRule')">
            <span class="scheduler-job-detail__mono">{{ job.cron_expr }}</span><span class="scheduler-job-detail__id">{{ job.timezone }}</span>
          </AppDetailItem>
          <AppDetailItem :label="t('scheduler.previousRun')">{{ job.last_run ? formatDateTime(job.last_run) : t('scheduler.notRun') }}</AppDetailItem>
          <AppDetailItem :label="t('scheduler.runDuration')">{{ formatDurationMs(job.last_duration_ms) }}</AppDetailItem>
          <AppDetailItem :label="t('scheduler.scheduledNextRun')">
            <template v-if="job.next_run">{{ formatDateTime(job.next_run) }}<span v-if="nextRunText" class="scheduler-job-detail__note">{{ nextRunText }}</span></template>
            <template v-else>{{ t('scheduler.notScheduled') }}</template>
          </AppDetailItem>
        </AppDetails>
      </section>

      <section class="scheduler-job-detail__group">
        <h3>{{ t('scheduler.healthSection') }}</h3>
        <AppDetails>
          <AppDetailItem :label="t('scheduler.health')">
            {{ job.stats.total ? t('scheduler.successSummary', { total: job.stats.total, rate: getSuccessRate(job.stats) }) : t('scheduler.notRun') }}
          </AppDetailItem>
          <AppDetailItem :label="t('scheduler.runBreakdown')">
            {{ t('scheduler.breakdown', { success: job.stats.success, failed: job.stats.failed, timeout: job.stats.timeout, retry: job.stats.retry }) }}
          </AppDetailItem>
        </AppDetails>
      </section>
    </div>
  </AppDialog>
</template>

<style lang="scss" scoped>
.scheduler-job-detail {
  display: grid;
  gap: 20px;
}

.scheduler-job-detail__group {
  display: grid;
  gap: 4px;
}

.scheduler-job-detail__group h3 {
  margin: 0;
  color: var(--text);
  font-size: 14px;
  font-weight: 600;
}

.scheduler-job-detail__id {
  margin-inline-start: 8px;
  color: var(--muted);
  font-family: var(--font-mono);
  font-size: 12px;
}

.scheduler-job-detail__note {
  margin-inline-start: 8px;
  color: var(--muted);
}

.scheduler-job-detail__mono {
  font-family: var(--font-mono);
}

.scheduler-job-detail__error-code {
  display: block;
  margin-top: 4px;
  color: var(--muted);
  font-family: var(--font-mono);
  font-size: 12px;
}
</style>
