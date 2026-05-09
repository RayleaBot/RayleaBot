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
    return text
      .split(/\n(?=\s*\{)/)
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => {
        const parsed = JSON.parse(line)
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
          throw new Error(`${label} 必须是 JSON 对象`)
        }
        return parsed
      })
  }

  function tokensForDisplay(settings, secrets) {
    return (settings.tokens || []).map((item) => ({
      ...item,
      secret_value: secrets[item.secret_key] || '',
    }))
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
    const tokensWithSecrets = parseJSONLines(tokensInput.value, 'Tokens')
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
      pageSubtitle.textContent = payload.plugin && payload.plugin.description
        ? payload.plugin.description
        : '管理 Bilibili 订阅、轮询和 token'
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
