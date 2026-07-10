<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'

import { useToastFeedback } from '@/adapter/feedback'
import ManagementContextActions from '@/components/ManagementContextActions.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getAdapterStateLabel, getReadinessStatusLabel, getStatusType } from '@/lib/display'
import { buildProtocolWorkbenchActions } from '@/lib/management-links'
import { ONEBOT11_PROTOCOL_NAME } from '@/lib/protocols'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'
import { useProtocolsStore } from '@/stores/protocols'
import { useProtocolConfigEditor } from './useProtocolConfigEditor'

const configStore = useConfigStore()
const protocolsStore = useProtocolsStore()

const {
  error: configError,
  loading: configLoading,
  redactedFields,
  restartRequired,
  saving,
} = storeToRefs(configStore)
const {
  error: protocolsError,
  snapshot,
} = storeToRefs(protocolsStore)

const {
  advancedExpanded,
  canSave,
  configSections,
  draft,
  readField,
  save,
  writeField,
} = useProtocolConfigEditor(configStore, protocolsStore)
const activeTransportState = computed(() => {
  if (!snapshot.value) {
    return undefined
  }

  const active = new Set(snapshot.value.active_transports)
  const preferred = snapshot.value.transport_status.find((item) => active.has(item.transport))
  return preferred?.state
})
const protocolStatusLabel = computed(() => (
  activeTransportState.value
    ? getAdapterStateLabel(activeTransportState.value)
    : getReadinessStatusLabel(snapshot.value?.readiness_status)
))
const protocolStatusType = computed(() => getStatusType(activeTransportState.value ?? snapshot.value?.readiness_status))
const readinessLabel = computed(() => getReadinessStatusLabel(snapshot.value?.readiness_status))
const readinessType = computed(() => getStatusType(snapshot.value?.readiness_status))
const protocolSummary = computed(() => snapshot.value?.summary ?? t('display.empty'))
const pageError = computed(() => configError.value || protocolsError.value)
const feedbackToast = computed(() => {
  if (pageError.value) {
    return {
      key: `protocols-error:${pageError.value}`,
      level: 'error' as const,
      message: pageError.value,
    }
  }

  if (redactedFields.value.length > 0) {
    return {
      key: `protocols-redacted:${redactedFields.value.join('|')}`,
      level: 'info' as const,
      message: `${t('config.redactedTitle')}：${redactedFields.value.join(', ')}`,
    }
  }

  return null
})
const transportLabelMap = {
  reverse_ws: t('config.sections.onebotReverseWs'),
  forward_ws: t('config.sections.onebotForwardWs'),
  http_api: t('config.sections.onebotHttpApi'),
  webhook: t('config.sections.onebotWebhook'),
} as const

function getStatusTagColor(status?: string) {
  if (status === 'success') return 'success'
  if (status === 'warning') return 'warning'
  if (status === 'danger') return 'error'
  return 'default'
}

function getTransportLabel(transport?: string) {
  if (!transport) {
    return t('display.empty')
  }
  return transportLabelMap[transport as keyof typeof transportLabelMap] ?? transport
}

function joinTransportLabels(transports?: readonly string[]) {
  if (!transports?.length) {
    return t('display.empty')
  }
  return transports.map((transport) => getTransportLabel(transport)).join(' / ')
}

const configuredTransportsText = computed(() => joinTransportLabels(snapshot.value?.configured_transports))
const activeTransportText = computed(() => joinTransportLabels(snapshot.value?.active_transports))
const transportStatusItems = computed(() => (
  snapshot.value?.transport_status.map((item) => ({
    ...item,
    label: getTransportLabel(item.transport),
    stateLabel: getAdapterStateLabel(item.state),
    stateType: getStatusType(item.state),
    endpointText: item.endpoint || t('display.empty'),
  })) ?? []
))
const transportIssues = computed(() => snapshot.value?.recent_transport_issues ?? [])
const protocolWorkbenchActions = computed(() => buildProtocolWorkbenchActions(snapshot.value))

function formatProvider(provider?: string) {
  switch (provider) {
    case 'standard':
      return 'OneBot11'
    case 'napcat':
      return 'NapCat'
    case 'luckylillia':
      return 'LuckyLillia'
    default:
      return t('protocols.unknownValue')
  }
}

function runtimeInfoItems(type: string) {
  const live = getLiveTransport(type)
  if (!live) {
    return []
  }
  if (!live.provider && !live.app_name && !live.protocol_version && !live.app_version) {
    return []
  }
  return [
    formatProvider(live.provider),
    live.app_name,
    live.protocol_version,
    live.app_version,
  ].filter((value): value is string => Boolean(value && value.trim()))
}

function loginInfoText(type: string) {
  const live = getLiveTransport(type)
  if (!live) {
    return t('display.empty')
  }
  const parts = [live.user_id, live.nickname].filter((value): value is string => Boolean(value && value.trim()))
  return parts.length > 0 ? parts.join(' / ') : t('protocols.unknownValue')
}

function getIssueTagColor(severity?: string) {
  if (severity === 'error') return 'error'
  if (severity === 'info') return 'processing'
  return 'warning'
}

async function loadPage() {
  try {
    await Promise.all([
      configStore.fetchConfig(),
      protocolsStore.refresh(),
    ])
  } catch {
    // store error state drives the page
  }
}

onMounted(() => {
  void loadPage()
})

useToastFeedback(feedbackToast)

// Unified Transports Layout data structure
const transportConfigs = computed(() => [
  {
    key: 'reverse_ws',
    type: 'reverse_ws',
    name: t('config.sections.onebotReverseWs'),
    description: 'OneBot11 reverse_ws',
    enabledPath: 'onebot.reverse_ws.enabled',
    urlPath: 'onebot.reverse_ws.url',
    tokenPath: 'onebot.reverse_ws.access_token',
    urlHint: t('config.hints.onebotOptional'),
    urlLabel: t('config.fields.onebotReverseWsUrl'),
    tokenLabel: t('config.fields.onebotAccessToken'),
  },
  {
    key: 'forward_ws',
    type: 'forward_ws',
    name: t('config.sections.onebotForwardWs'),
    description: 'OneBot11 forward_ws',
    enabledPath: 'onebot.forward_ws.enabled',
    urlPath: 'onebot.forward_ws.url',
    tokenPath: 'onebot.forward_ws.access_token',
    urlHint: t('config.hints.onebotForwardWs'),
    urlLabel: t('config.fields.onebotForwardWsUrl'),
    tokenLabel: t('config.fields.onebotAccessToken'),
  },
  {
    key: 'http_api',
    type: 'http_api',
    name: t('config.sections.onebotHttpApi'),
    description: 'OneBot11 http_api',
    enabledPath: 'onebot.http_api.enabled',
    urlPath: 'onebot.http_api.url',
    tokenPath: 'onebot.http_api.access_token',
    urlHint: t('config.hints.onebotHttpTransport'),
    urlLabel: t('config.fields.onebotHttpApiUrl'),
    tokenLabel: t('config.fields.onebotAccessToken'),
  },
  {
    key: 'webhook',
    type: 'webhook',
    name: t('config.sections.onebotWebhook'),
    description: 'OneBot11 webhook',
    enabledPath: 'onebot.webhook.enabled',
    urlPath: 'onebot.webhook.url',
    tokenPath: 'onebot.webhook.access_token',
    urlHint: t('config.hints.onebotHttpTransport'),
    urlLabel: t('config.fields.onebotWebhookUrl'),
    tokenLabel: t('config.fields.onebotAccessToken'),
  },
])

function getLiveTransport(type: string) {
  return transportStatusItems.value.find((item) => item.transport === type)
}

function getLiveTransportIssues(type: string) {
  return transportIssues.value.filter((issue) => {
    const code = issue.code.toLowerCase()
    const summary = issue.summary.toLowerCase()
    return code.includes(type) || summary.includes(type) || 
           (type === 'reverse_ws' && code.includes('reverse')) ||
           (type === 'forward_ws' && code.includes('forward'))
  })
}

// Columns for unified Table
const tableColumns = computed(() => [
  { title: t('protocols.transportStatusTitle'), key: 'name', width: 220 },
  { title: t('display.empty'), key: 'enabled', width: 80 },
  { title: t('protocols.activeTransportLabel'), key: 'status', width: 160 },
  { title: t('protocols.runtimeInfoColumn'), key: 'runtime', width: 260 },
  { title: t('protocols.loginInfoColumn'), key: 'login', width: 220 },
  { title: t('config.fields.onebotReverseWsUrl'), key: 'url', width: 320 },
  { title: t('config.fields.onebotAccessToken'), key: 'token', width: 220 },
])

const adapterConfigFields = computed(() => {
  const adapterSec = configSections.value.find(s => s.key === 'adapter')
  return adapterSec?.fields ?? []
})
</script>

<template>
  <AppPage :title="t('protocols.title')" :description="t('protocols.subtitle')">
    <template #extra>
      <div class="table-actions">
        <a-button type="primary" :disabled="!canSave" :loading="saving" @click="save" class="save-action-btn">
          <template #icon>
            <span class="btn-icon">✓</span>
          </template>
          {{ t('protocols.save') }}
        </a-button>
      </div>
    </template>

    <div class="protocol-settings-page">
      <div class="summary-status-strip">
        <div class="strip-item">
          <span class="strip-label">{{ t('protocols.overviewTitle') }}</span>
          <div class="strip-value-wrap">
            <span class="strip-value font-semibold">{{ ONEBOT11_PROTOCOL_NAME }}</span>
            <ManagementContextActions :actions="protocolWorkbenchActions" class="compact-actions" />
          </div>
        </div>

        <div class="strip-divider"></div>

        <div class="strip-item">
          <span class="strip-label">{{ t('protocols.activeTransportLabel') }}</span>
          <div class="strip-value-wrap">
            <span class="strip-value font-bold transport-count">{{ snapshot?.active_transports.length || 0 }}</span>
            <span class="strip-subtext">/ {{ snapshot?.configured_transports.length || 0 }} {{ t('config.fieldCount') }}</span>
          </div>
        </div>

        <div class="strip-divider"></div>

        <div class="strip-item">
          <span class="strip-label">{{ t('protocols.healthLabel') }}</span>
          <div class="strip-value-wrap">
            <span class="status-indicator" :class="protocolStatusType" aria-hidden="true"></span>
            <span class="strip-value" :class="`text-${protocolStatusType}`">{{ readinessLabel }}</span>
          </div>
          <span class="strip-subtext truncate" :title="protocolSummary" style="color: var(--app-text-secondary);">{{ protocolSummary }}</span>
        </div>
      </div>

      <div v-if="transportIssues.length > 0" class="premium-diagnostics-card" data-testid="protocol-issues" role="alert">
        <div class="diagnostics-header">
          <div class="diagnostics-title-wrap">
            <span class="diagnostics-alert-icon">⚠️</span>
            <span class="diagnostics-badge">{{ t('protocols.diagnosticsTitle') }}</span>
          </div>
          <span class="diagnostics-subtitle">{{ t('protocols.diagnosticsSubtitle') }}</span>
        </div>
        <div class="diagnostics-list">
          <div v-for="issue in transportIssues" :key="`${issue.code}-${issue.summary}`" class="diagnostics-item">
            <div class="diagnostics-meta">
              <a-tag :color="getIssueTagColor(issue.severity)" class="diagnostics-tag">
                {{ issue.code }}
              </a-tag>
              <span class="diagnostics-time">{{ t('protocols.diagnosticsLatest') }}</span>
            </div>
            <p class="diagnostics-summary">{{ issue.summary }}</p>
          </div>
        </div>
      </div>

      <div class="unified-workspace-card">
        <div class="workspace-header">
          <div class="title-area">
            <h2 class="workspace-title">{{ t('protocols.workspaceTitle') }}</h2>
            <p class="workspace-subtitle">{{ t('protocols.workspaceSubtitle') }}</p>
          </div>
          <div v-if="restartRequired !== null" class="restart-indicator">
            <a-tag :color="restartRequired ? 'warning' : 'success'" class="restart-status-tag">
              {{ restartRequired ? t('config.restartNeeded') : t('config.hotApplied') }}
            </a-tag>
          </div>
        </div>

        <div class="workspace-body">
          <RetryPanel
            v-if="pageError && !draft"
            :title="t('protocols.connectionSettings')"
            :description="pageError"
            :loading="configLoading"
            @retry="loadPage"
          />

          <!-- The Integrated Table -->
          <div v-else class="table-container">
            <a-table
              class="integrated-protocol-table"
              :columns="tableColumns"
              :data-source="transportConfigs"
              :pagination="false"
              :row-key="(row) => row.key"
              :scroll="{ x: 1420 }"
            >
              <template #bodyCell="{ column, record }">
                <!-- Column 1: Transport Name & Icon -->
                <template v-if="column.key === 'name'">
                  <div class="channel-identity">
                    <div class="channel-avatar" :class="record.type">
                      <span class="avatar-letter">{{ record.type.slice(0, 2).toUpperCase() }}</span>
                    </div>
                    <div class="channel-meta">
                      <span class="channel-name">{{ record.name }}</span>
                      <span class="channel-desc">{{ record.description }}</span>
                    </div>
                  </div>
                </template>

                <!-- Column 2: Enable Switch -->
                <template v-else-if="column.key === 'enabled'">
                  <div class="switch-cell">
                    <a-switch
                      :checked="Boolean(readField(record.enabledPath, 'boolean'))"
                      :aria-label="record.name"
                      @update:checked="(value) => writeField(record.enabledPath, 'boolean', value)"
                    />
                  </div>
                </template>

                <!-- Column 3: Live Running Status Badge + Warnings -->
                <template v-else-if="column.key === 'status'">
                  <div class="status-cell">
                    <template v-if="getLiveTransport(record.type)">
                      <div class="status-badge-wrap">
                        <span class="status-indicator-dot" :class="getLiveTransport(record.type)?.stateType"></span>
                        <a-tag :color="getStatusTagColor(getLiveTransport(record.type)?.stateType)" class="state-tag">
                          {{ getLiveTransport(record.type)?.stateLabel }}
                        </a-tag>
                      </div>
                      <span class="status-summary">{{ getLiveTransport(record.type)?.summary }}</span>
                    </template>
                    <template v-else>
                      <a-tag class="state-tag default-tag">{{ t('protocols.inactiveTransport') }}</a-tag>
                      <span class="status-summary text-muted">{{ t('protocols.unloadedTransport') }}</span>
                    </template>
                  </div>
                </template>

                <!-- Column 4: Runtime endpoint metadata -->
                <template v-else-if="column.key === 'runtime'">
                  <div class="runtime-cell">
                    <template v-if="runtimeInfoItems(record.type).length > 0">
                      <a-tag class="provider-runtime-tag">
                        {{ runtimeInfoItems(record.type)[0] }}
                      </a-tag>
                      <span class="runtime-meta">{{ runtimeInfoItems(record.type).slice(1).join(' / ') || t('protocols.unknownValue') }}</span>
                    </template>
                    <template v-else>
                      <span class="text-muted">{{ t('display.empty') }}</span>
                    </template>
                  </div>
                </template>

                <!-- Column 5: Login identity -->
                <template v-else-if="column.key === 'login'">
                  <span class="login-cell">{{ loginInfoText(record.type) }}</span>
                </template>

                <!-- Column 6: Connection URL Input -->
                <template v-else-if="column.key === 'url'">
                  <div class="input-cell-wrap">
                    <a-input
                      :value="String(readField(record.urlPath, 'text') ?? '')"
                      :placeholder="record.urlHint"
                      :aria-label="record.urlLabel"
                      class="refined-table-input"
                      @update:value="(value) => writeField(record.urlPath, 'text', value)"
                    />
                    
                    <div v-if="getLiveTransportIssues(record.type).length > 0" class="inline-error-alert">
                      <span class="inline-err-icon">⚠️</span>
                      <span class="inline-err-msg" :title="getLiveTransportIssues(record.type)[0].summary">
                        {{ getLiveTransportIssues(record.type)[0].summary }}
                      </span>
                    </div>
                  </div>
                </template>

                <!-- Column 7: Access Token Password Input -->
                <template v-else-if="column.key === 'token'">
                  <div class="input-cell-wrap">
                    <a-input-password
                      :value="String(readField(record.tokenPath, 'text') ?? '')"
                      placeholder="Access Token"
                      :aria-label="record.tokenLabel"
                      class="refined-table-input"
                      @update:value="(value) => writeField(record.tokenPath, 'text', value)"
                    />
                  </div>
                </template>
              </template>
            </a-table>
          </div>
        </div>
      </div>

      <!-- Collapsible advanced settings (Global & reconnection params) -->
      <div v-if="draft" class="advanced-settings-zone">
        <div class="advanced-toggle-bar" @click="advancedExpanded = !advancedExpanded">
          <div class="toggle-left">
            <span class="toggle-icon" :class="{ 'is-active': advancedExpanded }">⚙</span>
            <span class="toggle-title">{{ t('protocols.advancedSettingsTitle') }}</span>
          </div>
          <span class="toggle-hint">{{ advancedExpanded ? t('protocols.collapseSettings') : t('protocols.expandSettings') }}</span>
        </div>

        <transition name="collapse-fade">
          <div v-show="advancedExpanded" class="advanced-content-panel">
            <a-card :bordered="false" class="advanced-card">
              <a-form layout="vertical" class="advanced-form-grid">
                <!-- Adapter Parameter inputs -->
                <div v-for="field in adapterConfigFields" :key="field.path" class="grid-item">
                  <a-form-item class="advanced-form-item">
                    <template #label>
                      <div class="field-label-wrap">
                        <span class="field-label-text">{{ field.label }}</span>
                        <a-tooltip v-if="field.description" :title="field.description">
                          <span class="field-info-icon">?</span>
                        </a-tooltip>
                      </div>
                    </template>

                    <a-input-number
                      v-if="field.type === 'number'"
                      :value="typeof readField(field.path, field.type) === 'number' ? readField(field.path, field.type) : null"
                      :aria-label="field.label"
                      :min="0"
                      :step="1"
                      class="refined-number-input"
                      @update:value="(value) => writeField(field.path, field.type, value)"
                    />

                    <a-input
                      v-else
                      :value="String(readField(field.path, field.type) ?? '')"
                      :aria-label="field.label"
                      class="refined-input"
                      @update:value="(value) => writeField(field.path, field.type, value)"
                    />
                  </a-form-item>
                </div>
              </a-form>
            </a-card>
          </div>
        </transition>
      </div>
    </div>
  </AppPage>
</template>

<style lang="scss" scoped>
.protocol-settings-page {
  --success-rgb: 34, 197, 94;
  --warning-rgb: 234, 179, 8;
  --danger-rgb: 239, 68, 68;
  --font-mono: "Cascadia Mono", "Consolas", monospace;

  display: flex;
  flex-direction: column;
  gap: 20px;
  color: var(--app-text);
  padding: 4px;
}

.summary-status-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 24px;
  background: var(--surface-strong);
  border: 1px solid var(--app-border);
  border-radius: var(--radius-xl, 16px);
  box-shadow: var(--shadow-xs);
  gap: 16px;
}

.strip-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.strip-divider {
  width: 1px;
  height: 36px;
  background: var(--app-border);
  opacity: 0.5;
}

.strip-label {
  font-size: 0.75rem;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--app-text-secondary);
}

.strip-value-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}

.strip-value {
  font-size: 1.15rem;
  color: var(--app-text);
  line-height: 1.25;

  &.text-success { color: var(--app-success); }
  &.text-danger { color: var(--app-danger); }
  &.text-warning { color: var(--app-warning); }
}

.strip-subtext {
  font-size: 0.78rem;
  color: var(--app-text-secondary);
  align-self: flex-start;
}

.status-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: color-mix(in srgb, var(--app-border) 70%, var(--accent) 30%);
  display: inline-block;

  &.success {
    background: var(--app-success);
  }
  &.danger {
    background: var(--app-danger);
  }
  &.warning {
    background: var(--app-warning);
  }
}

.provider-tag {
  border-radius: 6px;
  font-weight: 600;
  padding: 2px 10px;
  box-shadow: 0 2px 8px rgba(124, 58, 237, 0.08);
}

.compact-actions {
  display: inline-flex;
  margin-left: 4px;
}

.unified-workspace-card {
  background: var(--app-bg-card, #ffffff);
  border: 1px solid var(--app-border);
  border-radius: var(--radius-xl, 16px);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.02);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.workspace-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 24px;
  border-bottom: 1px solid var(--app-border);
  background: linear-gradient(to right, var(--surface-soft), transparent);
}

.title-area {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.workspace-title {
  font-size: 1.15rem;
  font-weight: 700;
  color: var(--app-text);
  margin: 0;
  letter-spacing: -0.01em;
}

.workspace-subtitle {
  font-size: 0.8rem;
  color: var(--app-text-secondary);
  margin: 0;
}

.workspace-body {
  padding: 0;
}

.table-container {
  overflow: hidden;
  border-radius: 0 0 var(--radius-xl, 16px) var(--radius-xl, 16px);
}

.integrated-protocol-table {
  :deep(.ant-table-thead > tr > th) {
    background: var(--surface-soft, #f9fafb);
    font-weight: 600;
    color: var(--app-text-secondary);
    border-bottom: 1px solid var(--app-border);
    padding: 14px 20px;
    font-size: 0.82rem;
  }

  :deep(.ant-table-tbody > tr > td) {
    border-bottom: 1px solid var(--app-border);
    padding: 16px 20px;
    vertical-align: middle;
    transition: background-color 150ms ease;
  }

  :deep(.ant-table-row:hover > td) {
    background: linear-gradient(90deg, color-mix(in srgb, var(--accent) 3%, transparent), transparent) !important;
  }
}

/* Row elements styling */
.channel-identity {
  display: flex;
  align-items: center;
  gap: 14px;
}

.channel-avatar {
  width: 38px;
  height: 38px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 800;
  font-size: 0.72rem;
  color: #ffffff;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  transition: background-color 150ms ease, border-color 150ms ease;

  &.reverse_ws {
    background: linear-gradient(135deg, #3b82f6, #1d4ed8);
  }
  &.forward_ws {
    background: linear-gradient(135deg, #10b981, #047857);
  }
  &.http_api {
    background: linear-gradient(135deg, #8b5cf6, #5b21b6);
  }
  &.webhook {
    background: linear-gradient(135deg, #f59e0b, #b45309);
  }
}

.channel-meta {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.channel-name {
  font-weight: 600;
  font-size: 0.92rem;
  color: var(--app-text);
}

.channel-desc {
  font-size: 0.75rem;
  color: var(--app-text-secondary);
}

.switch-cell {
  display: flex;
  align-items: center;
}

.status-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  align-items: flex-start;
}

.status-badge-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-indicator-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--app-border);

  &.success { background: var(--app-success); }
  &.danger { background: var(--app-danger); }
  &.warning { background: var(--app-warning); }
}

.state-tag {
  border-radius: 4px;
  font-size: 0.78rem;
  font-weight: 500;
  border: none;
  padding: 0 6px;

  &.default-tag {
    background: var(--surface-soft);
    color: var(--app-text-secondary);
  }
}

.status-summary {
  font-size: 0.74rem;
  color: var(--app-text-secondary);
  max-width: 140px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.input-cell-wrap {
  display: flex;
  flex-direction: column;
  gap: 6px;
  position: relative;
}

:deep(.refined-table-input.ant-input),
:deep(.refined-table-input.ant-input-affix-wrapper) {
  border-radius: 8px;
  background: var(--surface-soft, #f9fafb);
  border: 1px solid var(--app-border);
  box-shadow: none;
  font-size: 0.85rem;
  padding: 6px 12px;
  transition: border-color 150ms ease, background-color 150ms ease;

  &:hover {
    border-color: color-mix(in srgb, var(--accent) 30%, var(--app-border));
  }

  &.ant-input-affix-wrapper-focused,
  &:focus {
    border-color: var(--accent);
    background: #ffffff;
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 15%, transparent);
  }
}

.inline-error-alert {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  background: color-mix(in srgb, var(--app-danger) 6%, transparent);
  border-radius: 6px;
  border: 1px solid color-mix(in srgb, var(--app-danger) 15%, transparent);
  max-width: 320px;
}

.inline-err-icon {
  font-size: 0.78rem;
  line-height: 1;
}

.inline-err-msg {
  font-size: 0.74rem;
  color: var(--app-danger);
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.advanced-settings-zone {
  border: 1px solid var(--app-border);
  border-radius: var(--radius-xl, 16px);
  background: var(--app-bg-card, #ffffff);
  overflow: hidden;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.01);
}

.advanced-toggle-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  background: var(--surface-soft);
  cursor: pointer;
  user-select: none;
  transition: background-color 150ms ease;

  &:hover {
    background: color-mix(in srgb, var(--accent) 2%, var(--surface-soft));
  }
}

.toggle-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.toggle-icon {
  font-size: 1rem;
  color: var(--app-text-secondary);
  transition: transform 150ms ease;

  &.is-active {
    transform: rotate(90deg);
    color: var(--accent);
  }
}

.toggle-title {
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--app-text);
  letter-spacing: -0.01em;
}

.toggle-hint {
  font-size: 0.76rem;
  color: var(--app-text-secondary);
  font-weight: 500;
}

.advanced-content-panel {
  border-top: 1px solid var(--app-border);
  background: #ffffff;
}

.advanced-card {
  :deep(.ant-card-body) {
    padding: 24px;
  }
}

.advanced-form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 20px 24px;

  .span-full {
    grid-column: 1 / -1;
  }
}

.advanced-form-item {
  margin-bottom: 0;
}

.field-label-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 2px;
}

.field-label-text {
  font-weight: 600;
  font-size: 0.82rem;
  color: var(--app-text);
}

.field-info-icon {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: var(--surface-soft);
  color: var(--app-text-secondary);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 0.72rem;
  cursor: help;
  font-weight: bold;
  transition: background-color 150ms ease, color 150ms ease;

  &:hover {
    background: var(--accent);
    color: #ffffff;
  }
}

:deep(.refined-select-input.ant-select) {
  width: 100%;

  .ant-select-selector {
    border-radius: 8px;
    background: var(--surface-soft);
    border-color: var(--app-border);
    box-shadow: none;
    padding: 4px 12px;
    height: auto;
  }

  &.ant-select-focused .ant-select-selector,
  &:hover .ant-select-selector {
    border-color: var(--accent);
  }
}

:deep(.refined-number-input.ant-input-number) {
  width: 100%;
  border-radius: 8px;
  background: var(--surface-soft);
  border-color: var(--app-border);
  box-shadow: none;
  padding: 2px 4px;

  &:hover {
    border-color: color-mix(in srgb, var(--accent) 30%, var(--app-border));
  }

  &.ant-input-number-focused {
    border-color: var(--accent);
    background: #ffffff;
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 15%, transparent);
  }
}

.collapse-fade-enter-active,
.collapse-fade-leave-active {
  transition: max-height 150ms ease, opacity 150ms linear;
  max-height: 500px;
  overflow: hidden;
}

.collapse-fade-enter-from,
.collapse-fade-leave-to {
  max-height: 0;
  opacity: 0;
}

.btn-icon {
  margin-right: 4px;
  font-weight: bold;
}

.transport-count {
  color: var(--accent);
}

@media (max-width: 1024px) {
  .summary-status-strip {
    flex-wrap: wrap;
    gap: 20px;
  }

  .strip-divider {
    display: none;
  }

  .strip-item {
    min-width: 40%;
  }
}

@media (max-width: 640px) {
  .summary-status-strip {
    padding: 14px 18px;
  }

  .strip-item {
    min-width: 100%;
  }

  .workspace-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }
}

.premium-diagnostics-card {
  padding: 16px 24px;
  background: linear-gradient(135deg, color-mix(in srgb, var(--app-danger) 6%, #ffffff), color-mix(in srgb, var(--app-danger) 2%, #ffffff));
  border: 1px solid color-mix(in srgb, var(--app-danger) 15%, var(--app-border));
  border-radius: var(--radius-xl, 16px);
  box-shadow: 0 4px 20px rgba(239, 68, 68, 0.03);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.diagnostics-header {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.diagnostics-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}

.diagnostics-alert-icon {
  font-size: 1.1rem;
}

.diagnostics-badge {
  font-weight: 700;
  font-size: 0.95rem;
  color: var(--app-danger);
}

.diagnostics-subtitle {
  font-size: 0.78rem;
  color: var(--app-text-secondary);
}

.diagnostics-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.diagnostics-item {
  background: var(--surface-strong);
  border: 1px solid rgba(239, 68, 68, 0.08);
  border-radius: 8px;
  padding: 10px 14px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  transition: border-color 150ms ease, background-color 150ms ease;

  &:hover {
    background: #ffffff;
    border-color: rgba(239, 68, 68, 0.15);
  }
}

.diagnostics-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.diagnostics-tag {
  font-family: var(--font-mono);
  font-size: 0.76rem;
  border-radius: 4px;
  padding: 1px 6px;
}

.diagnostics-time {
  font-size: 0.72rem;
  color: var(--app-text-secondary);
}

.diagnostics-summary {
  font-size: 0.82rem;
  color: var(--app-text);
  margin: 0;
  line-height: 1.4;
  font-weight: 500;
}
</style>
