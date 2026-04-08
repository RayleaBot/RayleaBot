<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'

import RetryPanel from '@/components/RetryPanel.vue'
import { getAdapterStateLabel, getLogLevelLabel, getLogProtocolLabel, getStatusType } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import { formatProtocolEventSummary, formatProtocolIssueSummary } from '@/lib/management-summary'
import { ONEBOT11_PROTOCOL_NAME, isProtocolEvent, isProtocolIssue } from '@/lib/protocols'
import { t } from '@/i18n'
import { useProtocolLogsStore } from '@/stores/protocol-logs'
import { useSystemStore } from '@/stores/system'

const protocolDetailFieldKeys = [
  'direction',
  'event_kind',
  'event_type',
  'conversation_type',
  'conversation_id',
  'sender_id',
  'message_id',
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
const systemStore = useSystemStore()

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
const { readiness, recentEvents, system } = storeToRefs(systemStore)

const terminalScroller = ref<HTMLElement | null>(null)

const protocolStatusLabel = computed(() => getAdapterStateLabel(system.value?.adapter_state))
const protocolStatusType = computed(() => getStatusType(system.value?.adapter_state))
const protocolSummary = computed(() => {
  const readinessIssue = readiness.value?.issues?.find((issue) => isProtocolIssue(issue))
  const issueSummary = formatProtocolIssueSummary(readinessIssue)
  if (issueSummary) {
    return issueSummary
  }

  const recentEvent = recentEvents.value.find((event) => isProtocolEvent(event))
  const eventSummary = formatProtocolEventSummary(recentEvent?.payload)
  if (eventSummary) {
    return eventSummary
  }

  return protocolStatusLabel.value
})
const selectedSummary = computed(() => currentDetail.value ?? selectedItem.value ?? null)
const detailEntries = computed(() => {
  const details = currentDetail.value?.details ?? {}
  return protocolDetailFieldKeys.flatMap((key) => (
    key in details
      ? [{
        key,
        label: t(`protocols.detailFields.${key}`),
        value: formatDetailValue(key, details[key as keyof typeof details]),
      }]
      : []
  ))
})
const detailJson = computed(() => JSON.stringify(currentDetail.value?.details ?? {}, null, 2))
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
    await scrollTerminalToBottom()
  },
)

async function loadPage() {
  try {
    const requests: Array<Promise<unknown>> = [protocolLogsStore.fetchList()]
    if (!system.value || !readiness.value) {
      requests.push(systemStore.refresh())
    }
    await Promise.all(requests)
    await scrollTerminalToBottom()
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
    await scrollTerminalToBottom()
  } catch {
    // store error state drives the page
  }
}

function clearBuffer() {
  protocolLogsStore.clearBuffer()
}

async function resumeAutoFollow() {
  await protocolLogsStore.resumeAutoFollow()
  await scrollTerminalToBottom()
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

async function scrollTerminalToBottom() {
  await nextTick()
  if (!terminalScroller.value) {
    return
  }

  terminalScroller.value.scrollTo({
    top: terminalScroller.value.scrollHeight,
    behavior: 'smooth',
  })
}

function formatDetailValue(key: ProtocolDetailFieldKey, value: unknown) {
  if (value === null || value === undefined || value === '') {
    return t('display.empty')
  }

  if (key === 'segments' && Array.isArray(value)) {
    return t('protocols.segmentCount', { count: value.length })
  }

  if (typeof value === 'object') {
    const raw = JSON.stringify(value)
    return raw.length > 140 ? `${raw.slice(0, 140)}...` : raw
  }

  return String(value)
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
  <div class="page-grid page-grid--viewport">
    <section class="hero-panel">
      <div>
        <h1>{{ t('protocols.logsPageTitle') }}</h1>
        <p>{{ t('protocols.logsSubtitle') }}</p>
      </div>

      <div class="hero-actions">
        <el-button plain @click="router.push('/protocols')">
          {{ t('protocols.openSettings') }}
        </el-button>
        <el-button :loading="logsLoading" @click="refreshLogs">
          {{ t('protocols.logsRefresh') }}
        </el-button>
      </div>
    </section>

    <el-card class="protocol-overview-card">
      <template #header>
        <div class="protocol-overview-header">
          <strong>{{ t('protocols.overviewTitle') }}</strong>
          <el-tag size="small" effect="dark" type="info">
            {{ t('protocols.fixedProtocolLabel') }}: {{ ONEBOT11_PROTOCOL_NAME }}
          </el-tag>
        </div>
      </template>

      <div class="protocol-overview-grid">
        <div class="protocol-overview-item">
          <small>{{ t('protocols.protocolNameLabel') }}</small>
          <strong>{{ ONEBOT11_PROTOCOL_NAME }}</strong>
        </div>
        <div class="protocol-overview-item">
          <small>{{ t('protocols.protocolStatusLabel') }}</small>
          <el-tag
            :type="protocolStatusType === 'danger' ? 'danger' : (protocolStatusType === 'warning' ? 'warning' : (protocolStatusType === 'success' ? 'success' : 'info'))"
            effect="plain"
          >
            {{ protocolStatusLabel }}
          </el-tag>
        </div>
        <div class="protocol-overview-item">
          <small>{{ t('protocols.protocolSummaryLabel') }}</small>
          <strong>{{ protocolSummary }}</strong>
        </div>
      </div>
    </el-card>

    <section class="protocol-logs-section">
      <div class="section-heading">
        <div>
          <h2>{{ t('protocols.logsTitle') }}</h2>
          <p>{{ t('protocols.logsStreamHint') }}</p>
        </div>
        <div class="terminal-header-actions">
          <el-tag effect="dark" :type="autoFollow ? 'success' : 'warning'">
            {{ terminalStatusLabel }}
          </el-tag>
          <el-tag effect="plain" type="info">
            {{ t('protocols.bufferCount', { count: items.length }) }}
          </el-tag>
        </div>
      </div>

      <el-card class="logs-filter-toolbar">
        <el-form label-position="top" class="logs-filter-grid">
          <el-form-item :label="t('protocols.filters.level')">
            <el-select v-model="filters.level" clearable :placeholder="t('protocols.filters.all')">
              <el-option :label="t('display.logLevels.debug')" value="debug" />
              <el-option :label="t('display.logLevels.info')" value="info" />
              <el-option :label="t('display.logLevels.warn')" value="warn" />
              <el-option :label="t('display.logLevels.error')" value="error" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('protocols.filters.source')">
            <el-input v-model="filters.source" :placeholder="t('protocols.filters.sourcePlaceholder')" />
          </el-form-item>
          <el-form-item :label="t('protocols.filters.requestId')">
            <el-input v-model="filters.requestId" :placeholder="t('protocols.filters.requestPlaceholder')" />
          </el-form-item>
        </el-form>

        <div class="logs-filter-actions">
          <el-button type="primary" @click="refreshLogs">{{ t('protocols.filters.apply') }}</el-button>
          <el-button v-if="autoFollow" @click="pauseAutoFollow">{{ t('protocols.logsPause') }}</el-button>
          <el-button v-else type="success" plain @click="resumeAutoFollow">{{ t('protocols.logsResume') }}</el-button>
          <el-button plain @click="clearBuffer">{{ t('protocols.logsClear') }}</el-button>
        </div>
      </el-card>

      <RetryPanel
        v-if="logsError && items.length === 0"
        :title="t('errors.common.loadFailed')"
        :description="logsError"
        :loading="logsLoading"
        @retry="refreshLogs"
      />

      <el-alert v-else-if="logsError" :title="t('errors.common.loadFailed')" type="error" :description="logsError" show-icon />

      <div v-else class="protocol-log-layout">
        <el-card class="protocol-log-terminal-card">
          <template #header>
            <div class="card-header">
              <div>
                <strong>{{ t('protocols.logsStreamTitle') }}</strong>
                <p>{{ t('protocols.logsStreamHint') }}</p>
              </div>
            </div>
          </template>

          <el-empty v-if="items.length === 0" :description="t('protocols.logsEmpty')" />

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
                <span>{{ formatDateTime(log.timestamp) }}</span>
                <span>{{ getLogLevelLabel(log.level) }}</span>
                <span>{{ getLogProtocolLabel(log.protocol) }}</span>
                <span>{{ log.source }}</span>
              </div>
              <div class="terminal-line__message">
                {{ log.message }}
              </div>
              <small class="terminal-line__request">
                {{ log.request_id || t('protocols.noRequestId') }}
              </small>
            </button>
          </div>
        </el-card>

        <el-card class="protocol-log-detail-card">
          <template #header>
            <div class="card-header">
              <div>
                <strong>{{ t('protocols.logsDetailTitle') }}</strong>
                <p>{{ t('protocols.logsDetailHint') }}</p>
              </div>
            </div>
          </template>

          <el-empty
            v-if="!selectedSummary && !detailLoading"
            :description="t('protocols.logsDetailEmpty')"
          />

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
                  <span class="detail-summary-card__level">{{ getLogLevelLabel(selectedSummary.level) }}</span>
                  <span class="detail-summary-card__protocol">{{ getLogProtocolLabel(selectedSummary.protocol) }}</span>
                </div>
                <strong>{{ selectedSummary.message }}</strong>
                <div class="detail-summary-card__meta">
                  <span>{{ formatDateTime(selectedSummary.timestamp) }}</span>
                  <span>{{ selectedSummary.source }}</span>
                  <span>{{ selectedSummary.request_id || t('protocols.noRequestId') }}</span>
                  <span>{{ selectedSummary.log_id }}</span>
                </div>
              </div>

              <div v-if="detailEntries.length > 0" class="detail-key-grid">
                <div v-for="entry in detailEntries" :key="entry.key" class="detail-key-card">
                  <small>{{ entry.label }}</small>
                  <strong>{{ entry.value }}</strong>
                </div>
              </div>

              <div class="detail-json-block">
                <div class="detail-json-block__header">
                  <strong>{{ t('protocols.logsDetailJson') }}</strong>
                </div>
                <pre>{{ detailJson }}</pre>
              </div>
            </div>
          </el-skeleton>
        </el-card>
      </div>
    </section>
  </div>
</template>

<style lang="scss" scoped>
.hero-actions,
.section-heading,
.protocol-overview-header,
.terminal-header-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.section-heading {
  margin-bottom: 12px;
}

.section-heading h2 {
  margin: 0;
  font-size: 1.2rem;
}

.section-heading p,
.card-header p {
  margin: 6px 0 0;
  color: var(--muted);
}

.protocol-overview-card,
.protocol-log-terminal-card,
.protocol-log-detail-card {
  border-radius: 24px;
}

.protocol-overview-grid,
.protocol-log-layout {
  display: grid;
  gap: 16px;
}

.protocol-overview-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.protocol-log-layout {
  grid-template-columns: minmax(0, 1.2fr) minmax(360px, 0.8fr);
  align-items: stretch;
}

.protocol-overview-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 18px 20px;
  border-radius: 20px;
  background: rgba(247, 250, 246, 0.88);
  border: 1px solid rgba(22, 33, 39, 0.08);
}

.protocol-overview-item small {
  color: var(--muted);
}

.protocol-terminal {
  min-height: 480px;
  max-height: 640px;
  overflow: auto;
  padding: 14px;
  border-radius: 20px;
  background:
    linear-gradient(180deg, rgba(14, 20, 25, 0.98), rgba(18, 26, 33, 0.98)),
    radial-gradient(circle at top right, rgba(86, 198, 255, 0.08), transparent 24%);
  border: 1px solid rgba(110, 204, 255, 0.12);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
  display: grid;
  gap: 10px;
}

.protocol-terminal-line {
  width: 100%;
  border: 0;
  padding: 12px 14px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.02);
  color: #e8eff6;
  cursor: pointer;
  text-align: left;
  display: grid;
  gap: 6px;
  transition: background-color 160ms ease, transform 160ms ease, box-shadow 160ms ease;

  &:hover {
    background: rgba(110, 204, 255, 0.08);
    transform: translateY(-1px);
  }

  &.is-selected {
    background: rgba(110, 204, 255, 0.14);
    box-shadow: inset 2px 0 0 #62d5ff;
  }
}

.terminal-line__meta,
.terminal-line__request,
.detail-summary-card__meta,
.detail-summary-card__top {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 12px;
  font-family: "Cascadia Mono", "Consolas", monospace;
  font-size: 0.78rem;
}

.terminal-line__meta {
  color: #8ca4b3;
}

.terminal-line__message {
  color: #f5f8fb;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-word;
}

.terminal-line__request {
  color: #6f8592;
}

.protocol-log-detail {
  display: grid;
  gap: 14px;
}

.detail-summary-card {
  display: grid;
  gap: 10px;
  padding: 16px 18px;
  border-radius: 18px;
  background: rgba(247, 250, 246, 0.9);
  border: 1px solid rgba(22, 33, 39, 0.08);
}

.detail-summary-card__top,
.detail-summary-card__meta {
  color: var(--muted);
}

.detail-summary-card__level,
.detail-summary-card__protocol {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  background: rgba(15, 111, 112, 0.08);
  color: #0f6f70;
}

.detail-key-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 12px;
}

.detail-key-card {
  display: grid;
  gap: 8px;
  padding: 14px 16px;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.84);
  border: 1px solid rgba(22, 33, 39, 0.08);
}

.detail-key-card small {
  color: var(--muted);
}

.detail-key-card strong {
  line-height: 1.55;
  word-break: break-word;
}

.detail-json-block {
  display: grid;
  gap: 10px;
  padding: 16px 18px;
  border-radius: 18px;
  background: #121a20;
  color: #e6edf3;
  border: 1px solid rgba(110, 204, 255, 0.12);
}

.detail-json-block__header {
  color: #8ca4b3;
}

.detail-json-block pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: "Cascadia Mono", "Consolas", monospace;
  font-size: 0.86rem;
  line-height: 1.55;
}

.is-debug {
  box-shadow: inset 2px 0 0 rgba(124, 140, 150, 0.48);
}

.is-info {
  box-shadow: inset 2px 0 0 rgba(88, 196, 255, 0.48);
}

.is-warn {
  box-shadow: inset 2px 0 0 rgba(255, 187, 74, 0.68);
}

.is-error {
  box-shadow: inset 2px 0 0 rgba(255, 104, 104, 0.72);
}

@media (max-width: 1024px) {
  .protocol-overview-grid,
  .protocol-log-layout {
    grid-template-columns: 1fr;
  }

  .logs-filter-toolbar .el-card__body,
  .logs-filter-grid {
    grid-template-columns: 1fr;
  }

  .logs-filter-actions,
  .hero-actions,
  .terminal-header-actions {
    justify-content: flex-start;
    flex-wrap: wrap;
  }
}
</style>
