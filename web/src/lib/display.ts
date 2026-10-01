import { resolveStatusTone } from '@/lib/status-tone'
import type {
  ConnectionStatus,
  LogLevel,
  LogProtocol,
  PluginRole,
  PluginState,
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

// Manifests may subscribe to names the host never emits, so an unknown event type has no label.
export function getEventTypeLabel(eventType: string) {
  const key = `display.eventTypes.${eventType}`
  return i18n.global.te(key) ? t(key) : null
}

export function getPluginSourceTypeLabel(type?: string | null) {
  switch (type) {
    case 'local_zip': return t('plugins.localZip')
    case 'local_directory': return t('plugins.localDirectory')
    case 'remote_url': return t('plugins.remoteUrl')
    case 'catalog': return t('plugins.catalogSource')
    case 'development': return t('plugins.developmentSource')
    default: return type || ''
  }
}

// How a plugin was installed, followed by the path or address it came from when that is recorded. A store install
// records the internal ID of its plugin source, which names nothing an operator recognizes, so it is left out.
export function getPluginInstallMethodLabel(source?: { package_source_type?: string | null; package_source_ref?: string | null } | null) {
  const method = getPluginSourceTypeLabel(source?.package_source_type)
  if (!method) return t('plugins.overview.origin.methodUnrecorded')
  const reference = source?.package_source_type === 'catalog' ? '' : source?.package_source_ref?.trim()
  return [method, reference].filter(Boolean).join(' · ')
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
