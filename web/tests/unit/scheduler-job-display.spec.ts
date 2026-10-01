import { describe, expect, it } from 'vitest'

import { conversationText, errorLabel, formatCronSchedule, formatNextRunRelative, isLatestRunFailed } from '@/lib/scheduler-job-display'
import type { SchedulerJobSummary } from '@/types/api'

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

  // The server keeps the last error after later runs succeed, so only an error from the last run describes the job.
  it('tells an error of the last run from an earlier one', () => {
    const job = (lastRun: string | null, errorAt: string) => ({
      last_run: lastRun,
      last_error: { code: 'plugin.event_timeout', message: 'timed out', at: errorAt },
    }) as SchedulerJobSummary
    expect(isLatestRunFailed(job('2026-09-23T08:00:00Z', '2026-09-23T08:00:00Z'))).toBe(true)
    expect(isLatestRunFailed(job('2026-09-24T08:00:00Z', '2026-09-23T08:00:00Z'))).toBe(false)
    expect(errorLabel('plugin.event_timeout')).toBe('插件事件处理超时')
    expect(errorLabel('timeout')).toBe('执行超时')
  })

  // Older records may hold a run cancelled by a stop as the last error; a cancelled run did not fail.
  it('does not read a cancelled last run as a failure', () => {
    const job = {
      last_run: '2026-09-23T08:00:00Z',
      last_error: { code: 'plugin.event_canceled', message: 'canceled', at: '2026-09-23T08:00:00Z' },
    } as SchedulerJobSummary
    expect(isLatestRunFailed(job)).toBe(false)
  })

  it('names the chat a job targets', () => {
    const job = (payload: Partial<SchedulerJobSummary['payload_summary']>) => ({
      payload_summary: { conversation_id: '', target_type: '', target_id: '', content: '', ...payload },
    }) as SchedulerJobSummary
    expect(conversationText(job({ target_type: 'group', target_id: '20001' }))).toBe('群 20001')
    expect(conversationText(job({ conversation_id: 'private:10001' }))).toBe('私聊 10001')
    expect(conversationText(job({ conversation_id: 'channel:abc' }))).toBe('channel:abc')
    expect(conversationText(job({}))).toBe('')
  })
})
