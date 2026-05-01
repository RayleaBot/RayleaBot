import { describe, expect, it } from 'vitest'

import { buildRateLimitValue, normalizePositiveInteger, parseRateLimitValue } from '@/lib/rate-limit'

describe('rate limit form helpers', () => {
  it('parses the persisted rate limit string into split input parts', () => {
    expect(parseRateLimitValue('200/10s')).toEqual({
      count: 200,
      windowValue: 10,
      unit: 's',
    })
  })

  it('builds the persisted rate limit string from split input parts', () => {
    expect(buildRateLimitValue({ count: 20, windowValue: 1, unit: 'm' })).toBe('20/1m')
    expect(buildRateLimitValue({ count: 0, windowValue: 1, unit: 'm' })).toBeNull()
    expect(buildRateLimitValue({ count: 20, windowValue: undefined, unit: 'm' })).toBeNull()
  })

  it('normalizes only positive integers', () => {
    expect(normalizePositiveInteger('10')).toBe(10)
    expect(normalizePositiveInteger(3.5)).toBeNull()
    expect(normalizePositiveInteger('0')).toBeNull()
    expect(normalizePositiveInteger('')).toBeNull()
  })
})
