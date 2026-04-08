import type { EventsPayload, LogProtocol, ReadinessIssue } from '@/types/api'

export const ONEBOT11_PROTOCOL: LogProtocol = 'onebot11'
export const ONEBOT11_PROTOCOL_NAME = 'OneBot11'

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
