import { t } from '@/i18n'
import { cloneConfig } from '@/lib/config-form'
import { findAdapterInstance, oneBotTransports, type AdapterInstanceDocument } from '@/lib/adapters'
import type { ConfigDocument } from '@/types/api'

export function validateAdapterDraft(draft: AdapterInstanceDocument) {
  const errors: Record<string, string> = {}
  if (!/^[a-z0-9][a-z0-9-]{0,63}$/.test(draft.id)) {
    errors.id = t('protocols.connectionDialog.validation.instanceId')
  }
  if (draft.type === 'qqofficial' && draft.qqofficial) {
    const settings = draft.qqofficial
    if ((draft.enabled && !settings.app_id) || (settings.app_id && !/^\d+$/.test(settings.app_id))) {
      errors.app_id = t('protocols.connectionDialog.validation.appId')
    }
    if (draft.enabled && !settings.app_secret.trim()) {
      errors.app_secret = t('protocols.connectionDialog.validation.appSecret')
    }
  }
  if (draft.type === 'onebot11' && draft.onebot11) {
    const settings = draft.onebot11
    if (draft.enabled && !oneBotTransports.some((key) => settings[key].enabled)) {
      errors.transports = t('protocols.connectionDialog.validation.transports')
    }
    for (const key of oneBotTransports) {
      const entry = settings[key]
      const url = String(entry.url ?? '').trim()
      const websocket = key.endsWith('_ws')
      const protocols = websocket ? ['ws:', 'wss:'] : ['http:', 'https:']
      let valid = !url && !(draft.enabled && entry.enabled)
      if (url) {
        try {
          const parsed = new URL(url)
          valid = protocols.includes(parsed.protocol) && Boolean(parsed.hostname)
        } catch { valid = false }
      }
      if (!valid) errors[`${key}.url`] = t(websocket ? 'protocols.connectionDialog.validation.wsUrl' : 'protocols.connectionDialog.validation.httpUrl')
    }
  }
  return errors
}

// Only replace the edited instance in a fresh document. A changed or removed
// instance needs review; unrelated changes made while the dialog was open survive.
export function mergeAdapterDraft(
  latest: ConfigDocument,
  baseline: AdapterInstanceDocument | null,
  draft: AdapterInstanceDocument,
) {
  const current = findAdapterInstance(latest, baseline?.id ?? draft.id)
  if (baseline && JSON.stringify(current) !== JSON.stringify(baseline)) {
    throw new Error(t('protocols.connectionDialog.connectionChanged'))
  }
  if (!baseline && current) {
    throw new Error(t('protocols.connectionDialog.instanceTaken'))
  }
  const result = cloneConfig(latest)
  const value = JSON.parse(JSON.stringify(draft)) as AdapterInstanceDocument
  result.adapters = baseline
    ? result.adapters.map((entry) => entry.id === baseline.id ? value : entry)
    : [...result.adapters, value]
  return result
}
