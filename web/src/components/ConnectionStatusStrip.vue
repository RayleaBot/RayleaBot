<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'

import { getConnectionChannelLabel, getConnectionStatusLabel } from '@/lib/display'
import { t } from '@/i18n'
import { useSocketStore } from '@/stores/sockets'
import type { ConnectionStatus } from '@/types/api'

const socketStore = useSocketStore()
const { snapshots } = storeToRefs(socketStore)

const managementChannels = ['events', 'logs'] as const

function formatLastErrorAt(value: string | undefined) {
  if (!value) return ''
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return ''
  return parsed.toLocaleTimeString()
}

const channelStates = computed(() =>
  managementChannels.map((channel) => {
    const snapshot = snapshots.value[channel]
    const genericError = `${channel} 连接异常`
    const secondary = snapshot.lastError && snapshot.lastError !== genericError ? snapshot.lastError : ''
    const reconnectSeconds = snapshot.nextBackoffMs !== undefined && snapshot.status === 'reconnecting'
      ? Math.max(1, Math.round(snapshot.nextBackoffMs / 1000))
      : null
    const reconnectHint = reconnectSeconds !== null
      ? t('dashboard.connectionReconnectIn', { seconds: reconnectSeconds })
      : ''
    const errorTime = formatLastErrorAt(snapshot.lastErrorAt)
    const errorHint = errorTime ? t('dashboard.connectionLastErrorAt', { time: errorTime }) : ''

    return {
      channel,
      snapshot,
      secondary,
      reconnectHint,
      errorHint,
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
        v-for="{ channel, snapshot, secondary, reconnectHint, errorHint } in channelStates"
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
        <small v-if="reconnectHint" class="connection-card__meta">{{ reconnectHint }}</small>
        <small v-if="errorHint" class="connection-card__meta">{{ errorHint }}</small>
      </section>
    </div>
  </a-card>
</template>

<style scoped lang="scss">
.connection-card {
  border-radius: var(--radius-xl);
  border: 1px solid var(--border);
  background: var(--surface-strong);
  box-shadow: var(--shadow-xs);
  transition: transform 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), box-shadow 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), border-color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), background-color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1), color 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);

  &:hover {
    box-shadow: var(--shadow-elevated);
    border-color: var(--border-accent);
  }
}

.connection-card :deep(.ant-card-body) {
  padding: var(--space-lg);
}

.card-header {
  span {
    font-size: 0.95rem;
    font-weight: 700;
    color: var(--text);
  }
  p {
    font-size: 0.78rem;
    color: var(--muted);
    margin: 2px 0 0;
    font-weight: 500;
  }
}

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
  transition: transform 0.24s cubic-bezier(0.25, 0.8, 0.25, 1), box-shadow 0.24s cubic-bezier(0.25, 0.8, 0.25, 1), border-color 0.24s cubic-bezier(0.25, 0.8, 0.25, 1), background-color 0.24s cubic-bezier(0.25, 0.8, 0.25, 1), color 0.24s cubic-bezier(0.25, 0.8, 0.25, 1);

  &:hover {
    transform: translateY(-2px);
    box-shadow: var(--shadow-sm);
    border-color: var(--border-accent);
  }
}

.connection-card__row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
}

.connection-card__label {
  font-weight: 700;
  font-size: 0.88rem;
  color: var(--text);
}

.connection-card__meta {
  color: var(--muted);
  line-height: 1.4;
  font-size: 0.78rem;
  font-weight: 500;
}

.connection-card__badge-wrap {
  display: inline-flex;
  border-radius: 999px;
  padding: 2px 6px;
  background: var(--surface-strong);
  border: 1px solid var(--border);
}

/* breathing status badge animations */
.status-pulse--success :deep(.ant-badge-status-dot) {
  animation: status-pulse-glow 2s infinite;
}

.status-pulse--processing :deep(.ant-badge-status-dot) {
  animation: status-pulse-glow-processing 2s infinite;
}

@keyframes status-pulse-glow {
  0% {
    box-shadow: 0 0 0 0 rgba(63, 190, 115, 0.65);
  }
  70% {
    box-shadow: 0 0 0 6px rgba(63, 190, 115, 0);
  }
  100% {
    box-shadow: 0 0 0 0 rgba(63, 190, 115, 0);
  }
}

@keyframes status-pulse-glow-processing {
  0% {
    box-shadow: 0 0 0 0 rgba(22, 104, 220, 0.65);
  }
  70% {
    box-shadow: 0 0 0 6px rgba(22, 104, 220, 0);
  }
  100% {
    box-shadow: 0 0 0 0 rgba(22, 104, 220, 0);
  }
}
</style>
