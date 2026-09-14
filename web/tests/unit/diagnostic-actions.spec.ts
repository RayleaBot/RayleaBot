import { describe, expect, it } from 'vitest'
import { ApiError } from '@/lib/http'
import { resolveExceptionStatus } from '@/lib/exception-status'
import { getPluginTrustLabel } from '@/lib/display'

describe('machine-readable management decisions', () => {
  it('resolves failure illustrations from codes and HTTP status only', () => {
    for (const message of ['无权访问', 'Not found', 'Internal server error']) {
      expect(resolveExceptionStatus(new ApiError(message, 404, 'platform.resource_not_found'))).toBe('404')
      expect(resolveExceptionStatus(new ApiError(message, 403, 'permission.denied'))).toBe('403')
      expect(resolveExceptionStatus(new Error(message))).toBe('500')
    }
  })

  it('covers every trust level and degrades unknown display values without granting official status', () => {
    expect(['official', 'development', 'third_party', 'unverified', 'future'].map(getPluginTrustLabel))
      .toEqual(['官方', '开发中', '第三方', '未验证来源', '来源未知'])
  })
})
