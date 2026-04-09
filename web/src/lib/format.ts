import { i18n } from '@/i18n'
import { t } from '@/i18n'

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

  const date = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(date.getTime())) {
    return null
  }

  return date
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
