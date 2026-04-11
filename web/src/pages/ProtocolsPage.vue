<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { storeToRefs } from 'pinia'

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
  ElMessage.success(response.restart_required ? t('config.saveRestart') : t('config.saveSuccess'))
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

    <div class="protocol-overview-layout">
      <div class="minimal-card protocol-main-status">
        <div class="card-header">
          <strong>{{ t('protocols.overviewTitle') }}</strong>
          <span class="minimal-badge">{{ t('protocols.fixedProtocolLabel') }}: {{ ONEBOT11_PROTOCOL_NAME }}</span>
        </div>
        
        <div class="protocol-status-display">
          <div class="status-main-info">
            <div class="status-indicator-wrap">
              <div class="status-indicator-ring" :class="protocolStatusType"></div>
              <div class="status-indicator-label" :class="`text-${protocolStatusType}`">{{ protocolStatusLabel }}</div>
            </div>
            <div class="status-summary-text">
              <small class="mono-label">{{ t('protocols.protocolSummaryLabel') }}</small>
              <div class="status-summary-value">{{ protocolSummary }}</div>
            </div>
          </div>
          
          <div class="status-meta-grid">
            <div class="status-meta-item">
              <small class="mono-label">{{ t('protocols.providerLabel') }}</small>
              <strong class="mono-value">{{ snapshot?.provider || t('display.empty') }}</strong>
            </div>
            <div class="status-meta-item">
              <small class="mono-label">{{ t('protocols.readinessLabel') }}</small>
              <span class="minimal-badge" :class="readinessType">{{ readinessLabel }}</span>
            </div>
            <div class="status-meta-item">
              <small class="mono-label">{{ t('protocols.configuredTransportLabel') }}</small>
              <strong class="mono-value">{{ configuredTransportsText }}</strong>
            </div>
            <div class="status-meta-item">
              <small class="mono-label">{{ t('protocols.activeTransportLabel') }}</small>
              <strong class="mono-value">{{ activeTransportText }}</strong>
            </div>
          </div>
        </div>
      </div>

      <div class="minimal-card transport-status-section">
        <div class="card-header">
          <strong>{{ t('protocols.transportStatusTitle') }}</strong>
        </div>
        <div class="transport-list-container">
          <div
            v-for="item in transportStatusItems"
            :key="item.transport"
            class="transport-line-item"
          >
            <div class="transport-line-header">
              <div class="transport-line-identity">
                <span class="transport-line-dot" :class="item.stateType"></span>
                <strong>{{ item.label }}</strong>
              </div>
              <span class="minimal-badge" :class="item.stateType">{{ item.stateLabel }}</span>
            </div>
            <div class="transport-line-content">
              <div class="transport-endpoint">
                <small class="mono-label">{{ t('protocols.fields.endpoint') }}</small>
                <code class="endpoint-code">{{ item.endpointText }}</code>
              </div>
              <div class="transport-summary">
                <small class="mono-label">{{ t('protocols.protocolSummaryLabel') }}</small>
                <div class="transport-summary-text">{{ item.summary }}</div>
              </div>
            </div>
          </div>
        </div>
        <div class="card-footer-hint">
          {{ t('transportStatusHint') }}
        </div>
      </div>
    </div>

    <div class="config-alerts-container" v-if="pageError || redactedFields.length > 0">
      <el-alert v-if="pageError" :title="t('errors.common.actionFailed')" type="error" :description="pageError" show-icon />
      <el-alert
        v-if="redactedFields.length > 0"
        :title="t('config.redactedTitle')"
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
          <div class="card-header">
            <strong>{{ section.title }}</strong>
            <span class="field-count-badge">{{ section.fields.length }} {{ t('config.fieldCount') }}</span>
          </div>

          <el-form label-position="top" class="protocol-settings-form" @submit.prevent>
            <div v-for="field in section.fields" :key="field.path" class="config-field-item">
              <el-form-item>
                <template #label>
                  <div class="field-label-wrap">
                    <span class="field-label-text">{{ field.label }}</span>
                    <el-tooltip v-if="field.description" :content="field.description" placement="top">
                      <span class="field-info-icon">?</span>
                    </el-tooltip>
                  </div>
                </template>

                <el-input
                  v-if="field.type === 'text'"
                  :model-value="String(readField(field.path, field.type) ?? '')"
                  :aria-label="field.label"
                  class="refined-input"
                  @update:model-value="(value) => writeField(field.path, field.type, value)"
                />

                <el-input-number
                  v-else-if="field.type === 'number'"
                  :model-value="Number(readField(field.path, field.type) ?? 0)"
                  :aria-label="field.label"
                  :min="0"
                  :step="1"
                  controls-position="right"
                  class="refined-number-input"
                  @update:model-value="(value) => writeField(field.path, field.type, value ?? 0)"
                />

                <div v-else-if="field.type === 'boolean'" class="switch-wrap">
                  <el-switch
                    :model-value="Boolean(readField(field.path, field.type))"
                    :aria-label="field.label"
                    @update:model-value="(value) => writeField(field.path, field.type, value)"
                    style="--el-switch-on-color: var(--theme-accent); --el-switch-off-color: var(--theme-border-strong)"
                  />
                </div>

                <el-select
                  v-else-if="field.type === 'select'"
                  :model-value="String(readField(field.path, field.type) ?? '')"
                  :aria-label="field.label"
                  class="refined-input"
                  @update:model-value="(value) => writeField(field.path, field.type, value)"
                  popper-class="minimal-popper"
                >
                  <el-option
                    v-for="option in field.options"
                    :key="String(option.value)"
                    :label="option.label"
                    :value="option.value"
                  />
                </el-select>

                <el-input
                  v-else
                  :model-value="String(readField(field.path, field.type) ?? '')"
                  :aria-label="field.label"
                  type="textarea"
                  :autosize="{ minRows: 4, maxRows: 8 }"
                  class="refined-input"
                  @update:model-value="(value) => writeField(field.path, field.type, value)"
                />
              </el-form-item>
            </div>
          </el-form>
        </div>
      </div>
    </section>
  </div>
</template>

<style lang="scss" scoped>
.protocol-overview-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 480px;
  gap: var(--space-lg);
  margin-bottom: var(--space-xl);
}

.protocol-main-status {
  flex-direction: column;
}

.protocol-status-display {
  padding: var(--space-xl);
  display: grid;
  grid-template-columns: 280px 1fr;
  gap: var(--space-2xl);
  flex: 1;
}

.status-main-info {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
  border-right: 1px solid var(--theme-border);
  padding-right: var(--space-2xl);
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
  font-size: 1.05rem;
  font-weight: 600;
  line-height: 1.5;
  margin-top: var(--space-xs);
  color: var(--theme-text);
}

.status-meta-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: var(--space-lg);
  align-content: start;
}

.status-meta-item {
  display: flex;
  flex-direction: column;
  gap: var(--space-xs);
}

/* Transport List */
.transport-status-section {
  display: flex;
  flex-direction: column;
}

.transport-list-container {
  padding: var(--space-md);
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
  flex: 1;
}

.transport-line-item {
  border: 1px solid var(--theme-border);
  border-radius: 8px;
  background: var(--theme-bg);
  padding: var(--space-md);
  transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);

  &:hover {
    background: var(--theme-surface);
    border-color: var(--theme-border-strong);
    transform: translateY(-1px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.02);
  }
}

.transport-line-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: var(--space-sm);
}

.transport-line-identity {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  font-family: var(--font-sans);
  font-weight: 600;
  font-size: 0.95rem;
  color: var(--theme-text);
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

.transport-line-content {
  display: grid;
  gap: var(--space-xs);
  padding-left: var(--space-lg);
}

.endpoint-code {
  background: var(--theme-surface-hover);
  border: 1px solid var(--theme-border);
  border-radius: 4px;
  padding: 2px 6px;
  font-family: var(--font-mono);
  font-size: 0.85rem;
  color: var(--theme-text-muted);
}

.transport-summary-text {
  font-size: 0.9rem;
  color: var(--theme-text-muted);
  line-height: 1.4;
}

.card-footer-hint {
  padding: var(--space-sm) var(--space-lg);
  font-size: 0.8rem;
  color: var(--theme-text-muted);
  background: var(--theme-bg);
  border-top: 1px solid var(--theme-border);
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

@media (max-width: 1280px) {
  .protocol-overview-layout {
    grid-template-columns: 1fr;
  }
  .protocol-status-display {
    grid-template-columns: 1fr;
  }
  .status-main-info {
    border-right: none;
    border-bottom: 1px solid var(--theme-border);
    padding-right: 0;
    padding-bottom: var(--space-lg);
  }
}
</style>
