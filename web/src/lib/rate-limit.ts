export type RateLimitUnit = 's' | 'm' | 'h'

export interface RateLimitParts {
  count: number
  unit: RateLimitUnit
  windowValue: number
}

export function parseRateLimitValue(value?: string | null): RateLimitParts | null {
  const trimmed = value?.trim()
  if (!trimmed) {
    return null
  }

  const match = /^([1-9][0-9]*)\/([1-9][0-9]*)(s|m|h)$/.exec(trimmed)
  if (!match) {
    return null
  }

  return {
    count: Number.parseInt(match[1], 10),
    windowValue: Number.parseInt(match[2], 10),
    unit: match[3] as RateLimitUnit,
  }
}

export function buildRateLimitValue(parts: Partial<RateLimitParts>): string | null {
  const count = normalizePositiveInteger(parts.count)
  const windowValue = normalizePositiveInteger(parts.windowValue)
  const unit = parts.unit

  if (!count || !windowValue || !unit) {
    return null
  }

  return `${count}/${windowValue}${unit}`
}

export function normalizePositiveInteger(value: unknown): number | null {
  if (value === null || value === undefined || value === '') {
    return null
  }

  const nextNumber = Number(value)
  if (!Number.isInteger(nextNumber) || nextNumber <= 0) {
    return null
  }

  return nextNumber
}
