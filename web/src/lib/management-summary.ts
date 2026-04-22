import type { EventsPayload, ReadinessIssue } from '@/types/api'

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
    return readableSummary(payload.summary, '管理事件已更新')
  }

  return null
}

export function formatProtocolIssueSummary(issue: ReadinessIssue | undefined | null): string | null {
  if (!issue) {
    return null
  }

  return protocolSummary(issue.code, issue.summary, issue.remediation)
}

export function formatProtocolEventSummary(payload: EventsPayload | undefined | null): string | null {
  if (!payload) {
    return null
  }

  if ('connection_status' in payload) {
    return connectionSummary(payload.connection_status)
  }

  if ('event_type' in payload) {
    return protocolSummary(payload.event_type, payload.summary)
  }

  return formatDashboardEventSummary(payload)
}

function connectionSummary(status: string) {
  switch (status) {
    case 'connected':
    case 'authenticated':
      return '协议连接正常'
    case 'connecting':
      return '协议正在连接'
    case 'reconnecting':
      return '协议正在重连'
    case 'auth_failed':
      return '协议鉴权失败，请检查访问令牌'
    case 'disconnected':
      return '协议连接已断开'
    default:
      return '协议状态已更新'
  }
}

function pluginSummary(payload: Extract<EventsPayload, { plugin_id: string }>) {
  const pluginID = payload.plugin_id

  if (payload.registration_state === 'removed') {
    return `插件 ${pluginID} 已移除`
  }
  if (payload.desired_state === 'disabled') {
    return `插件 ${pluginID} 已停用`
  }

  switch (payload.runtime_state) {
    case 'running':
      return `插件 ${pluginID} 运行中`
    case 'starting':
      return `插件 ${pluginID} 启动中`
    case 'stopping':
      return `插件 ${pluginID} 停止中`
    case 'crashed':
      return `插件 ${pluginID} 运行异常`
    case 'backoff':
      return `插件 ${pluginID} 正在等待重试`
    case 'dead_letter':
      return `插件 ${pluginID} 已进入异常挂起`
    case 'stopped':
      return `插件 ${pluginID} 已停止`
    default:
      return `插件 ${pluginID} 状态已更新`
  }
}

function serviceSummary(status: string, rawReason: string | undefined) {
  const reason = readableSummary(rawReason, '')

  switch (status) {
    case 'running':
      return reason || '服务运行中'
    case 'starting':
      return '服务启动中'
    case 'stopping':
      return '服务正在停止'
    case 'stopped':
      return '服务已停止'
    case 'degraded':
      return reason || '服务运行条件受限'
    case 'failed':
      return reason || '服务运行异常'
    case 'setup_required':
      return '服务等待初始化'
    default:
      return reason || '服务状态已更新'
  }
}

function protocolSummary(code: string, ...texts: Array<string | undefined>) {
  const raw = normalizeWhitespace([code, ...texts].filter(Boolean).join(' ')).toLowerCase()

  if (
    raw.includes('auth_failed') ||
    raw.includes('authentication failed') ||
    raw.includes('access_token')
  ) {
    return '协议鉴权失败，请检查访问令牌'
  }
  if (
    raw.includes('connection_failed') ||
    raw.includes('connect failed') ||
    raw.includes('dial tcp')
  ) {
    return '协议连接失败，请检查地址与网络'
  }
  if (
    raw.includes('connection_lost') ||
    raw.includes('connection lost') ||
    raw.includes('socket close') ||
    raw.includes('websocket close')
  ) {
    return '协议连接已断开，正在重连'
  }
  if (
    raw.includes('heartbeat timeout') ||
    raw.includes('heartbeat_timeout')
  ) {
    return '协议心跳超时，请检查连接稳定性'
  }
  if (
    raw.includes('partial_warning') ||
    raw.includes('warning')
  ) {
    return '协议有异常提示，请查看日志中心'
  }
  if (
    raw.includes('connected') ||
    raw.includes('ready') ||
    raw.includes('lifecycle.enable')
  ) {
    return '协议连接正常'
  }

  return readableSummary(texts[0], '协议状态已更新')
}

function readableSummary(text: string | undefined, fallback: string) {
  const normalized = normalizeWhitespace(text)
  if (!normalized) {
    return fallback
  }
  if (containsChinese(normalized)) {
    return normalized
  }

  const lowered = normalized.toLowerCase()
  if (lowered.includes('authentication failed')) {
    return '协议鉴权失败，请检查访问令牌'
  }
  if (lowered.includes('connection lost')) {
    return '协议连接已断开，正在重连'
  }
  if (lowered.includes('connection failed')) {
    return '协议连接失败，请检查地址与网络'
  }

  return fallback
}

function normalizeWhitespace(value: string | undefined) {
  return (value ?? '').replace(/\s+/g, ' ').trim()
}

function containsChinese(value: string) {
  return /[\u4e00-\u9fff]/.test(value)
}
