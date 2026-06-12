import { SERVICE_LABELS, SERVICE_ORDER, serviceCheckboxValues } from '../services.js'
import { targetDisplay } from '../targets.js'
import { escapeHTML } from './html.js'

export function serviceTagsHTML(services) {
  return serviceCheckboxValues(services).has('all')
    ? '<span class="service-tag">全部</span>'
    : [...serviceCheckboxValues(services)].map((service) => `
      <span class="service-tag">${escapeHTML(SERVICE_LABELS[service] || service)}</span>
    `).join('')
}

export function renderServiceCheckboxes(rowId, targetKeyValue, services) {
  const active = serviceCheckboxValues(services)
  return SERVICE_ORDER.map((service) => `
    <label>
      <input type="checkbox" class="service-checkbox" data-row-id="${escapeHTML(rowId)}" data-target-key="${escapeHTML(targetKeyValue)}" value="${escapeHTML(service)}" ${active.has(service) ? 'checked' : ''} />
      ${escapeHTML(SERVICE_LABELS[service])}
    </label>
  `).join('')
}

export function renderServiceEditor(row, context) {
  if (row.service_mode === 'mixed') {
    return renderMixedServices(row, context.targetMap)
  }
  return `<div class="inline-checks" aria-label="推送类型">${renderServiceCheckboxes(row.row_id, 'common', row.services)}</div>`
}

export function renderMixedServices(row, map) {
  return `
    <div class="target-service-editor">
      <span class="badge badge--warning">目标配置不同</span>
      ${row.targets.map((target) => `
        <div class="target-service-line">
          <span class="row-note">${escapeHTML(targetDisplay(target, map))}</span>
          <div class="inline-checks">${renderServiceCheckboxes(row.row_id, target.key, target.services)}</div>
        </div>
      `).join('')}
    </div>
  `
}
