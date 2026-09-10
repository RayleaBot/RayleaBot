import { i18n, t } from '@/i18n'
import { ApiError } from '@/lib/http'
import { errorCatalog } from '@/types/error-codes.generated'

function translateErrorCode(code: string | undefined) {
  if (!code) return undefined
  const entry = Object.hasOwn(errorCatalog, code) ? errorCatalog[code as keyof typeof errorCatalog] : undefined
  if (!entry) return undefined
  const localeKey = entry.messageKey
  return i18n.global.te(localeKey) ? t(localeKey) : undefined
}

export function getDisplayErrorMessage(error: unknown, fallbackKey = 'errors.common.actionFailed') {
  if (error instanceof ApiError) {
    if (error.code === 'client.request_cancelled') return t('errors.common.requestCancelled')
    if (error.code === 'client.request_timeout') return t('errors.common.requestTimeout')
    const messageCode = error.messageKey?.startsWith('errors.') ? error.messageKey.slice(7) : undefined
    const message = translateErrorCode(messageCode) ?? translateErrorCode(error.code)
    if (message) return message
  }
  return t(fallbackKey)
}
