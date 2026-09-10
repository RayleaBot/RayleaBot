<script setup lang="ts">
import AppTagsInput from '@/components/AppTagsInput.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppField from '@/components/AppField.vue'
import AppTooltip from '@/components/AppTooltip.vue'
import AppTextarea from '@/components/AppTextarea.vue'
import AppSwitch from '@/components/AppSwitch.vue'
import AppNumberInput from '@/components/AppNumberInput.vue'
import AppInput from '@/components/AppInput.vue'
import AppButton from '@/components/AppButton.vue'
import {
  PlugZapIcon,
  CircleCheckIcon,
  DatabaseIcon,
  CircleAlertIcon,
  FileTextIcon,
  MessageSquareIcon,
  ShieldCheckIcon,
  SaveIcon,
  SettingsIcon,
  ImageIcon,
} from '@lucide/vue'
import { computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'

import { useToastFeedback } from '@/adapter/feedback'
import AppSkeletonCard from '@/components/AppSkeletonCard.vue'
import RateLimitInput from '@/components/config/RateLimitInput.vue'
import AppPage from '@/components/page/AppPage.vue'
import { useConfigDraft } from '@/components/config/useConfigDraft'
import RetryPanel from '@/components/RetryPanel.vue'
import {
  getPluginSettingsConfigSections,
  getValueByPath,
  setValueByPath,
  type ConfigFieldDefinition,
} from '@/lib/config-form'
import { formatRateLimit,  } from '@/lib/format'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'

const configStore = useConfigStore()
const { error, loading, redactedFields, saving } = storeToRefs(configStore)

const { draft, saveStatus, hasUnsavedChanges, canSave, markDraftChanged, readField, writeField, save } = useConfigDraft()

function readNumberField(path: string) {
  const value = readField(path, 'number')
  return typeof value === 'number' ? value : null
}
function readSelectField(path: string) {
  const value = readField(path, 'select')
  return typeof value === 'boolean' ? value : String(value ?? '')
}
const configSections = computed(() => getPluginSettingsConfigSections())

const feedbackToast = computed(() => {
  if (error.value) {
    return {
      key: `plugin-settings-error:${error.value}`,
      level: 'error' as const,
      message: error.value,
    }
  }

  if (redactedFields.value.length > 0) {
    return {
      key: `plugin-settings-redacted:${redactedFields.value.join('|')}`,
      level: 'info' as const,
      message: `${t('config.redactedTitle')}：${redactedFields.value.join(', ')}`,
    }
  }

  return null
})
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

useToastFeedback(feedbackToast)


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
      return PlugZapIcon
    case 'permission':
      return ShieldCheckIcon
    case 'log':
      return FileTextIcon
    case 'message':
      return MessageSquareIcon
    case 'render':
      return ImageIcon
    case 'storage':
      return DatabaseIcon
    default:
      return SettingsIcon
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

</script>

<template>
  <AppPage :title="t('plugins.settings.title')" :show-header="false" width="form">
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
        <div class="plugin-settings-form-matrix">
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
                <AppField :floating="['text', 'number', 'select', 'textarea', 'list'].includes(field.type) && !isCommandPrefixField(field.path)" :label="field.label">
                  <template v-if="!(['text', 'number', 'select', 'textarea', 'list'].includes(field.type) && !isCommandPrefixField(field.path))" #label>
                    <div class="field-label-wrap">
                      <span class="field-label-text">{{ field.label }}</span>
                      <AppTooltip v-if="field.description" :title="field.description">
                        <button type="button" class="field-info-icon" :aria-label="t('config.fieldHelp')">?</button>
                      </AppTooltip>
                    </div>
                  </template>

                  <div class="plugin-settings-control-wrap" :class="{ 'plugin-settings-control-wrap--with-preview': getRateLimitPreview(field) }">
                    <AppTagsInput
                      v-if="isCommandPrefixField(field.path)"

                      class="plugin-settings-prefix-select"
                      data-testid="plugin-settings-command-prefixes"
                      :model-value="readCommandPrefixTags()"
                      :aria-label="field.label"
                      :placeholder="t('plugins.settings.placeholders.commandPrefixes')"
                      @update:model-value="writeCommandPrefixTags"
                    />

                    <RateLimitInput
                      v-else-if="field.type === 'rateLimit'"
                      :value="String(readField(field.path, field.type) ?? '')"
                      :ariaLabel="field.label"
                      @update:value="writeField(field.path, field.type, $event)"
                    />

                    <AppInput
                      v-else-if="field.type === 'text'"
                      :model-value="String(readField(field.path, field.type) ?? '')"
                      :aria-label="field.label"
                      @update:model-value="writeField(field.path, field.type, $event)"
                    />

                    <AppNumberInput nullable
                      v-else-if="field.type === 'number'"
                      class="plugin-settings-number-input"
                      :model-value="readNumberField(field.path)"
                      :min="0"
                      :step="1"
                      :aria-label="field.label"
                      @update:model-value="writeField(field.path, field.type, $event)"
                    />

                    <div v-else-if="field.type === 'boolean'" class="switch-wrap">
                      <AppSwitch
                        :model-value="Boolean(readField(field.path, field.type))"
                        :aria-label="field.label"
                        @update:model-value="writeField(field.path, field.type, $event)"
                      />
                    </div>

                    <AppSelect
                      v-else-if="field.type === 'select'"
                      :model-value="readSelectField(field.path)"
                      :options="field.options || []"
                      :aria-label="field.label"
                      @update:model-value="writeField(field.path, field.type, $event)"
                    />

                    <AppTextarea
                      v-else-if="field.type === 'textarea'"
                      :model-value="String(readField(field.path, field.type) ?? '')"
                      :rows="3" :max-rows="7"
                      :aria-label="field.label"
                      @update:model-value="writeField(field.path, field.type, $event)"
                    />

                    <AppTextarea
                      v-else
                      :model-value="String(readField(field.path, field.type) ?? '')"
                      :rows="3" :max-rows="7"
                      :aria-label="field.label"
                      @update:model-value="writeField(field.path, field.type, $event)"
                    />

                    <div v-if="getRateLimitPreview(field)" class="plugin-settings-rate-preview">
                      <span class="plugin-settings-rate-preview__label">{{ t('config.hints.rateLimitPreview') }}</span>
                      <strong class="plugin-settings-rate-preview__value">{{ getRateLimitPreview(field) }}</strong>
                    </div>
                  </div>

                  <div v-if="field.description" class="plugin-settings-field-note">
                    <p class="plugin-settings-field-note__text">{{ field.description }}</p>
                    <AppButton
                      v-if="field.defaultValue !== undefined"
                      size="sm"
                      variant="link"
                      class="plugin-settings-reset-default"
                      data-testid="plugin-settings-reset-default"
                      @click="resetFieldToDefault(field)"
                    >
                      {{ t('plugins.settings.resetDefault') }}
                    </AppButton>
                  </div>
                </AppField>
              </div>
            </div>
          </section>
        </div>
      </section>
      <footer class="plugin-settings-save-bar">
        <div class="plugin-settings-status-row" aria-live="polite">
          <span v-if="hasUnsavedChanges" class="plugin-settings-status-pill plugin-settings-status-pill--dirty" data-testid="plugin-settings-unsaved-status">
            <CircleAlertIcon />{{ t('plugins.settings.status.unsaved') }}
          </span>
          <span v-else-if="saveStatus" class="plugin-settings-status-pill plugin-settings-status-pill--saved" data-testid="plugin-settings-save-status">
            <CircleCheckIcon />{{ saveStatusLabel }}
          </span>
        </div>
        <AppButton variant="default" :disabled="!canSave" :loading="saving" :aria-label="t('config.save')" data-testid="plugin-settings-save" @click="save">
          <template #icon><SaveIcon /></template>
          {{ t('config.save') }}
        </AppButton>
      </footer>
    </div>
  </AppPage>
</template>

<style lang="scss" scoped>
.plugin-settings-skeleton-layout {
  display: grid;
}

.plugin-settings-layout {
  display: grid;
  width: 100%;
}

.plugin-settings-save-bar {
  position: sticky;
  bottom: 0;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 64px;
  padding: 12px 20px;
  border: 1px solid var(--border);
  margin-top: -1px;
  border-radius: 0 0 var(--app-card-radius) var(--app-card-radius);
  background: var(--surface);
}

.plugin-settings-save-bar .app-button { flex: 0 0 auto; }
@media (max-width: 639px) {
  .plugin-settings-save-bar { padding: 10px 12px max(10px, env(safe-area-inset-bottom)); }
  .plugin-settings-save-bar .app-button { min-height: 44px; }
}

.plugin-settings-board {
  display: grid;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: var(--app-card-radius) var(--app-card-radius) 0 0;
  background: var(--surface-strong);
  box-shadow: none;
}

.plugin-settings-status-row {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  min-width: 0;
  min-height: 28px;
}

.plugin-settings-status-pill {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-height: 28px;
  padding: 4px 10px;
  border-radius: var(--radius-sm);
  font-size: 13px;
  font-weight: 500;
  line-height: 1;
  box-shadow: none;
}

.plugin-settings-status-pill--dirty {
  color: var(--text-attention);
  background: var(--surface-attention);
  border: 1px solid var(--border-attention);
}

.plugin-settings-status-pill--saved {
  color: var(--success);
  background: var(--surface-success);
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
  font-weight: 600;
}

.plugin-settings-board__icon,
.plugin-settings-setting-row__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: var(--muted);
  background: transparent;
  border: 0;
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
  grid-template-columns: minmax(150px, 190px) minmax(0, 1fr);
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
  font-size: 15px;
  font-weight: 600;
  line-height: 1.35;
}

.plugin-settings-setting-row__controls {
  display: grid;
  gap: 12px;
  min-width: 0;
}

.plugin-settings-field-item :deep(.app-field) {
  margin-bottom: 0;
}

.field-label-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
}

.field-label-text {
  font-weight: 500;
  font-size: 14px;
  color: var(--theme-text, var(--text));
}

.field-info-icon {
  appearance: none;
  background: transparent;
  color: var(--muted);
  cursor: help;
  font-size: 13px;
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
  outline-offset: var(--focus-outline-offset);
}

.plugin-settings-number-input {
  width: 100%;
}

.plugin-settings-prefix-select {
  width: 100%;
}

.plugin-settings-prefix-select :deep(.app-select) {
  min-height: 40px;
  align-items: flex-start;
  padding-block: 4px;
}

.plugin-settings-prefix-select :deep(.app-tags-input__item) {
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

.plugin-settings-control-wrap :deep(.app-input),
.plugin-settings-control-wrap :deep(.app-number-input),
.plugin-settings-control-wrap :deep(.app-select),
.plugin-settings-control-wrap :deep(.app-input-wrap),
.plugin-settings-control-wrap :deep(textarea.app-textarea) {
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
  font-size: 13px;
  letter-spacing: 0;
  color: var(--accent);
}

.plugin-settings-rate-preview__value {
  color: var(--text);
  font-size: 0.9rem;
  line-height: 1.4;
}

@media (max-width: 860px) {
  .plugin-settings-setting-row {
    grid-template-columns: 1fr;
    gap: 12px;
  }

  .plugin-settings-control-wrap--with-preview {
    grid-template-columns: 1fr;
  }
}
</style>
