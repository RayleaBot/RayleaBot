import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { apiRequest } from '@/lib/http'
import { collectionURL, createCollectionPager, mergeCollectionItems, type CollectionQuery } from '@/lib/collection-pager'
import type {
  ThirdPartyAccountSummary,
  ThirdPartyAccountValidationResponse,
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
  const savingAccountId = ref<string | null>(null)
  const deletingAccountId = ref<string | null>(null)
  const validatingAccountIds = ref<string[]>([])
  const qrcodeCreating = ref(false)
  const qrcodePollingLoginId = ref<string | null>(null)

  const bilibiliAccounts = computed(() => accounts.value
    .filter((account) => account.platform === 'bilibili')
    .sort((left, right) => left.account_id.localeCompare(right.account_id)))
  const accountsByPlatform = computed(() => Object.fromEntries(thirdPartyPlatformOrder.map((platform) => [
    platform,
    accounts.value
      .filter((account) => account.platform === platform)
      .sort((left, right) => left.account_id.localeCompare(right.account_id)),
  ])) as Record<ThirdPartyPlatform, ThirdPartyAccountSummary[]>)

  const pager = createCollectionPager<ThirdPartyAccountsResponse>({
    request: (query, cursor, signal) => apiRequest(collectionURL('/api/third-party/accounts', query, cursor), { signal }),
    apply: (response, append) => { accounts.value = append ? mergeCollectionItems(accounts.value, response.items, item => accountOperationKey(item.platform, item.account_id)) : response.items },
  })
  const { total, nextCursor, loading, loadingMore, error } = pager
  async function fetchAll(signal?: AbortSignal, query?: CollectionQuery) { await pager.load(query, signal) }
  function search(query: CollectionQuery) { return fetchAll(undefined, query) }
  function loadMore() { return pager.loadMore() }

  async function saveAccount(platform: ThirdPartyPlatform, accountId: string, payload: ThirdPartyAccountUpsertRequest) {
    savingAccountId.value = accountOperationKey(platform, accountId)
    try {
      const response = await apiRequest<ThirdPartyAccountUpsertResponse>(
        `/api/third-party/accounts/${encodeURIComponent(platform)}/${encodeURIComponent(accountId)}`,
        { method: 'PUT', body: payload },
      )
      upsertAccount(response.account)
      await fetchAll()
      return response.account
    } finally {
      savingAccountId.value = null
    }
  }

  async function saveBilibiliAccount(accountId: string, payload: ThirdPartyAccountUpsertRequest) {
    return saveAccount('bilibili', accountId, payload)
  }

  async function validateAccount(platform: ThirdPartyPlatform, accountId: string) {
    const key = accountOperationKey(platform, accountId)
    if (!validatingAccountIds.value.includes(key)) {
      validatingAccountIds.value = [...validatingAccountIds.value, key]
    }
    try {
      const response = await apiRequest<ThirdPartyAccountValidationResponse>(
        `/api/third-party/accounts/${encodeURIComponent(platform)}/${encodeURIComponent(accountId)}/validate`,
        { method: 'POST' },
      )
      upsertAccount(response.account)
      return response.account
    } finally {
      validatingAccountIds.value = validatingAccountIds.value.filter((item) => item !== key)
    }
  }

  async function deleteAccount(platform: ThirdPartyPlatform, accountId: string) {
    deletingAccountId.value = accountOperationKey(platform, accountId)
    try {
      await apiRequest<void>(`/api/third-party/accounts/${encodeURIComponent(platform)}/${encodeURIComponent(accountId)}`, {
        method: 'DELETE',
      })
      await fetchAll()
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

  async function cancelQRCodeLogin(platform: ThirdPartyPlatform, loginId: string, keepalive = false) {
    await apiRequest<void>(
      `/api/third-party/accounts/${encodeURIComponent(platform)}/login/qrcode/${encodeURIComponent(loginId)}`,
      { method: 'DELETE', keepalive },
    )
  }

  async function applyQRCodeAccount(account: ThirdPartyAccountSummary | null | undefined) {
    if (!account) {
      return
    }
    upsertAccount(account)
    await fetchAll()
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

  return {
    total, nextCursor, loadingMore, loadMore, search,
    accounts,
    accountsByPlatform,
    bilibiliAccounts,
    deletingAccountId,
    error,
    loading,
    qrcodeCreating,
    qrcodePollingLoginId,
    savingAccountId,
    validatingAccountIds,
    cancelQRCodeLogin,
    createQRCodeLogin,
    deleteAccount,
    deleteBilibiliAccount,
    fetchAll,
    pollQRCodeLogin,
    saveAccount,
    saveBilibiliAccount,
    validateAccount,
  }
})
