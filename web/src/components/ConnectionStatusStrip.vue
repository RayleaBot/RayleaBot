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
          <span class="connection-card__badge-wrap">
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
  border: 1px solid var(--border);
  background: var(--surface-strong);
  box-shadow: none;
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
    font-size: 13px;
    color: var(--muted);
    margin: 2px 0 0;
    font-weight: 500;
  }
}

.connection-card__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0;
  border-block: 1px solid var(--border);
}

.connection-card__item {
  padding: 12px 0;
  display: grid;
  gap: 6px;
}

.connection-card__item + .connection-card__item {
  margin-inline-start: 16px;
  padding-inline-start: 16px;
  border-inline-start: 1px solid var(--border);
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
  font-size: 13px;
  font-weight: 500;
}

.connection-card__badge-wrap {
  display: inline-flex;
}

@media (max-width: 639px) {
  .connection-card__grid {
    grid-template-columns: 1fr;
  }

  .connection-card__item + .connection-card__item {
    margin-inline-start: 0;
    padding-inline-start: 0;
    border-inline-start: 0;
    border-top: 1px solid var(--border);
  }
}

</style>
