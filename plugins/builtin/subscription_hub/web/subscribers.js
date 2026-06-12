import { trim, unique } from './services.js'

export const numericPattern = /^[0-9]+$/

export function normalizeSubscriber(value) {
  const id = trim(value && value.id)
  if (!numericPattern.test(id)) {
    return null
  }
  return {
    id,
    nickname: trim(value.nickname),
    group_nickname: trim(value.group_nickname),
    title: trim(value.title),
    role: trim(value.role),
    role_label: trim(value.role_label),
    avatar_url: trim(value.avatar_url),
  }
}

export function collectSubscriberAvatars(settings) {
  const avatars = new Map()
  for (const subscription of settings.subscriptions || []) {
    for (const subscriber of subscription.subscribers || []) {
      if (subscriber.id && subscriber.avatar_url) {
        avatars.set(subscriber.id, subscriber.avatar_url)
      }
    }
  }
  return avatars
}

export function subscriberAvatarURL(avatars, userId) {
  const id = trim(userId)
  return (avatars && avatars.get(id)) || `https://q1.qlogo.cn/g?b=qq&nk=${encodeURIComponent(id)}&s=640`
}

export function identityKey(targetType, targetId, userId) {
  return `${targetType}:${targetId}:${userId}`
}

export function buildIdentityRequests(rows) {
  const items = []
  for (const row of rows || []) {
    for (const target of row.targets || []) {
      for (const userId of row.subscriber_ids || []) {
        items.push({
          target_type: target.target_type,
          target_id: target.target_id,
          user_id: userId,
        })
      }
    }
  }
  const seen = new Set()
  return items.filter((item) => {
    const key = identityKey(item.target_type, item.target_id, item.user_id)
    if (seen.has(key)) {
      return false
    }
    seen.add(key)
    return true
  })
}

export function mergeSubscriberIds(existing, incoming) {
  return unique([...(existing || []), ...(incoming || [])])
}
