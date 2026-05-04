(function () {
  const statusText = document.getElementById('status-text')
  const pageTitle = document.getElementById('page-title')
  const pageSubtitle = document.getElementById('page-subtitle')
  const triggerCommandsInput = document.getElementById('trigger-commands-input')
  const timezoneInput = document.getElementById('timezone-input')
  const specialDatesInput = document.getElementById('special-dates-input')
  const fortunesInput = document.getElementById('fortunes-input')
  const goodActionsInput = document.getElementById('good-actions-input')
  const badActionsInput = document.getElementById('bad-actions-input')
  const reloadButton = document.getElementById('reload-button')
  const resetButton = document.getElementById('reset-button')
  const saveButton = document.getElementById('save-button')

  let defaultSettings = {}
  let currentSettings = {}

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

  function toLines(value) {
    return Array.isArray(value) ? value.map((item) => String(item)).join('\n') : ''
  }

  function fromLines(value) {
    return String(value || '')
      .split(/\r?\n/)
      .map((item) => item.trim())
      .filter(Boolean)
  }

  function formatJSON(value) {
    return JSON.stringify(value || [], null, 2)
  }

  function parseJSONList(value, label) {
    const text = String(value || '').trim()
    if (!text) {
      return []
    }
    const parsed = JSON.parse(text)
    if (!Array.isArray(parsed)) {
      throw new Error(`${label} 必须是 JSON 数组`)
    }
    return parsed
  }

  function applySettings(values) {
    currentSettings = values || {}
    triggerCommandsInput.value = toLines(currentSettings.trigger_commands || defaultSettings.trigger_commands)
    timezoneInput.value = currentSettings.timezone || defaultSettings.timezone || 'Asia/Shanghai'
    specialDatesInput.value = formatJSON(currentSettings.special_dates || defaultSettings.special_dates)
    fortunesInput.value = formatJSON(currentSettings.fortunes || defaultSettings.fortunes)
    goodActionsInput.value = toLines(currentSettings.good_actions || defaultSettings.good_actions)
    badActionsInput.value = toLines(currentSettings.bad_actions || defaultSettings.bad_actions)
  }

  function buildPayload() {
    return {
      trigger_commands: fromLines(triggerCommandsInput.value),
      timezone: timezoneInput.value.trim() || 'Asia/Shanghai',
      special_dates: parseJSONList(specialDatesInput.value, '特殊日期'),
      fortunes: parseJSONList(fortunesInput.value, '运势库'),
      good_actions: fromLines(goodActionsInput.value),
      bad_actions: fromLines(badActionsInput.value),
    }
  }

  function saveSettings() {
    try {
      const values = buildPayload()
      setStatus('正在保存设置')
      postMessage('settings.save', { values }, `save-${Date.now()}`)
    } catch (error) {
      setStatus(error && error.message ? error.message : '设置格式不正确', true)
    }
  }

  function reloadSettings() {
    setStatus('正在重新读取设置')
    postMessage('settings.reload', undefined, `reload-${Date.now()}`)
  }

  function resetSettings() {
    applySettings(defaultSettings)
    setStatus('默认设置已载入，保存后生效')
  }

  window.addEventListener('message', (event) => {
    const message = event.data
    if (!message || message.version !== '1' || typeof message.type !== 'string') {
      return
    }

    if (message.type === 'host.init') {
      const payload = message.payload || {}
      pageTitle.textContent = payload.title || '运势设置'
      pageSubtitle.textContent = payload.plugin && payload.plugin.description
        ? payload.plugin.description
        : '设置触发词、时区、特殊日期和运势库'
      defaultSettings = payload.default_config || {}
      applySettings(payload.settings || defaultSettings)
      setStatus('已载入设置')
      return
    }

    if (message.type === 'settings.changed') {
      const payload = message.payload || {}
      applySettings(payload.values || defaultSettings)
      setStatus('设置已保存')
      return
    }

    if (message.type === 'error') {
      const payload = message.payload || {}
      setStatus(payload.message || '操作未完成', true)
    }
  })

  reloadButton.addEventListener('click', reloadSettings)
  resetButton.addEventListener('click', resetSettings)
  saveButton.addEventListener('click', saveSettings)

  postMessage('page.ready', undefined, `ready-${Date.now()}`)
})()
