<script setup lang="ts">
import {
  ExclamationCircleOutlined,
  SafetyCertificateOutlined,
  SafetyOutlined,
  SaveOutlined,
  CheckCircleOutlined,
  TeamOutlined,
  UserAddOutlined,
} from '@ant-design/icons-vue'
import { computed, onBeforeUnmount, onDeactivated, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import { notifySuccess, useToastFeedback } from '@/adapter/feedback'
import AppEmptyState from '@/components/AppEmptyState.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import {
  cloneConfig,
  getPermissionPolicyConfigSections,
  getValueByPath,
  setValueByPath,
  type ConfigFieldDefinition,
} from '@/lib/config-form'
import { fromMultilineList, toMultilineList } from '@/lib/format'
import { buildAccessListsLocation } from '@/lib/management-links'
import { t } from '@/i18n'
import { useConfigStore } from '@/stores/config'
import { useGovernanceStore } from '@/stores/governance'
import type { CommandPermissionLevel, ConfigDocument } from '@/types/api'
import { useMotionNavigation } from '@/motion/useMotionNavigation'

const navigate = useMotionNavigation()
const configStore = useConfigStore()
const governanceStore = useGovernanceStore()

const {
  document,
  error: configError,
  loading: configLoading,
  redactedFields,
  saving,
} = storeToRefs(configStore)
const {
  commandPolicy,
  commandPolicyError,
  commandPolicyLoading,
} = storeToRefs(governanceStore)

const draft = ref<ConfigDocument | null>(null)
const saveStatus = ref<'hot' | 'restart' | null>(null)
let saveStatusTimer: number | null = null

const configSections = computed(() => getPermissionPolicyConfigSections())
const pageBusy = computed(() => configLoading.value || commandPolicyLoading.value)
const pageError = computed(() => configError.value || commandPolicyError.value)
const showFatalError = computed(() => Boolean(pageError.value) && !draft.value && !commandPolicy.value)
const superAdminCount = computed(() => document.value?.admin.super_admins.length ?? 0)
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

watch(document, (value) => {
  draft.value = value ? cloneConfig(value) : null
}, { immediate: true })

const summaryCards = computed(() => [
  {
    key: 'super-admins',
    icon: SafetyCertificateOutlined,
    label: t('permissionPolicy.summary.superAdmins'),
    tone: superAdminCount.value > 0 ? 'success' : 'warning' as const,
    value: String(superAdminCount.value),
    description: t('permissionPolicy.summary.superAdminsMeta'),
  },
  {
    key: 'default-permission',
    icon: SafetyOutlined,
    label: t('permissionPolicy.summary.defaultPermission'),
    tone: 'primary' as const,
    value: getCommandPermissionLabel(commandPolicy.value?.default_level),
    description: t('permissionPolicy.summary.defaultPermissionMeta'),
  },
])

function getCommandPermissionLabel(level: CommandPermissionLevel | null | undefined) {
  switch (level) {
    case 'everyone':
      return t('commands.permissions.everyone')
    case 'group_admin':
      return t('commands.permissions.groupAdmin')
    case 'super_admin':
      return t('commands.permissions.superAdmin')
    default:
      return t('display.empty')
  }
}

function getSectionIcon(key: string) {
  switch (key) {
    case 'admin':
      return UserAddOutlined
    case 'permission':
      return SafetyOutlined
    case 'group':
      return TeamOutlined
    default:
      return SafetyCertificateOutlined
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

function showSaveStatus(restartRequired: boolean) {
  clearSaveStatus()
  saveStatus.value = restartRequired ? 'restart' : 'hot'
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
    normalized = Array.isArray(value) ? normalizeTagList(value) : fromMultilineList(String(value))
  }

  markDraftChanged()
  setValueByPath(draft.value as unknown as Record<string, unknown>, path, normalized)
}

async function save() {
  if (!draft.value || !hasUnsavedChanges.value) {
    return
  }

  const response = await configStore.saveConfig(draft.value)
  try {
    await governanceStore.fetchCommandPolicy()
  } catch {
    // store state drives the page
  }
  showSaveStatus(response.restart_required)
  notifySuccess(response.restart_required ? t('config.saveRestart') : t('config.saveSuccess'))
}
</script>

<template>
  <AppPage :title="t('permissionPolicy.title')" width="form">
    <template #extra>
      <div class="table-actions permission-policy-actions">
        <a-button data-testid="permission-policy-open-access-lists" @click="navigate(buildAccessListsLocation())">
          <template #icon>
            <TeamOutlined />
          </template>
          {{ t('permissionPolicy.actions.openAccessLists') }}
        </a-button>
        <a-button
          type="primary"
          data-testid="permission-policy-save"
          :disabled="!canSave"
          :loading="saving"
          @click="save"
        >
          <template #icon>
            <SaveOutlined />
          </template>
          {{ t('config.save') }}
        </a-button>
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
        <template v-if="draft || commandPolicy">
          <div
            class="permission-policy-summary-cards"
            data-testid="permission-policy-summary-card"
            :aria-label="t('permissionPolicy.sections.summary')"
          >
            <div
              v-for="card in summaryCards"
              :key="card.key"
              class="permission-policy-summary-item"
              :data-tone="card.tone"
            >
              <component :is="card.icon" class="permission-policy-summary-item__icon" />
              <div class="permission-policy-summary-item__copy">
                <span>{{ card.label }}</span>
                <strong>{{ card.value }}</strong>
                <small>{{ card.description }}</small>
              </div>
            </div>
          </div>
        </template>

        <div
          v-else
          class="permission-policy-summary-empty"
          data-testid="permission-policy-summary-card"
        >
          <AppEmptyState
            icon="command"
            :title="t('permissionPolicy.empty.summaryTitle')"
            :description="t('permissionPolicy.empty.summaryDescription')"
          />
        </div>

        <section class="permission-policy-settings-section">
          <div class="permission-policy-settings-header">
            <h2>{{ t('permissionPolicy.sections.settings') }}</h2>
            <div class="permission-policy-status-row" aria-live="polite">
              <span
                v-if="hasUnsavedChanges"
                class="permission-policy-status-pill permission-policy-status-pill--dirty"
                data-testid="permission-policy-unsaved-status"
              >
                <ExclamationCircleOutlined />
                {{ t('permissionPolicy.status.unsaved') }}
              </span>
              <span
                v-else-if="saveStatus"
                class="permission-policy-status-pill permission-policy-status-pill--saved"
                data-testid="permission-policy-save-status"
              >
                <CheckCircleOutlined />
                {{ saveStatusLabel }}
              </span>
            </div>
          </div>

          <div v-if="draft" class="permission-policy-settings-layout">
            <a-card v-for="section in configSections" :key="section.title" :bordered="false" class="permission-policy-config-card">
              <div class="card-header config-card-header">
                <div class="permission-policy-config-card__title">
                  <span class="permission-policy-config-card__icon">
                    <component :is="getSectionIcon(section.key)" />
                  </span>
                  <strong>{{ section.title }}</strong>
                </div>
                <span class="field-count-badge">{{ section.fields.length }} {{ t('config.fieldCount') }}</span>
              </div>

              <a-form layout="vertical" class="permission-policy-settings-form">
                <div v-for="field in section.fields" :key="field.path" class="config-field-item">
                  <a-form-item>
                    <template #label>
                      <div class="field-label-wrap">
                        <span class="field-label-text">{{ field.label }}</span>
                        <a-tooltip v-if="field.description" :title="field.description">
                          <button type="button" class="field-info-icon" :aria-label="t('config.fieldHelp')">?</button>
                        </a-tooltip>
                      </div>
                    </template>

                    <a-select
                      v-if="isSuperAdminField(field.path)"
                      mode="tags"
                      class="super-admin-tag-select"
                      data-testid="permission-policy-super-admins"
                      :value="readSuperAdminTags()"
                      :aria-label="field.label"
                      :placeholder="t('permissionPolicy.placeholders.superAdmins')"
                      :token-separators="[',', '，', ' ', '\n']"
                      @update:value="writeSuperAdminTags"
                    />

                    <a-input
                      v-else-if="field.type === 'text'"
                      :value="String(readField(field.path, field.type) ?? '')"
                      :aria-label="field.label"
                      @update:value="writeField(field.path, field.type, $event)"
                    />

                    <a-input-number
                      v-else-if="field.type === 'number'"
                      class="config-number-input"
                      :value="typeof readField(field.path, field.type) === 'number' ? readField(field.path, field.type) : null"
                      :min="0"
                      :step="1"
                      :aria-label="field.label"
                      @update:value="writeField(field.path, field.type, $event)"
                    />

                    <div v-else-if="field.type === 'boolean'" class="switch-wrap">
                      <a-switch
                        :checked="Boolean(readField(field.path, field.type))"
                        :aria-label="field.label"
                        @update:checked="writeField(field.path, field.type, $event)"
                      />
                    </div>

                    <a-select
                      v-else-if="field.type === 'select'"
                      :value="String(readField(field.path, field.type) ?? '')"
                      :options="field.options"
                      :aria-label="field.label"
                      @update:value="writeField(field.path, field.type, $event)"
                    />

                    <a-textarea
                      v-else
                      :value="String(readField(field.path, field.type) ?? '')"
                      :auto-size="{ minRows: 4, maxRows: 8 }"
                      :aria-label="field.label"
                      @update:value="writeField(field.path, field.type, $event)"
                    />

                    <div v-if="field.description" class="config-field-note">
                      <p v-if="field.description" class="config-field-note__text">{{ field.description }}</p>
                    </div>
                  </a-form-item>
                </div>
              </a-form>
            </a-card>
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

.permission-policy-actions :deep(.ant-btn) {
  min-height: 36px;
  padding-inline: 14px;
  border-radius: var(--radius-md);
}

.permission-policy-summary-cards {
  display: grid;
  gap: 0;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  border-block: 1px solid var(--border);
}

.permission-policy-summary-empty {
  border: 1px solid var(--border);
  border-radius: var(--app-card-radius);
  background: var(--surface-strong);
  padding: 28px;
}

.permission-policy-summary-item {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  padding: 14px 16px;
}

.permission-policy-summary-item + .permission-policy-summary-item {
  border-inline-start: 1px solid var(--border);
}

.permission-policy-summary-item__icon {
  color: var(--accent);
  font-size: 20px;
}

.permission-policy-summary-item[data-tone='success'] .permission-policy-summary-item__icon {
  color: var(--success);
}

.permission-policy-summary-item[data-tone='warning'] .permission-policy-summary-item__icon {
  color: var(--warning);
}

.permission-policy-summary-item__copy {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.permission-policy-summary-item__copy span,
.permission-policy-summary-item__copy small {
  color: var(--muted);
  font-size: 13px;
}

.permission-policy-summary-item__copy strong {
  color: var(--text);
  font-size: 18px;
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

.permission-policy-config-card :deep(.ant-card-body) {
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

.config-field-item :deep(.ant-form-item) {
  margin-bottom: 0;
}

.config-field-item :deep(.ant-input),
.config-field-item :deep(.ant-select-selector) {
  border-radius: var(--radius-md);
}

.super-admin-tag-select {
  width: 100%;
}

.super-admin-tag-select :deep(.ant-select-selector) {
  min-height: 44px;
  align-items: flex-start;
  padding-block: 5px;
}

.super-admin-tag-select :deep(.ant-select-selection-item) {
  border-radius: 8px;
  background: var(--surface-soft);
  border-color: var(--border);
  font-weight: 600;
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
  outline-offset: 2px;
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
  .permission-policy-summary-cards {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .permission-policy-settings-layout {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .permission-policy-summary-cards {
    grid-template-columns: 1fr;
  }
}
</style>
