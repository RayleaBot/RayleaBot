import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type { GovernanceBlacklistResponse, GovernanceCommandPolicyResponse } from '@/types/api'

export const useGovernanceStore = defineStore('governance', () => {
  const blacklist = ref<GovernanceBlacklistResponse | null>(null)
  const commandPolicy = ref<GovernanceCommandPolicyResponse | null>(null)
  const loading = ref(false)
  const blacklistLoading = ref(false)
  const commandPolicyLoading = ref(false)
  const error = ref<string | null>(null)
  const blacklistError = ref<string | null>(null)
  const commandPolicyError = ref<string | null>(null)

  const hasData = computed(() => Boolean(blacklist.value || commandPolicy.value))

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

    const [blacklistResult, commandPolicyResult] = await Promise.allSettled([
      fetchBlacklist(),
      fetchCommandPolicy(),
    ])

    loading.value = false

    if (blacklistResult.status === 'rejected' && commandPolicyResult.status === 'rejected') {
      error.value = blacklistError.value ?? commandPolicyError.value ?? '读取未完成，请稍后重试。'
      throw blacklistResult.reason ?? commandPolicyResult.reason
    }

    return {
      blacklist: blacklistResult.status === 'fulfilled' ? blacklistResult.value : null,
      commandPolicy: commandPolicyResult.status === 'fulfilled' ? commandPolicyResult.value : null,
    }
  }

  return {
    blacklist,
    commandPolicy,
    loading,
    blacklistLoading,
    commandPolicyLoading,
    error,
    blacklistError,
    commandPolicyError,
    hasData,
    fetchBlacklist,
    fetchCommandPolicy,
    refresh,
  }
})
