import { describe, expect, it } from 'vitest'

import { formatCronSchedule, formatNextRunRelative } from '@/lib/scheduler-job-display'

describe('scheduler job display', () => {
  it.each([
    ['* * * * *', '每分钟'],
    ['*/5 * * * *', '每 5 分钟'],
    ['0 */2 * * *', '每 2 小时整'],
    ['5 8 * * *', '每天 08:05'],
  ])('describes the supported schedule %s', (cron, expected) => {
    expect(formatCronSchedule(cron)).toBe(expected)
  })

  it.each([
    '0 8,12 * * *', '0 8-12 * * *', '0 8 * * MON',
    '0 0 8 * * *', '60 8 * * *', '0 24 * * *', '*/0 * * * *', '*/60 * * * *',
  ])('preserves the original expression when no accurate description is available: %s', (cron) => {
    expect(formatCronSchedule(cron)).toBe(cron)
  })

  it('uses the supplied clock and omits an invalid next run time', () => {
    const now = Date.parse('2026-09-23T12:00:00Z')
    expect(formatNextRunRelative('2026-09-23T12:00:25Z', now)).toBe('25 秒后')
    expect(formatNextRunRelative('2026-09-23T12:00:25Z', now + 10_000)).toBe('15 秒后')
    expect(formatNextRunRelative('2026-09-23T12:00:25Z', now + 30_000)).toBe('即将执行')
    expect(formatNextRunRelative('invalid', now)).toBe('')
    expect(formatNextRunRelative(undefined, now)).toBe('')
  })
})
