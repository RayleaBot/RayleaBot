<script setup lang="ts">
import { computed } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
import { ChevronRightIcon, CircleCheckIcon, CircleDashedIcon, CircleMinusIcon, CircleXIcon, TriangleAlertIcon } from '@lucide/vue'

import MotionRouterLink from '@/components/shell/MotionRouterLink.vue'
import type { StatusRowTone } from '@/views/dashboard/dashboard-status'

const props = defineProps<{
  detail?: string
  status?: string
  statusTone?: StatusRowTone
  title: string
  to?: RouteLocationRaw
  tone: StatusRowTone
}>()

const statusIcons = {
  danger: CircleXIcon,
  info: CircleDashedIcon,
  muted: CircleMinusIcon,
  success: CircleCheckIcon,
  warning: TriangleAlertIcon,
} as const
const resolvedStatusTone = computed(() => props.statusTone ?? props.tone)
</script>

<template>
  <!-- One object or check: a tinted glyph tile, its name and detail, and its state as icon plus text. -->
  <component :is="to ? MotionRouterLink : 'div'" :to="to" class="status-row" :class="{ 'status-row--link': to }">
    <span class="status-row__icon" :data-tone="tone" aria-hidden="true"><slot name="icon" /></span>
    <span class="status-row__copy">
      <span class="status-row__title">{{ title }}</span>
      <span v-if="detail" class="status-row__detail">{{ detail }}</span>
    </span>
    <span v-if="status" class="status-row__status" :data-tone="resolvedStatusTone">
      <component :is="statusIcons[resolvedStatusTone]" aria-hidden="true" />{{ status }}
    </span>
    <ChevronRightIcon v-if="to" class="status-row__chevron" aria-hidden="true" />
  </component>
</template>

<style scoped lang="scss">
.status-row { display: flex; align-items: center; gap: 12px; min-height: 52px; padding: 8px 0; color: var(--text); }
.status-row + .status-row { border-top: 1px solid var(--border); }
.status-row--link { margin-inline: -8px; padding-inline: 8px; border-radius: var(--radius-md); transition: background-color var(--motion-fast) var(--motion-easing); }
.status-row--link:hover { background: var(--control-fill-hover); }
.status-row--link:focus-visible { outline: 2px solid var(--focus); outline-offset: -2px; }
.status-row__icon { display: grid; flex: none; place-items: center; width: 34px; height: 34px; border-radius: 11px; }
.status-row__icon :deep(svg) { width: 17px; height: 17px; }
.status-row__icon[data-tone=success] { background: var(--success-soft); color: var(--success); }
.status-row__icon[data-tone=warning] { background: var(--warning-soft); color: var(--warning); }
.status-row__icon[data-tone=danger] { background: var(--danger-soft); color: var(--danger); }
.status-row__icon[data-tone=info] { background: var(--info-soft); color: var(--info); }
.status-row__icon[data-tone=muted] { background: var(--surface-soft); color: var(--muted); }
.status-row__copy { display: grid; min-width: 0; }
.status-row__title { overflow: hidden; font-size: var(--font-size-md); font-weight: 500; text-overflow: ellipsis; white-space: nowrap; }
.status-row__detail { overflow: hidden; color: var(--muted); font-size: var(--font-size-xs); font-variant-numeric: tabular-nums; text-overflow: ellipsis; white-space: nowrap; }
.status-row__status { display: inline-flex; flex: none; align-items: center; gap: 6px; margin-left: auto; font-size: var(--font-size-sm); font-weight: 500; white-space: nowrap; }
.status-row__status svg { width: 15px; height: 15px; }
.status-row__chevron { flex: none; width: 16px; height: 16px; margin-left: -4px; color: var(--muted); }
.status-row__status[data-tone=success] { color: var(--success); }
.status-row__status[data-tone=warning] { color: var(--warning); }
.status-row__status[data-tone=danger] { color: var(--danger); }
.status-row__status[data-tone=info] { color: var(--info); }
.status-row__status[data-tone=muted] { color: var(--muted); }
@media (prefers-reduced-motion: reduce) { .status-row--link { transition: none; } }
@media (forced-colors: active) { .status-row__icon { border: 1px solid CanvasText; } }
</style>
