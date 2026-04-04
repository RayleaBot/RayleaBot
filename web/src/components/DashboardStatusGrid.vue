<script setup lang="ts">
import type { StatusType } from '@/lib/display'

defineProps<{
  healthStatusType: StatusType
  readinessStatusType: StatusType
  systemStatusType: StatusType
  adapterStatusType: StatusType
  healthLabel: string
  healthValueText: string
  healthDetailText: string
  readinessLabel: string
  readinessValueText: string
  readinessDetailText: string
  systemLabel: string
  systemValueText: string
  systemDetailText: string
  adapterLabel: string
  adapterValueText: string
  adapterDetailText: string
  activePluginsLabel: string
  activePluginsCount: number
  uptimeLabel: string
  uptimeText: string
}>()
</script>

<template>
  <div class="stats-grid">
    <el-card :class="['stat-card', `stat-card--${healthStatusType}`]">
      <span class="stat-label">{{ healthLabel }}</span>
      <strong>{{ healthValueText }}</strong>
      <small>{{ healthDetailText }}</small>
    </el-card>
    <el-card :class="['stat-card', `stat-card--${readinessStatusType}`]">
      <span class="stat-label">{{ readinessLabel }}</span>
      <strong>{{ readinessValueText }}</strong>
      <small>{{ readinessDetailText }}</small>
    </el-card>
    <el-card :class="['stat-card', `stat-card--${systemStatusType}`]">
      <span class="stat-label">{{ systemLabel }}</span>
      <strong>{{ systemValueText }}</strong>
      <small>{{ systemDetailText }}</small>
    </el-card>
    <el-card :class="['stat-card', `stat-card--${adapterStatusType}`]">
      <span class="stat-label">{{ adapterLabel }}</span>
      <strong>{{ adapterValueText }}</strong>
      <small>{{ adapterDetailText }}</small>
    </el-card>
    <el-card class="stat-card">
      <span class="stat-label">{{ activePluginsLabel }}</span>
      <strong>{{ activePluginsCount }}</strong>
    </el-card>
    <el-card class="stat-card">
      <span class="stat-label">{{ uptimeLabel }}</span>
      <strong>{{ uptimeText }}</strong>
    </el-card>
  </div>
</template>
