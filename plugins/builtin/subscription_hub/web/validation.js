import { hasServiceSelection, trim, unique } from './services.js'
import { platformLabel, safeSubjectId, subjectLabel } from './platforms.js'
import { numericPattern } from './subscribers.js'
import { targetDisplay } from './targets.js'

export function validateRow(row, context) {
  const map = context && context.targetMap ? context.targetMap : new Map()
  const targetsLoaded = Boolean(context && context.targetsLoaded)
  const errors = []
  const uid = trim(row.uid)
  if (!row.resolved || !safeSubjectId(uid, row.platform) || !row.name) {
    errors.push(`${platformLabel(row.platform)} ${subjectLabel(row.platform)} 未完成`)
  }
  if (!targetsLoaded) {
    errors.push('推送对象未载入')
  }
  if (!row.targets.length) {
    errors.push('请选择推送对象')
  }
  for (const target of row.targets) {
    if (!map.has(target.key)) {
      errors.push(`${targetDisplay(target, map)} 不在协议对象列表中`)
    }
    if (row.service_mode === 'mixed' && !hasServiceSelection(target.services)) {
      errors.push(`${targetDisplay(target, map)} 请选择推送类型`)
    }
  }
  if (row.service_mode !== 'mixed' && !hasServiceSelection(row.services)) {
    errors.push('请选择推送类型')
  }
  for (const id of row.subscriber_ids) {
    if (!numericPattern.test(trim(id))) {
      errors.push(`订阅人 QQ 不合法：${id}`)
    }
  }
  return unique(errors)
}

export function validateRows(rows, context) {
  const errors = (rows || []).flatMap((row) => validateRow(row, context))
  return { ok: errors.length === 0, errors }
}
