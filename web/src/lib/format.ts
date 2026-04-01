import { i18n } from '@/i18n'
import { t } from '@/i18n'

export function formatDateTime(value?: string) {
  if (!value) {
    return t('display.empty')
  }

  return new Intl.DateTimeFormat(i18n.global.locale.value, {
    dateStyle: 'short',
    timeStyle: 'medium',
  }).format(new Date(value))
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
