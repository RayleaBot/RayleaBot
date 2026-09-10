import { getAdapterStateLabel, type StatusType } from '@/lib/display'
import type { SystemStatusResponse } from '@/types/api'

export function describeAdapterStates(adapters: SystemStatusResponse['adapters'] = []) {
  const enabled = adapters.filter(adapter => adapter.enabled)
  const connected = enabled.filter(adapter => adapter.state === 'connected')
  const status: StatusType = enabled.length === 0 ? 'muted'
    : enabled.some(adapter => adapter.state === 'auth_failed') ? 'danger'
      : connected.length === enabled.length ? 'success' : 'warning'
  return {
    status,
    value: adapters.length === 0 ? '未添加连接' : enabled.length === 0 ? '未启用连接' : `${connected.length} / ${enabled.length} 已连接`,
    detail: adapters.length === 0 ? '添加聊天连接后可接收和发送消息' : adapters.map(adapter => `${adapter.id}：${adapter.enabled ? getAdapterStateLabel(adapter.state) : '已停用'}`).join('；'),
  }
}
