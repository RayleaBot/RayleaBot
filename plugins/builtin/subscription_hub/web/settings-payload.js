import { normalizeServices, trim } from './services.js'
import { normalizePlatform } from './platforms.js'

export function buildSettingsPayload(settings, rows, targetsByKey) {
  const targets = targetsByKey || new Map()
  const subscriptions = []
  for (const row of rows || []) {
    const platform = normalizePlatform(row.platform)
    for (const target of row.targets || []) {
      const live = targets.get(target.key)
      const targetName = live ? live.label : target.target_name
      subscriptions.push({
        id: target.subscription_id || `${platform}-${row.uid}-${target.target_type}-${target.target_id}`,
        platform,
        uid: row.uid,
        name: row.name,
        avatar_url: row.avatar_url,
        target_type: target.target_type,
        target_id: target.target_id,
        target_name: targetName,
        services: normalizeServices(row.service_mode === 'mixed' ? target.services : row.services, platform),
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
