(function () {
  const statusText = document.getElementById('status-text')
  const settingsPreview = document.getElementById('settings-preview')
  const pageTitle = document.getElementById('page-title')
  const pageSubtitle = document.getElementById('page-subtitle')
  const pluginIdText = document.getElementById('plugin-id')
  const defaultCityInput = document.getElementById('default-city-input')
  const unitSelect = document.getElementById('unit-select')
  const reloadButton = document.getElementById('reload-button')
  const saveButton = document.getElementById('save-button')

  let latestSettings = {}
  let lastSuccessfulAction = null

  function setStatus(message) {
    statusText.textContent = message
  }

  function setPreview(value) {
    settingsPreview.textContent = JSON.stringify(value, null, 2)
  }

  function applySettings(values) {
    latestSettings = values || {}
    defaultCityInput.value = typeof latestSettings.default_city === 'string' ? latestSettings.default_city : ''
    unitSelect.value = typeof latestSettings.unit === 'string' ? latestSettings.unit : 'celsius'
    setPreview(latestSettings)
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

  function saveSettings() {
    setStatus('正在保存设置')
    postMessage('settings.save', {
      values: {
        default_city: defaultCityInput.value.trim(),
        unit: unitSelect.value,
      },
    }, `save-${Date.now()}`)
  }

  function reloadSettings() {
    setStatus('正在重新读取设置')
    postMessage('settings.reload', undefined, `reload-${Date.now()}`)
  }

  window.addEventListener('message', (event) => {
    const message = event.data
    if (!message || message.version !== '1' || typeof message.type !== 'string') {
      return
    }

    if (message.type === 'host.init') {
      const payload = message.payload || {}
      pageTitle.textContent = payload.title || '配置页面'
      pageSubtitle.textContent = payload.plugin && payload.plugin.description
        ? payload.plugin.description
        : '插件可通过宿主桥接读写自己的设置。'
      pluginIdText.textContent = payload.plugin_id || '--'
      applySettings(payload.settings || payload.default_config || {})
      setStatus(lastSuccessfulAction === 'save' ? '设置已更新' : '已载入设置')
      return
    }

    if (message.type === 'settings.changed') {
      const payload = message.payload || {}
      const changedKeys = Array.isArray(payload.changed_keys) ? payload.changed_keys : []
      applySettings(payload.values || {})
      lastSuccessfulAction = changedKeys.length > 0 ? 'save' : 'reload'
      setStatus(lastSuccessfulAction === 'save' ? '设置已更新' : '已载入设置')
      return
    }

    if (message.type === 'error') {
      const payload = message.payload || {}
      setStatus(payload.message || '操作未完成')
    }
  })

  reloadButton.addEventListener('click', reloadSettings)
  saveButton.addEventListener('click', saveSettings)

  setStatus('正在等待宿主初始化')
  postMessage('page.ready', undefined, `ready-${Date.now()}`)
})()
