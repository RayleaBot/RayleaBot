<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import { ChevronDownIcon, ChevronRightIcon, LayersIcon, PlusIcon, RadioTowerIcon, XIcon } from '@lucide/vue'

import AppAlert from '@/components/AppAlert.vue'
import AppButton from '@/components/AppButton.vue'
import AppPopover from '@/components/AppPopover.vue'
import AppSegmented from '@/components/AppSegmented.vue'
import AppSkeleton from '@/components/AppSkeleton.vue'
import MotionRouterLink from '@/components/shell/MotionRouterLink.vue'
import { i18n, t } from '@/i18n'
import { buildLogsLocation, buildProtocolsLocation } from '@/lib/management-links'
import { useAdaptersStore } from '@/stores/adapters'
import { useConfigStore } from '@/stores/config'
import ConnectionStatsRow from '@/views/dashboard/ConnectionStatsRow.vue'
import MessageDelta from '@/views/dashboard/MessageDelta.vue'
import MessagePeriodPicker from '@/views/dashboard/MessagePeriodPicker.vue'
import MessageTrendChart from '@/views/dashboard/MessageTrendChart.vue'
import {
  CONNECTION_SLOTS,
  arrangeConnections,
  buildLayers,
  layerColor,
  type ConnectionEntry,
  type MessagePeriod,
  type MessageSplit,
} from '@/views/dashboard/message-stats'
import { useMessageStats } from '@/views/dashboard/useMessageStats'
import { useTweenedNumber } from '@/views/dashboard/useTweenedNumber'

const titleId = useId()
const router = useRouter()
const adaptersStore = useAdaptersStore()
const { adapters, error: adaptersError, loaded: adaptersLoaded, loading: adaptersLoading } = storeToRefs(adaptersStore)
const configStore = useConfigStore()
const { effectiveTimezone, logRetentionDays } = storeToRefs(configStore)

const period = ref<MessagePeriod>({ kind: 'preset', preset: 'week' })
const { stats, loading, error, open, live, updatedAt, pulse, reload } = useMessageStats(period)
const periodSubject = computed(() => JSON.stringify(period.value))

const arrangement = computed(() => arrangeConnections(adapters.value, stats.value))
const entries = computed(() => arrangement.value.entries)
// A single connection is the whole total, so it is split by direction instead.
const splitChoice = ref<MessageSplit>('connection')
const split = computed<MessageSplit>(() => (entries.value.length > 1 ? splitChoice.value : 'direction'))
const layers = computed(() => (stats.value ? buildLayers(stats.value, arrangement.value, split.value) : null))
const total = computed(() => (stats.value ? stats.value.totals.received + stats.value.totals.sent : 0))
// The first reading of a period shows at once; later readings of it roll to the new total.
const shownTotal = useTweenedNumber(() => total.value, () => `${periodSubject.value}|${stats.value ? 'read' : 'unread'}`)
// Live while the window is still counting and the event stream is up; otherwise say when the figures were read.
const clockFormat = computed(() => new Intl.DateTimeFormat(i18n.global.locale.value, { timeZone: effectiveTimezone.value, hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23' }))
const freshness = computed(() => {
  if (!stats.value || !open.value) return null
  if (live.value) return { live: true, text: t('dashboard.messages.live'), hint: t('dashboard.messages.liveHint') }
  return { live: false, text: t('dashboard.messages.updatedAt', { time: clockFormat.value.format(updatedAt.value ?? Date.now()) }), hint: t('dashboard.messages.pausedHint') }
})
const previousTotal = computed(() => (stats.value?.previous ? stats.value.previous.received + stats.value.previous.sent : null))

const connected = computed(() => adapters.value.filter(adapter => adapter.enabled && adapter.state === 'connected').length)
const enabledCount = computed(() => adapters.value.filter(adapter => adapter.enabled).length)
const meta = computed(() => {
  if (!adapters.value.length) return ''
  if (!enabledCount.value) return t('dashboard.hub.connectionsNoneEnabled')
  return t('dashboard.hub.connectionsMeta', { connected: connected.value, total: enabledCount.value })
})

const numberFormat = computed(() => new Intl.NumberFormat(i18n.global.locale.value))
const fmt = (value: number) => numberFormat.value.format(value)
const periodKey = computed(() => (period.value.kind === 'custom' ? 'custom' : period.value.preset))
const totalLabel = computed(() => (period.value.kind === 'custom' ? t('dashboard.messages.customTotal') : t('dashboard.messages.totalLabel', { period: t(`dashboard.messages.periods.${period.value.preset}`) })))
const subline = computed(() => {
  if (!stats.value) return ''
  const counts = t('dashboard.messages.counts', { received: fmt(stats.value.totals.received), sent: fmt(stats.value.totals.sent) })
  if (previousTotal.value === null) return counts
  return `${t(`dashboard.messages.compare.${periodKey.value}`, { count: fmt(previousTotal.value) })} · ${counts}`
})

const names = computed(() => {
  const list: Record<string, string> = { received: t('dashboard.messages.received'), sent: t('dashboard.messages.sent'), other: t('dashboard.messages.other', { count: arrangement.value.hidden.length }) }
  for (const entry of entries.value) list[entry.id] = displayName(entry)
  return list
})
function displayName(entry: ConnectionEntry) {
  return entry.adapter?.identity?.name || entry.adapter?.display_name || entry.id
}
const colorOf = (entry: ConnectionEntry) => {
  if (split.value !== 'connection') return null
  const slot = arrangement.value.colorSlots.get(entry.id)
  return slot === undefined ? layerColor('other') : layerColor(slot)
}
const shareOf = (entry: ConnectionEntry) => (stats.value && total.value > 0 ? entry.total / total.value : stats.value ? 0 : null)
const solo = computed(() => entries.value.length === 1)
const compared = computed(() => Boolean(stats.value?.previous))

const hiddenTotal = computed(() => arrangement.value.hidden.reduce((sum, entry) => sum + entry.total, 0))
const hiddenNames = computed(() => arrangement.value.hidden.map((entry) => {
  if (entry.stage === 'removed') return t('dashboard.messages.removedName', { name: displayName(entry) })
  if (entry.stage === 'disabled') return t('dashboard.messages.disabledName', { name: displayName(entry) })
  return displayName(entry)
}).join(t('dashboard.messages.nameSeparator')))
const showAdd = computed(() => !arrangement.value.hidden.length && arrangement.value.shown.length < CONNECTION_SLOTS)

// Hovering a row brings its layer forward; connections folded into 其他 bring that layer.
const focusKey = ref<string | null>(null)
function focusEntry(entry: ConnectionEntry | null) {
  if (!entry || split.value !== 'connection') { focusKey.value = null; return }
  focusKey.value = arrangement.value.colorSlots.has(entry.id) ? entry.id : 'other'
}

const allOpen = ref(false)
const groups = computed(() => [
  { key: 'active', entries: entries.value.filter(entry => entry.stage === 'problem' || entry.stage === 'active'), note: t('dashboard.messages.groupNotes.active') },
  { key: 'disabled', entries: entries.value.filter(entry => entry.stage === 'disabled'), note: '' },
  { key: 'removed', entries: entries.value.filter(entry => entry.stage === 'removed'), note: t('dashboard.messages.groupNotes.removed') },
].filter(group => group.entries.length))
const periodLabel = computed(() => (period.value.kind === 'custom' ? t('dashboard.messages.customShort') : t(`dashboard.messages.periods.${period.value.preset}`)))

const incidentKinds = computed(() => [...new Set((stats.value?.incidents ?? []).map(item => item.kind))])
const logsFrom = computed(() => Date.now() - (logRetentionDays.value ?? 7) * 86_400_000)
function openLogs(range: { startAt: string; endAt: string }) {
  void router.push(buildLogsLocation({ history: true, startAt: range.startAt, endAt: range.endAt }))
}
function retryAdapters() {
  void adaptersStore.refresh().catch(() => undefined)
}
</script>

<template>
  <section class="connections-card app-box" :aria-labelledby="titleId" data-testid="dashboard-connections">
    <header class="connections-card__header">
      <h2 :id="titleId" class="connections-card__title">{{ t('dashboard.hub.connections') }}</h2>
      <span v-if="meta" class="connections-card__meta">{{ meta }}</span>
      <MotionRouterLink :to="{ name: 'protocols' }" class="connections-card__link">{{ t('dashboard.hub.openProtocols') }}<ChevronRightIcon aria-hidden="true" /></MotionRouterLink>
      <MessagePeriodPicker v-if="entries.length" v-model="period" class="connections-card__period" :time-zone="effectiveTimezone" :tracking-started-at="stats?.tracking_started_at" />
    </header>

    <!-- An unread list is loading or failed, never "no connections". -->
    <AppAlert v-if="adaptersError && !adapters.length" tone="danger" :title="t('dashboard.hub.connectionsLoadFailed')" :description="adaptersError" data-testid="dashboard-connections-error">
      <template #action><AppButton size="sm" :loading="adaptersLoading" @click="retryAdapters">{{ t('ui.retry') }}</AppButton></template>
    </AppAlert>
    <AppSkeleton v-else-if="!adaptersLoaded && !entries.length" :rows="3" />
    <div v-else-if="!entries.length" class="connections-card__empty">
      <span class="connections-card__empty-glyph" aria-hidden="true"><RadioTowerIcon /></span>
      <h3>{{ t('dashboard.messages.emptyTitle') }}</h3>
      <p>{{ t('dashboard.messages.emptyDetail') }}</p>
      <AppButton variant="default" @click="router.push(buildProtocolsLocation({ view: 'add' }))">
        <template #icon><PlusIcon /></template>{{ t('dashboard.hub.addConnection') }}
      </AppButton>
    </div>

    <div v-else class="connections-card__body">
      <div class="connections-card__chart">
        <MessageTrendChart
          v-if="stats && layers"
          :stats="stats"
          :layers="layers.stack"
          :order="layers.order"
          :dual="split === 'direction'"
          :names="names"
          :time-zone="effectiveTimezone"
          :focus-key="focusKey"
          :logs-from="logsFrom"
          :pulse="pulse"
          @open-logs="openLogs"
        />
        <AppAlert v-else-if="error" tone="danger" :title="t('dashboard.messages.loadFailed')" :description="error" class="connections-card__chart-state">
          <template #action><AppButton size="sm" :loading="loading" @click="reload">{{ t('ui.retry') }}</AppButton></template>
        </AppAlert>
        <div v-else class="connections-card__chart-state"><AppSkeleton :rows="5" /></div>
        <div v-if="incidentKinds.length" class="connections-card__track-legend">
          <span v-for="kind in incidentKinds" :key="kind"><i :data-kind="kind" />{{ t(`dashboard.messages.incidents.${kind}`) }}</span>
        </div>
      </div>

      <div class="connections-card__panel">
        <div class="connections-card__panel-head">
          <span class="connections-card__label">
            {{ totalLabel }}
            <span v-if="freshness" class="connections-card__live" :data-live="freshness.live || undefined" :title="freshness.hint">
              <span class="connections-card__signal" aria-hidden="true"><i v-if="freshness.live && pulse" :key="pulse" /></span>{{ freshness.text }}
            </span>
          </span>
          <AppSegmented v-if="entries.length > 1" v-model="splitChoice" class="connections-card__split" :label="t('dashboard.messages.split.label')" :options="[{ value: 'connection', label: t('dashboard.messages.split.connection') }, { value: 'direction', label: t('dashboard.messages.split.direction') }]" />
        </div>
        <div class="connections-card__total">
          <template v-if="stats">
            <b>{{ fmt(shownTotal) }}</b><small>{{ t('dashboard.messages.unit') }}</small>
            <MessageDelta :value="total" :previous="previousTotal" />
          </template>
          <AppSkeleton v-else :rows="1" class="connections-card__total-skeleton" />
        </div>
        <p class="connections-card__sub">{{ subline }}</p>
        <div v-if="layers && total > 0" class="connections-card__composition" aria-hidden="true">
          <i v-for="layer in layers.order.filter(item => item.total > 0)" :key="layer.key" :style="{ flexGrow: layer.total, background: layerColor(layer.color) }" />
        </div>
        <div v-if="layers && split === 'direction'" class="connections-card__direction-legend">
          <span v-for="layer in layers.order" :key="layer.key"><i :style="{ background: layerColor(layer.color) }" />{{ names[layer.key] }}<template v-if="total > 0"> {{ Math.round((layer.total / total) * 100) }}%</template></span>
        </div>

        <ul class="connections-card__rows">
          <li v-for="entry in arrangement.shown" :key="entry.id" @pointerenter="focusEntry(entry)" @pointerleave="focusEntry(null)">
            <ConnectionStatsRow
              :entry="entry"
              :color="colorOf(entry)"
              :share="shareOf(entry)"
              :compared="compared"
              :incidents="stats?.incidents"
              :solo="solo"
              :subject="periodSubject"
              :class="{ 'is-focus': focusKey === entry.id }"
            />
          </li>
          <li v-if="arrangement.hidden.length" @pointerenter="focusKey = split === 'connection' ? 'other' : null" @pointerleave="focusKey = null">
            <AppPopover v-model:open="allOpen" side="bottom" align="end" :width="600" :label="t('dashboard.messages.allTitle', { count: entries.length })">
              <button type="button" class="connections-card__more" :class="{ 'is-focus': focusKey === 'other' }" :aria-expanded="allOpen">
                <span class="connections-card__more-tile" aria-hidden="true"><LayersIcon /></span>
                <span class="connections-card__more-who">
                  <i v-if="split === 'connection'" :style="{ background: layerColor('other') }" aria-hidden="true" />
                  <span>{{ t('dashboard.messages.other', { count: arrangement.hidden.length }) }}</span>
                </span>
                <span v-if="stats" class="connections-card__more-count"><b>{{ fmt(hiddenTotal) }}</b><small>{{ total > 0 ? Math.round((hiddenTotal / total) * 100) : 0 }}%</small></span>
                <ChevronDownIcon class="connections-card__more-chevron" aria-hidden="true" />
                <span class="connections-card__more-meta">{{ hiddenNames }}</span>
                <span class="connections-card__more-all">{{ t('dashboard.messages.allConnections', { count: entries.length }) }}</span>
              </button>
              <template #content>
                <div class="connections-all">
                  <div class="connections-all__head">
                    <b>{{ t('dashboard.messages.allTitle', { count: entries.length }) }}</b><small>{{ periodLabel }}</small>
                    <button type="button" class="connections-all__close" :aria-label="t('dashboard.messages.collapse')" @click="allOpen = false"><XIcon aria-hidden="true" /></button>
                  </div>
                  <div class="connections-all__body">
                    <section v-for="group in groups" :key="group.key" class="connections-all__group">
                      <h4>{{ t(`dashboard.messages.groups.${group.key}`) }} · {{ group.entries.length }}<small v-if="group.note">{{ group.note }}</small></h4>
                      <ul>
                        <li v-for="entry in group.entries" :key="entry.id" @pointerenter="focusEntry(entry)" @pointerleave="focusEntry(null)">
                          <ConnectionStatsRow :entry="entry" :color="colorOf(entry)" :share="shareOf(entry)" :compared="compared" :incidents="stats?.incidents" :subject="periodSubject" wide @click="allOpen = false" />
                        </li>
                      </ul>
                    </section>
                  </div>
                  <div class="connections-all__foot">
                    <MotionRouterLink :to="buildProtocolsLocation({ view: 'add' })" class="connections-all__add" @click="allOpen = false"><PlusIcon aria-hidden="true" />{{ t('dashboard.hub.addConnection') }}</MotionRouterLink>
                    <MotionRouterLink :to="{ name: 'protocols' }" class="connections-card__link" @click="allOpen = false">{{ t('dashboard.messages.manage') }}<ChevronRightIcon aria-hidden="true" /></MotionRouterLink>
                  </div>
                </div>
              </template>
            </AppPopover>
          </li>
          <li v-else-if="showAdd">
            <MotionRouterLink :to="buildProtocolsLocation({ view: 'add' })" class="connections-card__add">
              <span class="connections-card__add-icon" aria-hidden="true"><PlusIcon /></span>{{ t('dashboard.hub.addConnection') }}
            </MotionRouterLink>
          </li>
        </ul>
      </div>
    </div>
  </section>
</template>

<style scoped lang="scss">
.connections-card { position: relative; display: flex; flex-direction: column; min-width: 0; padding: 16px 20px 18px; }
.connections-card__header { display: flex; align-items: center; gap: 10px; min-height: 40px; margin-bottom: 8px; }
.connections-card__title { margin: 0; color: var(--text); font-size: var(--font-size-lg); font-weight: 700; letter-spacing: -0.01em; }
.connections-card__meta { color: var(--muted); font-size: var(--font-size-sm); font-variant-numeric: tabular-nums; }
.connections-card__link { display: inline-flex; align-items: center; gap: 2px; border-radius: var(--radius-sm); color: var(--brand-foreground); font-size: var(--font-size-sm); font-weight: 500; }
.connections-card__link:hover { text-decoration: underline; text-underline-offset: 3px; }
.connections-card__link:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
.connections-card__link svg { width: 15px; height: 15px; }
.connections-card__period { margin-left: auto; }

// The chart takes the room; the right column holds the figures and four fixed row slots, so the card keeps its height
// however many connections there are.
.connections-card__body { display: grid; grid-template-columns: minmax(0, 1fr) 452px; gap: 32px; align-items: start; }
.connections-card__chart { min-width: 0; }
.connections-card__chart-state { display: grid; align-content: center; min-height: 330px; }
.connections-card__track-legend { display: flex; gap: 16px; min-height: 18px; margin: 4px 0 0 44px; color: var(--muted); font-size: var(--font-size-xs); }
.connections-card__track-legend span { display: inline-flex; align-items: center; gap: 6px; }
.connections-card__track-legend i { width: 14px; height: 5px; border-radius: 3px; background: var(--muted); }
.connections-card__track-legend i[data-kind=adapter_offline] { background: var(--warning); }

.connections-card__panel { display: grid; min-width: 0; }
.connections-card__panel-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 30px; color: var(--muted); font-size: var(--font-size-sm); }
.connections-card__label { display: inline-flex; align-items: center; gap: 10px; min-width: 0; }
// The live mark is the reconnect signal at rest: a dot on a soft disc that sends one ring when new counts arrive.
.connections-card__live { display: inline-flex; align-items: center; gap: 5px; color: var(--muted); font-size: var(--font-size-xs); font-weight: 500; white-space: nowrap; }
.connections-card__live[data-live] { color: var(--success); }
.connections-card__signal { position: relative; z-index: 0; display: grid; flex: none; place-items: center; width: 14px; height: 14px; border-radius: 50%; background: var(--surface-soft); }
.connections-card__signal::before { width: 6px; height: 6px; border-radius: 50%; background: var(--muted); content: ''; }
.connections-card__live[data-live] .connections-card__signal { background: var(--success-soft); }
.connections-card__live[data-live] .connections-card__signal::before { background: var(--success); }
.connections-card__signal i { position: absolute; z-index: -1; inset: 0; border-radius: 50%; background: var(--success-soft); animation: connections-card-ping 1.4s cubic-bezier(0, 0, 0.2, 1) both; }
@keyframes connections-card-ping {
  from { opacity: 1; transform: scale(1); }
  to { opacity: 0; transform: scale(2.4); }
}
@media (prefers-reduced-motion: reduce) { .connections-card__signal i { animation: none; opacity: 0; } }
.connections-card__split :deep(.app-segmented__item) { min-height: 26px; padding: 2px 10px; font-size: var(--font-size-xs); }
.connections-card__total { display: flex; align-items: baseline; gap: 8px; min-height: 40px; }
.connections-card__total b { font-size: 34px; font-weight: 700; line-height: 1.15; letter-spacing: -0.02em; font-variant-numeric: tabular-nums; }
.connections-card__total small { color: var(--muted); font-size: var(--font-size-md); font-weight: 500; }
.connections-card__total .message-delta { align-self: center; }
.connections-card__total-skeleton { width: 180px; }
.connections-card__sub { min-height: 18px; margin: 0; color: var(--muted); font-size: var(--font-size-xs); font-variant-numeric: tabular-nums; }
.connections-card__composition { display: flex; gap: 2px; height: 8px; margin: 14px 0 6px; }
.connections-card__composition i { display: block; min-width: 3px; border-radius: 2px; }
.connections-card__composition i:first-child { border-radius: 999px 2px 2px 999px; }
.connections-card__composition i:last-child { border-radius: 2px 999px 999px 2px; }
.connections-card__composition i:only-child { border-radius: 999px; }
.connections-card__direction-legend { display: flex; gap: 16px; color: var(--muted); font-size: var(--font-size-xs); }
.connections-card__direction-legend span { display: inline-flex; align-items: center; gap: 6px; }
.connections-card__direction-legend i { width: 10px; height: 10px; border-radius: 3px; }

.connections-card__rows { display: grid; grid-template-rows: repeat(4, 58px); gap: 2px; margin: 6px -10px 0; padding: 0; list-style: none; }
.connections-card__rows > li { min-width: 0; }

// "其他 N 个连接" is the fourth slot when there are more connections: it opens every connection in a floating list.
.connections-card__more { display: grid; width: 100%; grid-template-columns: 34px minmax(0, 1fr) auto 14px; grid-template-areas: "tile who count go" "tile meta all go"; align-items: center; column-gap: 12px; row-gap: 1px; min-height: 56px; padding: 7px 10px; border: 0; border-radius: var(--radius-md); background: none; color: var(--text); font: inherit; text-align: left; cursor: pointer; transition: background-color var(--motion-fast) var(--motion-easing); }
.connections-card__more:hover, .connections-card__more.is-focus, .connections-card__more[aria-expanded=true] { background: var(--control-fill-hover); }
.connections-card__more:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
.connections-card__more-tile { grid-area: tile; display: grid; place-items: center; width: 34px; height: 34px; border-radius: 11px; background: var(--surface-soft); color: var(--muted); }
.connections-card__more-tile svg { width: 17px; height: 17px; }
.connections-card__more-who { grid-area: who; display: flex; align-items: center; gap: 8px; min-width: 0; font-weight: 500; }
.connections-card__more-who i { width: 10px; height: 10px; flex: none; border-radius: 3px; }
.connections-card__more-count { grid-area: count; display: flex; align-items: baseline; gap: 8px; }
.connections-card__more-count b { font-size: var(--font-size-md); font-weight: 700; font-variant-numeric: tabular-nums; }
.connections-card__more-count small { width: 32px; color: var(--muted); font-size: var(--font-size-xs); text-align: right; }
.connections-card__more-chevron { grid-area: go; width: 15px; height: 15px; color: var(--muted); transition: transform var(--motion-fast) var(--motion-easing); }
.connections-card__more[aria-expanded=true] .connections-card__more-chevron { transform: rotate(180deg); }
.connections-card__more-meta { grid-area: meta; overflow: hidden; color: var(--muted); font-size: var(--font-size-xs); text-overflow: ellipsis; white-space: nowrap; }
.connections-card__more-all { grid-area: all; justify-self: end; color: var(--brand-foreground); font-size: var(--font-size-xs); font-weight: 500; }

.connections-card__add { display: flex; align-items: center; gap: 12px; height: 100%; padding: 0 10px; border-radius: var(--radius-md); color: var(--brand-foreground); font-weight: 500; }
.connections-card__add:hover { background: var(--control-fill-hover); }
.connections-card__add:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
.connections-card__add-icon { display: grid; place-items: center; width: 34px; height: 34px; border-radius: 11px; background: var(--control-fill); box-shadow: var(--shadow-xs); }
.connections-card__add-icon svg { width: 16px; height: 16px; }

// No connection yet: what will appear here, and the one action that gets there.
.connections-card__empty { display: grid; place-items: center; align-content: center; gap: 10px; min-height: 300px; text-align: center; }
.connections-card__empty-glyph { display: grid; place-items: center; width: 56px; height: 56px; border-radius: 18px; background: var(--control-fill); color: var(--brand-foreground); box-shadow: var(--shadow-xs); }
.connections-card__empty-glyph svg { width: 26px; height: 26px; }
.connections-card__empty h3 { margin: 0; font-size: var(--font-size-md); font-weight: 700; }
.connections-card__empty p { max-width: 420px; margin: 0; color: var(--muted); font-size: var(--font-size-sm); }

// Every connection, floating over the page; the card never grows for them.
// Short enough to open below the 其他 row on a 1080-pixel screen instead of flipping over the chart.
.connections-all { display: flex; flex-direction: column; max-height: min(440px, calc(var(--reka-popover-content-available-height, 440px) - 8px)); margin: -16px; }
.connections-all__head { display: flex; align-items: center; gap: 10px; padding: 14px 14px 8px 20px; }
.connections-all__head b { font-size: var(--font-size-md); font-weight: 700; }
.connections-all__head small { color: var(--muted); font-size: var(--font-size-xs); }
.connections-all__close { display: grid; place-items: center; width: 32px; height: 32px; margin-left: auto; border: 0; border-radius: 50%; background: none; color: var(--muted); cursor: pointer; }
.connections-all__close:hover { background: var(--nav-hover); color: var(--text); }
.connections-all__close:focus-visible { outline: 2px solid var(--focus); outline-offset: 0; }
.connections-all__close svg { width: 16px; height: 16px; }
.connections-all__body { flex: 1; min-height: 0; overflow-y: auto; overscroll-behavior: contain; padding: 0 10px 8px; }
.connections-all__group h4 { display: flex; align-items: baseline; gap: 8px; margin: 0; padding: 10px 10px 4px; color: var(--muted); font-size: var(--font-size-xs); font-weight: 500; }
.connections-all__group h4 small { font-size: var(--font-size-xs); font-weight: 400; }
.connections-all__group ul { display: grid; gap: 2px; margin: 0; padding: 0; list-style: none; }
.connections-all__foot { display: flex; align-items: center; gap: 16px; padding: 12px 20px 14px; border-top: 1px solid var(--border); font-size: var(--font-size-sm); }
.connections-all__add { display: inline-flex; align-items: center; gap: 6px; border-radius: var(--radius-sm); color: var(--brand-foreground); font-weight: 500; }
.connections-all__add:hover { text-decoration: underline; text-underline-offset: 3px; }
.connections-all__add:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
.connections-all__add svg { width: 15px; height: 15px; }
.connections-all__foot .connections-card__link { margin-left: auto; }
@media (prefers-reduced-motion: reduce) { .connections-card__more, .connections-card__more-chevron { transition: none; } }
@media (forced-colors: active) { .connections-card__composition i, .connections-card__direction-legend i, .connections-card__track-legend i { border: 1px solid CanvasText; } }
</style>
