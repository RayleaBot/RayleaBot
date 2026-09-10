<script setup lang="ts">
import AppTag from '@/components/AppTag.vue'
import { getLogLevelLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import { escapeUnsafeDisplayText } from '@/lib/text-safety'
import { usePluginDisplayName } from '@/lib/use-plugin-display-name'
import type { LogSummary } from '@/types/api'

const props = defineProps<{ item: LogSummary; selected: boolean }>()
defineEmits<{ select: [item: LogSummary] }>()
const pluginName = usePluginDisplayName(() => props.item.plugin_id)
function getLevelColor(level: string) {
  if (level === 'error') return 'danger'
  if (level === 'warn') return 'warning'
  if (level === 'info') return 'info'
  return 'neutral'
}
</script>

<template>
  <button
    type="button"
    class="logs-row"
    :class="{ 'is-selected': selected }"
    :aria-label="`${getLogLevelLabel(item.level)} · ${item.source} · ${formatDateTime(item.timestamp)} · ${escapeUnsafeDisplayText(item.message)}`"
    @click="$emit('select', item)"
  >
    <div class="logs-row__meta">
      <div class="logs-row__time">{{ formatDateTime(item.timestamp) }}</div>
      <div class="logs-row__source">
        <span>{{ item.source }}</span>
        <span v-if="item.protocol" class="logs-row__protocol">{{ item.protocol }}</span>
      </div>
    </div>

    <div class="logs-row__main">
      <div class="logs-row__headline">
        <AppTag size="small" :tone="getLevelColor(item.level)">
          {{ getLogLevelLabel(item.level) }}
        </AppTag>
        <span v-if="item.plugin_id" class="logs-row__sub" :title="item.plugin_id">{{ pluginName }}</span>
        <span v-if="item.request_id" class="logs-row__sub">{{ item.request_id }}</span>
      </div>
      <p class="logs-row__message">{{ escapeUnsafeDisplayText(item.message) }}</p>
    </div>
  </button>
</template>

<style scoped>
.logs-row {
  width: 100%;
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  gap: 14px;
  border: none;
  border-bottom: 1px solid var(--border);
  background: transparent;
  padding: 14px 16px;
  text-align: left;
  cursor: pointer;
}

.logs-row:hover,
.logs-row.is-selected {
  background: var(--surface-accent);
}

.logs-row:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -2px;
}

.logs-row.is-selected {
  outline: 2px solid color-mix(in srgb, var(--accent) 34%, transparent);
  outline-offset: -2px;
  background: var(--surface-accent) !important;
}

.logs-row__meta,
.logs-row__main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.logs-row__time,
.logs-row__source,
.logs-row__sub {
  font-family: var(--font-mono);
}

.logs-row__time {
  color: var(--muted);
  font-size: 0.82rem;
}

.logs-row__source {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  color: var(--muted);
  font-size: 13px;
}

.logs-row__protocol {
  color: var(--accent);
}

.logs-row__headline {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.logs-row__sub {
  color: var(--muted);
  font-size: 13px;
}

.logs-row__message {
  margin: 0;
  color: var(--text);
  line-height: 1.6;
  font-size: 0.9rem;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
  unicode-bidi: plaintext;
}

@media (max-width: 760px) {
  .logs-row { grid-template-columns: 1fr; }
}
</style>
