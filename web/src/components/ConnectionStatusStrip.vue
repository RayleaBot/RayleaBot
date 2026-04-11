<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'

import { getConnectionChannelLabel, getConnectionStatusLabel } from '@/lib/display'
import { t } from '@/i18n'
import { useSocketStore } from '@/stores/sockets'

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
</script>

<template>
  <div class="connection-strip-wrap">
    <div class="connection-strip">
      <div
        v-for="{ channel, snapshot, secondary } in channelStates"
        :key="channel"
        class="connection-pill"
        :class="`is-${snapshot.status}`"
      >
        <div>
          <span>{{ getConnectionChannelLabel(channel) }}</span>
          <strong>{{ getConnectionStatusLabel(snapshot.status) }}</strong>
        </div>
        <small v-if="secondary">{{ secondary }}</small>
      </div>
    </div>

    <a-button v-if="needsReconnect" size="small" @click="socketStore.reconnectAll()">
      {{ t('shell.reconnectAll') }}
    </a-button>
  </div>
</template>
