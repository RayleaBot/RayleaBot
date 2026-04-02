import type {
  ConnectionStatus,
  LogLevel,
  PluginDesiredState,
  PluginRole,
  PluginRuntimeState,
  PluginRegistrationState,
  ReadinessStatusResponse,
  SystemStatusResponse,
  TaskStatus,
  TaskType,
} from '@/types/api'
import { i18n, t } from '@/i18n'

function fallback(raw?: string) {
  return raw || t('display.empty')
}

function translated(key: string, raw?: string) {
  return i18n.global.te(key) ? t(key) : fallback(raw)
}

export function getConnectionChannelLabel(channel: 'events' | 'tasks' | 'logs' | 'pluginConsole') {
  return t(`display.connectionChannels.${channel}`)
}

export function getConnectionStatusLabel(status?: ConnectionStatus) {
  return status ? t(`display.connectionStatuses.${status}`) : t('display.empty')
}

export function getTaskTypeLabel(taskType?: TaskType | string) {
  return taskType ? translated(`display.taskTypes.${taskType}`, taskType) : t('display.empty')
}

export function getTaskStatusLabel(status?: TaskStatus) {
  return status ? t(`display.taskStatuses.${status}`) : t('display.empty')
}

export function getPluginRegistrationStateLabel(status?: PluginRegistrationState) {
  return status ? translated(`display.pluginRegistrationStates.${status}`, status) : t('display.empty')
}

export function getPluginDesiredStateLabel(status?: PluginDesiredState) {
  return status ? translated(`display.pluginDesiredStates.${status}`, status) : t('display.empty')
}

export function getPluginRuntimeStateLabel(status?: PluginRuntimeState) {
  return status ? translated(`display.pluginRuntimeStates.${status}`, status) : t('display.empty')
}

export function getPluginDisplayStateLabel(status?: string) {
  return status ? translated(`display.pluginDisplayStates.${status}`, status) : t('display.empty')
}

export function getPluginRoleLabel(role?: PluginRole) {
  return role ? translated(`display.pluginRoles.${role}`, role) : t('display.empty')
}

export function getLogLevelLabel(level?: LogLevel) {
  return level ? translated(`display.logLevels.${level}`, level) : t('display.empty')
}

export function getSystemStatusLabel(status?: SystemStatusResponse['status']) {
  return status ? translated(`display.systemStatuses.${status}`, status) : t('display.empty')
}

export function getReadinessStatusLabel(status?: ReadinessStatusResponse['status']) {
  return status ? translated(`display.readinessStatuses.${status}`, status) : t('display.empty')
}

export function getAdapterStateLabel(status?: string) {
  return status ? translated(`display.adapterStates.${status}`, status) : t('display.empty')
}

export function getBooleanLabel(value?: boolean) {
  if (value === undefined) {
    return t('display.empty')
  }

  return value ? '是' : '否'
}

export type StatusType = 'success' | 'warning' | 'danger' | 'muted'

const STATUS_TYPE_MAP: Record<string, StatusType> = {
  ok: 'success',
  ready: 'success',
  running: 'success',
  connected: 'success',
  degraded: 'warning',
  connecting: 'warning',
  reconnecting: 'warning',
  failed: 'danger',
  setup_required: 'danger',
  shutting_down: 'danger',
  disconnected: 'danger',
  auth_failed: 'danger',
}

export function getStatusType(status?: string): StatusType {
  if (!status) {
    return 'muted'
  }
  return STATUS_TYPE_MAP[status] ?? 'muted'
}
