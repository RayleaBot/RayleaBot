import { t } from '@/i18n'
import { ApiError } from '@/lib/http'

const errorMessageByKey: Record<string, string> = {
  'errors.permission.denied': t('errors.permission.denied'),
  'errors.permission.blacklisted': t('errors.permission.blacklisted'),
  'errors.permission.not_whitelisted': t('errors.permission.notWhitelisted'),
  'errors.platform.invalid_request': t('errors.platform.invalidRequest'),
  'errors.platform.not_found': t('errors.platform.notFound'),
  'errors.platform.resource_missing': t('errors.platform.resourceMissing'),
  'errors.platform.template_not_found': t('errors.platform.templateNotFound'),
}

const errorMessageByCode: Record<string, string> = {
  'permission.denied': t('errors.permission.denied'),
  'permission.blacklisted': t('errors.permission.blacklisted'),
  'permission.not_whitelisted': t('errors.permission.notWhitelisted'),
  'platform.invalid_request': t('errors.platform.invalidRequest'),
  'platform.not_found': t('errors.platform.notFound'),
  'platform.resource_missing': t('errors.platform.resourceMissing'),
  'platform.template_not_found': t('errors.platform.templateNotFound'),
}

function hasChineseText(value: string) {
  return /[\u3400-\u9fff]/.test(value)
}

export function getDisplayErrorMessage(error: unknown, fallbackKey = 'errors.common.actionFailed') {
  if (error instanceof ApiError) {
    if (error.messageKey && errorMessageByKey[error.messageKey]) {
      return errorMessageByKey[error.messageKey]
    }

    if (error.code && errorMessageByCode[error.code]) {
      return errorMessageByCode[error.code]
    }

    if (typeof error.message === 'string' && hasChineseText(error.message)) {
      return error.message
    }

    // Fallback: show the backend diagnostic error from details if available.
    if (error.details && typeof error.details === 'object' && 'error' in error.details) {
      const detailError = (error.details as Record<string, unknown>).error
      if (typeof detailError === 'string' && detailError.trim()) {
        return detailError.trim()
      }
    }
  }

  if (error instanceof Error && hasChineseText(error.message)) {
    return error.message
  }

  return t(fallbackKey)
}
