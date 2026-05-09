(function () {
  const statusText = document.getElementById('status-text')
  const pageTitle = document.getElementById('page-title')
  const pageSubtitle = document.getElementById('page-subtitle')
  const enabledInput = document.getElementById('enabled-input')
  const pollCronInput = document.getElementById('poll-cron-input')
  const pollTimeoutInput = document.getElementById('poll-timeout-input')
  const maxUpdatesInput = document.getElementById('max-updates-input')
  const tokensInput = document.getElementById('tokens-input')
  const subscriptionsInput = document.getElementById('subscriptions-input')
  const reloadButton = document.getElementById('reload-button')
  const resetButton = document.getElementById('reset-button')
  const saveButton = document.getElementById('save-button')

  let defaultSettings = {}
  let currentSettings = {}
  let currentSecrets = {}

  function setStatus(message, isError) {
    statusText.textContent = message
    statusText.classList.toggle('is-error', Boolean(isError))
  }

  function postMessage(type, payload, requestId) {
    window.parent.postMessage({
      version: '1',
      source: 'plugin_management_ui',
      type,
      request_id: requestId,
      payload,
    }, '*')
  }

  function formatJSONLines(value) {
    return Array.isArray(value) ? value.map((item) => JSON.stringify(item, null, 2)).join('\n') : ''
  }

  function parseJSONLines(value, label) {
    const text = String(value || '').trim()
    if (!text) {
      return []
    }
    if (text.startsWith('[')) {
      const parsed = JSON.parse(text)
      if (!Array.isArray(parsed)) {
        throw new Error(`${label} 必须是 JSON 对象或对象数组`)
      }
      for (const item of parsed) {
        if (!item || typeof item !== 'object' || Array.isArray(item)) {
          throw new Error(`${label} 必须是 JSON 对象或对象数组`)
        }
      }
      return parsed
    }

    const items = []
    let start = -1
    let depth = 0
    let inString = false
    let escaped = false

    for (let index = 0; index < text.length; index += 1) {
      const char = text[index]
      if (inString) {
        if (escaped) {
          escaped = false
        } else if (char === '\\') {
          escaped = true
        } else if (char === '"') {
          inString = false
        }
        continue
      }

      if (char === '"') {
        inString = true
        continue
      }
      if (char === '{') {
        if (depth === 0) {
          if (text.slice(items.length === 0 ? 0 : start, index).trim() && start < index) {
            throw new Error(`${label} 必须是 JSON 对象或对象数组`)
          }
          start = index
        }
        depth += 1
        continue
      }
      if (char === '}') {
        depth -= 1
        if (depth < 0) {
          throw new Error(`${label} JSON 格式不正确`)
        }
        if (depth === 0) {
          const parsed = JSON.parse(text.slice(start, index + 1))
          if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
            throw new Error(`${label} 必须是 JSON 对象或对象数组`)
          }
          items.push(parsed)
          start = index + 1
        }
      }
    }

    if (inString || depth !== 0) {
      throw new Error(`${label} JSON 内容不完整`)
    }
    if (items.length === 0 || text.slice(start).trim()) {
      throw new Error(`${label} 必须是 JSON 对象或对象数组`)
    }
    return items
  }

  function parseCookieInput(value) {
    const text = String(value || '').trim()
    if (!text) {
      return []
    }
    if (text.startsWith('{') || text.startsWith('[')) {
      return parseJSONLines(text, 'Bilibili Cookie')
    }
    return text
      .split(/\n+/)
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line, index) => ({
        id: index === 0 ? 'primary' : `cookie-${index + 1}`,
        label: index === 0 ? '主 Cookie' : `备用 Cookie ${index + 1}`,
        secret_key: index === 0 ? 'bili.primary' : `bili.cookie_${index + 1}`,
        enabled: true,
        secret_value: line,
      }))
  }

  function tokensForDisplay(settings, secrets) {
    return (settings.tokens || []).map((item) => ({
      ...item,
      secret_value: secrets[item.secret_key] || '',
    }))
  }

  function validateCookieItems(items) {
    for (const item of items) {
      if (item && item.enabled === false) {
        continue
      }

      const label = String((item && (item.label || item.id)) || 'Bilibili Cookie')
      const secretValue = String((item && item.secret_value) || '').trim()
      if (!secretValue) {
        throw new Error(`${label} 的 Bilibili Cookie 不能为空`)
      }
      if (!/SESSDATA\s*=/.test(secretValue)) {
        throw new Error(`${label} 的 Bilibili Cookie 至少需要包含 SESSDATA=...`)
      }
    }
  }

  function applySettings(values, secrets) {
    currentSettings = values || {}
    currentSecrets = secrets || currentSecrets || {}
    enabledInput.checked = Boolean(currentSettings.enabled ?? defaultSettings.enabled ?? true)
    pollCronInput.value = currentSettings.poll_cron || defaultSettings.poll_cron || '*/5 * * * *'
    pollTimeoutInput.value = String(currentSettings.poll_timeout_seconds || defaultSettings.poll_timeout_seconds || 12)
    maxUpdatesInput.value = String(currentSettings.max_updates_per_poll || defaultSettings.max_updates_per_poll || 6)
    tokensInput.value = formatJSONLines(tokensForDisplay(currentSettings, currentSecrets))
    subscriptionsInput.value = formatJSONLines(currentSettings.subscriptions || defaultSettings.subscriptions)
  }

  function buildPayload() {
    const tokensWithSecrets = parseCookieInput(tokensInput.value)
    validateCookieItems(tokensWithSecrets)
    const values = {}
    const tokens = tokensWithSecrets.map((item) => {
      const token = { ...item }
      delete token.secret_value
      values[token.secret_key] = String(item.secret_value || '')
      return token
    })
    const activeSecretKeys = new Set(tokens.map((item) => item.secret_key))
    const deletedKeys = Object.keys(currentSecrets).filter((key) => !activeSecretKeys.has(key))
    return {
      settings: {
        enabled: enabledInput.checked,
        poll_cron: pollCronInput.value.trim() || '*/5 * * * *',
        poll_timeout_seconds: Number(pollTimeoutInput.value || 12),
        max_updates_per_poll: Number(maxUpdatesInput.value || 6),
        tokens,
        subscriptions: parseJSONLines(subscriptionsInput.value, '订阅列表'),
      },
      secrets: values,
      deletedKeys,
    }
  }

  function saveAll() {
    try {
      const payload = buildPayload()
      setStatus('正在保存设置')
      postMessage('settings.save', { values: payload.settings }, `save-settings-${Date.now()}`)
      postMessage('secrets.save', { values: payload.secrets, deleted_keys: payload.deletedKeys }, `save-secrets-${Date.now()}`)
    } catch (error) {
      setStatus(error && error.message ? error.message : '设置格式不正确', true)
    }
  }

  function reloadAll() {
    setStatus('正在重新读取设置')
    postMessage('settings.reload', undefined, `reload-settings-${Date.now()}`)
    postMessage('secrets.reload', undefined, `reload-secrets-${Date.now()}`)
  }

  function resetSettings() {
    applySettings(defaultSettings, currentSecrets)
    setStatus('默认设置已载入，保存后生效')
  }

  window.addEventListener('message', (event) => {
    const message = event.data
    if (!message || message.version !== '1' || typeof message.type !== 'string') {
      return
    }

    if (message.type === 'host.init') {
      const payload = message.payload || {}
      pageTitle.textContent = payload.title || '订阅设置'
      pageSubtitle.textContent = '管理 Bilibili 订阅、轮询和 Cookie'
      defaultSettings = payload.default_config || {}
      currentSecrets = payload.secrets || {}
      applySettings(payload.settings || defaultSettings, currentSecrets)
      setStatus('已载入设置')
      return
    }

    if (message.type === 'settings.changed') {
      const payload = message.payload || {}
      applySettings(payload.values || defaultSettings, currentSecrets)
      setStatus('设置已保存')
      return
    }

    if (message.type === 'secrets.changed') {
      const payload = message.payload || {}
      currentSecrets = payload.values || {}
      applySettings(currentSettings || defaultSettings, currentSecrets)
      setStatus('敏感值已保存')
      return
    }

    if (message.type === 'error') {
      const payload = message.payload || {}
      setStatus(payload.message || '操作未完成', true)
    }
  })

  reloadButton.addEventListener('click', reloadAll)
  resetButton.addEventListener('click', resetSettings)
  saveButton.addEventListener('click', saveAll)

  postMessage('page.ready', undefined, `ready-${Date.now()}`)
})()
