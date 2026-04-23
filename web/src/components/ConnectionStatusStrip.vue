<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'

import { getConnectionChannelLabel, getConnectionStatusLabel } from '@/lib/display'
import { t } from '@/i18n'
import { useSocketStore } from '@/stores/sockets'
import type { ConnectionStatus } from '@/types/api'

const socketStore = useSocketStore()
const { snapshots } = storeToRefs(socketStore)

const managementChannels = ['events', 'tasks', 'logs'] as const

const channelStates = computed(() =>
  managementChannels.map((channel) => {
    const snapshot = snapshots.value[channel]
    const genericError = `${channel} 连接异常`

    return {
      channel,
      snapshot,
      secondary: snapshot.lastError && snapshot.lastError !== genericError ? snapshot.lastError : '',
    }
  }),
)

const needsReconnect = computed(() =>
  channelStates.value.some(({ snapshot }) => snapshot.status !== 'authenticated'),
)

function resolveBadgeStatus(status: ConnectionStatus) {
  switch (status) {
    case 'authenticated':
      return 'success'
    case 'connecting':
    case 'reconnecting':
      return 'processing'
    case 'connected':
      return 'warning'
    case 'auth_failed':
      return 'error'
    default:
      return 'default'
  }
}

function getPulseClass(status: ConnectionStatus) {
  if (status === 'authenticated') return 'status-pulse--success'
  if (status === 'connecting' || status === 'reconnecting') return 'status-pulse--processing'
  return ''
}
</script>

<template>
  <a-card :bordered="false" class="app-view-card connection-card" data-testid="dashboard-connection-card">
    <template #title>
      <div class="card-header">
        <div>
          <span>{{ t('dashboard.connectionStatus') }}</span>
          <p>{{ t('dashboard.connectionStatusHint') }}</p>
        </div>
      </div>
    </template>

    <template #extra>
      <a-button v-if="needsReconnect" size="small" @click="socketStore.reconnectAll()">
        {{ t('dashboard.reconnect') }}
      </a-button>
    </template>

    <div class="connection-card__grid">
      <section
        v-for="{ channel, snapshot, secondary } in channelStates"
        :key="channel"
        class="connection-card__item"
        :data-testid="`connection-card-${channel}`"
      >
        <div class="connection-card__row">
          <span class="connection-card__label">{{ getConnectionChannelLabel(channel) }}</span>
          <span :class="['connection-card__badge-wrap', getPulseClass(snapshot.status)]">
            <a-badge :status="resolveBadgeStatus(snapshot.status)" :text="getConnectionStatusLabel(snapshot.status)" />
          </span>
        </div>
        <small v-if="secondary" class="connection-card__meta">{{ secondary }}</small>
      </section>
    </div>
  </a-card>
</template>

<style scoped lang="scss">
.connection-card__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
}

.connection-card__item {
  padding: 12px 14px;
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
  background: var(--surface-soft);
  display: grid;
  gap: 6px;
}

.connection-card__row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
}

.connection-card__label {
  font-weight: 600;
}

.connection-card__meta {
  color: var(--muted);
  line-height: 1.4;
}

.connection-card__badge-wrap {
  display: inline-flex;
  border-radius: 999px;
}
</style>
