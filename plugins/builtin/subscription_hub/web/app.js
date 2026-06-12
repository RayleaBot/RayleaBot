(function () {
  'use strict'

  const SERVICE_ORDER = ['all', 'live', 'video', 'image_text', 'article', 'repost']
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
    return SERVICE_ORDER.filter((service) => services.includes(service))
  }

  function servicesKey(services) {
    return normalizeServices(services).join(',')
  }

  function servicesText(services) {
    return normalizeServices(services).map((service) => SERVICE_LABELS[service] || service).join('、')
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
      })),
      ...state.targets.private_users.map((target) => ({
        key: targetKey('private', target.target_id),
        target_type: 'private',
        target_id: trim(target.target_id),
        label: trim(target.nickname) || trim(target.target_id),
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

  function render() {
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

    const visibleRows = state.rows.filter(rowVisible)
    elements.list.innerHTML = visibleRows.length
      ? visibleRows.map(renderRow).join('')
      : '<div class="empty-state">没有匹配的订阅。可添加订阅或调整筛选条件。</div>'
  }

  function renderRow(row) {
    const map = targetMap()
    const title = row.name || row.uid || '未校验 UP'
    const subtitle = row.uid ? `UID ${row.uid}` : '输入 UID 或 Bilibili 用户名后校验'
    const serviceEditor = row.service_mode === 'mixed'
      ? renderMixedServices(row, map)
      : `<div class="inline-checks" aria-label="推送类型">${renderServiceCheckboxes(row.row_id, 'common', row.services)}</div>`
    const candidates = row.candidates.length
      ? `<div class="candidate-list">${row.candidates.map((candidate) => `
          <button type="button" class="button button--small" data-action="choose-candidate" data-row-id="${escapeHTML(row.row_id)}" data-user='${escapeHTML(JSON.stringify(candidate))}'>
            <span>${escapeHTML(candidate.name)} · UID ${escapeHTML(candidate.uid)}</span>
          </button>
        `).join('')}</div>`
      : ''
    const targetOptions = currentTargetsForMode(row.target_mode).map((target) => `
      <option value="${escapeHTML(target.key)}" ${row.targets.some((item) => item.key === target.key) ? 'selected' : ''}>
        ${escapeHTML(target.label)} (${escapeHTML(target.target_id)})
      </option>
    `).join('')
    const selectedTargets = row.targets.length
      ? row.targets.map((target) => `
          <span class="chip ${map.has(target.key) ? '' : 'badge--warning'}">
            <span>${escapeHTML(targetDisplay(target, map))}</span>
            <button type="button" aria-label="移除推送对象" data-action="remove-target" data-row-id="${escapeHTML(row.row_id)}" data-target-key="${escapeHTML(target.key)}">×</button>
          </span>
        `).join('')
      : '<span class="chip">未选择推送对象</span>'
    const subscribers = row.subscriber_ids.length
      ? row.subscriber_ids.map((id) => `
          <span class="chip">
            <span>QQ ${escapeHTML(id)}</span>
            <button type="button" aria-label="移除订阅人" data-action="remove-subscriber" data-row-id="${escapeHTML(row.row_id)}" data-user-id="${escapeHTML(id)}">×</button>
          </span>
        `).join('')
      : '<span class="chip badge--success">系统订阅</span>'
    const validation = validateRow(row)
    const validationHTML = validation.length
      ? `<ul class="validation-list">${validation.map((item) => `<li>${escapeHTML(item)}</li>`).join('')}</ul>`
      : '<span class="badge badge--success">可保存</span>'

    return `
      <article class="subscription-row ${row.enabled ? '' : 'is-disabled'}" data-row-id="${escapeHTML(row.row_id)}">
        <section class="row-block">
          <div class="row-title">
            <strong>${escapeHTML(title)}</strong>
            <span class="badge">${row.resolved ? '已校验' : row.resolve_state === 'checking' ? '校验中' : '待校验'}</span>
          </div>
          <div class="row-subtitle">${escapeHTML(subtitle)}</div>
          <div class="up-input-line">
            <input class="up-query-input" data-row-id="${escapeHTML(row.row_id)}" type="text" autocomplete="off" value="${escapeHTML(row.query)}" placeholder="UID 或 Bilibili 用户名" />
            <button type="button" class="button button--small" data-action="resolve-up" data-row-id="${escapeHTML(row.row_id)}">校验</button>
          </div>
          ${row.resolve_message ? `<div class="row-note">${escapeHTML(row.resolve_message)}</div>` : ''}
          ${candidates}
          ${serviceEditor}
        </section>

        <section class="row-block">
          <div class="row-title"><strong>推送对象</strong><span class="badge">${escapeHTML(TARGET_LABELS[row.target_mode])}</span></div>
          <div class="mode-tabs" role="group" aria-label="推送对象类型">
            <button type="button" class="button button--small ${row.target_mode === 'group' ? 'is-active' : ''}" data-action="target-mode" data-row-id="${escapeHTML(row.row_id)}" data-mode="group">群聊</button>
            <button type="button" class="button button--small ${row.target_mode === 'private' ? 'is-active' : ''}" data-action="target-mode" data-row-id="${escapeHTML(row.row_id)}" data-mode="private">私聊</button>
          </div>
          <select class="target-select" data-row-id="${escapeHTML(row.row_id)}" multiple size="5" ${state.targets.loaded ? '' : 'disabled'}>
            ${targetOptions}
          </select>
          <div class="chip-list">${selectedTargets}</div>
          ${state.targets.issues.length ? `<div class="target-note">${escapeHTML(state.targets.issues.map((issue) => issue.message).join('；'))}</div>` : ''}
        </section>

        <section class="row-block">
          <div class="row-title"><strong>订阅人</strong></div>
          <div class="subscriber-line">
            <input class="subscriber-input" data-row-id="${escapeHTML(row.row_id)}" type="text" inputmode="numeric" autocomplete="off" placeholder="QQ 号，留空为系统订阅" />
            <button type="button" class="button button--small" data-action="add-subscriber" data-row-id="${escapeHTML(row.row_id)}">添加</button>
          </div>
          <div class="chip-list">${subscribers}</div>
          <div class="row-note">只保存 QQ 号，昵称和群名片保存时刷新。</div>
        </section>

        <section class="row-state">
          <label><input type="checkbox" class="row-enabled-input" data-row-id="${escapeHTML(row.row_id)}" ${row.enabled ? 'checked' : ''} /> 启用</label>
          ${validationHTML}
          <div class="row-actions">
            <button type="button" class="button button--small" data-action="duplicate-row" data-row-id="${escapeHTML(row.row_id)}">复制</button>
            <button type="button" class="button button--small button--danger" data-action="delete-row" data-row-id="${escapeHTML(row.row_id)}">删除</button>
          </div>
        </section>
      </article>
    `
  }

  function renderServiceCheckboxes(rowId, targetKeyValue, services) {
    const active = normalizeServices(services)
    return SERVICE_ORDER.map((service) => `
      <label>
        <input type="checkbox" class="service-checkbox" data-row-id="${escapeHTML(rowId)}" data-target-key="${escapeHTML(targetKeyValue)}" value="${escapeHTML(service)}" ${active.includes(service) ? 'checked' : ''} />
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

  function readCheckedServices(rowId, targetKeyValue) {
    const checked = [...elements.list.querySelectorAll(`.service-checkbox[data-row-id="${selectorValue(rowId)}"][data-target-key="${selectorValue(targetKeyValue)}"]:checked`)]
      .map((input) => input.value)
    return normalizeServices(checked)
  }

  function updateService(row, targetKeyValue) {
    if (targetKeyValue === 'common') {
      row.service_mode = 'common'
      row.services = readCheckedServices(row.row_id, 'common')
      for (const target of row.targets) {
        target.services = row.services
      }
      return
    }
    const target = row.targets.find((item) => item.key === targetKeyValue)
    if (target) {
      target.services = readCheckedServices(row.row_id, targetKeyValue)
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
      const copy = JSON.parse(JSON.stringify(row))
      copy.row_id = nextRowId()
      copy.targets = copy.targets.map((target) => ({ ...target, subscription_id: '' }))
      state.rows.push(copy)
      markDirty()
      return
    }
    if (action === 'delete-row') {
      state.rows = state.rows.filter((item) => item.row_id !== row.row_id)
      markDirty()
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
      updateService(row, input.dataset.targetKey)
      markDirty()
      return
    }
    if (input.classList.contains('target-select')) {
      updateTargetsFromSelect(row, [...input.selectedOptions].map((option) => option.value))
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
      state.rows.unshift(createBlankRow())
      markDirty()
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
