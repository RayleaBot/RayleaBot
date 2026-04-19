import type {
  LocationQuery,
  LocationQueryRaw,
  LocationQueryValue,
  RouteLocationRaw,
} from 'vue-router'

import { t } from '@/i18n'
import type {
  EventsPayload,
  LogSummary,
  OneBot11ProtocolSnapshotResponse,
  TaskSummary,
} from '@/types/api'
import type { LogScope, LogFilters } from '@/stores/log-state'

export interface ManagementContextAction {
  key: string
  label: string
  to: RouteLocationRaw
}

export type PluginDetailPanel = 'overview' | 'management-ui'

interface LogsLocationOptions {
  filters?: Partial<LogFilters>
  history?: boolean
  logId?: string | null
  startAt?: string | null
  endAt?: string | null
}

interface ParsedLogWorkspaceState {
  filters: LogFilters
  logId: string | null
  startAt: string
  endAt: string
}

type ContextFieldSource = Record<string, unknown> | null | undefined

function normalizeQueryValue(value: LocationQueryValue | LocationQueryValue[] | undefined) {
  if (Array.isArray(value)) {
    return value
      .map((item) => item?.trim() ?? '')
      .filter((item) => item.length > 0)
  }

  const nextValue = value?.trim() ?? ''
  return nextValue ? [nextValue] : []
}

function normalizeSingleQueryValue(value: LocationQueryValue | LocationQueryValue[] | undefined) {
  return normalizeQueryValue(value)[0] ?? ''
}

function normalizeString(value: string | null | undefined) {
  const nextValue = value?.trim() ?? ''
  return nextValue || undefined
}

function normalizePluginIds(pluginIds: string[] | string | null | undefined) {
  const rawValues = Array.isArray(pluginIds) ? pluginIds : pluginIds ? [pluginIds] : []
  return Array.from(new Set(
    rawValues
      .map((item) => item.trim())
      .filter((item) => item.length > 0),
  )).sort((left, right) => left.localeCompare(right, 'zh-CN'))
}

function createLocationQuery(entries: Array<[string, string | string[] | undefined]>) {
  const query: LocationQueryRaw = {}

  for (const [key, rawValue] of entries) {
    if (Array.isArray(rawValue)) {
      if (rawValue.length > 0) {
        query[key] = rawValue
      }
      continue
    }

    if (rawValue) {
      query[key] = rawValue
    }
  }

  return query
}

function serializeQueryForCompare(query: LocationQuery | LocationQueryRaw) {
  const params = new URLSearchParams()

  for (const key of Object.keys(query).sort((left, right) => left.localeCompare(right, 'zh-CN'))) {
    const values = normalizeQueryValue(query[key])
    if (values.length === 0) {
      continue
    }

    for (const value of values) {
      params.append(key, value)
    }
  }

  return params.toString()
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function readContextString(source: ContextFieldSource, key: string) {
  if (!isPlainObject(source)) {
    return undefined
  }

  const value = source[key]
  return typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined
}

function pushAction(actions: ManagementContextAction[], action: ManagementContextAction | null) {
  if (!action) {
    return
  }

  if (actions.some((item) => item.key === action.key)) {
    return
  }

  actions.push(action)
}

function buildRequestLogsAction(requestId: string, scope: LogScope) {
  const history = scope === 'history'
  return {
    key: history ? `request-history:${requestId}` : `request-current:${requestId}`,
    label: history ? t('logs.actions.openRequestHistory') : t('logs.actions.openRequestLogs'),
    to: buildLogsLocation({
      history,
      filters: {
        requestId,
      },
    }),
  } satisfies ManagementContextAction
}

export function areLocationQueriesEqual(left: LocationQuery | LocationQueryRaw, right: LocationQuery | LocationQueryRaw) {
  return serializeQueryForCompare(left) === serializeQueryForCompare(right)
}

export function readCommandsPluginIds(query: LocationQuery) {
  return normalizePluginIds(normalizeQueryValue(query.plugin_id))
}

export function readPluginDetailPanel(query: LocationQuery) {
  return normalizeSingleQueryValue(query.panel) === 'management-ui'
    ? 'management-ui'
    : 'overview'
}

export function buildCommandsLocation(pluginIds?: string[] | string | null) {
  const normalizedPluginIds = normalizePluginIds(pluginIds)

  return {
    name: 'commands',
    query: createLocationQuery([
      ['plugin_id', normalizedPluginIds],
    ]),
  } satisfies RouteLocationRaw
}

export function readLogWorkspaceState(query: LocationQuery, options: { history?: boolean } = {}) {
  const filters: LogFilters = {}
  const level = normalizeSingleQueryValue(query.level)
  const source = normalizeSingleQueryValue(query.source)
  const protocol = normalizeSingleQueryValue(query.protocol)
  const pluginId = normalizeSingleQueryValue(query.plugin_id)
  const requestId = normalizeSingleQueryValue(query.request_id)
  const logId = normalizeSingleQueryValue(query.log_id)
  const startAt = options.history ? normalizeSingleQueryValue(query.start_at) : ''
  const endAt = options.history ? normalizeSingleQueryValue(query.end_at) : ''

  if (level) filters.level = level
  if (source) filters.source = source
  if (protocol) filters.protocol = protocol as LogFilters['protocol']
  if (pluginId) filters.pluginId = pluginId
  if (requestId) filters.requestId = requestId

  return {
    filters,
    logId: logId || null,
    startAt,
    endAt,
  } satisfies ParsedLogWorkspaceState
}

export function buildLogsLocation(options: LogsLocationOptions = {}) {
  const filters = options.filters ?? {}
  const history = Boolean(options.history)

  return {
    name: history ? 'logs-history' : 'logs',
    query: createLocationQuery([
      ['level', normalizeString(filters.level)],
      ['source', normalizeString(filters.source)],
      ['protocol', normalizeString(filters.protocol)],
      ['plugin_id', normalizeString(filters.pluginId)],
      ['request_id', normalizeString(filters.requestId)],
      ['log_id', normalizeString(options.logId ?? undefined)],
      ['start_at', history ? normalizeString(options.startAt ?? undefined) : undefined],
      ['end_at', history ? normalizeString(options.endAt ?? undefined) : undefined],
    ]),
  } satisfies RouteLocationRaw
}

export function buildPluginDetailLocation(pluginId: string, options: { panel?: PluginDetailPanel | null } = {}) {
  const panel = options.panel ?? undefined

  return {
    name: 'plugin-detail',
    params: { id: pluginId },
    query: createLocationQuery([
      ['panel', panel],
    ]),
  } satisfies RouteLocationRaw
}

export function buildTaskLocation(taskId?: string | null) {
  return {
    name: 'tasks',
    query: createLocationQuery([
      ['task_id', normalizeString(taskId ?? undefined)],
    ]),
  } satisfies RouteLocationRaw
}

export function buildProtocolsLocation(options: { protocolLogs?: boolean } = {}) {
  if (!options.protocolLogs) {
    return { name: 'protocols' } satisfies RouteLocationRaw
  }

  return buildLogsLocation({
    filters: {
      protocol: 'onebot11',
    },
  })
}

export function buildProtocolCompatibilityLocation() {
  return { name: 'protocols-compatibility' } satisfies RouteLocationRaw
}

export function buildRenderTemplateLocation(templateId?: string | null) {
  if (!templateId) {
    return { name: 'render-templates' } satisfies RouteLocationRaw
  }

  return {
    name: 'render-templates',
    params: { templateId },
  } satisfies RouteLocationRaw
}

export function buildPluginWorkbenchActions(pluginId: string) {
  return [
    {
      key: `plugin-commands:${pluginId}`,
      label: t('plugins.actions.openPluginCommands'),
      to: buildCommandsLocation([pluginId]),
    },
    {
      key: `plugin-logs:${pluginId}`,
      label: t('plugins.actions.openPluginLogs'),
      to: buildLogsLocation({
        history: true,
        filters: {
          pluginId,
        },
      }),
    },
  ] satisfies ManagementContextAction[]
}

export function buildProtocolWorkbenchActions(snapshot?: OneBot11ProtocolSnapshotResponse | null) {
  if (!snapshot) {
    return [] as ManagementContextAction[]
  }

  return [
    {
      key: 'protocol-compatibility',
      label: t('protocols.actions.openCompatibility'),
      to: buildProtocolCompatibilityLocation(),
    },
    {
      key: 'protocol-logs',
      label: t('protocols.actions.openFilteredLogs'),
      to: buildProtocolsLocation({ protocolLogs: true }),
    },
  ] satisfies ManagementContextAction[]
}

export function buildDashboardProtocolActions(snapshot?: OneBot11ProtocolSnapshotResponse | null) {
  if (!snapshot) {
    return [] as ManagementContextAction[]
  }

  return [
    {
      key: 'protocols',
      label: t('dashboard.actions.openProtocols'),
      to: buildProtocolsLocation(),
    },
    {
      key: 'protocol-logs',
      label: t('dashboard.actions.openProtocolLogs'),
      to: buildProtocolsLocation({ protocolLogs: true }),
    },
  ] satisfies ManagementContextAction[]
}

export function buildLogContextActions(summary: Pick<LogSummary, 'plugin_id' | 'protocol' | 'request_id'>, scope: LogScope) {
  const actions: ManagementContextAction[] = []

  if (summary.plugin_id) {
    pushAction(actions, {
      key: `plugin:${summary.plugin_id}`,
      label: t('logs.actions.openPlugin'),
      to: buildPluginDetailLocation(summary.plugin_id),
    })
  }

  if (summary.protocol === 'onebot11') {
    pushAction(actions, {
      key: 'protocol:onebot11',
      label: t('logs.actions.openProtocol'),
      to: buildProtocolsLocation(),
    })
  }

  if (summary.request_id) {
    pushAction(actions, buildRequestLogsAction(summary.request_id, scope))
  }

  return actions
}

export function buildTaskContextActions(task: TaskSummary) {
  const actions: ManagementContextAction[] = []
  const resultDetails = isPlainObject(task.result?.details) ? task.result?.details : null
  const errorDetails = isPlainObject(task.error?.details) ? task.error?.details : null
  const pluginId = readContextString(resultDetails, 'plugin_id') ?? readContextString(errorDetails, 'plugin_id')
  const requestId = readContextString(resultDetails, 'request_id') ?? readContextString(errorDetails, 'request_id')
  const protocol = readContextString(resultDetails, 'protocol') ?? readContextString(errorDetails, 'protocol')
  const templateId = readContextString(resultDetails, 'template')

  if (pluginId) {
    pushAction(actions, {
      key: `plugin:${pluginId}`,
      label: t('tasks.actions.openPlugin'),
      to: buildPluginDetailLocation(pluginId),
    })
  }

  if (protocol === 'onebot11') {
    pushAction(actions, {
      key: 'protocol:onebot11',
      label: t('tasks.actions.openProtocol'),
      to: buildProtocolsLocation(),
    })
  }

  if (requestId) {
    pushAction(actions, {
      key: `request-history:${requestId}`,
      label: t('tasks.actions.openRequestHistory'),
      to: buildLogsLocation({
        history: true,
        filters: {
          requestId,
        },
      }),
    })
  }

  if (templateId) {
    pushAction(actions, {
      key: `template:${templateId}`,
      label: t('tasks.actions.openTemplate'),
      to: buildRenderTemplateLocation(templateId),
    })
  }

  return actions
}

export function buildDashboardEventActions(payload: EventsPayload) {
  if ('plugin_id' in payload) {
    return [{
      key: `plugin:${payload.plugin_id}`,
      label: t('dashboard.actions.openPlugin'),
      to: buildPluginDetailLocation(payload.plugin_id),
    }] satisfies ManagementContextAction[]
  }

  if ('connection_status' in payload || 'protocol_snapshot' in payload) {
    return [{
      key: 'protocols',
      label: t('dashboard.actions.openProtocols'),
      to: buildProtocolsLocation(),
    }] satisfies ManagementContextAction[]
  }

  return [] as ManagementContextAction[]
}
