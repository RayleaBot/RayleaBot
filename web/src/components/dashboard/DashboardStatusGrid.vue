<script setup lang="ts">
import {
  BlocksIcon,
  TimerIcon,
  HeartIcon,
  ShieldCheckIcon,
} from '@lucide/vue'
import type { RouteLocationRaw } from 'vue-router'

import MotionRouterLink from '@/components/shell/MotionRouterLink.vue'
import { t } from '@/i18n'
import type { StatusType } from '@/lib/display'

const iconMap = {
  health: HeartIcon,
  plugins: BlocksIcon,
  readiness: ShieldCheckIcon,
  uptime: TimerIcon,
} as const

defineProps<{
  healthStatusType: StatusType
  readinessStatusType: StatusType
  healthLabel: string
  healthValueText: string
  healthDetailText: string
  readinessLabel: string
  readinessValueText: string
  readinessDetailText: string
  activePluginsLabel: string
  activePluginsCount: number
  activePluginsDetailText: string
  activePluginsTo: RouteLocationRaw
  activePluginsAriaLabel: string
  uptimeLabel: string
  uptimeText: string
  runtimeMetaText: string
}>()
</script>

<template>
  <section class="dashboard-status-grid" data-testid="dashboard-overview-grid" :aria-label="t('dashboard.statusSummaryLabel')">
    <div class="dashboard-status-item liquid-glass liquid-glass--strong" data-glass="clear" :data-tone="healthStatusType">
      <span class="dashboard-status-item__icon" aria-hidden="true">
        <component :is="iconMap.health" class="dashboard-status-item__glyph" />
      </span>
      <div class="dashboard-status-item__body">
        <span>{{ healthLabel }}</span>
        <strong>{{ healthValueText }}</strong>
        <small>{{ healthDetailText }}</small>
      </div>
    </div>

    <div class="dashboard-status-item liquid-glass liquid-glass--strong" data-glass="clear" :data-tone="readinessStatusType">
      <span class="dashboard-status-item__icon" aria-hidden="true">
        <component :is="iconMap.readiness" class="dashboard-status-item__glyph" />
      </span>
      <div class="dashboard-status-item__body">
        <span>{{ readinessLabel }}</span>
        <strong>{{ readinessValueText }}</strong>
        <small>{{ readinessDetailText }}</small>
      </div>
    </div>

    <MotionRouterLink
      :to="activePluginsTo"
      class="dashboard-status-item dashboard-status-item--link liquid-glass liquid-glass--strong"
      data-glass="clear"
      data-tone="brand"
      data-testid="dashboard-active-plugins-card"
      :aria-label="activePluginsAriaLabel"
    >
      <span class="dashboard-status-item__icon" aria-hidden="true">
        <component :is="iconMap.plugins" class="dashboard-status-item__glyph" />
      </span>
      <div class="dashboard-status-item__body">
        <span>{{ activePluginsLabel }}</span>
        <strong>{{ activePluginsCount }}</strong>
        <small>{{ activePluginsDetailText }}</small>
      </div>
    </MotionRouterLink>

    <div class="dashboard-status-item liquid-glass liquid-glass--strong" data-glass="clear" data-tone="neutral">
      <span class="dashboard-status-item__icon" aria-hidden="true">
        <component :is="iconMap.uptime" class="dashboard-status-item__glyph" />
      </span>
      <div class="dashboard-status-item__body">
        <span>{{ uptimeLabel }}</span>
        <strong>{{ uptimeText }}</strong>
        <small>{{ runtimeMetaText }}</small>
      </div>
    </div>
  </section>
</template>

<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;
// Summary capsules float on the light field as text-bearing glass; each icon sits in a soft disc of
// its status colour so the state reads before the words.
.dashboard-status-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.dashboard-status-item {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  padding: 11px 20px 11px 11px;
  border-radius: 999px;
  color: var(--text);
}

.dashboard-status-item--link {
  text-decoration: none;
  transition: translate var(--motion-fast) var(--motion-easing);

  &:hover {
    translate: 0 -1px;
  }

  &:focus-visible {
    outline: 2px solid var(--focus);
    outline-offset: 3px;
  }
}

.dashboard-status-item__icon {
  display: grid;
  flex-shrink: 0;
  place-items: center;
  width: 42px;
  height: 42px;
  border-radius: 50%;
  background: color-mix(in srgb, var(--text) 8%, transparent);
  color: var(--muted);
}

.dashboard-status-item__glyph {
  width: 20px;
  height: 20px;
  stroke-width: 2;
}

.dashboard-status-item__body {
  display: grid;
  min-width: 0;
}

.dashboard-status-item__body span,
.dashboard-status-item__body small {
  overflow: hidden;
  color: var(--muted);
  font-size: 12.5px;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dashboard-status-item__body strong {
  color: var(--text);
  font-size: 17px;
  font-weight: 700;
  line-height: 1.35;
  font-variant-numeric: tabular-nums;
}

.dashboard-status-item[data-tone='success'] .dashboard-status-item__icon { background: var(--surface-success); color: var(--text-success); }
.dashboard-status-item[data-tone='warning'] .dashboard-status-item__icon { background: var(--surface-warning); color: var(--text-warning); }
.dashboard-status-item[data-tone='danger'] .dashboard-status-item__icon { background: var(--surface-danger); color: var(--text-danger); }
.dashboard-status-item[data-tone='brand'] .dashboard-status-item__icon { background: color-mix(in srgb, var(--brand-fill) 20%, transparent); color: var(--brand-foreground); }

@media (max-width: #{bp.$dashboard - 1px}) {
  .dashboard-status-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: #{bp.$phone - 1px}) {
  .dashboard-status-item { gap: 10px; padding: 10px 14px 10px 10px; border-radius: 28px; }
  .dashboard-status-item__icon { width: 36px; height: 36px; }
  .dashboard-status-item__body strong { font-size: 16px; }
}

@media (prefers-reduced-motion: reduce) {
  .dashboard-status-item--link { transition: none; }
  .dashboard-status-item--link:hover { translate: none; }
}
</style>
