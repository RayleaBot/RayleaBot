import { t } from '@/i18n'
import { ApiError } from '@/lib/http'

const errorMessageByKey: Record<string, string> = {
  'errors.permission.denied': t('errors.permission.denied'),
  'errors.platform.invalid_request': t('errors.platform.invalidRequest'),
  'errors.platform.not_found': t('errors.platform.notFound'),
  'errors.platform.resource_missing': t('errors.platform.resourceMissing'),
}

const errorMessageByCode: Record<string, string> = {
  'permission.denied': t('errors.permission.denied'),
  'platform.invalid_request': t('errors.platform.invalidRequest'),
  'platform.not_found': t('errors.platform.notFound'),
  'platform.resource_missing': t('errors.platform.resourceMissing'),
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
  }

  if (error instanceof Error && hasChineseText(error.message)) {
    return error.message
  }

  return t(fallbackKey)
}
