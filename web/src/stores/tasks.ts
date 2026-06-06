import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type { TaskAcceptedResponse, TaskDetailResponse, TaskListResponse, TaskSummary } from '@/types/api'

const IN_PROGRESS_TASK_STATUSES = new Set(['pending', 'running'])

function taskTimeMs(task: TaskSummary) {
  const timestamp = task.started_at ?? task.finished_at ?? ''
  if (!timestamp) {
    return 0
  }

  const parsed = Date.parse(timestamp)
  return Number.isNaN(parsed) ? 0 : parsed
}

function compareTasksByTimeDesc(left: TaskSummary, right: TaskSummary) {
  const timeDiff = taskTimeMs(right) - taskTimeMs(left)
  if (timeDiff !== 0) {
    return timeDiff
  }

  return right.task_id.localeCompare(left.task_id)
}

export const useTasksStore = defineStore('tasks', () => {
  const items = ref<TaskSummary[]>([])
  const currentTask = ref<TaskSummary | null>(null)
  const loading = ref(false)
  const detailLoading = ref(false)
  const cancelPending = ref(false)
  const error = ref<string | null>(null)
  const taskVersions = new Map<string, number>()
  let taskClock = 0

  const sortedItems = computed(() => [...items.value].sort(compareTasksByTimeDesc))

  function getTaskVersion(taskId: string) {
    return taskVersions.get(taskId) ?? 0
  }

  function markTaskVersion(taskId: string) {
    taskClock += 1
    taskVersions.set(taskId, taskClock)
  }

  function findExistingTask(taskId: string) {
    if (currentTask.value?.task_id === taskId) {
      return currentTask.value
    }

    return items.value.find((item) => item.task_id === taskId) ?? null
  }

  function isInProgressTask(task: TaskSummary) {
    return IN_PROGRESS_TASK_STATUSES.has(task.status)
  }

  function findInProgressTaskByTypeFromItems(taskType: string) {
    if (currentTask.value?.task_type === taskType && isInProgressTask(currentTask.value)) {
      return currentTask.value
    }

    return items.value.find((item) => item.task_type === taskType && isInProgressTask(item)) ?? null
  }

  function mergeTask(existing: TaskSummary | null, incoming: TaskSummary) {
    if (!existing) {
      return incoming
    }

    return {
      ...existing,
      ...incoming,
      progress: incoming.progress ?? existing.progress,
      started_at: incoming.started_at ?? existing.started_at,
      finished_at: incoming.finished_at ?? existing.finished_at,
      result: incoming.result ?? existing.result,
      error: incoming.error ?? existing.error,
    }
  }

  function writeTask(task: TaskSummary, options: { requestVersion?: number; makeCurrent?: boolean } = {}) {
    const existing = findExistingTask(task.task_id)
    const hasNewerSnapshot = options.requestVersion !== undefined && getTaskVersion(task.task_id) > options.requestVersion
    const nextTask = hasNewerSnapshot && existing ? existing : mergeTask(existing, task)

    if (!hasNewerSnapshot || !existing) {
      markTaskVersion(nextTask.task_id)
    }

    const index = items.value.findIndex((item) => item.task_id === nextTask.task_id)
    if (index === -1) {
      items.value = [nextTask, ...items.value]
    } else {
      items.value = items.value.map((item, itemIndex) => (itemIndex === index ? nextTask : item))
    }

    if (options.makeCurrent || currentTask.value?.task_id === nextTask.task_id) {
      currentTask.value = nextTask
    }

    return nextTask
  }

  async function fetchList(filters: { status?: string; taskType?: string; limit?: number } = {}) {
    loading.value = true
    error.value = null
    const requestVersion = taskClock
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
      items.value = response.items.map((task) => {
        const existing = findExistingTask(task.task_id)
        if (getTaskVersion(task.task_id) > requestVersion && existing) {
          return existing
        }

        const nextTask = mergeTask(existing, task)
        markTaskVersion(nextTask.task_id)
        if (currentTask.value?.task_id === nextTask.task_id) {
          currentTask.value = nextTask
        }
        return nextTask
      })
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      loading.value = false
    }
  }

  async function fetchTask(taskId: string, options: { makeCurrent?: boolean } = {}) {
    detailLoading.value = true
    const requestVersion = getTaskVersion(taskId)
    try {
      const response = await apiRequest<TaskDetailResponse>(`/api/tasks/${taskId}`)
      return writeTask(response.task, { requestVersion, makeCurrent: options.makeCurrent ?? true })
    } finally {
      detailLoading.value = false
    }
  }

  async function fetchDetail(taskId: string) {
    return fetchTask(taskId, { makeCurrent: true })
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
    return writeTask(task)
  }

  function clearCurrentTask() {
    currentTask.value = null
  }

  async function findInProgressTaskByType(taskType: string, options: { refresh?: boolean } = {}) {
    const existing = findInProgressTaskByTypeFromItems(taskType)
    if (existing || options.refresh === false) {
      return existing
    }

    const params = new URLSearchParams()
    params.set('task_type', taskType)
    const response = await apiRequest<TaskListResponse>(`/api/tasks?${params}`)
    for (const task of response.items) {
      writeTask(task)
    }

    return response.items.find((task) => isInProgressTask(task)) ?? null
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
    fetchTask,
    fetchDetail,
    fetchList,
    findInProgressTaskByType,
    upsert,
  }
})
