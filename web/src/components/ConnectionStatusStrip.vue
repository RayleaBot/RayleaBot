<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'

import { useSocketStore } from '@/stores/sockets'

const socketStore = useSocketStore()
const { snapshots } = storeToRefs(socketStore)

const labels: Record<string, string> = {
  events: 'events',
  tasks: 'tasks',
  logs: 'logs',
  pluginConsole: 'console',
}

const needsReconnect = computed(() =>
  Object.values(snapshots.value).some((snapshot) => snapshot.status !== 'authenticated'),
)
</script>

<template>
  <div class="connection-strip-wrap">
    <div class="connection-strip">
      <div v-for="(snapshot, key) in snapshots" :key="key" class="connection-pill" :class="`is-${snapshot.status}`">
        <div>
          <span>{{ labels[key] }}</span>
          <strong>{{ snapshot.status }}</strong>
        </div>
        <small v-if="snapshot.lastError">{{ snapshot.lastError }}</small>
      </div>
    </div>

    <el-button v-if="needsReconnect" plain size="small" @click="socketStore.reconnectAll()">
      重新连接
    </el-button>
  </div>
</template>
