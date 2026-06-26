import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type {
  ThirdPartyAccountSummary,
  ThirdPartyAccountUpsertRequest,
  ThirdPartyAccountUpsertResponse,
  ThirdPartyAccountsResponse,
  ThirdPartyQRCodeLoginCreateResponse,
  ThirdPartyQRCodeLoginPollResponse,
} from '@/types/api'

export type ThirdPartyPlatform = ThirdPartyAccountSummary['platform']

export const thirdPartyPlatformOrder = ['bilibili', 'weibo', 'douyin', 'netease_music'] as const satisfies readonly ThirdPartyPlatform[]

export const thirdPartyPlatformLabels: Record<ThirdPartyPlatform, string> = {
  bilibili: 'Bilibili',
  weibo: '微博',
  douyin: '抖音',
  netease_music: '网易云音乐',
}

export const useThirdPartyAccountsStore = defineStore('third-party-accounts', () => {
  const accounts = ref<ThirdPartyAccountSummary[]>([])
  const loading = ref(false)
  const savingAccountId = ref<string | null>(null)
  const deletingAccountId = ref<string | null>(null)
  const qrcodeCreating = ref(false)
  const qrcodePollingLoginId = ref<string | null>(null)
  const error = ref<string | null>(null)

  const bilibiliAccounts = computed(() => accounts.value
    .filter((account) => account.platform === 'bilibili')
    .sort((left, right) => left.account_id.localeCompare(right.account_id)))
  const accountsByPlatform = computed(() => Object.fromEntries(thirdPartyPlatformOrder.map((platform) => [
    platform,
    accounts.value
      .filter((account) => account.platform === platform)
      .sort((left, right) => left.account_id.localeCompare(right.account_id)),
  ])) as Record<ThirdPartyPlatform, ThirdPartyAccountSummary[]>)

  async function fetchAll() {
    loading.value = true
    error.value = null
    try {
      const accountsResponse = await apiRequest<ThirdPartyAccountsResponse>('/api/third-party/accounts')
      accounts.value = accountsResponse.items
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      loading.value = false
    }
  }

  async function saveAccount(platform: ThirdPartyPlatform, accountId: string, payload: ThirdPartyAccountUpsertRequest) {
    savingAccountId.value = accountOperationKey(platform, accountId)
    try {
      const response = await apiRequest<ThirdPartyAccountUpsertResponse>(
        `/api/third-party/accounts/${encodeURIComponent(platform)}/${encodeURIComponent(accountId)}`,
        { method: 'PUT', body: payload },
      )
      upsertAccount(response.account)
      return response.account
    } finally {
      savingAccountId.value = null
    }
  }

  async function saveBilibiliAccount(accountId: string, payload: ThirdPartyAccountUpsertRequest) {
    return saveAccount('bilibili', accountId, payload)
  }

  async function deleteAccount(platform: ThirdPartyPlatform, accountId: string) {
    deletingAccountId.value = accountOperationKey(platform, accountId)
    try {
      await apiRequest<void>(`/api/third-party/accounts/${encodeURIComponent(platform)}/${encodeURIComponent(accountId)}`, {
        method: 'DELETE',
      })
      accounts.value = accounts.value.filter((account) => account.platform !== platform || account.account_id !== accountId)
    } finally {
      deletingAccountId.value = null
    }
  }

  async function deleteBilibiliAccount(accountId: string) {
    return deleteAccount('bilibili', accountId)
  }

  async function createQRCodeLogin(platform: ThirdPartyPlatform): Promise<ThirdPartyQRCodeLoginCreateResponse> {
    qrcodeCreating.value = true
    try {
      return await apiRequest<ThirdPartyQRCodeLoginCreateResponse>(
        `/api/third-party/accounts/${encodeURIComponent(platform)}/login/qrcode`,
        { method: 'POST' },
      )
    } finally {
      qrcodeCreating.value = false
    }
  }

  async function pollQRCodeLogin(platform: ThirdPartyPlatform, loginId: string): Promise<ThirdPartyQRCodeLoginPollResponse> {
    qrcodePollingLoginId.value = loginId
    try {
      const response = await apiRequest<ThirdPartyQRCodeLoginPollResponse>(
        `/api/third-party/accounts/${encodeURIComponent(platform)}/login/qrcode/${encodeURIComponent(loginId)}`,
      )
      await applyQRCodeAccount(response.account)
      return response
    } finally {
      qrcodePollingLoginId.value = null
    }
  }

  async function applyQRCodeAccount(account: ThirdPartyAccountSummary | null | undefined) {
    if (!account) {
      return
    }
    upsertAccount(account)
  }

  function upsertAccount(account: ThirdPartyAccountSummary) {
    const index = accounts.value.findIndex((item) => item.platform === account.platform && item.account_id === account.account_id)
    if (index === -1) {
      accounts.value = [...accounts.value, account]
      return
    }
    accounts.value = accounts.value.map((item, itemIndex) => (itemIndex === index ? account : item))
  }

  function accountOperationKey(platform: ThirdPartyPlatform, accountId: string) {
    return `${platform}:${accountId}`
  }

  function disposeMedia() {
  }

  return {
    accounts,
    accountsByPlatform,
    bilibiliAccounts,
    deletingAccountId,
    error,
    loading,
    qrcodeCreating,
    qrcodePollingLoginId,
    savingAccountId,
    createQRCodeLogin,
    deleteAccount,
    deleteBilibiliAccount,
    disposeMedia,
    fetchAll,
    pollQRCodeLogin,
    saveAccount,
    saveBilibiliAccount,
  }
})
