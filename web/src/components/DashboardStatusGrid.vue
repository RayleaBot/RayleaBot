<script setup lang="ts">
import {
  AppstoreOutlined,
  FieldTimeOutlined,
  HeartOutlined,
  SafetyOutlined,
} from '@ant-design/icons-vue'
import type { RouteLocationRaw } from 'vue-router'

import MotionRouterLink from '@/components/shell/MotionRouterLink.vue'
import type { StatusType } from '@/lib/display'

const iconMap = {
  health: HeartOutlined,
  plugins: AppstoreOutlined,
  readiness: SafetyOutlined,
  uptime: FieldTimeOutlined,
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
  <section class="dashboard-status-grid" data-testid="dashboard-overview-grid" aria-label="系统运行摘要">
    <div class="dashboard-status-item" :data-tone="healthStatusType">
      <div class="dashboard-status-item__icon">
        <component :is="iconMap.health" class="dashboard-status-item__glyph" />
      </div>
      <div class="dashboard-status-item__body">
        <span>{{ healthLabel }}</span>
        <strong>{{ healthValueText }}</strong>
        <small>{{ healthDetailText }}</small>
      </div>
    </div>

    <div class="dashboard-status-item" :data-tone="readinessStatusType">
      <div class="dashboard-status-item__icon">
        <component :is="iconMap.readiness" class="dashboard-status-item__glyph" />
      </div>
      <div class="dashboard-status-item__body">
        <span>{{ readinessLabel }}</span>
        <strong>{{ readinessValueText }}</strong>
        <small>{{ readinessDetailText }}</small>
      </div>
    </div>

    <MotionRouterLink
      :to="activePluginsTo"
      class="dashboard-status-item dashboard-status-item--link"
      data-tone="info"
      data-testid="dashboard-active-plugins-card"
      :aria-label="activePluginsAriaLabel"
    >
      <div class="dashboard-status-item__icon">
        <component :is="iconMap.plugins" class="dashboard-status-item__glyph" />
      </div>
      <div class="dashboard-status-item__body">
        <span>{{ activePluginsLabel }}</span>
        <strong>{{ activePluginsCount }}</strong>
        <small>{{ activePluginsDetailText }}</small>
      </div>
    </MotionRouterLink>

    <div class="dashboard-status-item" data-tone="neutral">
      <div class="dashboard-status-item__icon">
        <component :is="iconMap.uptime" class="dashboard-status-item__glyph" />
      </div>
      <div class="dashboard-status-item__body">
        <span>{{ uptimeLabel }}</span>
        <strong class="monospace">{{ uptimeText }}</strong>
        <small>{{ runtimeMetaText }}</small>
      </div>
    </div>
  </section>
</template>

<style scoped lang="scss">
.dashboard-status-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: var(--app-card-radius);
  background: var(--surface-strong);
  box-shadow: var(--shadow-xs);
}

.dashboard-status-item {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  padding: 16px;
}

.dashboard-status-item + .dashboard-status-item {
  border-inline-start: 1px solid var(--border);
}

.dashboard-status-item--link {
  color: inherit;
  text-decoration: none;
  transition: background-color var(--motion-fast) var(--motion-easing);

  &:hover {
    background: var(--surface-accent);
  }
  &:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -3px;
  }
}

.dashboard-status-item__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  color: var(--muted);
  flex-shrink: 0;
}

.dashboard-status-item__glyph {
  font-size: 20px;
}

.dashboard-status-item__body {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.dashboard-status-item__body span,
.dashboard-status-item__body small {
  color: var(--muted);
  font-size: 13px;
  overflow-wrap: anywhere;
}

.dashboard-status-item__body strong {
  color: var(--text);
  font-size: 18px;
  line-height: 1.3;

  &.monospace {
    font-family: var(--font-mono);
    font-size: 16px;
  }
}

.dashboard-status-item[data-tone='success'] .dashboard-status-item__icon {
  color: var(--success);
}

.dashboard-status-item[data-tone='warning'] .dashboard-status-item__icon {
  color: var(--warning);
}

.dashboard-status-item[data-tone='danger'] .dashboard-status-item__icon {
  color: var(--danger);
}

.dashboard-status-item[data-tone='info'] .dashboard-status-item__icon {
  color: var(--accent);
}

@media (max-width: 899px) {
  .dashboard-status-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dashboard-status-item:nth-child(3) {
    border-inline-start: 0;
  }

  .dashboard-status-item:nth-child(n + 3) {
    border-top: 1px solid var(--border);
  }
}

@media (max-width: 639px) {
  .dashboard-status-grid {
    grid-template-columns: 1fr;
  }

  .dashboard-status-item + .dashboard-status-item {
    border-inline-start: 0;
    border-top: 1px solid var(--border);
  }
}
</style>
