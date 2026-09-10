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
    return summaryOrFallback(payload.summary, '管理事件已更新')
  }

  return null
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

  switch (payload.state) {
    case 'running':
      return `插件 ${pluginID} 运行中`
    case 'starting':
      return `插件 ${pluginID} 启动中`
    case 'stopping':
      return `插件 ${pluginID} 停止中`
    case 'enabled':
      return `插件 ${pluginID} 已启用`
    case 'disabled':
      return `插件 ${pluginID} 已停用`
    case 'failed':
      return `插件 ${pluginID} 运行异常`
    case 'invalid':
      return `插件 ${pluginID} 清单异常`
    default:
      return `插件 ${pluginID} 状态已更新`
  }
}

function serviceSummary(status: string, rawReason: string | undefined) {
  const reason = summaryOrFallback(rawReason, '')

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

function summaryOrFallback(text: string | undefined, fallback: string) {
  return text?.replace(/\s+/g, ' ').trim() || fallback
}
