<script setup lang="ts">
import {
  ApiOutlined,
  CheckCircleOutlined,
  DatabaseOutlined,
  ExclamationCircleOutlined,
  FileTextOutlined,
  MessageOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  SaveOutlined,
  SettingOutlined,
  PictureOutlined,
} from '@ant-design/icons-vue'
import { computed, onBeforeUnmount, onDeactivated, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import { notifySuccess } from '@/adapter/feedback'
import AppSkeletonCard from '@/components/AppSkeletonCard.vue'
import RateLimitInput from '@/components/config/RateLimitInput.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import {
  cloneConfig,
  getPluginSettingsConfigSections,
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

const configSections = computed(() => getPluginSettingsConfigSections())
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
      return t('plugins.settings.status.savedRestart')
    case 'hot':
      return t('plugins.settings.status.savedHot')
    default:
      return ''
  }
})

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

function normalizeTagList(value: unknown) {
  const source = Array.isArray(value) ? value : [value]
  return source
    .map((item) => String(item).trim())
    .filter(Boolean)
}

function readCommandPrefixTags() {
  if (!draft.value) {
    return []
  }

  const current = getValueByPath(draft.value as unknown as Record<string, unknown>, 'command.prefixes')
  return Array.isArray(current) ? normalizeTagList(current) : []
}

function writeCommandPrefixTags(value: unknown) {
  if (!draft.value) {
    return
  }

  markDraftChanged()
  setValueByPath(draft.value as unknown as Record<string, unknown>, 'command.prefixes', normalizeTagList(value))
}

function isCommandPrefixField(path: string) {
  return path === 'command.prefixes'
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

function resetFieldToDefault(field: ConfigFieldDefinition) {
  if (!draft.value || field.defaultValue === undefined) {
    return
  }

  markDraftChanged()
  setValueByPath(draft.value as unknown as Record<string, unknown>, field.path, field.defaultValue)
}

function getSectionIcon(key: string) {
  switch (key) {
    case 'command':
      return ApiOutlined
    case 'permission':
      return SafetyCertificateOutlined
    case 'log':
      return FileTextOutlined
    case 'message':
      return MessageOutlined
    case 'render':
      return PictureOutlined
    case 'storage':
      return DatabaseOutlined
    default:
      return SettingOutlined
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
  <AppPage :title="t('plugins.settings.title')">
    <template #extra>
      <div class="table-actions">
        <a-button
          type="primary"
          :disabled="!canSave"
          :loading="saving"
          :aria-label="t('config.save')"
          data-testid="plugin-settings-save"
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

    <div v-if="error || redactedFields.length > 0" class="plugin-settings-alerts-container">
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
      :title="t('plugins.settings.title')"
      :description="error"
      :loading="loading"
      @retry="loadConfig"
    />

    <div v-else-if="loading && !draft" class="plugin-settings-skeleton-layout">
      <AppSkeletonCard show-header :rows="5" />
    </div>

    <div v-else-if="draft" class="plugin-settings-layout">
      <section class="plugin-settings-board" :aria-label="t('plugins.settings.title')">
        <div class="plugin-settings-board__header">
          <div class="plugin-settings-board__title">
            <span class="plugin-settings-board__icon">
              <SettingOutlined />
            </span>
            <h2>{{ t('plugins.settings.title') }}</h2>
          </div>
          <div class="plugin-settings-status-row" aria-live="polite">
            <span
              v-if="hasUnsavedChanges"
              class="plugin-settings-status-pill plugin-settings-status-pill--dirty"
              data-testid="plugin-settings-unsaved-status"
            >
              <ExclamationCircleOutlined />
              {{ t('plugins.settings.status.unsaved') }}
            </span>
            <span
              v-else-if="saveStatus"
              class="plugin-settings-status-pill plugin-settings-status-pill--saved"
              data-testid="plugin-settings-save-status"
            >
              <CheckCircleOutlined />
              {{ saveStatusLabel }}
            </span>
          </div>
        </div>

        <a-form layout="vertical" class="plugin-settings-form-matrix">
          <section
            v-for="section in configSections"
            :key="section.key"
            class="plugin-settings-setting-row"
          >
            <div class="plugin-settings-setting-row__intro">
              <span class="plugin-settings-setting-row__icon">
                <component :is="getSectionIcon(section.key)" />
              </span>
              <div class="plugin-settings-setting-row__title">
                <h3>{{ section.title }}</h3>
              </div>
            </div>

            <div class="plugin-settings-setting-row__controls">
              <div v-for="field in section.fields" :key="field.path" class="plugin-settings-field-item">
                <a-form-item>
                  <template #label>
                    <div class="field-label-wrap">
                      <span class="field-label-text">{{ field.label }}</span>
                      <a-tooltip v-if="field.description" :title="field.description">
                        <button type="button" class="field-info-icon" :aria-label="t('config.fieldHelp')">?</button>
                      </a-tooltip>
                    </div>
                  </template>

                  <div class="plugin-settings-control-wrap" :class="{ 'plugin-settings-control-wrap--with-preview': getRateLimitPreview(field) }">
                    <a-select
                      v-if="isCommandPrefixField(field.path)"
                      mode="tags"
                      class="plugin-settings-prefix-select"
                      data-testid="plugin-settings-command-prefixes"
                      :value="readCommandPrefixTags()"
                      :aria-label="field.label"
                      :placeholder="t('plugins.settings.placeholders.commandPrefixes')"
                      @update:value="writeCommandPrefixTags"
                    />

                    <RateLimitInput
                      v-else-if="field.type === 'rateLimit'"
                      :value="String(readField(field.path, field.type) ?? '')"
                      :aria-label="field.label"
                      @update:value="(value) => writeField(field.path, field.type, value)"
                    />

                    <a-input
                      v-else-if="field.type === 'text'"
                      :value="String(readField(field.path, field.type) ?? '')"
                      :aria-label="field.label"
                      @update:value="(value) => writeField(field.path, field.type, value)"
                    />

                    <a-input-number
                      v-else-if="field.type === 'number'"
                      class="plugin-settings-number-input"
                      :value="typeof readField(field.path, field.type) === 'number' ? readField(field.path, field.type) : null"
                      :min="0"
                      :step="1"
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

                    <a-select
                      v-else-if="field.type === 'select'"
                      :value="String(readField(field.path, field.type) ?? '')"
                      :options="field.options"
                      :aria-label="field.label"
                      @update:value="(value) => writeField(field.path, field.type, value)"
                    />

                    <a-textarea
                      v-else-if="field.type === 'textarea'"
                      :value="String(readField(field.path, field.type) ?? '')"
                      :auto-size="{ minRows: 3, maxRows: 7 }"
                      :aria-label="field.label"
                      @update:value="(value) => writeField(field.path, field.type, value)"
                    />

                    <a-textarea
                      v-else
                      :value="String(readField(field.path, field.type) ?? '')"
                      :auto-size="{ minRows: 3, maxRows: 7 }"
                      :aria-label="field.label"
                      @update:value="(value) => writeField(field.path, field.type, value)"
                    />

                    <div v-if="getRateLimitPreview(field)" class="plugin-settings-rate-preview">
                      <span class="plugin-settings-rate-preview__label">{{ t('config.hints.rateLimitPreview') }}</span>
                      <strong class="plugin-settings-rate-preview__value">{{ getRateLimitPreview(field) }}</strong>
                    </div>
                  </div>

                  <div v-if="field.description" class="plugin-settings-field-note">
                    <p class="plugin-settings-field-note__text">{{ field.description }}</p>
                    <a-button
                      v-if="field.defaultValue !== undefined"
                      size="small"
                      type="link"
                      class="plugin-settings-reset-default"
                      data-testid="plugin-settings-reset-default"
                      @click="resetFieldToDefault(field)"
                    >
                      {{ t('plugins.settings.resetDefault') }}
                    </a-button>
                  </div>
                </a-form-item>
              </div>
            </div>
          </section>
        </a-form>
      </section>
    </div>
  </AppPage>
</template>

<style lang="scss" scoped>
.plugin-settings-alerts-container {
  display: grid;
  gap: 12px;
}

.plugin-settings-skeleton-layout {
  display: grid;
}

.plugin-settings-layout {
  display: grid;
}

.plugin-settings-board {
  display: grid;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: var(--app-card-radius);
  background: var(--surface-strong);
  box-shadow: var(--shadow-xs);
}

.plugin-settings-board__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 18px 20px;
  border-bottom: 1px solid var(--border);
}

.plugin-settings-status-row {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  min-height: 28px;
}

.plugin-settings-status-pill {
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

.plugin-settings-status-pill--dirty {
  color: color-mix(in srgb, var(--warning) 72%, #7c2d12);
  background: color-mix(in srgb, var(--surface-warning) 86%, white);
  border: 1px solid color-mix(in srgb, var(--warning) 35%, var(--border));
}

.plugin-settings-status-pill--saved {
  color: color-mix(in srgb, var(--success) 76%, #14532d);
  background: color-mix(in srgb, var(--surface-success) 88%, white);
  border: 1px solid color-mix(in srgb, var(--success) 32%, var(--border));
}

.plugin-settings-board__title {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.plugin-settings-board__title h2 {
  margin: 0;
  color: var(--text);
  font-size: 1rem;
  font-weight: 700;
}

.plugin-settings-board__icon,
.plugin-settings-setting-row__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border-radius: 50%;
  color: var(--accent);
  background: var(--surface-accent);
  border: 1px solid var(--border-accent);
}

.plugin-settings-board__icon {
  width: 28px;
  height: 28px;
}

.plugin-settings-form-matrix {
  display: grid;
}

.plugin-settings-setting-row {
  display: grid;
  grid-template-columns: minmax(176px, 240px) minmax(0, 1fr);
  gap: 24px;
  padding: 18px 20px;
  border-top: 1px solid var(--border);
}

.plugin-settings-setting-row:first-child {
  border-top: 0;
}

.plugin-settings-setting-row__intro {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  min-width: 0;
  padding-top: 2px;
}

.plugin-settings-setting-row__icon {
  width: 26px;
  height: 26px;
}

.plugin-settings-setting-row__title h3 {
  margin: 0;
  color: var(--text);
  font-size: 0.95rem;
  font-weight: 700;
  line-height: 1.35;
}

.plugin-settings-setting-row__controls {
  display: grid;
  gap: 12px;
  min-width: 0;
}

.plugin-settings-field-item :deep(.ant-form-item) {
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

.plugin-settings-number-input {
  width: 100%;
}

.plugin-settings-prefix-select {
  width: 100%;
}

.plugin-settings-prefix-select :deep(.ant-select-selector) {
  min-height: 40px;
  align-items: flex-start;
  padding-block: 4px;
}

.plugin-settings-prefix-select :deep(.ant-select-selection-item) {
  border-radius: 8px;
  background: var(--surface-soft);
  border-color: var(--border);
  font-weight: 600;
}

.plugin-settings-control-wrap {
  display: grid;
  gap: 10px;
}

.plugin-settings-control-wrap--with-preview {
  grid-template-columns: minmax(220px, 1fr) minmax(180px, 240px);
  align-items: stretch;
}

.plugin-settings-control-wrap :deep(.ant-input),
.plugin-settings-control-wrap :deep(.ant-input-number),
.plugin-settings-control-wrap :deep(.ant-select-selector),
.plugin-settings-control-wrap :deep(.ant-input-affix-wrapper),
.plugin-settings-control-wrap :deep(textarea.ant-input) {
  border-radius: var(--radius-md);
}

.switch-wrap {
  display: flex;
  min-height: 36px;
  align-items: center;
}

.plugin-settings-field-note {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-top: 8px;
}

.plugin-settings-field-note__text {
  margin: 0;
  color: var(--muted);
  font-size: 0.82rem;
  line-height: 1.6;
}

.plugin-settings-reset-default {
  flex: 0 0 auto;
  height: auto;
  padding: 0;
  font-size: 0.82rem;
  font-weight: 650;
}

.plugin-settings-rate-preview {
  display: grid;
  align-content: center;
  gap: 3px;
  min-height: 36px;
  padding: 7px 10px;
  border-radius: var(--radius-md);
  background: var(--surface-accent);
  border: 1px solid var(--border-accent);
}

.plugin-settings-rate-preview__label {
  font-size: 0.75rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--accent);
}

.plugin-settings-rate-preview__value {
  color: var(--text);
  font-size: 0.9rem;
  line-height: 1.4;
}

@media (max-width: 860px) {
  .plugin-settings-board__header {
    align-items: flex-start;
    flex-direction: column;
  }

  .plugin-settings-setting-row {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .plugin-settings-control-wrap--with-preview {
    grid-template-columns: 1fr;
  }
}
</style>
