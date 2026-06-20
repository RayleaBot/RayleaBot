import { normalizeServices, servicesKey, trim, unique } from './services.js'
import { normalizePlatform, safeSubjectId } from './platforms.js'
import { normalizeSubscriber } from './subscribers.js'
import { targetKey } from './targets.js'

export function normalizeSubscription(value) {
  if (!value || typeof value !== 'object') {
    return null
  }
  const platform = normalizePlatform(value.platform)
  const uid = safeSubjectId(value.uid, platform)
  const targetType = trim(value.target_type)
  const targetId = trim(value.target_id)
  if (!/^[0-9]+$/.test(uid) || !['group', 'private'].includes(targetType) || !/^[0-9]+$/.test(targetId)) {
    if (platform === 'bilibili') {
      return null
    }
  }
  if (!uid || !['group', 'private'].includes(targetType) || !/^[0-9]+$/.test(targetId)) {
    return null
  }
  return {
    id: trim(value.id),
    platform,
    uid,
    name: trim(value.name) || uid,
    avatar_url: trim(value.avatar_url),
    target_type: targetType,
    target_id: targetId,
    target_name: trim(value.target_name),
    services: normalizeServices(value.services, platform),
    subscribers: Array.isArray(value.subscribers)
      ? value.subscribers.map(normalizeSubscriber).filter(Boolean)
      : [],
    enabled: value.enabled !== false,
  }
}

export function normalizeSettings(value) {
  const record = value && typeof value === 'object' ? value : {}
  return {
    enabled: record.enabled !== false,
    subscriptions: Array.isArray(record.subscriptions)
      ? record.subscriptions.map(normalizeSubscription).filter(Boolean)
      : [],
  }
}

export function createBlankRow(rowId) {
  return {
    row_id: rowId,
    platform: 'bilibili',
    uid: '',
    name: '',
    avatar_url: '',
    query: '',
    resolved: false,
    resolve_state: 'idle',
    resolve_message: '',
    candidates: [],
    enabled: true,
    services: ['all'],
    service_mode: 'common',
    target_mode: 'group',
    targets: [],
    subscriber_ids: [],
    edit_mode: true,
    _editSnapshot: null,
  }
}

export function buildRowsFromSettings(settings) {
  const grouped = new Map()
  for (const subscription of settings.subscriptions || []) {
    const groupKey = `${subscription.platform}:${subscription.uid}`
    let row = grouped.get(groupKey)
    if (!row) {
      row = {
        row_id: `${subscription.platform}-${subscription.uid}`,
        platform: subscription.platform,
        uid: subscription.uid,
        name: subscription.name || subscription.uid,
        avatar_url: subscription.avatar_url || '',
        query: subscription.name || subscription.uid,
        resolved: true,
        resolve_state: 'resolved',
        resolve_message: '',
        candidates: [],
        enabled: false,
        services: normalizeServices(subscription.services, subscription.platform),
        service_mode: 'common',
        target_mode: subscription.target_type || 'group',
        targets: [],
        subscriber_ids: [],
        edit_mode: false,
        _editSnapshot: null,
      }
      grouped.set(groupKey, row)
    }
    row.enabled = row.enabled || subscription.enabled !== false
    row.avatar_url = row.avatar_url || subscription.avatar_url || ''
    row.name = row.name || subscription.name || subscription.uid
    row.query = row.name

    const key = targetKey(subscription.target_type, subscription.target_id)
    row.targets.push({
      key,
      subscription_id: subscription.id,
      target_type: subscription.target_type,
      target_id: subscription.target_id,
      target_name: subscription.target_name || '',
      services: normalizeServices(subscription.services, subscription.platform),
    })
    for (const subscriber of subscription.subscribers || []) {
      if (subscriber.id) {
        row.subscriber_ids.push(subscriber.id)
      }
    }
  }

  const rows = [...grouped.values()]
  for (const row of rows) {
    row.subscriber_ids = unique(row.subscriber_ids)
    const serviceKeys = unique(row.targets.map((target) => servicesKey(target.services, row.platform)))
    if (serviceKeys.length > 1) {
      row.service_mode = 'mixed'
    } else if (serviceKeys.length === 1) {
      row.services = row.targets[0].services
    }
  }
  return rows
}

export function cloneRow(row) {
  return JSON.parse(JSON.stringify(row))
}
