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
  'frame_type',
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

function getLevelPillClass(level: string) {
  return `is-${level}`
}
</script>

<template>
  <div class="page-grid industrial-theme">
    <section class="hero-panel">
      <div class="hero-text">
        <h1 class="glitch-title">{{ t('protocols.logsPageTitle') }}</h1>
        <p class="subtitle">>> {{ t('protocols.logsSubtitle') }}</p>
      </div>

      <div class="hero-actions">
        <el-button class="industrial-btn outline" @click="router.push('/protocols')">
          [ {{ t('protocols.openSettings') }} ]
        </el-button>
        <el-button class="industrial-btn primary" :loading="logsLoading" @click="refreshLogs">
          [ {{ t('protocols.logsRefresh') }} ]
        </el-button>
      </div>
    </section>

    <section class="protocol-logs-section">
      <div class="section-heading">
        <div>
          <h2>{{ t('protocols.logsTitle') }}</h2>
          <p class="subtitle">>> {{ t('protocols.logsStreamHint') }}</p>
        </div>
        <div class="terminal-header-actions">
          <span class="industrial-badge" :class="autoFollow ? 'success' : 'warning'">
            {{ terminalStatusLabel }}
          </span>
          <span class="industrial-badge">
            [{{ t('protocols.bufferCount', { count: items.length }) }}]
          </span>
        </div>
      </div>

      <div class="industrial-card logs-filter-toolbar">
        <div class="card-header">
          <strong>> {{ t('protocols.filters.apply') }}</strong>
        </div>
        <el-form label-position="top" class="logs-filter-grid protocol-form-grid">
          <el-form-item :label="t('protocols.filters.level')">
            <el-select v-model="filters.level" clearable :placeholder="t('protocols.filters.all')" class="refined-input">
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
        </el-form>

        <div class="logs-filter-actions">
          <el-button class="industrial-btn primary" @click="refreshLogs">[ {{ t('protocols.filters.apply') }} ]</el-button>
          <el-button class="industrial-btn" v-if="autoFollow" @click="pauseAutoFollow">[ {{ t('protocols.logsPause') }} ]</el-button>
          <el-button class="industrial-btn" v-else @click="resumeAutoFollow">[ {{ t('protocols.logsResume') }} ]</el-button>
          <el-button class="industrial-btn outline" @click="clearBuffer">[ {{ t('protocols.logsClear') }} ]</el-button>
        </div>
      </div>

      <RetryPanel
        v-if="logsError && items.length === 0"
        :title="t('errors.common.loadFailed')"
        :description="logsError"
        :loading="logsLoading"
        @retry="refreshLogs"
      />

      <el-alert v-else-if="logsError" :title="t('errors.common.loadFailed')" type="error" :description="logsError" show-icon />

      <div v-else class="protocol-log-layout">
        <div class="industrial-card terminal-stream-panel">
          <div class="card-header">
            <strong>> {{ t('protocols.logsStreamTitle') }}</strong>
          </div>

          <div v-if="items.length === 0" class="term-empty">
            [{{ t('protocols.logsEmpty') }}]
          </div>

          <div v-else ref="terminalScroller" class="protocol-terminal" aria-label="协议日志终端流">
            <button
              v-for="log in items"
              :key="log.log_id"
              type="button"
              class="protocol-terminal-line"
              :class="[getLogRowClass(log.log_id), getLevelPillClass(log.level)]"
              @click="handleLogSelection(log.log_id)"
            >
              <div class="terminal-line__meta">
                <span class="meta-time">[{{ formatDateTime(log.timestamp) }}]</span>
                <span class="meta-level">[{{ getLogLevelLabel(log.level) }}]</span>
                <span class="meta-protocol">[{{ getLogProtocolLabel(log.protocol) }}]</span>
                <span class="meta-source">{{ log.source }}</span>
              </div>
              <div class="terminal-line__message">
                {{ log.message }}
              </div>
              <small class="terminal-line__request">
                ID: {{ log.request_id || t('protocols.noRequestId') }}
              </small>
            </button>
          </div>
        </div>

        <div class="industrial-card protocol-log-detail-panel">
          <div class="card-header">
            <strong>> {{ t('protocols.logsDetailTitle') }}</strong>
          </div>

          <div v-if="!selectedSummary && !detailLoading" class="term-empty">
            [{{ t('protocols.logsDetailEmpty') }}]
          </div>

          <el-skeleton v-else :loading="detailLoading && !currentDetail" animated>
            <div v-if="selectedSummary" class="protocol-log-detail">
              <el-alert
                v-if="detailError"
                :title="t('errors.common.loadFailed')"
                type="error"
                :description="detailError"
                show-icon
                class="section-gap"
              />

              <div class="detail-summary-card">
                <div class="detail-summary-card__top">
                  <span class="industrial-badge">{{ getLogLevelLabel(selectedSummary.level) }}</span>
                  <span class="industrial-badge">{{ getLogProtocolLabel(selectedSummary.protocol) }}</span>
                </div>
                <strong class="detail-message">> {{ selectedSummary.message }}</strong>
                <div class="detail-summary-card__meta">
                  <span>[{{ formatDateTime(selectedSummary.timestamp) }}]</span>
                  <span>{{ selectedSummary.source }}</span>
                  <span>{{ selectedSummary.request_id || t('protocols.noRequestId') }}</span>
                  <span class="mono-id">ID: {{ selectedSummary.log_id }}</span>
                </div>
              </div>

              <div v-if="detailEntries.length > 0" class="detail-key-grid">
                <div v-for="entry in detailEntries" :key="entry.key" class="detail-key-card">
                  <small class="mono-label">[{{ entry.label }}]</small>
                  <strong class="mono-value">{{ entry.value }}</strong>
                </div>
              </div>

              <div class="detail-json-block">
                <div class="detail-json-block__header">
                  <strong>> {{ t('protocols.logsDetailJson') }}</strong>
                </div>
                <pre>{{ detailJson }}</pre>
              </div>
            </div>
          </el-skeleton>
        </div>
      </div>
    </section>
  </div>
</template>

<style lang="scss" scoped>
.industrial-theme {
  --bg-color: #f4f4f0;
  --border-color: #111111;
  --text-main: #111111;
  --text-muted: #555555;
  --accent-color: #ff4500;
  --accent-hover: #e03c00;
  --card-bg: #ffffff;
  
  color: var(--text-main);
  background-color: var(--bg-color);
  background-image: 
    linear-gradient(rgba(17, 17, 17, 0.05) 1px, transparent 1px),
    linear-gradient(90deg, rgba(17, 17, 17, 0.05) 1px, transparent 1px);
  background-size: 20px 20px;
  padding: 24px;
  min-height: 100%;
}

.hero-panel {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  margin-bottom: 32px;
  border-bottom: 4px solid var(--border-color);
  padding-bottom: 16px;
}

.hero-text h1 {
  font-size: 2.5rem;
  font-weight: 900;
  text-transform: uppercase;
  margin: 0;
  letter-spacing: -0.05em;
  font-family: system-ui, -apple-system, sans-serif;
}

.hero-text .subtitle, .section-heading .subtitle {
  font-family: "Cascadia Mono", monospace;
  color: var(--accent-color);
  margin: 8px 0 0;
  font-weight: bold;
}

.hero-actions {
  display: flex;
  gap: 12px;
}

/* Industrial Buttons */
.industrial-btn {
  border: 2px solid var(--border-color) !important;
  background: var(--card-bg) !important;
  color: var(--text-main) !important;
  font-family: "Cascadia Mono", monospace !important;
  font-weight: bold !important;
  border-radius: 0 !important;
  padding: 8px 16px !important;
  text-transform: uppercase;
  box-shadow: 4px 4px 0px var(--border-color) !important;
  transition: transform 0.1s, box-shadow 0.1s !important;
}
.industrial-btn:hover:not(:disabled) {
  transform: translate(2px, 2px) !important;
  box-shadow: 2px 2px 0px var(--border-color) !important;
}
.industrial-btn.primary {
  background: var(--accent-color) !important;
  color: #fff !important;
}
.industrial-btn.outline {
  background: transparent !important;
}

/* Cards */
.industrial-card {
  background: var(--card-bg);
  border: 3px solid var(--border-color);
  box-shadow: 6px 6px 0px var(--border-color);
  margin-bottom: 32px;
}

.card-header {
  background: var(--border-color);
  color: #fff;
  padding: 12px 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-family: "Cascadia Mono", monospace;
  text-transform: uppercase;
}

.industrial-badge {
  background: var(--card-bg);
  color: var(--text-main);
  border: 1px solid var(--border-color);
  padding: 4px 8px;
  font-size: 0.8rem;
  font-weight: bold;
  font-family: "Cascadia Mono", monospace;
}
.industrial-badge.success { border-color: #00a86b; color: #00a86b; }
.industrial-badge.danger { border-color: #ff4500; color: #ff4500; }
.industrial-badge.warning { border-color: #ffb000; color: #ffb000; }

.mono-label {
  font-family: "Cascadia Mono", monospace;
  font-size: 0.85rem;
  color: var(--text-muted);
  text-transform: uppercase;
}

.mono-value {
  font-family: "Cascadia Mono", monospace;
  font-size: 1.1rem;
  font-weight: bold;
  word-break: break-all;
}

/* Section Heading */
.section-heading {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: 24px;
  border-bottom: 3px solid var(--border-color);
  padding-bottom: 8px;
}
.section-heading h2 {
  font-size: 1.5rem;
  font-weight: 800;
  margin: 0;
  text-transform: uppercase;
  font-family: system-ui, -apple-system, sans-serif;
}
.terminal-header-actions {
  display: flex;
  gap: 12px;
}

/* Filter */
.logs-filter-toolbar {
  margin-bottom: 32px;
}
.protocol-form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 24px;
  padding: 20px;
}

:deep(.el-form-item__label) {
  font-family: "Cascadia Mono", monospace;
  font-weight: bold;
  color: var(--text-main);
}

.refined-input {
  :deep(.el-input__wrapper) {
    border-radius: 0;
    border: 2px solid var(--border-color);
    background: #fff;
    box-shadow: none !important;
    font-family: "Cascadia Mono", monospace;
    transition: all 0.2s;

    &:hover, &.is-focus {
      border-color: var(--accent-color);
      box-shadow: 4px 4px 0px var(--border-color) !important;
      transform: translate(-2px, -2px);
    }
  }
}

.logs-filter-actions {
  display: flex;
  gap: 12px;
  padding: 0 20px 20px;
}

/* Layout */
.protocol-log-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(360px, 0.8fr);
  gap: 24px;
  align-items: stretch;
}

/* Terminal Stream */
.protocol-terminal {
  min-height: 520px;
  max-height: 680px;
  overflow-y: auto;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  background: rgba(17, 17, 17, 0.02);
}

.protocol-terminal-line {
  width: 100%;
  border: 0;
  border-left: 4px solid transparent;
  padding: 8px 12px;
  background: #fff;
  border: 1px solid var(--border-color);
  font-family: "Cascadia Mono", monospace;
  cursor: pointer;
  text-align: left;
  display: grid;
  gap: 6px;
  transition: all 0.1s;

  &:hover {
    transform: translateX(4px);
    box-shadow: -4px 0 0 var(--accent-color);
  }

  &.is-selected {
    background: var(--border-color);
    color: #fff;
    box-shadow: -4px 0 0 var(--accent-color);
  }
}

.terminal-line__meta {
  display: flex;
  gap: 12px;
  font-size: 0.8rem;
  color: var(--text-muted);
  flex-wrap: wrap;
}
.protocol-terminal-line.is-selected .terminal-line__meta {
  color: rgba(255, 255, 255, 0.7);
}

.terminal-line__message {
  font-weight: bold;
  line-height: 1.4;
  white-space: pre-wrap;
  word-break: break-word;
}

.terminal-line__request {
  font-size: 0.75rem;
  opacity: 0.6;
}

.term-empty {
  padding: 40px;
  text-align: center;
  font-family: "Cascadia Mono", monospace;
  color: var(--text-muted);
}

/* Detail Panel */
.protocol-log-detail {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 20px;
  max-height: 680px;
  overflow-y: auto;
}

.detail-summary-card {
  border: 2px solid var(--border-color);
  padding: 16px;
  background: rgba(17, 17, 17, 0.03);
  display: grid;
  gap: 12px;
}

.detail-summary-card__top {
  display: flex;
  gap: 8px;
}

.detail-message {
  font-size: 1.2rem;
  word-break: break-word;
}

.detail-summary-card__meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-family: "Cascadia Mono", monospace;
  font-size: 0.85rem;
  color: var(--text-muted);
}

.detail-key-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 12px;
}

.detail-key-card {
  border: 2px dashed var(--border-color);
  padding: 12px;
  display: grid;
  gap: 6px;
}

.detail-json-block {
  border: 2px solid var(--border-color);
  background: var(--border-color);
  color: #fff;
  padding: 16px;
  box-shadow: 4px 4px 0 var(--accent-color);
}
.detail-json-block__header {
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px dashed rgba(255, 255, 255, 0.3);
}
.detail-json-block pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: "Cascadia Mono", monospace;
  font-size: 0.85rem;
}

.is-info { border-left-color: #00a2ff; }
.is-warn { border-left-color: #ffb000; }
.is-error { border-left-color: #ff4500; }

@media (max-width: 1024px) {
  .protocol-log-layout {
    grid-template-columns: 1fr;
  }
  .hero-panel {
    flex-direction: column;
    align-items: flex-start;
    gap: 16px;
  }
}
</style>
