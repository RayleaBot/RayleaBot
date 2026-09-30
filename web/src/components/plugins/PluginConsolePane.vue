<script setup lang="ts">
import { EraserIcon, RotateCwIcon } from '@lucide/vue'
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'

import AppButton from '@/components/AppButton.vue'
import AppSkeleton from '@/components/AppSkeleton.vue'
import AppTag from '@/components/AppTag.vue'
import AppTooltip from '@/components/AppTooltip.vue'
import VirtualDataViewport from '@/components/VirtualDataViewport.vue'
import { t } from '@/i18n'
import { getConnectionStatusLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import { escapeUnsafeDisplayText } from '@/lib/text-safety'
import { usePluginConsoleStore } from '@/stores/plugin-console'
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

const CONSOLE_ROW_ESTIMATED_HEIGHT = 84

// active: the console tab is shown; ready: the page transition allows rendering heavy content.
const props = defineProps<{ pluginId: string; active: boolean; ready: boolean }>()

const pluginConsoleStore = usePluginConsoleStore()
const socketStore = useSocketStore()

const frames = computed(() => pluginConsoleStore.getConsole(props.pluginId))
const snapshot = computed(() => socketStore.snapshots.pluginConsole)
const connectionTone = computed(() => getConsoleConnectionTone(snapshot.value.status))
const connectionDotColor = computed(() => connectionTone.value === 'neutral' ? 'var(--muted)' : `var(--${connectionTone.value})`)
const viewportRef = ref<{ scrollToBottom: () => void } | null>(null)
const followBottom = ref(true)
let bottomSyncToken = 0

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
        <span class="console-status-indicator">
          <span class="console-status-dot" :style="{ backgroundColor: connectionDotColor }"></span>
          <AppTag :tone="connectionTone" class="console-status-tag">{{ getConnectionStatusLabel(snapshot.status) }}</AppTag>
        </span>
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
        <span class="plugin-console-empty__prompt">&gt;_</span>
        <span>{{ t('plugins.empty.console') }}</span>
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
        <template #default="{ item: frame }">
          <article class="console-terminal-line">
            <div class="console-terminal-line__meta">
              <time :datetime="frame.timestamp">{{ formatDateTime(frame.timestamp) }}</time>
              <div class="console-terminal-line__badges">
                <AppTag :tone="getConsoleStreamTone(frame.stream)" class="stream-badge">{{ getConsoleStreamLabel(frame.stream) }}</AppTag>
                <AppTag v-if="frame.stream === 'outbound'" :tone="getConsoleLevelTone(getConsoleLevel(frame))" class="level-badge">
                  {{ getConsoleLevelLabel(getConsoleLevel(frame)) }}
                </AppTag>
                <span v-if="getConsoleRequestId(frame)" class="console-request-id">{{ getConsoleRequestId(frame) }}</span>
              </div>
            </div>
            <pre class="console-terminal-line__text">{{ escapeUnsafeDisplayText(frame.text) }}</pre>
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

.console-status-indicator {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.console-status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
  display: inline-block;
}

.console-status-tag {
  font-family: var(--font-mono);
  font-size: 12px;
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

/* Console terminal surface */
.plugin-console-panel {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface-soft);
  box-shadow: none;

  &.is-empty {
    background: var(--surface-soft);
  }
}

[data-theme='dark'] .plugin-console-panel {
  background: var(--code-surface);
  box-shadow: none;
  border-color: var(--border);
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
  align-items: center;
  gap: 12px;
  flex: 1 1 auto;
  min-height: 0;
  padding: 32px 20px;
  color: var(--muted);
  font-size: 0.88rem;
}

.plugin-console-empty__prompt {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 26px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--surface-strong);
  color: var(--accent);
  font-family: var(--font-mono);
  font-size: 13px;
  font-weight: bold;
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
  grid-template-columns: minmax(210px, 260px) minmax(0, 1fr);
  gap: 16px;
  padding: 10px 16px;
  border-bottom: 1px solid color-mix(in srgb, var(--border) 40%, transparent);
  color: var(--text);

  &:last-child {
    border-bottom: none;
  }

  &:hover {
    background: color-mix(in srgb, var(--accent) 5%, transparent);
  }
}

.console-terminal-line__meta {
  display: grid;
  align-content: start;
  gap: 6px;
  min-width: 0;

  time {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 1.4;
  }
}

.console-terminal-line__badges {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
  min-width: 0;
}

.stream-badge, .level-badge {
  font-size: 12px;
  padding-inline: 4px;
  border-radius: var(--radius-sm);
  margin-inline-end: 0 !important;
}

.console-request-id {
  max-width: 100%;
  overflow: hidden;
  color: var(--muted);
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.console-terminal-line__text {
  min-width: 0;
  margin: 0;
  color: var(--text);
  white-space: pre-wrap;
  word-break: break-all;
  font-family: var(--font-mono);
  font-size: 0.82rem;
  line-height: 1.58;
  unicode-bidi: plaintext;
}

[data-theme='dark'] .console-terminal-line__text {
  color: var(--code-text);
}
</style>
