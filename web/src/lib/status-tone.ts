export type StatusTone = 'neutral' | 'info' | 'success' | 'warning' | 'attention' | 'danger'

const statusToneMap: Record<string, StatusTone> = {
  stopped: 'neutral',
  ok: 'success',
  connected: 'success',
  listening: 'info',
  authenticated: 'success',
  disconnected: 'danger',
  auth_failed: 'danger',
  shutting_down: 'attention',
  disabled: 'neutral',
  inactive: 'neutral',
  starting: 'info',
  stopping: 'info',
  connecting: 'info',
  reconnecting: 'warning',
  running: 'success',
  ready: 'success',
  healthy: 'success',
  enabled: 'warning',
  degraded: 'warning',
  warning: 'warning',
  retrying: 'warning',
  setup_required: 'attention',
  unverified: 'attention',
  pending_confirmation: 'attention',
  failed: 'danger',
  error: 'danger',
  invalid: 'danger',
  blocked: 'danger',
}

export function resolveStatusTone(status?: string | null): StatusTone {
  if (!status) {
    return 'neutral'
  }

  return statusToneMap[status.trim().toLowerCase()] ?? 'neutral'
}
