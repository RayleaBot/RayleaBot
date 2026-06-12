import { currentTargetsForMode, targetAvatar, targetDisplay } from '../targets.js'
import { avatarHTML, escapeHTML } from './html.js'

export function renderSelectedTargets(row, context) {
  const map = context.targetMap
  return row.targets.length
    ? row.targets.map((target) => `
        <span class="chip ${map.has(target.key) ? '' : 'badge--warning'}">
          ${avatarHTML(targetAvatar(target, map), targetDisplay(target, map), 'avatar--candidate', targetDisplay(target, map))}
          <span>${escapeHTML(targetDisplay(target, map))}</span>
          <button type="button" aria-label="移除推送对象" data-action="remove-target" data-row-id="${escapeHTML(row.row_id)}" data-target-key="${escapeHTML(target.key)}">×</button>
        </span>
      `).join('')
    : '<span class="chip">未选择推送对象</span>'
}

export function renderTargetOptions(row, context) {
  const selected = new Set(row.targets.map((target) => target.key))
  const targets = currentTargetsForMode(context.targets, row.target_mode)
  if (!targets.length) {
    return '<div class="target-option-empty">没有可选对象</div>'
  }
  return targets.map((target) => {
    const isSelected = selected.has(target.key)
    return `
      <button type="button" class="target-option ${isSelected ? 'is-selected' : ''}" data-action="toggle-target" data-row-id="${escapeHTML(row.row_id)}" data-target-key="${escapeHTML(target.key)}" role="option" aria-selected="${isSelected ? 'true' : 'false'}">
        <span class="target-option__mark" aria-hidden="true">${isSelected ? '✓' : ''}</span>
        <span class="target-option__label">${escapeHTML(target.label)}</span>
        <span class="target-option__id">${escapeHTML(target.target_id)}</span>
      </button>
    `
  }).join('')
}
