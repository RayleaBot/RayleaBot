<script setup lang="ts">
import { computed, onScopeDispose, ref, shallowRef, watch } from 'vue'
import { useElementSize } from '@vueuse/core'

import { i18n, t } from '@/i18n'
import { prefersReducedMotion } from '@/motion/runtime'
import type { MessageStatsIncident, MessageStatsResponse } from '@/types/api'
import {
  bucketEnd,
  layerColor,
  layerFill,
  localDate,
  localMidnight,
  niceScale,
  pathThrough,
  shiftDate,
  smoothSegments,
  type MessageLayer,
} from '@/views/dashboard/message-stats'

const props = withDefaults(defineProps<{
  stats: MessageStatsResponse
  // Stacking order, bottom layer first; the tooltip lists `order`, the legend order.
  layers: MessageLayer[]
  order: MessageLayer[]
  // Display names for layer keys and adapter ids.
  names: Record<string, string>
  timeZone: string
  focusKey?: string | null
  // Buckets that end after this instant (epoch ms) can open the history logs; null when the logs are not kept.
  logsFrom?: number | null
  // Grows each time a reading of the same period brought more messages; the newest point answers with one pulse.
  pulse?: number
  height?: number
}>(), { focusKey: null, logsFrom: null, pulse: 0, height: 330 })
const emit = defineEmits<{ openLogs: [range: { startAt: string; endAt: string }] }>()

const host = ref<HTMLElement | null>(null)
const { width } = useElementSize(host)
const HOUR = 3_600_000
const DAY = 86_400_000

const numberFormat = computed(() => new Intl.NumberFormat(i18n.global.locale.value))
const clockFormat = computed(() => new Intl.DateTimeFormat(i18n.global.locale.value, { timeZone: props.timeZone, hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }))
const dayFormat = computed(() => new Intl.DateTimeFormat(i18n.global.locale.value, { timeZone: props.timeZone, month: 'long', day: 'numeric' }))
const today = computed(() => localDate(new Date(props.stats.as_of), props.timeZone))
const dayLabel = (time: number) => (localDate(new Date(time), props.timeZone) === today.value ? t('dashboard.messages.today') : dayFormat.value.format(time))
const hourLabel = (time: number) => clockFormat.value.format(time)

const hasTrack = computed(() => props.stats.incidents.length > 0)
const padding = computed(() => ({ left: 44, right: 8, top: 10, bottom: hasTrack.value ? 42 : 26 }))
const plot = computed(() => ({
  width: Math.max(0, width.value - padding.value.left - padding.value.right),
  height: Math.max(0, props.height - padding.value.top - padding.value.bottom),
}))
const starts = computed(() => props.stats.buckets.map(value => Date.parse(value)))
const asOf = computed(() => Date.parse(props.stats.as_of))
// The axis starts with the requested range, so time before counting began stays visibly empty, and ends at the
// newest bucket.
const domain = computed(() => {
  const start = Date.parse(props.stats.start_at)
  const step = props.stats.granularity === 'hour' ? HOUR : DAY
  const last = starts.value.at(-1)
  return { start, end: Math.max(last ?? asOf.value, start + step) }
})
const x = (time: number) => padding.value.left + ((time - domain.value.start) / (domain.value.end - domain.value.start)) * plot.value.width

// A live reading of the same buckets glides the curves to their new values; other changes redraw at once. The tooltip
// always reads the exact values from the props.
const shown = shallowRef<number[][]>(props.layers.map(layer => [...layer.values]))
let tweenFrame = 0
watch(() => [props.stats.buckets, props.layers] as const, ([buckets, layers], [previousBuckets, previousLayers]) => {
  cancelAnimationFrame(tweenFrame)
  const target = layers.map(layer => [...layer.values])
  const sameShape = buckets.length === previousBuckets.length && buckets[0] === previousBuckets[0]
    && layers.length === previousLayers.length && layers.every((layer, index) => layer.key === previousLayers[index]?.key)
  if (!sameShape || prefersReducedMotion() || typeof requestAnimationFrame !== 'function') {
    shown.value = target
    return
  }
  const from = shown.value
  const started = performance.now()
  const step = (time: number) => {
    const progress = Math.min(1, (time - started) / 450)
    const eased = 1 - (1 - progress) ** 3
    shown.value = target.map((values, layer) => values.map((value, bucket) => {
      const before = from[layer]?.[bucket] ?? value
      return before + (value - before) * eased
    }))
    if (progress < 1) tweenFrame = requestAnimationFrame(step)
  }
  tweenFrame = requestAnimationFrame(step)
})
onScopeDispose(() => cancelAnimationFrame(tweenFrame))
const tops = computed(() => props.layers.map((_, index) => starts.value.map((__, bucket) => shown.value.slice(0, index + 1).reduce((sum, values) => sum + (values[bucket] ?? 0), 0))))
const totals = computed(() => tops.value.at(-1) ?? starts.value.map(() => 0))
const scale = computed(() => niceScale(Math.max(1, ...totals.value)))
const y = (value: number) => padding.value.top + plot.value.height - (value / scale.value.max) * plot.value.height
const partial = computed(() => starts.value.length > 0 && bucketEnd(props.stats, starts.value.length - 1) > asOf.value)

const yTicks = computed(() => Array.from({ length: scale.value.ticks + 1 }, (_, index) => {
  const value = (scale.value.max * index) / scale.value.ticks
  return { y: y(value), text: numberFormat.value.format(Math.round(value)) }
}))

// Several days by the hour are labelled at each local midnight; one day by the hour every few hours; days every few days.
const xLabels = computed(() => {
  const { start, end } = domain.value
  const midnights: number[] = []
  for (let date = localDate(new Date(start), props.timeZone), guard = 0; guard < 800; date = shiftDate(date, 1), guard++) {
    const time = Date.parse(localMidnight(date, props.timeZone))
    if (time > end) break
    if (time >= start) midnights.push(time)
  }
  if (props.stats.granularity === 'hour' && end - start > 36 * HOUR) return midnights.map(time => ({ x: x(time), text: dayFormat.value.format(time) }))
  if (props.stats.granularity === 'hour') {
    const hours = Math.round((end - start) / HOUR)
    const every = [1, 2, 3, 4, 6, 12].find(step => hours / step <= 8) ?? 12
    const labels = []
    for (let time = start; time <= end; time += HOUR) {
      if (Math.round((time - start) / HOUR) % every === 0) labels.push({ x: x(time), text: hourLabel(time) })
    }
    return labels
  }
  const every = Math.max(1, Math.ceil(midnights.length / 10))
  return midnights.filter((_, index) => index % every === 0).map(time => ({ x: x(time), text: dayFormat.value.format(time) }))
})

const shapes = computed(() => {
  if (!plot.value.width || !starts.value.length) return []
  const count = starts.value.length
  return props.layers.map((layer, index) => {
    const muted = props.focusKey !== null && layer.key !== props.focusKey
    const top = starts.value.map((time, bucket) => [x(time), y(tops.value[index]![bucket]!)] as [number, number])
    const below = index === 0 ? starts.value.map(() => 0) : tops.value[index - 1]!
    const base = starts.value.map((time, bucket) => [x(time), y(below[bucket]!)] as [number, number]).reverse()
    const segments = smoothSegments(top)
    const area = count > 1 ? `${pathThrough(top, segments, 0, count - 1)} L${base[0]![0]},${base[0]![1]}${smoothSegments(base).join('')} Z` : ''
    // The newest bucket is still filling, so its edge is dotted rather than read as a drop.
    const solidEnd = partial.value && count > 2 ? count - 2 : count - 1
    return {
      key: layer.key,
      area,
      line: count > 1 ? pathThrough(top, segments, 0, solidEnd) : '',
      tail: solidEnd < count - 1 ? pathThrough(top, segments, count - 2, count - 1) : '',
      dot: count === 1 ? top[0] : null,
      fill: muted ? 'var(--chart-muted-fill)' : layerFill(layer.color),
      stroke: muted ? 'var(--chart-muted-line)' : layerColor(layer.color),
    }
  })
})

// The newest point of an open period marks "now"; when more messages arrive it sends out one ring.
const head = computed(() => {
  const last = starts.value.length - 1
  if (last < 0 || !plot.value.width || !partial.value) return null
  const top = props.layers.length - 1
  const muted = props.focusKey !== null && props.layers[top]?.key !== props.focusKey
  return { x: x(starts.value[last]!), y: y(tops.value[top]?.[last] ?? 0), stroke: muted ? 'var(--chart-muted-line)' : layerColor(props.layers[top]?.color ?? 0) }
})

const tracking = computed(() => {
  const time = Date.parse(props.stats.tracking_started_at)
  if (!(time > domain.value.start) || !plot.value.width) return null
  const at = x(Math.min(time, domain.value.end))
  const label = t('dashboard.messages.trackingStarted', { time: `${dayLabel(time)} ${hourLabel(time)}` })
  return { x: at, label, emptyX: at - padding.value.left > 220 ? (padding.value.left + at) / 2 : null }
})

const incidentEnd = (incident: MessageStatsIncident) => (incident.ended_at ? Date.parse(incident.ended_at) : asOf.value)
const track = computed(() => {
  if (!plot.value.width) return []
  const right = padding.value.left + plot.value.width
  return props.stats.incidents.map((incident, index) => {
    const x0 = Math.max(padding.value.left, x(Math.max(Date.parse(incident.started_at), domain.value.start)))
    const x1 = Math.min(right, x(Math.min(incidentEnd(incident), domain.value.end)))
    return { key: `${incident.kind}-${incident.started_at}-${index}`, kind: incident.kind, x: Math.min(x0, right - 5), width: Math.max(5, x1 - x0) }
  })
})

function duration(ms: number) {
  const minutes = Math.max(1, Math.round(ms / 60_000))
  const hours = Math.floor(minutes / 60)
  if (!hours) return t('dashboard.messages.duration.minutes', { minutes })
  return minutes % 60 ? t('dashboard.messages.duration.hoursMinutes', { hours, minutes: minutes % 60 }) : t('dashboard.messages.duration.hours', { hours })
}

const active = ref<number | null>(null)
const tooltip = computed(() => {
  const index = active.value
  if (index === null || index >= starts.value.length) return null
  const start = starts.value[index]!
  const end = bucketEnd(props.stats, index)
  const title = props.stats.granularity === 'hour' ? `${dayLabel(start)} ${hourLabel(start)}` : dayLabel(start)
  const notes = props.stats.incidents.flatMap((incident): { kind: MessageStatsIncident['kind']; text: string }[] => {
    const overlap = Math.min(end, incidentEnd(incident)) - Math.max(start, Date.parse(incident.started_at))
    if (overlap <= 0) return []
    if (incident.kind === 'server_stopped') return [{ kind: incident.kind, text: t('dashboard.messages.incidentStopped', { duration: duration(overlap) }) }]
    const name = props.names[incident.adapter_id ?? ''] ?? incident.adapter_id ?? ''
    const ongoing = !incident.ended_at && end > asOf.value
    return [{
      kind: incident.kind,
      text: ongoing
        ? t('dashboard.messages.incidentOngoing', { name, duration: duration(asOf.value - Date.parse(incident.started_at)) })
        : t('dashboard.messages.incidentOffline', { name, duration: duration(overlap) }),
    }]
  })
  const openable = props.logsFrom !== null && end > props.logsFrom
  return {
    index,
    x: x(start),
    title,
    asOf: end > asOf.value ? t('dashboard.messages.asOf', { time: hourLabel(asOf.value) }) : '',
    rows: props.order.map(layer => ({ key: layer.key, name: props.names[layer.key] ?? layer.key, color: layerColor(layer.color), value: numberFormat.value.format(layer.values[index] ?? 0) })),
    total: props.order.length > 1 ? numberFormat.value.format(totals.value[index] ?? 0) : '',
    notes,
    hint: openable ? t(props.stats.granularity === 'hour' ? 'dashboard.messages.openLogsHour' : 'dashboard.messages.openLogsDay') : '',
    dots: props.layers.map((layer, layerIndex) => ({
      key: layer.key,
      y: y(tops.value[layerIndex]![index]!),
      stroke: props.focusKey !== null && layer.key !== props.focusKey ? 'var(--chart-muted-line)' : layerColor(layer.color),
    })),
    range: { startAt: new Date(start).toISOString(), endAt: new Date(end - 1000).toISOString() },
  }
})
const tooltipElement = ref<HTMLElement | null>(null)
const { width: tooltipWidth } = useElementSize(tooltipElement)
const tooltipLeft = computed(() => {
  const at = tooltip.value?.x ?? 0
  return at + 14 + tooltipWidth.value > width.value ? at - tooltipWidth.value - 14 : at + 14
})

function nearest(clientX: number) {
  const rect = host.value?.getBoundingClientRect()
  if (!rect || !starts.value.length) return null
  const time = domain.value.start + ((clientX - rect.left - padding.value.left) / Math.max(1, plot.value.width)) * (domain.value.end - domain.value.start)
  let best = 0
  starts.value.forEach((start, index) => { if (Math.abs(start - time) < Math.abs(starts.value[best]! - time)) best = index })
  return best
}
function onPointerMove(event: PointerEvent) { active.value = nearest(event.clientX) }
function onClick(event: MouseEvent) {
  active.value = nearest(event.clientX)
  if (tooltip.value?.hint) emit('openLogs', tooltip.value.range)
}
function onKeydown(event: KeyboardEvent) {
  if (!starts.value.length) return
  if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
    event.preventDefault()
    const current = active.value ?? starts.value.length - 1
    active.value = Math.max(0, Math.min(starts.value.length - 1, current + (event.key === 'ArrowLeft' ? -1 : 1)))
  } else if ((event.key === 'Enter' || event.key === ' ') && tooltip.value?.hint) {
    event.preventDefault()
    emit('openLogs', tooltip.value.range)
  } else if (event.key === 'Escape') {
    active.value = null
  }
}
</script>

<template>
  <div
    ref="host"
    class="message-chart"
    :style="{ height: `${height}px` }"
    tabindex="0"
    role="group"
    :aria-label="t('dashboard.messages.chartLabel')"
    @pointermove="onPointerMove"
    @pointerleave="active = null"
    @click="onClick"
    @focus="active = active ?? (starts.length ? starts.length - 1 : null)"
    @blur="active = null"
    @keydown="onKeydown"
  >
    <svg v-if="width" :viewBox="`0 0 ${width} ${height}`" :width="width" :height="height" aria-hidden="true">
      <g class="message-chart__grid">
        <line v-for="tick in yTicks" :key="`grid-${tick.text}`" :x1="padding.left" :x2="width - padding.right" :y1="tick.y" :y2="tick.y" />
      </g>
      <g class="message-chart__axis">
        <text v-for="tick in yTicks" :key="`y-${tick.text}`" :x="padding.left - 10" :y="tick.y + 4" text-anchor="end">{{ tick.text }}</text>
        <text v-for="label in xLabels" :key="`x-${label.x}`" :x="label.x" :y="height - 6" text-anchor="middle">{{ label.text }}</text>
      </g>
      <g v-for="shape in shapes" :key="shape.key">
        <path v-if="shape.area" :d="shape.area" :fill="shape.fill" />
        <path v-if="shape.line" :d="shape.line" class="message-chart__line" :stroke="shape.stroke" />
        <path v-if="shape.tail" :d="shape.tail" class="message-chart__line message-chart__line--partial" :stroke="shape.stroke" />
        <circle v-if="shape.dot" :cx="shape.dot[0]" :cy="shape.dot[1]" r="3.5" :fill="shape.stroke" />
      </g>
      <template v-if="tracking">
        <line class="message-chart__start" :x1="tracking.x" :x2="tracking.x" :y1="padding.top" :y2="padding.top + plot.height" />
        <text class="message-chart__note" :x="tracking.x - 8" :y="padding.top + 14" text-anchor="end">{{ tracking.label }}</text>
        <text v-if="tracking.emptyX !== null" class="message-chart__empty" :x="tracking.emptyX" :y="padding.top + plot.height / 2 + 4" text-anchor="middle">{{ t('dashboard.messages.notTrackedYet') }}</text>
      </template>
      <template v-if="head">
        <circle v-if="pulse" :key="`pulse-${pulse}`" class="message-chart__pulse" :cx="head.x" :cy="head.y" r="4" :fill="head.stroke" />
        <circle class="message-chart__head" :cx="head.x" :cy="head.y" r="3.5" :fill="head.stroke" />
      </template>
      <rect v-for="item in track" :key="item.key" class="message-chart__incident" :data-kind="item.kind" :x="item.x" :y="padding.top + plot.height + 9" :width="item.width" height="5" rx="2.5" />
      <template v-if="tooltip">
        <line class="message-chart__cursor" :x1="tooltip.x" :x2="tooltip.x" :y1="padding.top" :y2="padding.top + plot.height" />
        <circle v-for="dot in tooltip.dots" :key="`dot-${dot.key}`" :cx="tooltip.x" :cy="dot.y" r="3.5" class="message-chart__dot" :stroke="dot.stroke" />
      </template>
    </svg>
    <div v-if="tooltip" ref="tooltipElement" class="message-chart__tooltip" :style="{ left: `${tooltipLeft}px`, top: `${padding.top}px` }" role="status">
      <b>{{ tooltip.title }}<em v-if="tooltip.asOf">{{ tooltip.asOf }}</em></b>
      <div v-for="row in tooltip.rows" :key="row.key" class="message-chart__row"><i :style="{ background: row.color }" />{{ row.name }}<span>{{ row.value }}</span></div>
      <div v-if="tooltip.total" class="message-chart__row message-chart__row--sum">{{ t('dashboard.messages.total') }}<span>{{ tooltip.total }}</span></div>
      <p v-for="note in tooltip.notes" :key="note.text" class="message-chart__incident-note" :data-kind="note.kind"><i />{{ note.text }}</p>
      <p v-if="tooltip.hint" class="message-chart__hint">{{ tooltip.hint }}</p>
    </div>
  </div>
</template>

<style scoped lang="scss">
.message-chart { position: relative; min-width: 0; border-radius: var(--radius-md); outline: none; cursor: crosshair; }
.message-chart:focus-visible { outline: 2px solid var(--focus); outline-offset: 6px; }
.message-chart svg { display: block; overflow: visible; }
.message-chart__grid line { stroke: var(--border); stroke-width: 1; }
.message-chart__axis text { fill: var(--muted); font-size: 11px; font-variant-numeric: tabular-nums; }
.message-chart__line { fill: none; stroke-width: 1.8; stroke-linejoin: round; stroke-linecap: round; }
.message-chart__line--partial { stroke-dasharray: 1 5; }
.message-chart__start { stroke: var(--muted); stroke-width: 1; stroke-dasharray: 4 4; }
.message-chart__note, .message-chart__empty { paint-order: stroke; stroke: var(--surface); stroke-width: 6px; stroke-linejoin: round; }
.message-chart__note { fill: var(--muted); font-size: 12px; }
.message-chart__empty { fill: var(--muted); font-size: 13px; }
.message-chart__incident[data-kind=adapter_offline] { fill: var(--warning); }
.message-chart__incident[data-kind=server_stopped] { fill: var(--muted); }
.message-chart__cursor { stroke: var(--muted); stroke-width: 1; stroke-dasharray: 3 3; }
.message-chart__dot { fill: var(--surface-raised); stroke-width: 2; }
.message-chart__head { stroke: var(--surface); stroke-width: 2; }
// One soft ring from the newest point, like the reconnect signal; it fades out and stays gone until the next change.
.message-chart__pulse { transform-box: fill-box; transform-origin: center; opacity: 0; animation: message-chart-pulse 1.2s cubic-bezier(0, 0, 0.2, 1) both; }
@keyframes message-chart-pulse {
  from { opacity: .45; transform: scale(1); }
  to { opacity: 0; transform: scale(4); }
}
@media (prefers-reduced-motion: reduce) { .message-chart__pulse { animation: none; } }

// The reading floats above the plot on the raised control surface.
.message-chart__tooltip { position: absolute; z-index: 2; min-width: 168px; padding: 10px 12px; border: 1px solid var(--border); border-radius: var(--radius-md); background: var(--surface-raised); box-shadow: var(--shadow-floating); color: var(--text); font-size: var(--font-size-xs); white-space: nowrap; pointer-events: none; }
.message-chart__tooltip b { display: block; margin-bottom: 4px; font-weight: 700; }
.message-chart__tooltip em { margin-left: 8px; color: var(--muted); font-style: normal; font-weight: 500; }
.message-chart__row { display: flex; align-items: center; gap: 8px; line-height: 20px; }
.message-chart__row i { width: 8px; height: 8px; flex: none; border-radius: 2px; }
.message-chart__row span { margin-left: auto; padding-left: 16px; font-weight: 700; font-variant-numeric: tabular-nums; }
.message-chart__row--sum { margin-top: 4px; padding-top: 4px; border-top: 1px solid var(--border); }
.message-chart__incident-note { display: flex; align-items: center; gap: 8px; margin: 6px 0 0; padding-top: 6px; border-top: 1px solid var(--border); }
.message-chart__incident-note + .message-chart__incident-note { margin-top: 0; padding-top: 2px; border-top: 0; }
.message-chart__incident-note i { width: 8px; height: 8px; border-radius: 50%; background: var(--warning); }
.message-chart__incident-note[data-kind=server_stopped] i { background: var(--muted); }
.message-chart__hint { margin: 6px 0 0; color: var(--muted); font-size: 11px; }
@media (forced-colors: active) { .message-chart__tooltip { border-color: CanvasText; } }
</style>
