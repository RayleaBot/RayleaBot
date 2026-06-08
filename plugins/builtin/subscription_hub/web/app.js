(function () {
  const SERVICE_OPTIONS = [
    { value: 'all', label: '全部' },
    { value: 'live', label: '直播' },
    { value: 'video', label: '视频' },
    { value: 'image_text', label: '图文' },
    { value: 'article', label: '文章' },
    { value: 'repost', label: '转发' },
  ]
  const TARGET_OPTIONS = [
    { value: 'group', label: '群聊' },
    { value: 'private', label: '私聊' },
  ]
  const DEFAULT_SETTINGS = {
    enabled: true,
    subscriptions: [],
  }

  const elements = {
    statusText: document.getElementById('status-text'),
    pageTitle: document.getElementById('page-title'),
    pageSubtitle: document.getElementById('page-subtitle'),
    metricEnabled: document.getElementById('metric-enabled'),
    metricSubscriptions: document.getElementById('metric-subscriptions'),
    metricSource: document.getElementById('metric-source'),
    metricValidation: document.getElementById('metric-validation'),
    enabledInput: document.getElementById('enabled-input'),
    addSubscriptionButton: document.getElementById('add-subscription-button'),
    subscriptionSearchInput: document.getElementById('subscription-search-input'),
    statusFilterInput: document.getElementById('status-filter-input'),
    serviceFilterInput: document.getElementById('service-filter-input'),
    subscriptionList: document.getElementById('subscription-list'),
    subscriptionEditorPanel: document.getElementById('subscription-editor-panel'),
    subscriptionEditorTitle: document.getElementById('subscription-editor-title'),
    subscriptionEditorSubtitle: document.getElementById('subscription-editor-subtitle'),
    closeEditorButton: document.getElementById('close-editor-button'),
    subscriptionEditor: document.getElementById('subscription-editor'),
    exportJsonButton: document.getElementById('export-json-button'),
    importJsonButton: document.getElementById('import-json-button'),
    rawJsonInput: document.getElementById('raw-json-input'),
    rawJsonError: document.getElementById('raw-json-error'),
    dirtyState: document.getElementById('dirty-state'),
    reloadButton: document.getElementById('reload-button'),
    resetButton: document.getElementById('reset-button'),
    manualCheckButton: document.getElementById('manual-check-button'),
    previewButton: document.getElementById('preview-button'),
    saveButton: document.getElementById('save-button'),
  }

  let defaultSettings = normalizeSettings(DEFAULT_SETTINGS)
  let draft = normalizeSettings(DEFAULT_SETTINGS)
  let savedSnapshot = ''
  let validation = { errors: [] }
  let selectedSubscriptionId = ''
  let readyTimer = null
  let readyAttempts = 0
  let initialized = false
  let pendingSave = false

  function postMessage(type, payload, requestId) {
    window.parent.postMessage({
      version: '1',
      source: 'plugin_management_ui',
      type,
      request_id: requestId,
      payload,
    }, '*')
  }

  function stopReadyLoop() {
    if (readyTimer) {
      clearTimeout(readyTimer)
      readyTimer = null
    }
  }

  function announceReady() {
    stopReadyLoop()
    readyAttempts += 1
    postMessage('page.ready', undefined, `ready-${Date.now()}-${readyAttempts}`)
    if (readyAttempts < 10) {
      readyTimer = setTimeout(announceReady, 500)
    }
  }

  function setStatus(message, isError) {
    if (!elements.statusText) {
      return
    }
    elements.statusText.textContent = message
    elements.statusText.classList.toggle('is-error', Boolean(isError))
  }

  function isRecord(value) {
    return value && typeof value === 'object' && !Array.isArray(value)
  }

  function safeId(value, fallback) {
    const text = String(value || '').trim().toLowerCase()
    const normalized = Array.from(text)
      .filter((char) => /[a-z0-9_.-]/.test(char))
      .join('')
      .replace(/^[._-]+|[._-]+$/g, '')
      .slice(0, 96)
    return normalized || fallback
  }

  function normalizeServices(value) {
    const source = Array.isArray(value) ? value : ['all']
    const seen = new Set()
    const result = []
    for (const item of source) {
      const service = String(item || '').trim()
      if (!SERVICE_OPTIONS.some((option) => option.value === service) || seen.has(service)) {
        continue
      }
      seen.add(service)
      result.push(service)
    }
    return result.length > 0 ? result : ['all']
  }

  function normalizeSubscribers(value) {
    const source = Array.isArray(value) ? value : []
    return source
      .map((item) => {
        if (isRecord(item)) {
          const id = String(item.id || '').trim()
          const nickname = String(item.nickname || id).trim()
          if (!id) {
            return null
          }
          const subscriber = { id, nickname: nickname || id }
          for (const key of ['group_nickname', 'title', 'role', 'role_label', 'avatar_url']) {
            const text = String(item[key] || '').trim()
            if (text) {
              subscriber[key] = text
            }
          }
          return subscriber
        }
        const text = String(item || '').trim()
        return text ? { id: text, nickname: text } : null
      })
      .filter(Boolean)
  }

  function normalizeSubscriptions(value) {
    const source = Array.isArray(value) ? value : []
    const seen = new Set()
    return source
      .map((item, index) => {
        const sourceItem = isRecord(item) ? item : {}
        const uid = String(sourceItem.uid || '').trim()
        const targetType = String(sourceItem.target_type || '').trim()
        const targetId = String(sourceItem.target_id || '').trim()
        const fallbackId = uid && targetType && targetId
          ? `bilibili-${uid}-${targetType}-${targetId}`
          : `bilibili-draft-${index + 1}`
        const id = safeId(sourceItem.id, fallbackId)
        if (seen.has(id)) {
          return null
        }
        seen.add(id)
        return {
          id,
          platform: 'bilibili',
          uid,
          name: String(sourceItem.name || uid).trim(),
          avatar_url: String(sourceItem.avatar_url || '').trim(),
          target_type: ['group', 'private'].includes(targetType) ? targetType : 'group',
          target_id: targetId,
          target_name: String(sourceItem.target_name || '').trim(),
          services: normalizeServices(sourceItem.services),
          subscribers: normalizeSubscribers(sourceItem.subscribers),
          enabled: sourceItem.enabled !== false,
        }
      })
      .filter(Boolean)
  }

  function normalizeSettings(values) {
    const source = isRecord(values) ? values : {}
    return {
      enabled: source.enabled !== false,
      subscriptions: normalizeSubscriptions(source.subscriptions),
    }
  }

  function buildPayloadFromDraft(source) {
    return {
      settings: {
        enabled: source.enabled !== false,
        subscriptions: source.subscriptions.map(subscriptionPayload),
      },
    }
  }

  function subscriptionPayload(item) {
    const subscription = {
      id: safeId(item.id, `bilibili-${item.uid}-${item.target_type}-${item.target_id}`),
      platform: 'bilibili',
      uid: String(item.uid || '').trim(),
      name: String(item.name || item.uid || '').trim(),
      target_type: item.target_type === 'private' ? 'private' : 'group',
      target_id: String(item.target_id || '').trim(),
      services: normalizeServices(item.services),
      subscribers: normalizeSubscribers(item.subscribers),
      enabled: item.enabled !== false,
    }
    const avatarUrl = String(item.avatar_url || '').trim()
    if (avatarUrl) {
      subscription.avatar_url = avatarUrl
    }
    const targetName = String(item.target_name || '').trim()
    if (targetName) {
      subscription.target_name = targetName
    }
    return subscription
  }

  function snapshotFromPayload(payload) {
    return JSON.stringify(payload.settings)
  }

  function validateDraft() {
    const errors = []
    const subscriptionIds = new Set()
    draft.subscriptions.forEach((item, index) => {
      const label = item.name || item.uid || `订阅 ${index + 1}`
      if (!item.id) {
        errors.push({ scope: `subscription-${index}-id`, message: `${label} 的 ID 不能为空` })
      } else if (subscriptionIds.has(item.id)) {
        errors.push({ scope: `subscription-${index}-id`, message: `${label} 的 ID 重复` })
      }
      subscriptionIds.add(item.id)
      if (!/^\d+$/.test(String(item.uid || '').trim())) {
        errors.push({ scope: `subscription-${index}-uid`, message: `${label} 的 UID 只能填写数字` })
      }
      if (!['group', 'private'].includes(item.target_type)) {
        errors.push({ scope: `subscription-${index}-target_type`, message: `${label} 的目标类型不正确` })
      }
      if (!String(item.target_id || '').trim()) {
        errors.push({ scope: `subscription-${index}-target_id`, message: `${label} 的目标 ID 不能为空` })
      }
      if (normalizeServices(item.services).length === 0) {
        errors.push({ scope: `subscription-${index}-services`, message: `${label} 至少需要一个推送类型` })
      }
    })
    return { errors }
  }

  function firstError(scopePrefix) {
    return validation.errors.find((error) => error.scope === scopePrefix || error.scope.startsWith(`${scopePrefix}-`))
  }

  function isDirty() {
    return snapshotFromPayload(buildPayloadFromDraft(draft)) !== savedSnapshot
  }

  function applySettings(values, options) {
    draft = normalizeSettings(values || defaultSettings)
    if (options && options.markSaved) {
      savedSnapshot = snapshotFromPayload(buildPayloadFromDraft(draft))
    }
    if (!draft.subscriptions.some((item) => item.id === selectedSubscriptionId)) {
      selectedSubscriptionId = draft.subscriptions[0] ? draft.subscriptions[0].id : ''
    }
    render()
  }

  function render() {
    validation = validateDraft()
    renderOverview()
    renderControls()
    renderSubscriptions()
    renderSubscriptionEditor()
    renderRawJson()
    renderFooter()
  }

  function renderControls() {
    if (elements.enabledInput) {
      elements.enabledInput.checked = draft.enabled
    }
  }

  function renderOverview() {
    const enabledSubscriptions = draft.subscriptions.filter((item) => item.enabled !== false).length
    if (elements.metricEnabled) {
      elements.metricEnabled.textContent = draft.enabled ? '启用' : '停用'
    }
    if (elements.metricSubscriptions) {
      elements.metricSubscriptions.textContent = `${enabledSubscriptions} / ${draft.subscriptions.length}`
    }
    if (elements.metricSource) {
      elements.metricSource.textContent = '平台事件源'
    }
    if (elements.metricValidation) {
      elements.metricValidation.textContent = validation.errors.length === 0 ? '可保存' : `${validation.errors.length} 个问题`
      elements.metricValidation.classList.toggle('is-error', validation.errors.length > 0)
    }
  }

  function renderSubscriptions() {
    if (!elements.subscriptionList) {
      return
    }
    elements.subscriptionList.innerHTML = ''
    const filtered = getFilteredSubscriptions()
    if (draft.subscriptions.length === 0) {
      elements.subscriptionList.appendChild(emptyState('还没有订阅', '添加 UP 主 UID、目标和推送类型后开始管理订阅。', '添加订阅', addSubscription))
      return
    }
    if (filtered.length === 0) {
      elements.subscriptionList.appendChild(emptyState('没有符合条件的订阅', '清除搜索和筛选后查看全部订阅。', '清除筛选', clearFilters))
      return
    }

    filtered.forEach(({ item, index }) => {
      const card = document.createElement('article')
      card.className = `subscription-card${item.id === selectedSubscriptionId ? ' is-selected' : ''}${item.enabled === false ? ' is-muted' : ''}`

      const title = document.createElement('button')
      title.type = 'button'
      title.className = 'subscription-card__main'
      title.addEventListener('click', () => selectSubscription(item.id))
      title.appendChild(subscriptionAvatar(item))
      const info = document.createElement('span')
      info.className = 'subscription-card__info'
      const name = document.createElement('span')
      name.className = 'subscription-card__title'
      name.textContent = item.name || `Bilibili ${item.uid}`
      const meta = document.createElement('span')
      meta.className = 'subscription-card__meta'
      meta.textContent = `${item.uid ? `UID ${item.uid}` : '未填写 UID'} · ${sourceLabel(item)}`
      info.appendChild(name)
      info.appendChild(meta)
      title.appendChild(info)
      card.appendChild(title)

      const chips = document.createElement('div')
      chips.className = 'chip-list chip-list--plain'
      normalizeServices(item.services).forEach((service) => {
        const chip = document.createElement('span')
        chip.className = 'chip'
        chip.textContent = serviceLabel(service)
        chips.appendChild(chip)
      })
      card.appendChild(chips)

      const subscriberRow = document.createElement('div')
      subscriberRow.className = 'subscription-subscribers'
      subscriberRow.innerHTML = `
        <span class="subscription-subscribers__label">订阅人</span>
        <span class="subscription-subscribers__names">${escapeHtml(subscriberNames(item.subscribers) || '未记录')}</span>
      `
      card.appendChild(subscriberRow)

      const actions = document.createElement('div')
      actions.className = 'row-actions'
      actions.appendChild(smallButton(item.enabled ? '停用' : '启用', () => {
        item.enabled = !item.enabled
        markChanged()
      }))
      actions.appendChild(smallButton('复制', () => duplicateSubscription(index)))
      actions.appendChild(smallButton('删除', () => removeSubscription(index), 'button--danger'))
      card.appendChild(actions)

      const error = firstError(`subscription-${index}`)
      if (error) {
        card.appendChild(errorNode(error.message))
      }
      elements.subscriptionList.appendChild(card)
    })
  }

  function renderSubscriptionEditor() {
    if (!elements.subscriptionEditor) {
      return
    }
    elements.subscriptionEditor.innerHTML = ''
    const index = draft.subscriptions.findIndex((item) => item.id === selectedSubscriptionId)
    if (index < 0) {
      elements.subscriptionEditorPanel?.classList.add('is-collapsed')
      if (elements.subscriptionEditorTitle) elements.subscriptionEditorTitle.textContent = '订阅编辑'
      if (elements.subscriptionEditorSubtitle) elements.subscriptionEditorSubtitle.textContent = '选择一条订阅，或新建订阅。'
      return
    }

    elements.subscriptionEditorPanel?.classList.remove('is-collapsed')
    const item = draft.subscriptions[index]
    elements.subscriptionEditorTitle.textContent = item.name || `Bilibili ${item.uid || ''}`.trim() || '新订阅'
    elements.subscriptionEditorSubtitle.textContent = sourceLabel(item)

    const grid = document.createElement('div')
    grid.className = 'editor-grid'
    grid.appendChild(fieldInput('subscription-enabled-platform', '平台', 'bilibili', () => undefined, { disabled: true }))
    grid.appendChild(fieldInput('subscription-id', 'ID', item.id, (value) => {
      item.id = safeId(value, '')
      selectedSubscriptionId = item.id
      markChanged()
    }, { spellcheck: false }))
    grid.appendChild(fieldInput('subscription-uid', 'UP 主 UID', item.uid, (value) => {
      item.uid = value.trim()
      if (!item.name) {
        item.name = item.uid
      }
      markChanged(false)
    }, { inputmode: 'numeric', spellcheck: false }))
    grid.appendChild(fieldInput('subscription-name', 'Bilibili 用户名', item.name, (value) => {
      item.name = value
      markChanged(false)
    }))
    grid.appendChild(fieldInput('subscription-avatar-url', 'UP 主头像 URL', item.avatar_url, (value) => {
      item.avatar_url = value.trim()
      markChanged(false)
    }, { spellcheck: false }))
    grid.appendChild(selectField('subscription-target-type', '目标类型', TARGET_OPTIONS, item.target_type, (value) => {
      item.target_type = value
      markChanged()
    }))
    grid.appendChild(fieldInput('subscription-target-id', '目标 ID', item.target_id, (value) => {
      item.target_id = value.trim()
      markChanged(false)
    }, { spellcheck: false }))
    grid.appendChild(fieldInput('subscription-target-name', '目标名称', item.target_name, (value) => {
      item.target_name = value.trim()
      markChanged(false)
    }))
    elements.subscriptionEditor.appendChild(grid)

    const serviceWrap = document.createElement('fieldset')
    serviceWrap.className = 'service-fieldset'
    const legend = document.createElement('legend')
    legend.textContent = '推送类型'
    serviceWrap.appendChild(legend)
    SERVICE_OPTIONS.forEach((service) => {
      const label = document.createElement('label')
      label.className = 'check-chip'
      const input = document.createElement('input')
      input.type = 'checkbox'
      input.name = `service_${service.value}`
      input.checked = item.services.includes(service.value)
      input.addEventListener('change', () => {
        updateServiceSelection(item, service.value, input.checked)
        markChanged()
      })
      label.appendChild(input)
      label.appendChild(document.createTextNode(service.label))
      serviceWrap.appendChild(label)
    })
    elements.subscriptionEditor.appendChild(serviceWrap)

    const subscriberSection = document.createElement('div')
    subscriberSection.className = 'subscriber-section'
    subscriberSection.appendChild(rowHeader('订阅人', `${item.subscribers.length} 个`, [
      smallButton('添加订阅人', () => {
        item.subscribers.push({ id: '', nickname: '' })
        markChanged()
      }),
    ]))
    if (item.subscribers.length === 0) {
      subscriberSection.appendChild(document.createElement('p')).textContent = '未记录订阅人。'
    } else {
      item.subscribers.forEach((subscriber, subscriberIndex) => {
        const row = document.createElement('div')
        row.className = 'subscriber-row'
        row.appendChild(fieldInput(`subscriber-id-${subscriberIndex}`, 'ID', subscriber.id, (value) => {
          subscriber.id = value.trim()
          markChanged(false)
        }, { spellcheck: false }))
        row.appendChild(fieldInput(`subscriber-nickname-${subscriberIndex}`, '昵称', subscriber.nickname, (value) => {
          subscriber.nickname = value
          markChanged(false)
        }))
        row.appendChild(smallButton('删除', () => {
          item.subscribers.splice(subscriberIndex, 1)
          markChanged()
        }, 'button--danger'))
        subscriberSection.appendChild(row)
      })
    }
    elements.subscriptionEditor.appendChild(subscriberSection)

    const toggle = labelWrap('subscription-enabled', 'subscription-enabled')
    toggle.className = 'toggle-row toggle-row--compact'
    const enabled = document.createElement('input')
    enabled.id = 'subscription-enabled'
    enabled.type = 'checkbox'
    enabled.name = 'subscription_enabled'
    enabled.autocomplete = 'off'
    enabled.checked = item.enabled !== false
    enabled.addEventListener('change', () => {
      item.enabled = enabled.checked
      markChanged()
    })
    toggle.appendChild(enabled)
    toggle.appendChild(textBlock('启用这条订阅', '停用后保留配置，不会推送。'))
    elements.subscriptionEditor.appendChild(toggle)

    const error = firstError(`subscription-${index}`)
    if (error) {
      elements.subscriptionEditor.appendChild(errorNode(error.message))
    }
  }

  function renderRawJson() {
    if (!elements.rawJsonInput) {
      return
    }
    const text = JSON.stringify(buildPayloadFromDraft(draft).settings, null, 2)
    if (document.activeElement !== elements.rawJsonInput && elements.rawJsonInput.value !== text) {
      elements.rawJsonInput.value = text
    }
  }

  function renderFooter() {
    const dirty = isDirty()
    const hasErrors = validation.errors.length > 0
    if (!elements.dirtyState || !elements.saveButton) {
      return
    }
    elements.dirtyState.textContent = !initialized ? '等待载入' : pendingSave ? '正在保存…' : hasErrors ? `存在 ${validation.errors.length} 个问题` : dirty ? '有未保存更改' : '设置已同步'
    elements.dirtyState.classList.toggle('is-error', hasErrors)
    elements.dirtyState.classList.toggle('is-dirty', initialized && !pendingSave && dirty && !hasErrors)
    elements.dirtyState.classList.toggle('is-synced', initialized && !pendingSave && !dirty && !hasErrors)
    elements.saveButton.disabled = !initialized || pendingSave || hasErrors || !dirty
  }

  function markChanged(fullRender) {
    if (fullRender === false) {
      validation = validateDraft()
      renderOverview()
      renderFooter()
      renderRawJson()
      return
    }
    render()
  }

  function fieldInput(id, labelText, value, onInput, options) {
    const label = labelWrap(labelText, id)
    label.className = 'field'
    const span = document.createElement('span')
    span.textContent = labelText
    const input = document.createElement('input')
    input.id = id
    input.name = id.replace(/-/g, '_')
    input.type = options && options.type ? options.type : 'text'
    input.autocomplete = 'off'
    input.value = String(value || '')
    input.disabled = Boolean(options && options.disabled)
    if (options && options.inputmode) {
      input.inputMode = options.inputmode
    }
    if (options && options.placeholder) {
      input.placeholder = options.placeholder
    }
    if (options && options.spellcheck === false) {
      input.spellcheck = false
    }
    input.addEventListener('input', () => onInput(input.value))
    label.appendChild(span)
    label.appendChild(input)
    return label
  }

  function selectField(id, labelText, options, value, onChange) {
    const label = labelWrap(labelText, id)
    label.className = 'field'
    const span = document.createElement('span')
    span.textContent = labelText
    const select = document.createElement('select')
    select.id = id
    select.name = id.replace(/-/g, '_')
    select.autocomplete = 'off'
    options.forEach((item) => {
      const option = document.createElement('option')
      option.value = item.value
      option.textContent = item.label
      select.appendChild(option)
    })
    select.value = value
    select.addEventListener('change', () => onChange(select.value))
    label.appendChild(span)
    label.appendChild(select)
    return label
  }

  function labelWrap(text, id) {
    const label = document.createElement('label')
    label.setAttribute('for', id)
    label.setAttribute('aria-label', text)
    return label
  }

  function textBlock(title, description) {
    const span = document.createElement('span')
    const strong = document.createElement('strong')
    strong.textContent = title
    const small = document.createElement('small')
    small.textContent = description
    span.appendChild(strong)
    span.appendChild(small)
    return span
  }

  function rowHeader(title, badge, buttons) {
    const header = document.createElement('div')
    header.className = 'row-header'
    const text = document.createElement('div')
    const strong = document.createElement('strong')
    strong.textContent = title
    const small = document.createElement('small')
    small.textContent = badge
    text.appendChild(strong)
    text.appendChild(small)
    const actions = document.createElement('div')
    actions.className = 'row-actions'
    buttons.forEach((button) => actions.appendChild(button))
    header.appendChild(text)
    header.appendChild(actions)
    return header
  }

  function smallButton(label, onClick, extraClass) {
    const button = document.createElement('button')
    button.type = 'button'
    button.className = `button button--small${extraClass ? ` ${extraClass}` : ''}`
    button.textContent = label
    button.addEventListener('click', onClick)
    return button
  }

  function emptyState(title, description, actionLabel, action) {
    const empty = document.createElement('div')
    empty.className = 'empty-state'
    const strong = document.createElement('strong')
    strong.textContent = title
    const paragraph = document.createElement('p')
    paragraph.textContent = description
    const button = smallButton(actionLabel, action, 'button--primary-accent')
    empty.appendChild(strong)
    empty.appendChild(paragraph)
    empty.appendChild(button)
    return empty
  }

  function errorNode(message) {
    const node = document.createElement('small')
    node.className = 'field-error'
    node.textContent = message
    return node
  }

  function escapeHtml(value) {
    return String(value || '').replace(/[&<>"']/g, (char) => ({
      '&': '&amp;',
      '<': '&lt;',
      '>': '&gt;',
      '"': '&quot;',
      "'": '&#039;',
    })[char])
  }

  function serviceLabel(value) {
    return (SERVICE_OPTIONS.find((item) => item.value === value) || { label: value }).label
  }

  function targetLabel(value) {
    return (TARGET_OPTIONS.find((item) => item.value === value) || { label: value }).label
  }

  function sourceLabel(item) {
    const label = targetLabel(item.target_type)
    const name = String(item.target_name || '').trim()
    const id = String(item.target_id || '').trim()
    if (name && id) {
      return `${label} ${name} ${id}`
    }
    return id ? `${label} ${id}` : `${label} 未填写目标`
  }

  function subscriberDisplayName(item) {
    const id = String(item.id || '').trim()
    const name = String(item.group_nickname || item.nickname || id).trim()
    if (name && id && name !== id) {
      return `${name}（${id}）`
    }
    return name || id
  }

  function subscriberNames(value) {
    return normalizeSubscribers(value)
      .map(subscriberDisplayName)
      .filter(Boolean)
      .join('、')
  }

  function subscriptionAvatar(item) {
    const avatar = document.createElement('span')
    avatar.className = 'subscription-avatar'
    const firstChar = (item.name || item.uid || 'B').trim().charAt(0).toUpperCase()
    const fallback = document.createElement('span')
    fallback.className = 'subscription-avatar__fallback'
    fallback.textContent = firstChar || 'B'
    const avatarUrl = String(item.avatar_url || '').trim()
    if (avatarUrl) {
      const image = document.createElement('img')
      image.src = avatarUrl
      image.alt = `${item.name || item.uid || 'Bilibili'} 头像`
      image.loading = 'lazy'
      image.referrerPolicy = 'no-referrer'
      image.addEventListener('error', () => {
        image.remove()
        avatar.classList.add('is-fallback')
      }, { once: true })
      avatar.appendChild(image)
    } else {
      avatar.classList.add('is-fallback')
    }
    avatar.appendChild(fallback)
    return avatar
  }

  function getFilteredSubscriptions() {
    const query = elements.subscriptionSearchInput?.value.trim().toLowerCase() || ''
    const status = elements.statusFilterInput?.value || 'all'
    const service = elements.serviceFilterInput?.value || 'all'
    return draft.subscriptions
      .map((item, index) => ({ item, index }))
      .filter(({ item }) => {
        const searchText = `${item.uid} ${item.name} ${item.target_id} ${item.target_name} ${subscriberNames(item.subscribers)}`.toLowerCase()
        const statusMatches = status === 'all' || (status === 'enabled' ? item.enabled !== false : item.enabled === false)
        const services = normalizeServices(item.services)
        const serviceMatches = service === 'all' || services.includes(service) || services.includes('all')
        return (!query || searchText.includes(query)) && statusMatches && serviceMatches
      })
  }

  function clearFilters() {
    if (elements.subscriptionSearchInput) {
      elements.subscriptionSearchInput.value = ''
    }
    if (elements.statusFilterInput) {
      elements.statusFilterInput.value = 'all'
    }
    if (elements.serviceFilterInput) {
      elements.serviceFilterInput.value = 'all'
    }
    renderSubscriptions()
  }

  function addSubscription() {
    const next = draft.subscriptions.length + 1
    const item = {
      id: `bilibili-new-${next}`,
      platform: 'bilibili',
      uid: '',
      name: '',
      avatar_url: '',
      target_type: 'group',
      target_id: '',
      target_name: '',
      services: ['all'],
      subscribers: [],
      enabled: true,
    }
    draft.subscriptions.unshift(item)
    selectedSubscriptionId = item.id
    markChanged()
  }

  function selectSubscription(id) {
    selectedSubscriptionId = id
    renderSubscriptions()
    renderSubscriptionEditor()
  }

  function duplicateSubscription(index) {
    const source = draft.subscriptions[index]
    const copy = {
      ...source,
      id: `${source.id || 'bilibili-copy'}-copy-${Date.now().toString(36)}`,
      name: source.name ? `${source.name} 副本` : source.name,
      services: [...source.services],
      subscribers: source.subscribers.map((item) => ({ ...item })),
    }
    draft.subscriptions.splice(index + 1, 0, copy)
    selectedSubscriptionId = copy.id
    markChanged()
  }

  function removeSubscription(index) {
    const item = draft.subscriptions[index]
    if (!window.confirm(`删除 ${item.name || item.uid || item.id}？`)) {
      return
    }
    draft.subscriptions.splice(index, 1)
    if (selectedSubscriptionId === item.id) {
      selectedSubscriptionId = draft.subscriptions[0] ? draft.subscriptions[0].id : ''
    }
    markChanged()
  }

  function updateServiceSelection(item, value, checked) {
    const current = new Set(normalizeServices(item.services))
    if (checked) {
      if (value === 'all') {
        item.services = ['all']
        return
      }
      current.delete('all')
      current.add(value)
    } else {
      current.delete(value)
    }
    item.services = Array.from(current)
    if (item.services.length === 0) {
      item.services = ['all']
    }
  }

  function saveAll() {
    validation = validateDraft()
    if (validation.errors.length > 0) {
      render()
      setStatus(validation.errors[0].message, true)
      focusFirstError()
      return
    }

    const payload = buildPayloadFromDraft(draft)
    pendingSave = true
    setStatus('正在保存设置…')
    postMessage('settings.save', { values: payload.settings }, `save-settings-${Date.now()}`)
  }

  function focusFirstError() {
    const first = validation.errors[0]
    if (!first) {
      return
    }
    if (first.scope.startsWith('subscription-')) {
      const [, index] = first.scope.split('-')
      const item = draft.subscriptions[Number(index)]
      if (item) {
        selectedSubscriptionId = item.id
        render()
        const target = document.getElementById('subscription-uid') || document.getElementById('subscription-target-id')
        if (target) {
          target.focus()
        }
      }
    }
  }

  function reloadAll() {
    setStatus('正在重新读取设置…')
    postMessage('settings.reload', undefined, `reload-settings-${Date.now()}`)
  }

  function resetSettings() {
    draft = normalizeSettings(defaultSettings)
    selectedSubscriptionId = draft.subscriptions[0] ? draft.subscriptions[0].id : ''
    setStatus('默认订阅设置已载入，保存后生效')
    render()
  }

  function showSourceEntry() {
    setStatus('Bilibili 事件源状态在 Web 三方账号页面查看')
  }

  function openCardPreview() {
    postMessage('render_template.open', { template_id: 'plugin.raylea.subscription-hub.bilibili-update' }, `open-template-${Date.now()}`)
  }

  function importRawJson() {
    if (!elements.rawJsonInput || !elements.rawJsonError) {
      return
    }
    try {
      const parsed = JSON.parse(elements.rawJsonInput.value || '{}')
      draft = normalizeSettings(parsed)
      selectedSubscriptionId = draft.subscriptions[0] ? draft.subscriptions[0].id : ''
      elements.rawJsonError.textContent = ''
      setStatus('JSON 已导入，保存后生效')
      render()
    } catch (error) {
      elements.rawJsonError.textContent = error && error.message ? error.message : 'JSON 格式不正确'
      setStatus('JSON 格式不正确', true)
    }
  }

  function bindEvents() {
    bind(elements.enabledInput, 'change', () => {
      draft.enabled = elements.enabledInput.checked
      markChanged()
    })
    bind(elements.addSubscriptionButton, 'click', addSubscription)
    bind(elements.subscriptionSearchInput, 'input', renderSubscriptions)
    bind(elements.statusFilterInput, 'change', renderSubscriptions)
    bind(elements.serviceFilterInput, 'change', renderSubscriptions)
    bind(elements.closeEditorButton, 'click', () => {
      selectedSubscriptionId = ''
      renderSubscriptions()
      renderSubscriptionEditor()
    })
    bind(elements.exportJsonButton, 'click', () => {
      renderRawJson()
      setStatus('原始配置预览已刷新')
    })
    bind(elements.importJsonButton, 'click', importRawJson)
    bind(elements.reloadButton, 'click', reloadAll)
    bind(elements.resetButton, 'click', resetSettings)
    bind(elements.manualCheckButton, 'click', showSourceEntry)
    bind(elements.previewButton, 'click', openCardPreview)
    bind(elements.saveButton, 'click', saveAll)
  }

  function bind(element, eventName, handler) {
    if (element) {
      element.addEventListener(eventName, handler)
    }
  }

  window.addEventListener('message', (event) => {
    const message = event.data
    if (!message || message.version !== '1' || typeof message.type !== 'string') {
      return
    }

    if (message.type === 'host.init') {
      stopReadyLoop()
      const payload = message.payload || {}
      if (elements.pageTitle) {
        elements.pageTitle.textContent = payload.title || '订阅设置'
      }
      if (elements.pageSubtitle) {
        elements.pageSubtitle.textContent = '管理 Bilibili 订阅和推送目标'
      }
      defaultSettings = normalizeSettings(payload.default_config || DEFAULT_SETTINGS)
      initialized = true
      applySettings(payload.settings || defaultSettings, { markSaved: true })
      setStatus('已载入设置')
      return
    }

    if (message.type === 'settings.changed') {
      const payload = message.payload || {}
      pendingSave = false
      applySettings(payload.values || defaultSettings, { markSaved: true })
      setStatus('设置已保存')
      return
    }

    if (message.type === 'error') {
      const payload = message.payload || {}
      pendingSave = false
      setStatus(payload.message || '操作未完成', true)
      renderFooter()
    }
  })

  bindEvents()
  render()
  announceReady()

  window.__subscriptionHubSettingsPage = {
    buildPayload: () => buildPayloadFromDraft(draft),
    validate: validateDraft,
    importRawJson,
    getFilteredSubscriptions: () => getFilteredSubscriptions().map(({ item, index }) => ({ item, index })),
    subscriberNames,
    getDraft: () => JSON.parse(JSON.stringify(draft)),
    readyAttempts: () => readyAttempts,
  }
})()
