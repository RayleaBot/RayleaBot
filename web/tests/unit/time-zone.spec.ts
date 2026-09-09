import { createPinia, setActivePinia } from 'pinia'
import { describe, expect, it } from 'vitest'
import catalog from '@/lib/time-zones.generated.json'
import { DEFAULT_TIME_ZONE, formatTimeZoneOffset, isSupportedTimeZone, timeZoneDateTimeToUtc, toTimeZoneDateTimeInput } from '@/lib/time-zone'
import { formatDateTime } from '@/lib/format'
import { useConfigStore } from '@/stores/config'

describe('management time zones', () => {
  it('includes the complete IANA catalog with translated cities and legacy identifiers', () => {
    expect(catalog.zones.length).toBeGreaterThan(500)
    expect(new Set(catalog.zones.map(zone => zone.id)).size).toBe(catalog.zones.length)
    expect(catalog.zones.find(zone => zone.id === 'Asia/Shanghai')?.city).toBe('上海')
    expect(catalog.zones.some(zone => zone.id === 'US/Eastern')).toBe(true)
    expect(catalog.zones.every(zone => isSupportedTimeZone(zone.id))).toBe(true)
  })

  it('uses date-specific daylight-saving offsets and supports fractional hours', () => {
    expect(formatTimeZoneOffset(DEFAULT_TIME_ZONE)).toBe('UTC+08:00')
    expect(formatTimeZoneOffset('Asia/Kathmandu')).toBe('UTC+05:45')
    expect(formatTimeZoneOffset('America/New_York', new Date('2026-01-15T00:00:00Z'))).toBe('UTC−05:00')
    expect(formatTimeZoneOffset('America/New_York', new Date('2026-07-15T00:00:00Z'))).toBe('UTC−04:00')
  })

  it('formats and parses calendar inputs independently of the browser time zone', () => {
    expect(toTimeZoneDateTimeInput(new Date('2026-01-15T20:30:00Z'), 'Asia/Shanghai')).toBe('2026-01-16T04:30')
    expect(timeZoneDateTimeToUtc('2026-01-16T04:30', 'Asia/Shanghai')).toBe('2026-01-15T20:30:00Z')
    expect(timeZoneDateTimeToUtc('2026-01-16T02:15', 'Asia/Kathmandu')).toBe('2026-01-15T20:30:00Z')
  })

  it('rejects nonexistent calendar times and includes both occurrences of repeated range boundaries', () => {
    expect(timeZoneDateTimeToUtc('2026-03-08T02:30', 'America/New_York')).toBe('')
    expect(timeZoneDateTimeToUtc('2026-02-30T12:00', 'Asia/Shanghai')).toBe('')
    expect(timeZoneDateTimeToUtc('2011-12-30T12:00', 'Pacific/Apia')).toBe('')
    expect(timeZoneDateTimeToUtc('2026-11-01T01:30', 'America/New_York', 'start')).toBe('2026-11-01T05:30:00Z')
    expect(timeZoneDateTimeToUtc('2026-11-01T01:30', 'America/New_York', 'end')).toBe('2026-11-01T06:30:00Z')
    expect(timeZoneDateTimeToUtc('2026-04-05T01:45', 'Australia/Lord_Howe', 'start')).toBe('2026-04-04T14:45:00Z')
    expect(timeZoneDateTimeToUtc('2026-04-05T01:45', 'Australia/Lord_Howe', 'end')).toBe('2026-04-04T15:15:00Z')
  })

  it('uses the effective server timezone instead of a draft awaiting restart', () => {
    setActivePinia(createPinia())
    const config = useConfigStore()
    config.effectiveTimezone = 'Asia/Shanghai'
    config.document = { scheduler: { timezone: 'America/New_York' } } as never
    expect(formatDateTime('2026-01-15T20:30:00Z')).toContain('04:30:00')
    config.effectiveTimezone = 'America/New_York'
    expect(formatDateTime('2026-01-15T20:30:00Z')).toContain('15:30:00')
  })
})
