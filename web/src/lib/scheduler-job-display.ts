import { i18n, t } from '@/i18n'
import { timestampMilliseconds } from '@/lib/timestamp'
import type { SchedulerJobRunStats, SchedulerJobSummary } from '@/types/api'

export function formatDurationMs(value: number) {
  if (!Number.isFinite(value) || value <= 0) {
    return t('display.empty')
  }
  if (value < 1000) {
    return `${value} ${t('display.durationUnits.ms')}`
  }
  return `${(value / 1000).toFixed(value < 10_000 ? 1 : 0)} ${t('display.durationUnits.s')}`
}

// task_name is the job ID; the readable name a plugin gives its job is the log label.
export function taskLabel(job: SchedulerJobSummary) {
  return job.log_label?.trim() || job.task_name
}

// Runs cancelled by a stop are counted, not failed; records written before the server stopped keeping them as the
// last error may still hold one.
const cancellationCodes = new Set(['plugin.event_canceled'])

// The server keeps the last error after later runs succeed, so it describes the latest run only when it is that run's.
export function isLatestRunFailed(job: SchedulerJobSummary) {
  if (!job.last_error || cancellationCodes.has(job.last_error.code)) return false
  const errorAt = timestampMilliseconds(job.last_error.at)
  const lastRunAt = timestampMilliseconds(job.last_run)
  if (errorAt === null || lastRunAt === null) return true
  return errorAt >= lastRunAt
}

// Error codes have readable messages; a run outcome stands in for the code when the plugin gave none.
export function errorLabel(code: string) {
  if (i18n.global.te(`errors.${code}`)) return t(`errors.${code}`)
  if (i18n.global.te(`scheduler.outcomes.${code}`)) return t(`scheduler.outcomes.${code}`)
  return code
}

export function getDurationClass(value: number) {
  if (!Number.isFinite(value) || value <= 0) return 'duration-empty'
  if (value < 1000) return 'duration-fast'
  if (value < 5000) return 'duration-normal'
  return 'duration-slow'
}

// A target reads as the chat it addresses; a conversation key the page cannot read is shown as sent.
export function conversationText(job: SchedulerJobSummary) {
  const payload = job.payload_summary
  const [type, id] = payload.target_type && payload.target_id
    ? [payload.target_type, payload.target_id]
    : payload.conversation_id.split(/:(.*)/s)
  if (type === 'group' && id) return t('scheduler.groupTarget', { id })
  if (type === 'private' && id) return t('scheduler.privateTarget', { id })
  return payload.conversation_id
}

export function formatCronSchedule(cron?: string): string {
  if (!cron) return t('scheduler.unconfigured')
  const parts = cron.trim().split(/\s+/)
  if (parts.length !== 5) return cron

  const [min, hour, day, month, week] = parts
  if (day !== '*' || month !== '*' || week !== '*') return cron

  if (min === '*' && hour === '*') {
    return t('scheduler.everyMinute')
  }
  if (/^\*\/[1-9]\d*$/.test(min) && Number(min.slice(2)) < 60 && hour === '*') {
    return t('scheduler.everyMinutes', { count: min.substring(2) })
  }
  if (min === '0' && /^\*\/[1-9]\d*$/.test(hour) && Number(hour.slice(2)) < 24) {
    return t('scheduler.everyHours', { count: hour.substring(2) })
  }
  if (/^\d{1,2}$/.test(min) && Number(min) < 60 && /^\d{1,2}$/.test(hour) && Number(hour) < 24) {
    return t('scheduler.everyDayAt', { time: `${hour.padStart(2, '0')}:${min.padStart(2, '0')}` })
  }
  return cron
}

export function formatNextRunRelative(nextRunTime: string | undefined, now: number) {
  if (!nextRunTime) return ''
  const next = new Date(nextRunTime).getTime()
  if (!Number.isFinite(next) || !Number.isFinite(now)) return ''
  const diffMs = next - now
  if (diffMs <= 0) return t('scheduler.dueSoon')
  const diffMin = Math.round(diffMs / 60000)
  if (diffMin < 1) {
    const diffSec = Math.round(diffMs / 1000)
    return t('scheduler.inSeconds', { count: diffSec > 0 ? diffSec : 1 })
  }
  if (diffMin < 60) {
    return t('scheduler.inMinutes', { count: diffMin })
  }
  const diffHour = Math.floor(diffMin / 60)
  const remainMin = diffMin % 60
  if (diffHour < 24) {
    return t('scheduler.inHoursMinutes', { hours: diffHour, minutes: remainMin })
  }
  const diffDay = Math.floor(diffHour / 24)
  return t('scheduler.inDays', { count: diffDay })
}

export function getSuccessRate(stats: SchedulerJobRunStats): number {
  if (!stats.total) return 0
  return Math.round((stats.success / stats.total) * 100)
}

export function successRateText(stats: SchedulerJobRunStats): string {
  if (!stats.total) return t('scheduler.notRun')
  return t('scheduler.successRate', { rate: getSuccessRate(stats) })
}

