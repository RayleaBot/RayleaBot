<script setup lang="ts">
import {
  AppstoreOutlined,
  FieldTimeOutlined,
  HeartOutlined,
  SafetyOutlined,
} from '@ant-design/icons-vue'

import AppStatCard from '@/components/AppStatCard.vue'
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
  uptimeLabel: string
  uptimeText: string
}>()
</script>

<template>
  <div class="dashboard-status-grid dashboard-overview-grid" data-testid="dashboard-overview-grid">
    <AppStatCard
      :icon="iconMap.health"
      :label="healthLabel"
      :tone="healthStatusType === 'muted' ? 'default' : healthStatusType"
      :value="healthValueText"
      :description="healthDetailText"
    />
    <AppStatCard
      :icon="iconMap.readiness"
      :label="readinessLabel"
      :tone="readinessStatusType === 'muted' ? 'default' : readinessStatusType"
      :value="readinessValueText"
      :description="readinessDetailText"
    />
    <AppStatCard
      :icon="iconMap.plugins"
      :label="activePluginsLabel"
      :value="activePluginsCount"
    />
    <AppStatCard
      :icon="iconMap.uptime"
      :label="uptimeLabel"
      :value="uptimeText"
    />
  </div>
</template>

<style scoped lang="scss">
.dashboard-status-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: var(--app-layout-gap);
}
</style>
