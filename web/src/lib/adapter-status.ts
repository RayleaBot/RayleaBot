import { t } from '@/i18n'
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
    value: adapters.length === 0 ? t('protocols.summary.none')
      : enabled.length === 0 ? t('protocols.summary.noneEnabled')
        : t('protocols.summary.connected', { connected: connected.length, enabled: enabled.length }),
    detail: adapters.length === 0 ? t('protocols.summary.noneDetail') : adapters.map(adapter => t('protocols.summary.entry', {
      id: adapter.id,
      state: adapter.enabled ? getAdapterStateLabel(adapter.state) : t('protocols.summary.stopped'),
    })).join('；'),
  }
}
