<script setup lang="ts">
import AppTextarea from '@/components/AppTextarea.vue'
import AppTagsInput from '@/components/AppTagsInput.vue'
import AppSwitch from '@/components/AppSwitch.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppNumberInput from '@/components/AppNumberInput.vue'
import AppInput from '@/components/AppInput.vue'
import AppButton from '@/components/AppButton.vue'
import {
  ArrowUpRightIcon,
  ShieldCheckIcon,
  UserStarIcon,
} from '@lucide/vue'
import { computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'

import AppPage from '@/components/page/AppPage.vue'
import AppSkeletonCard from '@/components/AppSkeletonCard.vue'
import ConfigSaveBar from '@/components/config/ConfigSaveBar.vue'
import SettingSection from '@/components/config/SettingSection.vue'
import { useConfigDraft } from '@/components/config/useConfigDraft'
import RetryPanel from '@/components/RetryPanel.vue'
import {
  getPermissionPolicyConfigSections,
  getValueByPath,
  setValueByPath,
  type ConfigFieldDefinition,
} from '@/lib/config-form'
import { buildAccessListsLocation } from '@/lib/management-links'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'
import { useGovernanceStore } from '@/stores/governance'
import { useMotionNavigation } from '@/motion/useMotionNavigation'

const navigate = useMotionNavigation()
const configStore = useConfigStore()
const governanceStore = useGovernanceStore()

const {
  error: configError,
  loading: configLoading,
  saving,
} = storeToRefs(configStore)
const {
  commandPolicyError,
  commandPolicyLoading,
} = storeToRefs(governanceStore)

const { draft, saveStatusLabel, loadConfig, hasUnsavedChanges, canSave, markDraftChanged, readField, writeField, save: saveDraft } = useConfigDraft({ error: () => configError.value || commandPolicyError.value })

const configSections = computed(() => getPermissionPolicyConfigSections())
const pageBusy = computed(() => configLoading.value || commandPolicyLoading.value)
const pageError = computed(() => configError.value || commandPolicyError.value)
const showFatalError = computed(() => Boolean(configError.value) && !draft.value)

function getSectionIcon(key: string) {
  return key === 'admin' ? UserStarIcon : ShieldCheckIcon
}

async function loadPage() {
  try {
    await Promise.all([
      loadConfig(),
      governanceStore.fetchCommandPolicy(),
    ])
  } catch {
    // store state drives the page
  }
}

onMounted(() => {
  void loadPage()
})

function normalizeTagList(value: unknown) {
  const source = Array.isArray(value) ? value : [value]
  return source
    .flatMap((item) => String(item).split(/[\s,，;；]+/))
    .map((item) => item.trim())
    .filter(Boolean)
}

function readSuperAdminTags() {
  if (!draft.value) {
    return []
  }

  const current = getValueByPath(draft.value as unknown as Record<string, unknown>, 'admin.super_admins')
  return Array.isArray(current) ? normalizeTagList(current) : []
}

function writeSuperAdminTags(value: unknown) {
  if (!draft.value) {
    return
  }

  markDraftChanged()
  setValueByPath(draft.value as unknown as Record<string, unknown>, 'admin.super_admins', normalizeTagList(value))
}

function isSuperAdminField(path: string) {
  return path === 'admin.super_admins'
}

function readNumberField(path: string, type: ConfigFieldDefinition['type']) {
  const value = readField(path, type)
  return typeof value === 'number' ? value : null
}

function readSelectField(path: string, type: ConfigFieldDefinition['type']) {
  const value = readField(path, type)
  return typeof value === 'boolean' ? value : String(value ?? '')
}

async function save() {
  if (!await saveDraft()) return
  try { await governanceStore.fetchCommandPolicy() } catch { /* store state drives the page */ }
}
</script>

<template>
  <AppPage :title="t('permissionPolicy.title')" :description="t('permissionPolicy.subtitle')" width="form">
    <template #extra>
      <div class="table-actions">
        <AppButton data-testid="permission-policy-open-access-lists" @click="navigate(buildAccessListsLocation())">
          {{ t('permissionPolicy.actions.openAccessLists') }}
          <ArrowUpRightIcon class="permission-policy-page__arrow" aria-hidden="true" />
        </AppButton>
      </div>
    </template>

    <RetryPanel
      v-if="showFatalError"
      :title="t('permissionPolicy.title')"
      :description="pageError ?? t('errors.common.loadFailed')"
      :loading="pageBusy"
      @retry="loadPage"
    />

    <AppSkeletonCard v-else-if="!draft" show-header :rows="4" />

    <!-- A section with a single field is named by its section title; the control keeps the field name for assistive technology. -->
    <div v-else class="permission-policy-form app-box">
      <SettingSection
        v-for="section in configSections"
        :key="section.key"
        :title="section.title"
        :icon="getSectionIcon(section.key)"
      >
        <div v-for="field in section.fields" :key="field.path" class="permission-policy-field">
          <span v-if="section.fields.length > 1" class="permission-policy-field__label">{{ field.label }}</span>
          <AppTagsInput
            v-if="isSuperAdminField(field.path)"
            class="super-admin-tag-select"
            data-testid="permission-policy-super-admins"
            :model-value="readSuperAdminTags()"
            :aria-label="field.label"
            :placeholder="t('permissionPolicy.placeholders.superAdmins')"
            :separators="[',', '，', ' ', '\n']"
            @update:model-value="writeSuperAdminTags"
          />

          <AppInput
            v-else-if="field.type === 'text'"
            :model-value="String(readField(field.path, field.type) ?? '')"
            :aria-label="field.label"
            @update:model-value="writeField(field.path, field.type, $event)"
          />

          <AppNumberInput nullable
            v-else-if="field.type === 'number'"
            class="permission-policy-number-input"
            :model-value="readNumberField(field.path, field.type)"
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
            wrapper-class="permission-policy-select"
            :model-value="readSelectField(field.path, field.type)"
            :options="field.options || []"
            :aria-label="field.label"
            @update:model-value="writeField(field.path, field.type, $event)"
          />

          <AppTextarea
            v-else
            :model-value="String(readField(field.path, field.type) ?? '')"
            :rows="4" :max-rows="8"
            :aria-label="field.label"
            @update:model-value="writeField(field.path, field.type, $event)"
          />

          <p v-if="field.description" class="permission-policy-field__note">{{ field.description }}</p>
        </div>
      </SettingSection>

      <ConfigSaveBar
        test-id-prefix="permission-policy"
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
.permission-policy-page__arrow {
  width: 14px;
  height: 14px;
  color: var(--muted);
}

.permission-policy-form {
  display: grid;
}

.permission-policy-field {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.permission-policy-field__label {
  color: var(--text);
  font-size: 14px;
  font-weight: 500;
}

.permission-policy-field__note {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.6;
}

.super-admin-tag-select {
  width: 100%;
}

.super-admin-tag-select :deep(.app-select) {
  min-height: 40px;
  align-items: flex-start;
  padding-block: 4px;
}

.super-admin-tag-select :deep(.app-tags-input__item) {
  border-radius: 8px;
  background: var(--surface-soft);
  border-color: var(--border);
  font-weight: 600;
}

:deep(.permission-policy-select) {
  max-width: 320px;
}

.permission-policy-number-input {
  width: 100%;
}

.switch-wrap {
  display: flex;
  min-height: 36px;
  align-items: center;
}
</style>
