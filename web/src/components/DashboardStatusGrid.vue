<script setup lang="ts">
import type { StatusType } from '@/lib/display'

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
    <a-card :bordered="false" :class="['stat-card', `stat-card--${healthStatusType}`]">
      <span class="stat-label">{{ healthLabel }}</span>
      <strong>{{ healthValueText }}</strong>
      <small>{{ healthDetailText }}</small>
    </a-card>
    <a-card :bordered="false" :class="['stat-card', `stat-card--${readinessStatusType}`]">
      <span class="stat-label">{{ readinessLabel }}</span>
      <strong>{{ readinessValueText }}</strong>
      <small>{{ readinessDetailText }}</small>
    </a-card>
    <a-card :bordered="false" class="stat-card">
      <span class="stat-label">{{ activePluginsLabel }}</span>
      <strong>{{ activePluginsCount }}</strong>
    </a-card>
    <a-card :bordered="false" class="stat-card">
      <span class="stat-label">{{ uptimeLabel }}</span>
      <strong>{{ uptimeText }}</strong>
    </a-card>
  </div>
</template>
