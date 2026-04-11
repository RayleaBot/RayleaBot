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
const pageRoot = ref<HTMLElement | null>(null)
const workspaceRoot = ref<HTMLElement | null>(null)
const workspaceHeight = ref<number | null>(null)
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
const workspaceStyle = computed(() => (
  workspaceHeight.value && workspaceHeight.value > 0
    ? { height: `${workspaceHeight.value}px` }
    : undefined
))
let layoutObserver: ResizeObserver | null = null

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
    updateWorkspaceHeight()
    await scrollTerminalToBottom('auto')
  } catch {
    // store error state drives the page
  }
}

onMounted(() => {
  protocolLogsStore.activate()
  startLayoutObserver()
  void loadPage()
  updateWorkspaceHeight()
})

onUnmounted(() => {
  protocolLogsStore.deactivate()
  stopLayoutObserver()
})

async function refreshLogs() {
  try {
    await protocolLogsStore.fetchList()
    updateWorkspaceHeight()
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

function updateWorkspaceHeight() {
  if (typeof window === 'undefined') {
    return
  }

  void nextTick(() => {
    const mobileQuery = typeof window.matchMedia === 'function'
      ? window.matchMedia('(max-width: 900px)')
      : null

    if (mobileQuery?.matches) {
      workspaceHeight.value = null
      return
    }

    const root = pageRoot.value
    const workspace = workspaceRoot.value
    const shellMain = root?.closest('.shell-main') as HTMLElement | null
    if (!root || !workspace) {
      return
    }

    const workspaceRect = workspace.getBoundingClientRect()
    const rootStyles = window.getComputedStyle(root)
    const shellMainStyles = shellMain ? window.getComputedStyle(shellMain) : null
    const paddingBottom = Number.parseFloat(rootStyles.paddingBottom || '0') || 0
    const shellPaddingBottom = Number.parseFloat(shellMainStyles?.paddingBottom || '0') || 0
    const containerBottom = shellMain?.getBoundingClientRect().bottom ?? window.innerHeight
    const availableHeight = Math.floor(containerBottom - workspaceRect.top - shellPaddingBottom - paddingBottom)

    workspaceHeight.value = availableHeight > 0 ? availableHeight : null
  })
}

function startLayoutObserver() {
  if (typeof window === 'undefined') {
    return
  }

  const root = pageRoot.value
  const shellMain = root?.closest('.shell-main') as HTMLElement | null
  if (!root || typeof window.ResizeObserver !== 'function') {
    return
  }

  layoutObserver = new window.ResizeObserver(() => {
    updateWorkspaceHeight()
  })

  layoutObserver.observe(root)
  if (shellMain) {
    layoutObserver.observe(shellMain)
  }
}

function stopLayoutObserver() {
  layoutObserver?.disconnect()
  layoutObserver = null
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
  <div ref="pageRoot" class="page-grid page-grid--viewport minimal-protocol-theme protocol-logs-page">
    <section class="hero-panel">
      <div class="hero-text">
        <h1 class="main-title">{{ t('protocols.logsPageTitle') }}</h1>
        <p class="subtitle">{{ t('protocols.logsSubtitle') }}</p>
      </div>

      <div class="hero-actions">
        <button class="minimal-btn text" @click="router.push('/protocols')">
          {{ t('protocols.openSettings') }}
        </button>
        <button class="minimal-btn primary" :disabled="logsLoading" @click="refreshLogs">
          <span v-if="logsLoading">{{ t('protocols.logsRefresh') }}...</span>
          <span v-else>{{ t('protocols.logsRefresh') }}</span>
        </button>
      </div>
    </section>

    <div ref="workspaceRoot" class="protocol-logs-workspace" :style="workspaceStyle">
      <aside class="logs-sidebar">
        <div class="sidebar-palette">
          <div class="palette-header">
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

        <div class="sidebar-palette">
          <div class="palette-header">
            <strong>{{ t('dashboard.refresh') }}</strong>
            <span class="buffer-info">{{ items.length }} / 200</span>
          </div>
          <div class="sidebar-controls">
            <div class="follow-status-pill" :class="autoFollow ? 'is-following' : 'is-paused'">
              <span class="status-dot"></span>
              {{ terminalStatusLabel }}
            </div>
            <button class="minimal-btn outline" v-if="autoFollow" @click="pauseAutoFollow">
              {{ t('protocols.logsPause') }}
            </button>
            <button class="minimal-btn primary" v-else @click="resumeAutoFollow">
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
            <div class="terminal-header">
              <div class="terminal-dots">
                <span></span><span></span><span></span>
              </div>
              <strong class="terminal-title">{{ t('protocols.logsStreamTitle') }}</strong>
            </div>

            <div v-if="items.length === 0" class="term-empty-state">
              <div class="empty-icon">~</div>
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
                      <span class="line-time">{{ formatDateTime(log.timestamp).split(' ')[1] }}</span>
                      <span class="line-source">{{ log.source }}</span>
                    </div>
                    <div class="line-body">
                      <span class="line-text">{{ log.message }}</span>
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
                      <span class="mono-label">{{ t('protocols.fields.timestamp') }}</span>
                      <span class="mono-value">{{ formatDateTime(selectedSummary.timestamp) }}</span>
                    </div>
                    <div class="meta-row">
                      <span class="mono-label">{{ t('protocols.fields.source') }}</span>
                      <span class="mono-value">{{ selectedSummary.source }} <template v-if="selectedSummary.plugin_id">(@{{ selectedSummary.plugin_id }})</template></span>
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
.protocol-logs-page {
  display: flex;
  flex-direction: column;
  gap: var(--space-xl);
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.protocol-logs-page > .hero-panel {
  margin-bottom: 0;
}

.protocol-logs-workspace {
  display: grid;
  grid-template-columns: 280px minmax(0, 1fr);
  grid-template-rows: minmax(0, 1fr);
  gap: var(--space-xl);
  align-items: stretch;
  align-content: stretch;
  flex: 1;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.logs-sidebar {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
  height: 100%;
  overflow-y: auto;
  padding-right: 8px;
  min-height: 0;

  &::-webkit-scrollbar {
    width: 6px;
  }
  
  &::-webkit-scrollbar-track {
    background: transparent;
  }

  &::-webkit-scrollbar-thumb {
    background: rgba(0, 0, 0, 0.1);
    border-radius: 10px;
  }
  
  &:hover::-webkit-scrollbar-thumb,
  &::-webkit-scrollbar-thumb:hover {
    background: rgba(0, 0, 0, 0.2);
  }
}

.sidebar-palette {
  background: var(--theme-surface);
  border: 1px solid var(--theme-border);
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.02);
}

.palette-header {
  padding: var(--space-md) var(--space-lg);
  background: var(--theme-surface-soft);
  border-bottom: 1px solid var(--theme-border);
  display: flex;
  justify-content: space-between;
  align-items: center;

  strong {
    font-size: 0.9rem;
    font-weight: 700;
    color: var(--theme-text);
  }

  .buffer-info {
    font-family: var(--font-mono);
    font-size: 0.75rem;
    color: var(--theme-text-muted);
  }
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
  margin-top: var(--space-xs);
  
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

.follow-status-pill {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 0.85rem;
  font-weight: 600;
  background: var(--theme-bg);
  border: 1px solid var(--theme-border);

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--theme-text-muted);
  }

  &.is-following {
    border-color: oklch(70% 0.15 150 / 30%);
    background: oklch(70% 0.15 150 / 5%);
    color: var(--theme-success);
    .status-dot {
      background: var(--theme-success);
      box-shadow: 0 0 0 3px oklch(70% 0.15 150 / 15%);
    }
  }

  &.is-paused {
    color: var(--theme-text-muted);
  }
}

.logs-main-content {
  display: flex;
  flex-direction: column;
  flex: 1;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.logs-display-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 400px;
  grid-template-rows: minmax(0, 1fr);
  gap: var(--space-lg);
  align-content: stretch;
  flex: 1;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.terminal-container, .detail-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  margin-bottom: 0;
  overflow: hidden;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.04);
}

.terminal-header {
  display: flex;
  align-items: center;
  gap: var(--space-md);
  padding: 12px 16px;
  background: oklch(12% 0.02 235);
  border-bottom: 1px solid oklch(20% 0.02 235);
  border-radius: 16px 16px 0 0;

  .terminal-title {
    color: oklch(70% 0.02 235);
    font-family: var(--font-sans);
    font-size: 0.85rem;
    font-weight: 600;
    letter-spacing: 0.02em;
  }

  .terminal-dots {
    display: flex;
    gap: 6px;
    span {
      width: 12px;
      height: 12px;
      border-radius: 50%;
      background: oklch(30% 0.02 235);
      
      &:nth-child(1) { background: oklch(65% 0.18 25); }
      &:nth-child(2) { background: oklch(75% 0.15 70); }
      &:nth-child(3) { background: oklch(70% 0.15 150); }
    }
  }
}

.terminal-view-scroller {
  flex: 1 1 0;
  height: 0;
  min-height: 0;
  overflow-y: auto;
  background: oklch(15% 0.02 235);
  border-radius: 0 0 16px 16px;

  &::-webkit-scrollbar {
    width: 10px;
    background: oklch(15% 0.02 235);
    border-radius: 0 0 16px 0;
  }
  
  &::-webkit-scrollbar-track {
    background: transparent;
  }

  &::-webkit-scrollbar-thumb {
    background: rgba(255, 255, 255, 0.15) !important;
    border-radius: 10px;
    border: 3px solid oklch(15% 0.02 235);
  }
  
  &:hover::-webkit-scrollbar-thumb,
  &::-webkit-scrollbar-thumb:hover {
    background: rgba(255, 255, 255, 0.25) !important;
  }
}

.terminal-content {
  display: flex;
  flex-direction: column;
  padding: var(--space-sm) 0;
}

.terminal-line {
  width: 100%;
  background: transparent;
  border: none;
  padding: 6px var(--space-md);
  color: oklch(85% 0.01 235);
  font-family: var(--font-mono);
  text-align: left;
  cursor: pointer;
  display: flex;
  gap: var(--space-md);
  align-items: flex-start;
  transition: none;

  &:hover {
    background: oklch(22% 0.02 235);
  }

  &.is-selected {
    background: oklch(25% 0.04 235);
    position: relative;
    
    &::before {
      content: '';
      position: absolute;
      left: 0;
      top: 0;
      bottom: 0;
      width: 3px;
      background: var(--theme-accent);
    }
  }
}

.line-level-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-top: 5px;
  flex-shrink: 0;
  background: oklch(60% 0.01 235);

  &.success { background: var(--theme-success); box-shadow: 0 0 8px var(--theme-success); }
  &.warning { background: var(--theme-warning); box-shadow: 0 0 8px var(--theme-warning); }
  &.danger { background: var(--theme-danger); box-shadow: 0 0 8px var(--theme-danger); }
}

.line-content-wrap {
  display: flex;
  flex: 1;
  min-width: 0;
  gap: var(--space-md);
}

.line-meta {
  display: flex;
  gap: var(--space-sm);
  font-size: 0.8rem;
  opacity: 0.6;
  width: 160px;
  flex-shrink: 0;
}

.line-time {
  color: oklch(60% 0.02 235);
}

.line-source {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.line-body {
  display: flex;
  flex: 1;
  font-size: 0.85rem;
  line-height: 1.4;
  word-break: break-all;
  white-space: pre-wrap;
}

/* Detail View */
.detail-view-content {
  flex: 1 1 auto;
  height: 100%;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.detail-hero {
  padding: var(--space-lg);
  background: var(--theme-surface-soft);
  border-bottom: 1px solid var(--theme-border);
}

.detail-hero-top {
  display: flex;
  gap: var(--space-sm);
  margin-bottom: var(--space-md);
}

.detail-hero-message {
  font-family: var(--font-sans);
  font-size: 1.15rem;
  font-weight: 700;
  margin: 0 0 var(--space-md);
  line-height: 1.4;
  color: var(--theme-text);
  word-break: break-word;
}

.detail-hero-meta {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);
}

.meta-row {
  display: flex;
  gap: var(--space-sm);
  font-size: 0.8rem;
  
  .mono-label { width: 80px; flex-shrink: 0; }
}

.detail-fields-section {
  padding: var(--space-lg);
  border-bottom: 1px solid var(--theme-border);
}

.detail-fields-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: var(--space-sm);
}

.detail-field-box {
  display: flex;
  gap: var(--space-md);
  align-items: baseline;
  padding: var(--space-xs) 0;

  .field-label {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--theme-text-muted);
    font-family: var(--font-sans);
    width: 120px;
    flex-shrink: 0;
  }

  .field-value {
    font-family: var(--font-mono);
    font-size: 0.85rem;
    font-weight: 500;
    color: var(--theme-text);
    word-break: break-all;
  }
}

.detail-json-section {
  padding: var(--space-lg);
  background: var(--theme-surface);
  flex: 0 0 auto;
  overflow: visible;
}

.json-header {
  margin-bottom: var(--space-md);
  strong {
    font-family: var(--font-sans);
    font-weight: 600;
    font-size: 0.9rem;
    color: var(--theme-text);
  }
}

.json-content {
  margin: 0;
  font-family: var(--font-mono);
  font-size: 0.8rem;
  white-space: pre-wrap;
  word-break: break-all;
  color: var(--theme-text-muted);
  background: oklch(98% 0.005 235);
  padding: var(--space-md);
  border-radius: 8px;
  border: 1px solid var(--theme-border);
  box-shadow: inset 0 2px 8px rgba(0, 0, 0, 0.02);
  overflow: visible;
}

:deep(.detail-container .el-skeleton),
:deep(.detail-container .el-skeleton__content) {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
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
    opacity: 0.3;
  }
}

.term-empty-state {
  background: oklch(15% 0.02 235);
  border-radius: 0 0 16px 16px;
  .empty-icon { opacity: 0.1; }
}

.detail-empty-state {
  background: var(--theme-surface-soft);
}

@media (max-width: 1200px) {
  .logs-display-grid {
    grid-template-columns: 1fr;
  }
  
  .terminal-container {
    height: 500px;
  }
  
  .detail-container {
    height: 400px;
  }
}

@media (max-width: 900px) {
  .protocol-logs-page {
    gap: var(--space-xl);
    height: auto;
    overflow: visible;
  }

  .protocol-logs-workspace {
    grid-template-columns: 1fr;
    grid-template-rows: none;
    height: auto;
    overflow: visible;
  }
  
  .logs-sidebar {
    display: grid;
    grid-template-columns: 1fr 1fr;
    height: auto;
  }

  .logs-main-content,
  .logs-display-grid,
  .terminal-container,
  .detail-container {
    height: auto;
    overflow: visible;
  }

  .terminal-view-scroller,
  .detail-view-content {
    flex: none;
    height: auto;
    overflow: visible;
  }
}

@media (max-width: 600px) {
  .logs-sidebar {
    grid-template-columns: 1fr;
  }
}
</style>
