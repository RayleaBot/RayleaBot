import { t } from '@/i18n'
import type { SchedulerJobRunStats, SchedulerJobSummary } from '@/types/api'

export function formatDurationMs(value: number) {
  if (!Number.isFinite(value) || value <= 0) {
    return t('display.empty')
  }
  if (value < 1000) {
    return `${value} ms`
  }
  return `${(value / 1000).toFixed(value < 10_000 ? 1 : 0)} s`
}

export function getDurationClass(value: number) {
  if (!Number.isFinite(value) || value <= 0) return 'duration-empty'
  if (value < 1000) return 'duration-fast'
  if (value < 5000) return 'duration-normal'
  return 'duration-slow'
}

export function displayText(value?: string | null) {
  return value?.trim() || t('display.empty')
}

export function conversationText(job: SchedulerJobSummary) {
  const payload = job.payload_summary
  if (payload.conversation_id) {
    return payload.conversation_id
  }
  if (payload.target_type && payload.target_id) {
    return `${payload.target_type}:${payload.target_id}`
  }
  return ''
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

export function getHealthRingStyle(stats: SchedulerJobRunStats) {
  const rate = getSuccessRate(stats)
  return {
    background: `conic-gradient(var(--success) 0% ${rate}%, var(--border) ${rate}% 100%)`
  }
}
