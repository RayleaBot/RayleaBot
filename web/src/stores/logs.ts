import { ref } from 'vue'
import { defineStore } from 'pinia'

import { apiRequest } from '@/lib/http'
import type { LogListResponse, LogSummary } from '@/types/api'

export interface LogFilters {
  level?: string
  source?: string
  pluginId?: string
  requestId?: string
  limit?: number
}

export const useLogsStore = defineStore('logs', () => {
  const items = ref<LogSummary[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const filters = ref<LogFilters>({
    limit: 50,
  })

  async function fetchList() {
    loading.value = true
    error.value = null
    try {
      const params = new URLSearchParams()
      if (filters.value.level) {
        params.set('level', filters.value.level)
      }
      if (filters.value.source) {
        params.set('source', filters.value.source)
      }
      if (filters.value.pluginId) {
        params.set('plugin_id', filters.value.pluginId)
      }
      if (filters.value.requestId) {
        params.set('request_id', filters.value.requestId)
      }
      params.set('limit', String(filters.value.limit ?? 50))

      const response = await apiRequest<LogListResponse>(`/api/logs?${params.toString()}`)
      items.value = mergeLogs(items.value, response.items, filters.value.limit ?? 50)
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'log list failed'
      throw err
    } finally {
      loading.value = false
    }
  }

  function append(log: LogSummary) {
    items.value = mergeLogs([log], items.value, filters.value.limit ?? 50)
  }

  return {
    error,
    filters,
    items,
    loading,
    append,
    fetchList,
  }
})

function mergeLogs(primary: LogSummary[], secondary: LogSummary[], limit: number) {
  const merged: LogSummary[] = []
  const seen = new Set<string>()

  for (const log of [...primary, ...secondary]) {
    const key = [log.timestamp, log.level, log.source, log.plugin_id ?? '', log.request_id ?? '', log.message].join('|')
    if (seen.has(key)) {
      continue
    }

    seen.add(key)
    merged.push(log)
  }

  return merged.slice(0, limit)
}
