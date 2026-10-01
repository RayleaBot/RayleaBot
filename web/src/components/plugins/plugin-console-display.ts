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

// stderr is where a plugin writes its own log, ordinary lines included, so it is plain output like sent messages;
// only the platform's system notes carry a status tone.
export function getConsoleStreamTone(stream: ConsoleFrame['stream']): StatusTone {
  if (stream === 'system') return 'warning'
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

// The socket turns authenticated only on its first frame; an open stream that is still quiet is just as healthy.
export function getConsoleConnectionTone(status: string): StatusTone {
  if (status === 'authenticated' || status === 'connected') return 'success'
  if (status === 'reconnecting' || status === 'connecting') return 'warning'
  if (status === 'auth_failed') return 'danger'
  return 'neutral'
}
