import { describe, expect, it } from 'vitest'

import { formatDateTime, formatRelativeTime } from '@/lib/format'

describe('format helpers', () => {
  it('keeps invalid datetime values from throwing', () => {
    expect(formatDateTime('not-a-date')).toBe('not-a-date')
    expect(formatDateTime(undefined)).toBe('—')
    expect(formatDateTime(Number.MAX_SAFE_INTEGER)).toBe(String(Number.MAX_SAFE_INTEGER))
  })

  it('keeps invalid relative time values readable', () => {
    expect(formatRelativeTime('not-a-date')).toBe('not-a-date')
    expect(formatRelativeTime(undefined)).toBe('—')
  })
})
