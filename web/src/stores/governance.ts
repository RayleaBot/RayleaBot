import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type {
  GovernanceBlacklistResponse,
  GovernanceCommandPolicyResponse,
  GovernanceEntryType,
  GovernanceEntryUpsertRequest,
  GovernanceWhitelistResponse,
} from '@/types/api'

export const useGovernanceStore = defineStore('governance', () => {
  const blacklist = ref<GovernanceBlacklistResponse | null>(null)
  const whitelist = ref<GovernanceWhitelistResponse | null>(null)
  const commandPolicy = ref<GovernanceCommandPolicyResponse | null>(null)
  const loading = ref(false)
  const blacklistLoading = ref(false)
  const whitelistLoading = ref(false)
  const commandPolicyLoading = ref(false)
  const error = ref<string | null>(null)
  const blacklistError = ref<string | null>(null)
  const whitelistError = ref<string | null>(null)
  const commandPolicyError = ref<string | null>(null)

  const hasData = computed(() => Boolean(blacklist.value || whitelist.value || commandPolicy.value))

  async function fetchBlacklist() {
    blacklistLoading.value = true
    blacklistError.value = null
    try {
      const response = await apiRequest<GovernanceBlacklistResponse>('/api/governance/blacklist')
      blacklist.value = response
      return response
    } catch (err) {
      blacklistError.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      blacklistLoading.value = false
    }
  }

  async function fetchWhitelist() {
    whitelistLoading.value = true
    whitelistError.value = null
    try {
      const response = await apiRequest<GovernanceWhitelistResponse>('/api/governance/whitelist')
      whitelist.value = response
      return response
    } catch (err) {
      whitelistError.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      whitelistLoading.value = false
    }
  }

  async function fetchCommandPolicy() {
    commandPolicyLoading.value = true
    commandPolicyError.value = null
    try {
      const response = await apiRequest<GovernanceCommandPolicyResponse>('/api/governance/command-policy')
      commandPolicy.value = response
      return response
    } catch (err) {
      commandPolicyError.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      commandPolicyLoading.value = false
    }
  }

  async function refresh() {
    loading.value = true
    error.value = null

    const [blacklistResult, whitelistResult, commandPolicyResult] = await Promise.allSettled([
      fetchBlacklist(),
      fetchWhitelist(),
      fetchCommandPolicy(),
    ])

    loading.value = false

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

  async function removeBlacklistEntry(entryType: GovernanceEntryType, targetId: string) {
    blacklistLoading.value = true
    blacklistError.value = null
    try {
      await apiRequest<void>(`/api/governance/blacklist/entries/${encodeURIComponent(entryType)}/${encodeURIComponent(targetId)}`, {
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

  async function removeWhitelistEntry(entryType: GovernanceEntryType, targetId: string) {
    whitelistLoading.value = true
    whitelistError.value = null
    try {
      await apiRequest<void>(`/api/governance/whitelist/entries/${encodeURIComponent(entryType)}/${encodeURIComponent(targetId)}`, {
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
