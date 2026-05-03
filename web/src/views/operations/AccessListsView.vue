<script setup lang="ts">
import { MotionDirective as vMotion } from '@vueuse/motion'
import { computed, onMounted, reactive, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'

import { notifySuccess } from '@/adapter/feedback'
import AppCard from '@/components/AppCard.vue'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { formatDateTime } from '@/lib/format'
import { buildCommandsLocation } from '@/lib/management-links'
import { t } from '@/i18n'
import { useGovernanceStore } from '@/stores/governance'
import type {
  BlacklistEntry,
  GovernanceEntryType,
} from '@/types/api'

const router = useRouter()
const governanceStore = useGovernanceStore()

const {
  blacklist,
  blacklistError,
  blacklistLoading,
  whitelist,
  whitelistError,
  whitelistLoading,
} = storeToRefs(governanceStore)

const pageLoading = ref(false)
const pageLoadError = ref<string | null>(null)
const blacklistActionError = ref<string | null>(null)
const whitelistActionError = ref<string | null>(null)
const blacklistMutating = ref(false)
const whitelistMutating = ref(false)
const whitelistConfirmVisible = ref(false)

const addModalVisible = ref(false)
const addModalTarget = ref<'whitelist' | 'blacklist'>('blacklist')
const addModalError = ref<string | null>(null)
const addModalMutating = ref(false)
const addModalDraft = reactive({
  entry_type: 'user' as GovernanceEntryType,
  target_id: '',
  reason: '',
})

const blacklistScopeFilter = ref<'all' | 'user' | 'group'>('all')
const whitelistScopeFilter = ref<'all' | 'user' | 'group'>('all')

const hasAccessListData = computed(() => Boolean(blacklist.value || whitelist.value))
const pageBusy = computed(() => pageLoading.value || blacklistLoading.value || whitelistLoading.value)
const pageErrorMessage = computed(() => pageLoadError.value ?? blacklistError.value ?? whitelistError.value)
const showFatalError = computed(() => Boolean(pageErrorMessage.value) && !hasAccessListData.value)

const userBlacklistEntries = computed(() => blacklist.value?.user_entries ?? [])
const groupBlacklistEntries = computed(() => blacklist.value?.group_entries ?? [])
const totalBlacklistEntries = computed(() => userBlacklistEntries.value.length + groupBlacklistEntries.value.length)

const userWhitelistEntries = computed(() => whitelist.value?.user_entries ?? [])
const groupWhitelistEntries = computed(() => whitelist.value?.group_entries ?? [])
const totalWhitelistEntries = computed(() => userWhitelistEntries.value.length + groupWhitelistEntries.value.length)
const whitelistEnabled = computed(() => whitelist.value?.enabled ?? false)
const showWhitelistEmptyWarning = computed(() => whitelistEnabled.value && totalWhitelistEntries.value === 0)

const blacklistRegionError = computed(() => blacklistActionError.value ?? blacklistError.value)
const whitelistRegionError = computed(() => whitelistActionError.value ?? whitelistError.value)

function sortEntries(entries: BlacklistEntry[]) {
  return [...entries].sort((a, b) => {
    if (!a.created_at || !b.created_at) return 0
    return new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
  })
}

const filteredBlacklistEntries = computed(() => {
  const entries = blacklistScopeFilter.value === 'all'
    ? [...userBlacklistEntries.value, ...groupBlacklistEntries.value]
    : blacklistScopeFilter.value === 'user'
      ? userBlacklistEntries.value
      : groupBlacklistEntries.value
  return sortEntries(entries)
})

const filteredWhitelistEntries = computed(() => {
  const entries = whitelistScopeFilter.value === 'all'
    ? [...userWhitelistEntries.value, ...groupWhitelistEntries.value]
    : whitelistScopeFilter.value === 'user'
      ? userWhitelistEntries.value
      : groupWhitelistEntries.value
  return sortEntries(entries)
})

const scopeOptions = computed(() => [
  { label: t('accessLists.scopes.user'), value: 'user' },
  { label: t('accessLists.scopes.group'), value: 'group' },
])

const scopeFilterOptions = computed(() => [
  { label: t('accessLists.filters.all'), value: 'all' },
  { label: t('accessLists.scopes.user'), value: 'user' },
  { label: t('accessLists.scopes.group'), value: 'group' },
])

const tableColumns = computed(() => [
  { title: t('accessLists.table.columns.type'), key: 'type', dataIndex: 'entry_type', width: 90, align: 'center' as const },
  { title: t('accessLists.table.columns.targetId'), key: 'targetId', dataIndex: 'target_id', width: 200 },
  { title: t('accessLists.table.columns.reason'), key: 'reason', dataIndex: 'reason' },
  { title: t('accessLists.table.columns.createdAt'), key: 'createdAt', dataIndex: 'created_at', width: 170 },
  { title: t('accessLists.table.columns.actions'), key: 'actions', width: 100, align: 'center' as const, fixed: 'right' as const },
])

function cardMotion(delay: number) {
  return {
    initial: { opacity: 0, y: 12 },
    enter: { opacity: 1, y: 0, transition: { duration: 320, delay: delay * 60, ease: 'easeOut' } },
  }
}

function getEntryTypeLabel(type: GovernanceEntryType) {
  return type === 'user' ? t('accessLists.scopes.user') : t('accessLists.scopes.group')
}

function getEntryTypeTagColor(type: GovernanceEntryType) {
  return type === 'user' ? 'blue' : 'purple'
}

async function loadAccessLists() {
  pageLoading.value = true
  pageLoadError.value = null

  const [blacklistResult, whitelistResult] = await Promise.allSettled([
    governanceStore.fetchBlacklist(),
    governanceStore.fetchWhitelist(),
  ])

  pageLoading.value = false

  if (blacklistResult.status === 'rejected' && whitelistResult.status === 'rejected') {
    pageLoadError.value = blacklistError.value ?? whitelistError.value ?? t('errors.common.loadFailed')
  }
}

async function removeBlacklistEntry(entry: BlacklistEntry) {
  blacklistMutating.value = true
  blacklistActionError.value = null
  try {
    await governanceStore.removeBlacklistEntry(entry.entry_type, entry.target_id)
    notifySuccess(t('accessLists.feedback.blacklistRemoved'))
  } catch (error) {
    blacklistActionError.value = getDisplayErrorMessage(error)
  } finally {
    blacklistMutating.value = false
  }
}

async function removeWhitelistEntry(entry: BlacklistEntry) {
  whitelistMutating.value = true
  whitelistActionError.value = null
  try {
    await governanceStore.removeWhitelistEntry(entry.entry_type, entry.target_id)
    notifySuccess(t('accessLists.feedback.whitelistRemoved'))
  } catch (error) {
    whitelistActionError.value = getDisplayErrorMessage(error)
  } finally {
    whitelistMutating.value = false
  }
}

async function applyWhitelistEnabled(enabled: boolean) {
  whitelistMutating.value = true
  whitelistActionError.value = null
  try {
    await governanceStore.setWhitelistEnabled(enabled)
    notifySuccess(t(enabled ? 'accessLists.feedback.whitelistEnabled' : 'accessLists.feedback.whitelistDisabled'))
  } catch (error) {
    whitelistActionError.value = getDisplayErrorMessage(error)
  } finally {
    whitelistMutating.value = false
  }
}

function handleWhitelistToggle(checked: boolean) {
  if (checked && !whitelistEnabled.value && totalWhitelistEntries.value === 0) {
    whitelistConfirmVisible.value = true
    return
  }

  void applyWhitelistEnabled(checked)
}

async function confirmEmptyWhitelistEnable() {
  whitelistConfirmVisible.value = false
  await applyWhitelistEnabled(true)
}

function openAddModal(target: 'whitelist' | 'blacklist') {
  addModalTarget.value = target
  addModalError.value = null
  const scopeFilter = target === 'blacklist' ? blacklistScopeFilter.value : whitelistScopeFilter.value
  addModalDraft.entry_type = scopeFilter === 'all' ? 'user' : scopeFilter
  addModalDraft.target_id = ''
  addModalDraft.reason = ''
  addModalVisible.value = true
}

function closeAddModal() {
  addModalVisible.value = false
}

function resetAddModalDraft() {
  addModalDraft.entry_type = 'user'
  addModalDraft.target_id = ''
  addModalDraft.reason = ''
}

async function submitAddModal() {
  const targetId = addModalDraft.target_id.trim()
  const reason = addModalDraft.reason.trim()

  if (!targetId || !reason) {
    addModalError.value = t('accessLists.validation.entryRequired')
    return
  }

  addModalMutating.value = true
  addModalError.value = null

  const payload = {
    entry_type: addModalDraft.entry_type,
    target_id: targetId,
    reason,
  }

  try {
    if (addModalTarget.value === 'blacklist') {
      await governanceStore.addBlacklistEntry(payload)
      blacklistActionError.value = null
      notifySuccess(t('accessLists.feedback.blacklistSaved'))
    } else {
      await governanceStore.addWhitelistEntry(payload)
      whitelistActionError.value = null
      notifySuccess(t('accessLists.feedback.whitelistSaved'))
    }
    closeAddModal()
    resetAddModalDraft()
  } catch (error) {
    addModalError.value = getDisplayErrorMessage(error)
  } finally {
    addModalMutating.value = false
  }
}

async function copyTargetId(targetId: string) {
  try {
    await navigator.clipboard.writeText(targetId)
    notifySuccess(t('accessLists.actions.copyTargetId'))
  } catch {
    // Clipboard access can be unavailable in embedded or test contexts.
  }
}

onMounted(() => {
  void loadAccessLists()
})
</script>

<template>
  <AppPage :title="t('accessLists.title')" :description="t('accessLists.subtitle')">
    <template #extra>
      <div class="table-actions">
        <a-button :loading="pageBusy" :aria-label="t('accessLists.refresh')" @click="loadAccessLists()">
          {{ t('accessLists.refresh') }}
        </a-button>
        <a-button data-testid="access-lists-open-commands" type="primary" @click="router.push(buildCommandsLocation())">
          {{ t('accessLists.actions.openCommands') }}
        </a-button>
      </div>
    </template>

    <RetryPanel
      v-if="showFatalError"
      :title="t('errors.common.loadFailed')"
      :description="pageErrorMessage ?? t('errors.common.loadFailed')"
      :loading="pageBusy"
      @retry="loadAccessLists()"
    />

    <div v-else class="access-lists-page__stack">
      <!-- Whitelist Card -->
      <AppCard
        v-motion="cardMotion(0)"
        borderless
        class="access-lists-card"
        :loading="whitelistLoading && !whitelist"
      >
        <div data-testid="access-lists-whitelist-card" class="access-lists-card-content">
          <div class="access-lists-card-header">
            <div class="access-lists-card-header__copy">
              <div class="access-lists-card-header__title-row">
                <strong>{{ t('accessLists.cards.whitelistTitle') }}</strong>
                <a-tooltip :title="t('accessLists.cards.whitelistDescription')">
                  <button type="button" class="access-lists-help-badge" :aria-label="t('accessLists.cards.whitelistHelp')">?</button>
                </a-tooltip>
              </div>
            </div>
            <div class="access-lists-card-header__meta">
              <span class="access-lists-card-header__count">{{ totalWhitelistEntries }}</span>
              <a-tag :color="whitelistEnabled ? 'warning' : 'default'">
                {{ whitelistEnabled ? t('accessLists.summary.whitelistEnabled') : t('accessLists.summary.whitelistDisabled') }}
              </a-tag>
              <a-switch
                :checked="whitelistEnabled"
                :loading="whitelistMutating"
                :aria-label="t('accessLists.summary.whitelistStatus')"
                data-testid="access-lists-whitelist-enabled"
                @change="handleWhitelistToggle"
              />
            </div>
          </div>

          <a-alert
            v-if="whitelistRegionError"
            :message="t('errors.common.actionFailed')"
            type="warning"
            :description="whitelistRegionError"
            show-icon
            class="section-gap"
          />

          <div v-if="showWhitelistEmptyWarning" class="access-lists-risk-banner">
            <div class="access-lists-risk-banner__header">
              <strong>{{ t('accessLists.whitelist.emptyWarningTitle') }}</strong>
              <a-tag color="warning">{{ t('accessLists.summary.whitelistEnabled') }}</a-tag>
            </div>
            <p>{{ t('accessLists.whitelist.emptyWarningDescription') }}</p>
          </div>

          <div class="access-lists-toolbar">
            <div class="access-lists-toolbar__row">
              <a-select
                v-model:value="whitelistScopeFilter"
                :options="scopeFilterOptions"
                class="access-lists-toolbar__filter"
                :aria-label="t('accessLists.filters.all')"
              />
              <div class="access-lists-toolbar__actions">
                <span class="access-lists-toolbar__count">{{ t('accessLists.table.total', { total: filteredWhitelistEntries.length }) }}</span>
                <a-button type="primary" data-testid="access-lists-whitelist-add-btn" @click="openAddModal('whitelist')">
                  {{ t('accessLists.actions.addEntry') }}
                </a-button>
              </div>
            </div>
          </div>

          <a-table
            class="access-lists-data-table app-data-table"
            :columns="tableColumns"
            :data-source="filteredWhitelistEntries"
            :pagination="false"
            :row-key="(row: BlacklistEntry) => `${row.entry_type}-${row.target_id}`"
            :loading="whitelistLoading && !whitelist"
          >
            <template #emptyText>
              <div class="access-lists-empty-hint">
                <p>{{ t('accessLists.empty.whitelistTitle') }}</p>
                <span>{{ t('accessLists.empty.whitelistDescription') }}</span>
              </div>
            </template>

            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'type'">
                <a-tag :color="getEntryTypeTagColor(record.entry_type)">
                  {{ getEntryTypeLabel(record.entry_type) }}
                </a-tag>
              </template>

              <template v-else-if="column.key === 'targetId'">
                <button
                  type="button"
                  class="mono-text copyable-text"
                  :aria-label="`${t('accessLists.actions.copyTargetId')} ${record.target_id}`"
                  @click="copyTargetId(record.target_id)"
                >
                  {{ record.target_id }}
                </button>
              </template>

              <template v-else-if="column.key === 'createdAt'">
                <span>{{ formatDateTime(record.created_at) }}</span>
              </template>

              <template v-else-if="column.key === 'actions'">
                <a-popconfirm
                  :title="t('accessLists.confirm.removeTitle')"
                  :description="t('accessLists.confirm.removeDescription')"
                  @confirm="removeWhitelistEntry(record)"
                >
                  <a-button type="link" danger size="small">
                    {{ t('accessLists.entryForm.remove') }}
                  </a-button>
                </a-popconfirm>
              </template>
            </template>
          </a-table>
        </div>
      </AppCard>

      <!-- Blacklist Card -->
      <AppCard
        v-motion="cardMotion(1)"
        borderless
        class="access-lists-card"
        :loading="blacklistLoading && !blacklist"
      >
        <div data-testid="access-lists-blacklist-card" class="access-lists-card-content">
          <div class="access-lists-card-header">
            <div class="access-lists-card-header__copy">
              <div class="access-lists-card-header__title-row">
                <strong>{{ t('accessLists.cards.blacklistTitle') }}</strong>
                <a-tooltip :title="t('accessLists.cards.blacklistDescription')">
                  <button type="button" class="access-lists-help-badge" :aria-label="t('accessLists.cards.blacklistHelp')">?</button>
                </a-tooltip>
              </div>
            </div>
            <div class="access-lists-card-header__meta">
              <span class="access-lists-card-header__count">{{ totalBlacklistEntries }}</span>
            </div>
          </div>

          <a-alert
            v-if="blacklistRegionError"
            :message="t('errors.common.actionFailed')"
            type="warning"
            :description="blacklistRegionError"
            show-icon
            class="section-gap"
          />

          <div class="access-lists-toolbar">
            <div class="access-lists-toolbar__row">
              <a-select
                v-model:value="blacklistScopeFilter"
                :options="scopeFilterOptions"
                class="access-lists-toolbar__filter"
                :aria-label="t('accessLists.filters.all')"
              />
              <div class="access-lists-toolbar__actions">
                <span class="access-lists-toolbar__count">{{ t('accessLists.table.total', { total: filteredBlacklistEntries.length }) }}</span>
                <a-button type="primary" data-testid="access-lists-blacklist-add-btn" @click="openAddModal('blacklist')">
                  {{ t('accessLists.actions.addEntry') }}
                </a-button>
              </div>
            </div>
          </div>

          <a-table
            class="access-lists-data-table app-data-table"
            :columns="tableColumns"
            :data-source="filteredBlacklistEntries"
            :pagination="false"
            :row-key="(row: BlacklistEntry) => `${row.entry_type}-${row.target_id}`"
            :loading="blacklistLoading && !blacklist"
          >
            <template #emptyText>
              <div class="access-lists-empty-hint">
                <p>{{ t('accessLists.empty.blacklistTitle') }}</p>
                <span>{{ t('accessLists.empty.blacklistDescription') }}</span>
              </div>
            </template>

            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'type'">
                <a-tag :color="getEntryTypeTagColor(record.entry_type)">
                  {{ getEntryTypeLabel(record.entry_type) }}
                </a-tag>
              </template>

              <template v-else-if="column.key === 'targetId'">
                <button
                  type="button"
                  class="mono-text copyable-text"
                  :aria-label="`${t('accessLists.actions.copyTargetId')} ${record.target_id}`"
                  @click="copyTargetId(record.target_id)"
                >
                  {{ record.target_id }}
                </button>
              </template>

              <template v-else-if="column.key === 'createdAt'">
                <span>{{ formatDateTime(record.created_at) }}</span>
              </template>

              <template v-else-if="column.key === 'actions'">
                <a-popconfirm
                  :title="t('accessLists.confirm.removeTitle')"
                  :description="t('accessLists.confirm.removeDescription')"
                  @confirm="removeBlacklistEntry(record)"
                >
                  <a-button type="link" danger size="small">
                    {{ t('accessLists.entryForm.remove') }}
                  </a-button>
                </a-popconfirm>
              </template>
            </template>
          </a-table>
        </div>
      </AppCard>
    </div>

    <a-modal
      v-model:open="addModalVisible"
      :title="t('accessLists.modal.addTitle', {
        target: addModalTarget === 'whitelist'
          ? t('accessLists.modal.addTargetWhitelist')
          : t('accessLists.modal.addTargetBlacklist'),
      })"
      :confirm-loading="addModalMutating"
      :ok-text="t('accessLists.modal.save')"
      :cancel-text="t('accessLists.modal.cancel')"
      @ok="submitAddModal"
      @cancel="closeAddModal"
    >
      <a-alert
        v-if="addModalError"
        :message="t('errors.common.actionFailed')"
        type="warning"
        :description="addModalError"
        show-icon
        class="section-gap"
      />

      <a-form layout="vertical" class="add-modal-form">
        <a-form-item :label="t('accessLists.entryForm.scope')">
          <a-select v-model:value="addModalDraft.entry_type" :options="scopeOptions" :aria-label="t('accessLists.entryForm.scope')" />
        </a-form-item>
        <a-form-item :label="t('accessLists.entryForm.targetId')">
          <a-input
            v-model:value="addModalDraft.target_id"
            :placeholder="t('accessLists.entryForm.placeholderTargetId')"
            :aria-label="t('accessLists.entryForm.targetId')"
          />
        </a-form-item>
        <a-form-item :label="t('accessLists.entryForm.reason')">
          <a-input
            v-model:value="addModalDraft.reason"
            :placeholder="t('accessLists.entryForm.placeholderReason')"
            :aria-label="t('accessLists.entryForm.reason')"
          />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal
      v-model:open="whitelistConfirmVisible"
      data-testid="access-lists-whitelist-confirm-modal"
      :title="t('accessLists.whitelist.enableConfirmTitle')"
      :ok-text="t('accessLists.whitelist.enableConfirmAction')"
      :confirm-loading="whitelistMutating"
      @ok="confirmEmptyWhitelistEnable"
    >
      <p>{{ t('accessLists.whitelist.enableConfirmDescription') }}</p>
    </a-modal>
  </AppPage>
</template>

<style scoped lang="scss">
.access-lists-page__stack {
  display: grid;
  gap: 20px;
}

.access-lists-card {
  border-radius: var(--radius-lg);
}

:deep(.access-lists-card) {
  box-shadow: var(--shadow-xs);
}

.access-lists-card-content {
  display: grid;
  gap: 16px;
}

.access-lists-card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.access-lists-card-header__copy {
  display: grid;
  gap: 6px;
}

.access-lists-card-header__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.access-lists-card-header__copy strong {
  font-size: 1.05rem;
  line-height: 1.3;
}

.access-lists-help-badge {
  appearance: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--muted);
  cursor: help;
  font-size: 0.8rem;
  font-weight: 700;
  line-height: 1;
  opacity: 0.75;
  transition: color 0.15s ease, border-color 0.15s ease, opacity 0.15s ease;
}

.access-lists-help-badge:hover {
  color: var(--accent);
  border-color: var(--accent);
  opacity: 1;
}

.access-lists-help-badge:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.access-lists-card-header__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  flex-shrink: 0;
}

.access-lists-card-header__count {
  font-size: 1.5rem;
  font-weight: 700;
  line-height: 1;
  color: var(--fg);
  letter-spacing: -0.02em;
}

.access-lists-risk-banner {
  padding: 14px;
  border-radius: var(--radius-lg);
  background: color-mix(in srgb, var(--warning) 12%, var(--surface));
  border: 1px solid color-mix(in srgb, var(--warning) 22%, var(--border));
  display: grid;
  gap: 8px;
}

.access-lists-risk-banner__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.access-lists-risk-banner p {
  margin: 0;
  color: var(--muted);
}

.access-lists-toolbar {
  display: grid;
  gap: 12px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--border);
}

.access-lists-toolbar__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.access-lists-toolbar__filter {
  width: 120px;
}

.access-lists-toolbar__actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.access-lists-toolbar__count {
  font-size: 0.82rem;
  color: var(--muted);
}

.access-lists-data-table {
  border-radius: var(--app-card-radius);
  overflow: hidden;
}

.access-lists-data-table :deep(.ant-table-row:hover > td) {
  background: var(--surface-accent) !important;
}

.access-lists-empty-hint {
  padding: var(--space-xl) var(--space-md);
  text-align: center;
}

.access-lists-empty-hint p {
  margin: 0 0 4px;
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--muted);
}

.access-lists-empty-hint span {
  font-size: 0.82rem;
  color: var(--muted);
}

.mono-text {
  font-family: var(--font-mono);
  font-size: 0.88rem;
}

.copyable-text {
  appearance: none;
  border: 0;
  background: transparent;
  padding: 0;
  cursor: pointer;
  transition: color 0.15s ease;
  text-align: left;
}

.copyable-text:hover {
  color: var(--accent);
}

.copyable-text:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
  border-radius: 4px;
}

.add-modal-form {
  margin-top: 8px;
}

@media (max-width: 768px) {
  .access-lists-card-header {
    flex-direction: column;
  }

  .access-lists-card-header__meta {
    width: 100%;
  }

  .access-lists-toolbar__row {
    flex-direction: column;
    align-items: flex-start;
  }

  .access-lists-toolbar__actions {
    width: 100%;
    justify-content: space-between;
  }

  .access-lists-toolbar__count {
    text-align: right;
  }
}
</style>
