import { computed, onScopeDispose, ref } from 'vue'
import { defineStore } from 'pinia'

import { apiRequest } from '@/lib/http'
import { collectionURL, createCollectionPager, mergeCollectionItems, type CollectionQuery } from '@/lib/collection-pager'
import { createRefreshScheduler } from '@/lib/refresh-scheduler'
import type { SchedulerJobListResponse, SchedulerJobSummary, SchedulerJobTriggerResponse } from '@/types/api'

export const useSchedulerJobsStore = defineStore('scheduler-jobs', () => {
  const items = ref<SchedulerJobSummary[]>([])
  const triggeringJobId = ref<string | null>(null)
  const liveRefreshActive = ref(false)

  let lastRequest: Promise<void> | null = null
  const liveRefresh = createRefreshScheduler(async signal => {
    if (lastRequest) await lastRequest.catch(() => undefined)
    signal.throwIfAborted()
    if (liveRefreshActive.value) await fetchList(signal)
  })
  onScopeDispose(cancelDataSourceRefresh)

  const pager = createCollectionPager<SchedulerJobListResponse>({
    request: (query, cursor, signal) => apiRequest(collectionURL('/api/system/scheduler/jobs', query, cursor), { signal }),
    apply: (response, append) => { items.value = append ? mergeCollectionItems(items.value, response.items, item => item.job_id) : response.items },
  })
  const { total, nextCursor, loading, loadingMore, error } = pager
  const sortedItems = computed(() => items.value)
  async function fetchList(signal?: AbortSignal, query?: CollectionQuery) {
    const pending = pager.load(query, signal).then(() => undefined)
    lastRequest = pending
    try { await pending } finally { if (lastRequest === pending) lastRequest = null }
  }
  function search(query: CollectionQuery) { return fetchList(undefined, query) }
  function loadMore() { return pager.loadMore() }

  function scheduleDataSourceRefresh() {
    if (liveRefreshActive.value) liveRefresh.schedule()
  }

  function cancelDataSourceRefresh() {
    liveRefresh.cancel()
    pager.cancel()
  }

  function setLiveRefreshActive(active: boolean) {
    liveRefreshActive.value = active
    if (!active) cancelDataSourceRefresh()
  }

  async function trigger(jobId: string) {
    triggeringJobId.value = jobId
    try {
      const response = await apiRequest<SchedulerJobTriggerResponse>(`/api/system/scheduler/jobs/${encodeURIComponent(jobId)}/trigger`, {
        method: 'POST',
      })
      await fetchList()
      return response
    } finally {
      triggeringJobId.value = null
    }
  }

  return {
    total, nextCursor, loadingMore, loadMore, search,
    error,
    items,
    loading,
    sortedItems,
    triggeringJobId,
    fetchList,
    scheduleDataSourceRefresh,
    cancelDataSourceRefresh,
    setLiveRefreshActive,
    trigger,
  }
})
