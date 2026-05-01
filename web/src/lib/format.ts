import { i18n } from '@/i18n'
import { t } from '@/i18n'
import { parseRateLimitValue } from '@/lib/rate-limit'

export function formatDateTime(value?: string | number | Date | null) {
  const date = toValidDate(value)
  if (!date) {
    return formatFallbackValue(value)
  }

  return new Intl.DateTimeFormat(i18n.global.locale.value, {
    dateStyle: 'short',
    timeStyle: 'medium',
  }).format(date)
}

export function formatRelativeTime(value?: string | number | Date | null): string {
  const date = toValidDate(value)
  if (!date) {
    return formatFallbackValue(value)
  }

  const now = Date.now()
  const diffMs = now - date.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  const diffMin = Math.floor(diffSec / 60)
  const diffHour = Math.floor(diffMin / 60)
  const diffDay = Math.floor(diffHour / 24)

  if (diffSec < 60) {
    return `${diffSec} 秒前`
  }
  if (diffMin < 60) {
    return `${diffMin} 分钟前`
  }
  if (diffHour < 24) {
    return `${diffHour} 小时前`
  }
  return `${diffDay} 天前`
}

export function formatDurationSeconds(seconds?: number) {
  if (!seconds && seconds !== 0) {
    return t('display.empty')
  }

  if (seconds < 60) {
    return `${seconds} 秒`
  }

  if (seconds < 3600) {
    return `${Math.floor(seconds / 60)} 分钟 ${seconds % 60} 秒`
  }

  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  return `${hours} 小时 ${minutes} 分钟`
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

  return `${parsed.windowLabel}内最多 ${parsed.count} 次`
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
  if (value === undefined || value === null || value === '') {
    return null
  }

  if (value instanceof Date) {
    return Number.isNaN(value.getTime()) ? null : value
  }

  if (typeof value === 'number') {
    return toDateFromTimestampNumber(value)
  }

  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (!trimmed) {
      return null
    }

    if (isNumericTimestampString(trimmed)) {
      const timestamp = Number(trimmed)
      const date = toDateFromTimestampNumber(timestamp)
      if (date) {
        return date
      }
    }

    const date = new Date(trimmed)
    return Number.isNaN(date.getTime()) ? null : date
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return null
  }

  return date
}

function toDateFromTimestampNumber(value: number) {
  if (!Number.isFinite(value)) {
    return null
  }

  const normalized = normalizeUnixTimestamp(value)
  if (normalized === null) {
    return null
  }

  const date = new Date(normalized)
  return Number.isNaN(date.getTime()) ? null : date
}

function normalizeUnixTimestamp(value: number) {
  const absolute = Math.abs(value)
  if (absolute >= 1_000_000_000 && absolute < 1_000_000_000_000) {
    return value * 1000
  }
  if (absolute >= 1_000_000_000_000 && absolute <= 8_640_000_000_000_000) {
    return value
  }
  return null
}

function isNumericTimestampString(value: string) {
  return /^[+-]?(?:\d+\.?\d*|\d*\.?\d+)(?:e[+-]?\d+)?$/i.test(value)
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
  switch (unit) {
    case 'ms':
      return '毫秒'
    case 's':
      return '秒'
    case 'm':
      return '分钟'
    case 'h':
      return '小时'
    default:
      return unit
  }
}
