<script setup lang="ts">
import AppTag from '@/components/AppTag.vue'
import AppSwitch from '@/components/AppSwitch.vue'
import AppConfirmDialog from '@/components/AppConfirmDialog.vue'
import AppButton from '@/components/AppButton.vue'
import AppAlert from '@/components/AppAlert.vue'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'

import { notifySuccess, useToastFeedback } from '@/adapter/feedback'
import AppPage from '@/components/page/AppPage.vue'
import RetryPanel from '@/components/RetryPanel.vue'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { buildCommandsLocation } from '@/lib/management-links'
import { t } from '@/i18n'
import AccessListCard from '@/components/governance/AccessListCard.vue'
import { useAccessListEditor } from '@/components/governance/useAccessListEditor'
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

watch([() => blacklistEditor.searchQuery, () => blacklistEditor.scopeFilter], () => { void governanceStore.fetchBlacklist(undefined, { query: blacklistEditor.searchQuery, entry_type: blacklistEditor.scopeFilter === 'all' ? undefined : blacklistEditor.scopeFilter }).catch(() => undefined) })
watch([() => whitelistEditor.searchQuery, () => whitelistEditor.scopeFilter], () => { void governanceStore.fetchWhitelist(undefined, { query: whitelistEditor.searchQuery, entry_type: whitelistEditor.scopeFilter === 'all' ? undefined : whitelistEditor.scopeFilter }).catch(() => undefined) })

const hasAccessListData = computed(() => Boolean(blacklist.value || whitelist.value))
const pageBusy = computed(() => pageLoading.value || blacklistLoading.value || whitelistLoading.value)
const pageErrorMessage = computed(() => pageLoadError.value ?? blacklistError.value ?? whitelistError.value)
const showFatalError = computed(() => Boolean(pageErrorMessage.value) && !hasAccessListData.value)

const totalWhitelistEntries = computed(() => whitelist.value?.entry_count ?? 0)
const whitelistEnabled = computed(() => whitelist.value?.enabled ?? false)
const showWhitelistEmptyWarning = computed(() => whitelistEnabled.value && totalWhitelistEntries.value === 0)

const blacklistRegionError = computed(() => blacklistEditor.actionError ?? blacklistError.value)
const whitelistRegionError = computed(() => whitelistEditor.actionError ?? whitelistError.value)
const whitelistRegionErrorToast = computed(() => (
  whitelistRegionError.value
    ? {
        key: `access-lists-whitelist:${whitelistRegionError.value}`,
        level: 'warning' as const,
        message: whitelistRegionError.value,
      }
    : null
))
const blacklistRegionErrorToast = computed(() => (
  blacklistRegionError.value
    ? {
        key: `access-lists-blacklist:${blacklistRegionError.value}`,
        level: 'warning' as const,
        message: blacklistRegionError.value,
      }
    : null
))
const whitelistEmptyToast = computed(() => (
  showWhitelistEmptyWarning.value
    ? {
        key: 'access-lists-whitelist-empty',
        level: 'warning' as const,
        message: `${t('accessLists.whitelist.emptyWarningTitle')}：${t('accessLists.whitelist.emptyWarningDescription')}`,
      }
    : null
))

useToastFeedback(whitelistRegionErrorToast)
useToastFeedback(blacklistRegionErrorToast)
useToastFeedback(whitelistEmptyToast)

async function loadAccessLists() {
  pageLoading.value = true
  pageLoadError.value = null

  const [blacklistResult, whitelistResult] = await Promise.allSettled([
    governanceStore.fetchBlacklist(undefined, { query: blacklistEditor.searchQuery, entry_type: blacklistEditor.scopeFilter === 'all' ? undefined : blacklistEditor.scopeFilter }),
    governanceStore.fetchWhitelist(undefined, { query: whitelistEditor.searchQuery, entry_type: whitelistEditor.scopeFilter === 'all' ? undefined : whitelistEditor.scopeFilter }),
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
        <AppButton data-testid="access-lists-open-commands" variant="default" @click="navigate(buildCommandsLocation())">
          {{ t('accessLists.actions.openCommands') }}
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
        :editor="whitelistEditor"
        @remove="removeCandidate = { kind: 'whitelist', entry: $event }; removeOpen = true"
        @more="governanceStore.loadMoreWhitelist().catch(() => undefined)"
      >
        <template #policy>
          <AppTag :tone="whitelistEnabled ? 'warning' : 'neutral'">
            {{ whitelistEnabled ? t('accessLists.summary.whitelistEnabled') : t('accessLists.summary.whitelistDisabled') }}
          </AppTag>
          <AppSwitch
            :model-value="whitelistEnabled"
            :disabled="whitelistEditor.mutating"
            :aria-label="t('accessLists.summary.whitelistStatus')"
            data-testid="access-lists-whitelist-enabled"
            @update:model-value="handleWhitelistToggle"
          />
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
        :editor="blacklistEditor"
        @remove="removeCandidate = { kind: 'blacklist', entry: $event }; removeOpen = true"
        @more="governanceStore.loadMoreBlacklist().catch(() => undefined)"
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
      :description="t('accessLists.confirm.removeDescription')"
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
</style>
