import { timeZoneDateTimeToUtc, toTimeZoneDateTimeInput } from '@/lib/time-zone'
import type { AdapterDescriptor, MessageStatsConnection, MessageStatsGranularity, MessageStatsResponse } from '@/types/api'
import { toRowTone, type StatusRowTone } from '@/views/dashboard/dashboard-status'

// ---------- periods ----------

export type MessagePeriodPreset = 'today' | 'week' | 'month'
// Custom periods are inclusive local dates (YYYY-MM-DD) in the management time zone.
export type MessagePeriod = { kind: 'preset'; preset: MessagePeriodPreset } | { kind: 'custom'; from: string; to: string }
export interface MessageWindow { startAt: string; endAt: string; granularity: MessageStatsGranularity }

// Up to three days read better by the hour; longer ranges by the day. The server allows 732 day buckets.
export const HOURLY_CUSTOM_DAYS = 3
export const MAX_CUSTOM_DAYS = 732

export function localDate(date: Date, timeZone: string) {
  return toTimeZoneDateTimeInput(date, timeZone).slice(0, 10)
}

export function shiftDate(date: string, days: number) {
  const [year, month, day] = date.split('-').map(Number)
  return new Date(Date.UTC(year!, month! - 1, day! + days)).toISOString().slice(0, 10)
}

export function daysBetween(from: string, to: string) {
  return Math.round((Date.parse(`${to}T00:00:00Z`) - Date.parse(`${from}T00:00:00Z`)) / 86_400_000)
}

// The first instant of a local date; zones that skip midnight for daylight saving start the day at 01:00.
export function localMidnight(date: string, timeZone: string) {
  return timeZoneDateTimeToUtc(`${date}T00:00`, timeZone) || timeZoneDateTimeToUtc(`${date}T01:00`, timeZone)
}

// Presets are calendar windows that end with today; the server counts up to now and compares with the window of the
// same length right before it.
export function periodWindow(period: MessagePeriod, now: Date, timeZone: string): MessageWindow {
  if (period.kind === 'custom') {
    const days = daysBetween(period.from, period.to) + 1
    return {
      startAt: localMidnight(period.from, timeZone),
      endAt: localMidnight(shiftDate(period.to, 1), timeZone),
      granularity: days <= HOURLY_CUSTOM_DAYS ? 'hour' : 'day',
    }
  }
  const today = localDate(now, timeZone)
  const firstDay = { today, week: shiftDate(today, -6), month: shiftDate(today, -29) }[period.preset]
  return {
    startAt: localMidnight(firstDay, timeZone),
    endAt: localMidnight(shiftDate(today, 1), timeZone),
    granularity: period.preset === 'month' ? 'day' : 'hour',
  }
}

// ---------- connections ----------

export const CONNECTION_SLOTS = 4
const SERIES_COUNT = 4

export type ConnectionStage = 'problem' | 'active' | 'disabled' | 'removed'
export interface ConnectionEntry {
  id: string
  order: number
  adapter?: AdapterDescriptor
  stats?: MessageStatsConnection
  stage: ConnectionStage
  tone: StatusRowTone
  received: number
  sent: number
  total: number
  previous?: number
}

export interface ConnectionArrangement {
  entries: ConnectionEntry[]
  shown: ConnectionEntry[]
  hidden: ConnectionEntry[]
  // Palette slot (0-based) for every connection drawn as its own layer.
  colorSlots: Map<string, number>
}

const stageRank: Record<ConnectionStage, number> = { problem: 0, active: 1, disabled: 2, removed: 3 }

function stageOf(adapter: AdapterDescriptor | undefined): { stage: ConnectionStage; tone: StatusRowTone } {
  if (!adapter) return { stage: 'removed', tone: 'muted' }
  if (!adapter.enabled) return { stage: 'disabled', tone: 'muted' }
  const tone = toRowTone(adapter.state)
  return { stage: tone === 'danger' || tone === 'warning' ? 'problem' : 'active', tone }
}

// Configured connections come from the adapter list in configuration order; the statistics add connections that were
// removed from the configuration but still have counts.
export function arrangeConnections(adapters: AdapterDescriptor[], stats?: MessageStatsResponse | null): ConnectionArrangement {
  const statsById = new Map((stats?.connections ?? []).map(item => [item.adapter_id, item]))
  const entries: ConnectionEntry[] = adapters.map((adapter, order) => ({ id: adapter.id, order, adapter, stats: statsById.get(adapter.id), ...stageOf(adapter), received: 0, sent: 0, total: 0 }))
  for (const item of stats?.connections ?? []) {
    if (!item.configured && !adapters.some(adapter => adapter.id === item.adapter_id)) {
      entries.push({ id: item.adapter_id, order: adapters.length + entries.length, stats: item, ...stageOf(undefined), received: 0, sent: 0, total: 0 })
    }
  }
  for (const entry of entries) {
    entry.received = entry.stats?.totals.received ?? 0
    entry.sent = entry.stats?.totals.sent ?? 0
    entry.total = entry.received + entry.sent
    entry.previous = entry.stats?.previous ? entry.stats.previous.received + entry.stats.previous.sent : undefined
  }
  // Problems first so they are never folded away, then the busiest; ties keep configuration order.
  const sorted = [...entries].sort((a, b) => stageRank[a.stage] - stageRank[b.stage] || b.total - a.total || a.order - b.order)
  const overflow = sorted.length > CONNECTION_SLOTS
  const shown = overflow ? sorted.slice(0, CONNECTION_SLOTS - 1) : sorted
  const hidden = overflow ? sorted.slice(CONNECTION_SLOTS - 1) : []
  // Colours follow configuration order so a connection keeps its colour when the period changes.
  const colorSlots = new Map<string, number>()
  const used = new Set<number>()
  for (const entry of [...shown].sort((a, b) => a.order - b.order)) {
    let slot = entry.order % SERIES_COUNT
    while (used.has(slot) && used.size < SERIES_COUNT) slot = (slot + 1) % SERIES_COUNT
    used.add(slot)
    colorSlots.set(entry.id, slot)
  }
  return { entries: sorted, shown, hidden, colorSlots }
}

// ---------- layers ----------

export type MessageSplit = 'connection' | 'direction'
export interface MessageLayer {
  key: string
  // Palette slot, or 'other' for the merged layer.
  color: number | 'other'
  total: number
  values: number[]
}

function sumColumns(columns: number[][], length: number) {
  return Array.from({ length }, (_, index) => columns.reduce((sum, column) => sum + (column[index] ?? 0), 0))
}

// The legend order (rows) and the stacking order (busiest at the bottom, 其他 on top) differ, so both are returned.
export function buildLayers(stats: MessageStatsResponse, arrangement: ConnectionArrangement, split: MessageSplit) {
  const length = stats.buckets.length
  const all = stats.connections
  if (split === 'direction') {
    const order: MessageLayer[] = [
      { key: 'received', color: 0, total: stats.totals.received, values: sumColumns(all.map(item => item.received), length) },
      { key: 'sent', color: 1, total: stats.totals.sent, values: sumColumns(all.map(item => item.sent), length) },
    ]
    return { order, stack: order }
  }
  const series = (entry: ConnectionEntry) => sumColumns(entry.stats ? [entry.stats.received, entry.stats.sent] : [], length)
  const order: MessageLayer[] = arrangement.shown.map(entry => ({ key: entry.id, color: arrangement.colorSlots.get(entry.id) ?? 0, total: entry.total, values: series(entry) }))
  if (arrangement.hidden.length) {
    order.push({ key: 'other', color: 'other', total: arrangement.hidden.reduce((sum, entry) => sum + entry.total, 0), values: sumColumns(arrangement.hidden.map(series), length) })
  }
  const stack = [...order.filter(layer => layer.key !== 'other').sort((a, b) => b.total - a.total), ...order.filter(layer => layer.key === 'other')]
  return { order, stack }
}

export function layerColor(color: number | 'other') {
  return color === 'other' ? 'var(--chart-other)' : `var(--chart-series-${color + 1})`
}

export function layerFill(color: number | 'other') {
  return color === 'other' ? 'var(--chart-other-soft)' : `var(--chart-series-${color + 1}-soft)`
}

// ---------- chart geometry ----------

// Integer ticks, three to five of them, with the smallest top that fits.
export function niceScale(value: number) {
  const top = Math.max(1, value)
  const order = Math.floor(Math.log10(top))
  let best: { max: number; ticks: number } | null = null
  for (let exponent = order - 2; exponent <= order + 1; exponent++) {
    for (const multiple of [1, 2, 2.5, 5]) {
      const step = multiple * 10 ** exponent
      if (step < 1 || !Number.isInteger(step)) continue
      const ticks = Math.ceil(top / step)
      if (ticks < 3 || ticks > 5) continue
      if (!best || ticks * step < best.max || (ticks * step === best.max && ticks > best.ticks)) best = { max: ticks * step, ticks }
    }
  }
  return best ?? { max: 3, ticks: 3 }
}

// A second axis keeps the first axis's tick count so both share the grid lines; it takes the smallest nice step that fits.
export function niceScaleForTicks(value: number, ticks: number) {
  const top = Math.max(1, value)
  for (let exponent = 0; ; exponent++) {
    for (const multiple of [1, 2, 2.5, 5]) {
      const step = multiple * 10 ** exponent
      if (Number.isInteger(step) && step * ticks >= top) return { max: step * ticks, ticks }
    }
  }
}

// Gently smoothed cubic segments; segment i runs from points[i] to points[i + 1].
export function smoothSegments(points: [number, number][]) {
  const tension = 0.18
  return points.slice(0, -1).map((p1, index) => {
    const p0 = points[index - 1] ?? p1
    const p2 = points[index + 1]!
    const p3 = points[index + 2] ?? p2
    return ` C${p1[0] + (p2[0] - p0[0]) * tension},${p1[1] + (p2[1] - p0[1]) * tension} ${p2[0] - (p3[0] - p1[0]) * tension},${p2[1] - (p3[1] - p1[1]) * tension} ${p2[0]},${p2[1]}`
  })
}

export function pathThrough(points: [number, number][], segments: string[], from: number, to: number) {
  const start = points[from]
  return start ? `M${start[0]},${start[1]}${segments.slice(from, to).join('')}` : ''
}

// ---------- figures ----------

// Message volume is neither good nor bad: the change is a ratio for a neutral chip, or null when there is nothing to compare.
export function changeRatio(value: number, previous?: number | null) {
  if (previous === undefined || previous === null || previous === 0) return null
  return (value - previous) / previous
}

export function bucketEnd(stats: MessageStatsResponse, index: number) {
  const next = stats.buckets[index + 1]
  if (next) return Date.parse(next)
  const start = Date.parse(stats.buckets[index]!)
  if (stats.granularity === 'hour') return Math.min(start + 3_600_000, Date.parse(stats.end_at))
  return Date.parse(stats.end_at)
}
