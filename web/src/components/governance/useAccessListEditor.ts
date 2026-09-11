import { computed, reactive, ref, type Ref, type UnwrapRef } from 'vue'
import { notifySuccess } from '@/adapter/feedback'
import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { oneBotGlobalScope } from '@/lib/governance-scope'
import type { BlacklistEntry, GovernanceEntryType, GovernanceEntryUpsertRequest } from '@/types/api'

export function useAccessListEditor(options: {
  kind: 'blacklist' | 'whitelist'
  entries: Ref<BlacklistEntry[]>
  add: (entry: GovernanceEntryUpsertRequest) => Promise<unknown>
  remove: (entry: BlacklistEntry) => Promise<unknown>
  removed: () => void
}) {
  const searchQuery = ref('')
  const scopeFilter = ref<'all' | GovernanceEntryType>('all')
  const isAdding = ref(false)
  const adding = ref(false)
  const mutating = ref(false)
  const actionError = ref<string | null>(null)
  const draft = reactive({ scope: oneBotGlobalScope(), entry_type: 'user' as GovernanceEntryType, target_id: '', reason: '' })
  const draftErrors = reactive({ scope: '', target_id: '', reason: '' })
  const filteredEntries = computed(() => options.entries.value)
  const tableData = computed(() => {
    const entries: (BlacklistEntry & { isDraft?: boolean })[] = [...filteredEntries.value]
    if (isAdding.value) entries.unshift({ ...draft, scope: { ...draft.scope }, target_id: `__${options.kind}_draft__`, created_at: '', isDraft: true })
    return entries
  })

  function resetDraft() {
    draft.target_id = ''
    draft.reason = ''
    Object.assign(draftErrors, { scope: '', target_id: '', reason: '' })
  }
  function startAdd() {
    resetDraft()
    draft.entry_type = scopeFilter.value === 'all' ? 'user' : scopeFilter.value
    isAdding.value = true
  }
  function cancel() {
    isAdding.value = false
    resetDraft()
  }
  async function save() {
    const targetID = draft.target_id.trim()
    const reason = draft.reason.trim()
    draftErrors.scope = draft.scope.kind === 'instance' && (!draft.scope.source_adapter.trim() || !draft.scope.bot_id.trim()) ? t('accessLists.namespace.required') : ''
    draftErrors.target_id = targetID ? '' : t('accessLists.validation.entryRequired')
    draftErrors.reason = reason ? '' : t('accessLists.validation.entryRequired')
    if (Object.values(draftErrors).some(Boolean)) return
    adding.value = true
    actionError.value = null
    try {
      await options.add({ scope: { ...draft.scope }, entry_type: draft.entry_type, target_id: targetID, reason })
      cancel()
      notifySuccess(t(options.kind === 'blacklist' ? 'accessLists.feedback.blacklistSaved' : 'accessLists.feedback.whitelistSaved'))
    } catch (error) {
      actionError.value = getDisplayErrorMessage(error)
    } finally {
      adding.value = false
    }
  }
  async function remove(entry: BlacklistEntry) {
    mutating.value = true
    actionError.value = null
    try {
      await options.remove(entry)
      options.removed()
      notifySuccess(t(options.kind === 'blacklist' ? 'accessLists.feedback.blacklistRemoved' : 'accessLists.feedback.whitelistRemoved'))
    } catch (error) {
      actionError.value = getDisplayErrorMessage(error)
    } finally {
      mutating.value = false
    }
  }
  return { searchQuery, scopeFilter, isAdding, adding, mutating, actionError, draft, draftErrors, filteredEntries, tableData, startAdd, cancel, save, remove }
}

export type AccessListEditor = UnwrapRef<ReturnType<typeof useAccessListEditor>>
