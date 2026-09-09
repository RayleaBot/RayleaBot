export const DEFAULT_TIME_ZONE = 'Asia/Shanghai'

const dateFormatters = new Map<string, Intl.DateTimeFormat>()

function formatter(timeZone: string) {
  let value = dateFormatters.get(timeZone)
  if (!value) {
    value = new Intl.DateTimeFormat('en-CA', {
      timeZone, calendar: 'iso8601', numberingSystem: 'latn', hourCycle: 'h23',
      year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit',
    })
    dateFormatters.set(timeZone, value)
  }
  return value
}

function localParts(date: Date, timeZone: string) {
  const parts = Object.fromEntries(formatter(timeZone).formatToParts(date).map(part => [part.type, part.value]))
  return [parts.year, parts.month, parts.day, parts.hour, parts.minute, parts.second].map(Number)
}

export function isSupportedTimeZone(timeZone: string) {
  try { formatter(timeZone); return true } catch { return false }
}

export function timeZoneOffsetMinutes(timeZone: string, date = new Date()) {
  const [year, month, day, hour, minute, second] = localParts(date, timeZone)
  return Math.round((Date.UTC(year!, month! - 1, day, hour, minute, second) - Math.floor(date.getTime() / 1000) * 1000) / 60_000)
}

export function formatTimeZoneOffset(timeZone: string, date = new Date()) {
  const minutes = timeZoneOffsetMinutes(timeZone, date)
  const absolute = Math.abs(minutes)
  return `UTC${minutes < 0 ? '−' : '+'}${String(Math.floor(absolute / 60)).padStart(2, '0')}:${String(absolute % 60).padStart(2, '0')}`
}

export function toTimeZoneDateTimeInput(date: Date, timeZone: string) {
  const [year, month, day, hour, minute] = localParts(date, timeZone).map(value => String(value).padStart(2, '0'))
  return `${year}-${month}-${day}T${hour}:${minute}`
}

// A local wall-clock value may map to zero (spring gap) or two (autumn fold) instants.
export function timeZoneDateTimeToUtc(value: string, timeZone: string, boundary: 'start' | 'end' = 'start') {
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/.exec(value.trim())
  if (!match) return ''
  const expected = match.slice(1).map(part => Number(part ?? '0'))
  const [year, month, day, hour, minute, second] = expected
  const wallTime = Date.UTC(year!, month! - 1, day, hour, minute, second)
  const offsets = new Set<number>()
  for (let hours = -36; hours <= 36; hours += 12) {
    offsets.add(timeZoneOffsetMinutes(timeZone, new Date(wallTime + hours * 3_600_000)))
  }
  const candidates = [...offsets].map(offset => new Date(wallTime - offset * 60_000))
    .filter(date => localParts(date, timeZone).every((part, index) => part === expected[index]))
    .sort((a, b) => a.getTime() - b.getTime())
  const selected = boundary === 'end' ? candidates.at(-1) : candidates[0]
  return selected?.toISOString().replace(/\.\d{3}Z$/, 'Z') ?? ''
}
