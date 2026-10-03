<script setup lang="ts">
import AppSwitch from '@/components/AppSwitch.vue'
import {
  HourglassIcon,
  SendIcon,
  UserIcon,
  UsersIcon,
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
  getRateLimitConfigSections,
  type ConfigFieldDefinition,
} from '@/lib/config-form'
import { formatRateLimitPreview } from '@/lib/format'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'

const configStore = useConfigStore()
const { error, loading, saving } = storeToRefs(configStore)

const { draft, saveStatusLabel, loadConfig, hasUnsavedChanges, canSave, readField, writeField, save } = useConfigDraft()

const configSections = computed(() => getRateLimitConfigSections().map(section => ({
  ...section,
  fields: section.fields.map(field => ({ ...field, rateLimitPreview: field.type === 'rateLimit'
    ? formatRateLimitPreview(readField(field.path, field.type)) : null })),
})))

onMounted(() => {
  void loadConfig()
})

function isFieldDisabled(field: ConfigFieldDefinition) {
  return field.enabledBy !== undefined && !readField(field.enabledBy, 'boolean')
}

function getSectionIcon(key: string) {
  switch (key) {
    case 'user':
      return UserIcon
    case 'group':
      return UsersIcon
    case 'cooldown-reply':
      return HourglassIcon
    default:
      return SendIcon
  }
}

</script>

<template>
  <AppPage :title="t('rateLimits.title')" :description="t('rateLimits.subtitle')" width="form">
    <RetryPanel
      v-if="error && !draft"
      :title="t('rateLimits.title')"
      :description="error"
      :loading="loading"
      @retry="loadConfig"
    />

    <AppSkeletonCard v-else-if="!draft" show-header :rows="5" />

    <!-- A section with a single field is named by its section title; the control keeps the field name for assistive technology. -->
    <div v-else class="rate-limits-form app-box">
      <SettingSection
        v-for="section in configSections"
        :key="section.key"
        :title="section.title"
        :icon="getSectionIcon(section.key)"
      >
        <div v-for="field in section.fields" :key="field.path" class="rate-limits-field">
          <span v-if="section.fields.length > 1 && field.type !== 'boolean'" class="rate-limits-field__label">{{ field.label }}</span>

          <div v-if="field.type === 'rateLimit'" class="rate-limits-field__rate">
            <RateLimitInput
              :value="String(readField(field.path, field.type) ?? '')"
              :ariaLabel="field.label"
              @update:value="writeField(field.path, field.type, $event)"
            />
            <RateLimitPreview :text="field.rateLimitPreview" class="rate-limits-field__preview" />
          </div>

          <label
            v-else-if="field.type === 'boolean'"
            class="rate-limits-field__switch"
            :class="{ 'rate-limits-field__switch--disabled': isFieldDisabled(field) }"
          >
            <AppSwitch
              :model-value="Boolean(readField(field.path, field.type))"
              :aria-label="field.label"
              :disabled="isFieldDisabled(field)"
              @update:model-value="writeField(field.path, field.type, $event)"
            />
            <span>{{ field.label }}</span>
          </label>

          <p v-if="field.description" class="rate-limits-field__note">{{ field.description }}</p>
        </div>
      </SettingSection>

      <ConfigSaveBar
        test-id-prefix="rate-limits"
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
.rate-limits-form {
  display: grid;
}

.rate-limits-field {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.rate-limits-field__label {
  color: var(--text);
  font-size: 14px;
  font-weight: 500;
}

// Three short numbers do not need the full column; the reading of the value sits beside them.
.rate-limits-field__rate {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 8px 20px;
}

.rate-limits-field__rate > :first-child {
  flex: 0 1 480px;
  min-width: 0;
}

.rate-limits-field__preview {
  padding-bottom: 9px;
}

.rate-limits-field__switch {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  width: fit-content;
  min-height: 32px;
  color: var(--text);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
}

.rate-limits-field__switch--disabled {
  color: var(--muted);
  cursor: not-allowed;
}

.rate-limits-field__note {
  max-width: 72ch;
  margin: 0;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.6;
}
</style>
