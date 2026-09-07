import { cloneConfig } from '@/lib/config-form'
import { findAdapterInstance, type AdapterInstanceDocument } from '@/lib/adapters'
import type { ConfigDocument } from '@/types/api'

export function validateAdapterDraft(draft: AdapterInstanceDocument) {
  const errors: Record<string, string> = {}
  if (!/^[a-z0-9][a-z0-9-]{0,63}$/.test(draft.id)) {
    errors.id = '使用 1–64 位小写字母、数字或连字符，以字母或数字开头。'
  }
  if (draft.type === 'qqofficial' && draft.qqofficial) {
    const settings = draft.qqofficial
    if ((draft.enabled && !settings.app_id) || (settings.app_id && !/^\d+$/.test(settings.app_id))) {
      errors.app_id = '请输入由数字组成的 AppID。'
    }
    if (draft.enabled && !settings.app_secret.trim()) {
      errors.app_secret = '请输入 AppSecret。'
    }
  }
  if (draft.type === 'onebot11' && draft.onebot11) {
    if (draft.enabled && !Object.values(draft.onebot11).some((entry) => entry.enabled)) {
      errors.transports = '请至少启用一种连接方式，或关闭此连接。'
    }
    for (const [key, entry] of Object.entries(draft.onebot11)) {
      const url = String(entry.url ?? '').trim()
      const protocols = key.endsWith('_ws') ? ['ws:', 'wss:'] : ['http:', 'https:']
      let valid = !url && !(draft.enabled && entry.enabled)
      if (url) {
        try {
          const parsed = new URL(url)
          valid = protocols.includes(parsed.protocol) && Boolean(parsed.hostname)
        } catch { valid = false }
      }
      if (!valid) errors[`${key}.url`] = `请输入有效的 ${protocols.map((item) => `${item}//`).join(' 或 ')} 地址。`
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
    throw new Error('此连接已在其他位置更改或删除。请保留所需内容，关闭弹窗后重新打开。')
  }
  if (!baseline && current) {
    throw new Error('连接标识已被使用，请在高级设置中更换标识。')
  }
  const result = cloneConfig(latest)
  const value = JSON.parse(JSON.stringify(draft)) as AdapterInstanceDocument
  result.adapters = baseline
    ? result.adapters.map((entry) => entry.id === baseline.id ? value : entry)
    : [...result.adapters, value]
  return result
}
