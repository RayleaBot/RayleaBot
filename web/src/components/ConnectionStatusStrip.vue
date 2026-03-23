<script setup lang="ts">
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
</script>

<template>
  <div class="connection-strip">
    <div v-for="(snapshot, key) in snapshots" :key="key" class="connection-pill" :class="`is-${snapshot.status}`">
      <span>{{ labels[key] }}</span>
      <strong>{{ snapshot.status }}</strong>
    </div>
  </div>
</template>
