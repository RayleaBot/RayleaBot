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
  activePluginsTo: RouteLocationRaw
  activePluginsAriaLabel: string
  uptimeLabel: string
  uptimeText: string
}>()
</script>

<template>
  <div class="dashboard-status-grid dashboard-overview-grid" data-testid="dashboard-overview-grid">
    <!-- Health Check Card -->
    <div :class="['custom-stat-card', 'stat-card', `custom-stat-card--${healthStatusType}`, `stat-card--${healthStatusType}`]">
      <div class="custom-stat-card__icon-container">
        <component :is="iconMap.health" class="custom-stat-card__icon" />
      </div>
      <div class="custom-stat-card__body">
        <span class="custom-stat-card__label">{{ healthLabel }}</span>
        <strong class="custom-stat-card__value">{{ healthValueText }}</strong>
        <span class="custom-stat-card__desc">{{ healthDetailText }}</span>
      </div>
      <div class="custom-stat-card__shine" />
    </div>

    <!-- Readiness Status Card -->
    <div :class="['custom-stat-card', 'stat-card', `custom-stat-card--${readinessStatusType}`, `stat-card--${readinessStatusType}`]">
      <div class="custom-stat-card__icon-container">
        <component :is="iconMap.readiness" class="custom-stat-card__icon" />
      </div>
      <div class="custom-stat-card__body">
        <span class="custom-stat-card__label">{{ readinessLabel }}</span>
        <strong class="custom-stat-card__value">{{ readinessValueText }}</strong>
        <span class="custom-stat-card__desc">{{ readinessDetailText }}</span>
      </div>
      <div class="custom-stat-card__shine" />
    </div>

    <!-- Active Plugins Card -->
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
      </div>
      <div class="custom-stat-card__shine" />
    </RouterLink>

    <!-- Uptime Card -->
    <div class="custom-stat-card stat-card custom-stat-card--info stat-card--info">
      <div class="custom-stat-card__icon-container">
        <component :is="iconMap.uptime" class="custom-stat-card__icon" />
      </div>
      <div class="custom-stat-card__body">
        <span class="custom-stat-card__label">{{ uptimeLabel }}</span>
        <strong class="custom-stat-card__value monospace">{{ uptimeText }}</strong>
      </div>
      <div class="custom-stat-card__shine" />
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
  transition: all 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);
  cursor: default;

  &::after {
    content: '';
    position: absolute;
    inset: 0;
    border-radius: inherit;
    padding: 1px;
    background: linear-gradient(135deg, rgba(255, 255, 255, 0.08), rgba(255, 255, 255, 0));
    -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
    -webkit-mask-composite: xor;
    mask-composite: exclude;
    pointer-events: none;
  }
}

.custom-stat-card--link {
  color: inherit;
  cursor: pointer;
  text-decoration: none;

  &:focus-visible {
    border-color: var(--card-color, var(--border-accent));
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--card-color, var(--accent)) 22%, transparent);
    outline: none;
  }
}

.custom-stat-card:hover {
  transform: translateY(-4px);
  box-shadow: var(--shadow-elevated), 0 8px 24px -8px color-mix(in srgb, var(--card-color, var(--accent)) 25%, transparent);
  border-color: var(--card-color, var(--border-accent));

  .custom-stat-card__icon-container {
    transform: scale(1.06);
    box-shadow: 0 0 12px color-mix(in srgb, var(--card-color, var(--accent)) 25%, transparent);
  }

  .custom-stat-card__shine {
    transform: translateX(100%) rotate(45deg);
  }
}

.custom-stat-card__shine {
  position: absolute;
  top: 0;
  left: -50%;
  width: 20%;
  height: 100%;
  background: linear-gradient(
    to right,
    rgba(255, 255, 255, 0) 0%,
    rgba(255, 255, 255, 0.12) 50%,
    rgba(255, 255, 255, 0) 100%
  );
  transform: skewX(-25deg);
  transition: transform 0.6s ease;
  pointer-events: none;
}

[data-theme='dark'] .custom-stat-card__shine {
  background: linear-gradient(
    to right,
    rgba(255, 255, 255, 0) 0%,
    rgba(255, 255, 255, 0.06) 50%,
    rgba(255, 255, 255, 0) 100%
  );
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
  transition: all 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);
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
