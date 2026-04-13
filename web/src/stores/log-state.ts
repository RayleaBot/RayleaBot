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
      items.value = dedupeLogs(response.items, filters.value.limit ?? 50)
    } catch (err) {
      error.value = getDisplayErrorMessage(err, 'errors.common.loadFailed')
      throw err
    } finally {
      loading.value = false
    }
  }

  function append(log: LogSummary) {
    if (!matchesLogFilters(log, filters.value)) {
      return
    }

    items.value = dedupeLogs([log, ...items.value], filters.value.limit ?? 50)
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
  return dedupeLogs([...primary, ...secondary], limit)
}

export function matchesLogFilters(log: LogSummary, filters: LogFilters) {
  if (filters.level && log.level !== filters.level) {
    return false
  }
  if (filters.source && log.source !== filters.source) {
    return false
  }
  if (filters.protocol && log.protocol !== filters.protocol) {
    return false
  }
  if (filters.pluginId && log.plugin_id !== filters.pluginId) {
    return false
  }
  if (filters.requestId && log.request_id !== filters.requestId) {
    return false
  }

  return true
}

function dedupeLogs(logs: LogSummary[], limit: number) {
  const merged: LogSummary[] = []
  const seen = new Set<string>()

  for (const log of logs) {
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
