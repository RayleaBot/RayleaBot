import { trim } from './services.js'

export const TARGET_LABELS = {
  group: '群聊',
  private: '私聊',
}

const numericPattern = /^[0-9]+$/

export function targetKey(targetType, targetId) {
  return `${trim(targetType)}:${trim(targetId)}`
}

export function deriveTargetAvatarURL(targetType, targetId) {
  const id = trim(targetId)
  if (targetType === 'private' && numericPattern.test(id)) {
    return `https://q1.qlogo.cn/g?b=qq&nk=${encodeURIComponent(id)}&s=640`
  }
  if (targetType === 'group' && numericPattern.test(id)) {
    return `https://p.qlogo.cn/gh/${encodeURIComponent(id)}/${encodeURIComponent(id)}/100`
  }
  return ''
}

export function normalizeTargets(payload) {
  return {
    loaded: true,
    available: payload && payload.available === true,
    groups: Array.isArray(payload && payload.groups) ? payload.groups : [],
    private_users: Array.isArray(payload && payload.private_users) ? payload.private_users : [],
    issues: Array.isArray(payload && payload.issues) ? payload.issues : [],
  }
}

export function allTargets(targetsState) {
  const state = targetsState || {}
  return [
    ...(state.groups || []).map((target) => ({
      key: targetKey('group', target.target_id),
      target_type: 'group',
      target_id: trim(target.target_id),
      label: trim(target.target_name) || trim(target.target_id),
      avatar_url: trim(target.avatar_url) || deriveTargetAvatarURL('group', target.target_id),
    })),
    ...(state.private_users || []).map((target) => ({
      key: targetKey('private', target.target_id),
      target_type: 'private',
      target_id: trim(target.target_id),
      label: trim(target.nickname) || trim(target.target_id),
      avatar_url: trim(target.avatar_url) || deriveTargetAvatarURL('private', target.target_id),
    })),
  ]
}

export function targetMap(targetsState) {
  return new Map(allTargets(targetsState).map((target) => [target.key, target]))
}

export function currentTargetsForMode(targetsState, mode) {
  return allTargets(targetsState).filter((target) => target.target_type === mode)
}

export function targetDisplay(target, map) {
  const live = map && map.get(target.key)
  const label = live ? live.label : target.target_name || target.target_id
  return `${TARGET_LABELS[target.target_type] || target.target_type} ${label}`
}

export function targetAvatar(target, map) {
  const live = map && map.get(target.key)
  return live ? live.avatar_url : deriveTargetAvatarURL(target.target_type, target.target_id)
}
