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
    trigger,
  }
})
