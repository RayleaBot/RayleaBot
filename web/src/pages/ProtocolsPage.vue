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
import { getAdapterStateLabel, getStatusType } from '@/lib/display'
import { formatProtocolEventSummary, formatProtocolIssueSummary } from '@/lib/management-summary'
import { fromMultilineList, toMultilineList } from '@/lib/format'
import { ONEBOT11_PROTOCOL_NAME, isProtocolEvent, isProtocolIssue } from '@/lib/protocols'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'
import { useSystemStore } from '@/stores/system'
import type { ConfigDocument } from '@/types/api'

const router = useRouter()
const configStore = useConfigStore()
const systemStore = useSystemStore()

const {
  document,
  error: configError,
  loading: configLoading,
  redactedFields,
  restartRequired,
  saving,
} = storeToRefs(configStore)
const { readiness, recentEvents, system } = storeToRefs(systemStore)

const draft = ref<ConfigDocument | null>(null)

const configSections = computed(() => getProtocolConfigSections())
const protocolStatusLabel = computed(() => getAdapterStateLabel(system.value?.adapter_state))
const protocolStatusType = computed(() => getStatusType(system.value?.adapter_state))
const pageLoading = computed(() => configLoading.value)
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

watch(document, (value) => {
  draft.value = value ? cloneConfig(value) : null
}, { immediate: true })

async function loadPage() {
  try {
    const requests: Array<Promise<unknown>> = [configStore.fetchConfig()]
    if (!system.value || !readiness.value) {
      requests.push(systemStore.refresh())
    }
    await Promise.all(requests)
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
  <div class="page-grid">
    <section class="hero-panel">
      <div>
        <h1>{{ t('protocols.title') }}</h1>
        <p>{{ t('protocols.subtitle') }}</p>
      </div>

      <div class="hero-actions">
        <el-button type="primary" :disabled="!canSave" :loading="saving" @click="save">
          {{ t('protocols.save') }}
        </el-button>
        <el-button :loading="pageLoading" @click="loadPage">
          {{ t('dashboard.refresh') }}
        </el-button>
        <el-button plain @click="router.push('/protocols/logs')">
          {{ t('protocols.openLogs') }}
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

    <div class="config-alerts-container" v-if="configError || redactedFields.length > 0">
      <el-alert v-if="configError" :title="t('errors.common.actionFailed')" type="error" :description="configError" show-icon />
      <el-alert
        v-if="redactedFields.length > 0"
        :title="t('config.redactedTitle')"
        type="info"
        :description="redactedFields.join(', ')"
        show-icon
      />
    </div>

    <RetryPanel
      v-if="configError && !draft"
      :title="t('protocols.connectionSettings')"
      :description="configError"
      :loading="configLoading"
      @retry="loadPage"
    />

    <section v-else class="protocol-settings-section">
      <div class="section-heading">
        <div>
          <h2>{{ t('protocols.connectionSettings') }}</h2>
          <p>{{ t('protocols.connectionSettingsHint') }}</p>
        </div>
        <div v-if="restartRequired !== null" class="restart-indicator">
          <el-tag :type="restartRequired ? 'warning' : 'success'" size="small" effect="dark">
            {{ restartRequired ? t('config.restartNeeded') : t('config.hotApplied') }}
          </el-tag>
        </div>
      </div>

      <div v-if="draft" class="protocol-settings-grid">
        <el-card v-for="section in configSections" :key="section.key" class="protocol-settings-card">
          <template #header>
            <div class="protocol-settings-card__header">
              <strong>{{ section.title }}</strong>
              <span>{{ section.fields.length }} {{ t('config.fieldCount') }}</span>
            </div>
          </template>

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
                  class="refined-input"
                  @update:model-value="(value) => writeField(field.path, field.type, value)"
                />

                <el-input-number
                  v-else-if="field.type === 'number'"
                  :model-value="Number(readField(field.path, field.type) ?? 0)"
                  :min="0"
                  :step="1"
                  controls-position="right"
                  class="refined-number-input"
                  @update:model-value="(value) => writeField(field.path, field.type, value ?? 0)"
                />

                <div v-else-if="field.type === 'boolean'" class="switch-wrap">
                  <el-switch
                    :model-value="Boolean(readField(field.path, field.type))"
                    @update:model-value="(value) => writeField(field.path, field.type, value)"
                  />
                </div>

                <el-select
                  v-else-if="field.type === 'select'"
                  :model-value="String(readField(field.path, field.type) ?? '')"
                  class="refined-input"
                  @update:model-value="(value) => writeField(field.path, field.type, value)"
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
                  type="textarea"
                  :autosize="{ minRows: 4, maxRows: 8 }"
                  class="refined-input refined-textarea"
                  @update:model-value="(value) => writeField(field.path, field.type, value)"
                />
              </el-form-item>
            </div>
          </el-form>
        </el-card>
      </div>
    </section>
  </div>
</template>

<style lang="scss" scoped>
.hero-actions,
.section-heading,
.protocol-overview-header,
.protocol-settings-card__header {
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

.section-heading p {
  margin: 6px 0 0;
  color: var(--muted);
}

.config-alerts-container {
  display: grid;
  gap: 12px;
}

.protocol-overview-card,
.protocol-settings-card {
  border-radius: 24px;
}

.protocol-overview-grid,
.protocol-settings-grid {
  display: grid;
  gap: 16px;
}

.protocol-overview-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.protocol-settings-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
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

.protocol-overview-item small,
.protocol-settings-card__header span {
  color: var(--muted);
}

.protocol-form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 20px 24px;
}

.field-label-wrap {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.field-info-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: rgba(15, 111, 112, 0.12);
  color: #0f6f70;
  font-size: 0.75rem;
  font-weight: 700;
  cursor: help;
}

.refined-input {
  :deep(.el-input__wrapper),
  :deep(.el-textarea__inner) {
    border-radius: 16px;
    background: rgba(255, 255, 255, 0.7);
    border: 1px solid rgba(0, 0, 0, 0.08);
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.02) inset;
    transition: all 0.2s;

    &:hover {
      background: #fff;
      border-color: #0f6f70;
    }

    &.is-focus {
      background: #fff;
      box-shadow: 0 0 0 1px #0f6f70 inset, 0 10px 25px rgba(15, 111, 112, 0.1);
    }
  }
}

@media (max-width: 1024px) {
  .protocol-overview-grid,
  .protocol-settings-grid {
    grid-template-columns: 1fr;
  }

  .hero-actions {
    flex-wrap: wrap;
  }
}
</style>
