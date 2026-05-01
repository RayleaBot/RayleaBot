<script setup lang="ts">
import {
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  MessageOutlined,
  NotificationOutlined,
  ReloadOutlined,
  SaveOutlined,
  SendOutlined,
  TeamOutlined,
  ThunderboltOutlined,
  UserOutlined,
} from '@ant-design/icons-vue'
import { computed, onBeforeUnmount, onDeactivated, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import { notifySuccess } from '@/adapter/feedback'
import AppSkeletonCard from '@/components/AppSkeletonCard.vue'
import RateLimitInput from '@/components/config/RateLimitInput.vue'
import AppPage from '@/components/page/AppPage.vue'
import AppStatCard from '@/components/AppStatCard.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import {
  cloneConfig,
  getRateLimitConfigSections,
  getValueByPath,
  setValueByPath,
  type ConfigFieldDefinition,
} from '@/lib/config-form'
import { formatRateLimit, fromMultilineList, toMultilineList } from '@/lib/format'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'
import type { ConfigDocument } from '@/types/api'

const configStore = useConfigStore()
const { document, error, loading, redactedFields, saving } = storeToRefs(configStore)

const draft = ref<ConfigDocument | null>(null)
const saveStatus = ref<'hot' | 'restart' | null>(null)
let saveStatusTimer: number | null = null

const configSections = computed(() => getRateLimitConfigSections())
const hasUnsavedChanges = computed(() => {
  if (!draft.value || !document.value) {
    return false
  }

  return JSON.stringify(draft.value) !== JSON.stringify(document.value)
})
const canSave = computed(() => hasUnsavedChanges.value && !saving.value)
const saveStatusLabel = computed(() => {
  switch (saveStatus.value) {
    case 'restart':
      return t('rateLimits.status.savedRestart')
    case 'hot':
      return t('rateLimits.status.savedHot')
    default:
      return ''
  }
})
const summaryCards = computed(() => [
  {
    key: 'user-command',
    icon: UserOutlined,
    label: t('rateLimits.summary.userCommand'),
    value: formatRateLimit(document.value?.user.command_rate_limit),
    description: t('rateLimits.summary.userCommandMeta'),
    tone: 'primary' as const,
  },
  {
    key: 'group-command',
    icon: TeamOutlined,
    label: t('rateLimits.summary.groupCommand'),
    value: formatRateLimit(document.value?.group.command_rate_limit),
    description: t('rateLimits.summary.groupCommandMeta'),
    tone: 'default' as const,
  },
  {
    key: 'plugin-message',
    icon: MessageOutlined,
    label: t('rateLimits.summary.pluginMessage'),
    value: formatRateLimit(document.value?.message.rate_limit_per_plugin),
    description: t('rateLimits.summary.pluginMessageMeta'),
    tone: 'success' as const,
  },
  {
    key: 'target-message',
    icon: NotificationOutlined,
    label: t('rateLimits.summary.targetMessage'),
    value: formatRateLimit(document.value?.message.rate_limit_per_target),
    description: t('rateLimits.summary.targetMessageMeta'),
    tone: 'warning' as const,
  },
])

watch(document, (value) => {
  draft.value = value ? cloneConfig(value) : null
}, { immediate: true })

async function loadConfig() {
  try {
    await configStore.fetchConfig()
  } catch {
    // store error state drives the page
  }
}

onMounted(() => {
  void loadConfig()
})

onDeactivated(() => {
  clearSaveStatus()
})

onBeforeUnmount(() => {
  clearSaveStatus()
})

function clearSaveStatus() {
  if (saveStatusTimer !== null) {
    window.clearTimeout(saveStatusTimer)
    saveStatusTimer = null
  }
  saveStatus.value = null
}

function showSaveStatus(nextRestartRequired: boolean) {
  clearSaveStatus()
  saveStatus.value = nextRestartRequired ? 'restart' : 'hot'
  saveStatusTimer = window.setTimeout(() => {
    saveStatus.value = null
    saveStatusTimer = null
  }, 3000)
}

function markDraftChanged() {
  if (saveStatus.value !== null) {
    clearSaveStatus()
  }
}

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

  markDraftChanged()
  setValueByPath(draft.value as unknown as Record<string, unknown>, path, normalized)
}

function getSectionIcon(sectionTitle: string) {
  switch (sectionTitle) {
    case t('rateLimits.sections.userCommand'):
      return UserOutlined
    case t('rateLimits.sections.groupCommand'):
      return TeamOutlined
    case t('rateLimits.sections.cooldownReply'):
      return SendOutlined
    case t('rateLimits.sections.pluginMessage'):
      return MessageOutlined
    case t('rateLimits.sections.targetMessage'):
      return NotificationOutlined
    default:
      return ThunderboltOutlined
  }
}

function getRateLimitPreview(field: ConfigFieldDefinition) {
  if (field.type !== 'rateLimit') {
    return null
  }

  const rawValue = String(readField(field.path, field.type) ?? '').trim()
  if (!rawValue) {
    return null
  }

  const preview = formatRateLimit(rawValue)
  return preview !== rawValue ? preview : null
}

async function save() {
  if (!draft.value || !hasUnsavedChanges.value) {
    return
  }

  const response = await configStore.saveConfig(draft.value)
  showSaveStatus(response.restart_required)
  notifySuccess(response.restart_required ? t('config.saveRestart') : t('config.saveSuccess'))
}
</script>

<template>
  <AppPage :title="t('rateLimits.title')">
    <template #extra>
      <div class="table-actions">
        <a-button
          type="primary"
          :disabled="!canSave"
          :loading="saving"
          :aria-label="t('config.save')"
          data-testid="rate-limits-save"
          @click="save"
        >
          <template #icon>
            <SaveOutlined />
          </template>
          {{ t('config.save') }}
        </a-button>
        <a-button :loading="loading" :aria-label="t('dashboard.refresh')" @click="loadConfig">
          <template #icon>
            <ReloadOutlined />
          </template>
          {{ t('dashboard.refresh') }}
        </a-button>
      </div>
    </template>

    <div class="rate-limits-page">
      <div v-if="error || redactedFields.length > 0" class="rate-limits-alerts-container">
        <a-alert v-if="error" :message="t('errors.common.actionFailed')" type="error" :description="error" show-icon />
        <a-alert
          v-if="redactedFields.length > 0"
          :message="t('config.redactedTitle')"
          type="info"
          :description="redactedFields.join(', ')"
          show-icon
        />
      </div>

      <RetryPanel
        v-if="error && !draft"
        :title="t('rateLimits.title')"
        :description="error"
        :loading="loading"
        @retry="loadConfig"
      />

      <div v-else-if="loading && !draft" class="rate-limits-skeleton-layout">
        <AppSkeletonCard show-header :rows="5" />
      </div>

      <template v-else-if="draft">
        <div class="rate-limits-summary-cards" data-testid="rate-limits-summary-card">
          <AppStatCard
            v-for="card in summaryCards"
            :key="card.key"
            :icon="card.icon"
            :label="card.label"
            :tone="card.tone"
            :value="card.value"
            :description="card.description"
          />
        </div>

        <section class="rate-limits-board" :aria-label="t('rateLimits.sections.settings')">
          <div class="rate-limits-board__header">
            <div class="rate-limits-board__title">
              <span class="rate-limits-board__icon">
                <ThunderboltOutlined />
              </span>
              <h2>{{ t('rateLimits.sections.settings') }}</h2>
            </div>
            <div class="rate-limits-status-row" aria-live="polite">
              <span
                v-if="hasUnsavedChanges"
                class="rate-limits-status-pill rate-limits-status-pill--dirty"
                data-testid="rate-limits-unsaved-status"
              >
                <ExclamationCircleOutlined />
                {{ t('rateLimits.status.unsaved') }}
              </span>
              <span
                v-else-if="saveStatus"
                class="rate-limits-status-pill rate-limits-status-pill--saved"
                data-testid="rate-limits-save-status"
              >
                <CheckCircleOutlined />
                {{ saveStatusLabel }}
              </span>
            </div>
          </div>

          <a-form layout="vertical" class="rate-limits-form-matrix">
            <section
              v-for="section in configSections"
              :key="section.key"
              class="rate-limits-setting-row"
            >
              <div class="rate-limits-setting-row__intro">
                <span class="rate-limits-setting-row__icon">
                  <component :is="getSectionIcon(section.title)" />
                </span>
                <div class="rate-limits-setting-row__title">
                  <h3>{{ section.title }}</h3>
                </div>
              </div>

              <div class="rate-limits-setting-row__controls">
                <div v-for="field in section.fields" :key="field.path" class="rate-limits-field-item">
                  <a-form-item>
                    <template #label>
                      <div class="field-label-wrap">
                        <span class="field-label-text">{{ field.label }}</span>
                        <a-tooltip v-if="field.description" :title="field.description">
                          <button type="button" class="field-info-icon" :aria-label="t('config.fieldHelp')">?</button>
                        </a-tooltip>
                      </div>
                    </template>

                    <div class="rate-limits-control-wrap" :class="{ 'rate-limits-control-wrap--with-preview': getRateLimitPreview(field) }">
                      <RateLimitInput
                        v-if="field.type === 'rateLimit'"
                        :value="String(readField(field.path, field.type) ?? '')"
                        :aria-label="field.label"
                        @update:value="(value) => writeField(field.path, field.type, value)"
                      />

                      <div v-else-if="field.type === 'boolean'" class="switch-wrap">
                        <a-switch
                          :checked="Boolean(readField(field.path, field.type))"
                          :aria-label="field.label"
                          @update:checked="(value) => writeField(field.path, field.type, value)"
                        />
                      </div>

                      <div v-if="getRateLimitPreview(field)" class="rate-limits-rate-preview">
                        <span class="rate-limits-rate-preview__label">{{ t('config.hints.rateLimitPreview') }}</span>
                        <strong class="rate-limits-rate-preview__value">{{ getRateLimitPreview(field) }}</strong>
                      </div>
                    </div>

                    <div v-if="field.description" class="rate-limits-field-note">
                      <p class="rate-limits-field-note__text">{{ field.description }}</p>
                    </div>
                  </a-form-item>
                </div>
              </div>
            </section>
          </a-form>
        </section>
      </template>
    </div>
  </AppPage>
</template>

<style lang="scss" scoped>
.rate-limits-page {
  display: grid;
  gap: 18px;
}

.rate-limits-alerts-container,
.rate-limits-skeleton-layout {
  display: grid;
  gap: 12px;
}

.rate-limits-summary-cards {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.rate-limits-summary-cards :deep(> *) {
  min-height: 112px;
  padding: 18px;
  border-radius: var(--app-card-radius);
  background: linear-gradient(135deg, color-mix(in srgb, var(--surface-strong) 92%, white) 0%, var(--surface) 100%);
  box-shadow: 0 8px 24px color-mix(in srgb, var(--shadow-color, #0f172a) 7%, transparent);
}

.rate-limits-summary-cards :deep(.app-stat-card__accent) {
  opacity: 0;
}

.rate-limits-summary-cards :deep(.stat-card)::before {
  display: none;
}

.rate-limits-summary-cards :deep(.app-stat-card__icon-wrap) {
  width: 44px;
  height: 44px;
}

.rate-limits-summary-cards :deep(.app-stat-card__value) {
  font-size: 1.08rem;
}

.rate-limits-board {
  display: grid;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: var(--app-card-radius);
  background: var(--surface-strong);
  box-shadow: var(--shadow-xs);
}

.rate-limits-board__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 18px 20px;
  border-bottom: 1px solid var(--border);
}

.rate-limits-board__title {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.rate-limits-board__title h2 {
  margin: 0;
  color: var(--text);
  font-size: 1rem;
  font-weight: 700;
}

.rate-limits-board__icon,
.rate-limits-setting-row__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border-radius: 50%;
  color: var(--accent);
  background: var(--surface-accent);
  border: 1px solid var(--border-accent);
}

.rate-limits-board__icon {
  width: 28px;
  height: 28px;
}

.rate-limits-status-row {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  min-height: 28px;
}

.rate-limits-status-pill {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-height: 28px;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 0.82rem;
  font-weight: 650;
  line-height: 1;
  box-shadow: var(--shadow-xs);
}

.rate-limits-status-pill--dirty {
  color: color-mix(in srgb, var(--warning) 72%, #7c2d12);
  background: color-mix(in srgb, var(--surface-warning) 86%, white);
  border: 1px solid color-mix(in srgb, var(--warning) 35%, var(--border));
}

.rate-limits-status-pill--saved {
  color: color-mix(in srgb, var(--success) 76%, #14532d);
  background: color-mix(in srgb, var(--surface-success) 88%, white);
  border: 1px solid color-mix(in srgb, var(--success) 32%, var(--border));
}

.rate-limits-form-matrix {
  display: grid;
}

.rate-limits-setting-row {
  display: grid;
  grid-template-columns: minmax(176px, 240px) minmax(0, 1fr);
  gap: 24px;
  padding: 18px 20px;
  border-top: 1px solid var(--border);
}

.rate-limits-setting-row:first-child {
  border-top: 0;
}

.rate-limits-setting-row__intro {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  min-width: 0;
  padding-top: 2px;
}

.rate-limits-setting-row__icon {
  width: 26px;
  height: 26px;
}

.rate-limits-setting-row__title h3 {
  margin: 0;
  color: var(--text);
  font-size: 0.95rem;
  font-weight: 700;
  line-height: 1.35;
}

.rate-limits-setting-row__controls {
  display: grid;
  gap: 12px;
  min-width: 0;
}

.rate-limits-field-item :deep(.ant-form-item) {
  margin-bottom: 0;
}

.field-label-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
}

.field-label-text {
  font-weight: 600;
  font-size: 0.85rem;
  color: var(--theme-text, var(--text));
}

.field-info-icon {
  appearance: none;
  background: transparent;
  color: var(--muted);
  cursor: help;
  font-size: 0.8rem;
  font-weight: bold;
  opacity: 0.7;
  width: 18px;
  height: 18px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
}

.field-info-icon:hover {
  opacity: 1;
  color: var(--accent);
  border-color: var(--accent);
}

.field-info-icon:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.rate-limits-control-wrap {
  display: grid;
  gap: 10px;
  align-items: stretch;
}

.rate-limits-control-wrap--with-preview {
  grid-template-columns: minmax(260px, 1fr) minmax(180px, 240px);
}

.switch-wrap {
  display: flex;
  min-height: 36px;
  align-items: center;
}

.rate-limits-rate-preview {
  display: grid;
  align-content: center;
  gap: 3px;
  min-height: 36px;
  padding: 7px 10px;
  border-radius: var(--radius-md);
  background: var(--surface-accent);
  border: 1px solid var(--border-accent);
}

.rate-limits-rate-preview__label {
  font-size: 0.75rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--accent);
}

.rate-limits-rate-preview__value {
  color: var(--text);
  font-size: 0.9rem;
  line-height: 1.4;
}

.rate-limits-field-note {
  display: grid;
  margin-top: 8px;
}

.rate-limits-field-note__text {
  margin: 0;
  color: var(--muted);
  font-size: 0.82rem;
  line-height: 1.6;
}

@media (max-width: 1180px) {
  .rate-limits-summary-cards {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 860px) {
  .rate-limits-board__header {
    align-items: flex-start;
    flex-direction: column;
  }

  .rate-limits-setting-row {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .rate-limits-control-wrap--with-preview {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .rate-limits-summary-cards {
    grid-template-columns: 1fr;
  }
}
</style>
