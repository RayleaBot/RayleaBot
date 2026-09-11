import { describe, expect, it } from 'vitest'
import { timestampMilliseconds } from '@/lib/timestamp'
import { formatDateTime } from '@/lib/format'

describe('shared timestamp normalization', () => {
  it('normalizes every supported representation to the same instant', () => {
    const milliseconds = Date.parse('2026-04-10T03:30:00Z')
    for (const value of [milliseconds, milliseconds / 1000, String(milliseconds), (milliseconds / 1000).toExponential(), '2026-04-10T03:30:00Z', new Date(milliseconds)]) {
      expect(timestampMilliseconds(value)).toBe(milliseconds)
    }
  })
  it('keeps numeric outliers invalid instead of parsing them as calendar dates', () => {
    for (const value of ['7', '2026', '1e999', '', '   ', NaN, Infinity, new Date(NaN), Number.MAX_SAFE_INTEGER]) {
      expect(timestampMilliseconds(value)).toBeNull()
    }
    expect(formatDateTime('7')).toBe('7')
  })
})
