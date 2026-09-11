import { apiPath } from '@/lib/api-path'
import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import { collectionURL, createCollectionPager, mergeCollectionItems, type CollectionQuery } from '@/lib/collection-pager'
import { governanceEntryKey } from '@/lib/governance-scope'
import type {
  GovernanceBlacklistResponse,
  GovernanceCommandPolicyResponse,
  GovernanceEntryType,
  GovernanceScope,
  GovernanceEntryUpsertRequest,
  GovernanceWhitelistResponse,
} from '@/types/api'

export const useGovernanceStore = defineStore('governance', () => {
  const blacklist = ref<GovernanceBlacklistResponse | null>(null)
  const whitelist = ref<GovernanceWhitelistResponse | null>(null)
  const commandPolicy = ref<GovernanceCommandPolicyResponse | null>(null)
  const loading = ref(false)
  const commandPolicyLoading = ref(false)
  const error = ref<string | null>(null)
  const commandPolicyError = ref<string | null>(null)

  let commandPolicyRequestID = 0
  let refreshRequestID = 0

  const hasData = computed(() => Boolean(blacklist.value || whitelist.value || commandPolicy.value))

  const blacklistPager = createCollectionPager<GovernanceBlacklistResponse>({
    request: (query, cursor, signal) => apiRequest(collectionURL('/api/governance/blacklist', query, cursor), { signal }),
    apply: (response, append) => { blacklist.value = { ...response,
      user_entries: append ? mergeCollectionItems(blacklist.value?.user_entries ?? [], response.user_entries, governanceEntryKey) : response.user_entries,
      group_entries: append ? mergeCollectionItems(blacklist.value?.group_entries ?? [], response.group_entries, governanceEntryKey) : response.group_entries,
    } },
  })
  const whitelistPager = createCollectionPager<GovernanceWhitelistResponse>({
    request: (query, cursor, signal) => apiRequest(collectionURL('/api/governance/whitelist', query, cursor), { signal }),
    apply: (response, append) => { whitelist.value = { ...response,
      user_entries: append ? mergeCollectionItems(whitelist.value?.user_entries ?? [], response.user_entries, governanceEntryKey) : response.user_entries,
      group_entries: append ? mergeCollectionItems(whitelist.value?.group_entries ?? [], response.group_entries, governanceEntryKey) : response.group_entries,
    } },
  })
  const { loading: blacklistLoading, error: blacklistError, loadingMore: blacklistLoadingMore } = blacklistPager
  const { loading: whitelistLoading, error: whitelistError, loadingMore: whitelistLoadingMore } = whitelistPager
  function fetchBlacklist(signal?: AbortSignal, query?: CollectionQuery) { return blacklistPager.load(query, signal) }
  function fetchWhitelist(signal?: AbortSignal, query?: CollectionQuery) { return whitelistPager.load(query, signal) }
  function loadMoreBlacklist() { return blacklistPager.loadMore() }
  function loadMoreWhitelist() { return whitelistPager.loadMore() }

  async function fetchCommandPolicy(signal?: AbortSignal) {
    const requestID = ++commandPolicyRequestID
    commandPolicyLoading.value = true
    commandPolicyError.value = null
    try {
      const response = await apiRequest<GovernanceCommandPolicyResponse>('/api/governance/command-policy', { signal })
      signal?.throwIfAborted()
      if (requestID === commandPolicyRequestID) commandPolicy.value = response
      return response
    } catch (err) {
      if (!signal?.aborted && requestID === commandPolicyRequestID) commandPolicyError.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      if (requestID === commandPolicyRequestID) commandPolicyLoading.value = false
    }
  }

  async function refresh(signal?: AbortSignal) {
    const requestID = ++refreshRequestID
    loading.value = true
    error.value = null

    const [blacklistResult, whitelistResult, commandPolicyResult] = await Promise.allSettled([
      fetchBlacklist(signal),
      fetchWhitelist(signal),
      fetchCommandPolicy(signal),
    ])

    if (requestID === refreshRequestID) loading.value = false
    signal?.throwIfAborted()
    if (requestID !== refreshRequestID) return

    if (
      blacklistResult.status === 'rejected'
      && whitelistResult.status === 'rejected'
      && commandPolicyResult.status === 'rejected'
    ) {
      error.value = blacklistError.value
        ?? whitelistError.value
        ?? commandPolicyError.value
        ?? '读取未完成，请稍后重试。'
      throw blacklistResult.reason ?? whitelistResult.reason ?? commandPolicyResult.reason
    }

    return {
      blacklist: blacklistResult.status === 'fulfilled' ? blacklistResult.value : null,
      whitelist: whitelistResult.status === 'fulfilled' ? whitelistResult.value : null,
      commandPolicy: commandPolicyResult.status === 'fulfilled' ? commandPolicyResult.value : null,
    }
  }

  async function addBlacklistEntry(payload: GovernanceEntryUpsertRequest) {
    blacklistLoading.value = true
    blacklistError.value = null
    try {
      await apiRequest('/api/governance/blacklist/entries', {
        method: 'POST',
        body: payload,
      })
      return await fetchBlacklist()
    } catch (err) {
      blacklistError.value = getDisplayErrorMessage(err)
      throw err
    } finally {
      blacklistLoading.value = false
    }
  }

  async function removeBlacklistEntry(entryType: GovernanceEntryType, targetId: string, scope: GovernanceScope) {
    blacklistLoading.value = true
    blacklistError.value = null
    try {
      await apiRequest<void>(apiPath('/api/governance/blacklist/entries/{entry_type}/{target_id}', { entry_type: entryType, target_id: targetId }, new URLSearchParams(scope)), {
        method: 'DELETE',
      })
      return await fetchBlacklist()
    } catch (err) {
      blacklistError.value = getDisplayErrorMessage(err)
      throw err
    } finally {
      blacklistLoading.value = false
    }
  }

  async function setWhitelistEnabled(enabled: boolean) {
    whitelistLoading.value = true
    whitelistError.value = null
    try {
      await apiRequest('/api/governance/whitelist/state', {
        method: 'PUT',
        body: { enabled },
      })
      return await fetchWhitelist()
    } catch (err) {
      whitelistError.value = getDisplayErrorMessage(err)
      throw err
    } finally {
      whitelistLoading.value = false
    }
  }

  async function addWhitelistEntry(payload: GovernanceEntryUpsertRequest) {
    whitelistLoading.value = true
    whitelistError.value = null
    try {
      await apiRequest('/api/governance/whitelist/entries', {
        method: 'POST',
        body: payload,
      })
      return await fetchWhitelist()
    } catch (err) {
      whitelistError.value = getDisplayErrorMessage(err)
      throw err
    } finally {
      whitelistLoading.value = false
    }
  }

  async function removeWhitelistEntry(entryType: GovernanceEntryType, targetId: string, scope: GovernanceScope) {
    whitelistLoading.value = true
    whitelistError.value = null
    try {
      await apiRequest<void>(apiPath('/api/governance/whitelist/entries/{entry_type}/{target_id}', { entry_type: entryType, target_id: targetId }, new URLSearchParams(scope)), {
        method: 'DELETE',
      })
      return await fetchWhitelist()
    } catch (err) {
      whitelistError.value = getDisplayErrorMessage(err)
      throw err
    } finally {
      whitelistLoading.value = false
    }
  }

  return {
    blacklistLoadingMore, whitelistLoadingMore, loadMoreBlacklist, loadMoreWhitelist,
    blacklist,
    whitelist,
    commandPolicy,
    loading,
    blacklistLoading,
    whitelistLoading,
    commandPolicyLoading,
    error,
    blacklistError,
    whitelistError,
    commandPolicyError,
    hasData,
    fetchBlacklist,
    fetchWhitelist,
    fetchCommandPolicy,
    refresh,
    addBlacklistEntry,
    removeBlacklistEntry,
    setWhitelistEnabled,
    addWhitelistEntry,
    removeWhitelistEntry,
  }
})
