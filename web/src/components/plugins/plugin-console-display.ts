import { t } from '@/i18n'
import type { ConsoleFrame } from '@/stores/plugin-console'
import type { StatusTone } from '@/lib/status-tone'

export function getConsoleFrameKey(frame: ConsoleFrame, index: number) {
  if (frame.stream === 'outbound') {
    return frame.log_id
  }
  return `${frame.plugin_id}-${frame.stream}-${frame.timestamp}-${index}`
}

export function getConsoleLevel(frame: ConsoleFrame) {
  return frame.stream === 'outbound' ? frame.level : ''
}

export function getConsoleRequestId(frame: ConsoleFrame) {
  return frame.stream === 'outbound' ? frame.request_id ?? '' : ''
}

export function getConsoleStreamLabel(stream: ConsoleFrame['stream']) {
  return t(`plugins.console.streams.${stream}`)
}

export function getConsoleStreamTone(stream: ConsoleFrame['stream']): StatusTone {
  if (stream === 'stderr') return 'danger'
  if (stream === 'system') return 'warning'
  if (stream === 'outbound') return 'info'
  return 'neutral'
}

export function getConsoleLevelLabel(level: string) {
  if (level === 'debug' || level === 'info' || level === 'warn' || level === 'error') {
    return t(`plugins.console.levels.${level}`)
  }

  return level || t('display.empty')
}

export function getConsoleLevelTone(level: string): StatusTone {
  if (level === 'error') return 'danger'
  if (level === 'warn') return 'warning'
  if (level === 'info') return 'info'
  return 'neutral'
}

export function getConsoleConnectionTone(status: string): StatusTone {
  if (status === 'authenticated') return 'success'
  if (status === 'reconnecting' || status === 'connecting') return 'warning'
  if (status === 'auth_failed') return 'danger'
  return 'neutral'
}
