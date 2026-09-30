<script setup lang="ts">
import { computed } from 'vue'
import AppTag from '@/components/AppTag.vue'
import { getLogLevelLabel, getLogProtocolLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import { escapeUnsafeDisplayText } from '@/lib/text-safety'
import { usePluginDisplayName } from '@/lib/use-plugin-display-name'
import { correlatedRequestId } from '@/stores/log-state'
import { t } from '@/i18n'
import type { LogSummary } from '@/types/api'

const props = defineProps<{ item: LogSummary; selected: boolean }>()
defineEmits<{ select: [item: LogSummary] }>()
const pluginName = usePluginDisplayName(() => props.item.plugin_id)
// The reserved request ID of logs written outside a request correlates nothing, so it is not shown.
const requestId = computed(() => correlatedRequestId(props.item.request_id))
function getLevelColor(level: string) {
  if (level === 'error') return 'danger'
  if (level === 'warn') return 'warning'
  if (level === 'info') return 'info'
  return 'neutral'
}
</script>

<template>
  <!-- One line per entry: time, level, source and protocol, then the message led by its plugin; long messages wrap. -->
  <button
    type="button"
    class="logs-row"
    :class="{ 'is-selected': selected }"
    :aria-label="`${getLogLevelLabel(item.level)} · ${item.source} · ${formatDateTime(item.timestamp)} · ${escapeUnsafeDisplayText(item.message)}`"
    @click="$emit('select', item)"
  >
    <span class="logs-row__time">{{ formatDateTime(item.timestamp) }}</span>
    <span class="logs-row__level">
      <AppTag size="small" :tone="getLevelColor(item.level)">{{ getLogLevelLabel(item.level) }}</AppTag>
    </span>
    <span class="logs-row__source">
      <span class="logs-row__source-name">{{ item.source }}</span>
      <AppTag v-if="item.protocol" size="small">{{ getLogProtocolLabel(item.protocol) }}</AppTag>
    </span>
    <!-- No whitespace between the parts: the message keeps pre-wrap, so any would show as a leading space. -->
    <p class="logs-row__message"><span v-if="item.plugin_id" class="logs-row__plugin" :title="item.plugin_id">{{ pluginName }}</span>{{ escapeUnsafeDisplayText(item.message) }}</p>
    <span v-if="requestId" class="logs-row__request" :title="`${t('logs.filters.requestId')} ${requestId}`">{{ requestId }}</span>
  </button>
</template>

<style scoped lang="scss">
.logs-row {
  width: 100%;
  display: grid;
  grid-template-columns: 156px 64px minmax(120px, 200px) minmax(0, 1fr) auto;
  align-items: start;
  gap: 12px;
  border: none;
  border-bottom: 1px solid var(--border);
  background: transparent;
  padding: 10px 16px;
  text-align: left;
  cursor: pointer;
}

// Hover stays neutral; only the selected row takes the blue selected face and an inner ring,
// so the row under the pointer is never mistaken for the one shown in the detail window.
.logs-row:hover {
  background: var(--nav-hover);
}

.logs-row:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: var(--focus-outline-offset);
}

.logs-row.is-selected {
  background: var(--brand-soft);
  box-shadow: inset 0 0 0 2px var(--brand-foreground);
}

@media (forced-colors: active) {
  .logs-row.is-selected { outline: 2px solid Highlight; outline-offset: -2px; }
}

.logs-row__time,
.logs-row__level,
.logs-row__source,
.logs-row__request {
  display: flex;
  align-items: center;
  min-height: 22px;
  min-width: 0;
}

.logs-row__time,
.logs-row__source-name,
.logs-row__request {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}

.logs-row__source {
  gap: 6px;
}

.logs-row__source-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.logs-row__request {
  max-width: 18ch;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.logs-row__plugin {
  margin-inline-end: 8px;
  color: var(--text);
  font-weight: 600;
}

.logs-row__message {
  margin: 0;
  min-width: 0;
  color: var(--text);
  line-height: 22px;
  font-size: 14px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-word;
  unicode-bidi: plaintext;
}
</style>
