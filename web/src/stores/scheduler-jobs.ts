import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type { SchedulerJobListResponse, SchedulerJobSummary, SchedulerJobTriggerResponse } from '@/types/api'

export const useSchedulerJobsStore = defineStore('scheduler-jobs', () => {
  const items = ref<SchedulerJobSummary[]>([])
  const loading = ref(false)
  const triggeringJobId = ref<string | null>(null)
  const error = ref<string | null>(null)
  const liveRefreshActive = ref(false)

  const liveRefreshDebounceMs = 120
  let liveRefreshHandle: ReturnType<typeof window.setTimeout> | null = null
  let liveRefreshInFlight = false
  let liveRefreshQueued = false

  const sortedItems = computed(() => (
    [...items.value].sort((left, right) => {
      if (left.plugin_name === right.plugin_name) {
        return left.task_name.localeCompare(right.task_name)
      }
      return left.plugin_name.localeCompare(right.plugin_name)
    })
  ))

  async function fetchList() {
    loading.value = true
    error.value = null
    try {
      const response = await apiRequest<SchedulerJobListResponse>('/api/system/scheduler/jobs')
      items.value = response.items
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      loading.value = false
      if (liveRefreshQueued && liveRefreshActive.value && !liveRefreshInFlight) {
        liveRefreshQueued = false
        scheduleDataSourceRefresh()
      }
    }
  }

  async function runDataSourceRefresh() {
    if (!liveRefreshActive.value) {
      return
    }
    if (loading.value) {
      liveRefreshQueued = true
      return
    }
    if (liveRefreshInFlight) {
      liveRefreshQueued = true
      return
    }

    liveRefreshInFlight = true
    try {
      await fetchList()
    } catch {
      return
    } finally {
      liveRefreshInFlight = false
      if (liveRefreshQueued) {
        liveRefreshQueued = false
        scheduleDataSourceRefresh()
      }
    }
  }

  function scheduleDataSourceRefresh() {
    if (!liveRefreshActive.value) {
      return
    }
    if (liveRefreshInFlight) {
      liveRefreshQueued = true
      return
    }
    if (liveRefreshHandle !== null) {
      return
    }

    liveRefreshHandle = window.setTimeout(() => {
      liveRefreshHandle = null
      void runDataSourceRefresh()
    }, liveRefreshDebounceMs)
  }

  function setLiveRefreshActive(active: boolean) {
    liveRefreshActive.value = active
    if (!active && liveRefreshHandle !== null) {
      window.clearTimeout(liveRefreshHandle)
      liveRefreshHandle = null
    }
    if (!active) {
      liveRefreshQueued = false
    }
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
    error,
    items,
    loading,
    sortedItems,
    triggeringJobId,
    fetchList,
    scheduleDataSourceRefresh,
    setLiveRefreshActive,
    trigger,
  }
})
