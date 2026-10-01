<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { CalendarDate, parseDate, type DateValue } from '@internationalized/date'
import { CalendarRangeIcon, ChevronLeftIcon, ChevronRightIcon } from '@lucide/vue'
import {
  RangeCalendarCell,
  RangeCalendarCellTrigger,
  RangeCalendarGrid,
  RangeCalendarGridBody,
  RangeCalendarGridHead,
  RangeCalendarGridRow,
  RangeCalendarHeadCell,
  RangeCalendarNext,
  RangeCalendarPrev,
  RangeCalendarRoot,
  type DateRange,
} from 'reka-ui'

import AppPopover from '@/components/AppPopover.vue'
import AppSegmented from '@/components/AppSegmented.vue'
import { i18n, t } from '@/i18n'
import {
  HOURLY_CUSTOM_DAYS,
  MAX_CUSTOM_DAYS,
  daysBetween,
  localDate,
  shiftDate,
  type MessagePeriod,
  type MessagePeriodPreset,
} from '@/views/dashboard/message-stats'

const props = defineProps<{ timeZone: string; trackingStartedAt?: string | null }>()
const period = defineModel<MessagePeriod>({ required: true })

const today = computed(() => localDate(new Date(), props.timeZone))
const earliest = computed(() => {
  if (!props.trackingStartedAt) return null
  const date = localDate(new Date(props.trackingStartedAt), props.timeZone)
  return date < today.value ? date : today.value
})

const segment = computed<MessagePeriodPreset | 'custom'>({
  get: () => (period.value.kind === 'preset' ? period.value.preset : 'custom'),
  set: value => { if (value !== 'custom') period.value = { kind: 'preset', preset: value } },
})
const segmentOptions = computed(() => (['today', 'week', 'month'] as const).map(value => ({ value, label: t(`dashboard.messages.periods.${value}`) })))

const shortDate = computed(() => new Intl.DateTimeFormat(i18n.global.locale.value, { month: 'long', day: 'numeric', timeZone: 'UTC' }))
const formatDate = (date: string) => shortDate.value.format(new Date(`${date}T00:00:00Z`))
const customLabel = computed(() => (period.value.kind === 'custom'
  ? (period.value.from === period.value.to ? formatDate(period.value.from) : t('dashboard.messages.customRange', { from: formatDate(period.value.from), to: formatDate(period.value.to) }))
  : t('dashboard.messages.custom')))

// ---------- the custom range popover ----------
const open = ref(false)
const range = shallowRef<DateRange | undefined>()
const placeholder = shallowRef<DateValue>(parseDate(shiftDate(today.value, -31)))
watch(open, (value) => {
  if (!value) return
  range.value = period.value.kind === 'custom' ? { start: parseDate(period.value.from), end: parseDate(period.value.to) } : undefined
  // Two months side by side, the later one holding today or the chosen end.
  const anchor = period.value.kind === 'custom' ? period.value.to : today.value
  const [year, month] = anchor.split('-').map(Number)
  placeholder.value = new CalendarDate(year!, month!, 1).subtract({ months: 1 })
})
const minValue = computed(() => (earliest.value ? parseDate(earliest.value) : undefined))
const maxValue = computed(() => parseDate(today.value))

// Picking the end date applies the range at once, like the log filters; there is no apply button.
// Reka's maximumDays would shade the whole allowed span while picking, so an over-long range is shortened here instead.
function onRange(value: DateRange | undefined) {
  range.value = value
  if (!value?.start || !value.end) return
  const to = value.end.toString()
  const from = daysBetween(value.start.toString(), to) >= MAX_CUSTOM_DAYS ? shiftDate(to, 1 - MAX_CUSTOM_DAYS) : value.start.toString()
  apply(from, to)
}
function apply(from: string, to: string) {
  period.value = { kind: 'custom', from, to }
  open.value = false
}

const presets = computed(() => {
  const day = today.value
  const firstOfMonth = `${day.slice(0, 8)}01`
  const lastMonthEnd = shiftDate(firstOfMonth, -1)
  return [
    { key: 'today', label: t('dashboard.messages.presets.today'), pick: () => choosePreset('today') },
    { key: 'yesterday', label: t('dashboard.messages.presets.yesterday'), pick: () => apply(shiftDate(day, -1), shiftDate(day, -1)) },
    { key: 'week', label: t('dashboard.messages.presets.week'), pick: () => choosePreset('week') },
    { key: 'month', label: t('dashboard.messages.presets.month'), pick: () => choosePreset('month') },
    { key: 'thisMonth', label: t('dashboard.messages.presets.thisMonth'), pick: () => apply(firstOfMonth, day) },
    { key: 'lastMonth', label: t('dashboard.messages.presets.lastMonth'), pick: () => apply(`${lastMonthEnd.slice(0, 8)}01`, lastMonthEnd) },
    { key: 'quarter', label: t('dashboard.messages.presets.quarter'), pick: () => apply(shiftDate(day, -89), day) },
  ]
})
function choosePreset(preset: MessagePeriodPreset) {
  period.value = { kind: 'preset', preset }
  open.value = false
}

const summary = computed(() => {
  const start = range.value?.start?.toString()
  const end = range.value?.end?.toString()
  if (!start) return t('dashboard.messages.rangePickStart')
  if (!end) return t('dashboard.messages.rangePickEnd', { date: formatDate(start) })
  const days = daysBetween(start, end) + 1
  return t('dashboard.messages.rangeSummary', {
    from: formatDate(start),
    to: formatDate(end),
    days,
    unit: t(days <= HOURLY_CUSTOM_DAYS ? 'dashboard.messages.byHour' : 'dashboard.messages.byDay'),
  })
})
const monthHeading = (value: DateValue) => new Intl.DateTimeFormat(i18n.global.locale.value, { year: 'numeric', month: 'long', timeZone: 'UTC' })
  .format(new Date(Date.UTC(value.year, value.month - 1, 1)))
</script>

<template>
  <div class="message-period">
    <AppSegmented v-model="segment" :options="segmentOptions" :label="t('dashboard.messages.periodLabel')" class="message-period__segments" />
    <AppPopover v-model:open="open" side="bottom" align="end" :width="716" :label="t('dashboard.messages.customTitle')">
      <button type="button" class="message-period__custom" :data-active="period.kind === 'custom' || undefined" :aria-expanded="open">
        <CalendarRangeIcon aria-hidden="true" />{{ customLabel }}
      </button>
      <template #content>
        <div class="message-range">
          <div class="message-range__presets" role="group" :aria-label="t('dashboard.messages.presetsLabel')">
            <button v-for="preset in presets" :key="preset.key" type="button" @click="preset.pick">{{ preset.label }}</button>
          </div>
          <div class="message-range__main">
            <RangeCalendarRoot
              v-slot="{ grid, weekDays }"
              :model-value="range"
              :placeholder="placeholder"
              :min-value="minValue"
              :max-value="maxValue"
              :number-of-months="2"
              :week-starts-on="1"
              :locale="i18n.global.locale.value"
              weekday-format="narrow"
              fixed-weeks
              class="message-range__calendar"
              @update:model-value="onRange"
              @update:placeholder="value => (placeholder = value)"
            >
              <div class="message-range__months">
                <div v-for="(month, index) in grid" :key="month.value.toString()" class="message-range__month">
                  <div class="message-range__heading">
                    <RangeCalendarPrev v-if="index === 0" class="message-range__nav" :aria-label="t('dashboard.messages.previousMonth')"><ChevronLeftIcon aria-hidden="true" /></RangeCalendarPrev>
                    <span v-else class="message-range__nav-spacer" />
                    <span>{{ monthHeading(month.value) }}</span>
                    <RangeCalendarNext v-if="index === grid.length - 1" class="message-range__nav" :aria-label="t('dashboard.messages.nextMonth')"><ChevronRightIcon aria-hidden="true" /></RangeCalendarNext>
                    <span v-else class="message-range__nav-spacer" />
                  </div>
                  <RangeCalendarGrid class="message-range__grid">
                    <RangeCalendarGridHead>
                      <RangeCalendarGridRow class="message-range__week">
                        <RangeCalendarHeadCell v-for="day in weekDays" :key="day" class="message-range__weekday">{{ day }}</RangeCalendarHeadCell>
                      </RangeCalendarGridRow>
                    </RangeCalendarGridHead>
                    <RangeCalendarGridBody>
                      <RangeCalendarGridRow v-for="(week, row) in month.rows" :key="row" class="message-range__week">
                        <RangeCalendarCell v-for="date in week" :key="date.toString()" :date="date" class="message-range__cell">
                          <RangeCalendarCellTrigger :day="date" :month="month.value" class="message-range__day" />
                        </RangeCalendarCell>
                      </RangeCalendarGridRow>
                    </RangeCalendarGridBody>
                  </RangeCalendarGrid>
                </div>
              </div>
            </RangeCalendarRoot>
            <p class="message-range__foot">
              <span>{{ summary }}</span>
              <span v-if="earliest" class="message-range__earliest">{{ t('dashboard.messages.rangeEarliest', { date: formatDate(earliest) }) }}</span>
            </p>
          </div>
        </div>
      </template>
    </AppPopover>
  </div>
</template>

<style scoped lang="scss">
.message-period { display: flex; align-items: center; gap: 10px; }
.message-period__segments :deep(.app-segmented__item) { min-height: 30px; padding: 3px 14px; }
// The custom range shares the raised control surface; once chosen it names the range and keeps a brand ring.
.message-period__custom { display: inline-flex; align-items: center; gap: 6px; height: 36px; padding: 0 14px; border: 0; border-radius: 999px; background: var(--surface-raised); box-shadow: var(--shadow-xs); color: var(--text); font: inherit; font-size: var(--font-size-sm); font-weight: 500; white-space: nowrap; cursor: pointer; }
.message-period__custom svg { width: 15px; height: 15px; color: var(--muted); }
.message-period__custom[data-active], .message-period__custom[aria-expanded=true] { box-shadow: var(--shadow-xs), inset 0 0 0 1.5px var(--brand-foreground); }
.message-period__custom:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }

.message-range { display: grid; grid-template-columns: 116px minmax(0, 1fr); margin: -16px; }
.message-range__presets { display: grid; align-content: start; gap: 2px; padding: 12px 8px; border-right: 1px solid var(--border); }
.message-range__presets button { height: 34px; padding: 0 12px; border: 0; border-radius: 999px; background: none; color: var(--text); font: inherit; font-size: var(--font-size-sm); text-align: left; cursor: pointer; }
.message-range__presets button:hover { background: var(--nav-hover); }
.message-range__presets button:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
.message-range__main { display: grid; gap: 10px; padding: 14px 18px 12px; }
.message-range__months { display: grid; grid-template-columns: repeat(2, auto); justify-content: space-between; gap: 28px; }
.message-range__heading { display: flex; align-items: center; gap: 8px; height: 32px; margin-bottom: 4px; font-size: var(--font-size-md); font-weight: 700; }
.message-range__heading > span:not(.message-range__nav-spacer) { flex: 1; text-align: center; }
.message-range__nav, .message-range__nav-spacer { display: grid; flex: none; place-items: center; width: 30px; height: 30px; }
.message-range__nav { border: 0; border-radius: 50%; background: none; color: var(--muted); cursor: pointer; }
.message-range__nav:hover { background: var(--nav-hover); color: var(--text); }
.message-range__nav:focus-visible { outline: 2px solid var(--focus); outline-offset: 0; }
.message-range__nav[data-disabled] { opacity: .4; cursor: default; }
.message-range__nav svg { width: 17px; height: 17px; }
.message-range__grid { border-collapse: collapse; }
.message-range__weekday { width: 36px; height: 28px; color: var(--muted); font-size: var(--font-size-xs); font-weight: 400; }
.message-range__cell { position: relative; width: 36px; height: 36px; padding: 0; }
// The band behind a range's ends lives on the cell, under the filled end circle.
.message-range__cell::before { position: absolute; top: 1px; bottom: 1px; content: none; background: var(--brand-soft); }
.message-range__cell:has(> [data-selection-start]:not([data-selection-end]))::before { content: ''; left: 50%; right: 0; }
.message-range__cell:has(> [data-selection-end]:not([data-selection-start]))::before { content: ''; left: 0; right: 50%; }
.message-range__day { position: relative; z-index: 1; display: grid; place-items: center; width: 34px; height: 34px; margin: 1px; border-radius: 50%; color: var(--text); font-size: var(--font-size-sm); font-variant-numeric: tabular-nums; cursor: pointer; outline: none; }
.message-range__day:hover:not([data-disabled]):not([data-selected]) { background: var(--nav-hover); }
.message-range__day:focus-visible { box-shadow: 0 0 0 2px var(--focus); }
.message-range__day[data-today]:not([data-selected]) { box-shadow: inset 0 0 0 1.5px var(--border-strong); }
.message-range__day[data-outside-view] { visibility: hidden; }
.message-range__day[data-disabled], .message-range__day[data-unavailable] { color: var(--muted); cursor: default; }
// The chosen range, and the one being picked, is one soft band; its ends are filled brand circles.
.message-range__day[data-selected], .message-range__day[data-highlighted] { width: 36px; margin-inline: 0; border-radius: 0; background: var(--brand-soft); }
.message-range__day[data-selection-start], .message-range__day[data-selection-end] { width: 34px; margin-inline: 1px; border-radius: 50%; background: var(--brand-fill); color: var(--on-brand); font-weight: 700; }
.message-range__foot { display: flex; align-items: center; gap: 12px; margin: 0; padding-top: 10px; border-top: 1px solid var(--border); color: var(--muted); font-size: var(--font-size-xs); }
.message-range__earliest { margin-left: auto; }
@media (prefers-reduced-motion: reduce) { .message-range__day { transition: none; } }
@media (forced-colors: active) { .message-range__day[data-selection-start], .message-range__day[data-selection-end] { outline: 2px solid Highlight; } }
</style>
