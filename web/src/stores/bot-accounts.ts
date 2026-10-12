import { ref } from 'vue'
import { defineStore } from 'pinia'

import { apiPath } from '@/lib/api-path'
import { formatBotAccountKey } from '@/lib/bot-accounts'
import { collectionURL, createCollectionPager, mergeCollectionItems, type CollectionQuery } from '@/lib/collection-pager'
import { ApiError, apiRequest } from '@/lib/http'
import type { BotAccount, BotAccountListResponse, BotAccountRef } from '@/types/api'

export const useBotAccountsStore = defineStore('bot-accounts', () => {
  const accounts = ref<BotAccount[]>([])
  // Bumped on every directory change notice so views holding a single account reread it.
  const revision = ref(0)
  const pager = createCollectionPager<BotAccountListResponse>({
    request: (query, cursor, signal) => apiRequest(collectionURL('/api/bot-accounts', query, cursor), { signal }),
    apply: (response, append) => {
      accounts.value = append ? mergeCollectionItems(accounts.value, response.items, formatBotAccountKey) : response.items
    },
  })

  function fetchAccounts(query?: CollectionQuery, signal?: AbortSignal) {
    return pager.load(query, signal)
  }

  // fetchAccount reads one account whether or not it is on a loaded page; null means
  // the directory does not hold it.
  async function fetchAccount(account: BotAccountRef, signal?: AbortSignal): Promise<BotAccount | null> {
    try {
      return await apiRequest<BotAccount>(apiPath('/api/bot-accounts/{source_protocol}/{bot_id}', account), { signal })
    } catch (cause) {
      if (cause instanceof ApiError && cause.code === 'platform.resource_not_found') return null
      throw cause
    }
  }

  async function removeAccount(account: BotAccountRef) {
    await apiRequest<void>(apiPath('/api/bot-accounts/{source_protocol}/{bot_id}', account), { method: 'DELETE' })
    await refresh()
  }

  // refresh rereads the pages already loaded; a never loaded directory stays unloaded.
  async function refresh() {
    revision.value++
    if (pager.loaded.value) await pager.load()
  }

  return {
    accounts,
    revision,
    total: pager.total,
    nextCursor: pager.nextCursor,
    loading: pager.loading,
    loadingMore: pager.loadingMore,
    loaded: pager.loaded,
    error: pager.error,
    fetchAccounts,
    loadMore: () => pager.loadMore(),
    fetchAccount,
    removeAccount,
    refresh,
  }
})
