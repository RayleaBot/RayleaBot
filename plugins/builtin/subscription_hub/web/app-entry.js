import { createBridgeClient } from './bridge-client.js'
import {
  buildRowsFromSettings,
  cloneRow,
  createBlankRow,
  normalizeSettings,
} from './model.js'
import { buildSettingsPayload } from './settings-payload.js'
import {
  normalizeServices,
  serviceTypes,
  servicesKey,
  trim,
  unique,
} from './services.js'
import { normalizePlatform, platformLabel } from './platforms.js'
import {
  buildIdentityRequests,
  collectSubscriberAvatars,
  identityKey,
  numericPattern,
} from './subscribers.js'
import {
  currentTargetsForMode,
  normalizeTargets,
  targetMap,
} from './targets.js'
import { validateRow, validateRows } from './validation.js'
import { selectorValue } from './render/html.js'
import { renderEmptyState } from './render/layout.js'
import { renderRowEdit } from './render/row-edit.js'
import { renderRowView } from './render/row-view.js'
import { renderServiceEditor } from './render/service-picker.js'
import { renderSubscriberChips } from './render/subscriber-editor.js'
import { renderRowValidation } from './render/status.js'
import { renderSelectedTargets, renderTargetOptions } from './render/target-picker.js'

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
  targets: {
    loaded: false,
    available: false,
    groups: [],
    private_users: [],
    issues: [],
  },
  identities: {
    subscriberAvatars: new Map(),
  },
  requests: {
    pending: new Map(),
    resolveTimers: new Map(),
    composingRows: new Set(),
    savingRequestId: '',
  },
  filters: {
    search: '',
    status: 'all',
    service: 'all',
  },
  ui: {
    loaded: false,
    dirty: false,
    rowCounter: 0,
  },
}

let bridge

const resolveDebounceMs = 700

function renderContext() {
  const liveTargets = targetMap(state.targets)
  return {
    targets: state.targets,
    targetMap: liveTargets,
    targetsLoaded: state.targets.loaded,
    subscriberAvatars: state.identities.subscriberAvatars,
  }
}

function nextRowId() {
  state.ui.rowCounter += 1
  return `row-${Date.now()}-${state.ui.rowCounter}`
}

function nextFrame(callback) {
  const schedule = window.requestAnimationFrame || ((fn) => window.setTimeout(fn, 0))
  schedule.call(window, callback)
}

function scrollRowIntoCenter(row) {
  nextFrame(() => {
    const element = elements.list.querySelector(`.sub-card[data-row-id="${selectorValue(row.row_id)}"]`)
    if (element && typeof element.scrollIntoView === 'function') {
      element.scrollIntoView({ block: 'center', behavior: 'smooth' })
    }
  })
}

function findRow(rowId) {
  return state.rows.find((row) => row.row_id === rowId)
}

function setStatus(text) {
  elements.statusText.textContent = text
}

function currentValidation() {
  return validateRows(state.rows, renderContext())
}

function renderPageState() {
  elements.enabledInput.checked = state.settings.enabled !== false
  elements.metricEnabled.textContent = state.settings.enabled === false ? '停用' : '启用'
  elements.metricSubscriptions.textContent = `${state.rows.length} / ${(state.settings.subscriptions || []).length}`
  elements.metricTargets.textContent = state.targets.loaded
    ? `${state.targets.groups.length} 群聊 / ${state.targets.private_users.length} 私聊`
    : '未载入'

  const validation = currentValidation()
  elements.metricValidation.textContent = validation.ok ? '可保存' : '需处理'
  elements.saveButton.disabled = !state.ui.loaded || !validation.ok || state.requests.savingRequestId !== ''
  elements.dirtyState.textContent = state.requests.savingRequestId
    ? '正在保存'
    : state.ui.dirty
      ? '设置有修改'
      : state.ui.loaded
        ? '设置已同步'
        : '等待载入'
}

function renderRow(row) {
  const context = renderContext()
  return row.edit_mode ? renderRowEdit(row, context) : renderRowView(row, context)
}

function render() {
  renderPageState()
  const visibleRows = state.rows.filter(rowVisible)
  elements.list.innerHTML = visibleRows.length
    ? visibleRows.map(renderRow).join('')
    : renderEmptyState()
}

function markDirty() {
  state.ui.dirty = true
  render()
}

function markDirtyWithRowRefresh(row, refresh) {
  state.ui.dirty = true
  refresh(row)
}

function rowSearchText(row) {
  const map = targetMap(state.targets)
  return [
    row.uid,
    row.name,
    row.query,
    ...row.targets.map((target) => `${target.target_id} ${target.target_name || ''}`),
    ...row.targets.map((target) => map.get(target.key)?.label || ''),
    ...row.subscriber_ids,
    platformLabel(row.platform),
  ].join(' ').toLowerCase()
}

function rowVisible(row) {
  const query = trim(state.filters.search).toLowerCase()
  if (query && !rowSearchText(row).includes(query)) {
    return false
  }
  if (state.filters.status === 'enabled' && !row.enabled) {
    return false
  }
  if (state.filters.status === 'disabled' && row.enabled) {
    return false
  }
  if (state.filters.service !== 'all') {
    const services = row.service_mode === 'mixed'
      ? row.targets.flatMap((target) => target.services)
      : row.services
    if (!services.includes('all') && !services.includes(state.filters.service)) {
      return false
    }
  }
  return true
}

function refreshValidationAndPage(card, row) {
  const validationSlot = card.querySelector('.row-validation-slot')
  if (validationSlot) {
    validationSlot.innerHTML = renderRowValidation(row, renderContext())
  }
  renderPageState()
}

function refreshRowTargetEditor(row) {
  const card = elements.list.querySelector(`.sub-card[data-row-id="${selectorValue(row.row_id)}"]`)
  if (!card) {
    render()
    return
  }

  const context = renderContext()
  const targetList = card.querySelector('.target-select')
  const targetScrollTop = targetList ? targetList.scrollTop : 0
  const activeKey = document.activeElement && document.activeElement.dataset
    ? document.activeElement.dataset.targetKey
    : ''

  const optionsList = card.querySelector('.target-options-list')
  if (optionsList) {
    optionsList.innerHTML = renderTargetOptions(row, context)
  }

  const targetChips = card.querySelector('.target-chip-list')
  if (targetChips) {
    targetChips.innerHTML = renderSelectedTargets(row, context)
  }

  const serviceSlot = card.querySelector('.service-editor-slot')
  if (serviceSlot) {
    serviceSlot.innerHTML = renderServiceEditor(row, context)
  }

  refreshValidationAndPage(card, row)
  if (targetList) {
    targetList.scrollTop = targetScrollTop
  }
  if (activeKey) {
    const activeOption = card.querySelector(`.target-option[data-target-key="${selectorValue(activeKey)}"]`)
    if (activeOption) {
      activeOption.focus({ preventScroll: true })
    }
  }
}

function refreshRowServiceEditor(row) {
  const card = elements.list.querySelector(`.sub-card[data-row-id="${selectorValue(row.row_id)}"]`)
  if (!card) {
    render()
    return
  }
  const active = document.activeElement
  const activeSelector = active && active.classList && active.classList.contains('service-checkbox')
    ? `.service-checkbox[data-row-id="${selectorValue(active.dataset.rowId)}"][data-target-key="${selectorValue(active.dataset.targetKey)}"][value="${selectorValue(active.value)}"]`
    : ''
  const serviceSlot = card.querySelector('.service-editor-slot')
  if (serviceSlot) {
    serviceSlot.innerHTML = renderServiceEditor(row, renderContext())
  }
  refreshValidationAndPage(card, row)
  if (activeSelector) {
    const nextActive = card.querySelector(activeSelector)
    if (nextActive) {
      nextActive.focus({ preventScroll: true })
    }
  }
}

function refreshRowSubscriberEditor(row) {
  const card = elements.list.querySelector(`.sub-card[data-row-id="${selectorValue(row.row_id)}"]`)
  if (!card) {
    render()
    return
  }
  const chips = card.querySelector('.subscriber-chip-list')
  if (chips) {
    chips.innerHTML = renderSubscriberChips(row, renderContext())
  }
  refreshValidationAndPage(card, row)
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
  const errors = validateRow(row, renderContext())
  if (errors.length) {
    setStatus(errors[0] || '行未通过校验')
    render()
    return
  }
  row._editSnapshot = null
  row.edit_mode = false
  markDirty()
}

function readCheckedServices(row, targetKeyValue, changedService, isChecked) {
  if (changedService === 'all') {
    return isChecked ? ['all'] : []
  }
  const types = serviceTypes(row.platform)
  const checkedServices = [...elements.list.querySelectorAll(`.service-checkbox[data-row-id="${selectorValue(row.row_id)}"][data-target-key="${selectorValue(targetKeyValue)}"]:checked`)]
    .map((input) => input.value)
    .filter((service) => service !== 'all')
  if (types.every((service) => checkedServices.includes(service))) {
    return ['all']
  }
  return types.filter((service) => checkedServices.includes(service))
}

function updateService(row, targetKeyValue, changedService, isChecked) {
  if (targetKeyValue === 'common') {
    row.service_mode = 'common'
    row.services = readCheckedServices(row, 'common', changedService, isChecked)
    for (const target of row.targets) {
      target.services = row.services
    }
    return
  }
  const target = row.targets.find((item) => item.key === targetKeyValue)
  if (target) {
    target.services = readCheckedServices(row, targetKeyValue, changedService, isChecked)
    const serviceKeys = unique(row.targets.map((item) => servicesKey(item.services, row.platform)))
    row.service_mode = serviceKeys.length > 1 ? 'mixed' : 'common'
    if (row.service_mode === 'common' && row.targets[0]) {
      row.services = row.targets[0].services
    }
  }
}

function requestTargets() {
  setStatus('正在刷新推送对象…')
  bridge.reloadTargets()
}

function clearResolveTimer(rowId) {
  clearTimeout(state.requests.resolveTimers.get(rowId))
  state.requests.resolveTimers.delete(rowId)
}

function clearPendingResolve(rowId) {
  for (const [requestId, request] of state.requests.pending) {
    if (request.kind === 'platform-user' && request.row_id === rowId) {
      state.requests.pending.delete(requestId)
    }
  }
}

function requestResolve(row, immediate) {
  requestPlatformResolve(row, immediate)
}

function requestPlatformResolve(row, immediate) {
  const platform = normalizePlatform(row.platform)
  const query = trim(row.query)
  if (!query) {
    clearResolveTimer(row.row_id)
    clearPendingResolve(row.row_id)
    row.resolved = false
    row.resolve_state = 'error'
    row.resolve_message = `请填写${platformLabel(platform)}对象。`
    render()
    return
  }
  const run = () => {
    state.requests.resolveTimers.delete(row.row_id)
    if (state.requests.composingRows.has(row.row_id)) {
      return
    }
    clearPendingResolve(row.row_id)
    const requestId = bridge.nextRequestId('third-party-user')
    state.requests.pending.set(requestId, { kind: 'platform-user', row_id: row.row_id, platform, query })
    row.resolve_state = 'checking'
    row.resolve_message = `正在校验${platformLabel(platform)}对象…`
    row.candidates = []
    row.resolved = false
    render()
    bridge.resolvePlatformUser(platform, query, requestId)
  }
  if (immediate) {
    clearResolveTimer(row.row_id)
    run()
    return
  }
  clearResolveTimer(row.row_id)
  state.requests.resolveTimers.set(row.row_id, setTimeout(run, resolveDebounceMs))
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

function applyPlatformResolved(message) {
  const request = state.requests.pending.get(message.request_id)
  if (!request || request.kind !== 'platform-user') {
    return
  }
  state.requests.pending.delete(message.request_id)
  const row = findRow(request.row_id)
  if (!row || request.platform !== normalizePlatform(message.payload.platform) || normalizePlatform(row.platform) !== request.platform || request.query !== message.payload.query) {
    return
  }
  if (message.payload.exact && message.payload.user) {
    applyResolvedUser(row, message.payload.user)
    row.resolve_message = `${platformLabel(row.platform)}对象已校验。`
  } else {
    row.resolved = false
    row.resolve_state = 'error'
    row.resolve_message = message.payload.message || `请选择一个候选${platformLabel(row.platform)}对象后保存。`
    row.candidates = Array.isArray(message.payload.candidates) ? message.payload.candidates : []
  }
  markDirty()
}

function applyBilibiliResolved(message) {
  applyPlatformResolved({
    ...message,
    payload: {
      platform: 'bilibili',
      ...(message.payload || {}),
    },
  })
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
    services: normalizeServices(services, row.platform),
  })
}

function updateTargetsFromSelect(row, selectedKeys) {
  const liveTargets = currentTargetsForMode(state.targets, row.target_mode)
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
  markDirtyWithRowRefresh(row, refreshRowSubscriberEditor)
}

function saveSettings() {
  const validation = currentValidation()
  if (!validation.ok) {
    setStatus(validation.errors[0] || '设置未通过校验')
    render()
    return
  }
  const identityRequests = buildIdentityRequests(state.rows)
  if (!identityRequests.length) {
    const payload = buildSettingsPayload(state.settings, state.rows, targetMap(state.targets))
    state.requests.savingRequestId = bridge.nextRequestId('settings-save')
    bridge.saveSettings(payload, state.requests.savingRequestId)
    render()
    return
  }
  const requestId = bridge.nextRequestId('protocol-identities')
  state.requests.pending.set(requestId, { kind: 'save-identities', expected: identityRequests })
  state.requests.savingRequestId = requestId
  setStatus('正在刷新订阅人身份…')
  bridge.resolveIdentities(identityRequests, requestId)
  render()
}

function applyIdentitiesResolved(message) {
  const request = state.requests.pending.get(message.request_id)
  if (!request || request.kind !== 'save-identities') {
    return
  }
  state.requests.pending.delete(message.request_id)
  state.requests.savingRequestId = ''
  const issues = Array.isArray(message.payload.issues) ? message.payload.issues : []
  const items = Array.isArray(message.payload.items) ? message.payload.items : []
  for (const item of items) {
    if (item.user_id && item.avatar_url) {
      state.identities.subscriberAvatars.set(trim(item.user_id), trim(item.avatar_url))
    }
  }
  const received = new Set(items.map((item) => identityKey(item.target_type, item.target_id, item.user_id)))
  const missing = request.expected.filter((item) => !received.has(identityKey(item.target_type, item.target_id, item.user_id)))
  if (issues.length || missing.length) {
    setStatus(issues[0] && issues[0].message ? issues[0].message : '订阅人身份刷新失败')
    render()
    return
  }
  const payload = buildSettingsPayload(state.settings, state.rows, targetMap(state.targets))
  state.requests.savingRequestId = bridge.nextRequestId('settings-save')
  bridge.saveSettings(payload, state.requests.savingRequestId)
  render()
}

function loadSettingsIntoRows(settings) {
  state.settings = settings
  state.rows = buildRowsFromSettings(settings)
  state.identities.subscriberAvatars = collectSubscriberAvatars(settings)
}

function applySettingsChanged(message) {
  loadSettingsIntoRows(normalizeSettings(message.payload && message.payload.values))
  state.ui.loaded = true
  state.ui.dirty = false
  state.requests.savingRequestId = ''
  setStatus('设置已同步')
  render()
}

function applyHostInit(payload) {
  state.defaultSettings = normalizeSettings(payload.default_config)
  loadSettingsIntoRows(normalizeSettings(payload.settings))
  state.ui.loaded = true
  state.ui.dirty = false
  setStatus('设置已载入')
  render()
  requestTargets()
}

function applyTargetsChanged(payload) {
  state.targets = normalizeTargets(payload)
  setStatus(state.targets.available ? '推送对象已刷新' : '推送对象不可用')
  render()
}

function handleBridgeMessage(message) {
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
    case 'thirdparty.user.resolved':
      applyPlatformResolved(message)
      return
    case 'bilibili.user.resolved':
      applyBilibiliResolved(message)
      return
    default:
      return
  }
}

function handleBridgeError(message) {
  const request = state.requests.pending.get(message.error.request_id)
  if (request && request.kind === 'platform-user') {
    state.requests.pending.delete(message.error.request_id)
    const row = findRow(request.row_id)
    if (row) {
      row.resolved = false
      row.resolve_state = 'error'
      row.resolve_message = message.error.message || `${platformLabel(row.platform)}对象校验失败。`
      row.candidates = []
      markDirty()
      return
    }
  }
  state.requests.savingRequestId = ''
  setStatus((message.error && message.error.message) || '操作失败')
  render()
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
    requestResolve(row, true)
    return
  }
  if (action === 'choose-candidate') {
    try {
      applyResolvedUser(row, JSON.parse(button.dataset.user || '{}'))
      row.resolve_message = `${platformLabel(row.platform)}对象已校验。`
      markDirty()
    } catch {
      setStatus('候选对象数据不正确')
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
    markDirtyWithRowRefresh(row, refreshRowTargetEditor)
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
    markDirtyWithRowRefresh(row, refreshRowSubscriberEditor)
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
    clearResolveTimer(row.row_id)
    clearPendingResolve(row.row_id)
    state.requests.composingRows.delete(row.row_id)
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
    state.ui.dirty = true
    clearResolveTimer(row.row_id)
    if (event.isComposing || state.requests.composingRows.has(row.row_id)) {
      return
    }
    requestResolve(row, false)
  }
}

function handleListCompositionStart(event) {
  const input = event.target
  if (!input.classList.contains('up-query-input')) {
    return
  }
  state.requests.composingRows.add(input.dataset.rowId)
  clearResolveTimer(input.dataset.rowId)
}

function handleListCompositionEnd(event) {
  const input = event.target
  if (!input.classList.contains('up-query-input')) {
    return
  }
  state.requests.composingRows.delete(input.dataset.rowId)
  const row = findRow(input.dataset.rowId)
  if (!row) {
    return
  }
  row.query = input.value
  row.resolved = false
  row.resolve_state = 'idle'
  row.resolve_message = ''
  row.candidates = []
  state.ui.dirty = true
  requestResolve(row, false)
}

function handleListChange(event) {
  const input = event.target
  const row = findRow(input.dataset.rowId)
  if (!row) {
    return
  }
  if (input.classList.contains('service-checkbox')) {
    updateService(row, input.dataset.targetKey, input.value, input.checked)
    markDirtyWithRowRefresh(row, refreshRowServiceEditor)
    return
  }
  if (input.classList.contains('platform-select')) {
    clearResolveTimer(row.row_id)
    clearPendingResolve(row.row_id)
    state.requests.composingRows.delete(row.row_id)
    row.platform = normalizePlatform(input.value)
    row.uid = ''
    row.name = ''
    row.avatar_url = ''
    row.query = ''
    row.resolved = false
    row.resolve_state = 'idle'
    row.resolve_message = ''
    row.candidates = []
    row.services = ['all']
    for (const target of row.targets) {
      target.services = ['all']
    }
    row.service_mode = 'common'
    markDirty()
    return
  }
  if (input.classList.contains('row-enabled-input')) {
    row.enabled = input.checked
    markDirty()
  }
}

function resetToDefault() {
  loadSettingsIntoRows(normalizeSettings(state.defaultSettings))
  state.ui.dirty = true
  setStatus('已恢复默认设置，保存后生效')
  render()
}

function bindEvents() {
  elements.enabledInput.addEventListener('change', () => {
    state.settings.enabled = elements.enabledInput.checked
    markDirty()
  })
  elements.targetsReloadButton.addEventListener('click', requestTargets)
  elements.searchInput.addEventListener('input', () => {
    state.filters.search = elements.searchInput.value
    render()
  })
  elements.statusFilter.addEventListener('change', () => {
    state.filters.status = elements.statusFilter.value
    render()
  })
  elements.serviceFilter.addEventListener('change', () => {
    state.filters.service = elements.serviceFilter.value
    render()
  })
  elements.addButton.addEventListener('click', () => {
    const newRow = createBlankRow(nextRowId())
    state.rows.unshift(newRow)
    markDirty()
    scrollRowIntoCenter(newRow)
  })
  elements.list.addEventListener('click', handleListClick)
  elements.list.addEventListener('input', handleListInput)
  elements.list.addEventListener('compositionstart', handleListCompositionStart)
  elements.list.addEventListener('compositionend', handleListCompositionEnd)
  elements.list.addEventListener('change', handleListChange)
  elements.reloadButton.addEventListener('click', () => {
    setStatus('正在重新载入设置…')
    bridge.reloadSettings()
  })
  elements.resetButton.addEventListener('click', resetToDefault)
  elements.manualCheckButton.addEventListener('click', () => {
    setStatus('Bilibili 事件源状态在 Web 三方监控页面查看')
  })
  elements.previewButton.addEventListener('click', () => {
    bridge.openRenderTemplate('plugin.raylea.subscription-hub.bilibili-update')
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

bridge = createBridgeClient(window, {
  onMessage: handleBridgeMessage,
  onError: handleBridgeError,
})

bindEvents()
render()
bridge.pageReady()

window.__subscriptionHubSettingsPage = {
  state,
  normalizeSettings,
  buildRowsFromSettings,
  buildSettingsPayload: (identityItems = []) => {
    void identityItems
    return buildSettingsPayload(state.settings, state.rows, targetMap(state.targets))
  },
  validateRows: () => validateRows(state.rows, renderContext()),
}
