import { describe, expect, it } from 'vitest'

import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { ApiError } from '@/lib/http'

function hasChineseText(value: string) {
  return /[\u3400-\u9fff]/.test(value)
}

describe('error text helpers', () => {
  it('says why an install was refused for the RayleaBot version from the structured reason', () => {
    const unknown = new ApiError('无法确认', 409, 'plugin.core_version_incompatible', undefined, { incompatible_reason: 'core_version_unknown', min_core_version: '0.4.0' })
    const tooOld = new ApiError('版本过旧', 409, 'plugin.core_version_incompatible', undefined, { incompatible_reason: 'core_version_too_old', min_core_version: '0.9.0' })
    const bare = new ApiError('不兼容', 409, 'plugin.core_version_incompatible')

    expect(getDisplayErrorMessage(unknown)).toBe(t('errors.coreVersion.unknown'))
    expect(getDisplayErrorMessage(tooOld)).toBe(t('errors.coreVersion.tooOld', { version: 'v0.9.0' }))
    expect(getDisplayErrorMessage(bare)).toBe(t('errors.plugin.core_version_incompatible'))
  })

  it('names the missing prerequisite plugins of a refused install', () => {
    const error = new ApiError('缺少前置插件', 409, 'plugin.dependency_missing', undefined, { plugin_ids: ['raylea.mihoyo-accounts', 'raylea.panel-assets'] })

    expect(getDisplayErrorMessage(error)).toBe(t('errors.dependencyMissing', { plugins: 'raylea.mihoyo-accounts、raylea.panel-assets' }))
  })

  it('maps structured API errors without exposing raw backend text', () => {
    const error = new ApiError(
      'invalid socket channel',
      400,
      'platform.invalid_request',
      'req_fixture',
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
    )
    const fallback = getDisplayErrorMessage(new Error('boom'))
    const result = getDisplayErrorMessage(error)

    expect(result).toBe(fallback)
    expect(result).not.toContain('内部中文诊断')
    expect(result).not.toContain('details 中文诊断')
  })
})
