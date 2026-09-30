<script setup lang="ts">
import AppTagsInput from '@/components/AppTagsInput.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppField from '@/components/AppField.vue'
import AppTextarea from '@/components/AppTextarea.vue'
import AppSwitch from '@/components/AppSwitch.vue'
import AppNumberInput from '@/components/AppNumberInput.vue'
import AppInput from '@/components/AppInput.vue'
import AppButton from '@/components/AppButton.vue'
import {
  TerminalIcon,
  DatabaseIcon,
  FileTextIcon,
  MessageSquareIcon,
  ShieldCheckIcon,
  SettingsIcon,
  ImageIcon,
} from '@lucide/vue'
import { computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'

import AppSkeletonCard from '@/components/AppSkeletonCard.vue'
import ConfigSaveBar from '@/components/config/ConfigSaveBar.vue'
import RateLimitInput from '@/components/config/RateLimitInput.vue'
import RateLimitPreview from '@/components/config/RateLimitPreview.vue'
import SettingSection from '@/components/config/SettingSection.vue'
import AppPage from '@/components/page/AppPage.vue'
import { useConfigDraft } from '@/components/config/useConfigDraft'
import RetryPanel from '@/components/RetryPanel.vue'
import {
  getPluginSettingsConfigSections,
  getValueByPath,
  setValueByPath,
  type ConfigFieldDefinition,
} from '@/lib/config-form'
import { formatRateLimitPreview } from '@/lib/format'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'

const configStore = useConfigStore()
const { error, loading, saving } = storeToRefs(configStore)

const { draft, saveStatusLabel, loadConfig, hasUnsavedChanges, canSave, markDraftChanged, readField, writeField, save } = useConfigDraft()

function readNumberField(path: string) {
  const value = readField(path, 'number')
  return typeof value === 'number' ? value : null
}
function readSelectField(path: string) {
  const value = readField(path, 'select')
  return typeof value === 'boolean' ? value : String(value ?? '')
}
const configSections = computed(() => getPluginSettingsConfigSections().map(section => ({
  ...section,
  fields: section.fields.map(field => ({ ...field, rateLimitPreview: field.type === 'rateLimit'
    ? formatRateLimitPreview(readField(field.path, field.type)) : null })),
})))

onMounted(() => {
  void loadConfig()
})

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

// "恢复默认" only acts while the draft differs from the default value.
function isAtDefault(field: ConfigFieldDefinition) {
  if (!draft.value || field.defaultValue === undefined) return true
  return JSON.stringify(getValueByPath(draft.value as unknown as Record<string, unknown>, field.path) ?? null) === JSON.stringify(field.defaultValue)
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
      return TerminalIcon
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

</script>

<template>
  <AppPage :title="t('plugins.settings.title')" :description="t('plugins.settings.subtitle')" width="form">
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

    <div v-else-if="draft" class="plugin-settings-layout app-box">
      <section class="plugin-settings-board" :aria-label="t('plugins.settings.title')">
        <div class="plugin-settings-form-matrix">
          <SettingSection
            v-for="section in configSections"
            :key="section.key"
            :title="section.title"
            :icon="getSectionIcon(section.key)"
          >
              <div v-for="field in section.fields" :key="field.path" class="plugin-settings-field-item">
                <AppField :floating="['text', 'number', 'select', 'textarea', 'list'].includes(field.type) && !isCommandPrefixField(field.path)" :label="field.label">
                  <template v-if="!(['text', 'number', 'select', 'textarea', 'list'].includes(field.type) && !isCommandPrefixField(field.path))" #label>
                    <div class="field-label-wrap">
                      <span class="field-label-text">{{ field.label }}</span>
                    </div>
                  </template>

                  <div class="plugin-settings-control-wrap" :class="{ 'plugin-settings-control-wrap--with-preview': field.rateLimitPreview }">
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

                    <RateLimitPreview :text="field.rateLimitPreview" class="plugin-settings-rate-preview" />
                  </div>

                  <div v-if="field.description" class="plugin-settings-field-note">
                    <p class="plugin-settings-field-note__text">{{ field.description }}</p>
                    <AppButton
                      v-if="field.defaultValue !== undefined"
                      size="sm"
                      variant="link"
                      class="plugin-settings-reset-default"
                      data-testid="plugin-settings-reset-default"
                      :disabled="isAtDefault(field)"
                      @click="resetFieldToDefault(field)"
                    >
                      {{ t('plugins.settings.resetDefault') }}
                    </AppButton>
                  </div>
                </AppField>
              </div>
          </SettingSection>
        </div>
      </section>
      <ConfigSaveBar
        test-id-prefix="plugin-settings"
        :dirty="hasUnsavedChanges"
        :saved-label="saveStatusLabel"
        :can-save="canSave"
        :saving="saving"
        @save="save"
      />
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

.plugin-settings-board {
  display: grid;
}

.plugin-settings-form-matrix {
  display: grid;
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
</style>
