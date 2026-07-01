import { describe, expect, it } from 'vitest'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { ApiError } from '@/lib/http'

function hasChineseText(value: string) {
  return /[\u3400-\u9fff]/.test(value)
}

describe('error text helpers', () => {
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

  it('uses a fallback for unstructured errors', () => {
    const result = getDisplayErrorMessage(new Error('boom'))

    expect(result).not.toBe('boom')
    expect(hasChineseText(result)).toBe(true)
  })
})
