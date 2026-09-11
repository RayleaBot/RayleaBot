// Timestamp normalization shared by display and stream ordering. Invalid input
// remains invalid so each caller can apply its own display or ordering fallback.
export function timestampMilliseconds(value?: string | number | Date | null): number | null {
  if (value === undefined || value === null || value === '') return null
  if (value instanceof Date) return Number.isFinite(value.getTime()) ? value.getTime() : null
  const text = typeof value === 'string' ? value.trim() : undefined
  if (text === '') return null
  const numeric = typeof value === 'number' ? value
    : text && /^[+-]?(?:\d+\.?\d*|\d*\.?\d+)(?:e[+-]?\d+)?$/i.test(text) ? Number(text) : undefined
  if (numeric !== undefined) {
    if (!Number.isFinite(numeric)) return null
    const absolute = Math.abs(numeric)
    if (absolute < 1_000_000_000 || absolute > 8_640_000_000_000_000) return null
    const milliseconds = absolute < 1_000_000_000_000 ? numeric * 1000 : numeric
    return Number.isFinite(new Date(milliseconds).getTime()) ? milliseconds : null
  }
  const parsed = Date.parse(text!)
  return Number.isFinite(parsed) ? parsed : null
}
