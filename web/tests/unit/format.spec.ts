import { afterEach, describe, expect, it, vi } from 'vitest'

import { formatDurationSeconds, formatRateLimit, formatRelativeTime } from '@/lib/format'

afterEach(() => {
  vi.useRealTimers()
})

describe('format helpers', () => {
  it('keeps invalid relative time values readable', () => {
    expect(formatRelativeTime('not-a-date')).toBe('not-a-date')
    expect(formatRelativeTime(undefined)).toBe('—')
  })

  it('distinguishes future instants and rejects invalid durations', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-04-10T03:30:00Z'))
    expect(formatRelativeTime(Date.now() + 90_000)).toBe('1 分钟后')
    expect(formatRelativeTime(Date.now() - 90_000)).toBe('1 分钟前')
    for (const value of [Number.NaN, Number.POSITIVE_INFINITY, -1]) expect(formatDurationSeconds(value)).toBe('—')
    expect(formatDurationSeconds(0)).toBe('0 秒')
  })

  it('formats rate limits into readable chinese text', () => {
    expect(formatRateLimit('10/60s')).toBe('60 秒内最多 10 次')
    expect(formatRateLimit('3/1h')).toBe('1 小时内最多 3 次')
    expect(formatRateLimit('')).toBe('—')
    expect(formatRateLimit('not-a-rate-limit')).toBe('not-a-rate-limit')
  })
})
