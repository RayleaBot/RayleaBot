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
  compatibility,
  error: protocolsError,
  loading: protocolsLoading,
  snapshot,
} = storeToRefs(protocolsStore)

const draft = ref<ConfigDocument | null>(null)

const configSections = computed(() => getProtocolConfigSections())
const protocolStatusLabel = computed(() => getAdapterStateLabel(snapshot.value?.connection_state))
const protocolStatusType = computed(() => getStatusType(snapshot.value?.connection_state))
const readinessLabel = computed(() => getReadinessStatusLabel(snapshot.value?.readiness_status))
const readinessType = computed(() => getStatusType(snapshot.value?.readiness_status))
const pageLoading = computed(() => configLoading.value || protocolsLoading.value)
const protocolSummary = computed(() => snapshot.value?.summary ?? t('display.empty'))
const configuredTransportsText = computed(() => (
  snapshot.value?.configured_transports.length
    ? snapshot.value.configured_transports.join(' / ')
    : t('display.empty')
))
const activeTransportText = computed(() => snapshot.value?.active_transport ?? t('display.empty'))
const pageError = computed(() => configError.value || protocolsError.value)

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
  <div class="page-grid industrial-theme">
    <section class="hero-panel">
      <div class="hero-text">
        <h1 class="glitch-title">{{ t('protocols.title') }}</h1>
        <p class="subtitle">>> {{ t('protocols.subtitle') }}</p>
      </div>

      <div class="hero-actions">
        <el-button class="industrial-btn primary" :disabled="!canSave" :loading="saving" @click="save">
          [ {{ t('protocols.save') }} ]
        </el-button>
        <el-button class="industrial-btn" :loading="pageLoading" @click="loadPage">
          [ {{ t('dashboard.refresh') }} ]
        </el-button>
        <el-button class="industrial-btn outline" @click="router.push('/protocols/logs')">
          [ {{ t('protocols.openLogs') }} ]
        </el-button>
      </div>
    </section>

    <div class="industrial-card protocol-overview-card">
      <div class="card-header">
        <strong class="uppercase">> {{ t('protocols.overviewTitle') }}</strong>
        <span class="industrial-badge">{{ t('protocols.fixedProtocolLabel') }}: {{ ONEBOT11_PROTOCOL_NAME }}</span>
      </div>

      <div class="protocol-overview-grid">
        <div class="overview-item">
          <small class="mono-label">[{{ t('protocols.protocolNameLabel') }}]</small>
          <strong class="mono-value">{{ ONEBOT11_PROTOCOL_NAME }}</strong>
        </div>
        <div class="overview-item">
          <small class="mono-label">[{{ t('protocols.protocolStatusLabel') }}]</small>
          <span class="industrial-badge status-badge" :class="protocolStatusType">
            {{ protocolStatusLabel }}
          </span>
        </div>
        <div class="overview-item">
          <small class="mono-label">[{{ t('protocols.protocolSummaryLabel') }}]</small>
          <strong class="mono-value highlight-value">{{ protocolSummary }}</strong>
        </div>
        <div class="overview-item">
          <small class="mono-label">[{{ t('protocols.providerLabel') }}]</small>
          <strong class="mono-value">{{ snapshot?.provider || t('display.empty') }}</strong>
        </div>
        <div class="overview-item">
          <small class="mono-label">[{{ t('protocols.readinessLabel') }}]</small>
          <span class="industrial-badge status-badge" :class="readinessType">
            {{ readinessLabel }}
          </span>
        </div>
        <div class="overview-item">
          <small class="mono-label">[{{ t('protocols.configuredTransportLabel') }}]</small>
          <strong class="mono-value">{{ configuredTransportsText }}</strong>
        </div>
        <div class="overview-item">
          <small class="mono-label">[{{ t('protocols.activeTransportLabel') }}]</small>
          <strong class="mono-value">{{ activeTransportText }}</strong>
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
      <div class="industrial-card protocol-matrix-card" v-if="compatibility">
        <div class="card-header">
          <strong>> {{ t('protocols.compatibilityTitle') }}</strong>
          <span>{{ compatibility.generated_at }}</span>
        </div>

        <div class="protocol-matrix-groups">
          <div v-for="group in compatibility.groups" :key="group.group" class="matrix-group">
            <h3>{{ group.title }}</h3>
            <div class="matrix-item-grid">
              <div v-for="item in group.items" :key="`${group.group}-${item.name}`" class="matrix-item">
                <strong>{{ item.name }}</strong>
                <span class="industrial-badge" :class="getStatusType(item.status)">
                  {{ item.status }}
                </span>
                <small v-if="item.provider">{{ item.provider }}</small>
                <small v-else-if="item.notes">{{ item.notes }}</small>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="section-heading">
        <div>
          <h2>{{ t('protocols.connectionSettings') }}</h2>
          <p class="subtitle">>> {{ t('protocols.connectionSettingsHint') }}</p>
        </div>
        <div v-if="restartRequired !== null" class="restart-indicator">
          <span class="industrial-badge" :class="restartRequired ? 'warning' : 'success'">
            {{ restartRequired ? t('config.restartNeeded') : t('config.hotApplied') }}
          </span>
        </div>
      </div>

      <div v-if="draft" class="protocol-settings-grid">
        <div v-for="section in configSections" :key="section.key" class="industrial-card">
          <div class="card-header">
            <strong>> {{ section.title }}</strong>
            <span>[{{ section.fields.length }} {{ t('config.fieldCount') }}]</span>
          </div>

          <el-form label-position="top" class="protocol-form-grid">
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
                    style="--el-switch-on-color: var(--accent-color); --el-switch-off-color: var(--border-color)"
                  />
                </div>

                <el-select
                  v-else-if="field.type === 'select'"
                  :model-value="String(readField(field.path, field.type) ?? '')"
                  :aria-label="field.label"
                  class="refined-input"
                  @update:model-value="(value) => writeField(field.path, field.type, value)"
                  popper-class="industrial-popper"
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
                  class="refined-input refined-textarea"
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
.industrial-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  transform: translate(2px, 2px) !important;
  box-shadow: 2px 2px 0px var(--border-color) !important;
}

/* Cards */
.industrial-card {
  background: var(--card-bg);
  border: 3px solid var(--border-color);
  box-shadow: 6px 6px 0px var(--border-color);
  margin-bottom: 32px;
}

.protocol-matrix-groups {
  display: grid;
  gap: 20px;
  padding: 20px;
}

.matrix-group h3 {
  margin: 0 0 12px;
}

.matrix-item-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
}

.matrix-item {
  display: grid;
  gap: 8px;
  padding: 12px;
  border: 2px solid var(--border-color);
  background: rgba(255, 255, 255, 0.75);
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

.protocol-overview-grid, .protocol-settings-grid {
  display: grid;
  gap: 20px;
}
.protocol-overview-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
  padding: 20px;
}
.protocol-settings-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.overview-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 16px;
  background: rgba(17, 17, 17, 0.03);
  border: 2px dashed var(--border-color);
}

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
}

.highlight-value {
  color: var(--accent-color);
}

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

/* Forms */
.protocol-form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 24px;
  padding: 20px;
}

.config-field-item {
  display: flex;
  flex-direction: column;
}

.field-label-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  font-family: "Cascadia Mono", monospace;
  font-weight: bold;
  margin-bottom: 8px;
}

.field-info-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  background: var(--text-main);
  color: #fff;
  font-size: 0.8rem;
  font-weight: bold;
  cursor: help;
}

.refined-input {
  :deep(.el-input__wrapper),
  :deep(.el-textarea__inner) {
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

.refined-number-input {
  :deep(.el-input__wrapper) {
    border-radius: 0;
    border: 2px solid var(--border-color);
    box-shadow: none !important;
    font-family: "Cascadia Mono", monospace;
    
    &:hover, &.is-focus {
      border-color: var(--accent-color);
      box-shadow: 4px 4px 0px var(--border-color) !important;
      transform: translate(-2px, -2px);
    }
  }
}

.config-alerts-container {
  display: grid;
  gap: 12px;
  margin-bottom: 32px;
}

@media (max-width: 1024px) {
  .protocol-overview-grid, .protocol-settings-grid {
    grid-template-columns: 1fr;
  }
  .hero-panel {
    flex-direction: column;
    align-items: flex-start;
    gap: 16px;
  }
}
</style>
