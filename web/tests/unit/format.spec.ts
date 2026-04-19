import { afterEach, describe, expect, it, vi } from 'vitest'

import { formatDateTime, formatRateLimit, formatRelativeTime } from '@/lib/format'
import { i18n } from '@/i18n'

afterEach(() => {
  vi.useRealTimers()
})

describe('format helpers', () => {
  it('keeps invalid datetime values from throwing', () => {
    expect(formatDateTime('not-a-date')).toBe('not-a-date')
    expect(formatDateTime(undefined)).toBe('—')
    expect(formatDateTime(Number.MAX_SAFE_INTEGER)).toBe(String(Number.MAX_SAFE_INTEGER))
  })

  it('formats unix-second timestamps from numbers and scientific-notation strings', () => {
    const unixSeconds = 1.775762955e+09
    const expected = new Intl.DateTimeFormat(i18n.global.locale.value, {
      dateStyle: 'short',
      timeStyle: 'medium',
    }).format(new Date(unixSeconds * 1000))

    expect(formatDateTime(unixSeconds)).toBe(expected)
    expect(formatDateTime(String(unixSeconds))).toBe(expected)
  })

  it('keeps invalid relative time values readable', () => {
    expect(formatRelativeTime('not-a-date')).toBe('not-a-date')
    expect(formatRelativeTime(undefined)).toBe('—')
  })

  it('formats unix-second timestamps for relative time', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-04-10T03:30:00Z'))
    const thirtySecondsAgo = (Date.now() - 30_000) / 1000
    const scientificUnixSeconds = thirtySecondsAgo.toExponential()

    expect(formatRelativeTime(scientificUnixSeconds)).toBe('30 秒前')
    expect(formatRelativeTime(thirtySecondsAgo)).toBe('30 秒前')
  })

  it('formats rate limits into readable chinese text', () => {
    expect(formatRateLimit('10/60s')).toBe('60 秒内最多 10 次')
    expect(formatRateLimit('3/1h30m')).toBe('1 小时 30 分钟内最多 3 次')
    expect(formatRateLimit('')).toBe('—')
    expect(formatRateLimit('not-a-rate-limit')).toBe('not-a-rate-limit')
  })
})
