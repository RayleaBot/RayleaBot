<script setup lang="ts">
import type { StatusType } from '@/lib/display'

defineProps<{
  title: string
  statusBadge: {
    type: StatusType
    icon: string
    label: string
  }
  lastRefreshedLabel: string | null
  autoRefresh: boolean
  countdown: number
  autoRefreshInterval: number
  loading: boolean
}>()

defineEmits<{
  refresh: []
  'toggle-auto-refresh': [boolean]
}>()
</script>

<template>
  <section class="hero-panel">
    <div>
      <h1>{{ title }}</h1>
      <div class="hero-meta">
        <div :class="['status-badge', `status-badge--${statusBadge.type}`]">
          <span class="status-badge__icon">{{ statusBadge.icon }}</span>
          <span>{{ statusBadge.label }}</span>
        </div>
        <div v-if="lastRefreshedLabel" class="hero-meta__time">
          {{ lastRefreshedLabel }}
          <template v-if="autoRefresh"> · {{ countdown }}s</template>
        </div>
        <div v-if="autoRefresh" class="auto-refresh-bar">
          <div class="auto-refresh-bar__fill" :style="{ width: `${(countdown / autoRefreshInterval) * 100}%` }" />
        </div>
        <div class="hero-auto-refresh">
          <span>自动刷新</span>
          <el-switch
            :model-value="autoRefresh"
            size="small"
            @change="$emit('toggle-auto-refresh', $event)"
          />
        </div>
      </div>
    </div>

    <div class="table-actions">
      <el-button :loading="loading" @click="$emit('refresh')">
        刷新
      </el-button>
    </div>
  </section>
</template>
