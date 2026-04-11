<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'

import { notifySuccess } from '@/adapter/feedback'
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
import { ONEBOT11_PROTOCOL_NAME } from '@/lib/protocols'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'
import { useProtocolsStore } from '@/stores/protocols'
import type { ConfigDocument } from '@/types/api'

const router = useRouter()
const configStore = useConfigStore()
const protocolsStore = useProtocolsStore()

const {
  document,
  error: configError,
  loading: configLoading,
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
    return type === 'boolean' ? false : ''
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
    normalized = Number(value)
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
  notifySuccess(response.restart_required ? t('config.saveRestart') : t('config.saveSuccess'))
}
</script>

<template>
  <div class="page-grid minimal-protocol-theme">
    <section class="hero-panel">
      <div class="hero-text">
        <h1 class="main-title">{{ t('protocols.title') }}</h1>
        <p class="subtitle">{{ t('protocols.subtitle') }}</p>
      </div>

      <div class="hero-actions">
        <button class="minimal-btn outline" @click="router.push('/protocols/logs')">
          {{ t('protocols.openLogs') }}
        </button>
        <button class="minimal-btn outline" :disabled="pageLoading" @click="loadPage">
          {{ t('dashboard.refresh') }}
        </button>
        <button class="minimal-btn primary" :disabled="!canSave || saving" @click="save">
          <span v-if="saving">{{ t('protocols.save') }}...</span>
          <span v-else>{{ t('protocols.save') }}</span>
        </button>
      </div>
    </section>

    <!-- V2 Dashboard Metrics Grid -->
    <div class="dashboard-metrics-grid">
      <div class="minimal-card metric-card">
        <div class="metric-header">
          <span class="mono-label">{{ t('protocols.overviewTitle') }}</span>
          <span class="minimal-badge">{{ ONEBOT11_PROTOCOL_NAME }}</span>
        </div>
        <div class="metric-body">
          <div class="status-indicator-wrap">
            <div class="status-indicator-ring" :class="protocolStatusType"></div>
            <div class="status-indicator-label" :class="`text-${protocolStatusType}`">{{ protocolStatusLabel }}</div>
          </div>
          <div class="status-summary-value">{{ protocolSummary }}</div>
        </div>
      </div>

      <div class="minimal-card metric-card">
        <div class="metric-header">
          <span class="mono-label">{{ t('protocols.providerLabel') }}</span>
          <span class="minimal-badge" :class="readinessType">{{ readinessLabel }}</span>
        </div>
        <div class="metric-body centered-metric">
          <div class="metric-big-value">{{ snapshot?.provider || t('display.empty') }}</div>
        </div>
      </div>

      <div class="minimal-card metric-card">
        <div class="metric-header">
          <span class="mono-label">Transports</span>
        </div>
        <div class="metric-body transport-counts">
          <div class="transport-count">
            <div class="count-value">{{ snapshot?.configured_transports.length || 0 }}</div>
            <div class="count-label">{{ t('protocols.configuredTransportLabel') }}</div>
          </div>
          <div class="transport-count-divider"></div>
          <div class="transport-count active">
            <div class="count-value text-success">{{ snapshot?.active_transports.length || 0 }}</div>
            <div class="count-label">{{ t('protocols.activeTransportLabel') }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- V2 Transport Cards Grid -->
    <div class="transport-cards-section">
      <div class="section-heading">
        <div>
          <h2>{{ t('protocols.transportStatusTitle') }}</h2>
          <p class="subtitle">{{ t('protocols.transportStatusHint') }}</p>
        </div>
      </div>
      <div class="transport-cards-grid">
        <div
          v-for="item in transportStatusItems"
          :key="item.transport"
          class="minimal-card transport-card"
        >
          <div class="transport-card-header">
            <div class="transport-identity">
              <span class="transport-line-dot" :class="item.stateType"></span>
              <strong>{{ item.label }}</strong>
            </div>
            <span class="minimal-badge" :class="item.stateType">{{ item.stateLabel }}</span>
          </div>
          <div class="transport-card-body">
            <div class="transport-endpoint">
              <code class="endpoint-code">{{ item.endpointText }}</code>
            </div>
            <div class="transport-summary-text">{{ item.summary }}</div>
          </div>
        </div>
      </div>
    </div>

    <div class="config-alerts-container" v-if="pageError || redactedFields.length > 0">
      <a-alert v-if="pageError" :message="t('errors.common.actionFailed')" type="error" :description="pageError" show-icon />
      <a-alert
        v-if="redactedFields.length > 0"
        :message="t('config.redactedTitle')"
        type="info"
        :description="redactedFields.join(', ')"
        show-icon
      />
    </div>

    <RetryPanel
      v-if="pageError && !draft"
      :title="t('protocols.connectionSettings')"
      :description="pageError"
      :loading="configLoading"
      @retry="loadPage"
    />

    <section v-else class="protocol-settings-section">
      <div class="section-heading">
        <div>
          <h2>{{ t('protocols.connectionSettings') }}</h2>
          <p class="subtitle">{{ t('protocols.connectionSettingsHint') }}</p>
        </div>
        <div v-if="restartRequired !== null" class="restart-indicator">
          <span class="minimal-badge" :class="restartRequired ? 'warning' : 'success'">
            {{ restartRequired ? t('config.restartNeeded') : t('config.hotApplied') }}
          </span>
        </div>
      </div>

      <div v-if="draft" class="protocol-settings-layout">
        <div v-for="section in configSections" :key="section.key" class="minimal-card protocol-config-card">
          <div class="card-header config-card-header">
            <strong>{{ section.title }}</strong>
            <span class="field-count-badge">{{ section.fields.length }} {{ t('config.fieldCount') }}</span>
          </div>

          <a-form layout="vertical" class="protocol-settings-form">
            <div v-for="field in section.fields" :key="field.path" class="config-field-item">
              <a-form-item>
                <template #label>
                  <div class="field-label-wrap">
                    <span class="field-label-text">{{ field.label }}</span>
                    <a-tooltip v-if="field.description" :title="field.description">
                      <span class="field-info-icon">?</span>
                    </a-tooltip>
                  </div>
                </template>

                <a-input
                  v-if="field.type === 'text'"
                  :value="String(readField(field.path, field.type) ?? '')"
                  :aria-label="field.label"
                  class="refined-input"
                  @update:value="(value) => writeField(field.path, field.type, value)"
                />

                <a-input-number
                  v-else-if="field.type === 'number'"
                  :value="Number(readField(field.path, field.type) ?? 0)"
                  :aria-label="field.label"
                  :min="0"
                  :step="1"
                  class="refined-number-input"
                  @update:value="(value) => writeField(field.path, field.type, value ?? 0)"
                />

                <div v-else-if="field.type === 'boolean'" class="switch-wrap">
                  <a-switch
                    :checked="Boolean(readField(field.path, field.type))"
                    :aria-label="field.label"
                    @update:checked="(value) => writeField(field.path, field.type, value)"
                  />
                </div>

                <a-select
                  v-else-if="field.type === 'select'"
                  :value="String(readField(field.path, field.type) ?? '')"
                  :aria-label="field.label"
                  class="refined-input"
                  :options="field.options"
                  @update:value="(value) => writeField(field.path, field.type, value)"
                />

                <a-textarea
                  v-else
                  :value="String(readField(field.path, field.type) ?? '')"
                  :aria-label="field.label"
                  :auto-size="{ minRows: 4, maxRows: 8 }"
                  class="refined-input"
                  @update:value="(value) => writeField(field.path, field.type, value)"
                />
              </a-form-item>
            </div>
          </a-form>
        </div>
      </div>
    </section>
  </div>
</template>

<style lang="scss" scoped>

.dashboard-metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: var(--space-lg);
  margin-bottom: var(--space-2xl);
}

.metric-card {
  padding: var(--space-lg);
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
  min-height: 160px;
}

.metric-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.metric-body {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
  flex: 1;
  justify-content: center;
}

.centered-metric {
  align-items: center;
  text-align: center;
}

.metric-big-value {
  font-size: 2rem;
  font-weight: 800;
  font-family: var(--font-sans);
  color: var(--theme-text);
  letter-spacing: -0.02em;
}

.status-indicator-wrap {
  display: flex;
  align-items: center;
  gap: var(--space-md);
}

.status-indicator-ring {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  position: relative;
  background: var(--theme-border-strong);
  
  &.success { background: var(--theme-success); box-shadow: 0 0 0 4px oklch(70% 0.15 150 / 15%); }
  &.danger { background: var(--theme-danger); box-shadow: 0 0 0 4px oklch(65% 0.18 25 / 15%); }
  &.warning { background: var(--theme-warning); box-shadow: 0 0 0 4px oklch(75% 0.15 70 / 15%); }
}

.status-indicator-label {
  font-size: 1.5rem;
  font-weight: 700;
  letter-spacing: -0.02em;
  font-family: var(--font-sans);
  
  &.text-success { color: var(--theme-success); }
  &.text-danger { color: var(--theme-danger); }
  &.text-warning { color: var(--theme-warning); }
}

.status-summary-value {
  font-size: 1rem;
  font-weight: 600;
  line-height: 1.5;
  margin-top: var(--space-xs);
  color: var(--theme-text-muted);
}

.transport-counts {
  flex-direction: row;
  justify-content: space-around;
  align-items: center;
}

.transport-count {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-xs);
  text-align: center;
}

.transport-count-divider {
  width: 1px;
  height: 40px;
  background: var(--theme-border);
}

.count-value {
  font-size: 2rem;
  font-weight: 800;
  font-family: var(--font-sans);
  line-height: 1;
  color: var(--theme-text);
  
  &.text-success {
    color: var(--theme-success);
  }
}

.count-label {
  font-size: 0.75rem;
  color: var(--theme-text-muted);
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.transport-cards-section {
  margin-bottom: var(--space-2xl);
}

.transport-cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(380px, 1fr));
  gap: var(--space-md);
}

.transport-card {
  padding: var(--space-md) var(--space-lg);
  gap: var(--space-md);
}

.transport-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px dashed var(--theme-border);
  padding-bottom: var(--space-sm);
}

.transport-identity {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  font-weight: 600;
  font-size: 0.95rem;
  font-family: var(--font-sans);
}

.transport-line-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--theme-border-strong);
  
  &.success { background: var(--theme-success); }
  &.danger { background: var(--theme-danger); }
  &.warning { background: var(--theme-warning); }
}

.transport-card-body {
  display: grid;
  gap: var(--space-xs);
}

.endpoint-code {
  display: inline-block;
  background: var(--theme-surface-soft);
  border: 1px solid var(--theme-border);
  border-radius: 6px;
  padding: 4px 8px;
  font-family: var(--font-mono);
  font-size: 0.85rem;
  color: var(--theme-text-muted);
  word-break: break-all;
}

.transport-summary-text {
  font-size: 0.9rem;
  color: var(--theme-text-muted);
  line-height: 1.4;
  margin-top: var(--space-xs);
}

/* Settings Layout */
.protocol-settings-section {
  margin-top: var(--space-xl);
}

.protocol-settings-layout {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(480px, 1fr));
  gap: var(--space-lg);
}

.protocol-config-card {
  background: var(--theme-surface);
}

.config-card-header {
  background: var(--theme-surface-soft);
  border-bottom: 1px solid var(--theme-border);
}

.protocol-settings-form {
  padding: var(--space-lg);
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: var(--space-lg);
}

.field-label-wrap {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
}

.field-label-text {
  font-weight: 600;
  font-size: 0.85rem;
  color: var(--theme-text);
  font-family: var(--font-sans);
}

.field-info-icon {
  color: var(--theme-text-muted);
  cursor: help;
  font-family: var(--font-sans);
  font-size: 0.8rem;
  font-weight: bold;
  opacity: 0.7;
  
  &:hover {
    opacity: 1;
    color: var(--theme-accent);
  }
}

.field-count-badge {
  background: transparent;
  color: var(--theme-text-muted);
  font-size: 0.8rem;
  font-weight: 500;
}

@media (max-width: 768px) {
  .dashboard-metrics-grid {
    grid-template-columns: 1fr;
  }
  
  .protocol-settings-layout {
    grid-template-columns: 1fr;
  }
  
  .transport-cards-grid {
    grid-template-columns: 1fr;
  }
}
</style>
