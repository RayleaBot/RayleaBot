import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { apiRequest } from '@/lib/http'
import type { TaskAcceptedResponse, TaskDetailResponse, TaskListResponse, TaskSummary } from '@/types/api'

export const useTasksStore = defineStore('tasks', () => {
  const items = ref<TaskSummary[]>([])
  const currentTask = ref<TaskSummary | null>(null)
  const loading = ref(false)
  const detailLoading = ref(false)
  const cancelPending = ref(false)
  const error = ref<string | null>(null)

  const sortedItems = computed(() => [...items.value].sort((left, right) => left.task_id.localeCompare(right.task_id)))

  async function fetchList(filters: { status?: string; taskType?: string; limit?: number } = {}) {
    loading.value = true
    error.value = null
    try {
      const params = new URLSearchParams()
      if (filters.status) {
        params.set('status', filters.status)
      }
      if (filters.taskType) {
        params.set('task_type', filters.taskType)
      }
      if (filters.limit) {
        params.set('limit', String(filters.limit))
      }

      const suffix = params.size > 0 ? `?${params}` : ''
      const response = await apiRequest<TaskListResponse>(`/api/tasks${suffix}`)
      items.value = response.items
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'task list failed'
      throw err
    } finally {
      loading.value = false
    }
  }

  async function fetchDetail(taskId: string) {
    detailLoading.value = true
    try {
      const response = await apiRequest<TaskDetailResponse>(`/api/tasks/${taskId}`)
      currentTask.value = response.task
      upsert(response.task)
      return response.task
    } finally {
      detailLoading.value = false
    }
  }

  async function cancelTask(taskId: string) {
    cancelPending.value = true
    try {
      return await apiRequest<TaskAcceptedResponse>(`/api/tasks/${taskId}/cancel`, {
        method: 'POST',
      })
    } finally {
      cancelPending.value = false
    }
  }

  function upsert(task: TaskSummary) {
    const index = items.value.findIndex((item) => item.task_id === task.task_id)
    if (index === -1) {
      items.value = [task, ...items.value]
    } else {
      items.value = items.value.map((item, itemIndex) => (itemIndex === index ? task : item))
    }

    if (currentTask.value?.task_id === task.task_id) {
      currentTask.value = task
    }
  }

  function clearCurrentTask() {
    currentTask.value = null
  }

  return {
    cancelPending,
    clearCurrentTask,
    currentTask,
    detailLoading,
    error,
    items,
    loading,
    sortedItems,
    cancelTask,
    fetchDetail,
    fetchList,
    upsert,
  }
})
