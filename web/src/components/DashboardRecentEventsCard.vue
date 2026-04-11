<script setup lang="ts">
import { formatRelativeTime } from '@/lib/format'
import { t } from '@/i18n'

defineProps<{
  recentEvents: Array<{
    timestamp: string
    summary: string
    payload: Record<string, unknown>
  }>
}>()

function getEventSeverity(payload: Record<string, unknown>): string | undefined {
  const severity = payload.severity
  return typeof severity === 'string' ? severity : undefined
}

function getEventSeverityClass(severity?: string): string {
  if (severity === 'error' || severity === 'danger') return 'event-item--danger'
  if (severity === 'warning') return 'event-item--warning'
  if (severity === 'success') return 'event-item--success'
  return ''
}
</script>

<template>
  <a-card :bordered="false">
    <template #title>
      <div class="card-header">
        <span>{{ t('dashboard.recentEvents') }}</span>
      </div>
    </template>

    <a-empty v-if="recentEvents.length === 0" :description="t('dashboard.recentEventsEmpty')" />

    <div v-else class="events-section">
      <div
        v-for="event in recentEvents"
        :key="`${event.timestamp}-${event.summary}`"
        :class="['event-item', getEventSeverityClass(getEventSeverity(event.payload))]"
      >
        <strong>{{ event.summary }}</strong>
        <span
          class="event-item__time"
          :data-absolute="event.timestamp"
        >{{ formatRelativeTime(event.timestamp) }}</span>
      </div>
    </div>
  </a-card>
</template>
