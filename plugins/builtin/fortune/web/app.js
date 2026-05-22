(function () {
  const DEFAULT_TIMEZONE = 'Asia/Shanghai'
  const DEFAULT_TRIGGER_COMMANDS = ['我的运势']
  const DEFAULT_STATS_TRIGGER_COMMANDS = ['运势统计']
  const DEFAULT_GOOD_ACTIONS = ['整理计划']
  const DEFAULT_BAD_ACTIONS = ['熬夜']
  const FORTUNE_NAMES = ['大吉', '吉', '中吉', '小吉', '末吉', '凶', '大凶', '吉凶未定']
  const EXPECTED_STARS = {
    大吉: ['★★★★★★★'],
    吉: ['★★★★★★☆'],
    中吉: ['★★★★★☆☆'],
    小吉: ['★★★★☆☆☆'],
    末吉: ['★★★☆☆☆☆'],
    凶: ['★★☆☆☆☆☆', '★☆☆☆☆☆☆'],
    大凶: ['☆☆☆☆☆☆☆'],
    吉凶未定: ['???????'],
  }
  const TIMEZONE_OPTIONS = [
    { value: 'Asia/Shanghai', offset: 'UTC+08:00', label: '中国标准时间' },
    { value: 'UTC', offset: 'UTC+00:00', label: '协调世界时' },
    { value: 'Etc/UTC', offset: 'UTC+00:00', label: 'UTC 标准名称' },
    { value: 'PRC', offset: 'UTC+08:00', label: '中国时区别名' },
    { value: 'Asia/Tokyo', offset: 'UTC+09:00', label: '日本标准时间' },
    { value: 'Asia/Seoul', offset: 'UTC+09:00', label: '韩国标准时间' },
    { value: 'Asia/Singapore', offset: 'UTC+08:00', label: '新加坡时间' },
    { value: 'Europe/London', offset: 'UTC+00:00/UTC+01:00', label: '伦敦时间' },
    { value: 'Europe/Paris', offset: 'UTC+01:00/UTC+02:00', label: '巴黎时间' },
    { value: 'America/New_York', offset: 'UTC-05:00/UTC-04:00', label: '纽约时间' },
    { value: 'America/Los_Angeles', offset: 'UTC-08:00/UTC-07:00', label: '洛杉矶时间' },
    { value: 'UTC+08:00', offset: 'UTC+08:00', label: '固定东八区' },
    { value: '+08:00', offset: 'UTC+08:00', label: '固定东八区简写' },
  ]

  const elements = {
    statusText: document.getElementById('status-text'),
    pageTitle: document.getElementById('page-title'),
    pageSubtitle: document.getElementById('page-subtitle'),
    fortuneCount: document.getElementById('fortune-count'),
    specialDateCount: document.getElementById('special-date-count'),
    triggerCount: document.getElementById('trigger-count'),
    timezoneSummary: document.getElementById('timezone-summary'),
    validationSummary: document.getElementById('validation-summary'),
    dirtyState: document.getElementById('dirty-state'),
    timezoneInput: document.getElementById('timezone-input'),
    timezoneOptions: document.getElementById('timezone-options'),
    timezoneList: document.getElementById('timezone-list'),
    fortuneTriggerInput: document.getElementById('fortune-trigger-input'),
    statsTriggerInput: document.getElementById('stats-trigger-input'),
    goodActionsInput: document.getElementById('good-actions-input'),
    badActionsInput: document.getElementById('bad-actions-input'),
    fortuneTriggerList: document.getElementById('fortune-trigger-list'),
    statsTriggerList: document.getElementById('stats-trigger-list'),
    goodActionsList: document.getElementById('good-actions-list'),
    badActionsList: document.getElementById('bad-actions-list'),
    fortuneFilterInput: document.getElementById('fortune-filter-input'),
    fortuneTableBody: document.getElementById('fortune-table-body'),
    specialDateTableBody: document.getElementById('special-date-table-body'),
    addFortuneButton: document.getElementById('add-fortune-button'),
    addSpecialDateButton: document.getElementById('add-special-date-button'),
    reloadButton: document.getElementById('reload-button'),
    resetButton: document.getElementById('reset-button'),
    saveButton: document.getElementById('save-button'),
  }

  let defaultSettings = {}
  let draft = emptyDraft()
  let savedSnapshot = stableJson(buildPayloadFromDraft(draft))
  let validation = { errors: [] }
  let readyTimer = null
  let readyAttempts = 0

  function emptyDraft() {
    return {
      trigger_commands: [],
      stats_trigger_commands: [],
      timezone: DEFAULT_TIMEZONE,
      fortunes: [],
      special_dates: [],
      good_actions: [],
      bad_actions: [],
    }
  }

  function setStatus(message, isError) {
    elements.statusText.textContent = message
    elements.statusText.classList.toggle('is-error', Boolean(isError))
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

  function stableJson(value) {
    return JSON.stringify(value)
  }

  function normalizeStringList(value, fallback) {
    const hasExplicitValue = Array.isArray(value)
    const source = hasExplicitValue ? value : fallback
    const seen = new Set()
    const items = []
    for (const item of source || []) {
      const text = String(item || '').trim()
      if (!text || seen.has(text)) {
        continue
      }
      seen.add(text)
      items.push(text)
    }
    return items.length > 0 || hasExplicitValue ? items : Array.from(fallback || [])
  }

  function normalizeFortune(item) {
    const source = item && typeof item === 'object' ? item : {}
    const name = String(source.name || '大吉').trim()
    const stars = String(source.stars || firstStarsForName(name)).trim()
    return {
      name,
      stars,
      sign: String(source.sign || '').trim(),
      explanation: String(source.explanation || '').trim(),
    }
  }

  function normalizeSpecialDate(item) {
    const source = item && typeof item === 'object' ? item : {}
    const fortuneValue = source.fortune && typeof source.fortune === 'object' ? source.fortune.name : source.fortune
    return {
      date: String(source.date || '').trim(),
      fortune_name: String(source.fortune_name || fortuneValue || '').trim(),
    }
  }

  function firstStarsForName(name) {
    const options = EXPECTED_STARS[name] || EXPECTED_STARS['大吉']
    return options[0]
  }

  function coerceSettings(values) {
    const source = values && typeof values === 'object' ? values : {}
    return {
      trigger_commands: normalizeStringList(source.trigger_commands, defaultSettings.trigger_commands || DEFAULT_TRIGGER_COMMANDS),
      stats_trigger_commands: normalizeStringList(
        source.stats_trigger_commands,
        defaultSettings.stats_trigger_commands || DEFAULT_STATS_TRIGGER_COMMANDS,
      ),
      timezone: String(source.timezone || defaultSettings.timezone || DEFAULT_TIMEZONE).trim() || DEFAULT_TIMEZONE,
      fortunes: Array.isArray(source.fortunes)
        ? source.fortunes.map(normalizeFortune)
        : Array.isArray(defaultSettings.fortunes) ? defaultSettings.fortunes.map(normalizeFortune) : [],
      special_dates: Array.isArray(source.special_dates)
        ? source.special_dates.map(normalizeSpecialDate)
        : Array.isArray(defaultSettings.special_dates) ? defaultSettings.special_dates.map(normalizeSpecialDate) : [],
      good_actions: normalizeStringList(source.good_actions, defaultSettings.good_actions || DEFAULT_GOOD_ACTIONS),
      bad_actions: normalizeStringList(source.bad_actions, defaultSettings.bad_actions || DEFAULT_BAD_ACTIONS),
    }
  }

  function buildPayloadFromDraft(source) {
    return {
      trigger_commands: normalizeStringList(source.trigger_commands, DEFAULT_TRIGGER_COMMANDS),
      stats_trigger_commands: normalizeStringList(source.stats_trigger_commands, DEFAULT_STATS_TRIGGER_COMMANDS),
      timezone: String(source.timezone || '').trim() || DEFAULT_TIMEZONE,
      special_dates: source.special_dates.map((item) => ({
        date: String(item.date || '').trim(),
        fortune_name: String(item.fortune_name || '').trim(),
      })),
      fortunes: source.fortunes.map((item) => ({
        name: String(item.name || '').trim(),
        stars: String(item.stars || '').trim(),
        sign: String(item.sign || '').trim(),
        explanation: String(item.explanation || '').trim(),
      })),
      good_actions: normalizeStringList(source.good_actions, DEFAULT_GOOD_ACTIONS),
      bad_actions: normalizeStringList(source.bad_actions, DEFAULT_BAD_ACTIONS),
    }
  }

  function applySettings(values, options) {
    draft = coerceSettings(values)
    if (options && options.markSaved) {
      savedSnapshot = stableJson(buildPayloadFromDraft(draft))
    }
    render()
  }

  function markChanged() {
    render()
  }

  function refreshStatusOnly() {
    validation = validateDraft()
    renderOverview()
    renderFooter()
  }

  function isDirty() {
    return stableJson(buildPayloadFromDraft(draft)) !== savedSnapshot
  }

  function validateDraft() {
    const errors = []
    const fortuneNames = new Set(draft.fortunes.map((fortune) => fortune.name).filter(Boolean))

    if (draft.fortunes.length === 0) {
      errors.push({ scope: 'fortunes', message: '运势库至少需要一条可用运势' })
    }

    draft.fortunes.forEach((fortune, index) => {
      if (!fortune.name) {
        errors.push({ scope: `fortune-${index}`, message: '运势名不能为空' })
      } else if (!EXPECTED_STARS[fortune.name]) {
        errors.push({ scope: `fortune-${index}`, message: '运势名不在支持范围内' })
      }
      if (!fortune.stars) {
        errors.push({ scope: `fortune-${index}`, message: '星级不能为空' })
      } else if (!validStars(fortune.name, fortune.stars)) {
        errors.push({ scope: `fortune-${index}`, message: '星级与运势名不匹配' })
      }
      if (!fortune.sign) {
        errors.push({ scope: `fortune-${index}`, message: '签文不能为空' })
      }
      if (!fortune.explanation) {
        errors.push({ scope: `fortune-${index}`, message: '解签不能为空' })
      }
    })

    draft.special_dates.forEach((item, index) => {
      if (!isSpecialDateKey(item.date)) {
        errors.push({ scope: `special-${index}`, message: '日期格式应为 YYYY-MM-DD 或 MM-DD' })
      }
      if (!item.fortune_name) {
        errors.push({ scope: `special-${index}`, message: '特殊日期需要指定运势' })
      } else if (!fortuneNames.has(item.fortune_name)) {
        errors.push({ scope: `special-${index}`, message: '指定运势不在当前运势库中' })
      }
    })

    const timezone = String(draft.timezone || '').trim()
    if (!timezone || !isSupportedTimezoneInput(timezone)) {
      errors.push({ scope: 'timezone', message: '时区格式不正确' })
    }

    return { errors }
  }

  function validStars(name, stars) {
    return (EXPECTED_STARS[name] || []).includes(stars)
  }

  function isSpecialDateKey(value) {
    return /^\d{4}-\d{2}-\d{2}$/.test(value) || /^\d{2}-\d{2}$/.test(value)
  }

  function isSupportedTimezoneInput(value) {
    const text = String(value || '').trim()
    if (!text) {
      return false
    }
    if (TIMEZONE_OPTIONS.some((item) => item.value === text)) {
      return true
    }
    if (/^(?:UTC)?[+-](?:\d|0\d|1[0-4])(?::?[0-5]\d)?$/i.test(text)) {
      const match = text.match(/([+-])(\d{1,2})(?::?(\d{2}))?$/)
      if (!match) {
        return false
      }
      const hours = Number(match[2])
      const minutes = Number(match[3] || '0')
      return hours < 14 || (hours === 14 && minutes === 0)
    }
    return /^[A-Za-z_]+(?:\/[A-Za-z0-9_+\-]+)+$/.test(text)
  }

  function render() {
    validation = validateDraft()
    renderOverview()
    renderTimezone()
    renderChips('trigger_commands', elements.fortuneTriggerList)
    renderChips('stats_trigger_commands', elements.statsTriggerList)
    renderChips('good_actions', elements.goodActionsList)
    renderChips('bad_actions', elements.badActionsList)
    renderFortunes()
    renderSpecialDates()
    renderFooter()
  }

  function renderOverview() {
    elements.fortuneCount.textContent = String(draft.fortunes.length)
    elements.specialDateCount.textContent = String(draft.special_dates.length)
    elements.triggerCount.textContent = String(draft.trigger_commands.length + draft.stats_trigger_commands.length)
    elements.timezoneSummary.textContent = draft.timezone || DEFAULT_TIMEZONE
    elements.validationSummary.textContent = validation.errors.length === 0 ? '可保存' : `${validation.errors.length} 个问题`
    elements.validationSummary.classList.toggle('is-error', validation.errors.length > 0)
  }

  function renderTimezone() {
    if (elements.timezoneInput.value !== draft.timezone) {
      elements.timezoneInput.value = draft.timezone
    }
    elements.timezoneOptions.innerHTML = ''
    TIMEZONE_OPTIONS.forEach((item) => {
      const option = document.createElement('option')
      option.value = item.value
      option.label = `${item.offset} · ${item.label}`
      elements.timezoneOptions.appendChild(option)
    })

    elements.timezoneList.innerHTML = ''
    TIMEZONE_OPTIONS.slice(0, 6).forEach((item) => {
      const button = document.createElement('button')
      button.type = 'button'
      button.className = 'timezone-option'
      button.classList.toggle('is-active', draft.timezone === item.value)
      button.textContent = `${item.value} · ${item.offset} · ${item.label}`
      button.addEventListener('click', () => {
        draft.timezone = item.value
        markChanged()
      })
      elements.timezoneList.appendChild(button)
    })
  }

  function renderChips(key, container) {
    container.innerHTML = ''
    draft[key].forEach((item) => {
      const chip = document.createElement('span')
      chip.className = 'chip'
      chip.textContent = item
      const remove = document.createElement('button')
      remove.type = 'button'
      remove.setAttribute('aria-label', `删除 ${item}`)
      remove.textContent = '×'
      remove.addEventListener('click', () => {
        draft[key] = draft[key].filter((value) => value !== item)
        markChanged()
      })
      chip.appendChild(remove)
      container.appendChild(chip)
    })
  }

  function renderFortunes() {
    const filter = elements.fortuneFilterInput.value.trim().toLowerCase()
    elements.fortuneTableBody.innerHTML = ''

    draft.fortunes.forEach((fortune, index) => {
      const searchable = `${fortune.name} ${fortune.sign} ${fortune.explanation}`.toLowerCase()
      if (filter && !searchable.includes(filter)) {
        return
      }

      const row = document.createElement('tr')
      row.className = hasError(`fortune-${index}`) ? 'has-error' : ''

      row.appendChild(cell(selectForFortuneName(fortune, index)))
      row.appendChild(cell(selectForStars(fortune, index)))
      row.appendChild(cell(textareaForFortune(index, 'sign', '签文')))
      row.appendChild(cell(textareaForFortune(index, 'explanation', '解签')))
      row.appendChild(actionCell([
        actionButton('复制', () => duplicateFortune(index)),
        actionButton('删除', () => removeFortune(index), 'button--danger'),
      ], errorText(`fortune-${index}`)))

      elements.fortuneTableBody.appendChild(row)
    })

    if (elements.fortuneTableBody.children.length === 0) {
      elements.fortuneTableBody.appendChild(emptyRow(5, '没有符合条件的运势'))
    }
  }

  function selectForFortuneName(fortune, index) {
    const select = document.createElement('select')
    select.setAttribute('aria-label', '运势名')
    FORTUNE_NAMES.forEach((name) => {
      const option = document.createElement('option')
      option.value = name
      option.textContent = name
      select.appendChild(option)
    })
    select.value = fortune.name
    select.addEventListener('change', () => {
      draft.fortunes[index].name = select.value
      draft.fortunes[index].stars = firstStarsForName(select.value)
      markChanged()
    })
    return select
  }

  function selectForStars(fortune, index) {
    const select = document.createElement('select')
    select.setAttribute('aria-label', '星级')
    const options = EXPECTED_STARS[fortune.name] || []
    options.forEach((stars) => {
      const option = document.createElement('option')
      option.value = stars
      option.textContent = stars
      select.appendChild(option)
    })
    if (!options.includes(fortune.stars)) {
      const option = document.createElement('option')
      option.value = fortune.stars
      option.textContent = fortune.stars || '未设置'
      select.appendChild(option)
    }
    select.value = fortune.stars
    select.addEventListener('change', () => {
      draft.fortunes[index].stars = select.value
      markChanged()
    })
    return select
  }

  function textareaForFortune(index, key, label) {
    const textarea = document.createElement('textarea')
    textarea.rows = 3
    textarea.value = draft.fortunes[index][key]
    textarea.setAttribute('aria-label', label)
    textarea.addEventListener('input', () => {
      draft.fortunes[index][key] = textarea.value
      refreshStatusOnly()
    })
    return textarea
  }

  function duplicateFortune(index) {
    draft.fortunes.splice(index + 1, 0, { ...draft.fortunes[index] })
    markChanged()
  }

  function removeFortune(index) {
    draft.fortunes.splice(index, 1)
    markChanged()
  }

  function renderSpecialDates() {
    elements.specialDateTableBody.innerHTML = ''
    draft.special_dates.forEach((item, index) => {
      const row = document.createElement('tr')
      row.className = hasError(`special-${index}`) ? 'has-error' : ''

      row.appendChild(cell(inputForSpecialDate(index)))
      row.appendChild(cell(selectForSpecialFortune(index)))
      row.appendChild(cell(statusForSpecialDate(index)))
      row.appendChild(actionCell([
        actionButton('复制', () => duplicateSpecialDate(index)),
        actionButton('删除', () => removeSpecialDate(index), 'button--danger'),
      ], errorText(`special-${index}`)))

      elements.specialDateTableBody.appendChild(row)
    })

    if (draft.special_dates.length === 0) {
      elements.specialDateTableBody.appendChild(emptyRow(4, '没有特殊日期'))
    }
  }

  function inputForSpecialDate(index) {
    const input = document.createElement('input')
    input.type = 'text'
    input.placeholder = '05-04 或 2026-05-04'
    input.value = draft.special_dates[index].date
    input.setAttribute('aria-label', '特殊日期')
    input.addEventListener('input', () => {
      draft.special_dates[index].date = input.value.trim()
      markChanged()
    })
    return input
  }

  function selectForSpecialFortune(index) {
    const select = document.createElement('select')
    select.setAttribute('aria-label', '指定运势')
    const names = Array.from(new Set(draft.fortunes.map((fortune) => fortune.name).filter(Boolean)))
    if (!names.includes(draft.special_dates[index].fortune_name) && draft.special_dates[index].fortune_name) {
      names.push(draft.special_dates[index].fortune_name)
    }
    names.forEach((name) => {
      const option = document.createElement('option')
      option.value = name
      option.textContent = name
      select.appendChild(option)
    })
    select.value = draft.special_dates[index].fortune_name
    select.addEventListener('change', () => {
      draft.special_dates[index].fortune_name = select.value
      markChanged()
    })
    return select
  }

  function statusForSpecialDate(index) {
    const status = document.createElement('span')
    status.className = hasError(`special-${index}`) ? 'row-status is-error' : 'row-status'
    status.textContent = errorText(`special-${index}`) || '有效'
    return status
  }

  function duplicateSpecialDate(index) {
    draft.special_dates.splice(index + 1, 0, { ...draft.special_dates[index] })
    markChanged()
  }

  function removeSpecialDate(index) {
    draft.special_dates.splice(index, 1)
    markChanged()
  }

  function cell(child) {
    const td = document.createElement('td')
    td.appendChild(child)
    return td
  }

  function actionCell(buttons, message) {
    const td = document.createElement('td')
    const wrap = document.createElement('div')
    wrap.className = 'row-actions'
    buttons.forEach((button) => wrap.appendChild(button))
    td.appendChild(wrap)
    if (message) {
      const error = document.createElement('small')
      error.className = 'field-error'
      error.textContent = message
      td.appendChild(error)
    }
    return td
  }

  function actionButton(label, onClick, extraClass) {
    const button = document.createElement('button')
    button.type = 'button'
    button.className = `button button--small${extraClass ? ` ${extraClass}` : ''}`
    button.textContent = label
    button.addEventListener('click', onClick)
    return button
  }

  function emptyRow(colspan, message) {
    const row = document.createElement('tr')
    const td = document.createElement('td')
    td.colSpan = colspan
    td.className = 'empty-cell'
    td.textContent = message
    row.appendChild(td)
    return row
  }

  function hasError(scope) {
    return validation.errors.some((error) => error.scope === scope)
  }

  function errorText(scope) {
    const found = validation.errors.find((error) => error.scope === scope)
    return found ? found.message : ''
  }

  function renderFooter() {
    const dirty = isDirty()
    const hasErrors = validation.errors.length > 0
    elements.dirtyState.textContent = hasErrors ? '存在未修正问题' : dirty ? '有未保存更改' : '设置已同步'
    elements.dirtyState.classList.toggle('is-error', hasErrors)
    elements.saveButton.disabled = hasErrors || !dirty
  }

  function addChip(key, input) {
    const text = input.value.trim()
    if (!text) {
      return
    }
    draft[key] = normalizeStringList([...draft[key], text], [])
    input.value = ''
    markChanged()
  }

  function addFortune() {
    draft.fortunes.push({
      name: '大吉',
      stars: '★★★★★★★',
      sign: '',
      explanation: '',
    })
    markChanged()
  }

  function addSpecialDate() {
    const firstName = draft.fortunes[0] ? draft.fortunes[0].name : '大吉'
    draft.special_dates.push({ date: '', fortune_name: firstName })
    markChanged()
  }

  function saveSettings() {
    validation = validateDraft()
    if (validation.errors.length > 0) {
      render()
      setStatus(validation.errors[0].message, true)
      return
    }
    const values = buildPayloadFromDraft(draft)
    setStatus('正在保存设置')
    postMessage('settings.save', { values }, `save-${Date.now()}`)
  }

  function reloadSettings() {
    setStatus('正在重新读取设置')
    postMessage('settings.reload', undefined, `reload-${Date.now()}`)
  }

  function resetSettings() {
    applySettings(defaultSettings)
    setStatus('默认设置已载入，保存后生效')
  }

  function bindChipInput(input, key) {
    input.addEventListener('keydown', (event) => {
      if (event.key !== 'Enter') {
        return
      }
      event.preventDefault()
      addChip(key, input)
    })
  }

  function bindEvents() {
    bindChipInput(elements.fortuneTriggerInput, 'trigger_commands')
    bindChipInput(elements.statsTriggerInput, 'stats_trigger_commands')
    bindChipInput(elements.goodActionsInput, 'good_actions')
    bindChipInput(elements.badActionsInput, 'bad_actions')

    elements.timezoneInput.addEventListener('input', () => {
      draft.timezone = elements.timezoneInput.value.trim()
      markChanged()
    })
    elements.fortuneFilterInput.addEventListener('input', renderFortunes)
    elements.addFortuneButton.addEventListener('click', addFortune)
    elements.addSpecialDateButton.addEventListener('click', addSpecialDate)
    elements.reloadButton.addEventListener('click', reloadSettings)
    elements.resetButton.addEventListener('click', resetSettings)
    elements.saveButton.addEventListener('click', saveSettings)
  }

  window.addEventListener('message', (event) => {
    const message = event.data
    if (!message || message.version !== '1' || typeof message.type !== 'string') {
      return
    }

    if (message.type === 'host.init') {
      stopReadyLoop()
      const payload = message.payload || {}
      elements.pageTitle.textContent = payload.title || '运势设置'
      elements.pageSubtitle.textContent = payload.plugin && payload.plugin.description
        ? payload.plugin.description
        : '管理触发词、时区、特殊日期和运势库'
      defaultSettings = payload.default_config || {}
      applySettings(payload.settings || defaultSettings, { markSaved: true })
      setStatus('已载入设置')
      return
    }

    if (message.type === 'settings.changed') {
      const payload = message.payload || {}
      applySettings(payload.values || defaultSettings, { markSaved: true })
      setStatus('设置已保存')
      return
    }

    if (message.type === 'error') {
      const payload = message.payload || {}
      setStatus(payload.message || '操作未完成', true)
    }
  })

  bindEvents()
  render()
  announceReady()

  window.__fortuneSettingsPage = {
    buildPayload: () => buildPayloadFromDraft(draft),
    validate: validateDraft,
    readyAttempts: () => readyAttempts,
  }
})()
