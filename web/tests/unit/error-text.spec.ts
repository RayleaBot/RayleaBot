import { describe, expect, it } from 'vitest'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { ApiError } from '@/lib/http'

describe('error text helpers', () => {
  it('prefers structured error keys when available', () => {
    const error = new ApiError(
      'invalid socket channel',
      400,
      'platform.invalid_request',
      'req_fixture',
      undefined,
      'errors.platform.invalid_request',
    )

    expect(getDisplayErrorMessage(error)).toBe('请求参数不正确，请检查后重试。')
  })

  it('falls back to a generic chinese message for unstructured errors', () => {
    expect(getDisplayErrorMessage(new Error('boom'))).toBe('操作未完成，请稍后重试。')
  })
})
