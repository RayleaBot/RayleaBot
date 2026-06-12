import { subscriberAvatarURL } from '../subscribers.js'
import { targetAvatar, targetDisplay } from '../targets.js'
import { avatarHTML, avatarStackHTML, escapeHTML } from './html.js'
import { serviceTagsHTML } from './service-picker.js'
import { renderValidationBadge } from './status.js'

export function renderRowView(row, context) {
  const map = context.targetMap
  const title = row.name || row.uid || '未校验 UP'
  const subtitle = row.uid ? `UID ${row.uid}` : '输入 UID 或 Bilibili 用户名后校验'
  const upAvatar = avatarHTML(row.avatar_url, title, 'avatar--up', title)
  const services = row.service_mode === 'mixed'
    ? '<span class="service-tag">目标配置不同</span>'
    : serviceTagsHTML(row.services)
  const targetSummaryItems = row.targets.map((target) => ({
    avatar_url: targetAvatar(target, map),
    label: targetDisplay(target, map),
  }))
  const targetStack = avatarStackHTML(targetSummaryItems, 5, 'avatar--target', (item) => item.avatar_url, (item) => item.label)
  const targetLabel = row.targets.length ? `${row.targets.length} 个推送对象` : '未选择推送对象'

  const subscriberItems = row.subscriber_ids.map((id) => ({
    id,
    avatar_url: subscriberAvatarURL(context.subscriberAvatars, id),
  }))
  const subscriberStack = row.subscriber_ids.length
    ? avatarStackHTML(subscriberItems, 5, 'avatar--subscriber', (item) => item.avatar_url, (item) => `QQ ${item.id}`)
    : '<span class="chip chip--success">系统订阅</span>'
  const subscriberLabel = row.subscriber_ids.length ? `${row.subscriber_ids.length} 位订阅人` : '系统订阅'

  return `
    <article class="sub-card ${row.enabled ? '' : 'sub-card--disabled'}" data-row-id="${escapeHTML(row.row_id)}">
      <div class="sub-card__head">
        ${upAvatar}
        <div class="sub-card__meta">
          <strong>${escapeHTML(title)}</strong>
          <small>${escapeHTML(subtitle)}</small>
        </div>
        <div class="sub-card__status">
          ${row.resolved ? '<span class="badge">已校验</span>' : ''}
          ${renderValidationBadge(row, context)}
          <label class="switch-row" title="启用">
            <input type="checkbox" class="row-enabled-input" data-row-id="${escapeHTML(row.row_id)}" ${row.enabled ? 'checked' : ''} />
          </label>
        </div>
      </div>

      <div class="sub-card__body">
        <div class="sub-card__section">
          <div class="sub-card__section-title">推送类型</div>
          <div class="sub-card__services">${services}</div>
        </div>

        <div class="sub-card__section">
          <div class="sub-card__section-title">推送对象</div>
          <div class="sub-card__targets-summary">
            ${targetStack}
            <span class="sub-card__summary-label">${escapeHTML(targetLabel)}</span>
          </div>
        </div>

        <div class="sub-card__section">
          <div class="sub-card__section-title">订阅人</div>
          <div class="sub-card__subscribers-summary">
            ${subscriberStack}
            <span class="sub-card__summary-label">${escapeHTML(subscriberLabel)}</span>
          </div>
        </div>
      </div>

      <div class="sub-card__actions">
        <button type="button" class="button button--primary button--small" data-action="edit-row" data-row-id="${escapeHTML(row.row_id)}">编辑</button>
        <button type="button" class="button button--small" data-action="duplicate-row" data-row-id="${escapeHTML(row.row_id)}">复制</button>
        <button type="button" class="button button--small button--danger" data-action="delete-row" data-row-id="${escapeHTML(row.row_id)}">删除</button>
      </div>
    </article>
  `
}
