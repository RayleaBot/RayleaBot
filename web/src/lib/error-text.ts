import { t } from '@/i18n'
import { ApiError } from '@/lib/http'

const errorMessageByKey: Record<string, string> = {
  'errors.permission.denied': t('errors.permission.denied'),
  'errors.platform.invalid_request': t('errors.platform.invalidRequest'),
  'errors.platform.not_found': t('errors.platform.notFound'),
  'errors.platform.resource_missing': t('errors.platform.resourceMissing'),
  'errors.platform.template_not_found': t('errors.platform.templateNotFound'),
  'errors.platform.template_source_invalid': t('errors.platform.templateSourceInvalid'),
  'errors.platform.template_revision_conflict': t('errors.platform.templateRevisionConflict'),
  'errors.platform.template_revision_not_found': t('errors.platform.templateRevisionNotFound'),
  'errors.platform.template_rollback_target_invalid': t('errors.platform.templateRollbackTargetInvalid'),
}

const errorMessageByCode: Record<string, string> = {
  'permission.denied': t('errors.permission.denied'),
  'platform.invalid_request': t('errors.platform.invalidRequest'),
  'platform.not_found': t('errors.platform.notFound'),
  'platform.resource_missing': t('errors.platform.resourceMissing'),
  'platform.template_not_found': t('errors.platform.templateNotFound'),
  'platform.template_source_invalid': t('errors.platform.templateSourceInvalid'),
  'platform.template_revision_conflict': t('errors.platform.templateRevisionConflict'),
  'platform.template_revision_not_found': t('errors.platform.templateRevisionNotFound'),
  'platform.template_rollback_target_invalid': t('errors.platform.templateRollbackTargetInvalid'),
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
