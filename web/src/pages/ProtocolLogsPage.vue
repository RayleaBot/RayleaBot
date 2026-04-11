<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'

import RetryPanel from '@/components/RetryPanel.vue'
import { getLogLevelLabel, getLogProtocolLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import { t } from '@/i18n'
import { useProtocolLogsStore } from '@/stores/protocol-logs'

const protocolDetailFieldKeys = [
  'direction',
  'event_kind',
  'event_type',
  'post_type',
  'message_type',
  'event_timestamp',
  'time',
  'conversation_type',
  'conversation_id',
  'group_id',
  'sender_id',
  'user_id',
  'sender_nickname',
  'sender_card',
  'sender_role',
  'sender_title',
  'message_id',
  'real_id',
  'message_seq',
  'raw_message',
  'message_format',
  'font',
  'plain_text',
  'target_type',
  'target_id',
  'action_kind',
  'delivery_kind',
  'command_name',
  'frame_type',
  'error_code',
  'reason',
  'echo_value_type',
  'payload_preview',
  'segments',
] as const

type ProtocolDetailFieldKey = (typeof protocolDetailFieldKeys)[number]

const router = useRouter()
const protocolLogsStore = useProtocolLogsStore()

const {
  autoFollow,
  currentDetail,
  detailError,
  detailLoading,
  error: logsError,
  filters,
  items,
  loading: logsLoading,
  selectedItem,
  selectedLogId,
} = storeToRefs(protocolLogsStore)

const terminalScroller = ref<HTMLElement | null>(null)
const selectedSummary = computed(() => currentDetail.value ?? selectedItem.value ?? null)
const detailEntries = computed(() => {
  const details = toDetailRecord(currentDetail.value?.details)
  return protocolDetailFieldKeys.flatMap((key) => (
    key in details
      ? [{
        key,
        label: t(`protocols.detailFields.${key}`),
        value: formatDetailValue(key, details[key]),
      }]
      : []
  ))
})
const detailJson = computed(() => safeJsonStringify(toDetailRecord(currentDetail.value?.details)))
const terminalStatusLabel = computed(() => (
  autoFollow.value ? t('protocols.logsFollowing') : t('protocols.logsPaused')
))

watch(
  () => [items.value.length, autoFollow.value, selectedLogId.value] as const,
  async ([, followEnabled, logId]) => {
    const latest = items.value.at(-1)
    if (!followEnabled || !latest || latest.log_id !== logId) {
      return
    }
    await scrollTerminalToBottom('smooth')
  },
)

async function loadPage() {
  try {
    await protocolLogsStore.fetchList()
    await scrollTerminalToBottom('auto')
  } catch {
    // store error state drives the page
  }
}

onMounted(() => {
  protocolLogsStore.activate()
  void loadPage()
})

onUnmounted(() => {
  protocolLogsStore.deactivate()
})

async function refreshLogs() {
  try {
    await protocolLogsStore.fetchList()
    await scrollTerminalToBottom('auto')
  } catch {
    // store error state drives the page
  }
}

function clearBuffer() {
  protocolLogsStore.clearBuffer()
}

async function resumeAutoFollow() {
  await protocolLogsStore.resumeAutoFollow()
  await scrollTerminalToBottom('auto')
}

function pauseAutoFollow() {
  protocolLogsStore.pauseAutoFollow()
}

async function handleLogSelection(logId: string) {
  try {
    await protocolLogsStore.selectLog(logId)
  } catch {
    // detailError exposes the failure on the page
  }
}

async function scrollTerminalToBottom(behavior: ScrollBehavior = 'smooth') {
  await nextTick()
  if (!terminalScroller.value) {
    return
  }

  terminalScroller.value.scrollTo({
    top: terminalScroller.value.scrollHeight,
    behavior,
  })
}

function formatDetailValue(key: ProtocolDetailFieldKey, value: unknown) {
  if (value === null || value === undefined || value === '') {
    return t('display.empty')
  }

  if (key === 'event_timestamp' || key === 'time') {
    return formatProtocolEventTime(value)
  }

  if (key === 'segments' && Array.isArray(value)) {
    return t('protocols.segmentCount', { count: value.length })
  }

  if (typeof value === 'object') {
    const raw = safeJsonStringify(value)
    return raw.length > 140 ? `${raw.slice(0, 140)}...` : raw
  }

  return String(value)
}

function formatProtocolEventTime(value: unknown) {
  if (typeof value === 'number' && Number.isFinite(value) && value > 0) {
    return formatDateTime(normalizeUnixTimestamp(value))
  }

  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (!trimmed) {
      return t('display.empty')
    }

    const numeric = Number(trimmed)
    if (Number.isFinite(numeric) && numeric > 0) {
      return formatDateTime(normalizeUnixTimestamp(numeric))
    }

    return formatDateTime(trimmed)
  }

  return String(value)
}

function normalizeUnixTimestamp(value: number) {
  return value >= 1_000_000_000_000 ? value : value * 1000
}

function toDetailRecord(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return {}
  }

  return value as Record<string, unknown>
}

function safeJsonStringify(value: unknown) {
  try {
    return JSON.stringify(value ?? {}, null, 2)
  } catch {
    return '{}'
  }
}

function getLogRowClass(logId: string) {
  return {
    'is-selected': selectedLogId.value === logId,
  }
}

function getLevelClass(level: string) {
  return `is-${level}`
}

function getLevelColor(level: string) {
  switch (level) {
    case 'error': return 'danger';
    case 'warn': return 'warning';
    case 'info': return 'success';
    default: return 'debug';
  }
}
</script>

<template>
  <div class="page-grid minimal-protocol-theme">
    <section class="hero-panel">
      <div class="hero-text">
        <h1 class="main-title">{{ t('protocols.logsPageTitle') }}</h1>
        <p class="subtitle">{{ t('protocols.logsSubtitle') }}</p>
      </div>

      <div class="hero-actions">
        <button class="minimal-btn outline" @click="router.push('/protocols')">
          {{ t('protocols.openSettings') }}
        </button>
        <button class="minimal-btn primary" :disabled="logsLoading" @click="refreshLogs">
          <span v-if="logsLoading">{{ t('protocols.logsRefresh') }}...</span>
          <span v-else>{{ t('protocols.logsRefresh') }}</span>
        </button>
      </div>
    </section>

    <div class="protocol-logs-workspace">
      <aside class="logs-sidebar">
        <div class="minimal-card sidebar-card">
          <div class="card-header">
            <strong>{{ t('protocols.filters.apply') }}</strong>
          </div>
          <el-form label-position="top" class="sidebar-filter-form" @submit.prevent>
            <el-form-item :label="t('protocols.filters.level')">
              <el-select v-model="filters.level" clearable :placeholder="t('protocols.filters.all')" class="refined-input" popper-class="minimal-popper">
                <el-option :label="t('display.logLevels.debug')" value="debug" />
                <el-option :label="t('display.logLevels.info')" value="info" />
                <el-option :label="t('display.logLevels.warn')" value="warn" />
                <el-option :label="t('display.logLevels.error')" value="error" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('protocols.filters.source')">
              <el-input v-model="filters.source" :placeholder="t('protocols.filters.sourcePlaceholder')" class="refined-input" />
            </el-form-item>
            <el-form-item :label="t('protocols.filters.requestId')">
              <el-input v-model="filters.requestId" :placeholder="t('protocols.filters.requestPlaceholder')" class="refined-input" />
            </el-form-item>
            
            <div class="sidebar-actions">
              <button class="minimal-btn outline" @click="clearBuffer">{{ t('protocols.logsClear') }}</button>
              <button class="minimal-btn primary" @click="refreshLogs">{{ t('protocols.filters.apply') }}</button>
            </div>
          </el-form>
        </div>

        <div class="minimal-card sidebar-card control-card">
          <div class="card-header">
            <strong>{{ t('dashboard.refresh') }}</strong>
          </div>
          <div class="sidebar-controls">
            <div class="follow-status">
              <span class="minimal-badge" :class="autoFollow ? 'success' : 'warning'">
                {{ terminalStatusLabel }}
              </span>
              <span class="buffer-info">{{ t('protocols.bufferCount', { count: items.length }) }}</span>
            </div>
            <button class="minimal-btn outline" v-if="autoFollow" @click="pauseAutoFollow">
              {{ t('protocols.logsPause') }}
            </button>
            <button class="minimal-btn outline" v-else @click="resumeAutoFollow">
              {{ t('protocols.logsResume') }}
            </button>
          </div>
        </div>
      </aside>

      <main class="logs-main-content">
        <RetryPanel
          v-if="logsError && items.length === 0"
          :title="t('errors.common.loadFailed')"
          :description="logsError"
          :loading="logsLoading"
          @retry="refreshLogs"
        />

        <el-alert v-else-if="logsError" :title="t('errors.common.loadFailed')" type="error" :description="logsError" show-icon class="error-alert" />

        <div v-else class="logs-display-grid">
          <div class="minimal-card terminal-container">
            <div class="card-header">
              <strong>{{ t('protocols.logsStreamTitle') }}</strong>
              <span class="terminal-hint">{{ t('protocols.logsStreamHint') }}</span>
            </div>

            <div v-if="items.length === 0" class="term-empty-state">
              <div class="empty-icon">!</div>
              <p>{{ t('protocols.logsEmpty') }}</p>
            </div>

            <div v-else ref="terminalScroller" class="terminal-view-scroller">
              <div class="terminal-content">
                <button
                  v-for="log in items"
                  :key="log.log_id"
                  type="button"
                  class="terminal-line"
                  :class="[getLogRowClass(log.log_id), getLevelClass(log.level)]"
                  @click="handleLogSelection(log.log_id)"
                >
                  <div class="line-level-indicator" :class="getLevelColor(log.level)"></div>
                  <div class="line-content-wrap">
                    <div class="line-meta">
                      <span class="line-time">[{{ formatDateTime(log.timestamp) }}]</span>
                      <span class="line-level">[{{ getLogLevelLabel(log.level) }}]</span>
                      <span class="line-protocol">[{{ getLogProtocolLabel(log.protocol) }}]</span>
                      <span class="line-source">{{ log.source }}</span>
                      <span v-if="log.plugin_id" class="line-plugin">@{{ log.plugin_id }}</span>
                    </div>
                    <div class="line-body">
                      <span class="line-prompt">></span>
                      <span class="line-text">{{ log.message }}</span>
                    </div>
                    <div class="line-request" v-if="log.request_id">
                      ID: {{ log.request_id }}
                    </div>
                  </div>
                </button>
              </div>
            </div>
          </div>

          <div class="minimal-card detail-container">
            <div class="card-header">
              <strong>{{ t('protocols.logsDetailTitle') }}</strong>
            </div>

            <div v-if="!selectedSummary && !detailLoading" class="detail-empty-state">
              <div class="empty-icon">?</div>
              <p>{{ t('protocols.logsDetailEmpty') }}</p>
            </div>

            <el-skeleton v-else :loading="detailLoading && !currentDetail" animated>
              <div v-if="selectedSummary" class="detail-view-content">
                <el-alert
                  v-if="detailError"
                  :title="t('errors.common.loadFailed')"
                  type="error"
                  :description="detailError"
                  show-icon
                  class="detail-error"
                />

                <header class="detail-hero">
                  <div class="detail-hero-top">
                    <span class="minimal-badge" :class="getLevelColor(selectedSummary.level)">{{ getLogLevelLabel(selectedSummary.level) }}</span>
                    <span class="minimal-badge info">{{ getLogProtocolLabel(selectedSummary.protocol) }}</span>
                  </div>
                  <h3 class="detail-hero-message">{{ selectedSummary.message }}</h3>
                  <div class="detail-hero-meta">
                    <div class="meta-row">
                      <span class="mono-label">{{ t('protocols.fields.timestamp') }}:</span>
                      <span class="mono-value">{{ formatDateTime(selectedSummary.timestamp) }}</span>
                    </div>
                    <div class="meta-row">
                      <span class="mono-label">{{ t('protocols.fields.source') }}:</span>
                      <span class="mono-value">{{ selectedSummary.source }} <template v-if="selectedSummary.plugin_id">(@{{ selectedSummary.plugin_id }})</template></span>
                    </div>
                    <div class="meta-row" v-if="selectedSummary.request_id">
                      <span class="mono-label">{{ t('protocols.fields.requestId') }}:</span>
                      <span class="mono-value">{{ selectedSummary.request_id }}</span>
                    </div>
                    <div class="meta-row">
                      <span class="mono-label">LOG_ID:</span>
                      <span class="mono-value">{{ selectedSummary.log_id }}</span>
                    </div>
                  </div>
                </header>

                <div v-if="detailEntries.length > 0" class="detail-fields-section">
                  <div class="detail-fields-grid">
                    <div v-for="entry in detailEntries" :key="entry.key" class="detail-field-box">
                      <label class="field-label">{{ entry.label }}</label>
                      <div class="field-value">{{ entry.value }}</div>
                    </div>
                  </div>
                </div>

                <div class="detail-json-section">
                  <div class="json-header">
                    <strong>{{ t('protocols.logsDetailJson') }}</strong>
                  </div>
                  <pre class="json-content">{{ detailJson }}</pre>
                </div>
              </div>
            </el-skeleton>
          </div>
        </div>
      </main>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.protocol-logs-workspace {
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr);
  gap: var(--space-xl);
  align-items: stretch;
  height: calc(100vh - 220px);
  min-height: 600px;
}

.logs-sidebar {
  display: flex;
  flex-direction: column;
  gap: var(--space-xl);
  overflow-y: auto;
}

.sidebar-card {
  margin-bottom: 0;
}

.sidebar-filter-form {
  padding: var(--space-lg);
  display: flex;
  flex-direction: column;
  gap: var(--space-md);

  :deep(.el-form-item) {
    margin-bottom: 0;
  }
}

.sidebar-actions {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
  margin-top: var(--space-sm);
  
  .minimal-btn {
    width: 100%;
  }
}

.sidebar-controls {
  padding: var(--space-lg);
  display: flex;
  flex-direction: column;
  gap: var(--space-md);

  .minimal-btn {
    width: 100%;
  }
}

.follow-status {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.buffer-info {
  font-family: var(--font-mono);
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--theme-text-muted);
}

.logs-main-content {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.logs-display-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr);
  gap: var(--space-xl);
  flex: 1;
  min-height: 0;
}

.terminal-container, .detail-container {
  display: flex;
  flex-direction: column;
  margin-bottom: 0;
  height: 100%;
}

.terminal-hint {
  font-size: 0.8rem;
  color: var(--theme-text-muted);
  font-weight: normal;
}

/* Terminal View */
.terminal-view-scroller {
  flex: 1;
  overflow-y: auto;
  background: oklch(18% 0.01 235);
  padding: var(--space-sm) 0;
  border-radius: 0 0 12px 12px;
}

.terminal-content {
  display: flex;
  flex-direction: column;
}

.terminal-line {
  width: 100%;
  background: transparent;
  border: none;
  padding: var(--space-sm) var(--space-md);
  color: oklch(85% 0.01 235);
  font-family: var(--font-mono);
  text-align: left;
  cursor: pointer;
  transition: background-color 0.15s ease;
  display: flex;
  gap: var(--space-sm);

  &:hover {
    background: oklch(22% 0.01 235);
    color: #fff;
  }

  &.is-selected {
    background: oklch(25% 0.04 235);
    color: #fff;
  }

  &.is-debug { color: oklch(65% 0.01 235); }
}

.line-level-indicator {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  margin-top: 6px;
  flex-shrink: 0;
  background: oklch(60% 0.01 235); /* debug */

  &.success { background: var(--theme-success); }
  &.warning { background: var(--theme-warning); }
  &.danger { background: var(--theme-danger); }
}

.line-content-wrap {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.line-meta {
  display: flex;
  gap: var(--space-md);
  font-size: 0.75rem;
  opacity: 0.7;
  margin-bottom: var(--space-xs);
  flex-wrap: wrap;
}

.line-body {
  display: flex;
  gap: var(--space-sm);
  font-size: 0.85rem;
  line-height: 1.4;
}

.line-prompt {
  color: var(--theme-accent);
  font-weight: bold;
}

.line-text {
  word-break: break-all;
  white-space: pre-wrap;
}

.line-request {
  font-size: 0.7rem;
  opacity: 0.5;
  margin-top: var(--space-xs);
}

/* Detail View */
.detail-view-content {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}

.detail-hero {
  padding: var(--space-xl);
  background: var(--theme-bg);
  border-bottom: 1px solid var(--theme-border);
}

.detail-hero-top {
  display: flex;
  gap: var(--space-sm);
  margin-bottom: var(--space-md);
}

.detail-hero-message {
  font-family: var(--font-sans);
  font-size: 1.3rem;
  font-weight: 700;
  margin: 0 0 var(--space-md);
  line-height: 1.3;
  color: var(--theme-text);
}

.detail-hero-meta {
  display: grid;
  gap: var(--space-sm);
}

.meta-row {
  display: flex;
  gap: var(--space-md);
  font-size: 0.85rem;
  
  .mono-label { width: 120px; flex-shrink: 0; }
}

.detail-fields-section {
  padding: var(--space-xl);
  border-bottom: 1px solid var(--theme-border);
}

.detail-fields-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: var(--space-md);
}

.detail-field-box {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);

  .field-label {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--theme-text-muted);
    text-transform: uppercase;
    letter-spacing: 0.02em;
    font-family: var(--font-sans);
  }

  .field-value {
    font-family: var(--font-mono);
    font-size: 0.9rem;
    font-weight: 500;
    color: var(--theme-text);
    word-break: break-all;
  }
}

.detail-json-section {
  padding: var(--space-xl);
  background: var(--theme-surface);
}

.json-header {
  margin-bottom: var(--space-md);
  strong {
    font-family: var(--font-sans);
    font-weight: 600;
    color: var(--theme-text);
  }
}

.json-content {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 0.85rem;
  white-space: pre-wrap;
  word-break: break-all;
  color: var(--theme-text-muted);
  background: var(--theme-bg);
  padding: var(--space-md);
  border-radius: 8px;
  border: 1px solid var(--theme-border);
}

/* Empty States */
.term-empty-state, .detail-empty-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px;
  color: var(--theme-text-muted);
  font-family: var(--font-sans);
  text-align: center;

  .empty-icon {
    font-size: 2.5rem;
    font-weight: 800;
    margin-bottom: var(--space-md);
    opacity: 0.2;
  }
}

.detail-empty-state {
  background: var(--theme-bg);
}

@media (max-width: 1200px) {
  .protocol-logs-workspace {
    grid-template-columns: 1fr;
    height: auto;
  }
  
  .logs-sidebar {
    display: grid;
    grid-template-columns: 1fr 1fr;
  }
  
  .logs-display-grid {
    grid-template-columns: 1fr;
  }
  
  .terminal-container, .detail-container {
    height: 600px;
  }
}

@media (max-width: 768px) {
  .logs-sidebar {
    grid-template-columns: 1fr;
  }
}
</style>
