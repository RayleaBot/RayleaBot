import { i18n, t } from '@/i18n'
import { ApiError } from '@/lib/http'

function translateErrorCode(code: string | undefined) {
  if (!code) return undefined
  const localeKey = `errors.${code.replace(/_([a-z])/g, (_, letter: string) => letter.toUpperCase())}`
  return i18n.global.te(localeKey) ? t(localeKey) : undefined
}

export function getDisplayErrorMessage(error: unknown, fallbackKey = 'errors.common.actionFailed') {
  if (error instanceof ApiError) {
    const messageCode = error.messageKey?.startsWith('errors.') ? error.messageKey.slice(7) : undefined
    const message = translateErrorCode(messageCode) ?? translateErrorCode(error.code)
    if (message) return message
  }
  return t(fallbackKey)
}
