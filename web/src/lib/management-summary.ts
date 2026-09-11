import { t } from '@/i18n'
import type { EventsPayload } from '@/types/api'

export function formatDashboardEventSummary(payload: EventsPayload): string | null {
  if ('observability_scope' in payload && payload.observability_scope === 'bridge_runtime') {
    return null
  }

  if ('connection_status' in payload) {
    return connectionSummary(payload.connection_status)
  }

  if ('plugin_id' in payload) {
    return pluginSummary(payload)
  }

  if ('service_status' in payload) {
    return serviceSummary(payload.service_status, payload.reason ?? payload.summary)
  }

  if ('event_type' in payload) {
    return summaryOrFallback(payload.summary, t('dashboard.eventSummary.managementUpdated'))
  }

  return null
}

function connectionSummary(status: string) {
  switch (status) {
    case 'connected':
    case 'authenticated':
      return t('dashboard.eventSummary.connection.connected')
    case 'connecting':
      return t('dashboard.eventSummary.connection.connecting')
    case 'reconnecting':
      return t('dashboard.eventSummary.connection.reconnecting')
    case 'auth_failed':
      return t('dashboard.eventSummary.connection.authFailed')
    case 'disconnected':
      return t('dashboard.eventSummary.connection.disconnected')
    default:
      return t('dashboard.eventSummary.connection.updated')
  }
}

function pluginSummary(payload: Extract<EventsPayload, { plugin_id: string }>) {
  const pluginId = payload.plugin_id

  switch (payload.state) {
    case 'running':
      return t('dashboard.eventSummary.plugin.running', { pluginId })
    case 'starting':
      return t('dashboard.eventSummary.plugin.starting', { pluginId })
    case 'stopping':
      return t('dashboard.eventSummary.plugin.stopping', { pluginId })
    case 'enabled':
      return t('dashboard.eventSummary.plugin.enabled', { pluginId })
    case 'disabled':
      return t('dashboard.eventSummary.plugin.disabled', { pluginId })
    case 'failed':
      return t('dashboard.eventSummary.plugin.failed', { pluginId })
    case 'invalid':
      return t('dashboard.eventSummary.plugin.invalid', { pluginId })
    default:
      return t('dashboard.eventSummary.plugin.updated', { pluginId })
  }
}

function serviceSummary(status: string, rawReason: string | undefined) {
  const reason = summaryOrFallback(rawReason, '')

  switch (status) {
    case 'running':
      return reason || t('dashboard.eventSummary.service.running')
    case 'starting':
      return t('dashboard.eventSummary.service.starting')
    case 'stopping':
      return t('dashboard.eventSummary.service.stopping')
    case 'stopped':
      return t('dashboard.eventSummary.service.stopped')
    case 'degraded':
      return reason || t('dashboard.eventSummary.service.degraded')
    case 'failed':
      return reason || t('dashboard.eventSummary.service.failed')
    case 'setup_required':
      return t('dashboard.eventSummary.service.setupRequired')
    default:
      return reason || t('dashboard.eventSummary.service.updated')
  }
}

function summaryOrFallback(text: string | undefined, fallback: string) {
  return text?.replace(/\s+/g, ' ').trim() || fallback
}
