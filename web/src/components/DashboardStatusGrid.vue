<script setup lang="ts">
import {
  AppstoreOutlined,
  FieldTimeOutlined,
  HeartOutlined,
  SafetyOutlined,
} from '@ant-design/icons-vue'

import type { StatusType } from '@/lib/display'

const iconMap = {
  health: HeartOutlined,
  readiness: SafetyOutlined,
  plugins: AppstoreOutlined,
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
  uptimeLabel: string
  uptimeText: string
}>()
</script>

<template>
  <div class="stats-grid dashboard-overview-grid" data-testid="dashboard-overview-grid">
    <a-card :bordered="false" :class="['stat-card bento-stat', `stat-card--${healthStatusType}`]">
      <component :is="iconMap.health" class="bento-stat__bg-icon" />
      <span class="stat-label">{{ healthLabel }}</span>
      <strong class="bento-stat__value">{{ healthValueText }}</strong>
      <small class="bento-stat__detail">{{ healthDetailText }}</small>
    </a-card>
    <a-card :bordered="false" :class="['stat-card bento-stat', `stat-card--${readinessStatusType}`]">
      <component :is="iconMap.readiness" class="bento-stat__bg-icon" />
      <span class="stat-label">{{ readinessLabel }}</span>
      <strong class="bento-stat__value">{{ readinessValueText }}</strong>
      <small class="bento-stat__detail">{{ readinessDetailText }}</small>
    </a-card>
    <a-card :bordered="false" class="stat-card bento-stat">
      <component :is="iconMap.plugins" class="bento-stat__bg-icon" />
      <span class="stat-label">{{ activePluginsLabel }}</span>
      <strong class="bento-stat__value">{{ activePluginsCount }}</strong>
    </a-card>
    <a-card :bordered="false" class="stat-card bento-stat">
      <component :is="iconMap.uptime" class="bento-stat__bg-icon" />
      <span class="stat-label">{{ uptimeLabel }}</span>
      <strong class="bento-stat__value">{{ uptimeText }}</strong>
    </a-card>
  </div>
</template>

<style scoped lang="scss">
.bento-stat {
  position: relative;
  overflow: hidden;
}

.bento-stat__bg-icon {
  position: absolute;
  top: 8px;
  right: 8px;
  font-size: 3.2rem;
  color: color-mix(in srgb, var(--accent) 14%, transparent);
  opacity: 0.8;
  pointer-events: none;
  transform: rotate(12deg);
}

.bento-stat__value {
  display: block;
  font-size: 1.5rem;
  font-weight: 700;
  line-height: 1.2;
  margin-top: 6px;
}

.bento-stat__detail {
  display: block;
  margin-top: 6px;
  color: var(--muted);
  line-height: 1.45;
}

.stat-card--success .bento-stat__bg-icon {
  color: color-mix(in srgb, var(--success) 18%, transparent);
}

.stat-card--warning .bento-stat__bg-icon {
  color: color-mix(in srgb, var(--warning) 18%, transparent);
}

.stat-card--danger .bento-stat__bg-icon {
  color: color-mix(in srgb, var(--danger) 18%, transparent);
}
</style>
