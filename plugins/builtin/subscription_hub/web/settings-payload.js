import { normalizeServices, trim } from './services.js'

export function buildSettingsPayload(settings, rows, targetsByKey) {
  const targets = targetsByKey || new Map()
  const subscriptions = []
  for (const row of rows || []) {
    for (const target of row.targets || []) {
      const live = targets.get(target.key)
      const targetName = live ? live.label : target.target_name
      subscriptions.push({
        id: target.subscription_id || `bilibili-${row.uid}-${target.target_type}-${target.target_id}`,
        platform: 'bilibili',
        uid: row.uid,
        name: row.name,
        avatar_url: row.avatar_url,
        target_type: target.target_type,
        target_id: target.target_id,
        target_name: targetName,
        services: normalizeServices(row.service_mode === 'mixed' ? target.services : row.services),
        subscribers: (row.subscriber_ids || []).map((userId) => ({ id: trim(userId) })),
        enabled: row.enabled,
      })
    }
  }
  return {
    enabled: settings.enabled !== false,
    subscriptions,
  }
}
