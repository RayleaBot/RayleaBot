<script setup lang="ts">
import {
  AppstoreOutlined,
  FieldTimeOutlined,
  HeartOutlined,
  SafetyOutlined,
} from '@ant-design/icons-vue'
import type { RouteLocationRaw } from 'vue-router'

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
  <div class="dashboard-status-grid dashboard-overview-grid" data-testid="dashboard-overview-grid">
    <div :class="['custom-stat-card', 'stat-card', `custom-stat-card--${healthStatusType}`, `stat-card--${healthStatusType}`]">
      <div class="custom-stat-card__icon-container">
        <component :is="iconMap.health" class="custom-stat-card__icon" />
      </div>
      <div class="custom-stat-card__body">
        <span class="custom-stat-card__label">{{ healthLabel }}</span>
        <strong class="custom-stat-card__value">{{ healthValueText }}</strong>
        <span class="custom-stat-card__desc">{{ healthDetailText }}</span>
      </div>
    </div>

    <div :class="['custom-stat-card', 'stat-card', `custom-stat-card--${readinessStatusType}`, `stat-card--${readinessStatusType}`]">
      <div class="custom-stat-card__icon-container">
        <component :is="iconMap.readiness" class="custom-stat-card__icon" />
      </div>
      <div class="custom-stat-card__body">
        <span class="custom-stat-card__label">{{ readinessLabel }}</span>
        <strong class="custom-stat-card__value">{{ readinessValueText }}</strong>
        <span class="custom-stat-card__desc">{{ readinessDetailText }}</span>
      </div>
    </div>

    <RouterLink
      :to="activePluginsTo"
      class="custom-stat-card stat-card custom-stat-card--primary stat-card--primary custom-stat-card--link"
      data-testid="dashboard-active-plugins-card"
      :aria-label="activePluginsAriaLabel"
    >
      <div class="custom-stat-card__icon-container">
        <component :is="iconMap.plugins" class="custom-stat-card__icon" />
      </div>
      <div class="custom-stat-card__body">
        <span class="custom-stat-card__label">{{ activePluginsLabel }}</span>
        <strong class="custom-stat-card__value">{{ activePluginsCount }}</strong>
        <span class="custom-stat-card__desc">{{ activePluginsDetailText }}</span>
      </div>
    </RouterLink>

    <div class="custom-stat-card stat-card custom-stat-card--info stat-card--info">
      <div class="custom-stat-card__icon-container">
        <component :is="iconMap.uptime" class="custom-stat-card__icon" />
      </div>
      <div class="custom-stat-card__body">
        <span class="custom-stat-card__label">{{ uptimeLabel }}</span>
        <strong class="custom-stat-card__value monospace">{{ uptimeText }}</strong>
        <span class="custom-stat-card__desc">{{ runtimeMetaText }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
.dashboard-status-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: var(--app-layout-gap);
  margin-bottom: var(--app-layout-gap);
}

.custom-stat-card {
  position: relative;
  display: flex;
  align-items: flex-start;
  gap: var(--space-md);
  padding: var(--space-lg);
  border-radius: var(--radius-xl);
  border: 1px solid var(--border);
  background: var(--surface-strong);
  box-shadow: var(--shadow-xs);
  overflow: hidden;
  transition: border-color 150ms ease, background-color 150ms ease;
  cursor: default;
}

.custom-stat-card--link {
  color: inherit;
  cursor: pointer;
  text-decoration: none;

  &:focus-visible {
    border-color: var(--card-color, var(--border-accent));
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
}

.custom-stat-card:hover {
  border-color: var(--card-color, var(--border-accent));
}

.custom-stat-card__icon-container {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 42px;
  height: 42px;
  border-radius: var(--radius-lg);
  background: var(--surface-soft);
  border: 1px solid var(--border);
  flex-shrink: 0;
  transition: border-color 150ms ease, background-color 150ms ease;
}

.custom-stat-card__icon {
  font-size: 1.35rem;
  color: var(--card-color, var(--accent));
}

.custom-stat-card__body {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.custom-stat-card__label {
  font-size: 0.74rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--muted);
}

.custom-stat-card__value {
  font-size: 1.45rem;
  font-weight: 800;
  line-height: 1.2;
  color: var(--text);
  letter-spacing: -0.01em;

  &.monospace {
    font-family: var(--font-mono);
    font-size: 1.2rem;
    font-weight: 700;
  }
}

.custom-stat-card__desc {
  font-size: 0.8rem;
  color: var(--muted);
  line-height: 1.4;
  margin-top: 1px;
}

/* Card States & Tone Mapping */
.custom-stat-card--success {
  --card-color: var(--success);
  border-left: 3px solid var(--success);
  .custom-stat-card__icon-container {
    background: var(--surface-success);
    border-color: var(--border-success);
  }
}

.custom-stat-card--warning {
  --card-color: var(--warning);
  border-left: 3px solid var(--warning);
  .custom-stat-card__icon-container {
    background: var(--surface-warning);
    border-color: var(--border-warning);
  }
}

.custom-stat-card--danger {
  --card-color: var(--danger);
  border-left: 3px solid var(--danger);
  .custom-stat-card__icon-container {
    background: var(--surface-danger);
    border-color: var(--border-danger);
  }
}

.custom-stat-card--primary {
  --card-color: var(--accent);
  border-left: 3px solid var(--accent);
  .custom-stat-card__icon-container {
    background: var(--surface-accent);
    border-color: var(--border-accent);
  }
}

.custom-stat-card--info {
  --card-color: #17a2b8;
  border-left: 3px solid #17a2b8;
  .custom-stat-card__icon-container {
    background: color-mix(in srgb, #17a2b8 10%, var(--surface));
    border-color: color-mix(in srgb, #17a2b8 30%, var(--border));
  }
}

.custom-stat-card--muted {
  --card-color: var(--muted);
  border-left: 3px solid var(--muted);
  .custom-stat-card__icon-container {
    background: var(--surface-soft);
    border-color: var(--border);
  }
}
</style>
