import { subscriberAvatarURL } from '../subscribers.js'
import { avatarHTML, escapeHTML } from './html.js'

export function renderSubscriberChips(row, context) {
  return row.subscriber_ids.length
    ? row.subscriber_ids.map((id) => `
        <span class="chip">
          ${avatarHTML(subscriberAvatarURL(context.subscriberAvatars, id), `QQ ${id}`, 'avatar--candidate', `QQ ${id}`)}
          <span>QQ ${escapeHTML(id)}</span>
          <button type="button" aria-label="移除订阅人" data-action="remove-subscriber" data-row-id="${escapeHTML(row.row_id)}" data-user-id="${escapeHTML(id)}">×</button>
        </span>
      `).join('')
    : '<span class="chip chip--success">系统订阅</span>'
}

export function renderSubscriberEditor(row, context) {
  return `
    <div class="subscriber-line">
      <input class="subscriber-input" data-row-id="${escapeHTML(row.row_id)}" type="text" inputmode="numeric" autocomplete="off" placeholder="QQ 号，留空为系统订阅" />
      <button type="button" class="button button--small" data-action="add-subscriber" data-row-id="${escapeHTML(row.row_id)}">添加</button>
    </div>
    <div class="chip-list subscriber-chip-list" data-row-id="${escapeHTML(row.row_id)}">${renderSubscriberChips(row, context)}</div>
    <div class="row-note">只保存 QQ 号，昵称和群名片保存时刷新。</div>
  `
}
