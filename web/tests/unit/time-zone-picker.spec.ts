import { describe, expect, it } from 'vitest'
import { PICKER_TIME_ZONE_IDS, getTimeZonePickerChoices } from '@/lib/time-zone-picker'
import { isSupportedTimeZone, timeZoneOffsetMinutes } from '@/lib/time-zone'

describe('representative timezone choices', () => {
  it('offers Windows regional representatives and familiar cities without legacy duplicates', () => {
    const choices = getTimeZonePickerChoices()
    const ids = choices.map(zone => zone.id)
    expect(new Set(ids).size).toBe(PICKER_TIME_ZONE_IDS.length)
    expect(ids).toEqual(expect.arrayContaining(['Asia/Shanghai', 'Asia/Hong_Kong', 'America/New_York', 'Europe/Paris', 'Asia/Kathmandu', 'UTC', 'Etc/GMT+12', 'Pacific/Kiritimati', 'Asia/Kabul', 'Australia/Eucla', 'Pacific/Chatham', 'Pacific/Marquesas', 'Atlantic/Cape_Verde', 'Atlantic/Reykjavik']))
    for (const id of ['Asia/Katmandu', 'Asia/Calcutta', 'Europe/Kiev', 'US/Eastern', 'Antarctica/Casey', 'Etc/GMT-8']) {
      expect(ids).not.toContain(id)
    }
    expect(choices.every(zone => isSupportedTimeZone(zone.id))).toBe(true)
  })

  it('covers UTC -12 through +14 and fractional offsets across both seasons', () => {
    const offsets = new Set(['2026-01-15T12:00:00Z', '2026-07-15T12:00:00Z'].flatMap(date =>
      getTimeZonePickerChoices().map(zone => timeZoneOffsetMinutes(zone.id, new Date(date))),
    ))
    const wholeHours = Array.from({ length: 27 }, (_, hour) => (hour - 12) * 60)
    for (const minutes of [...wholeHours, -570, -210, 210, 270, 330, 345, 390, 525, 570, 630, 765, 825]) {
      expect(offsets.has(minutes), `Missing UTC offset ${minutes} minutes`).toBe(true)
    }
  })

  it.each(['Asia/Katmandu', 'Antarctica/Casey', 'Etc/GMT-8'])('preserves the exact configured value %s outside the common list', id => {
    const choices = getTimeZonePickerChoices(id)
    expect(choices).toHaveLength(PICKER_TIME_ZONE_IDS.length + 1)
    expect(choices.find(zone => zone.id === id)?.retained).toBe(true)
    expect(choices.filter(zone => zone.retained)).toHaveLength(1)
  })
})
