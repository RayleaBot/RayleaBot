import { escapeHTML } from './html.js'
import { validateRow } from '../validation.js'

export function renderRowValidation(row, context) {
  const validation = validateRow(row, context)
  return validation.length
    ? `<ul class="validation-list">${validation.map((item) => `<li>${escapeHTML(item)}</li>`).join('')}</ul>`
    : '<span class="badge badge--success">可保存</span>'
}

export function renderValidationBadge(row, context) {
  return validateRow(row, context).length
    ? '<span class="badge badge--danger">需处理</span>'
    : '<span class="badge badge--success">可保存</span>'
}
