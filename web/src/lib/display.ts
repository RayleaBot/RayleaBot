import { resolveStatusTone } from '@/lib/status-tone'
import type {
  ConnectionStatus,
  LogLevel,
  LogProtocol,
  PluginRole,
  PluginState,
  RecoveryCompatibilitySummary,
  ReadinessStatusResponse,
  SystemStatusResponse,
} from '@/types/api'
import { i18n, t } from '@/i18n'

function fallback(raw?: string) {
  return raw || t('display.empty')
}

function translated(key: string, raw?: string) {
  return i18n.global.te(key) ? t(key) : fallback(raw)
}

export function getPluginPermissionLabel(permission: string) {
  const labels = i18n.global.tm('plugins.permissionLabels') as Record<string, unknown>
  const label = labels[permission]

  return typeof label === 'string' && label.trim() ? label : permission
}

export function getPluginPermissionRawTitle(permission: string) {
  return t('plugins.permissionRawTitle', { permission })
}

export function getConnectionChannelLabel(channel: 'events' | 'logs' | 'pluginConsole') {
  return t(`display.connectionChannels.${channel}`)
}

export function getConnectionStatusLabel(status?: ConnectionStatus) {
  return status ? t(`display.connectionStatuses.${status}`) : t('display.empty')
}

export function getPluginStateLabel(status?: PluginState | string) {
  return status ? translated(`display.pluginStates.${status}`, status) : t('display.empty')
}

export function getPluginRoleLabel(role?: PluginRole) {
  return role ? translated(`display.pluginRoles.${role}`, role) : t('display.empty')
}

export function formatPluginVersion(version?: string | null) {
  const value = version?.trim()
  return value ? `v${value.replace(/^v(?=\d)/, '')}` : t('display.empty')
}

export function getLogLevelLabel(level?: LogLevel) {
  return level ? translated(`display.logLevels.${level}`, level) : t('display.empty')
}

export function getLogProtocolLabel(protocol?: LogProtocol | string) {
  return protocol ? translated(`display.logProtocols.${protocol}`, protocol) : t('display.empty')
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

export function getRecoveryStatusLabel(status?: RecoveryCompatibilitySummary['status']) {
  return status ? translated(`display.recoveryStatuses.${status}`, status) : t('display.empty')
}

export type StatusType = 'success' | 'warning' | 'danger' | 'muted'

// Compact indicators have four tones; project the shared semantic palette.
export function getStatusType(status?: string): StatusType {
  const tone = resolveStatusTone(status)
  if (tone === 'neutral') return 'muted'
  if (tone === 'attention' || tone === 'info') return 'warning'
  return tone
}

export function getPluginTrustLabel(level?: string) {
  switch (level) {
    case 'official': return t('plugins.trustLabels.official')
    case 'development': return t('plugins.trustLabels.development')
    case 'third_party': return t('plugins.trustLabels.thirdParty')
    case 'unverified': return t('plugins.trustLabels.unverified')
    default: return t('plugins.trustLabels.unknown')
  }
}
