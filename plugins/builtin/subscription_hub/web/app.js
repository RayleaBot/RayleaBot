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
    poll_cron: '*/5 * * * *',
    poll_timeout_seconds: 12,
    dynamic_time_range_seconds: 7200,
    max_updates_per_poll: 6,
    tokens: [],
    subscriptions: [],
  }

  const elements = {
    statusText: document.getElementById('status-text'),
    pageTitle: document.getElementById('page-title'),
    pageSubtitle: document.getElementById('page-subtitle'),
    metricEnabled: document.getElementById('metric-enabled'),
    metricSubscriptions: document.getElementById('metric-subscriptions'),
    metricCookies: document.getElementById('metric-cookies'),
    metricCron: document.getElementById('metric-cron'),
    metricValidation: document.getElementById('metric-validation'),
    enabledInput: document.getElementById('enabled-input'),
    pollCronInput: document.getElementById('poll-cron-input'),
    pollTimeoutInput: document.getElementById('poll-timeout-input'),
    dynamicTimeRangeInput: document.getElementById('dynamic-time-range-input'),
    maxUpdatesInput: document.getElementById('max-updates-input'),
    pollCronError: document.getElementById('poll-cron-error'),
    pollTimeoutError: document.getElementById('poll-timeout-error'),
    dynamicTimeRangeError: document.getElementById('dynamic-time-range-error'),
    maxUpdatesError: document.getElementById('max-updates-error'),
    addCookieButton: document.getElementById('add-cookie-button'),
    cookieList: document.getElementById('cookie-list'),
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

  let defaultSettings = { ...DEFAULT_SETTINGS }
  let draft = normalizeSettings(DEFAULT_SETTINGS, {})
  let currentSecrets = {}
  let savedSnapshot = ''
  let validation = { errors: [] }
  let selectedSubscriptionId = ''
  let readyTimer = null
  let readyAttempts = 0
  let initialized = false
  let pendingSave = null

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

  function clampNumber(value, minimum, maximum, fallback) {
    const number = Number(value)
    if (!Number.isFinite(number)) {
      return fallback
    }
    return Math.max(minimum, Math.min(maximum, Math.trunc(number)))
  }

  function normalizeCron(value) {
    const text = String(value || '').trim()
    return text.split(/\s+/).length === 5 ? text : DEFAULT_SETTINGS.poll_cron
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
          return id ? { id, nickname: nickname || id } : null
        }
        const text = String(item || '').trim()
        return text ? { id: text, nickname: text } : null
      })
      .filter(Boolean)
  }

  function normalizeTokens(value, secrets) {
    const source = Array.isArray(value) ? value : []
    const seen = new Set()
    return source
      .map((item, index) => {
        const sourceItem = isRecord(item) ? item : {}
        const id = safeId(sourceItem.id, index === 0 ? 'primary' : `cookie-${index + 1}`)
        const secretKey = safeId(sourceItem.secret_key, index === 0 ? 'bili.primary' : `bili.cookie_${index + 1}`)
        if (seen.has(id)) {
          return null
        }
        seen.add(id)
        return {
          id,
          label: String(sourceItem.label || id).trim() || id,
          secret_key: secretKey,
          enabled: sourceItem.enabled !== false,
          secret_value: String(sourceItem.secret_value || secrets[secretKey] || ''),
          show_secret: false,
        }
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
          target_type: ['group', 'private'].includes(targetType) ? targetType : 'group',
          target_id: targetId,
          services: normalizeServices(sourceItem.services),
          subscribers: normalizeSubscribers(sourceItem.subscribers),
          enabled: sourceItem.enabled !== false,
        }
      })
      .filter(Boolean)
  }

  function normalizeSettings(values, secrets) {
    const source = isRecord(values) ? values : {}
    return {
      enabled: source.enabled !== false,
      poll_cron: normalizeCron(source.poll_cron),
      poll_timeout_seconds: clampNumber(source.poll_timeout_seconds, 5, 60, DEFAULT_SETTINGS.poll_timeout_seconds),
      dynamic_time_range_seconds: clampNumber(source.dynamic_time_range_seconds, 60, 604800, DEFAULT_SETTINGS.dynamic_time_range_seconds),
      max_updates_per_poll: clampNumber(source.max_updates_per_poll, 1, 20, DEFAULT_SETTINGS.max_updates_per_poll),
      tokens: normalizeTokens(source.tokens, secrets || {}),
      subscriptions: normalizeSubscriptions(source.subscriptions),
    }
  }

  function buildPayloadFromDraft(source) {
    const secrets = {}
    const tokens = source.tokens.map((item) => {
      const secretKey = safeId(item.secret_key, item.id)
      secrets[secretKey] = String(item.secret_value || '')
      return {
        id: safeId(item.id, secretKey),
        label: String(item.label || item.id || secretKey).trim(),
        secret_key: secretKey,
        enabled: item.enabled !== false,
      }
    })

    const activeSecretKeys = new Set(tokens.map((item) => item.secret_key))
    const deletedKeys = Object.keys(currentSecrets).filter((key) => !activeSecretKeys.has(key))

    return {
      settings: {
        enabled: source.enabled !== false,
        poll_cron: String(source.poll_cron || '').trim() || DEFAULT_SETTINGS.poll_cron,
        poll_timeout_seconds: clampNumber(source.poll_timeout_seconds, 5, 60, DEFAULT_SETTINGS.poll_timeout_seconds),
        dynamic_time_range_seconds: clampNumber(source.dynamic_time_range_seconds, 60, 604800, DEFAULT_SETTINGS.dynamic_time_range_seconds),
        max_updates_per_poll: clampNumber(source.max_updates_per_poll, 1, 20, DEFAULT_SETTINGS.max_updates_per_poll),
        tokens,
        subscriptions: source.subscriptions.map((item) => ({
          id: safeId(item.id, `bilibili-${item.uid}-${item.target_type}-${item.target_id}`),
          platform: 'bilibili',
          uid: String(item.uid || '').trim(),
          name: String(item.name || item.uid || '').trim(),
          target_type: item.target_type === 'private' ? 'private' : 'group',
          target_id: String(item.target_id || '').trim(),
          services: normalizeServices(item.services),
          subscribers: normalizeSubscribers(item.subscribers),
          enabled: item.enabled !== false,
        })),
      },
      secrets,
      deletedKeys,
    }
  }

  function stableJson(value) {
    return JSON.stringify(value)
  }

  function snapshotFromPayload(payload) {
    return stableJson({
      settings: payload.settings,
      secrets: payload.secrets,
      deletedKeys: payload.deletedKeys,
    })
  }

  function validateDraft() {
    const errors = []
    const cron = String(draft.poll_cron || '').trim()
    if (cron.split(/\s+/).length !== 5) {
      errors.push({ scope: 'poll_cron', message: 'Cron 需要 5 段，例如 */5 * * * *' })
    }
    validateRange('poll_timeout_seconds', draft.poll_timeout_seconds, 5, 60, '请求超时需在 5 - 60 秒之间')
    validateRange('dynamic_time_range_seconds', draft.dynamic_time_range_seconds, 60, 604800, '动态有效时间需在 60 - 604800 秒之间')
    validateRange('max_updates_per_poll', draft.max_updates_per_poll, 1, 20, '单轮最多推送需在 1 - 20 条之间')

    const tokenIds = new Set()
    const secretKeys = new Set()
    draft.tokens.forEach((item, index) => {
      const label = item.label || item.id || `Cookie ${index + 1}`
      if (!item.id) {
        errors.push({ scope: `token-${index}-id`, message: `${label} 的 ID 不能为空` })
      } else if (tokenIds.has(item.id)) {
        errors.push({ scope: `token-${index}-id`, message: `${label} 的 ID 重复` })
      }
      tokenIds.add(item.id)

      if (!item.secret_key) {
        errors.push({ scope: `token-${index}-secret_key`, message: `${label} 的密钥名不能为空` })
      } else if (secretKeys.has(item.secret_key)) {
        errors.push({ scope: `token-${index}-secret_key`, message: `${label} 的密钥名重复` })
      }
      secretKeys.add(item.secret_key)

      if (item.enabled !== false) {
        const value = String(item.secret_value || '').trim()
        if (!value) {
          errors.push({ scope: `token-${index}-secret_value`, message: `${label} 的 Bilibili Cookie 不能为空` })
        } else if (!/SESSDATA\s*=/.test(value)) {
          errors.push({ scope: `token-${index}-secret_value`, message: `${label} 至少需要包含 SESSDATA=...` })
        }
      }
    })

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

    function validateRange(scope, value, minimum, maximum, message) {
      const number = Number(value)
      if (!Number.isFinite(number) || number < minimum || number > maximum) {
        errors.push({ scope, message })
      }
    }
  }

  function firstError(scopePrefix) {
    return validation.errors.find((error) => error.scope === scopePrefix || error.scope.startsWith(`${scopePrefix}-`))
  }

  function isDirty() {
    return snapshotFromPayload(buildPayloadFromDraft(draft)) !== savedSnapshot
  }

  function applySettings(values, secrets, options) {
    currentSecrets = secrets || currentSecrets || {}
    draft = normalizeSettings(values || defaultSettings, currentSecrets)
    if (options && options.markSaved) {
      savedSnapshot = snapshotFromPayload(buildPayloadFromDraft(draft))
    }
    if (!draft.subscriptions.some((item) => item.id === selectedSubscriptionId)) {
      selectedSubscriptionId = draft.subscriptions[0] ? draft.subscriptions[0].id : ''
    }
    render()
  }

  function finishPendingSave() {
    if (!pendingSave || !pendingSave.settingsAck || !pendingSave.secretsAck) {
      return false
    }
    savedSnapshot = snapshotFromPayload(buildPayloadFromDraft(draft))
    pendingSave = null
    setStatus('设置已保存')
    return true
  }

  function render() {
    validation = validateDraft()
    renderControls()
    renderOverview()
    renderCookies()
    renderSubscriptions()
    renderSubscriptionEditor()
    renderRawJson()
    renderFooter()
  }

  function renderControls() {
    elements.enabledInput.checked = draft.enabled
    syncInputValue(elements.pollCronInput, draft.poll_cron)
    syncInputValue(elements.pollTimeoutInput, draft.poll_timeout_seconds)
    syncInputValue(elements.dynamicTimeRangeInput, draft.dynamic_time_range_seconds)
    syncInputValue(elements.maxUpdatesInput, draft.max_updates_per_poll)
    setFieldError(elements.pollCronError, firstError('poll_cron'), 'field-error')
    setFieldError(elements.pollTimeoutError, firstError('poll_timeout_seconds'), 'field-hint')
    setFieldError(elements.dynamicTimeRangeError, firstError('dynamic_time_range_seconds'), 'field-hint')
    setFieldError(elements.maxUpdatesError, firstError('max_updates_per_poll'), 'field-hint')
  }

  function syncInputValue(input, value) {
    const text = String(value)
    if (input.value !== text) {
      input.value = text
    }
  }

  function setFieldError(element, error, fallbackClass) {
    if (!element.dataset.defaultText) {
      element.dataset.defaultText = element.textContent
    }
    element.textContent = error ? error.message : element.dataset.defaultText
    element.className = error ? 'field-error' : fallbackClass
  }

  function renderOverview() {
    const enabledSubscriptions = draft.subscriptions.filter((item) => item.enabled !== false).length
    const enabledCookies = draft.tokens.filter((item) => item.enabled !== false).length
    elements.metricEnabled.textContent = draft.enabled ? '启用' : '停用'
    elements.metricSubscriptions.textContent = `${enabledSubscriptions} / ${draft.subscriptions.length}`
    elements.metricCookies.textContent = `${enabledCookies} / ${draft.tokens.length}`
    elements.metricCron.textContent = draft.poll_cron || DEFAULT_SETTINGS.poll_cron
    elements.metricValidation.textContent = validation.errors.length === 0 ? '可保存' : `${validation.errors.length} 个问题`
    elements.metricValidation.classList.toggle('is-error', validation.errors.length > 0)
  }

  function renderCookies() {
    elements.cookieList.innerHTML = ''
    if (draft.tokens.length === 0) {
      const empty = emptyState('还没有 Cookie', '添加 Bilibili Cookie 后才能轮询需要登录态的动态。', '添加 Bilibili Cookie', addCookie)
      elements.cookieList.appendChild(empty)
      return
    }

    draft.tokens.forEach((token, index) => {
      const card = document.createElement('article')
      card.className = `cookie-card${token.enabled === false ? ' is-muted' : ''}`
      card.appendChild(rowHeader(token.label || token.id, token.enabled ? '启用' : '停用', [
        smallButton(token.show_secret ? '隐藏' : '显示', () => {
          token.show_secret = !token.show_secret
          renderCookies()
        }),
        smallButton('删除', () => removeCookie(index), 'button--danger'),
      ]))

      card.appendChild(fieldInput(`cookie-label-${index}`, '标签', token.label, (value) => {
        token.label = value
        markChanged(false)
      }))
      card.appendChild(fieldInput(`cookie-id-${index}`, 'ID', token.id, (value) => {
        token.id = safeId(value, '')
        markChanged()
      }, { spellcheck: false }))
      card.appendChild(fieldInput(`cookie-secret-key-${index}`, '密钥名', token.secret_key, (value) => {
        token.secret_key = safeId(value, '')
        markChanged()
      }, { spellcheck: false }))
      card.appendChild(fieldInput(`cookie-secret-value-${index}`, 'Cookie', token.secret_value, (value) => {
        token.secret_value = value
        markChanged(false)
      }, { type: token.show_secret ? 'text' : 'password', spellcheck: false }))

      const toggle = labelWrap('cookie-enabled', `cookie-enabled-${index}`)
      toggle.className = 'toggle-row toggle-row--compact'
      const input = document.createElement('input')
      input.id = `cookie-enabled-${index}`
      input.type = 'checkbox'
      input.name = `cookie_enabled_${index}`
      input.autocomplete = 'off'
      input.checked = token.enabled !== false
      input.addEventListener('change', () => {
        token.enabled = input.checked
        markChanged()
      })
      toggle.appendChild(input)
      toggle.appendChild(textBlock('启用这个 Cookie', '停用后不会用于轮询。'))
      card.appendChild(toggle)

      const error = firstError(`token-${index}`)
      if (error) {
        card.appendChild(errorNode(error.message))
      }
      elements.cookieList.appendChild(card)
    })
  }

  function renderSubscriptions() {
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
      title.innerHTML = `
        <span class="subscription-card__title">${escapeHtml(item.name || `Bilibili ${item.uid}`)}</span>
        <span class="subscription-card__meta">${escapeHtml(item.uid || '未填写 UID')} · ${targetLabel(item.target_type)} ${escapeHtml(item.target_id || '未填写目标')}</span>
      `
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
    elements.subscriptionEditor.innerHTML = ''
    const index = draft.subscriptions.findIndex((item) => item.id === selectedSubscriptionId)
    if (index < 0) {
      elements.subscriptionEditorPanel.classList.add('is-collapsed')
      elements.subscriptionEditorTitle.textContent = '订阅编辑'
      elements.subscriptionEditorSubtitle.textContent = '选择一条订阅，或新建订阅。'
      return
    }

    elements.subscriptionEditorPanel.classList.remove('is-collapsed')
    const item = draft.subscriptions[index]
    elements.subscriptionEditorTitle.textContent = item.name || `Bilibili ${item.uid || ''}`.trim() || '新订阅'
    elements.subscriptionEditorSubtitle.textContent = `${targetLabel(item.target_type)} ${item.target_id || '未填写目标'}`

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
    grid.appendChild(fieldInput('subscription-name', '显示名称', item.name, (value) => {
      item.name = value
      markChanged(false)
    }))
    grid.appendChild(selectField('subscription-target-type', '目标类型', TARGET_OPTIONS, item.target_type, (value) => {
      item.target_type = value
      markChanged()
    }))
    grid.appendChild(fieldInput('subscription-target-id', '目标 ID', item.target_id, (value) => {
      item.target_id = value.trim()
      markChanged(false)
    }, { spellcheck: false }))
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
    const payload = buildPayloadFromDraft(draft).settings
    const text = JSON.stringify(payload, null, 2)
    if (document.activeElement !== elements.rawJsonInput && elements.rawJsonInput.value !== text) {
      elements.rawJsonInput.value = text
    }
  }

  function renderFooter() {
    const dirty = isDirty()
    const hasErrors = validation.errors.length > 0
    elements.dirtyState.textContent = !initialized ? '等待载入' : pendingSave ? '正在保存…' : hasErrors ? `存在 ${validation.errors.length} 个问题` : dirty ? '有未保存更改' : '设置已同步'
    elements.dirtyState.classList.toggle('is-error', hasErrors)
    elements.dirtyState.classList.toggle('is-dirty', initialized && !pendingSave && dirty && !hasErrors)
    elements.dirtyState.classList.toggle('is-synced', initialized && !pendingSave && !dirty && !hasErrors)
    elements.saveButton.disabled = !initialized || Boolean(pendingSave) || hasErrors || !dirty
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

  function getFilteredSubscriptions() {
    const query = elements.subscriptionSearchInput.value.trim().toLowerCase()
    const status = elements.statusFilterInput.value
    const service = elements.serviceFilterInput.value
    return draft.subscriptions
      .map((item, index) => ({ item, index }))
      .filter(({ item }) => {
        const searchText = `${item.uid} ${item.name} ${item.target_id}`.toLowerCase()
        const statusMatches = status === 'all' || (status === 'enabled' ? item.enabled !== false : item.enabled === false)
        const services = normalizeServices(item.services)
        const serviceMatches = service === 'all' || services.includes(service) || services.includes('all')
        return (!query || searchText.includes(query)) && statusMatches && serviceMatches
      })
  }

  function clearFilters() {
    elements.subscriptionSearchInput.value = ''
    elements.statusFilterInput.value = 'all'
    elements.serviceFilterInput.value = 'all'
    renderSubscriptions()
  }

  function addCookie() {
    const next = draft.tokens.length + 1
    draft.tokens.push({
      id: next === 1 ? 'primary' : `cookie-${next}`,
      label: next === 1 ? '主 Cookie' : `备用 Cookie ${next}`,
      secret_key: next === 1 ? 'bili.primary' : `bili.cookie_${next}`,
      enabled: true,
      secret_value: '',
      show_secret: true,
    })
    markChanged()
  }

  function removeCookie(index) {
    const token = draft.tokens[index]
    if (!window.confirm(`删除 ${token.label || token.id}？`)) {
      return
    }
    draft.tokens.splice(index, 1)
    markChanged()
  }

  function addSubscription() {
    const next = draft.subscriptions.length + 1
    const item = {
      id: `bilibili-new-${next}`,
      platform: 'bilibili',
      uid: '',
      name: '',
      target_type: 'group',
      target_id: '',
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
    pendingSave = { settingsAck: false, secretsAck: false }
    setStatus('正在保存设置…')
    postMessage('settings.save', { values: payload.settings }, `save-settings-${Date.now()}`)
    postMessage('secrets.save', { values: payload.secrets, deleted_keys: payload.deletedKeys }, `save-secrets-${Date.now()}`)
  }

  function focusFirstError() {
    const first = validation.errors[0]
    if (!first) {
      return
    }
    if (first.scope.startsWith('token-')) {
      const [, index] = first.scope.split('-')
      const input = document.getElementById(`cookie-secret-value-${index}`) || document.getElementById(`cookie-id-${index}`)
      if (input) {
        input.focus()
      }
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
      return
    }
    const target = document.querySelector(`[name="${first.scope}"]`)
    if (target) {
      target.focus()
    }
  }

  function reloadAll() {
    setStatus('正在重新读取设置…')
    postMessage('settings.reload', undefined, `reload-settings-${Date.now()}`)
    postMessage('secrets.reload', undefined, `reload-secrets-${Date.now()}`)
  }

  function resetSettings() {
    applySettings(defaultSettings, currentSecrets)
    setStatus('默认设置已载入，保存后生效')
  }

  function triggerManualCheck() {
    setStatus('正在触发订阅检查…')
    postMessage('scheduler.trigger', { job_id: 'subscription-hub-poll' }, `trigger-scheduler-${Date.now()}`)
  }

  function openCardPreview() {
    postMessage('render_template.open', { template_id: 'plugin.raylea.subscription-hub.bilibili-update' }, `open-template-${Date.now()}`)
  }

  function importRawJson() {
    try {
      const parsed = JSON.parse(elements.rawJsonInput.value || '{}')
      draft = normalizeSettings(parsed, currentSecrets)
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
    elements.enabledInput.addEventListener('change', () => {
      draft.enabled = elements.enabledInput.checked
      markChanged()
    })
    elements.pollCronInput.addEventListener('input', () => {
      draft.poll_cron = elements.pollCronInput.value
      markChanged(false)
    })
    elements.pollTimeoutInput.addEventListener('input', () => {
      draft.poll_timeout_seconds = elements.pollTimeoutInput.value
      markChanged(false)
    })
    elements.dynamicTimeRangeInput.addEventListener('input', () => {
      draft.dynamic_time_range_seconds = elements.dynamicTimeRangeInput.value
      markChanged(false)
    })
    elements.maxUpdatesInput.addEventListener('input', () => {
      draft.max_updates_per_poll = elements.maxUpdatesInput.value
      markChanged(false)
    })
    elements.addCookieButton.addEventListener('click', addCookie)
    elements.addSubscriptionButton.addEventListener('click', addSubscription)
    elements.subscriptionSearchInput.addEventListener('input', renderSubscriptions)
    elements.statusFilterInput.addEventListener('change', renderSubscriptions)
    elements.serviceFilterInput.addEventListener('change', renderSubscriptions)
    elements.closeEditorButton.addEventListener('click', () => {
      selectedSubscriptionId = ''
      renderSubscriptions()
      renderSubscriptionEditor()
    })
    elements.exportJsonButton.addEventListener('click', () => {
      renderRawJson()
      setStatus('原始配置预览已刷新')
    })
    elements.importJsonButton.addEventListener('click', importRawJson)
    elements.reloadButton.addEventListener('click', reloadAll)
    elements.resetButton.addEventListener('click', resetSettings)
    elements.manualCheckButton.addEventListener('click', triggerManualCheck)
    elements.previewButton.addEventListener('click', openCardPreview)
    elements.saveButton.addEventListener('click', saveAll)
  }

  window.addEventListener('message', (event) => {
    const message = event.data
    if (!message || message.version !== '1' || typeof message.type !== 'string') {
      return
    }

    if (message.type === 'host.init') {
      stopReadyLoop()
      const payload = message.payload || {}
      elements.pageTitle.textContent = payload.title || '订阅设置'
      elements.pageSubtitle.textContent = payload.plugin && payload.plugin.description
        ? payload.plugin.description
        : '管理 Bilibili 订阅、轮询和 Cookie'
      defaultSettings = normalizeSettings(payload.default_config || DEFAULT_SETTINGS, {})
      currentSecrets = payload.secrets || {}
      initialized = true
      applySettings(payload.settings || defaultSettings, currentSecrets, { markSaved: true })
      setStatus('已载入设置')
      return
    }

    if (message.type === 'settings.changed') {
      const payload = message.payload || {}
      if (pendingSave) {
        pendingSave.settingsAck = true
        draft = normalizeSettings(payload.values || defaultSettings, buildPayloadFromDraft(draft).secrets)
        if (!draft.subscriptions.some((item) => item.id === selectedSubscriptionId)) {
          selectedSubscriptionId = draft.subscriptions[0] ? draft.subscriptions[0].id : ''
        }
        if (!finishPendingSave()) {
          setStatus('设置已保存，正在保存敏感值…')
        }
        render()
        return
      }
      applySettings(payload.values || defaultSettings, currentSecrets, { markSaved: true })
      setStatus('设置已保存')
      return
    }

    if (message.type === 'secrets.changed') {
      const payload = message.payload || {}
      currentSecrets = payload.values || {}
      if (pendingSave) {
        pendingSave.secretsAck = true
        draft = normalizeSettings(buildPayloadFromDraft(draft).settings, currentSecrets)
        if (!finishPendingSave()) {
          setStatus('敏感值已保存，正在等待设置回写…')
        }
        render()
        return
      }
      applySettings(buildPayloadFromDraft(draft).settings, currentSecrets, { markSaved: true })
      setStatus('敏感值已保存')
      return
    }

    if (message.type === 'scheduler.triggered') {
      setStatus('已触发订阅检查')
      return
    }

    if (message.type === 'error') {
      const payload = message.payload || {}
      pendingSave = null
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
    getDraft: () => JSON.parse(JSON.stringify(draft)),
    readyAttempts: () => readyAttempts,
  }
})()
