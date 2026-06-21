import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiDownload, apiRequest } from '@/lib/http'
import type {
  BilibiliQRCodeLoginCreateResponse,
  BilibiliQRCodeLoginPollResponse,
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
  const mediaObjectURLs = new Map<string, string>()

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
      accounts.value = await resolveAccountMedia(accountsResponse.items)
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
      const account = await resolveAccountMediaItem(response.account)
      upsertAccount(account)
      return account
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

  async function createBilibiliQRCodeLogin() {
    qrcodeCreating.value = true
    try {
      return await apiRequest<BilibiliQRCodeLoginCreateResponse>('/api/bilibili/login/qrcode', {
        method: 'POST',
      })
    } finally {
      qrcodeCreating.value = false
    }
  }

  async function createQRCodeLogin(platform: ThirdPartyPlatform): Promise<ThirdPartyQRCodeLoginCreateResponse> {
    if (platform === 'bilibili') {
      const response = await createBilibiliQRCodeLogin()
      return { ...response, platform }
    }
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

  async function pollBilibiliQRCodeLogin(loginId: string) {
    qrcodePollingLoginId.value = loginId
    try {
      return await apiRequest<BilibiliQRCodeLoginPollResponse>(
        `/api/bilibili/login/qrcode/${encodeURIComponent(loginId)}`,
      )
    } finally {
      qrcodePollingLoginId.value = null
    }
  }

  async function pollQRCodeLogin(platform: ThirdPartyPlatform, loginId: string): Promise<ThirdPartyQRCodeLoginPollResponse> {
    if (platform === 'bilibili') {
      const response = await pollBilibiliQRCodeLogin(loginId)
      return { ...response, platform }
    }
    qrcodePollingLoginId.value = loginId
    try {
      return await apiRequest<ThirdPartyQRCodeLoginPollResponse>(
        `/api/third-party/accounts/${encodeURIComponent(platform)}/login/qrcode/${encodeURIComponent(loginId)}`,
      )
    } finally {
      qrcodePollingLoginId.value = null
    }
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

  async function resolveAccountMedia(items: ThirdPartyAccountSummary[]) {
    return Promise.all(items.map(resolveAccountMediaItem))
  }

  async function resolveAccountMediaItem(account: ThirdPartyAccountSummary) {
    const avatarURL = await downloadThirdPartyMedia(account.profile?.avatar_url || '')
    if (!avatarURL || !account.profile || avatarURL === account.profile.avatar_url) {
      return account
    }
    return {
      ...account,
      profile: {
        ...account.profile,
        avatar_url: avatarURL,
      },
    }
  }

  async function downloadThirdPartyMedia(url: string) {
    const normalizedURL = normalizeThirdPartyMediaURL(url)
    if (!normalizedURL) {
      return url
    }
    const cached = mediaObjectURLs.get(normalizedURL)
    if (cached) {
      return cached
    }
    try {
      const { blob } = await apiDownload(`/api/third-party/media?url=${encodeURIComponent(normalizedURL)}`)
      const objectURL = window.URL.createObjectURL(blob)
      mediaObjectURLs.set(normalizedURL, objectURL)
      return objectURL
    } catch {
      return url
    }
  }

  function disposeMedia() {
    for (const objectURL of mediaObjectURLs.values()) {
      window.URL.revokeObjectURL(objectURL)
    }
    mediaObjectURLs.clear()
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
    createBilibiliQRCodeLogin,
    createQRCodeLogin,
    deleteAccount,
    deleteBilibiliAccount,
    disposeMedia,
    fetchAll,
    pollBilibiliQRCodeLogin,
    pollQRCodeLogin,
    saveAccount,
    saveBilibiliAccount,
  }
})

function normalizeThirdPartyMediaURL(value: string) {
  const text = value.trim()
  if (!text) {
    return ''
  }
  try {
    const parsed = new URL(text.startsWith('//') ? `https:${text}` : text)
    if ((parsed.protocol !== 'https:' && parsed.protocol !== 'http:') || !isSupportedThirdPartyMediaHost(parsed.hostname)) {
      return ''
    }
    if (isBilibiliMediaHost(parsed.hostname) && !parsed.pathname.startsWith('/bfs/') && !parsed.pathname.startsWith('/fs/')) {
      return ''
    }
    if (isWeiboMediaHost(parsed.hostname) && (!parsed.pathname || parsed.pathname === '/')) {
      return ''
    }
    parsed.protocol = 'https:'
    parsed.search = ''
    parsed.hash = ''
    return parsed.toString()
  } catch {
    return ''
  }
}

function isSupportedThirdPartyMediaHost(hostname: string) {
  return isBilibiliMediaHost(hostname) || isWeiboMediaHost(hostname)
}

function isBilibiliMediaHost(hostname: string) {
  const host = hostname.toLowerCase()
  return host === 'hdslb.com' || host.endsWith('.hdslb.com')
}

function isWeiboMediaHost(hostname: string) {
  const host = hostname.toLowerCase()
  return host === 'sinaimg.cn' || host.endsWith('.sinaimg.cn')
}
