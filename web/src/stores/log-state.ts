import { ref } from 'vue'

import { getDisplayErrorMessage } from '@/lib/error-text'
import { apiRequest } from '@/lib/http'
import type { LogListResponse, LogProtocol, LogSummary } from '@/types/api'

export interface LogFilters {
  level?: string
  source?: string
  protocol?: LogProtocol
  pluginId?: string
  requestId?: string
  limit?: number
}

export function createLogsState(initialFilters: LogFilters = {}) {
  const items = ref<LogSummary[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const filters = ref<LogFilters>({
    limit: 50,
    ...initialFilters,
  })

  async function fetchList() {
    loading.value = true
    error.value = null
    try {
      const response = await apiRequest<LogListResponse>(buildLogListPath(filters.value))
      items.value = mergeLogs(items.value, response.items, filters.value.limit ?? 50)
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
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
}

export function buildLogListPath(filters: LogFilters) {
  const params = new URLSearchParams()
  if (filters.level) {
    params.set('level', filters.level)
  }
  if (filters.source) {
    params.set('source', filters.source)
  }
  if (filters.protocol) {
    params.set('protocol', filters.protocol)
  }
  if (filters.pluginId) {
    params.set('plugin_id', filters.pluginId)
  }
  if (filters.requestId) {
    params.set('request_id', filters.requestId)
  }
  params.set('limit', String(filters.limit ?? 50))

  return `/api/logs?${params.toString()}`
}

function mergeLogs(primary: LogSummary[], secondary: LogSummary[], limit: number) {
  const merged: LogSummary[] = []
  const seen = new Set<string>()

  for (const log of [...primary, ...secondary]) {
    const key = log.log_id || [
      log.timestamp,
      log.level,
      log.protocol ?? '',
      log.source,
      log.plugin_id ?? '',
      log.request_id ?? '',
      log.message,
    ].join('|')
    if (seen.has(key)) {
      continue
    }

    seen.add(key)
    merged.push(log)
  }

  return merged.slice(0, limit)
}
