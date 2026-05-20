<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import { notifySuccess } from '@/adapter/feedback'
import ManagementContextActions from '@/components/ManagementContextActions.vue'
import ConfigApplyEffectsSummary from '@/components/config/ConfigApplyEffectsSummary.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import {
  cloneConfig,
  getProtocolConfigSections,
  getValueByPath,
  setValueByPath,
  type ConfigFieldDefinition,
} from '@/lib/config-form'
import { getAdapterStateLabel, getReadinessStatusLabel, getStatusType } from '@/lib/display'
import { fromMultilineList, toMultilineList } from '@/lib/format'
import { buildProtocolWorkbenchActions } from '@/lib/management-links'
import { ONEBOT11_PROTOCOL_NAME } from '@/lib/protocols'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'
import { useProtocolsStore } from '@/stores/protocols'
import type { ConfigDocument } from '@/types/api'

const configStore = useConfigStore()
const protocolsStore = useProtocolsStore()

const {
  document,
  error: configError,
  loading: configLoading,
  applyEffects,
  redactedFields,
  restartRequired,
  saving,
} = storeToRefs(configStore)
const {
  error: protocolsError,
  loading: protocolsLoading,
  snapshot,
} = storeToRefs(protocolsStore)

const draft = ref<ConfigDocument | null>(null)
const advancedExpanded = ref(false)

const configSections = computed(() => getProtocolConfigSections())
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
const pageLoading = computed(() => configLoading.value || protocolsLoading.value)
const protocolSummary = computed(() => snapshot.value?.summary ?? t('display.empty'))
const pageError = computed(() => configError.value || protocolsError.value)
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

function getIssueTagColor(severity?: string) {
  if (severity === 'error') return 'error'
  if (severity === 'info') return 'processing'
  return 'warning'
}

watch(document, (value) => {
  draft.value = value ? cloneConfig(value) : null
}, { immediate: true })

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

function readField(path: string, type: ConfigFieldDefinition['type']) {
  if (!draft.value) {
    if (type === 'boolean') {
      return false
    }

    return type === 'number' ? null : ''
  }

  const current = getValueByPath(draft.value as unknown as Record<string, unknown>, path)
  if (type === 'list') {
    return Array.isArray(current) ? toMultilineList(current as string[]) : ''
  }
  return current
}

function writeField(path: string, type: ConfigFieldDefinition['type'], value: unknown) {
  if (!draft.value) {
    return
  }

  let normalized = value
  if (type === 'number') {
    if (value === null || value === undefined || value === '') {
      normalized = undefined
    } else {
      const nextNumber = Number(value)
      normalized = Number.isFinite(nextNumber) ? nextNumber : undefined
    }
  } else if (type === 'list') {
    normalized = fromMultilineList(String(value))
  }

  setValueByPath(draft.value as unknown as Record<string, unknown>, path, normalized)
}

const canSave = computed(() => Boolean(draft.value) && !saving.value)

async function save() {
  if (!draft.value) {
    return
  }

  const response = await configStore.saveConfig(draft.value)
  try {
    await protocolsStore.refresh()
  } catch {
    // store error state drives the page
  }
  notifySuccess(response.restart_required ? t('config.saveRestart') : t('config.saveSuccess'))
}

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
  { title: t('config.fields.onebotReverseWsUrl'), key: 'url', width: 320 },
  { title: t('config.fields.onebotAccessToken'), key: 'token', width: 220 },
])

const providerConfig = computed(() => {
  const mainSec = configSections.value.find(s => s.key === 'onebot')
  return mainSec?.fields.find(f => f.path === 'onebot.provider')
})

const adapterConfigFields = computed(() => {
  const adapterSec = configSections.value.find(s => s.key === 'adapter')
  return adapterSec?.fields ?? []
})
</script>

<template>
  <AppPage :title="t('protocols.title')" :description="t('protocols.subtitle')">
    <template #extra>
      <div class="table-actions">
        <a-button :loading="pageLoading" @click="loadPage">
          <template #icon>
            <span class="btn-icon">↻</span>
          </template>
          {{ t('dashboard.refresh') }}
        </a-button>
        <a-button type="primary" :disabled="!canSave" :loading="saving" @click="save" class="save-glow-btn">
          <template #icon>
            <span class="btn-icon">✓</span>
          </template>
          {{ t('protocols.save') }}
        </a-button>
      </div>
    </template>

    <div class="protocol-settings-page">
      <!-- Summary status dashboard strip -->
      <div class="summary-status-strip" v-motion="{ initial: { opacity: 0, y: -10 }, enter: { opacity: 1, y: 0 } }">
        <div class="strip-item">
          <span class="strip-label">{{ t('protocols.overviewTitle') }}</span>
          <div class="strip-value-wrap">
            <span class="strip-value font-semibold">{{ ONEBOT11_PROTOCOL_NAME }}</span>
            <ManagementContextActions :actions="protocolWorkbenchActions" class="compact-actions" />
          </div>
        </div>

        <div class="strip-divider"></div>

        <div class="strip-item">
          <span class="strip-label">{{ t('protocols.providerLabel') }}</span>
          <span class="strip-value">
            <a-tag color="purple" class="provider-tag">
              {{ snapshot?.provider || t('display.empty') }}
            </a-tag>
          </span>
        </div>

        <div class="strip-divider"></div>

        <div class="strip-item">
          <span class="strip-label">{{ t('protocols.activeTransportLabel') }}</span>
          <div class="strip-value-wrap">
            <span class="strip-value font-bold text-gradient">{{ snapshot?.active_transports.length || 0 }}</span>
            <span class="strip-subtext">/ {{ snapshot?.configured_transports.length || 0 }} {{ t('config.fieldCount') }}</span>
          </div>
        </div>

        <div class="strip-divider"></div>

        <div class="strip-item">
          <span class="strip-label">{{ t('protocols.healthLabel') }}</span>
          <div class="strip-value-wrap">
            <span class="status-pulse-dot" :class="protocolStatusType"></span>
            <span class="strip-value" :class="`text-${protocolStatusType}`">{{ readinessLabel }}</span>
          </div>
          <span class="strip-subtext truncate" :title="protocolSummary" style="color: var(--app-text-secondary);">{{ protocolSummary }}</span>
        </div>
      </div>

      <!-- Config effects alerts -->
      <div v-if="pageError || applyEffects || redactedFields.length > 0" class="config-alerts-container">
        <a-alert v-if="pageError" :message="t('errors.common.actionFailed')" type="error" :description="pageError" show-icon />
        <a-alert
          v-if="applyEffects"
          :message="t('config.applyEffects.title')"
          :type="restartRequired ? 'warning' : 'success'"
          show-icon
        >
          <template #description>
            <ConfigApplyEffectsSummary :effects="applyEffects" />
          </template>
        </a-alert>
        <a-alert
          v-if="redactedFields.length > 0"
          :message="t('config.redactedTitle')"
          type="info"
          :description="redactedFields.join(', ')"
          show-icon
        />
      </div>

      <!-- Sleek Transmission Exceptions Banner -->
      <div v-if="transportIssues.length > 0" class="premium-diagnostics-card" data-testid="protocol-issues" v-motion="{ initial: { opacity: 0, y: 10 }, enter: { opacity: 1, y: 0 } }">
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

      <!-- Main unified content workspace -->
      <div class="unified-workspace-card" v-motion="{ initial: { opacity: 0, y: 15 }, enter: { opacity: 1, y: 0, transition: { duration: 350 } } }">
        <div class="workspace-header">
          <div class="title-area">
            <h2 class="workspace-title">{{ t('protocols.workspaceTitle') }}</h2>
            <p class="workspace-subtitle">{{ t('protocols.workspaceSubtitle') }}</p>
          </div>
          <div v-if="restartRequired !== null" class="restart-indicator">
            <a-tag :color="restartRequired ? 'warning' : 'success'" class="pulse-tag">
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
              :scroll="{ x: 1040 }"
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

                <!-- Column 4: Connection URL Input -->
                <template v-else-if="column.key === 'url'">
                  <div class="input-cell-wrap">
                    <a-input
                      :value="String(readField(record.urlPath, 'text') ?? '')"
                      :placeholder="record.urlHint"
                      :aria-label="record.urlLabel"
                      class="refined-table-input"
                      @update:value="(value) => writeField(record.urlPath, 'text', value)"
                    />
                    
                    <!-- Integrated Inline Exception alerts -->
                    <div v-if="getLiveTransportIssues(record.type).length > 0" class="inline-error-alert">
                      <span class="inline-err-icon">⚠️</span>
                      <span class="inline-err-msg" :title="getLiveTransportIssues(record.type)[0].summary">
                        {{ getLiveTransportIssues(record.type)[0].summary }}
                      </span>
                    </div>
                  </div>
                </template>

                <!-- Column 5: Access Token Password Input -->
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
                <!-- Provider Select in Advanced Panel -->
                <div v-if="providerConfig" class="grid-item span-full">
                  <a-form-item class="advanced-form-item">
                    <template #label>
                      <div class="field-label-wrap">
                        <span class="field-label-text">{{ providerConfig.label }}</span>
                        <a-tooltip v-if="providerConfig.description" :title="providerConfig.description">
                          <span class="field-info-icon">?</span>
                        </a-tooltip>
                      </div>
                    </template>
                    <a-select
                      :value="String(readField(providerConfig.path, providerConfig.type) ?? '')"
                      :aria-label="providerConfig.label"
                      class="refined-select-input"
                      :options="providerConfig.options"
                      @update:value="(value) => writeField(providerConfig.path, providerConfig.type, value)"
                    />
                  </a-form-item>
                </div>

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
  --primary-rgb: 99, 102, 241;
  --success-rgb: 34, 197, 94;
  --warning-rgb: 234, 179, 8;
  --danger-rgb: 239, 68, 68;
  --glass-bg: rgba(255, 255, 255, 0.45);
  --glass-border: rgba(255, 255, 255, 0.6);
  --font-mono: "Cascadia Mono", "Consolas", monospace;

  display: flex;
  flex-direction: column;
  gap: 20px;
  color: var(--app-text);
  padding: 4px;
}

/* Glassmorphism Summary Dashboard Strip */
.summary-status-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 24px;
  background: var(--glass-bg);
  border: 1px solid var(--glass-border);
  border-radius: var(--radius-xl, 16px);
  backdrop-filter: blur(12px);
  box-shadow: 0 8px 32px 0 rgba(31, 38, 135, 0.05);
  gap: 16px;
  transition: all 0.3s ease;

  &:hover {
    box-shadow: 0 10px 40px 0 rgba(31, 38, 135, 0.08);
    background: rgba(255, 255, 255, 0.55);
  }
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

.status-pulse-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  position: relative;
  background: color-mix(in srgb, var(--app-border) 70%, var(--accent) 30%);
  display: inline-block;

  &::after {
    content: '';
    position: absolute;
    top: -2px;
    left: -2px;
    right: -2px;
    bottom: -2px;
    border-radius: 50%;
    border: 2px solid currentColor;
    opacity: 0;
    animation: ripple 2s infinite ease-out;
  }

  &.success {
    background: var(--app-success);
    color: var(--app-success);
    &::after { opacity: 0.4; }
  }
  &.danger {
    background: var(--app-danger);
    color: var(--app-danger);
    &::after { opacity: 0.4; }
  }
  &.warning {
    background: var(--app-warning);
    color: var(--app-warning);
    &::after { opacity: 0.4; }
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

/* Premium Main Workspace Card */
.unified-workspace-card {
  background: var(--app-bg-card, #ffffff);
  border: 1px solid var(--app-border);
  border-radius: var(--radius-xl, 16px);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.02);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transition: all 0.3s ease;

  &:hover {
    box-shadow: 0 6px 30px rgba(0, 0, 0, 0.04);
  }
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

/* Integrated Table custom styles */
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
    transition: all 0.25s ease;
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
  transition: all 0.3s ease;

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
  transition: all 0.25s ease;

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

/* Integrated Inline Error */
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

/* Collapsible Advanced settings section */
.advanced-settings-zone {
  border: 1px solid var(--app-border);
  border-radius: var(--radius-xl, 16px);
  background: var(--app-bg-card, #ffffff);
  overflow: hidden;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.01);
  transition: all 0.3s ease;

  &:hover {
    box-shadow: 0 6px 25px rgba(0, 0, 0, 0.03);
  }
}

.advanced-toggle-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  background: var(--surface-soft);
  cursor: pointer;
  user-select: none;
  transition: background 0.2s ease;

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
  transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);

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
  transition: all 0.2s ease;

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

/* Animations & Transitions */
.collapse-fade-enter-active,
.collapse-fade-leave-active {
  transition: max-height 0.3s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.2s linear;
  max-height: 500px;
  overflow: hidden;
}

.collapse-fade-enter-from,
.collapse-fade-leave-to {
  max-height: 0;
  opacity: 0;
}

@keyframes ripple {
  0% {
    transform: scale(0.95);
    opacity: 0.5;
  }
  100% {
    transform: scale(2.2);
    opacity: 0;
  }
}

.save-glow-btn {
  box-shadow: 0 4px 14px rgba(var(--primary-rgb), 0.25);
  transition: all 0.25s ease;

  &:hover:not(:disabled) {
    box-shadow: 0 6px 20px rgba(var(--primary-rgb), 0.4);
    transform: translateY(-1px);
  }

  &:active:not(:disabled) {
    transform: translateY(0);
  }
}

.btn-icon {
  margin-right: 4px;
  font-weight: bold;
}

.text-gradient {
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

/* Responsive queries */
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

/* Sleek Diagnostics Banner */
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
  background: rgba(255, 255, 255, 0.6);
  border: 1px solid rgba(239, 68, 68, 0.08);
  border-radius: 8px;
  padding: 10px 14px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  transition: all 0.2s ease;

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
