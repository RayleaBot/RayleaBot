import { t } from '@/i18n'
import { ApiError } from '@/lib/http'

const errorMessageByKey: Record<string, string> = {
  'errors.plugin.event_canceled': t('errors.plugin.eventCanceled'),
  'errors.adapter.send_unconfirmed': t('errors.adapter.sendUnconfirmed'),
  'errors.permission.denied': t('errors.permission.denied'),
  'errors.permission.blacklisted': t('errors.permission.blacklisted'),
  'errors.permission.not_whitelisted': t('errors.permission.notWhitelisted'),
  'errors.platform.invalid_request': t('errors.platform.invalidRequest'),
  'errors.platform.third_party_account_not_found': t('errors.platform.thirdPartyAccountNotFound'),
  'errors.platform.resource_missing': t('errors.platform.resourceMissing'),
  'errors.platform.template_not_found': t('errors.platform.templateNotFound'),
  'errors.platform.task_queue_full': t('errors.platform.taskQueueFull'),
  'errors.plugin.install_failed': t('errors.plugin.installFailed'),
  'errors.plugin.install_inspection_required': t('errors.plugin.installInspectionRequired'),
  'errors.plugin.install_inspection_expired': t('errors.plugin.installInspectionExpired'),
  'errors.plugin.install_digest_mismatch': t('errors.plugin.installDigestMismatch'),
  'errors.plugin.trusted_code_confirmation_required': t('errors.plugin.trustedCodeConfirmationRequired'),
  'errors.plugin.package_resource_limit_exceeded': t('errors.plugin.packageResourceLimitExceeded'),
  'errors.plugin.package_unsafe_entry': t('errors.plugin.packageUnsafeEntry'),
  'errors.plugin.artifact_invalid': t('errors.plugin.artifactInvalid'),
  'errors.plugin.platform_mismatch': t('errors.plugin.platformMismatch'),
  'errors.plugin.store_catalog_unavailable': t('errors.plugin.storeCatalogUnavailable'),
  'errors.plugin.store_release_unavailable': t('errors.plugin.storeReleaseUnavailable'),
  'errors.plugin.store_integrity_mismatch': t('errors.plugin.storeIntegrityMismatch'),
}

const errorMessageByCode: Record<string, string> = {
  'plugin.event_canceled': t('errors.plugin.eventCanceled'),
  'adapter.send_unconfirmed': t('errors.adapter.sendUnconfirmed'),
  'permission.denied': t('errors.permission.denied'),
  'permission.blacklisted': t('errors.permission.blacklisted'),
  'permission.not_whitelisted': t('errors.permission.notWhitelisted'),
  'platform.invalid_request': t('errors.platform.invalidRequest'),
  'platform.third_party_account_not_found': t('errors.platform.thirdPartyAccountNotFound'),
  'platform.resource_missing': t('errors.platform.resourceMissing'),
  'platform.template_not_found': t('errors.platform.templateNotFound'),
  'platform.task_queue_full': t('errors.platform.taskQueueFull'),
  'plugin.install_failed': t('errors.plugin.installFailed'),
  'plugin.install_inspection_required': t('errors.plugin.installInspectionRequired'),
  'plugin.install_inspection_expired': t('errors.plugin.installInspectionExpired'),
  'plugin.install_digest_mismatch': t('errors.plugin.installDigestMismatch'),
  'plugin.trusted_code_confirmation_required': t('errors.plugin.trustedCodeConfirmationRequired'),
  'plugin.package_resource_limit_exceeded': t('errors.plugin.packageResourceLimitExceeded'),
  'plugin.package_unsafe_entry': t('errors.plugin.packageUnsafeEntry'),
  'plugin.artifact_invalid': t('errors.plugin.artifactInvalid'),
  'plugin.platform_mismatch': t('errors.plugin.platformMismatch'),
  'plugin.store_catalog_unavailable': t('errors.plugin.storeCatalogUnavailable'),
  'plugin.store_release_unavailable': t('errors.plugin.storeReleaseUnavailable'),
  'plugin.store_integrity_mismatch': t('errors.plugin.storeIntegrityMismatch'),
}

export function getDisplayErrorMessage(error: unknown, fallbackKey = 'errors.common.actionFailed') {
  if (error instanceof ApiError) {
    if (error.messageKey && errorMessageByKey[error.messageKey]) {
      return errorMessageByKey[error.messageKey]
    }

    if (error.code && errorMessageByCode[error.code]) {
      return errorMessageByCode[error.code]
    }
  }

  return t(fallbackKey)
}
