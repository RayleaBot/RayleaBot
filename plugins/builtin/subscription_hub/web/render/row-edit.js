import { avatarHTML, escapeHTML } from './html.js'
import { renderServiceEditor } from './service-picker.js'
import { renderSubscriberEditor } from './subscriber-editor.js'
import { renderRowValidation } from './status.js'
import { renderSelectedTargets, renderTargetOptions } from './target-picker.js'

export function renderRowEdit(row, context) {
  const title = row.name || row.uid || '未校验 UP'
  const subtitle = row.uid ? `UID ${row.uid}` : '输入 UID 或 Bilibili 用户名后校验'
  const upAvatar = avatarHTML(row.avatar_url, title, 'avatar--up', title)
  const candidates = row.candidates.length
    ? `<div class="candidate-list">${row.candidates.map((candidate) => `
        <button type="button" class="button candidate-button" data-action="choose-candidate" data-row-id="${escapeHTML(row.row_id)}" data-user='${escapeHTML(JSON.stringify(candidate))}'>
          ${avatarHTML(candidate.avatar_url, candidate.name, 'avatar--candidate', candidate.name)}
          <span>${escapeHTML(candidate.name)} · UID ${escapeHTML(candidate.uid)}</span>
        </button>
      `).join('')}</div>`
    : ''

  return `
    <article class="sub-card sub-card--editing ${row.enabled ? '' : 'sub-card--disabled'}" data-row-id="${escapeHTML(row.row_id)}">
      <div class="sub-card__head">
        ${upAvatar}
        <div class="sub-card__meta">
          <strong>${escapeHTML(title)}</strong>
          <small>${escapeHTML(subtitle)}</small>
        </div>
        <div class="sub-card__status">
          <span class="badge">${row.resolved ? '已校验' : row.resolve_state === 'checking' ? '校验中' : '待校验'}</span>
          <div class="row-validation-slot" data-row-id="${escapeHTML(row.row_id)}">${renderRowValidation(row, context)}</div>
        </div>
      </div>

      <div class="sub-card__body">
        <div class="sub-card__section">
          <div class="sub-card__section-title">UP 信息</div>
          <div class="up-input-line">
            <input class="up-query-input" data-row-id="${escapeHTML(row.row_id)}" type="text" autocomplete="off" value="${escapeHTML(row.query)}" placeholder="UID 或 Bilibili 用户名" />
            <button type="button" class="button button--small" data-action="resolve-up" data-row-id="${escapeHTML(row.row_id)}">校验</button>
          </div>
          ${row.resolve_message ? `<div class="row-note">${escapeHTML(row.resolve_message)}</div>` : ''}
          ${candidates}
          <div class="service-editor-slot" data-row-id="${escapeHTML(row.row_id)}">${renderServiceEditor(row, context)}</div>
        </div>

        <div class="sub-card__section">
          <div class="sub-card__section-title">推送对象</div>
          <div class="mode-tabs" role="group" aria-label="推送对象类型">
            <button type="button" class="button button--small ${row.target_mode === 'group' ? 'is-active' : ''}" data-action="target-mode" data-row-id="${escapeHTML(row.row_id)}" data-mode="group">群聊</button>
            <button type="button" class="button button--small ${row.target_mode === 'private' ? 'is-active' : ''}" data-action="target-mode" data-row-id="${escapeHTML(row.row_id)}" data-mode="private">私聊</button>
          </div>
          <div class="target-select" data-row-id="${escapeHTML(row.row_id)}" role="listbox" aria-multiselectable="true" aria-disabled="${context.targets.loaded ? 'false' : 'true'}" tabindex="0">
            <div class="target-options-list">${renderTargetOptions(row, context)}</div>
          </div>
          <div class="chip-list target-chip-list" data-row-id="${escapeHTML(row.row_id)}">${renderSelectedTargets(row, context)}</div>
          ${context.targets.issues.length ? `<div class="target-note">${escapeHTML(context.targets.issues.map((issue) => issue.message).join('；'))}</div>` : ''}
        </div>

        <div class="sub-card__section">
          <div class="sub-card__section-title">订阅人</div>
          ${renderSubscriberEditor(row, context)}
        </div>
      </div>

      <div class="sub-card__actions">
        <div class="button-group">
          <label class="switch-row" title="启用">
            <input type="checkbox" class="row-enabled-input" data-row-id="${escapeHTML(row.row_id)}" ${row.enabled ? 'checked' : ''} />
            <span>${row.enabled ? '已启用' : '已停用'}</span>
          </label>
        </div>
        <div class="button-group">
          <button type="button" class="button button--primary button--small" data-action="finish-edit" data-row-id="${escapeHTML(row.row_id)}">完成</button>
          <button type="button" class="button button--ghost button--small" data-action="cancel-edit" data-row-id="${escapeHTML(row.row_id)}">取消</button>
          <button type="button" class="button button--small" data-action="duplicate-row" data-row-id="${escapeHTML(row.row_id)}">复制</button>
          <button type="button" class="button button--small button--danger" data-action="delete-row" data-row-id="${escapeHTML(row.row_id)}">删除</button>
        </div>
      </div>
    </article>
  `
}
