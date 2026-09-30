<script setup lang="ts">
import { EraserIcon, RotateCwIcon, TerminalIcon } from '@lucide/vue'
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'

import AppBadge from '@/components/AppBadge.vue'
import AppButton from '@/components/AppButton.vue'
import AppSkeleton from '@/components/AppSkeleton.vue'
import AppTag from '@/components/AppTag.vue'
import AppTooltip from '@/components/AppTooltip.vue'
import VirtualDataViewport from '@/components/VirtualDataViewport.vue'
import { t } from '@/i18n'
import { getConnectionStatusLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import { escapeUnsafeDisplayText } from '@/lib/text-safety'
import { correlatedRequestId } from '@/stores/log-state'
import { type ConsoleFrame, usePluginConsoleStore } from '@/stores/plugin-console'
import { useSocketStore } from '@/stores/sockets'
import {
  getConsoleConnectionTone,
  getConsoleFrameKey,
  getConsoleLevel,
  getConsoleLevelLabel,
  getConsoleLevelTone,
  getConsoleRequestId,
  getConsoleStreamLabel,
  getConsoleStreamTone,
} from './plugin-console-display'

const CONSOLE_ROW_ESTIMATED_HEIGHT = 44

// active: the console tab is shown; ready: the page transition allows rendering heavy content.
// pluginState: a stopped plugin produces no output, which the empty state says instead of "waiting".
const props = defineProps<{ pluginId: string; active: boolean; ready: boolean; pluginState?: string }>()

const pluginConsoleStore = usePluginConsoleStore()
const socketStore = useSocketStore()

const frames = computed(() => pluginConsoleStore.getConsole(props.pluginId))
const snapshot = computed(() => socketStore.snapshots.pluginConsole)
const connectionTone = computed(() => getConsoleConnectionTone(snapshot.value.status))
const emptyText = computed(() => (props.pluginState === 'disabled' ? t('plugins.empty.consoleDisabled') : t('plugins.empty.console')))
const viewportRef = ref<{ scrollToBottom: () => void } | null>(null)
const followBottom = ref(true)
let bottomSyncToken = 0

// As in the log rows, the reserved request ID of output written outside a request is not shown.
function rowRequestId(frame: ConsoleFrame) {
  return correlatedRequestId(getConsoleRequestId(frame))
}

function onViewportBottomChange(atBottom: boolean) {
  followBottom.value = atBottom
  if (!atBottom) {
    bottomSyncToken += 1
  }
}

function waitForAnimationFrame() {
  if (typeof window === 'undefined' || typeof window.requestAnimationFrame !== 'function') {
    return nextTick()
  }
  return new Promise<void>((resolve) => {
    window.requestAnimationFrame(() => resolve())
  })
}

// Rows are measured after they render, so the bottom is re-applied for a few frames.
async function syncViewportToBottom() {
  const syncToken = ++bottomSyncToken
  followBottom.value = true
  await nextTick()
  for (let attempt = 0; attempt < 4; attempt += 1) {
    if (syncToken !== bottomSyncToken || !props.active) return
    await waitForAnimationFrame()
    if (syncToken !== bottomSyncToken || !props.active) return
    viewportRef.value?.scrollToBottom()
    await nextTick()
  }
}

watch(() => props.active, (active) => {
  if (active) void syncViewportToBottom()
})

watch(() => props.ready, (ready) => {
  if (ready && props.active) void syncViewportToBottom()
}, { immediate: true })

watch(() => frames.value.length, async () => {
  if (props.active && followBottom.value) {
    await nextTick()
    viewportRef.value?.scrollToBottom()
  }
})

onBeforeUnmount(() => {
  bottomSyncToken += 1
})
</script>

<template>
  <div class="plugin-console-pane">
    <div class="plugin-console-header">
      <div class="plugin-console-title">
        <AppBadge :tone="connectionTone">{{ t('plugins.console.streamStatus', { status: getConnectionStatusLabel(snapshot.status) }) }}</AppBadge>
        <span class="plugin-console-count">{{ t('plugins.console.outputCount', { count: frames.length }) }}</span>
      </div>
      <div class="plugin-console-actions">
        <AppTooltip :title="t('plugins.actions.reconnectConsole')">
          <AppButton
            size="sm"
            class="plugin-console-icon-button"
            :aria-label="t('plugins.actions.reconnectConsole')"
            @click="socketStore.reconnectConsole()"
          >
            <template #icon>
              <RotateCwIcon />
            </template>
            {{ t('plugins.actions.reconnectConsole') }}
          </AppButton>
        </AppTooltip>
        <AppTooltip :title="t('plugins.actions.clearConsole')">
          <AppButton
            size="sm"
            class="plugin-console-icon-button"
            :disabled="frames.length === 0"
            :aria-label="t('plugins.actions.clearConsole')"
            @click="pluginConsoleStore.clearConsole(pluginId)"
          >
            <template #icon>
              <EraserIcon />
            </template>
          </AppButton>
        </AppTooltip>
      </div>
    </div>

    <div class="plugin-console-panel" :class="{ 'is-empty': frames.length === 0 }">
      <div v-if="snapshot.lastError" class="plugin-console-warning" role="status">
        <strong>{{ t('plugins.consoleUnavailable') }}</strong>
        <span>{{ snapshot.lastError }}</span>
      </div>

      <div v-if="frames.length === 0" class="plugin-console-empty">
        <TerminalIcon class="plugin-console-empty__icon" :size="20" aria-hidden="true" />
        <span>{{ emptyText }}</span>
      </div>

      <AppSkeleton
        v-else-if="!ready"
        class="console-terminal-skeleton"
        :rows="6"
      />

      <VirtualDataViewport
        v-else
        ref="viewportRef"
        class="console-terminal"
        :aria-label="t('plugins.console.ariaLabel')"
        :items="frames"
        :item-height="CONSOLE_ROW_ESTIMATED_HEIGHT"
        :dynamic-item-height="true"
        :overscan="6"
        :follow-bottom="followBottom"
        :empty-label="t('plugins.empty.console')"
        :get-item-key="getConsoleFrameKey"
        @at-bottom-change="onViewportBottomChange"
      >
        <!-- Rows follow the log rows: time, stream and level, then the text on one line when it fits; a real request ID ends the row. -->
        <template #default="{ item: frame }">
          <article class="console-terminal-line" :data-stream="frame.stream">
            <time class="console-terminal-line__time" :datetime="frame.timestamp">{{ formatDateTime(frame.timestamp) }}</time>
            <span class="console-terminal-line__badges">
              <AppTag size="small" :tone="getConsoleStreamTone(frame.stream)">{{ getConsoleStreamLabel(frame.stream) }}</AppTag>
              <AppTag v-if="frame.stream === 'outbound'" size="small" :tone="getConsoleLevelTone(getConsoleLevel(frame))">
                {{ getConsoleLevelLabel(getConsoleLevel(frame)) }}
              </AppTag>
            </span>
            <pre class="console-terminal-line__text">{{ escapeUnsafeDisplayText(frame.text) }}</pre>
            <span v-if="rowRequestId(frame)" class="console-request-id" :title="`${t('logs.filters.requestId')} ${rowRequestId(frame)}`">{{ rowRequestId(frame) }}</span>
          </article>
        </template>
      </VirtualDataViewport>
    </div>
  </div>
</template>

<style scoped lang="scss">

.plugin-console-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}

.plugin-console-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.plugin-console-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.plugin-console-icon-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 28px;
  gap: 6px;
  font-size: 13px;
}

// The console output is a white inset inside the gray tab box, without a second border.
.plugin-console-panel {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  border-radius: var(--radius-lg);
  background: var(--surface-raised);
}

.plugin-console-warning {
  display: grid;
  gap: 4px;
  margin: 12px;
  padding: 10px 0;
  border-bottom: 1px solid var(--border);

  strong {
    color: var(--text);
    font-size: 0.86rem;
  }
  span {
    color: var(--muted);
    font-size: 0.82rem;
    word-break: break-all;
  }
}

.plugin-console-empty {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 12px;
  flex: 1 1 auto;
  min-height: 0;
  padding: 32px 20px;
  color: var(--muted);
  font-size: 0.88rem;
}

.plugin-console-empty__icon {
  flex: none;
  color: var(--muted);
}

.console-terminal-skeleton {
  flex: 1 1 auto;
  min-height: 0;
  padding: 16px;
}

.console-terminal {
  flex: 1 1 auto;
  min-height: 0;
  border: none;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.console-terminal :deep(.data-viewport__scroller) {
  scrollbar-gutter: stable;
}

.console-terminal :deep(.data-viewport__empty) {
  display: none;
}

.console-terminal-line {
  display: grid;
  grid-template-columns: 156px 112px minmax(0, 1fr) auto;
  align-items: start;
  gap: 12px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--border);
  color: var(--text);

  &:last-child {
    border-bottom: none;
  }

  &:hover {
    background: var(--nav-hover);
  }
}

.console-terminal-line__time,
.console-terminal-line__badges,
.console-request-id {
  display: flex;
  align-items: center;
  min-height: 22px;
  min-width: 0;
}

.console-terminal-line__time,
.console-request-id {
  color: var(--muted);
  font-family: var(--font-mono);
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

.console-terminal-line__badges {
  gap: 4px;
}

.console-terminal-line__badges :deep(.app-tag) {
  margin-inline-end: 0;
}

.console-request-id {
  max-width: 18ch;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

// Messages the plugin sent read like log messages in the interface font; raw process output stays monospace.
.console-terminal-line__text {
  min-width: 0;
  margin: 0;
  color: var(--text);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  font-family: var(--font-sans);
  font-size: 14px;
  line-height: 22px;
  unicode-bidi: plaintext;
}

.console-terminal-line:not([data-stream=outbound]) .console-terminal-line__text {
  font-family: var(--font-mono);
  font-size: 13px;
}
@media (forced-colors: active) {
  .plugin-console-panel { border: 1px solid CanvasText; }
}
</style>
