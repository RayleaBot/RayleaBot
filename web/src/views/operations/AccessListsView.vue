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

// Search filters
const whitelistSearchQuery = ref('')
const blacklistSearchQuery = ref('')

// Inline Whitelist adding state
const isAddingWhitelist = ref(false)
const whitelistAdding = ref(false)
const whitelistDraft = reactive({
  entry_type: 'user' as GovernanceEntryType,
  target_id: '',
  reason: '',
})
const whitelistDraftErrors = reactive({
  target_id: '',
  reason: '',
})

// Inline Blacklist adding state
const isAddingBlacklist = ref(false)
const blacklistAdding = ref(false)
const blacklistDraft = reactive({
  entry_type: 'user' as GovernanceEntryType,
  target_id: '',
  reason: '',
})
const blacklistDraftErrors = reactive({
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
  let entries = blacklistScopeFilter.value === 'all'
    ? [...userBlacklistEntries.value, ...groupBlacklistEntries.value]
    : blacklistScopeFilter.value === 'user'
      ? userBlacklistEntries.value
      : groupBlacklistEntries.value

  if (blacklistSearchQuery.value.trim()) {
    const query = blacklistSearchQuery.value.trim().toLowerCase()
    entries = entries.filter(e =>
      e.target_id.toLowerCase().includes(query) ||
      (e.reason && e.reason.toLowerCase().includes(query))
    )
  }
  return sortEntries(entries)
})

const filteredWhitelistEntries = computed(() => {
  let entries = whitelistScopeFilter.value === 'all'
    ? [...userWhitelistEntries.value, ...groupWhitelistEntries.value]
    : whitelistScopeFilter.value === 'user'
      ? userWhitelistEntries.value
      : groupWhitelistEntries.value

  if (whitelistSearchQuery.value.trim()) {
    const query = whitelistSearchQuery.value.trim().toLowerCase()
    entries = entries.filter(e =>
      e.target_id.toLowerCase().includes(query) ||
      (e.reason && e.reason.toLowerCase().includes(query))
    )
  }
  return sortEntries(entries)
})

// Data sources including inline draft rows
const whitelistTableData = computed(() => {
  const list = [...filteredWhitelistEntries.value]
  if (isAddingWhitelist.value) {
    list.unshift({
      entry_type: whitelistDraft.entry_type,
      target_id: '__whitelist_draft__',
      reason: whitelistDraft.reason,
      created_at: '',
      isDraft: true,
    } as any)
  }
  return list
})

const blacklistTableData = computed(() => {
  const list = [...filteredBlacklistEntries.value]
  if (isAddingBlacklist.value) {
    list.unshift({
      entry_type: blacklistDraft.entry_type,
      target_id: '__blacklist_draft__',
      reason: blacklistDraft.reason,
      created_at: '',
      isDraft: true,
    } as any)
  }
  return list
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
  { title: t('accessLists.table.columns.targetId'), key: 'targetId', dataIndex: 'target_id', width: 180 },
  { title: t('accessLists.table.columns.reason'), key: 'reason', dataIndex: 'reason' },
  { title: t('accessLists.table.columns.createdAt'), key: 'createdAt', dataIndex: 'created_at', width: 170 },
  { title: t('accessLists.table.columns.actions'), key: 'actions', width: 120, align: 'center' as const, fixed: 'right' as const },
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

// Inline Whitelist controls
function startAddWhitelist() {
  isAddingWhitelist.value = true
  const scopeFilter = whitelistScopeFilter.value
  whitelistDraft.entry_type = scopeFilter === 'all' ? 'user' : scopeFilter
  whitelistDraft.target_id = ''
  whitelistDraft.reason = ''
  whitelistDraftErrors.target_id = ''
  whitelistDraftErrors.reason = ''
}

function cancelWhitelistInline() {
  isAddingWhitelist.value = false
  whitelistDraft.target_id = ''
  whitelistDraft.reason = ''
  whitelistDraftErrors.target_id = ''
  whitelistDraftErrors.reason = ''
}

async function saveWhitelistInline() {
  const targetId = whitelistDraft.target_id.trim()
  const reason = whitelistDraft.reason.trim()

  let hasError = false
  if (!targetId) {
    whitelistDraftErrors.target_id = t('accessLists.validation.entryRequired')
    hasError = true
  }
  if (!reason) {
    whitelistDraftErrors.reason = t('accessLists.validation.entryRequired')
    hasError = true
  }

  if (hasError) return

  whitelistAdding.value = true
  whitelistActionError.value = null

  try {
    await governanceStore.addWhitelistEntry({
      entry_type: whitelistDraft.entry_type,
      target_id: targetId,
      reason,
    })
    cancelWhitelistInline()
    notifySuccess(t('accessLists.feedback.whitelistSaved'))
  } catch (error) {
    whitelistActionError.value = getDisplayErrorMessage(error)
  } finally {
    whitelistAdding.value = false
  }
}

// Inline Blacklist controls
function startAddBlacklist() {
  isAddingBlacklist.value = true
  const scopeFilter = blacklistScopeFilter.value
  blacklistDraft.entry_type = scopeFilter === 'all' ? 'user' : scopeFilter
  blacklistDraft.target_id = ''
  blacklistDraft.reason = ''
  blacklistDraftErrors.target_id = ''
  blacklistDraftErrors.reason = ''
}

function cancelBlacklistInline() {
  isAddingBlacklist.value = false
  blacklistDraft.target_id = ''
  blacklistDraft.reason = ''
  blacklistDraftErrors.target_id = ''
  blacklistDraftErrors.reason = ''
}

async function saveBlacklistInline() {
  const targetId = blacklistDraft.target_id.trim()
  const reason = blacklistDraft.reason.trim()

  let hasError = false
  if (!targetId) {
    blacklistDraftErrors.target_id = t('accessLists.validation.entryRequired')
    hasError = true
  }
  if (!reason) {
    blacklistDraftErrors.reason = t('accessLists.validation.entryRequired')
    hasError = true
  }

  if (hasError) return

  blacklistAdding.value = true
  blacklistActionError.value = null

  try {
    await governanceStore.addBlacklistEntry({
      entry_type: blacklistDraft.entry_type,
      target_id: targetId,
      reason,
    })
    cancelBlacklistInline()
    notifySuccess(t('accessLists.feedback.blacklistSaved'))
  } catch (error) {
    blacklistActionError.value = getDisplayErrorMessage(error)
  } finally {
    blacklistAdding.value = false
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

    <div v-else class="access-lists-page__grid">
      <!-- Whitelist Card -->
      <AppCard
        v-motion="cardMotion(0)"
        borderless
        class="access-lists-card whitelist-card-premium"
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
              <div class="toolbar-left-group">
                <a-select
                  v-model:value="whitelistScopeFilter"
                  :options="scopeFilterOptions"
                  class="access-lists-toolbar__filter"
                  :aria-label="t('accessLists.filters.all')"
                />
                <a-input
                  v-model:value="whitelistSearchQuery"
                  :placeholder="t('accessLists.entryForm.searchPlaceholder')"
                  class="access-lists-toolbar__search"
                  allow-clear
                  data-testid="whitelist-search-input"
                >
                  <template #prefix>
                    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-muted-svg">
                      <circle cx="11" cy="11" r="8"></circle>
                      <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
                    </svg>
                  </template>
                </a-input>
              </div>
              <div class="access-lists-toolbar__actions">
                <span class="access-lists-toolbar__count">{{ t('accessLists.table.total', { total: filteredWhitelistEntries.length }) }}</span>
                <a-button type="primary" data-testid="access-lists-whitelist-add-btn" :disabled="isAddingWhitelist" @click="startAddWhitelist">
                  {{ t('accessLists.actions.addEntry') }}
                </a-button>
              </div>
            </div>
          </div>

          <a-table
            class="access-lists-data-table app-data-table"
            :columns="tableColumns"
            :data-source="whitelistTableData"
            :pagination="false"
            :row-key="(row: any) => row.isDraft ? 'draft-whitelist' : `${row.entry_type}-${row.target_id}`"
            :loading="whitelistLoading && !whitelist"
          >
            <template #emptyText>
              <div class="access-lists-empty-container">
                <div class="empty-graphic">
                  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="42" height="42" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                    <circle cx="12" cy="12" r="10"></circle>
                    <line x1="12" y1="8" x2="12" y2="12"></line>
                    <line x1="12" y1="16" x2="12.01" y2="16"></line>
                  </svg>
                </div>
                <p class="empty-title">{{ t('accessLists.empty.whitelistTitle') }}</p>
                <p class="empty-desc">{{ t('accessLists.empty.whitelistDescription') }}</p>
              </div>
            </template>

            <template #bodyCell="{ column, record }">
              <template v-if="record.isDraft">
                <template v-if="column.key === 'type'">
                  <a-select
                    v-model:value="whitelistDraft.entry_type"
                    :options="scopeOptions"
                    size="small"
                    style="width: 100%"
                    data-testid="whitelist-draft-type"
                  />
                </template>

                <template v-else-if="column.key === 'targetId'">
                  <div class="inline-edit-cell">
                    <a-input
                      v-model:value="whitelistDraft.target_id"
                      :placeholder="t('accessLists.entryForm.placeholderTargetId')"
                      size="small"
                      :status="whitelistDraftErrors.target_id ? 'error' : ''"
                      data-testid="whitelist-draft-target-id"
                      @input="whitelistDraftErrors.target_id = ''"
                    />
                    <div v-if="whitelistDraftErrors.target_id" class="inline-error-text">
                      {{ whitelistDraftErrors.target_id }}
                    </div>
                  </div>
                </template>

                <template v-else-if="column.key === 'reason'">
                  <div class="inline-edit-cell">
                    <a-input
                      v-model:value="whitelistDraft.reason"
                      :placeholder="t('accessLists.entryForm.placeholderReason')"
                      size="small"
                      :status="whitelistDraftErrors.reason ? 'error' : ''"
                      data-testid="whitelist-draft-reason"
                      @input="whitelistDraftErrors.reason = ''"
                    />
                    <div v-if="whitelistDraftErrors.reason" class="inline-error-text">
                      {{ whitelistDraftErrors.reason }}
                    </div>
                  </div>
                </template>

                <template v-else-if="column.key === 'createdAt'">
                  <span class="text-muted-inline">-</span>
                </template>

                <template v-else-if="column.key === 'actions'">
                  <div class="inline-actions">
                    <a-button
                      type="link"
                      size="small"
                      :loading="whitelistAdding"
                      data-testid="whitelist-draft-save"
                      @click="saveWhitelistInline"
                    >
                      {{ t('accessLists.modal.save') }}
                    </a-button>
                    <a-button
                      type="link"
                      size="small"
                      class="text-muted-btn"
                      data-testid="whitelist-draft-cancel"
                      @click="cancelWhitelistInline"
                    >
                      {{ t('accessLists.modal.cancel') }}
                    </a-button>
                  </div>
                </template>
              </template>

              <template v-else>
                <template v-if="column.key === 'type'">
                  <a-tag :color="getEntryTypeTagColor(record.entry_type)">
                    {{ getEntryTypeLabel(record.entry_type) }}
                  </a-tag>
                </template>

                <template v-else-if="column.key === 'targetId'">
                  <button
                    type="button"
                    class="target-id-chip copyable-text mono-text"
                    :aria-label="`${t('accessLists.actions.copyTargetId')} ${record.target_id}`"
                    @click="copyTargetId(record.target_id)"
                  >
                    <span class="chip-dot font-dot-success"></span>
                    <span class="chip-text">{{ record.target_id }}</span>
                    <span class="copy-icon-hover">
                      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="10" height="10" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                        <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                        <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                      </svg>
                    </span>
                  </button>
                </template>

                <template v-else-if="column.key === 'reason'">
                  <span class="cell-reason">{{ record.reason }}</span>
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
                    <a-button type="link" danger size="small" class="remove-btn">
                      {{ t('accessLists.entryForm.remove') }}
                    </a-button>
                  </a-popconfirm>
                </template>
              </template>
            </template>
          </a-table>
        </div>
      </AppCard>

      <!-- Blacklist Card -->
      <AppCard
        v-motion="cardMotion(1)"
        borderless
        class="access-lists-card blacklist-card-premium"
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
              <div class="toolbar-left-group">
                <a-select
                  v-model:value="blacklistScopeFilter"
                  :options="scopeFilterOptions"
                  class="access-lists-toolbar__filter"
                  :aria-label="t('accessLists.filters.all')"
                />
                <a-input
                  v-model:value="blacklistSearchQuery"
                  :placeholder="t('accessLists.entryForm.searchPlaceholder')"
                  class="access-lists-toolbar__search"
                  allow-clear
                  data-testid="blacklist-search-input"
                >
                  <template #prefix>
                    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="text-muted-svg">
                      <circle cx="11" cy="11" r="8"></circle>
                      <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
                    </svg>
                  </template>
                </a-input>
              </div>
              <div class="access-lists-toolbar__actions">
                <span class="access-lists-toolbar__count">{{ t('accessLists.table.total', { total: filteredBlacklistEntries.length }) }}</span>
                <a-button type="primary" data-testid="access-lists-blacklist-add-btn" :disabled="isAddingBlacklist" @click="startAddBlacklist">
                  {{ t('accessLists.actions.addEntry') }}
                </a-button>
              </div>
            </div>
          </div>

          <a-table
            class="access-lists-data-table app-data-table"
            :columns="tableColumns"
            :data-source="blacklistTableData"
            :pagination="false"
            :row-key="(row: any) => row.isDraft ? 'draft-blacklist' : `${row.entry_type}-${row.target_id}`"
            :loading="blacklistLoading && !blacklist"
          >
            <template #emptyText>
              <div class="access-lists-empty-container">
                <div class="empty-graphic">
                  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="42" height="42" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                    <circle cx="12" cy="12" r="10"></circle>
                    <line x1="12" y1="8" x2="12" y2="12"></line>
                    <line x1="12" y1="16" x2="12.01" y2="16"></line>
                  </svg>
                </div>
                <p class="empty-title">{{ t('accessLists.empty.blacklistTitle') }}</p>
                <p class="empty-desc">{{ t('accessLists.empty.blacklistDescription') }}</p>
              </div>
            </template>

            <template #bodyCell="{ column, record }">
              <template v-if="record.isDraft">
                <template v-if="column.key === 'type'">
                  <a-select
                    v-model:value="blacklistDraft.entry_type"
                    :options="scopeOptions"
                    size="small"
                    style="width: 100%"
                    data-testid="blacklist-draft-type"
                  />
                </template>

                <template v-else-if="column.key === 'targetId'">
                  <div class="inline-edit-cell">
                    <a-input
                      v-model:value="blacklistDraft.target_id"
                      :placeholder="t('accessLists.entryForm.placeholderTargetId')"
                      size="small"
                      :status="blacklistDraftErrors.target_id ? 'error' : ''"
                      data-testid="blacklist-draft-target-id"
                      @input="blacklistDraftErrors.target_id = ''"
                    />
                    <div v-if="blacklistDraftErrors.target_id" class="inline-error-text">
                      {{ blacklistDraftErrors.target_id }}
                    </div>
                  </div>
                </template>

                <template v-else-if="column.key === 'reason'">
                  <div class="inline-edit-cell">
                    <a-input
                      v-model:value="blacklistDraft.reason"
                      :placeholder="t('accessLists.entryForm.placeholderReason')"
                      size="small"
                      :status="blacklistDraftErrors.reason ? 'error' : ''"
                      data-testid="blacklist-draft-reason"
                      @input="blacklistDraftErrors.reason = ''"
                    />
                    <div v-if="blacklistDraftErrors.reason" class="inline-error-text">
                      {{ blacklistDraftErrors.reason }}
                    </div>
                  </div>
                </template>

                <template v-else-if="column.key === 'createdAt'">
                  <span class="text-muted-inline">-</span>
                </template>

                <template v-else-if="column.key === 'actions'">
                  <div class="inline-actions">
                    <a-button
                      type="link"
                      size="small"
                      :loading="blacklistAdding"
                      data-testid="blacklist-draft-save"
                      @click="saveBlacklistInline"
                    >
                      {{ t('accessLists.modal.save') }}
                    </a-button>
                    <a-button
                      type="link"
                      size="small"
                      class="text-muted-btn"
                      data-testid="blacklist-draft-cancel"
                      @click="cancelBlacklistInline"
                    >
                      {{ t('accessLists.modal.cancel') }}
                    </a-button>
                  </div>
                </template>
              </template>

              <template v-else>
                <template v-if="column.key === 'type'">
                  <a-tag :color="getEntryTypeTagColor(record.entry_type)">
                    {{ getEntryTypeLabel(record.entry_type) }}
                  </a-tag>
                </template>

                <template v-else-if="column.key === 'targetId'">
                  <button
                    type="button"
                    class="target-id-chip copyable-text mono-text"
                    :aria-label="`${t('accessLists.actions.copyTargetId')} ${record.target_id}`"
                    @click="copyTargetId(record.target_id)"
                  >
                    <span class="chip-dot font-dot-danger"></span>
                    <span class="chip-text">{{ record.target_id }}</span>
                    <span class="copy-icon-hover">
                      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="10" height="10" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                        <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                        <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                      </svg>
                    </span>
                  </button>
                </template>

                <template v-else-if="column.key === 'reason'">
                  <span class="cell-reason">{{ record.reason }}</span>
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
                    <a-button type="link" danger size="small" class="remove-btn">
                      {{ t('accessLists.entryForm.remove') }}
                    </a-button>
                  </a-popconfirm>
                </template>
              </template>
            </template>
          </a-table>
        </div>
      </AppCard>
    </div>

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
.access-lists-page__grid {
  display: grid;
  grid-template-columns: repeat(1, minmax(0, 1fr));
  gap: 24px;
}

@media (min-width: 1024px) {
  .access-lists-page__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

.access-lists-card {
  border-radius: var(--radius-lg);
  background: var(--surface);
  border: 1px solid var(--border);
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

:deep(.access-lists-card) {
  box-shadow: var(--shadow-xs);
}

.whitelist-card-premium {
  border-top: 4px solid var(--accent, #3b82f6) !important;

  &:hover {
    transform: translateY(-2px);
    box-shadow: 0 8px 30px rgba(0, 0, 0, 0.06) !important;
  }
}

.blacklist-card-premium {
  border-top: 4px solid #f43f5e !important;

  &:hover {
    transform: translateY(-2px);
    box-shadow: 0 8px 30px rgba(0, 0, 0, 0.06) !important;
  }
}

.access-lists-card-content {
  display: grid;
  gap: 20px;
}

.access-lists-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.access-lists-card-header__copy {
  display: grid;
  gap: 4px;
}

.access-lists-card-header__title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.access-lists-card-header__copy strong {
  font-size: 1.15rem;
  font-weight: 700;
  line-height: 1.2;
  color: var(--fg);
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
  gap: 10px;
  flex-wrap: wrap;
  flex-shrink: 0;
}

.access-lists-card-header__count {
  font-size: 1.65rem;
  font-weight: 800;
  line-height: 1;
  color: var(--fg);
  letter-spacing: -0.02em;
}

.access-lists-risk-banner {
  padding: 14px;
  border-radius: var(--radius-lg);
  background: color-mix(in srgb, var(--warning) 10%, var(--surface));
  border: 1px solid color-mix(in srgb, var(--warning) 20%, var(--border));
  display: grid;
  gap: 6px;
  font-size: 0.85rem;
}

.access-lists-risk-banner__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.access-lists-risk-banner p {
  margin: 0;
  color: var(--muted);
  line-height: 1.4;
}

.access-lists-toolbar {
  display: grid;
  gap: 12px;
}

.access-lists-toolbar__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.toolbar-left-group {
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 1;
  max-width: 400px;
}

.access-lists-toolbar__filter {
  width: 110px;
  flex-shrink: 0;
}

.access-lists-toolbar__search {
  flex: 1;
}

.access-lists-toolbar__actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.access-lists-toolbar__count {
  font-size: 0.82rem;
  color: var(--muted);
}

.access-lists-data-table {
  border-radius: var(--radius-lg);
  overflow: hidden;
  border: 1px solid var(--border);
}

.access-lists-data-table :deep(.ant-table-thead > tr > th) {
  background: color-mix(in srgb, var(--surface-accent) 25%, var(--surface));
  font-weight: 600;
  font-size: 0.85rem;
  color: var(--fg);
  border-bottom: 1px solid var(--border);
}

.access-lists-data-table :deep(.ant-table-row:hover > td) {
  background: var(--surface-accent) !important;
}

.target-id-chip {
  appearance: none;
  border: 1px solid color-mix(in srgb, var(--accent) 15%, var(--border));
  background: color-mix(in srgb, var(--accent) 5%, var(--surface));
  padding: 4px 10px;
  border-radius: 20px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--fg);
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  font-size: 0.85rem;
  font-weight: 600;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;

  .chip-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }

  .font-dot-success {
    background-color: var(--accent, #3b82f6);
    box-shadow: 0 0 8px var(--accent);
  }

  .font-dot-danger {
    background-color: #f43f5e;
    box-shadow: 0 0 8px #f43f5e;
  }

  .chip-text {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .copy-icon-hover {
    color: var(--muted);
    opacity: 0;
    width: 0;
    transition: opacity 0.15s ease, width 0.15s ease;
    display: inline-flex;
    align-items: center;
  }

  &:hover {
    border-color: var(--accent);
    background: color-mix(in srgb, var(--accent) 12%, var(--surface));
    transform: scale(1.02);

    .copy-icon-hover {
      opacity: 1;
      width: 12px;
      margin-left: 2px;
    }
  }

  &:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 2px;
  }
}

.cell-reason {
  font-size: 0.88rem;
  color: var(--fg-light, var(--fg));
}

.remove-btn {
  font-size: 0.85rem;
  font-weight: 500;
  padding: 0 4px;

  &:hover {
    color: #ef4444 !important;
  }
}

.inline-edit-cell {
  display: grid;
  gap: 4px;
  width: 100%;
}

.inline-error-text {
  font-size: 0.78rem;
  color: #ef4444;
  text-align: left;
  line-height: 1.2;
}

.inline-actions {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.text-muted-btn {
  color: var(--muted) !important;

  &:hover {
    color: var(--fg) !important;
  }
}

.text-muted-inline {
  color: var(--muted);
}

.text-muted-svg {
  color: var(--muted);
  opacity: 0.7;
}

.access-lists-empty-container {
  padding: 44px 20px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  background: color-mix(in srgb, var(--surface-accent) 15%, transparent);
  border-radius: var(--radius-lg);
  border: 1px dashed var(--border);
  margin: 12px 0;

  .empty-graphic {
    color: var(--muted);
    opacity: 0.45;
    margin-bottom: 12px;
    transition: transform 0.25s ease, color 0.25s ease;
  }

  &:hover .empty-graphic {
    transform: scale(1.1) rotate(5deg);
    color: var(--accent);
    opacity: 0.8;
  }

  .empty-title {
    font-size: 0.95rem;
    font-weight: 600;
    color: var(--fg);
    margin: 0 0 4px;
  }

  .empty-desc {
    font-size: 0.82rem;
    color: var(--muted);
    margin: 0;
    max-width: 280px;
  }
}

.mono-text {
  font-family: var(--font-mono);
}

@media (max-width: 768px) {
  .access-lists-card-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .access-lists-card-header__meta {
    width: 100%;
  }

  .access-lists-toolbar__row {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .toolbar-left-group {
    width: 100%;
    max-width: 100%;
  }

  .access-lists-toolbar__actions {
    width: 100%;
    justify-content: space-between;
  }
}
</style>
