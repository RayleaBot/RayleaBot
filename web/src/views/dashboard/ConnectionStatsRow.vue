<script setup lang="ts">
import { computed } from 'vue'
import { BotIcon, ChevronRightIcon, CircleCheckIcon, CircleMinusIcon, CircleXIcon, RadioTowerIcon, TriangleAlertIcon } from '@lucide/vue'

import MotionRouterLink from '@/components/shell/MotionRouterLink.vue'
import { i18n, t } from '@/i18n'
import { getAdapterStateLabel } from '@/lib/display'
import { formatRelativeTime, managementTimeZone } from '@/lib/format'
import { buildProtocolsLocation } from '@/lib/management-links'
import type { MessageStatsIncident } from '@/types/api'
import type { StatusRowTone } from '@/views/dashboard/dashboard-status'
import MessageDelta from '@/views/dashboard/MessageDelta.vue'
import type { ConnectionEntry } from '@/views/dashboard/message-stats'

const props = withDefaults(defineProps<{
  entry: ConnectionEntry
  // Series colour when the connection is drawn as its own layer.
  color?: string | null
  // Share of the period's messages, 0..1; omitted when the counts are not read yet.
  share?: number | null
  // Whether the comparison with the previous period exists at all.
  compared?: boolean
  incidents?: MessageStatsIncident[]
  // The wider floating list also names the protocol.
  wide?: boolean
  // A single connection is the total, so its row only reports status.
  solo?: boolean
}>(), { color: null, share: null, compared: false, incidents: () => [], wide: false, solo: false })

const adapter = computed(() => props.entry.adapter)
const protocol = computed(() => adapter.value?.protocol ?? props.entry.stats?.protocol ?? 'onebot11')
const protocolLabel = computed(() => (protocol.value === 'qqofficial' ? t('protocols.qqTitle') : 'OneBot11'))
const title = computed(() => adapter.value?.identity?.name || adapter.value?.display_name || props.entry.id)
const account = computed(() => {
  const identity = adapter.value?.identity?.id
  if (identity) return `${protocol.value === 'qqofficial' ? t('protocols.connectionCard.botId') : 'QQ'} ${identity}`
  return t('dashboard.hub.connectionIdentifier', { id: props.entry.id })
})
const stateText = computed(() => {
  if (props.entry.stage === 'removed') return t('dashboard.messages.removed')
  if (props.entry.stage === 'disabled') return t('dashboard.hub.connectionDisabled')
  return getAdapterStateLabel(adapter.value?.state)
})
const stateIcons: Record<StatusRowTone, unknown> = { success: CircleCheckIcon, warning: TriangleAlertIcon, danger: CircleXIcon, info: CircleMinusIcon, muted: CircleMinusIcon }
const lastReceived = computed(() => {
  const at = props.entry.stats?.last_received_at
  return at ? t('dashboard.messages.lastReceived', { time: formatRelativeTime(at) }) : t('dashboard.messages.neverReceived')
})
const meta = computed(() => {
  if (props.entry.stage === 'removed') return t('dashboard.messages.removedDetail')
  // A connection in trouble says what is happening instead of when it last heard something.
  if (props.entry.stage === 'problem' && adapter.value?.summary) return [props.wide ? protocolLabel.value : '', adapter.value.summary].filter(Boolean).join(' · ')
  return [props.wide || !adapter.value?.identity ? protocolLabel.value : '', account.value, lastReceived.value].filter(Boolean).join(' · ')
})

// Finished outages in the period; one that is still going on is already told by the state.
const timeFormat = computed(() => new Intl.DateTimeFormat(i18n.global.locale.value, { month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit', hourCycle: 'h23', timeZone: managementTimeZone() }))
const outages = computed(() => props.incidents.filter(item => item.kind === 'adapter_offline' && item.adapter_id === props.entry.id && item.ended_at))
const outageTitle = computed(() => outages.value.map(item => `${timeFormat.value.format(new Date(item.started_at))} – ${timeFormat.value.format(new Date(item.ended_at!))}`).join('\n'))

const numberFormat = computed(() => new Intl.NumberFormat(i18n.global.locale.value))
const link = computed(() => (props.entry.stage === 'removed' ? null : buildProtocolsLocation({ adapterId: props.entry.id })))
</script>

<template>
  <component :is="link ? MotionRouterLink : 'div'" :to="link ?? undefined" class="connection-row" :class="{ 'connection-row--link': link }">
    <span class="connection-row__tile" :data-tone="entry.tone" aria-hidden="true"><BotIcon v-if="protocol === 'qqofficial'" /><RadioTowerIcon v-else /></span>
    <span class="connection-row__who">
      <i v-if="color" class="connection-row__swatch" :style="{ background: color }" aria-hidden="true" />
      <span class="connection-row__title">{{ title }}</span>
      <span class="connection-row__state" :data-tone="entry.tone"><component :is="stateIcons[entry.tone]" aria-hidden="true" />{{ stateText }}</span>
      <span v-if="outages.length" class="connection-row__outage" :title="outageTitle">{{ t('dashboard.messages.outages', { count: outages.length }) }}</span>
    </span>
    <span v-if="!solo && entry.stats" class="connection-row__count">
      <b>{{ numberFormat.format(entry.total) }}</b><small v-if="share !== null">{{ Math.round(share * 100) }}%</small>
    </span>
    <span class="connection-row__go" aria-hidden="true"><ChevronRightIcon v-if="link" /></span>
    <span class="connection-row__meta">{{ meta }}</span>
    <MessageDelta v-if="!solo && entry.stats" class="connection-row__delta" :value="entry.total" :previous="compared ? entry.previous : null" />
  </component>
</template>

<style scoped lang="scss">
// Tile over two lines; name, state and outages first, then account and the last message, counts on the right.
.connection-row { display: grid; grid-template-columns: 34px minmax(0, 1fr) auto 14px; grid-template-areas: "tile who count go" "tile meta delta go"; align-items: center; column-gap: 12px; row-gap: 1px; min-height: 56px; padding: 7px 10px; border-radius: var(--radius-md); color: var(--text); }
.connection-row--link { transition: background-color var(--motion-fast) var(--motion-easing); }
.connection-row--link:hover, .connection-row.is-focus { background: var(--control-fill-hover); }
.connection-row--link:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
.connection-row__tile { grid-area: tile; display: grid; place-items: center; width: 34px; height: 34px; border-radius: 11px; }
.connection-row__tile svg { width: 17px; height: 17px; }
.connection-row__tile[data-tone=success] { background: var(--success-soft); color: var(--success); }
.connection-row__tile[data-tone=warning] { background: var(--warning-soft); color: var(--warning); }
.connection-row__tile[data-tone=danger] { background: var(--danger-soft); color: var(--danger); }
.connection-row__tile[data-tone=info] { background: var(--info-soft); color: var(--info); }
.connection-row__tile[data-tone=muted] { background: var(--surface-soft); color: var(--muted); }
.connection-row__who { grid-area: who; display: flex; align-items: center; gap: 8px; min-width: 0; white-space: nowrap; }
.connection-row__swatch { flex: none; width: 10px; height: 10px; border-radius: 3px; }
.connection-row__title { overflow: hidden; font-weight: 500; text-overflow: ellipsis; }
.connection-row__state { display: inline-flex; flex: none; align-items: center; gap: 4px; font-size: var(--font-size-xs); font-weight: 500; }
.connection-row__state svg { width: 14px; height: 14px; }
.connection-row__state[data-tone=success] { color: var(--success); }
.connection-row__state[data-tone=warning] { color: var(--warning); }
.connection-row__state[data-tone=danger] { color: var(--danger); }
.connection-row__state[data-tone=info] { color: var(--info); }
.connection-row__state[data-tone=muted] { color: var(--muted); }
.connection-row__outage { flex: none; color: var(--warning); font-size: var(--font-size-xs); font-weight: 500; }
.connection-row__count { grid-area: count; display: flex; align-items: baseline; justify-content: flex-end; gap: 8px; }
.connection-row__count b { font-size: var(--font-size-md); font-weight: 700; font-variant-numeric: tabular-nums; }
.connection-row__count small { width: 32px; color: var(--muted); font-size: var(--font-size-xs); font-variant-numeric: tabular-nums; text-align: right; }
.connection-row__meta { grid-area: meta; overflow: hidden; color: var(--muted); font-size: var(--font-size-xs); font-variant-numeric: tabular-nums; text-overflow: ellipsis; white-space: nowrap; }
.connection-row__delta { grid-area: delta; justify-self: end; }
.connection-row__go { grid-area: go; display: grid; place-items: center; color: var(--muted); visibility: hidden; }
.connection-row__go svg { width: 15px; height: 15px; }
.connection-row--link:hover .connection-row__go, .connection-row--link:focus-visible .connection-row__go { visibility: visible; }
@media (prefers-reduced-motion: reduce) { .connection-row--link { transition: none; } }
@media (forced-colors: active) { .connection-row__tile, .connection-row__swatch { border: 1px solid CanvasText; } }
</style>
