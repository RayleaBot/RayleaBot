import { i18n, t } from '@/i18n'
import { timestampMilliseconds } from '@/lib/timestamp'
import { parseRateLimitValue } from '@/lib/rate-limit'
import { getActivePinia } from 'pinia'
import { useConfigStore } from '@/stores/config'
import { DEFAULT_TIME_ZONE } from '@/lib/time-zone'

export function managementTimeZone() {
  return getActivePinia() ? useConfigStore().effectiveTimezone : DEFAULT_TIME_ZONE
}

export function formatDateTime(value?: string | number | Date | null) {
  const date = toValidDate(value)
  if (!date) {
    return formatFallbackValue(value)
  }

  return new Intl.DateTimeFormat(i18n.global.locale.value, {
    dateStyle: 'short',
    timeStyle: 'medium',
    timeZone: managementTimeZone(),
  }).format(date)
}

export function formatTime(value?: string | number | Date | null) {
  const date = toValidDate(value)
  if (!date) return formatFallbackValue(value)
  return new Intl.DateTimeFormat(i18n.global.locale.value, { timeStyle: 'medium', timeZone: managementTimeZone() }).format(date)
}

export function formatRelativeTime(value?: string | number | Date | null): string {
  const date = toValidDate(value)
  if (!date) {
    return formatFallbackValue(value)
  }

  const now = Date.now()
  const diffMs = now - date.getTime()
  const direction = t(diffMs < 0 ? 'display.relative.after' : 'display.relative.before')
  const diffSec = Math.floor(Math.abs(diffMs) / 1000)
  const diffMin = Math.floor(diffSec / 60)
  const diffHour = Math.floor(diffMin / 60)
  const diffDay = Math.floor(diffHour / 24)

  if (diffSec < 60) {
    return t('display.relative.seconds', { count: diffSec, direction })
  }
  if (diffMin < 60) {
    return t('display.relative.minutes', { count: diffMin, direction })
  }
  if (diffHour < 24) {
    return t('display.relative.hours', { count: diffHour, direction })
  }
  return t('display.relative.days', { count: diffDay, direction })
}

export function formatDurationSeconds(seconds?: number) {
  if (seconds === undefined || !Number.isFinite(seconds) || seconds < 0) {
    return t('display.empty')
  }

  if (seconds < 60) {
    return t('display.duration.seconds', { seconds })
  }

  if (seconds < 3600) {
    return t('display.duration.minutesSeconds', { minutes: Math.floor(seconds / 60), seconds: seconds % 60 })
  }

  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  return t('display.duration.hoursMinutes', { hours, minutes })
}

export function formatRateLimit(value?: string | null) {
  if (value === undefined || value === null) {
    return t('display.empty')
  }

  const trimmed = value.trim()
  if (!trimmed) {
    return t('display.empty')
  }

  const parsed = parseRateLimit(trimmed)
  if (!parsed) {
    return trimmed
  }

  return t('display.rateLimit', { window: parsed.windowLabel, count: parsed.count })
}

export function toMultilineList(values: string[]) {
  return values.join('\n')
}

export function fromMultilineList(value: string) {
  return value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function toValidDate(value?: string | number | Date | null) {
  const milliseconds = timestampMilliseconds(value)
  return milliseconds === null ? null : new Date(milliseconds)
}

function formatFallbackValue(value?: string | number | Date | null) {
  if (typeof value === 'string') {
    const trimmed = value.trim()
    return trimmed || t('display.empty')
  }

  if (typeof value === 'number' && Number.isFinite(value)) {
    return String(value)
  }

  return t('display.empty')
}

function parseRateLimit(raw: string) {
  const parsed = parseRateLimitValue(raw)
  if (!parsed) {
    return null
  }

  const windowLabel = formatDurationLabel(`${parsed.windowValue}${parsed.unit}`)
  if (!windowLabel) {
    return null
  }

  return {
    count: parsed.count,
    windowLabel,
  }
}

function formatDurationLabel(raw: string) {
  if (!raw) {
    return null
  }

  const tokenPattern = /(\d+(?:\.\d+)?)(ms|s|m|h)/g
  const labels: string[] = []
  let lastIndex = 0

  for (const match of raw.matchAll(tokenPattern)) {
    const [token, amount, unit] = match
    if (match.index !== lastIndex) {
      return null
    }

    labels.push(`${amount} ${durationUnitLabel(unit)}`)
    lastIndex += token.length
  }

  if (lastIndex !== raw.length || labels.length === 0) {
    return null
  }

  return labels.join(' ')
}

function durationUnitLabel(unit: string) {
  return i18n.global.te(`display.durationUnits.${unit}`) ? t(`display.durationUnits.${unit}`) : unit
}
