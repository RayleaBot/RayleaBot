<script setup lang="ts">
import AppSwitch from '@/components/AppSwitch.vue'
import AppConfirmDialog from '@/components/AppConfirmDialog.vue'
import AppButton from '@/components/AppButton.vue'
import AppAlert from '@/components/AppAlert.vue'
import { ArrowUpRightIcon } from '@lucide/vue'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import { notifySuccess, useToastFeedback } from '@/adapter/feedback'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { governanceScopeLabel } from '@/lib/governance-scope'
import { buildCommandsLocation } from '@/lib/management-links'
import { t } from '@/i18n'
import AccessListCard from '@/components/governance/AccessListCard.vue'
import { useAccessListEditor, type AccessListEditor } from '@/components/governance/useAccessListEditor'
import { useGovernanceStore } from '@/stores/governance'
import { useMotionNavigation } from '@/motion/useMotionNavigation'
import type {
  BlacklistEntry,
} from '@/types/api'

const navigate = useMotionNavigation()
const governanceStore = useGovernanceStore()

const {
  blacklistLoadingMore, whitelistLoadingMore,
  blacklist,
  blacklistError,
  blacklistLoading,
  whitelist,
  whitelistError,
  whitelistLoading,
} = storeToRefs(governanceStore)

const pageLoading = ref(false)
const pageLoadError = ref<string | null>(null)
const whitelistConfirmVisible = ref(false)
const removeOpen = ref(false)
const removeCandidate = ref<{ kind: 'whitelist' | 'blacklist'; entry: BlacklistEntry } | null>(null)

const blacklistEditor = reactive(useAccessListEditor({
  kind: 'blacklist',
  entries: computed(() => [...(blacklist.value?.user_entries ?? []), ...(blacklist.value?.group_entries ?? [])]),
  add: governanceStore.addBlacklistEntry,
  remove: entry => governanceStore.removeBlacklistEntry(entry.entry_type, entry.target_id, entry.scope),
  removed: () => { removeOpen.value = false },
}))

const whitelistEditor = reactive(useAccessListEditor({
  kind: 'whitelist',
  entries: computed(() => [...(whitelist.value?.user_entries ?? []), ...(whitelist.value?.group_entries ?? [])]),
  add: governanceStore.addWhitelistEntry,
  remove: entry => governanceStore.removeWhitelistEntry(entry.entry_type, entry.target_id, entry.scope),
  removed: () => { removeOpen.value = false },
}))

function listQuery(editor: AccessListEditor) {
  return { query: editor.searchQuery, entry_type: editor.scopeFilter === 'all' ? undefined : editor.scopeFilter }
}

function fetchList(kind: 'blacklist' | 'whitelist') {
  return kind === 'blacklist'
    ? governanceStore.fetchBlacklist(undefined, listQuery(blacklistEditor))
    : governanceStore.fetchWhitelist(undefined, listQuery(whitelistEditor))
}

watch([() => blacklistEditor.searchQuery, () => blacklistEditor.scopeFilter], () => { void fetchList('blacklist').catch(() => undefined) })
watch([() => whitelistEditor.searchQuery, () => whitelistEditor.scopeFilter], () => { void fetchList('whitelist').catch(() => undefined) })

const hasAccessListData = computed(() => Boolean(blacklist.value || whitelist.value))
const pageBusy = computed(() => pageLoading.value || blacklistLoading.value || whitelistLoading.value)
const pageErrorMessage = computed(() => pageLoadError.value ?? blacklistError.value ?? whitelistError.value)
const showFatalError = computed(() => Boolean(pageErrorMessage.value) && !hasAccessListData.value)

const totalWhitelistEntries = computed(() => whitelist.value?.entry_count ?? 0)
const whitelistEnabled = computed(() => whitelist.value?.enabled ?? false)
const showWhitelistEmptyWarning = computed(() => whitelistEnabled.value && totalWhitelistEntries.value === 0)

// Read failures stay inside their list; only a failed add, remove or switch arrives as a toast.
const whitelistActionErrorToast = computed(() => (
  whitelistEditor.actionError
    ? {
        key: `access-lists-whitelist:${whitelistEditor.actionError}`,
        level: 'warning' as const,
        message: whitelistEditor.actionError,
      }
    : null
))
const blacklistActionErrorToast = computed(() => (
  blacklistEditor.actionError
    ? {
        key: `access-lists-blacklist:${blacklistEditor.actionError}`,
        level: 'warning' as const,
        message: blacklistEditor.actionError,
      }
    : null
))

useToastFeedback(whitelistActionErrorToast)
useToastFeedback(blacklistActionErrorToast)

const removeDescription = computed(() => {
  const candidate = removeCandidate.value
  if (!candidate) return ''
  return t('accessLists.confirm.removeDescription', {
    list: t(`accessLists.cards.${candidate.kind}Title`),
    type: candidate.entry.entry_type === 'user' ? t('accessLists.scopes.user') : t('accessLists.scopes.group'),
    target: candidate.entry.target_id,
    scope: governanceScopeLabel(candidate.entry.scope),
  })
})

async function loadAccessLists() {
  pageLoading.value = true
  pageLoadError.value = null

  const [blacklistResult, whitelistResult] = await Promise.allSettled([
    fetchList('blacklist'),
    fetchList('whitelist'),
  ])

  pageLoading.value = false

  if (blacklistResult.status === 'rejected' && whitelistResult.status === 'rejected') {
    pageLoadError.value = blacklistError.value ?? whitelistError.value ?? t('errors.common.loadFailed')
  }
}

async function applyWhitelistEnabled(enabled: boolean) {
  whitelistEditor.mutating = true
  whitelistEditor.actionError = null
  try {
    await governanceStore.setWhitelistEnabled(enabled)
    notifySuccess(t(enabled ? 'accessLists.feedback.whitelistEnabled' : 'accessLists.feedback.whitelistDisabled'))
  } catch (error) {
    whitelistEditor.actionError = getDisplayErrorMessage(error)
  } finally {
    whitelistEditor.mutating = false
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


onMounted(() => {
  void loadAccessLists()
})
</script>

<template>
  <AppPage :title="t('accessLists.title')" :description="t('accessLists.subtitle')" width="form">
    <template #extra>
      <div class="table-actions">
        <AppButton data-testid="access-lists-open-commands" @click="navigate(buildCommandsLocation())">
          {{ t('accessLists.actions.openCommands') }}
          <ArrowUpRightIcon class="access-lists-page__arrow" aria-hidden="true" />
        </AppButton>
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
      <AccessListCard
        kind="whitelist"
        :data="whitelist"
        :loading="whitelistLoading"
        :loading-more="whitelistLoadingMore"
        :error="whitelistError"
        :editor="whitelistEditor"
        @remove="removeCandidate = { kind: 'whitelist', entry: $event }; removeOpen = true"
        @more="governanceStore.loadMoreWhitelist().catch(() => undefined)"
        @retry="fetchList('whitelist').catch(() => undefined)"
      >
        <template #policy>
          <label class="access-lists-policy">
            <span>{{ t('accessLists.whitelist.enableLabel') }}</span>
            <AppSwitch
              :model-value="whitelistEnabled"
              :disabled="whitelistEditor.mutating"
              data-testid="access-lists-whitelist-enabled"
              @update:model-value="handleWhitelistToggle"
            />
          </label>
        </template>
        <template #notice>
          <AppAlert
            v-if="showWhitelistEmptyWarning"
            tone="warning"
            data-testid="access-lists-whitelist-empty-warning"
            :title="t('accessLists.whitelist.emptyWarningTitle')"
            :description="t('accessLists.whitelist.emptyWarningDescription')"
          />
        </template>
      </AccessListCard>
      <AccessListCard
        kind="blacklist"
        :data="blacklist"
        :loading="blacklistLoading"
        :loading-more="blacklistLoadingMore"
        :error="blacklistError"
        :editor="blacklistEditor"
        @remove="removeCandidate = { kind: 'blacklist', entry: $event }; removeOpen = true"
        @more="governanceStore.loadMoreBlacklist().catch(() => undefined)"
        @retry="fetchList('blacklist').catch(() => undefined)"
      />
    </div>

    <AppConfirmDialog
      :open="whitelistConfirmVisible"
      data-testid="access-lists-whitelist-confirm-modal"
      :title="t('accessLists.whitelist.enableConfirmTitle')"
      :description="t('accessLists.whitelist.enableConfirmDescription')"
      :confirm-text="t('accessLists.whitelist.enableConfirmAction')"
      :busy="whitelistEditor.mutating"
      @cancel="whitelistConfirmVisible = false"
      @confirm="confirmEmptyWhitelistEnable"
    />
    <AppConfirmDialog
      :open="removeOpen"
      :title="t('accessLists.confirm.removeTitle')"
      :description="removeDescription"
      :confirm-text="t('accessLists.entryForm.remove')"
      :busy="blacklistEditor.mutating || whitelistEditor.mutating"
      :fallback-focus="removeCandidate ? `[data-testid='access-lists-${removeCandidate.kind}-add-btn']` : undefined"
      danger
      @cancel="removeOpen = false"
      @after-close="removeCandidate = null"
      @confirm="removeCandidate && (removeCandidate.kind === 'whitelist' ? whitelistEditor.remove(removeCandidate.entry) : blacklistEditor.remove(removeCandidate.entry))"
    />
  </AppPage>
</template>

<style scoped lang="scss">
.access-lists-page__grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 24px;
}

.access-lists-page__arrow {
  width: 14px;
  height: 14px;
  color: var(--muted);
}

// The whole label toggles the switch, and names it for assistive technology.
.access-lists-policy {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: var(--text);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
}
</style>
