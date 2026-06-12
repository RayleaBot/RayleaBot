(function () {
  'use strict'

  const SERVICE_ORDER = ['all', 'live', 'video', 'image_text', 'article', 'repost']
  const SERVICE_TYPES = SERVICE_ORDER.filter((service) => service !== 'all')
  const SERVICE_LABELS = {
    all: '全部',
    live: '直播',
    video: '视频',
    image_text: '图文',
    article: '文章',
    repost: '转发',
  }
  const TARGET_LABELS = {
    group: '群聊',
    private: '私聊',
  }
  const numericPattern = /^[0-9]+$/

  const elements = {
    statusText: document.getElementById('status-text'),
    enabledInput: document.getElementById('enabled-input'),
    metricEnabled: document.getElementById('metric-enabled'),
    metricSubscriptions: document.getElementById('metric-subscriptions'),
    metricTargets: document.getElementById('metric-targets'),
    metricValidation: document.getElementById('metric-validation'),
    targetsReloadButton: document.getElementById('targets-reload-button'),
    searchInput: document.getElementById('subscription-search-input'),
    statusFilter: document.getElementById('status-filter-input'),
    serviceFilter: document.getElementById('service-filter-input'),
    addButton: document.getElementById('add-subscription-button'),
    list: document.getElementById('subscription-list'),
    dirtyState: document.getElementById('dirty-state'),
    reloadButton: document.getElementById('reload-button'),
    resetButton: document.getElementById('reset-button'),
    manualCheckButton: document.getElementById('manual-check-button'),
    previewButton: document.getElementById('preview-button'),
    saveButton: document.getElementById('save-button'),
  }

  const state = {
    defaultSettings: { enabled: true, subscriptions: [] },
    settings: { enabled: true, subscriptions: [] },
    rows: [],
    loaded: false,
    dirty: false,
    rowCounter: 0,
    requestCounter: 0,
    savingRequestId: '',
    pending: new Map(),
    resolveTimers: new Map(),
    subscriberAvatars: new Map(),
    targets: {
      loaded: false,
      available: false,
      groups: [],
      private_users: [],
      issues: [],
    },
  }

  function escapeHTML(value) {
    return String(value ?? '')
      .replaceAll('&', '&amp;')
      .replaceAll('<', '&lt;')
      .replaceAll('>', '&gt;')
      .replaceAll('"', '&quot;')
      .replaceAll("'", '&#39;')
  }

  function selectorValue(value) {
    return String(value ?? '').replaceAll('\\', '\\\\').replaceAll('"', '\\"')
  }

  function trim(value) {
    return String(value ?? '').trim()
  }

  function unique(values) {
    return [...new Set(values.map(trim).filter(Boolean))]
  }

  function normalizeServices(value) {
    const services = unique(Array.isArray(value) ? value : ['all'])
      .filter((item) => SERVICE_ORDER.includes(item))
    if (!services.length || services.includes('all')) {
      return ['all']
    }
    const selected = SERVICE_TYPES.filter((service) => services.includes(service))
    return selected.length === SERVICE_TYPES.length ? ['all'] : selected
  }

  function serviceCheckboxValues(value) {
    if (Array.isArray(value) && value.length === 0) {
      return new Set()
    }
    const services = normalizeServices(value)
    if (services.includes('all')) {
      return new Set(SERVICE_ORDER)
    }
    return new Set(services)
  }

  function hasServiceSelection(value) {
    return !(Array.isArray(value) && value.length === 0)
  }

  function servicesKey(services) {
    return normalizeServices(services).join(',')
  }

  function servicesText(services) {
    return normalizeServices(services).map((service) => SERVICE_LABELS[service] || service).join('、')
  }

  function serviceTagsHTML(services) {
    return normalizeServices(services).map((service) => `
      <span class="service-tag">${escapeHTML(SERVICE_LABELS[service] || service)}</span>
    `).join('')
  }

  function targetKey(targetType, targetId) {
    return `${trim(targetType)}:${trim(targetId)}`
  }

  function nextRowId() {
    state.rowCounter += 1
    return `row-${Date.now()}-${state.rowCounter}`
  }

  function nextRequestId(prefix) {
    state.requestCounter += 1
    return `${prefix}-${Date.now()}-${state.requestCounter}`
  }

  function postMessage(type, payload, requestId) {
    window.parent.postMessage({
      version: '1',
      source: 'plugin_management_ui',
      type,
      request_id: requestId || nextRequestId(type.replaceAll('.', '-')),
      ...(payload === undefined ? {} : { payload }),
    }, '*')
  }

  function generateHueFromString(value) {
    let hash = 0
    const text = trim(value) || '?'
    for (let i = 0; i < text.length; i += 1) {
      hash = text.charCodeAt(i) + ((hash << 5) - hash)
    }
    return Math.abs(hash) % 360
  }

  function avatarHTML(avatarUrl, fallbackText, sizeClass, alt) {
    const hue = generateHueFromString(fallbackText || '?')
    const bg = `hsl(${hue} 72% 58%)`
    const text = (fallbackText || '?').slice(0, 1).toUpperCase()
    const safeBg = escapeHTML(bg)
    const safeText = escapeHTML(text)
    const safeAlt = escapeHTML(alt || '')
    const safeUrl = escapeHTML(avatarUrl || '')
    const safeSize = escapeHTML(sizeClass)
    return `
      <span class="avatar ${safeSize}" style="background:${safeBg}" aria-label="${safeAlt}">
        <img src="${safeUrl}" alt="${safeAlt}" loading="lazy" referrerpolicy="no-referrer" onerror="this.style.display='none'; this.parentNode.querySelector('.avatar-fallback__text').style.display='flex'" />
        <span class="avatar-fallback__text">${safeText}</span>
      </span>
    `
  }

  function avatarStackHTML(items, maxVisible, sizeClass, getAvatar, getLabel) {
    if (!items.length) {
      return '<span class="sub-card__summary-label">无</span>'
    }
    const visible = items.slice(0, maxVisible)
    const overflow = items.length - visible.length
    const avatars = visible.map((item) => avatarHTML(getAvatar(item), getLabel(item), sizeClass, getLabel(item))).join('')
    const overflowHTML = overflow > 0 ? `<span class="avatar-stack__overflow">+${overflow}</span>` : ''
    return `<span class="avatar-stack">${avatars}${overflowHTML}</span>`
  }

  function deriveTargetAvatarURL(targetType, targetId) {
    const id = trim(targetId)
    if (targetType === 'private' && numericPattern.test(id)) {
      return `https://q1.qlogo.cn/g?b=qq&nk=${encodeURIComponent(id)}&s=640`
    }
    if (targetType === 'group' && numericPattern.test(id)) {
      return `https://p.qlogo.cn/gh/${encodeURIComponent(id)}/${encodeURIComponent(id)}/100`
    }
    return ''
  }

  function subscriberAvatarURL(userId) {
    return state.subscriberAvatars.get(trim(userId)) || `https://q1.qlogo.cn/g?b=qq&nk=${encodeURIComponent(trim(userId))}&s=640`
  }

  function normalizeSubscriber(value) {
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

  function normalizeSubscription(value) {
    if (!value || typeof value !== 'object') {
      return null
    }
    const uid = trim(value.uid)
    const targetType = trim(value.target_type)
    const targetId = trim(value.target_id)
    if (!numericPattern.test(uid) || !['group', 'private'].includes(targetType) || !numericPattern.test(targetId)) {
      return null
    }
    return {
      id: trim(value.id),
      platform: 'bilibili',
      uid,
      name: trim(value.name) || uid,
      avatar_url: trim(value.avatar_url),
      target_type: targetType,
      target_id: targetId,
      target_name: trim(value.target_name),
      services: normalizeServices(value.services),
      subscribers: Array.isArray(value.subscribers)
        ? value.subscribers.map(normalizeSubscriber).filter(Boolean)
        : [],
      enabled: value.enabled !== false,
    }
  }

  function normalizeSettings(value) {
    const record = value && typeof value === 'object' ? value : {}
    return {
      enabled: record.enabled !== false,
      subscriptions: Array.isArray(record.subscriptions)
        ? record.subscriptions.map(normalizeSubscription).filter(Boolean)
        : [],
    }
  }

  function createBlankRow() {
    return {
      row_id: nextRowId(),
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

  function buildRowsFromSettings(settings) {
    const grouped = new Map()
    for (const subscription of settings.subscriptions || []) {
      let row = grouped.get(subscription.uid)
      if (!row) {
        row = {
          row_id: `uid-${subscription.uid}`,
          uid: subscription.uid,
          name: subscription.name || subscription.uid,
          avatar_url: subscription.avatar_url || '',
          query: subscription.name || subscription.uid,
          resolved: true,
          resolve_state: 'resolved',
          resolve_message: '',
          candidates: [],
          enabled: false,
          services: normalizeServices(subscription.services),
          service_mode: 'common',
          target_mode: subscription.target_type || 'group',
          targets: [],
          subscriber_ids: [],
          edit_mode: false,
          _editSnapshot: null,
        }
        grouped.set(subscription.uid, row)
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
        services: normalizeServices(subscription.services),
      })
      for (const subscriber of subscription.subscribers || []) {
        if (subscriber.id) {
          row.subscriber_ids.push(subscriber.id)
          if (subscriber.avatar_url) {
            state.subscriberAvatars.set(subscriber.id, subscriber.avatar_url)
          }
        }
      }
    }

    const rows = [...grouped.values()]
    for (const row of rows) {
      row.subscriber_ids = unique(row.subscriber_ids)
      const serviceKeys = unique(row.targets.map((target) => servicesKey(target.services)))
      if (serviceKeys.length > 1) {
        row.service_mode = 'mixed'
      } else if (serviceKeys.length === 1) {
        row.services = row.targets[0].services
      }
    }
    return rows
  }

  function allTargets() {
    return [
      ...state.targets.groups.map((target) => ({
        key: targetKey('group', target.target_id),
        target_type: 'group',
        target_id: trim(target.target_id),
        label: trim(target.target_name) || trim(target.target_id),
        avatar_url: trim(target.avatar_url) || deriveTargetAvatarURL('group', target.target_id),
      })),
      ...state.targets.private_users.map((target) => ({
        key: targetKey('private', target.target_id),
        target_type: 'private',
        target_id: trim(target.target_id),
        label: trim(target.nickname) || trim(target.target_id),
        avatar_url: trim(target.avatar_url) || deriveTargetAvatarURL('private', target.target_id),
      })),
    ]
  }

  function targetMap() {
    return new Map(allTargets().map((target) => [target.key, target]))
  }

  function currentTargetsForMode(mode) {
    return allTargets().filter((target) => target.target_type === mode)
  }

  function targetDisplay(target, map) {
    const live = map.get(target.key)
    const label = live ? live.label : target.target_name || target.target_id
    return `${TARGET_LABELS[target.target_type] || target.target_type} ${label}`
  }

  function targetAvatar(target, map) {
    const live = map.get(target.key)
    return live ? live.avatar_url : deriveTargetAvatarURL(target.target_type, target.target_id)
  }

  function rowSearchText(row) {
    const map = targetMap()
    return [
      row.uid,
      row.name,
      row.query,
      ...row.targets.map((target) => `${target.target_id} ${targetDisplay(target, map)}`),
      ...row.subscriber_ids,
    ].join(' ').toLowerCase()
  }

  function rowVisible(row) {
    const query = trim(elements.searchInput.value).toLowerCase()
    if (query && !rowSearchText(row).includes(query)) {
      return false
    }
    const status = elements.statusFilter.value
    if (status === 'enabled' && !row.enabled) {
      return false
    }
    if (status === 'disabled' && row.enabled) {
      return false
    }
    const service = elements.serviceFilter.value
    if (service !== 'all') {
      const services = row.service_mode === 'mixed'
        ? row.targets.flatMap((target) => target.services)
        : row.services
      if (!services.includes('all') && !services.includes(service)) {
        return false
      }
    }
    return true
  }

  function renderPageState() {
    elements.enabledInput.checked = state.settings.enabled !== false
    elements.metricEnabled.textContent = state.settings.enabled === false ? '停用' : '启用'
    elements.metricSubscriptions.textContent = `${state.rows.length} / ${(state.settings.subscriptions || []).length}`
    if (!state.targets.loaded) {
      elements.metricTargets.textContent = '未载入'
    } else {
      elements.metricTargets.textContent = `${state.targets.groups.length} 群聊 / ${state.targets.private_users.length} 私聊`
    }
    const validation = validateRows()
    elements.metricValidation.textContent = validation.ok ? '可保存' : '需处理'
    elements.saveButton.disabled = !state.loaded || !validation.ok || state.savingRequestId !== ''
    elements.dirtyState.textContent = state.savingRequestId
      ? '正在保存'
      : state.dirty
        ? '设置有修改'
        : state.loaded
          ? '设置已同步'
          : '等待载入'
  }

  function render() {
    renderPageState()
    const visibleRows = state.rows.filter(rowVisible)
    elements.list.innerHTML = visibleRows.length
      ? visibleRows.map(renderRow).join('')
      : '<div class="empty-state"><p>没有匹配的订阅</p><p>可添加订阅或调整筛选条件</p></div>'
  }

  function renderRow(row) {
    return row.edit_mode ? renderRowEdit(row) : renderRowView(row)
  }

  function renderRowView(row) {
    const map = targetMap()
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
      avatar_url: subscriberAvatarURL(id),
    }))
    const subscriberStack = row.subscriber_ids.length
      ? avatarStackHTML(subscriberItems, 5, 'avatar--subscriber', (item) => item.avatar_url, (item) => `QQ ${item.id}`)
      : '<span class="chip chip--success">系统订阅</span>'
    const subscriberLabel = row.subscriber_ids.length ? `${row.subscriber_ids.length} 位订阅人` : '系统订阅'

    const validation = validateRow(row)
    const statusBadge = validation.length
      ? '<span class="badge badge--danger">需处理</span>'
      : '<span class="badge badge--success">可保存</span>'

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
            ${statusBadge}
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

  function renderRowEdit(row) {
    const map = targetMap()
    const title = row.name || row.uid || '未校验 UP'
    const subtitle = row.uid ? `UID ${row.uid}` : '输入 UID 或 Bilibili 用户名后校验'
    const upAvatar = avatarHTML(row.avatar_url, title, 'avatar--up', title)
    const serviceEditor = row.service_mode === 'mixed'
      ? renderMixedServices(row, map)
      : `<div class="inline-checks" aria-label="推送类型">${renderServiceCheckboxes(row.row_id, 'common', row.services)}</div>`
    const candidates = row.candidates.length
      ? `<div class="candidate-list">${row.candidates.map((candidate) => `
          <button type="button" class="button candidate-button" data-action="choose-candidate" data-row-id="${escapeHTML(row.row_id)}" data-user='${escapeHTML(JSON.stringify(candidate))}'>
            ${avatarHTML(candidate.avatar_url, candidate.name, 'avatar--candidate', candidate.name)}
            <span>${escapeHTML(candidate.name)} · UID ${escapeHTML(candidate.uid)}</span>
          </button>
        `).join('')}</div>`
      : ''
    const targetOptions = renderTargetOptions(row)
    const selectedTargets = renderSelectedTargets(row, map)
    const subscribers = row.subscriber_ids.length
      ? row.subscriber_ids.map((id) => `
          <span class="chip">
            ${avatarHTML(subscriberAvatarURL(id), `QQ ${id}`, 'avatar--candidate', `QQ ${id}`)}
            <span>QQ ${escapeHTML(id)}</span>
            <button type="button" aria-label="移除订阅人" data-action="remove-subscriber" data-row-id="${escapeHTML(row.row_id)}" data-user-id="${escapeHTML(id)}">×</button>
          </span>
        `).join('')
      : '<span class="chip chip--success">系统订阅</span>'
    const validationHTML = renderRowValidation(row)

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
            <div class="row-validation-slot" data-row-id="${escapeHTML(row.row_id)}">${validationHTML}</div>
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
            <div class="service-editor-slot" data-row-id="${escapeHTML(row.row_id)}">${serviceEditor}</div>
          </div>

          <div class="sub-card__section">
            <div class="sub-card__section-title">推送对象</div>
            <div class="mode-tabs" role="group" aria-label="推送对象类型">
              <button type="button" class="button button--small ${row.target_mode === 'group' ? 'is-active' : ''}" data-action="target-mode" data-row-id="${escapeHTML(row.row_id)}" data-mode="group">群聊</button>
              <button type="button" class="button button--small ${row.target_mode === 'private' ? 'is-active' : ''}" data-action="target-mode" data-row-id="${escapeHTML(row.row_id)}" data-mode="private">私聊</button>
            </div>
            <div class="target-select" data-row-id="${escapeHTML(row.row_id)}" role="listbox" aria-multiselectable="true" aria-disabled="${state.targets.loaded ? 'false' : 'true'}" tabindex="0">
              <div class="target-options-list">${targetOptions}</div>
            </div>
            <div class="chip-list target-chip-list" data-row-id="${escapeHTML(row.row_id)}">${selectedTargets}</div>
            ${state.targets.issues.length ? `<div class="target-note">${escapeHTML(state.targets.issues.map((issue) => issue.message).join('；'))}</div>` : ''}
          </div>

          <div class="sub-card__section">
            <div class="sub-card__section-title">订阅人</div>
            <div class="subscriber-line">
              <input class="subscriber-input" data-row-id="${escapeHTML(row.row_id)}" type="text" inputmode="numeric" autocomplete="off" placeholder="QQ 号，留空为系统订阅" />
              <button type="button" class="button button--small" data-action="add-subscriber" data-row-id="${escapeHTML(row.row_id)}">添加</button>
            </div>
            <div class="chip-list">${subscribers}</div>
            <div class="row-note">只保存 QQ 号，昵称和群名片保存时刷新。</div>
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

  function renderServiceCheckboxes(rowId, targetKeyValue, services) {
    const active = serviceCheckboxValues(services)
    return SERVICE_ORDER.map((service) => `
      <label>
        <input type="checkbox" class="service-checkbox" data-row-id="${escapeHTML(rowId)}" data-target-key="${escapeHTML(targetKeyValue)}" value="${escapeHTML(service)}" ${active.has(service) ? 'checked' : ''} />
        ${escapeHTML(SERVICE_LABELS[service])}
      </label>
    `).join('')
  }

  function renderMixedServices(row, map) {
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

  function nextFrame(callback) {
    const schedule = window.requestAnimationFrame || ((fn) => window.setTimeout(fn, 0))
    schedule.call(window, callback)
  }

  function renderSelectedTargets(row, map) {
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

  function renderTargetOptions(row) {
    const selected = new Set(row.targets.map((target) => target.key))
    const targets = currentTargetsForMode(row.target_mode)
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

  function renderRowValidation(row) {
    const validation = validateRow(row)
    return validation.length
      ? `<ul class="validation-list">${validation.map((item) => `<li>${escapeHTML(item)}</li>`).join('')}</ul>`
      : '<span class="badge badge--success">可保存</span>'
  }

  function refreshRowTargetEditor(row) {
    const card = elements.list.querySelector(`.sub-card[data-row-id="${selectorValue(row.row_id)}"]`)
    if (!card) {
      render()
      return
    }

    const map = targetMap()
    const targetList = card.querySelector('.target-select')
    const targetScrollTop = targetList ? targetList.scrollTop : 0
    const optionsList = card.querySelector('.target-options-list')
    if (optionsList) {
      optionsList.innerHTML = renderTargetOptions(row)
    }

    const targetChips = card.querySelector('.target-chip-list')
    if (targetChips) {
      targetChips.innerHTML = renderSelectedTargets(row, map)
    }

    const validationSlot = card.querySelector('.row-validation-slot')
    if (validationSlot) {
      validationSlot.innerHTML = renderRowValidation(row)
    }

    const serviceSlot = card.querySelector('.service-editor-slot')
    if (serviceSlot) {
      serviceSlot.innerHTML = row.service_mode === 'mixed'
        ? renderMixedServices(row, map)
        : `<div class="inline-checks" aria-label="推送类型">${renderServiceCheckboxes(row.row_id, 'common', row.services)}</div>`
    }

    renderPageState()
    if (targetList) {
      targetList.scrollTop = targetScrollTop
    }
  }

  function cloneRow(row) {
    return JSON.parse(JSON.stringify(row))
  }

  function scrollRowIntoCenter(row) {
    nextFrame(() => {
      const element = elements.list.querySelector(`.sub-card[data-row-id="${selectorValue(row.row_id)}"]`)
      if (element && typeof element.scrollIntoView === 'function') {
        element.scrollIntoView({ block: 'center', behavior: 'smooth' })
      }
    })
  }

  function beginEdit(row) {
    row._editSnapshot = cloneRow(row)
    row.edit_mode = true
    render()
    scrollRowIntoCenter(row)
  }

  function cancelEdit(row) {
    if (row._editSnapshot) {
      const snapshot = row._editSnapshot
      const preservedRowId = row.row_id
      Object.assign(row, snapshot)
      row.row_id = preservedRowId
      row._editSnapshot = null
      row.edit_mode = false
    }
    render()
  }

  function finishEdit(row) {
    const errors = validateRow(row)
    if (errors.length) {
      setStatus(errors[0] || '行未通过校验')
      render()
      return
    }
    row._editSnapshot = null
    row.edit_mode = false
    markDirty()
  }

  function findRow(rowId) {
    return state.rows.find((row) => row.row_id === rowId)
  }

  function markDirty() {
    state.dirty = true
    render()
  }

  function setStatus(text) {
    elements.statusText.textContent = text
  }

  function validateRow(row) {
    const errors = []
    if (!row.resolved || !numericPattern.test(row.uid) || !row.name) {
      errors.push('UP 未完成校验')
    }
    if (!state.targets.loaded) {
      errors.push('推送对象未载入')
    }
    if (!row.targets.length) {
      errors.push('请选择推送对象')
    }
    const map = targetMap()
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
      if (!numericPattern.test(id)) {
        errors.push(`订阅人 QQ 不合法：${id}`)
      }
    }
    return unique(errors)
  }

  function validateRows() {
    const errors = state.rows.flatMap(validateRow)
    return { ok: errors.length === 0, errors }
  }

  function readCheckedServices(rowId, targetKeyValue, changedService, isChecked) {
    if (changedService === 'all') {
      return isChecked ? ['all'] : []
    }
    const checkedServices = [...elements.list.querySelectorAll(`.service-checkbox[data-row-id="${selectorValue(rowId)}"][data-target-key="${selectorValue(targetKeyValue)}"]:checked`)]
      .map((input) => input.value)
      .filter((service) => service !== 'all')
    if (SERVICE_TYPES.every((service) => checkedServices.includes(service))) {
      return ['all']
    }
    return SERVICE_TYPES.filter((service) => checkedServices.includes(service))
  }

  function updateService(row, targetKeyValue, changedService, isChecked) {
    if (targetKeyValue === 'common') {
      row.service_mode = 'common'
      row.services = readCheckedServices(row.row_id, 'common', changedService, isChecked)
      for (const target of row.targets) {
        target.services = row.services
      }
      return
    }
    const target = row.targets.find((item) => item.key === targetKeyValue)
    if (target) {
      target.services = readCheckedServices(row.row_id, targetKeyValue, changedService, isChecked)
      const serviceKeys = unique(row.targets.map((item) => servicesKey(item.services)))
      row.service_mode = serviceKeys.length > 1 ? 'mixed' : 'common'
      if (row.service_mode === 'common' && row.targets[0]) {
        row.services = row.targets[0].services
      }
    }
  }

  function requestTargets() {
    setStatus('正在刷新推送对象…')
    postMessage('protocol.targets.reload', undefined, nextRequestId('protocol-targets'))
  }

  function requestBilibiliResolve(row, immediate) {
    const query = trim(row.query)
    if (!query) {
      row.resolved = false
      row.resolve_state = 'error'
      row.resolve_message = '请填写 UID 或 Bilibili 用户名。'
      render()
      return
    }
    const run = () => {
      const requestId = nextRequestId('bilibili-user')
      state.pending.set(requestId, { kind: 'bilibili-user', row_id: row.row_id, query })
      row.resolve_state = 'checking'
      row.resolve_message = '正在校验 UP…'
      row.candidates = []
      row.resolved = false
      render()
      postMessage('bilibili.user.resolve', { query }, requestId)
    }
    if (immediate) {
      run()
      return
    }
    clearTimeout(state.resolveTimers.get(row.row_id))
    state.resolveTimers.set(row.row_id, setTimeout(run, 450))
  }

  function applyBilibiliResolved(message) {
    const request = state.pending.get(message.request_id)
    if (!request || request.kind !== 'bilibili-user') {
      return
    }
    state.pending.delete(message.request_id)
    const row = findRow(request.row_id)
    if (!row || request.query !== message.payload.query) {
      return
    }
    if (message.payload.exact && message.payload.user) {
      applyResolvedUser(row, message.payload.user)
      row.resolve_message = 'UP 已校验。'
    } else {
      row.resolved = false
      row.resolve_state = 'error'
      row.resolve_message = message.payload.message || '请选择一个候选 UP 后保存。'
      row.candidates = Array.isArray(message.payload.candidates) ? message.payload.candidates : []
    }
    markDirty()
  }

  function applyResolvedUser(row, user) {
    row.uid = trim(user.uid)
    row.name = trim(user.name)
    row.avatar_url = trim(user.avatar_url)
    row.query = row.name || row.uid
    row.resolved = Boolean(row.uid && row.name)
    row.resolve_state = row.resolved ? 'resolved' : 'error'
    row.candidates = []
  }

  function addTargetToRow(row, liveTarget) {
    if (row.targets.some((target) => target.key === liveTarget.key)) {
      return
    }
    const services = row.service_mode === 'mixed' ? ['all'] : row.services
    row.targets.push({
      key: liveTarget.key,
      subscription_id: '',
      target_type: liveTarget.target_type,
      target_id: liveTarget.target_id,
      target_name: liveTarget.label,
      services: normalizeServices(services),
    })
  }

  function updateTargetsFromSelect(row, selectedKeys) {
    const liveTargets = currentTargetsForMode(row.target_mode)
    const liveMap = new Map(liveTargets.map((target) => [target.key, target]))
    row.targets = row.targets.filter((target) => target.target_type !== row.target_mode || selectedKeys.includes(target.key))
    for (const key of selectedKeys) {
      const liveTarget = liveMap.get(key)
      if (liveTarget) {
        addTargetToRow(row, liveTarget)
      }
    }
  }

  function toggleTarget(row, targetKeyValue) {
    const currentModeKeys = row.targets
      .filter((target) => target.target_type === row.target_mode)
      .map((target) => target.key)
    const selected = new Set(currentModeKeys)
    if (selected.has(targetKeyValue)) {
      selected.delete(targetKeyValue)
    } else {
      selected.add(targetKeyValue)
    }
    updateTargetsFromSelect(row, [...selected])
  }

  function addSubscriber(row, input) {
    if (!input) {
      return
    }
    const id = trim(input.value)
    if (!numericPattern.test(id)) {
      setStatus('订阅人 QQ 号不正确')
      return
    }
    row.subscriber_ids = unique([...row.subscriber_ids, id])
    input.value = ''
    markDirty()
  }

  function buildIdentityRequests() {
    const items = []
    for (const row of state.rows) {
      for (const target of row.targets) {
        for (const userId of row.subscriber_ids) {
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
      const key = `${item.target_type}:${item.target_id}:${item.user_id}`
      if (seen.has(key)) {
        return false
      }
      seen.add(key)
      return true
    })
  }

  function identityKey(targetType, targetId, userId) {
    return `${targetType}:${targetId}:${userId}`
  }

  function buildSettingsPayload(identityItems) {
    const targets = targetMap()
    const identities = new Map((identityItems || []).map((item) => [identityKey(item.target_type, item.target_id, item.user_id), item]))
    const subscriptions = []
    for (const row of state.rows) {
      for (const target of row.targets) {
        const live = targets.get(target.key)
        const targetName = live ? live.label : target.target_name
        const subscribers = row.subscriber_ids.map((userId) => {
          const identity = identities.get(identityKey(target.target_type, target.target_id, userId))
          return {
            id: userId,
            nickname: trim(identity && identity.nickname),
            group_nickname: trim(identity && identity.group_nickname),
            title: trim(identity && identity.title),
            role: trim(identity && identity.role),
            role_label: trim(identity && identity.role_label),
            avatar_url: trim(identity && identity.avatar_url),
          }
        })
        subscriptions.push({
          id: target.subscription_id || `bilibili-${row.uid}-${target.target_type}-${target.target_id}`,
          platform: 'bilibili',
          uid: row.uid,
          name: row.name,
          avatar_url: row.avatar_url,
          target_type: target.target_type,
          target_id: target.target_id,
          target_name: targetName,
          services: normalizeServices(row.service_mode === 'mixed' ? target.services : row.services),
          subscribers,
          enabled: row.enabled,
        })
      }
    }
    return {
      enabled: state.settings.enabled !== false,
      subscriptions,
    }
  }

  function saveSettings() {
    const validation = validateRows()
    if (!validation.ok) {
      setStatus(validation.errors[0] || '设置未通过校验')
      render()
      return
    }
    const identityRequests = buildIdentityRequests()
    if (!identityRequests.length) {
      const payload = buildSettingsPayload([])
      state.savingRequestId = nextRequestId('settings-save')
      postMessage('settings.save', { values: payload }, state.savingRequestId)
      render()
      return
    }
    const requestId = nextRequestId('protocol-identities')
    state.pending.set(requestId, { kind: 'save-identities', expected: identityRequests })
    state.savingRequestId = requestId
    setStatus('正在刷新订阅人身份…')
    postMessage('protocol.identities.resolve', { items: identityRequests }, requestId)
    render()
  }

  function applyIdentitiesResolved(message) {
    const request = state.pending.get(message.request_id)
    if (!request || request.kind !== 'save-identities') {
      return
    }
    state.pending.delete(message.request_id)
    state.savingRequestId = ''
    const issues = Array.isArray(message.payload.issues) ? message.payload.issues : []
    const items = Array.isArray(message.payload.items) ? message.payload.items : []
    for (const item of items) {
      if (item.user_id && item.avatar_url) {
        state.subscriberAvatars.set(trim(item.user_id), trim(item.avatar_url))
      }
    }
    const received = new Set(items.map((item) => identityKey(item.target_type, item.target_id, item.user_id)))
    const missing = request.expected.filter((item) => !received.has(identityKey(item.target_type, item.target_id, item.user_id)))
    if (issues.length || missing.length) {
      setStatus(issues[0] && issues[0].message ? issues[0].message : '订阅人身份刷新失败')
      render()
      return
    }
    const payload = buildSettingsPayload(items)
    state.savingRequestId = nextRequestId('settings-save')
    postMessage('settings.save', { values: payload }, state.savingRequestId)
    render()
  }

  function applySettingsChanged(message) {
    state.settings = normalizeSettings(message.payload && message.payload.values)
    state.rows = buildRowsFromSettings(state.settings)
    state.loaded = true
    state.dirty = false
    state.savingRequestId = ''
    setStatus('设置已同步')
    render()
  }

  function applyHostInit(payload) {
    state.defaultSettings = normalizeSettings(payload.default_config)
    state.settings = normalizeSettings(payload.settings)
    state.rows = buildRowsFromSettings(state.settings)
    state.loaded = true
    state.dirty = false
    setStatus('设置已载入')
    render()
    requestTargets()
  }

  function applyTargetsChanged(payload) {
    state.targets = {
      loaded: true,
      available: payload.available === true,
      groups: Array.isArray(payload.groups) ? payload.groups : [],
      private_users: Array.isArray(payload.private_users) ? payload.private_users : [],
      issues: Array.isArray(payload.issues) ? payload.issues : [],
    }
    setStatus(state.targets.available ? '推送对象已刷新' : '推送对象不可用')
    render()
  }

  function handleBridgeMessage(event) {
    const message = event.data || {}
    if (message.version !== '1' || message.source !== 'management_host') {
      return
    }
    switch (message.type) {
      case 'host.init':
        applyHostInit(message.payload || {})
        return
      case 'settings.changed':
        applySettingsChanged(message)
        return
      case 'protocol.targets.changed':
        applyTargetsChanged(message.payload || {})
        return
      case 'protocol.identities.resolved':
        applyIdentitiesResolved(message)
        return
      case 'bilibili.user.resolved':
        applyBilibiliResolved(message)
        return
      case 'error':
        state.savingRequestId = ''
        setStatus((message.payload && message.payload.message) || '操作失败')
        render()
        return
    }
  }

  function handleListClick(event) {
    const button = event.target.closest('button[data-action]')
    if (!button) {
      return
    }
    const row = findRow(button.dataset.rowId)
    if (!row) {
      return
    }
    const action = button.dataset.action
    if (action === 'resolve-up') {
      requestBilibiliResolve(row, true)
      return
    }
    if (action === 'choose-candidate') {
      try {
        applyResolvedUser(row, JSON.parse(button.dataset.user || '{}'))
        row.resolve_message = 'UP 已校验。'
        markDirty()
      } catch {
        setStatus('候选 UP 数据不正确')
      }
      return
    }
    if (action === 'target-mode') {
      row.target_mode = button.dataset.mode === 'private' ? 'private' : 'group'
      render()
      return
    }
    if (action === 'toggle-target') {
      toggleTarget(row, button.dataset.targetKey)
      state.dirty = true
      refreshRowTargetEditor(row)
      return
    }
    if (action === 'remove-target') {
      row.targets = row.targets.filter((target) => target.key !== button.dataset.targetKey)
      markDirty()
      return
    }
    if (action === 'add-subscriber') {
      const input = elements.list.querySelector(`.subscriber-input[data-row-id="${selectorValue(row.row_id)}"]`)
      addSubscriber(row, input)
      return
    }
    if (action === 'remove-subscriber') {
      row.subscriber_ids = row.subscriber_ids.filter((id) => id !== button.dataset.userId)
      markDirty()
      return
    }
    if (action === 'duplicate-row') {
      const copy = cloneRow(row)
      copy.row_id = nextRowId()
      copy.targets = copy.targets.map((target) => ({ ...target, subscription_id: '' }))
      copy.edit_mode = true
      copy._editSnapshot = null
      state.rows.push(copy)
      markDirty()
      return
    }
    if (action === 'delete-row') {
      state.rows = state.rows.filter((item) => item.row_id !== row.row_id)
      markDirty()
      return
    }
    if (action === 'edit-row') {
      beginEdit(row)
      return
    }
    if (action === 'finish-edit') {
      finishEdit(row)
      return
    }
    if (action === 'cancel-edit') {
      cancelEdit(row)
    }
  }

  function handleListInput(event) {
    const input = event.target
    if (input.classList.contains('up-query-input')) {
      const row = findRow(input.dataset.rowId)
      if (!row) {
        return
      }
      row.query = input.value
      row.resolved = false
      row.resolve_state = 'idle'
      row.resolve_message = ''
      row.candidates = []
      state.dirty = true
      requestBilibiliResolve(row, false)
    }
  }

  function handleListChange(event) {
    const input = event.target
    const row = findRow(input.dataset.rowId)
    if (!row) {
      return
    }
    if (input.classList.contains('service-checkbox')) {
      updateService(row, input.dataset.targetKey, input.value, input.checked)
      markDirty()
      return
    }
    if (input.classList.contains('row-enabled-input')) {
      row.enabled = input.checked
      markDirty()
    }
  }

  function resetToDefault() {
    state.settings = normalizeSettings(state.defaultSettings)
    state.rows = buildRowsFromSettings(state.settings)
    state.dirty = true
    setStatus('已恢复默认设置，保存后生效')
    render()
  }

  function bindEvents() {
    window.addEventListener('message', handleBridgeMessage)
    elements.enabledInput.addEventListener('change', () => {
      state.settings.enabled = elements.enabledInput.checked
      markDirty()
    })
    elements.targetsReloadButton.addEventListener('click', requestTargets)
    elements.searchInput.addEventListener('input', render)
    elements.statusFilter.addEventListener('change', render)
    elements.serviceFilter.addEventListener('change', render)
    elements.addButton.addEventListener('click', () => {
      const newRow = createBlankRow()
      state.rows.unshift(newRow)
      markDirty()
      scrollRowIntoCenter(newRow)
    })
    elements.list.addEventListener('click', handleListClick)
    elements.list.addEventListener('input', handleListInput)
    elements.list.addEventListener('change', handleListChange)
    elements.reloadButton.addEventListener('click', () => {
      setStatus('正在重新载入设置…')
      postMessage('settings.reload', undefined, nextRequestId('settings-reload'))
    })
    elements.resetButton.addEventListener('click', resetToDefault)
    elements.manualCheckButton.addEventListener('click', () => {
      setStatus('Bilibili 事件源状态在 Web 三方监控页面查看')
    })
    elements.previewButton.addEventListener('click', () => {
      postMessage('render_template.open', { template_id: 'plugin.raylea.subscription-hub.bilibili-update' }, nextRequestId('open-template'))
    })
    elements.saveButton.addEventListener('click', saveSettings)
    elements.list.addEventListener('keydown', (event) => {
      if (event.key !== 'Enter' || !event.target.classList.contains('subscriber-input')) {
        return
      }
      event.preventDefault()
      const row = findRow(event.target.dataset.rowId)
      if (row) {
        addSubscriber(row, event.target)
      }
    })
  }

  bindEvents()
  render()
  postMessage('page.ready', undefined, nextRequestId('page-ready'))

  window.__subscriptionHubSettingsPage = {
    state,
    normalizeSettings,
    buildRowsFromSettings,
    buildSettingsPayload,
    validateRows,
  }
})()
