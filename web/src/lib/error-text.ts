import { i18n, t } from '@/i18n'
import { ApiError } from '@/lib/http'

// Browser-side failures have no server error code; each client code maps to its own message.
const clientErrorKeys: Record<string, string> = {
  'client.request_cancelled': 'errors.common.requestCancelled',
  'client.request_timeout': 'errors.common.requestTimeout',
  'client.task_cancelled': 'errors.common.taskCancelled',
  'client.task_interrupted': 'errors.common.taskInterrupted',
  'client.task_still_running': 'errors.common.taskStillRunning',
}

function translateErrorCode(code: string | undefined) {
  if (!code) return undefined
  const localeKey = `errors.${code}`
  return i18n.global.te(localeKey) ? t(localeKey) : undefined
}

export function getErrorCodeMessage(code: string | undefined, fallbackKey = 'errors.common.actionFailed') {
  return translateErrorCode(code) ?? t(fallbackKey)
}

export function getDisplayErrorMessage(error: unknown, fallbackKey = 'errors.common.actionFailed') {
  if (error instanceof ApiError) {
    const clientKey = clientErrorKeys[error.code]
    if (clientKey) return t(clientKey)
    const message = translateErrorCode(error.code)
    if (message) return message
  }
  return t(fallbackKey)
}
