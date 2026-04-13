<script setup lang="ts">
import { computed, nextTick, onActivated, onDeactivated, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'

import AppPage from '@/components/page/AppPage.vue'
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
    hasDetailFieldValue(resolveDetailFieldValue(key, details))
      ? [{
        key,
        label: t(`protocols.detailFields.${key}`),
        value: formatDetailValue(key, resolveDetailFieldValue(key, details)),
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
const desktopWorkspaceBottomGap = 12
const skipNextActivation = ref(false)
const initialHistoryLoaded = ref(false)
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
    initialHistoryLoaded.value = true
    updateWorkspaceHeight()
    await scrollTerminalToBottom('auto')
  } catch {
    // store error state drives the page
  }
}

async function activatePage() {
  protocolLogsStore.activate()
  startLayoutObserver()
  updateWorkspaceHeight()
  if (!initialHistoryLoaded.value || items.value.length === 0) {
    await loadPage()
    return
  }

  await scrollTerminalToBottom('auto')
}

onMounted(() => {
  skipNextActivation.value = true
  void activatePage()
})

onActivated(() => {
  if (skipNextActivation.value) {
    skipNextActivation.value = false
    return
  }

  void activatePage()
})

onDeactivated(() => {
  protocolLogsStore.deactivate()
  stopLayoutObserver()
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
    const shellMain = root?.closest('.admin-layout__content') as HTMLElement | null
    if (!root || !workspace) {
      return
    }

    const workspaceRect = workspace.getBoundingClientRect()
    const rootStyles = window.getComputedStyle(root)
    const shellMainStyles = shellMain ? window.getComputedStyle(shellMain) : null
    const paddingBottom = Number.parseFloat(rootStyles.paddingBottom || '0') || 0
    const shellPaddingBottom = Number.parseFloat(shellMainStyles?.paddingBottom || '0') || 0
    const containerBottom = shellMain?.getBoundingClientRect().bottom ?? window.innerHeight
    const visibleBottom = Math.min(containerBottom, window.innerHeight)
    const availableHeight = Math.floor(
      visibleBottom
      - workspaceRect.top
      - shellPaddingBottom
      - paddingBottom
      - desktopWorkspaceBottomGap,
    )

    workspaceHeight.value = availableHeight > 0 ? availableHeight : null
  })
}

function startLayoutObserver() {
  if (typeof window === 'undefined') {
    return
  }

  const root = pageRoot.value
  const shellMain = root?.closest('.admin-layout__content') as HTMLElement | null
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

function resolveDetailFieldValue(key: ProtocolDetailFieldKey, details: Record<string, unknown>) {
  switch (key) {
    case 'sender_id':
      return getSenderDetailValue(details, 'user_id')
    case 'sender_nickname':
      return getSenderDetailValue(details, 'nickname')
    case 'sender_card':
      return getSenderDetailValue(details, 'card')
    case 'sender_role':
      return getSenderDetailValue(details, 'role')
    case 'sender_title':
      return getSenderDetailValue(details, 'title')
    default:
      return details[key]
  }
}

function getSenderDetailValue(details: Record<string, unknown>, key: string) {
  return toDetailRecord(details.sender)[key]
}

function hasDetailFieldValue(value: unknown) {
  return value !== null && value !== undefined && value !== ''
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

function getLevelTagColor(level: string) {
  switch (level) {
    case 'error': return 'error'
    case 'warn': return 'warning'
    case 'info': return 'success'
    default: return 'default'
  }
}
</script>

<template>
  <AppPage :title="t('protocols.logsPageTitle')" :description="t('protocols.logsSubtitle')" full-height>
    <template #extra>
      <div class="table-actions">
        <a-button @click="router.push('/protocols')">{{ t('protocols.openSettings') }}</a-button>
        <a-button type="primary" :loading="logsLoading" @click="refreshLogs">
          {{ t('protocols.logsRefresh') }}
        </a-button>
      </div>
    </template>

    <div ref="pageRoot" class="protocol-logs-page">
      <div ref="workspaceRoot" class="protocol-logs-workspace" :style="workspaceStyle">
        <aside class="logs-sidebar">
          <a-card :bordered="false" class="sidebar-card">
            <template #title>
              <strong>{{ t('protocols.filters.apply') }}</strong>
            </template>

          <a-form layout="vertical" class="sidebar-filter-form">
            <a-form-item :label="t('protocols.filters.level')">
              <a-select
                v-model:value="filters.level"
                allow-clear
                :placeholder="t('protocols.filters.all')"
                class="refined-input"
                :options="[
                  { label: t('display.logLevels.debug'), value: 'debug' },
                  { label: t('display.logLevels.info'), value: 'info' },
                  { label: t('display.logLevels.warn'), value: 'warn' },
                  { label: t('display.logLevels.error'), value: 'error' },
                ]"
              />
            </a-form-item>
            <a-form-item :label="t('protocols.filters.source')">
              <a-input v-model:value="filters.source" :placeholder="t('protocols.filters.sourcePlaceholder')" class="refined-input" />
            </a-form-item>
            <a-form-item :label="t('protocols.filters.requestId')">
              <a-input v-model:value="filters.requestId" :placeholder="t('protocols.filters.requestPlaceholder')" class="refined-input" />
            </a-form-item>

            <div class="sidebar-actions">
              <a-button block @click="clearBuffer">{{ t('protocols.logsClear') }}</a-button>
              <a-button type="primary" block @click="refreshLogs">{{ t('protocols.filters.apply') }}</a-button>
            </div>
          </a-form>
          </a-card>

          <a-card :bordered="false" class="sidebar-card">
            <template #title>
              <strong>{{ t('dashboard.refresh') }}</strong>
            </template>
            <template #extra>
              <span class="buffer-info">{{ items.length }} / 200</span>
            </template>

          <div class="sidebar-controls">
            <div class="follow-status-pill" :class="autoFollow ? 'is-following' : 'is-paused'">
              <span class="status-dot"></span>
              {{ terminalStatusLabel }}
            </div>
            <a-button v-if="autoFollow" block @click="pauseAutoFollow">{{ t('protocols.logsPause') }}</a-button>
            <a-button v-else type="primary" block @click="resumeAutoFollow">{{ t('protocols.logsResume') }}</a-button>
          </div>
          </a-card>
        </aside>

        <main class="logs-main-content">
          <RetryPanel
            v-if="logsError && items.length === 0"
            :title="t('errors.common.loadFailed')"
            :description="logsError"
            :loading="logsLoading"
            @retry="refreshLogs"
          />

          <a-alert v-else-if="logsError" :message="t('errors.common.loadFailed')" type="error" :description="logsError" show-icon class="error-alert" />

          <div v-else class="logs-display-grid">
            <a-card :bordered="false" class="terminal-card">
              <template #title>
                <strong>{{ t('protocols.logsStreamTitle') }}</strong>
              </template>
              <template #extra>
                <a-tag :color="autoFollow ? 'success' : 'default'">{{ terminalStatusLabel }}</a-tag>
              </template>

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
            </a-card>

            <a-card :bordered="false" class="detail-card">
              <template #title>
                <strong>{{ t('protocols.logsDetailTitle') }}</strong>
              </template>

              <div v-if="!selectedSummary && !detailLoading" class="detail-empty-state">
                <div class="empty-icon">?</div>
                <p>{{ t('protocols.logsDetailEmpty') }}</p>
              </div>

              <a-skeleton v-else :loading="detailLoading && !currentDetail" active>
                <div v-if="selectedSummary" class="detail-view-content">
                  <a-alert
                    v-if="detailError"
                    :message="t('errors.common.loadFailed')"
                    type="error"
                    :description="detailError"
                    show-icon
                    class="detail-error"
                  />

                  <header class="detail-hero">
                    <div class="detail-hero-top">
                      <a-tag :color="getLevelTagColor(selectedSummary.level)">{{ getLogLevelLabel(selectedSummary.level) }}</a-tag>
                      <a-tag color="blue">{{ getLogProtocolLabel(selectedSummary.protocol) }}</a-tag>
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
              </a-skeleton>
            </a-card>
          </div>
        </main>
      </div>
    </div>
  </AppPage>
</template>

<style lang="scss" scoped>
.protocol-logs-page {
  --space-xs: 4px;
  --space-sm: 8px;
  --space-md: 12px;
  --space-lg: 16px;
  --space-xl: 20px;
  --font-sans: "PingFang SC", "Hiragino Sans GB", "Noto Sans SC", "Microsoft YaHei", sans-serif;
  --font-mono: "Cascadia Mono", "Consolas", monospace;
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  color: var(--app-text);
}

.protocol-logs-workspace {
  display: grid;
  grid-template-columns: 260px minmax(0, 1fr);
  gap: var(--space-lg);
  flex: 1;
  height: 100%;
  max-height: 100%;
  min-height: 0;
  overflow: hidden;
}

.logs-sidebar {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
  min-height: 0;
  overflow-y: auto;
}

.sidebar-card :deep(.ant-card-head),
.terminal-card :deep(.ant-card-head),
.detail-card :deep(.ant-card-head) {
  min-height: 48px;
  padding-inline: 16px;
}

.sidebar-card :deep(.ant-card-body) {
  padding: 14px 16px 16px;
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.sidebar-filter-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.sidebar-filter-form :deep(.ant-form-item) {
  margin-bottom: 0;
}

.buffer-info {
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--app-text-secondary);
}

.sidebar-actions,
.sidebar-controls {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.follow-status-pill {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  padding: 9px 12px;
  border-radius: 10px;
  font-size: 0.85rem;
  font-weight: 600;
  background: var(--surface-soft);
  border: 1px solid var(--app-border);
  color: var(--app-text-secondary);

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: 999px;
    background: currentColor;
  }

  &.is-following {
    color: var(--app-success);
    border-color: color-mix(in srgb, var(--app-success) 28%, transparent);
    background: color-mix(in srgb, var(--app-success) 10%, transparent);
  }
}

.logs-main-content {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.logs-display-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 360px;
  gap: var(--space-lg);
  flex: 1;
  max-height: 100%;
  min-height: 0;
  overflow: hidden;
}

.terminal-card,
.detail-card {
  display: flex;
  flex-direction: column;
  height: 100%;
  max-height: 100%;
  min-height: 0;
  overflow: hidden;
}

.terminal-card :deep(.ant-card-head),
.detail-card :deep(.ant-card-head) {
  flex-shrink: 0;
}

.terminal-card :deep(.ant-card-body),
.detail-card :deep(.ant-card-body) {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  height: auto;
  min-height: 0;
  padding: 0;
  overflow: hidden;
}

.terminal-view-scroller {
  flex: 1 1 0;
  max-height: 100%;
  min-height: 0;
  overflow-y: auto;
  background: transparent;
  scroll-padding-block: 20px 24px;
}

.terminal-content {
  display: flex;
  flex-direction: column;
  padding: 8px 0 16px;
}

.terminal-line {
  width: 100%;
  display: flex;
  align-items: flex-start;
  gap: 12px;
  border: none;
  border-bottom: 1px solid color-mix(in srgb, var(--app-border) 70%, transparent);
  background: transparent;
  padding: 10px 14px;
  text-align: left;
  cursor: pointer;
  color: var(--app-text);
  transition: background-color 0.2s ease;
  scroll-margin-block: 24px;

  &:hover {
    background: color-mix(in srgb, var(--app-primary) 6%, transparent);
  }

  &.is-selected {
    background: color-mix(in srgb, var(--app-primary) 12%, transparent);
    box-shadow: inset 2px 0 0 var(--app-primary);
  }
}

.line-level-indicator {
  width: 8px;
  height: 8px;
  margin-top: 6px;
  border-radius: 999px;
  flex-shrink: 0;
  background: var(--app-text-secondary);

  &.success {
    background: var(--app-success);
  }

  &.warning {
    background: var(--app-warning);
  }

  &.danger {
    background: var(--app-danger);
  }
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
  width: 140px;
  flex-shrink: 0;
  font-size: 0.78rem;
  color: var(--app-text-secondary);
  font-family: var(--font-mono);
}

.line-source {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.line-body {
  flex: 1;
  font-size: 0.85rem;
  line-height: 1.5;
  word-break: break-word;
  white-space: pre-wrap;
}

.detail-view-content {
  flex: 1;
  max-height: 100%;
  min-height: 0;
  overflow-y: auto;
}

.detail-hero {
  padding: 16px;
  background: transparent;
  border-bottom: 1px solid var(--app-border);
}

.detail-hero-top {
  display: flex;
  gap: var(--space-sm);
  margin-bottom: var(--space-md);
}

.detail-hero-message {
  margin: 0 0 var(--space-md);
  font-family: var(--font-sans);
  font-size: 1rem;
  font-weight: 700;
  line-height: 1.5;
  color: var(--app-text);
  word-break: break-word;
}

.detail-hero-meta {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);
}

.mono-label {
  color: var(--app-text-secondary);
}

.mono-value {
  font-family: var(--font-mono);
  color: var(--app-text);
}

.meta-row {
  display: flex;
  gap: var(--space-sm);
  font-size: 0.8rem;

  .mono-label {
    width: 80px;
    flex-shrink: 0;
  }
}

.detail-fields-section {
  padding: 16px;
  border-bottom: 1px solid var(--app-border);
}

.detail-fields-grid {
  display: grid;
  gap: var(--space-sm);
}

.detail-field-box {
  display: flex;
  gap: var(--space-md);
  align-items: baseline;
}

.field-label {
  width: 108px;
  flex-shrink: 0;
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--app-text-secondary);
}

.field-value {
  font-family: var(--font-mono);
  font-size: 0.85rem;
  color: var(--app-text);
  word-break: break-all;
}

.detail-json-section {
  padding: 16px;
}

.json-header {
  margin-bottom: var(--space-md);
}

.json-content {
  margin: 0;
  padding: 14px;
  border-radius: 10px;
  border: 1px solid var(--app-border);
  background: var(--surface-soft);
  color: var(--app-text-secondary);
  font-family: var(--font-mono);
  font-size: 0.8rem;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
}

:deep(.detail-card .ant-skeleton),
:deep(.detail-card .ant-skeleton-content) {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
}

:deep(.refined-input.ant-input),
:deep(.refined-input.ant-select .ant-select-selector) {
  border-radius: 10px;
  background: color-mix(in srgb, var(--app-card-bg) 88%, var(--app-border) 12%);
  border-color: transparent;
  box-shadow: inset 0 -1px 0 color-mix(in srgb, var(--app-border) 70%, var(--app-primary) 30%);
}

:deep(.refined-input.ant-input:hover),
:deep(.refined-input.ant-select:hover .ant-select-selector) {
  border-color: transparent;
}

:deep(.refined-input.ant-input:focus),
:deep(.refined-input.ant-select.ant-select-focused .ant-select-selector) {
  border-color: var(--app-primary);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--app-primary) 16%, transparent);
}

.term-empty-state,
.detail-empty-state {
  display: flex;
  flex: 1;
  min-height: 240px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px;
  text-align: center;
  color: var(--app-text-secondary);

  .empty-icon {
    margin-bottom: var(--space-md);
    font-size: 2.2rem;
    font-weight: 800;
    opacity: 0.35;
  }
}

.term-empty-state {
  background: color-mix(in srgb, var(--surface-soft) 84%, var(--surface-strong));
}

.detail-empty-state {
  background: color-mix(in srgb, var(--surface-soft) 88%, var(--surface-strong));
}

@media (max-width: 1200px) {
  .logs-display-grid {
    grid-template-columns: 1fr;
  }

  .terminal-card {
    min-height: 420px;
  }

  .detail-card {
    min-height: 360px;
  }
}

@media (max-width: 900px) {
  .protocol-logs-page,
  .protocol-logs-workspace,
  .logs-main-content,
  .logs-display-grid,
  .terminal-card,
  .detail-card,
  .terminal-view-scroller,
  .detail-view-content {
    height: auto;
    min-height: 0;
    overflow: visible;
  }

  .protocol-logs-workspace {
    grid-template-columns: 1fr;
  }

  .logs-sidebar {
    display: grid;
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 600px) {
  .logs-sidebar {
    grid-template-columns: 1fr;
  }

  .line-content-wrap {
    flex-direction: column;
    gap: 4px;
  }

  .line-meta {
    width: auto;
  }
}
</style>
