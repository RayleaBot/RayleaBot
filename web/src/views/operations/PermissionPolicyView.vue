<script setup lang="ts">
import AppTextarea from '@/components/AppTextarea.vue'
import AppTagsInput from '@/components/AppTagsInput.vue'
import AppSwitch from '@/components/AppSwitch.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppNumberInput from '@/components/AppNumberInput.vue'
import AppInput from '@/components/AppInput.vue'
import AppField from '@/components/AppField.vue'
import AppCard from '@/components/AppCard.vue'
import AppButton from '@/components/AppButton.vue'
import {
  CircleAlertIcon,
  ShieldCheckIcon,
  ShieldIcon,
  SaveIcon,
  CircleCheckIcon,
  UsersIcon,
  UserRoundPlusIcon,
} from '@lucide/vue'
import { computed, onMounted } from 'vue'
import { storeToRefs } from 'pinia'

import { useToastFeedback } from '@/adapter/feedback'
import AppPage from '@/components/page/AppPage.vue'
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
  redactedFields,
  saving,
} = storeToRefs(configStore)
const {
  commandPolicyError,
  commandPolicyLoading,
} = storeToRefs(governanceStore)

const { draft, saveStatus, hasUnsavedChanges, canSave, markDraftChanged, readField, writeField, save: saveDraft } = useConfigDraft()

const configSections = computed(() => getPermissionPolicyConfigSections())
const pageBusy = computed(() => configLoading.value || commandPolicyLoading.value)
const pageError = computed(() => configError.value || commandPolicyError.value)
const showFatalError = computed(() => Boolean(configError.value) && !draft.value)

const saveStatusLabel = computed(() => {
  switch (saveStatus.value) {
    case 'restart':
      return t('permissionPolicy.status.savedRestart')
    case 'hot':
      return t('permissionPolicy.status.savedHot')
    default:
      return ''
  }
})
const feedbackToast = computed(() => {
  if (pageError.value) {
    return {
      key: `permission-policy-error:${pageError.value}`,
      level: 'error' as const,
      message: pageError.value,
    }
  }

  if (redactedFields.value.length > 0) {
    return {
      key: `permission-policy-redacted:${redactedFields.value.join('|')}`,
      level: 'info' as const,
      message: `${t('config.redactedTitle')}：${redactedFields.value.join(', ')}`,
    }
  }

  return null
})



function getSectionIcon(key: string) {
  switch (key) {
    case 'admin':
      return UserRoundPlusIcon
    case 'permission':
      return ShieldIcon
    case 'group':
      return UsersIcon
    default:
      return ShieldCheckIcon
  }
}

async function loadPage() {
  try {
    await Promise.all([
      configStore.fetchConfig(),
      governanceStore.fetchCommandPolicy(),
    ])
  } catch {
    // store state drives the page
  }
}

onMounted(() => {
  void loadPage()
})

useToastFeedback(feedbackToast)


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
  <AppPage :title="t('permissionPolicy.title')" width="form">
    <template #extra>
      <div class="table-actions permission-policy-actions">
        <AppButton data-testid="permission-policy-open-access-lists" @click="navigate(buildAccessListsLocation())">
          <template #icon>
            <UsersIcon />
          </template>
          {{ t('permissionPolicy.actions.openAccessLists') }}
        </AppButton>
        <AppButton
          variant="default"
          data-testid="permission-policy-save"
          :disabled="!canSave"
          :loading="saving"
          @click="save"
        >
          <template #icon>
            <SaveIcon />
          </template>
          {{ t('config.save') }}
        </AppButton>
      </div>
    </template>

    <div class="permission-policy-page">
      <RetryPanel
        v-if="showFatalError"
        :title="t('permissionPolicy.title')"
        :description="pageError ?? t('errors.common.loadFailed')"
        :loading="pageBusy"
        @retry="loadPage"
      />

      <template v-else>
        <section class="permission-policy-settings-section">
          <div class="permission-policy-settings-header">
            <h2>{{ t('permissionPolicy.sections.settings') }}</h2>
            <div class="permission-policy-status-row" aria-live="polite">
              <span
                v-if="hasUnsavedChanges"
                class="permission-policy-status-pill permission-policy-status-pill--dirty"
                data-testid="permission-policy-unsaved-status"
              >
                <CircleAlertIcon />
                {{ t('permissionPolicy.status.unsaved') }}
              </span>
              <span
                v-else-if="saveStatus"
                class="permission-policy-status-pill permission-policy-status-pill--saved"
                data-testid="permission-policy-save-status"
              >
                <CircleCheckIcon />
                {{ saveStatusLabel }}
              </span>
            </div>
          </div>

          <div v-if="draft" class="permission-policy-settings-layout">
            <AppCard v-for="section in configSections" :key="section.title" borderless class="permission-policy-config-card">
              <div class="card-header config-card-header">
                <div class="permission-policy-config-card__title">
                  <span class="permission-policy-config-card__icon">
                    <component :is="getSectionIcon(section.key)" :size="16" />
                  </span>
                  <strong>{{ section.title }}</strong>
                </div>
                <span class="field-count-badge">{{ section.fields.length }} {{ t('config.fieldCount') }}</span>
              </div>

              <div class="permission-policy-settings-form">
                <div v-for="field in section.fields" :key="field.path" class="config-field-item">
                  <AppField label="">
                    <template #label>
                      <span class="field-label-text">{{ field.label }}</span>
                    </template>

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
                      class="config-number-input"
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

                    <div v-if="field.description" class="config-field-note">
                      <p v-if="field.description" class="config-field-note__text">{{ field.description }}</p>
                    </div>
                  </AppField>
                </div>
              </div>
            </AppCard>
          </div>
        </section>
      </template>
    </div>
  </AppPage>
</template>

<style lang="scss" scoped>
.permission-policy-page {
  display: grid;
  gap: 22px;
}

.permission-policy-actions :deep(.app-button) {
  min-height: 36px;
  padding-inline: 14px;
  border-radius: var(--radius-md);
}

.permission-policy-settings-section {
  display: grid;
  gap: 12px;
}

.permission-policy-settings-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.permission-policy-settings-header h2 {
  margin: 0;
  font-size: 1rem;
  font-weight: 700;
  color: var(--text);
}

.permission-policy-status-row {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  min-height: 28px;
}

.permission-policy-status-pill {
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

.permission-policy-status-pill--dirty {
  color: var(--text-attention);
  background: var(--surface-attention);
  border: 1px solid var(--border-attention);
}

.permission-policy-status-pill--saved {
  color: var(--success);
  background: var(--surface-success);
  border: 1px solid color-mix(in srgb, var(--success) 32%, var(--border));
}

.permission-policy-settings-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 0;
  align-items: stretch;
}

.permission-policy-config-card {
  overflow: hidden;
  border: 0;
  border-top: 1px solid var(--border);
  border-radius: 0;
  background: transparent;
  box-shadow: none;
  box-shadow: var(--shadow-xs);
}

.permission-policy-config-card :deep(.app-card__body) {
  padding: 0;
}

.config-card-header {
  padding: 18px 20px 4px;
  background: transparent;
  border-bottom: 0;
}

.permission-policy-config-card__title {
  display: inline-flex;
  align-items: center;
  min-width: 0;
  gap: 10px;
}

.permission-policy-config-card__icon {
  display: inline-flex;
  width: 26px;
  height: 26px;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border-radius: 50%;
  color: var(--accent);
  background: var(--surface-accent);
  border: 1px solid var(--border-accent);
}

.field-count-badge {
  color: var(--muted);
  font-size: 13px;
  font-weight: 500;
}

.permission-policy-settings-form {
  display: grid;
  gap: 12px;
  padding: 14px 20px 20px;
}

.config-field-item :deep(.app-field) {
  margin-bottom: 0;
}

.config-field-item :deep(.app-input),
.config-field-item :deep(.app-select) {
  border-radius: var(--radius-md);
}

.super-admin-tag-select {
  width: 100%;
}

.super-admin-tag-select :deep(.app-select) {
  min-height: 44px;
  align-items: flex-start;
  padding-block: 5px;
}

.super-admin-tag-select :deep(.app-tags-input__item) {
  border-radius: 8px;
  background: var(--surface-soft);
  border-color: var(--border);
  font-weight: 600;
}

.field-label-text {
  font-weight: 600;
  font-size: 0.85rem;
  color: var(--theme-text, var(--text));
}

.config-number-input {
  width: 100%;
}

.switch-wrap {
  display: flex;
  min-height: 36px;
  align-items: center;
}

.config-field-note {
  display: grid;
  gap: 8px;
  margin-top: 10px;
}

.config-field-note__text {
  margin: 0;
  color: var(--muted);
  font-size: 0.82rem;
  line-height: 1.6;
}

@media (max-width: 768px) {
  .permission-policy-actions {
    justify-content: flex-start;
  }

  .permission-policy-settings-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .permission-policy-status-row {
    justify-content: flex-start;
  }
}

@media (max-width: 1180px) {
  .permission-policy-settings-layout {
    grid-template-columns: 1fr;
  }
}
</style>
