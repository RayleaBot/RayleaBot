import type { EventsPayload, LogProtocol, ReadinessIssue } from '@/types/api'

export const ONEBOT11_PROTOCOL: LogProtocol = 'onebot11'
export const ONEBOT11_PROTOCOL_NAME = 'OneBot11'
// Reverse-WebSocket ingress is addressed per adapter instance, so the callback
// URL an operator pastes into their OneBot client names the instance it
// belongs to. Renaming the instance changes this URL.
export function oneBot11ReverseWsPath(adapterId: string) {
  return `/api/adapters/${encodeURIComponent(adapterId)}/reverse-ws`
}

export function buildOneBot11ReverseWsUrl(baseUrl: string, adapterId: string) {
  const endpoint = new URL(oneBot11ReverseWsPath(adapterId), baseUrl)
  endpoint.protocol = ['https:', 'wss:'].includes(endpoint.protocol) ? 'wss:' : 'ws:'
  return endpoint.toString()
}

export function buildOneBot11WebhookUrl(baseUrl: string, adapterId: string) {
  const endpoint = new URL(`/api/adapters/${encodeURIComponent(adapterId)}/webhook`, baseUrl)
  endpoint.protocol = ['https:', 'wss:'].includes(endpoint.protocol) ? 'https:' : 'http:'
  return endpoint.toString()
}

export function isProtocolIssue(issue: ReadinessIssue | undefined | null) {
  if (!issue) {
    return false
  }

  const summary = `${issue.code} ${issue.summary} ${issue.remediation ?? ''}`.toLowerCase()
  return summary.includes('adapter') || summary.includes('onebot') || summary.includes('websocket')
}

export function isProtocolEvent(event: { summary: string; payload: EventsPayload } | undefined | null) {
  if (!event) {
    return false
  }

  if ('connection_status' in event.payload) {
    return true
  }

  const summary = event.summary.toLowerCase()
  return summary.includes('adapter') || summary.includes('onebot') || summary.includes('websocket') || summary.includes('连接')
}
