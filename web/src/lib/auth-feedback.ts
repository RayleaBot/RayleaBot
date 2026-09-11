import { t } from '@/i18n'
import { ApiError } from '@/lib/http'

function isNetworkError(error: unknown) {
  return error instanceof TypeError || (error instanceof Error && /fetch/i.test(error.message))
}

export function toLoginErrorMessage(error: unknown) {
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'permission.denied':
        return t('auth.feedback.loginDenied')
      case 'platform.invalid_request':
        return t('auth.feedback.loginInvalid')
      default:
        return t('auth.feedback.loginFailed')
    }
  }

  if (isNetworkError(error)) {
    return t('auth.feedback.serviceUnreachable')
  }

  return t('auth.feedback.loginFailed')
}

export function toSetupErrorMessage(error: unknown) {
  if (error instanceof ApiError) {
    switch (error.code) {
      case 'permission.denied':
        return t('auth.feedback.setupInitialized')
      case 'platform.invalid_request':
        return t('auth.feedback.setupInvalid')
      default:
        return t('auth.feedback.setupFailed')
    }
  }

  if (isNetworkError(error)) {
    return t('auth.feedback.serviceUnreachable')
  }

  return t('auth.feedback.setupFailed')
}

export function toBootstrapStatusMessage(error: unknown) {
  if (error instanceof ApiError) {
    if (error.code === 'permission.denied') {
      return t('auth.feedback.bootstrapUnknown')
    }

    if (error.code === 'platform.invalid_request') {
      return t('auth.feedback.bootstrapUnavailable')
    }

    return t('auth.feedback.bootstrapUnknown')
  }

  if (isNetworkError(error)) {
    return t('auth.feedback.serviceUnreachable')
  }

  return t('auth.feedback.bootstrapUnknown')
}
