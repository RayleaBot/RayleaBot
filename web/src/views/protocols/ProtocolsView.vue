<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import { notifySuccess } from '@/adapter/feedback'
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
</script>

<template>
  <AppPage :title="t('protocols.title')" :description="t('protocols.subtitle')">
    <template #extra>
      <div class="table-actions">
        <a-button :loading="pageLoading" @click="loadPage">{{ t('dashboard.refresh') }}</a-button>
        <a-button type="primary" :disabled="!canSave" :loading="saving" @click="save">
          {{ t('protocols.save') }}
        </a-button>
      </div>
    </template>

    <div class="protocol-settings-page">
      <div class="protocol-overview-grid">
        <a-card :bordered="false" class="protocol-overview-card">
          <div class="protocol-overview-card__top">
            <span class="overview-label">{{ t('protocols.overviewTitle') }}</span>
            <a-tag color="blue">{{ ONEBOT11_PROTOCOL_NAME }}</a-tag>
          </div>
          <div class="protocol-overview-card__value-row">
            <strong>{{ protocolStatusLabel }}</strong>
            <a-tag :color="getStatusTagColor(protocolStatusType)">{{ readinessLabel }}</a-tag>
          </div>
          <p>{{ protocolSummary }}</p>
        </a-card>

        <a-card :bordered="false" class="protocol-overview-card">
          <div class="protocol-overview-card__top">
            <span class="overview-label">{{ t('protocols.providerLabel') }}</span>
            <a-tag :color="getStatusTagColor(readinessType)">{{ readinessLabel }}</a-tag>
          </div>
          <strong>{{ snapshot?.provider || t('display.empty') }}</strong>
          <p>{{ protocolStatusLabel }}</p>
        </a-card>

        <a-card :bordered="false" class="protocol-overview-card">
          <div class="protocol-overview-card__top">
            <span class="overview-label">{{ t('protocols.activeTransportLabel') }}</span>
            <a-tag>{{ snapshot?.active_transports.length || 0 }}</a-tag>
          </div>
          <strong>{{ activeTransportText }}</strong>
          <p>{{ t('protocols.configuredTransportLabel') }}：{{ configuredTransportsText }}</p>
        </a-card>
      </div>

      <div class="transport-cards-section">
        <div class="section-heading">
          <div>
            <h2>{{ t('protocols.transportStatusTitle') }}</h2>
            <p class="subtitle">{{ t('protocols.transportStatusHint') }}</p>
          </div>
        </div>
        <div class="transport-cards-grid">
          <a-card
            v-for="item in transportStatusItems"
            :key="item.transport"
            :bordered="false"
            class="transport-card"
          >
            <div class="transport-card-header">
              <div class="transport-identity">
                <span class="transport-line-dot" :class="item.stateType"></span>
                <strong>{{ item.label }}</strong>
              </div>
              <a-tag :color="getStatusTagColor(item.stateType)">{{ item.stateLabel }}</a-tag>
            </div>
            <div class="transport-card-body">
              <div class="transport-endpoint">
                <code class="endpoint-code">{{ item.endpointText }}</code>
              </div>
              <div class="transport-summary-text">{{ item.summary }}</div>
            </div>
          </a-card>
        </div>
      </div>

      <div v-if="transportIssues.length" class="transport-issues-section" data-testid="protocol-issues">
        <div class="section-heading">
          <div>
            <h2>{{ t('protocols.transportIssuesTitle') }}</h2>
            <p class="subtitle">{{ t('protocols.transportIssuesHint') }}</p>
          </div>
        </div>
        <div class="transport-issues-list">
          <section
            v-for="issue in transportIssues"
            :key="`${issue.code}-${issue.summary}`"
            class="transport-issue-card"
          >
            <div class="transport-issue-card__header">
              <a-tag :color="getIssueTagColor(issue.severity)">{{ issue.code }}</a-tag>
            </div>
            <p>{{ issue.summary }}</p>
          </section>
        </div>
      </div>

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
            <a-tag :color="restartRequired ? 'warning' : 'success'">
              {{ restartRequired ? t('config.restartNeeded') : t('config.hotApplied') }}
            </a-tag>
          </div>
        </div>
        <div v-if="draft" class="protocol-settings-layout">
          <a-card v-for="section in configSections" :key="section.key" :bordered="false" class="protocol-config-card">
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
                    :value="typeof readField(field.path, field.type) === 'number' ? readField(field.path, field.type) : null"
                    :aria-label="field.label"
                    :min="0"
                    :step="1"
                    class="refined-number-input"
                    @update:value="(value) => writeField(field.path, field.type, value)"
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
          </a-card>
        </div>
      </section>
    </div>
  </AppPage>
</template>

<style lang="scss" scoped>
.protocol-settings-page {
  --font-mono: "Cascadia Mono", "Consolas", monospace;
  display: grid;
  gap: var(--app-layout-gap);
  color: var(--app-text);
}

.protocol-overview-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.protocol-overview-card {
  min-height: 0;
}

.protocol-overview-card :deep(.ant-card-body),
.transport-card :deep(.ant-card-body),
.protocol-config-card :deep(.ant-card-body) {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.protocol-overview-card :deep(.ant-card-body) {
  padding: 14px;
}

.transport-card :deep(.ant-card-body) {
  padding: 14px 16px;
}

.protocol-config-card :deep(.ant-card-body) {
  padding: 0;
}

.protocol-overview-card__top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.overview-label {
  color: var(--app-text-secondary);
  font-size: 0.78rem;
  letter-spacing: 0.02em;
}

.protocol-overview-card__value-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.protocol-overview-card strong {
  font-size: 1.1rem;
  line-height: 1.35;
  color: var(--app-text);
}

.protocol-overview-card p {
  margin: 0;
  color: var(--app-text-secondary);
  font-size: 0.86rem;
  line-height: 1.5;
}

.transport-cards-section {
  display: grid;
  gap: 12px;
}

.transport-cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: var(--space-md);
}

.transport-issues-section {
  display: grid;
  gap: 12px;
}

.transport-issues-list {
  display: grid;
  gap: 12px;
}

.transport-issue-card {
  display: grid;
  gap: 10px;
  padding: 14px 16px;
  border-radius: var(--radius-lg);
  border: 1px solid color-mix(in srgb, var(--warning) 24%, var(--app-border));
  background: color-mix(in srgb, var(--warning) 7%, transparent);
}

.transport-issue-card__header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.transport-issue-card p {
  margin: 0;
  line-height: 1.55;
  color: var(--app-text);
}

.transport-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid var(--app-border);
  padding-bottom: var(--space-sm);
}

.transport-identity {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  font-weight: 600;
  font-size: 0.92rem;
}

.transport-line-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: color-mix(in srgb, var(--app-border) 70%, var(--app-primary) 30%);
  
  &.success { background: var(--app-success); }
  &.danger { background: var(--app-danger); }
  &.warning { background: var(--app-warning); }
}

.transport-card-body {
  display: grid;
  gap: var(--space-xs);
}

.endpoint-code {
  display: inline-block;
  background: var(--surface-soft);
  border: 1px solid var(--app-border);
  border-radius: 6px;
  padding: 4px 8px;
  font-family: var(--font-mono);
  font-size: 0.85rem;
  color: var(--app-text-secondary);
  word-break: break-all;
}

.transport-summary-text {
  font-size: 0.9rem;
  color: var(--app-text-secondary);
  line-height: 1.4;
  margin-top: var(--space-xs);
}

/* Settings Layout */
.protocol-settings-section {
  display: grid;
  gap: 12px;
}

.protocol-settings-layout {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
  gap: var(--space-lg);
}

.config-card-header {
  background: var(--surface-soft);
  border-bottom: 1px solid var(--app-border);
  padding: 14px 16px;
}

.protocol-settings-form {
  padding: 16px;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px 20px;
}

.field-label-wrap {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
}

.field-label-text {
  font-weight: 600;
  font-size: 0.85rem;
  color: var(--app-text);
}

.field-info-icon {
  color: var(--app-text-secondary);
  cursor: help;
  font-size: 0.8rem;
  font-weight: bold;
  opacity: 0.7;

  &:hover {
    opacity: 1;
    color: var(--app-primary);
  }
}

.field-count-badge {
  background: transparent;
  color: var(--app-text-secondary);
  font-size: 0.8rem;
  font-weight: 500;
}

:deep(.protocol-config-card .ant-card-body),
:deep(.protocol-overview-card .ant-card-body),
:deep(.transport-card .ant-card-body) {
  box-sizing: border-box;
}

:deep(.refined-input.ant-input),
:deep(.refined-input.ant-input-affix-wrapper),
:deep(.refined-input.ant-input-textarea textarea.ant-input),
:deep(.refined-input.ant-select .ant-select-selector) {
  border-radius: 10px;
  background: var(--surface-soft);
  border-color: var(--app-border);
  box-shadow: none;
}

:deep(.refined-input.ant-input:hover),
:deep(.refined-input.ant-input-affix-wrapper:hover),
:deep(.refined-input.ant-input-textarea:hover textarea.ant-input),
:deep(.refined-input.ant-select:hover .ant-select-selector) {
  border-color: color-mix(in srgb, var(--app-primary) 24%, var(--app-border));
}

:deep(.refined-input.ant-input:focus),
:deep(.refined-input.ant-input-affix-wrapper.ant-input-affix-wrapper-focused),
:deep(.refined-input.ant-input-textarea textarea.ant-input:focus),
:deep(.refined-input.ant-select.ant-select-focused .ant-select-selector) {
  border-color: var(--app-primary);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--app-primary) 14%, transparent);
}

:deep(.refined-number-input.ant-input-number) {
  border-radius: 10px;
  background: var(--surface-soft);
  border-color: var(--app-border);
  box-shadow: none;
}

:deep(.refined-number-input.ant-input-number.ant-input-number-focused) {
  border-color: var(--app-primary);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--app-primary) 14%, transparent);
}

@media (max-width: 768px) {
  .protocol-overview-grid {
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
