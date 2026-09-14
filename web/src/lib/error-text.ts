import { i18n, t } from '@/i18n'
import { ApiError } from '@/lib/http'

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
    if (error.code === 'client.request_cancelled') return t('errors.common.requestCancelled')
    if (error.code === 'client.request_timeout') return t('errors.common.requestTimeout')
    const message = translateErrorCode(error.code)
    if (message) return message
  }
  return t(fallbackKey)
}
