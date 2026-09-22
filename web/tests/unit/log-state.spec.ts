import { describe, expect, it } from 'vitest'
import { canAppendInPlace, mergeSortedLogItemsAsc, sortLogItemsAsc } from '@/stores/log-state'
import type { LogSummary } from '@/types/api'

function log(logID: string, timestamp: string): LogSummary {
  return { log_id: logID, timestamp, level: 'info', source: 'fixture', message: 'fixture' }
}

describe('log timestamp ordering', () => {
  it('sorts UTC and historical offsets without losing submillisecond precision', () => {
    const earlier = log('z-earlier', '2026-09-22T08:00:00.000000001+08:00')
    const later = log('a-later', '2026-09-22T00:00:00.000000002Z')
    const whole = log('whole', '2026-09-22T00:00:00Z')
    expect(sortLogItemsAsc([later, earlier, whole])).toEqual([whole, earlier, later])
    expect(mergeSortedLogItemsAsc([whole, later], [earlier])).toEqual([whole, earlier, later])
    expect(canAppendInPlace([later], earlier)).toBe(false)
    expect(canAppendInPlace([earlier], later)).toBe(true)
  })

  it('uses identity only for the same instant and keeps repeated daylight-saving hours distinct', () => {
    const first = log('b-first', '2026-11-01T01:30:00.1-04:00')
    const same = log('a-same', '2026-11-01T05:30:00.100000000Z')
    const repeated = log('a-repeated', '2026-11-01T01:30:00.1-05:00')
    expect(sortLogItemsAsc([repeated, first, same])).toEqual([same, first, repeated])
  })
})
