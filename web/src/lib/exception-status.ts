import { t } from '@/i18n'
import { ApiError } from '@/lib/http'

export type ExceptionStatus = '403' | '404' | '500' | 'offline'

export function resolveExceptionStatus(error: unknown, fallback: ExceptionStatus = '500'): ExceptionStatus {
  if (error instanceof ApiError) {
    if (error.status === 403 || error.code === 'permission.denied') {
      return '403'
    }

    if (error.status === 404 || error.code === 'platform.not_found') {
      return '404'
    }

    if (error.status >= 500) {
      return '500'
    }
  }

  if (error instanceof Error) {
    return resolveExceptionStatusFromText(error.message, fallback)
  }

  return fallback
}

export function resolveExceptionStatusFromText(value: string | null | undefined, fallback: ExceptionStatus = '500'): ExceptionStatus {
  const text = value?.trim()
  if (!text) {
    return fallback
  }

  if (text === t('errors.permission.denied')) {
    return '403'
  }

  if (text === t('errors.platform.notFound')) {
    return '404'
  }

  return fallback
}
