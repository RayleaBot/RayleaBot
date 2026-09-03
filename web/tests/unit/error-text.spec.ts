import { describe, expect, it } from 'vitest'

import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { ApiError } from '@/lib/http'

function hasChineseText(value: string) {
  return /[\u3400-\u9fff]/.test(value)
}

describe('error text helpers', () => {
  it.each([
    ['platform.task_queue_full', 'errors.platform.taskQueueFull'],
    ['plugin.install_failed', 'errors.plugin.installFailed'],
    ['plugin.install_inspection_required', 'errors.plugin.installInspectionRequired'],
    ['plugin.install_inspection_expired', 'errors.plugin.installInspectionExpired'],
    ['plugin.install_digest_mismatch', 'errors.plugin.installDigestMismatch'],
    ['plugin.trusted_code_confirmation_required', 'errors.plugin.trustedCodeConfirmationRequired'],
    ['plugin.package_resource_limit_exceeded', 'errors.plugin.packageResourceLimitExceeded'],
    ['plugin.package_unsafe_entry', 'errors.plugin.packageUnsafeEntry'],
    ['plugin.artifact_invalid', 'errors.plugin.artifactInvalid'],
    ['plugin.platform_mismatch', 'errors.plugin.platformMismatch'],
    ['plugin.store_catalog_unavailable', 'errors.plugin.storeCatalogUnavailable'],
    ['plugin.store_release_unavailable', 'errors.plugin.storeReleaseUnavailable'],
    ['plugin.store_integrity_mismatch', 'errors.plugin.storeIntegrityMismatch'],
  ])('preserves the recovery message for %s through its code or message key', (code, localeKey) => {
    const codeError = new ApiError('内部中文诊断', 409, code, undefined, { error: '详细诊断' })
    const keyError = new ApiError('内部中文诊断', 409, undefined, undefined, undefined, `errors.${code}`)

    expect(getDisplayErrorMessage(codeError)).toBe(t(localeKey))
    expect(getDisplayErrorMessage(keyError)).toBe(t(localeKey))
  })

  it('maps structured API errors without exposing raw backend text', () => {
    const error = new ApiError(
      'invalid socket channel',
      400,
      'platform.invalid_request',
      'req_fixture',
      undefined,
      'errors.platform.invalid_request',
    )

    const result = getDisplayErrorMessage(error)
    const fallback = getDisplayErrorMessage(new Error('boom'))

    expect(result).not.toBe('invalid socket channel')
    expect(result).not.toContain('platform.invalid_request')
    expect(result).not.toBe(fallback)
    expect(hasChineseText(result)).toBe(true)
  })

  it.each(['boom', '内部中文诊断'])('uses a fallback for unstructured errors: %s', (message) => {
    const result = getDisplayErrorMessage(new Error(message))

    expect(result).toBe(t('errors.common.actionFailed'))
    expect(hasChineseText(result)).toBe(true)
  })

  it('does not expose localized backend diagnostics from unknown structures', () => {
    const error = new ApiError(
      '内部中文诊断',
      500,
      'platform.unknown',
      'req_fixture_unknown',
      { error: 'details 中文诊断' },
      'errors.platform.unknown',
    )
    const fallback = getDisplayErrorMessage(new Error('boom'))
    const result = getDisplayErrorMessage(error)

    expect(result).toBe(fallback)
    expect(result).not.toContain('内部中文诊断')
    expect(result).not.toContain('details 中文诊断')
  })
})
