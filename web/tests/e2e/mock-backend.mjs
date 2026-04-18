import http from 'node:http'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { readFile } from 'node:fs/promises'

import YAML from 'yaml'
import { WebSocketServer } from 'ws'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, '..', '..', '..')
const previewArtifactBytes = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO2W4n8AAAAASUVORK5CYII=',
  'base64',
)

async function readFixture(relativePath) {
  const absolutePath = path.join(repoRoot, relativePath)
  const raw = await readFile(absolutePath, 'utf8')
  if (relativePath.endsWith('.json')) {
    return JSON.parse(raw)
  }
  return YAML.parse(raw)
}

const fixtures = {
  healthz: await readFixture('fixtures/web-api/ok.healthz-response.yaml'),
  readyz: await readFixture('fixtures/web-api/edge.readyz-degraded-response.yaml'),
  setupAdmin: await readFixture('fixtures/web-api/ok.setup-admin.yaml'),
  setupAdminDenied: await readFixture('fixtures/web-api/edge.setup-admin-already-initialized.yaml'),
  setupStatus: await readFixture('fixtures/web-api/ok.setup-status.yaml'),
  sessionLogin: await readFixture('fixtures/web-api/ok.session-login.yaml'),
  sessionLauncherToken: await readFixture('fixtures/web-api/ok.session-launcher-token.yaml'),
  sessionLauncherAdmission: await readFixture('fixtures/web-api/ok.session-launcher-admission.yaml'),
  sessionDenied: await readFixture('fixtures/web-api/invalid.session-login-bad-credentials.yaml'),
  configGet: await readFixture('fixtures/web-api/ok.config-get-response.yaml'),
  protocolSnapshot: await readFixture('fixtures/web-api/ok.protocol-onebot11-snapshot.yaml'),
  protocolCompatibility: await readFixture('fixtures/web-api/ok.protocol-onebot11-compatibility.yaml'),
  logsList: await readFixture('fixtures/web-api/ok.logs-list-response.yaml'),
  logDetail: await readFixture('fixtures/web-api/ok.log-detail-response.yaml'),
  logDetailLegacy: await readFixture('fixtures/web-api/edge.log-detail-legacy-empty-details.yaml'),
  logDetailNotFound: await readFixture('fixtures/web-api/edge.log-detail-not-found.yaml'),
  tasksList: await readFixture('fixtures/web-api/ok.tasks-list-response.yaml'),
  taskDetail: await readFixture('fixtures/web-api/ok.task-detail-response.yaml'),
  taskDetailSucceededInstall: await readFixture('fixtures/web-api/ok.task-detail-succeeded-install.yaml'),
  taskDetailSucceededRenderPreview: await readFixture('fixtures/web-api/ok.task-detail-succeeded-render-preview.yaml'),
  taskDetailFailedInstallScriptBlocked: await readFixture('fixtures/web-api/edge.task-detail-failed-install-script-blocked.yaml'),
  taskCancel: await readFixture('fixtures/web-api/ok.task-cancel-accepted.yaml'),
  systemStatus: await readFixture('fixtures/web-api/ok.system-status.yaml'),
  systemShutdown: await readFixture('fixtures/web-api/ok.system-shutdown.yaml'),
  systemBackupAccepted: await readFixture('fixtures/web-api/ok.system-backup-accepted.yaml'),
  systemRenderPreviewAccepted: await readFixture('fixtures/web-api/ok.system-render-preview-accepted.yaml'),
  systemDiagnosticsExport: await readFixture('fixtures/web-api/ok.system-diagnostics-export.yaml'),
  pluginEnable: await readFixture('fixtures/web-api/ok.plugins-enable-response.yaml'),
  pluginDisable: await readFixture('fixtures/web-api/edge.plugins-disable-response.yaml'),
  pluginReload: await readFixture('fixtures/web-api/ok.plugins-reload-response.yaml'),
  pluginInstallAccepted: await readFixture('fixtures/web-api/ok.plugins-install-accepted.yaml'),
  pluginInstallAcceptedWithScripts: await readFixture('fixtures/web-api/ok.plugins-install-accepted-with-scripts.yaml'),
  pluginInstallRemoteUrl: await readFixture('fixtures/web-api/ok.plugins-install-remote-url.yaml'),
  pluginList: await readFixture('fixtures/web-api/ok.plugins-list-response.yaml'),
  pluginDetail: await readFixture('fixtures/web-api/ok.plugin-detail-response.yaml'),
  pluginUninstallAccepted: await readFixture('fixtures/web-api/ok.plugins-uninstall-accepted.yaml'),
  pluginGrantsList: await readFixture('fixtures/web-api/ok.plugins-grants-list-response.yaml'),
  pluginGrant: await readFixture('fixtures/web-api/ok.plugins-grant-response.yaml'),
  pluginGrantWithExpiry: await readFixture('fixtures/web-api/ok.plugins-grant-with-expiry-response.yaml'),
  invalidGrantExpiry: await readFixture('fixtures/web-api/invalid.plugins-grant-invalid-expires-at.yaml'),
  invalidUninstallNotFound: await readFixture('fixtures/web-api/invalid.plugins-uninstall-not-found.yaml'),
  wsLogs: await readFixture('fixtures/websocket/ok.logs-appended.protocol-onebot11.json'),
  wsTasks: await readFixture('fixtures/websocket/ok.tasks-updated-running.json'),
  wsEvents: await readFixture('fixtures/websocket/edge.events-received-degraded.json'),
  wsEventsProtocolSnapshot: await readFixture('fixtures/websocket/ok.events-received-protocol-snapshot.json'),
  wsConsole: await readFixture('fixtures/websocket/ok.plugins-console-stderr.json'),
  wsSessionExpired: await readFixture('fixtures/websocket/edge.session-expired.json'),
}

const sockets = {
  events: new Set(),
  tasks: new Set(),
  logs: new Set(),
  plugin_console: new Set(),
}

function baseState() {
  const initialLogs = structuredClone(fixtures.logsList.response.body.items)
  const pluginItems = structuredClone(fixtures.pluginList.response.body.items)
  const pluginMap = Object.fromEntries(pluginItems.map((item) => [item.id, item]))
  pluginMap.weather = structuredClone(fixtures.pluginDetail.response.body.plugin)
  return {
    initialized: false,
    token: null,
    plugins: pluginMap,
    tasks: structuredClone(fixtures.tasksList.response.body.items),
    logs: initialLogs,
    currentSessionLogIds: new Set(initialLogs.map((item) => item.log_id)),
    logDetails: createLogDetailMap(),
    config: structuredClone(fixtures.configGet.response.body.config),
    protocolSnapshot: structuredClone(fixtures.protocolSnapshot.response.body),
    grants: {
      weather: structuredClone(fixtures.pluginGrantsList.response.body.items),
      'builtin-help': [],
    },
    launcherTokens: new Set(['launcher_token_fixture_0001']),
    systemStatus: structuredClone(fixtures.systemStatus.response.body),
    failures: {
      failPluginsListOnce: false,
      failPluginDetailOnce: false,
      failLogsOnce: false,
      failSystemStatusOnce: false,
      failUninstallOnce: false,
    },
  }
}

function computeProtocolSnapshotFromConfig(config, currentSnapshot) {
  const snapshot = structuredClone(currentSnapshot)
  const onebot = config.onebot ?? {}
  const reverseWs = onebot.reverse_ws ?? { enabled: false, url: '' }
  const forwardWs = onebot.forward_ws ?? { enabled: false, url: '' }
  const httpApi = onebot.http_api ?? { enabled: false, url: '' }
  const webhook = onebot.webhook ?? { enabled: false, url: '' }
  const transports = [
    ['reverse_ws', reverseWs],
    ['forward_ws', forwardWs],
    ['http_api', httpApi],
    ['webhook', webhook],
  ]

  snapshot.provider = onebot.provider ?? 'standard'
  snapshot.transport_status = transports.map(([transport, entry]) => {
    const configured = Boolean(entry.url)
    let state = 'idle'
    let summary = '未启用'

    if (entry.enabled && configured) {
      if (transport === 'forward_ws') {
        state = 'connected'
        summary = '主动连接已建立'
      } else if (transport === 'reverse_ws') {
        state = 'listening'
        summary = '等待 OneBot 回连'
      } else if (transport === 'http_api') {
        state = 'connected'
        summary = 'HTTP API 可用'
      } else if (transport === 'webhook') {
        state = 'listening'
        summary = 'Webhook 入口可接收上报'
      }
    }

    return {
      transport,
      enabled: Boolean(entry.enabled),
      configured,
      endpoint: entry.url ? entry.url.replace(/^(https?:\/\/[^/]+|wss?:\/\/[^/]+).*$/, '$1') : '',
      state,
      summary,
    }
  })
  snapshot.configured_transports = transports
    .filter(([, entry]) => Boolean(entry.url))
    .map(([name]) => name)

  if (forwardWs.enabled && forwardWs.url) {
    snapshot.active_transports = ['forward_ws']
    snapshot.readiness_status = 'ready'
    snapshot.summary = 'OneBot11 主动连接已就绪'
  } else if (reverseWs.enabled && reverseWs.url) {
    snapshot.active_transports = ['reverse_ws']
    snapshot.readiness_status = 'degraded'
    snapshot.summary = 'OneBot11 等待回连'
  } else if (httpApi.enabled && httpApi.url && webhook.enabled && webhook.url) {
    snapshot.active_transports = ['http_api', 'webhook']
    snapshot.readiness_status = 'ready'
    snapshot.summary = 'OneBot11 HTTP API 与 Webhook 已就绪'
  } else if (httpApi.enabled && httpApi.url) {
    snapshot.active_transports = ['http_api']
    snapshot.readiness_status = 'degraded'
    snapshot.summary = 'OneBot11 仅 HTTP API 可用'
  } else if (webhook.enabled && webhook.url) {
    snapshot.active_transports = ['webhook']
    snapshot.readiness_status = 'degraded'
    snapshot.summary = 'OneBot11 仅 Webhook 上报可用'
  } else {
    snapshot.active_transports = []
    snapshot.readiness_status = 'setup_required'
    snapshot.summary = 'OneBot11 尚未配置连接'
  }
  return snapshot
}

function normalizeTransport(entry = {}) {
  return {
    enabled: Boolean(entry.enabled),
    url: String(entry.url ?? ''),
  }
}

function pickOneBotHotState(config) {
  const onebot = config.onebot ?? {}
  const adapter = config.adapter ?? {}

  return {
    adapter: {
      connect_timeout_seconds: adapter.connect_timeout_seconds ?? 0,
      reconnect_initial_seconds: adapter.reconnect_initial_seconds ?? 0,
      reconnect_multiplier: adapter.reconnect_multiplier ?? 0,
      reconnect_max_seconds: adapter.reconnect_max_seconds ?? 0,
      reconnect_jitter_ratio: adapter.reconnect_jitter_ratio ?? 0,
    },
    onebot: {
      provider: onebot.provider ?? 'standard',
      access_token: onebot.access_token ?? '',
      reverse_ws: normalizeTransport(onebot.reverse_ws),
      forward_ws: normalizeTransport(onebot.forward_ws),
      http_api: normalizeTransport(onebot.http_api),
      webhook: normalizeTransport(onebot.webhook),
    },
  }
}

function computeRestartRequiredForConfig(prevConfig, nextConfig) {
  return computeConfigApplyEffects(prevConfig, nextConfig).restart_required_fields.length > 0
}

const configRestartRequiredFields = new Set([
  'admin.max_sessions',
  'admin.session_ttl_days',
  'admin.sliding_renewal',
  'database.engine',
  'database.path',
  'render.browser_args',
  'render.browser_path',
  'render.worker_count',
  'server.host',
  'server.port',
  'web.exposure_mode',
  'web.setup_local_only',
])

function computeConfigApplyEffects(prevConfig, nextConfig) {
  const changedPaths = []
  collectChangedConfigPaths('', prevConfig ?? {}, nextConfig ?? {}, changedPaths)
  changedPaths.sort()

  const effects = {
    applied_now: [],
    reloaded_now: [],
    restart_required_fields: [],
  }

  for (const path of [...new Set(changedPaths)]) {
    if (path.startsWith('onebot.') || path.startsWith('adapter.')) {
      effects.reloaded_now.push(path)
    } else if (configRestartRequiredFields.has(path) || path.startsWith('database.') || path.startsWith('server.') || path.startsWith('web.')) {
      effects.restart_required_fields.push(path)
    } else {
      effects.applied_now.push(path)
    }
  }

  return effects
}

function collectChangedConfigPaths(prefix, prevValue, nextValue, changedPaths) {
  if (isPlainObject(prevValue) && isPlainObject(nextValue)) {
    const keys = [...new Set([...Object.keys(prevValue), ...Object.keys(nextValue)])].sort()
    for (const key of keys) {
      collectChangedConfigPaths(prefix ? `${prefix}.${key}` : key, prevValue[key], nextValue[key], changedPaths)
    }
    return
  }

  if (prefix && JSON.stringify(prevValue) !== JSON.stringify(nextValue)) {
    changedPaths.push(prefix)
  }
}

function isPlainObject(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function createLogDetailMap() {
  return {
    [fixtures.logDetail.response.body.log_id]: structuredClone(fixtures.logDetail.response.body),
    [fixtures.logDetailLegacy.response.body.log_id]: structuredClone(fixtures.logDetailLegacy.response.body),
    log_runtime_0001: {
      log_id: 'log_runtime_0001',
      timestamp: '2026-03-20T10:00:00Z',
      level: 'error',
      source: 'runtime',
      message: 'plugin runtime stderr truncated',
      plugin_id: 'weather',
      request_id: 'req_plugin_0001',
      details: {
        direction: 'internal',
        reason: 'stderr exceeded preview limit',
        payload_preview: {
          plugin_id: 'weather',
          stream: 'stderr',
          line_preview: 'Traceback (most recent call last): ...',
        },
      },
    },
    log_adapter_0001: {
      log_id: 'log_adapter_0001',
      timestamp: '2026-03-20T10:00:01Z',
      level: 'error',
      source: 'adapter.onebot11',
      protocol: 'onebot11',
      message: 'reverse websocket connection lost',
      details: {
        direction: 'inbound',
        frame_type: 'socket.close',
        reason: 'reverse websocket connection lost',
      },
    },
  }
}

let state = baseState()

function json(response, status, body) {
  response.writeHead(status, { 'Content-Type': 'application/json' })
  response.end(JSON.stringify(body))
}

function noContent(response) {
  response.writeHead(204)
  response.end()
}

function parseBody(request) {
  return new Promise((resolve, reject) => {
    const chunks = []
    request.on('data', (chunk) => chunks.push(chunk))
    request.on('end', () => {
      if (chunks.length === 0) {
        resolve({})
        return
      }

      try {
        resolve(JSON.parse(Buffer.concat(chunks).toString('utf8')))
      } catch (error) {
        reject(error)
      }
    })
    request.on('error', reject)
  })
}

function requestUrl(request) {
  return new URL(request.url ?? '/', 'http://127.0.0.1:4010')
}

function authToken(request) {
  const header = request.headers.authorization ?? ''
  return header.startsWith('Bearer ') ? header.slice('Bearer '.length) : null
}

function requireAuth(request, response) {
  if (authToken(request) && authToken(request) === state.token) {
    return true
  }

  json(response, 401, {
    error: {
      code: 'permission.denied',
      message: '需要有效的管理会话',
      message_key: 'errors.permission.denied',
      request_id: 'req_auth_missing_fixture',
    },
  })
  return false
}

function broadcast(channel, frame) {
  for (const socket of sockets[channel]) {
    if (socket.readyState === 1) {
      socket.send(JSON.stringify(frame))
    }
  }
}

function resetState(payload = {}) {
  for (const channel of Object.keys(sockets)) {
    for (const socket of sockets[channel]) {
      socket.close()
    }
    sockets[channel].clear()
  }

  state = baseState()
  state.initialized = Boolean(payload.initialized)
  state.token = null
  state.failures = {
    ...state.failures,
    ...(payload.failures ?? {}),
  }
}

function takeFailureFlag(name) {
  if (!state.failures[name]) {
    return false
  }

  state.failures[name] = false
  return true
}

function sessionExpiredFrame(channel = 'events') {
  return {
    ...fixtures.wsSessionExpired.frame,
    channel,
  }
}

function pluginListBody() {
  return {
    items: Object.values(state.plugins),
  }
}

function pluginDetailBody(pluginId) {
  return {
    plugin: state.plugins[pluginId],
  }
}

function updatePluginPermission(pluginId, capability, nextState) {
  const plugin = state.plugins[pluginId]
  if (!plugin?.permissions) {
    return
  }

  plugin.permissions = plugin.permissions.map((permission) => (
    permission.capability === capability
      ? {
        ...permission,
        ...nextState,
      }
      : permission
  ))
}

function taskSummary(taskId, taskType, summary) {
  return {
    task_id: taskId,
    task_type: taskType,
    status: 'pending',
    summary,
  }
}

function appendLogSummary(summary, detail, options = {}) {
  state.logs = [
    ...state.logs.filter((item) => item.log_id !== summary.log_id),
    structuredClone(summary),
  ]

  if (options.currentSession !== false) {
    state.currentSessionLogIds.add(summary.log_id)
  }

  if (detail) {
    state.logDetails[summary.log_id] = structuredClone(detail)
  }
}

function normalizeSortableTimestamp(value) {
  if (value === null || value === undefined) {
    return Number.NEGATIVE_INFINITY
  }

  if (typeof value === 'number' && Number.isFinite(value)) {
    return normalizeUnixTimestamp(value)
  }

  const raw = String(value).trim()
  if (!raw) {
    return Number.NEGATIVE_INFINITY
  }

  const numeric = Number(raw)
  if (Number.isFinite(numeric)) {
    return normalizeUnixTimestamp(numeric)
  }

  const parsed = Date.parse(raw)
  return Number.isFinite(parsed) ? parsed : Number.NEGATIVE_INFINITY
}

function normalizeUnixTimestamp(value) {
  const absolute = Math.abs(value)
  if (absolute >= 1_000_000_000 && absolute < 1_000_000_000_000) {
    return value * 1000
  }
  return value
}

function compareLogsDesc(left, right) {
  const leftTimestamp = normalizeSortableTimestamp(left.timestamp)
  const rightTimestamp = normalizeSortableTimestamp(right.timestamp)

  if (leftTimestamp !== rightTimestamp) {
    return rightTimestamp - leftTimestamp
  }

  return String(right.log_id ?? '').localeCompare(String(left.log_id ?? ''))
}

function encodeLogCursor(item) {
  return Buffer.from(JSON.stringify({
    log_id: item.log_id,
  }), 'utf8').toString('base64url')
}

function decodeLogCursor(raw) {
  if (!raw) {
    return null
  }

  try {
    const decoded = JSON.parse(Buffer.from(raw, 'base64url').toString('utf8'))
    return typeof decoded?.log_id === 'string' ? decoded.log_id : null
  } catch {
    return null
  }
}

function listLogPage(searchParams) {
  const scope = searchParams.get('scope') === 'current_session' ? 'current_session' : 'history'
  const startAt = searchParams.get('start_at')
  const endAt = searchParams.get('end_at')
  const level = searchParams.get('level')
  const source = searchParams.get('source')
  const protocol = searchParams.get('protocol')
  const pluginId = searchParams.get('plugin_id')
  const requestId = searchParams.get('request_id')
  const limit = Math.max(1, Number(searchParams.get('limit') ?? '50') || 50)
  const direction = searchParams.get('direction') === 'newer' ? 'newer' : 'older'
  const cursorLogId = decodeLogCursor(searchParams.get('cursor'))

  const filtered = state.logs
    .filter((item) => {
      const timestamp = normalizeSortableTimestamp(item.timestamp)
      if (scope === 'current_session' && !state.currentSessionLogIds.has(item.log_id)) return false
      if (scope === 'history' && startAt && timestamp < normalizeSortableTimestamp(startAt)) return false
      if (scope === 'history' && endAt && timestamp > normalizeSortableTimestamp(endAt)) return false
      if (level && item.level !== level) return false
      if (source && item.source !== source) return false
      if (protocol && item.protocol !== protocol) return false
      if (pluginId && item.plugin_id !== pluginId) return false
      if (requestId && item.request_id !== requestId) return false
      return true
    })
    .slice()
    .sort(compareLogsDesc)

  let startIndex = 0
  let endIndex = Math.min(limit, filtered.length)
  const cursorIndex = cursorLogId
    ? filtered.findIndex((item) => item.log_id === cursorLogId)
    : -1

  if (cursorIndex >= 0) {
    if (direction === 'older') {
      startIndex = cursorIndex + 1
      endIndex = Math.min(filtered.length, startIndex + limit)
    } else {
      endIndex = cursorIndex
      startIndex = Math.max(0, endIndex - limit)
    }
  }

  const items = filtered.slice(startIndex, endIndex)
  const hasNewer = startIndex > 0
  const hasOlder = endIndex < filtered.length

  return {
    items,
    page: {
      limit,
      has_older: hasOlder,
      has_newer: hasNewer,
      older_cursor: hasOlder && items.length > 0 ? encodeLogCursor(items.at(-1)) : null,
      newer_cursor: hasNewer && items.length > 0 ? encodeLogCursor(items[0]) : null,
    },
  }
}

function defaultProtocolLiveLog() {
  const summary = {
    log_id: 'log_adapter_live_0001',
    timestamp: '2026-04-08T10:16:00Z',
    level: 'warn',
    source: 'adapter.onebot11',
    protocol: 'onebot11',
    message: 'ignored OneBot API response with unsupported echo',
    request_id: 'req_adapter_ignored_0001',
  }

  return {
    summary,
    detail: {
      ...summary,
      details: {
        direction: 'inbound',
        frame_type: 'api.response.ignored',
        reason: 'api response echo must be a non-empty string',
        echo_value_type: 'number',
        payload_preview: {
          status: 'ok',
          retcode: 0,
          echo: 123,
          wording: 'ignored by adapter',
        },
      },
    },
  }
}

function errorEnvelope(code, message, requestId, details) {
  return {
    error: {
      code,
      message,
      message_key: `errors.${code}`,
      request_id: requestId,
      ...(details ? { details } : {}),
    },
  }
}

const server = http.createServer(async (request, response) => {
  const url = requestUrl(request)
  const { pathname, searchParams } = url

  if (pathname === '/__test/ping') {
    json(response, 200, { ok: true })
    return
  }

  if (pathname === '/__test/reset' && request.method === 'POST') {
    const payload = await parseBody(request)
    resetState(payload)
    json(response, 200, { ok: true, initialized: state.initialized })
    return
  }

  if (pathname === '/__test/session-expire' && request.method === 'POST') {
    state.token = null
    for (const channel of Object.keys(sockets)) {
      for (const socket of sockets[channel]) {
        socket.send(JSON.stringify(sessionExpiredFrame(channel)))
        socket.close()
      }
    }
    json(response, 200, { ok: true })
    return
  }

  if (pathname === '/__test/socket-close' && request.method === 'POST') {
    const payload = await parseBody(request)
    const channel = payload.channel
    if (!channel || !sockets[channel]) {
      json(response, 400, errorEnvelope('platform.invalid_request', 'invalid socket channel', 'req_socket_close_invalid'))
      return
    }

    for (const socket of sockets[channel]) {
      socket.close()
    }

    json(response, 200, { ok: true, channel })
    return
  }

  if (pathname === '/__test/push-log' && request.method === 'POST') {
    const payload = await parseBody(request)
    const seed = defaultProtocolLiveLog()
    const summary = {
      ...seed.summary,
      ...(payload.summary ?? payload),
      log_id: (payload.summary?.log_id ?? payload.log_id ?? seed.summary.log_id),
      timestamp: (payload.summary?.timestamp ?? payload.timestamp ?? new Date().toISOString()),
    }
    const detail = {
      ...seed.detail,
      ...(payload.detail ?? {}),
      ...summary,
      details: structuredClone(payload.detail?.details ?? payload.details ?? seed.detail.details),
    }

    appendLogSummary(summary, detail, {
      currentSession: payload.scope !== 'history',
    })
    broadcast('logs', {
      channel: 'logs',
      type: 'logs.appended',
      timestamp: summary.timestamp,
      data: summary,
    })
    json(response, 200, { ok: true, log_id: summary.log_id })
    return
  }

  if (pathname === '/healthz' && request.method === 'GET') {
    json(response, fixtures.healthz.response.status, fixtures.healthz.response.body)
    return
  }

  if (pathname === '/readyz' && request.method === 'GET') {
    json(response, fixtures.readyz.response.status, fixtures.readyz.response.body)
    return
  }

  if (pathname === '/api/setup/status' && request.method === 'GET') {
    json(response, fixtures.setupStatus.response.status, {
      initialized: state.initialized,
    })
    return
  }

  if (pathname === '/api/setup/admin' && request.method === 'POST') {
    if (state.initialized) {
      json(response, fixtures.setupAdminDenied.response.status, fixtures.setupAdminDenied.response.body)
      return
    }

    const payload = await parseBody(request)
    if (!payload.identifier || !payload.secret) {
      json(response, 400, {
        error: {
          code: 'platform.invalid_request',
          message: '缺少初始化字段',
          message_key: 'errors.platform.invalid_request',
          request_id: 'req_setup_admin_invalid',
        },
      })
      return
    }

    state.initialized = true
    state.token = fixtures.setupAdmin.response.body.session_token
    json(response, fixtures.setupAdmin.response.status, fixtures.setupAdmin.response.body)
    return
  }

  if (pathname === '/api/session/login' && request.method === 'POST') {
    const payload = await parseBody(request)
    if (!state.initialized || payload.identifier !== 'admin' || payload.secret !== 'fixture-only-secret') {
      json(response, fixtures.sessionDenied.response.status, fixtures.sessionDenied.response.body)
      return
    }

    state.token = fixtures.sessionLogin.response.body.session_token
    json(response, fixtures.sessionLogin.response.status, fixtures.sessionLogin.response.body)
    return
  }

  if (pathname === '/api/session/launcher-token' && request.method === 'POST') {
    if (!state.initialized) {
      json(response, 403, errorEnvelope('permission.denied', '当前用户无权执行该操作', 'req_launcher_token_denied'))
      return
    }

    state.launcherTokens.add(fixtures.sessionLauncherToken.response.body.launcher_token)
    json(response, fixtures.sessionLauncherToken.response.status, fixtures.sessionLauncherToken.response.body)
    return
  }

  if (pathname === '/api/session/launcher-admission' && request.method === 'POST') {
    if (!state.initialized) {
      json(response, 403, errorEnvelope('permission.denied', '当前用户无权执行该操作', 'req_launcher_admission_forbidden'))
      return
    }

    const payload = await parseBody(request)
    if (!payload.launcher_token) {
      json(response, 400, errorEnvelope('platform.invalid_request', '缺少 launcher_token', 'req_launcher_admission_invalid'))
      return
    }
    if (!state.launcherTokens.has(payload.launcher_token)) {
      json(response, 401, errorEnvelope('permission.denied', '当前用户无权执行该操作', 'req_launcher_admission_denied'))
      return
    }

    state.launcherTokens.delete(payload.launcher_token)
    state.token = fixtures.sessionLauncherAdmission.response.body.session_token
    json(response, fixtures.sessionLauncherAdmission.response.status, fixtures.sessionLauncherAdmission.response.body)
    return
  }

  if (pathname === '/api/session' && request.method === 'DELETE') {
    if (!requireAuth(request, response)) {
      return
    }

    state.token = null
    noContent(response)
    return
  }

  if (pathname === '/api/system/status' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    if (takeFailureFlag('failSystemStatusOnce')) {
      json(response, 500, errorEnvelope('plugin.internal_error', 'system status failed', 'req_system_status_failed'))
      return
    }

    json(response, fixtures.systemStatus.response.status, state.systemStatus)
    return
  }

  if (pathname === '/api/system/shutdown' && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    state.systemStatus.status = 'shutting_down'
    json(response, fixtures.systemShutdown.response.status, fixtures.systemShutdown.response.body)
    return
  }

  if (pathname === '/api/system/backup' && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const taskId = fixtures.systemBackupAccepted.response.body.task_id
    state.tasks = [
      taskSummary(taskId, 'backup.create', 'create online backup'),
      ...state.tasks.filter((item) => item.task_id !== taskId),
    ]

    json(response, fixtures.systemBackupAccepted.response.status, fixtures.systemBackupAccepted.response.body)
    return
  }

  if (pathname === '/api/system/diagnostics/export' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    response.writeHead(fixtures.systemDiagnosticsExport.response.status, {
      'Content-Type': fixtures.systemDiagnosticsExport.response.headers['Content-Type'],
      'Content-Disposition': fixtures.systemDiagnosticsExport.response.headers['Content-Disposition'],
    })
    response.end(Buffer.from('PK\x03\x04fixture-diagnostics'))
    return
  }

  if (pathname === '/api/system/render/preview' && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const payload = await parseBody(request)
    const taskId = fixtures.systemRenderPreviewAccepted.response.body.task_id
    state.tasks = [
      taskSummary(taskId, 'render.preview', `render preview for ${payload.template ?? 'unknown-template'}`),
      ...state.tasks.filter((item) => item.task_id !== taskId),
    ]

    json(response, fixtures.systemRenderPreviewAccepted.response.status, fixtures.systemRenderPreviewAccepted.response.body)
    return
  }

  if (pathname.startsWith('/api/system/render/artifacts/') && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    response.writeHead(200, {
      'Content-Type': 'image/png',
      'Cache-Control': 'no-store',
    })
    response.end(previewArtifactBytes)
    return
  }

  if (pathname === '/api/config' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    json(response, 200, {
      config: state.config,
      redacted_fields: ['onebot.access_token'],
    })
    return
  }

  if (pathname === '/api/config' && request.method === 'PUT') {
    if (!requireAuth(request, response)) {
      return
    }

    const payload = await parseBody(request)
    const previousConfig = structuredClone(state.config)
    state.config = payload
    const applyEffects = computeConfigApplyEffects(previousConfig, state.config)
    state.protocolSnapshot = computeProtocolSnapshotFromConfig(state.config, state.protocolSnapshot)
    broadcast('events', {
      channel: 'events',
      type: 'events.received',
      timestamp: new Date().toISOString(),
      data: {
        protocol: 'onebot11',
        protocol_snapshot: structuredClone(state.protocolSnapshot),
      },
    })
    json(response, 200, {
      config: state.config,
      redacted_fields: ['onebot.access_token'],
      restart_required: computeRestartRequiredForConfig(previousConfig, state.config),
      apply_effects: applyEffects,
    })
    return
  }

  if (pathname === '/api/protocols/onebot11' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    json(response, 200, structuredClone(state.protocolSnapshot))
    return
  }

  if (pathname === '/api/protocols/onebot11/compatibility' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    json(response, 200, structuredClone(fixtures.protocolCompatibility.response.body))
    return
  }

  if (pathname === '/api/logs' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    if (takeFailureFlag('failLogsOnce')) {
      json(response, 500, errorEnvelope('plugin.internal_error', 'log list failed', 'req_logs_failed'))
      return
    }

    json(response, 200, listLogPage(searchParams))
    return
  }

  if (pathname.startsWith('/api/logs/') && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    const logId = decodeURIComponent(pathname.split('/')[3] ?? '')
    const detail = state.logDetails[logId]
    if (!detail) {
      json(response, fixtures.logDetailNotFound.response.status, fixtures.logDetailNotFound.response.body)
      return
    }

    json(response, 200, structuredClone(detail))
    return
  }

  if (pathname === '/api/tasks' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    json(response, 200, { items: state.tasks })
    return
  }

  if (pathname.startsWith('/api/tasks/') && pathname.endsWith('/cancel') && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const taskId = pathname.split('/')[3]
    const task = state.tasks.find((item) => item.task_id === taskId)
    if (task) {
      task.status = 'cancelled'
      task.summary = `cancel requested for ${taskId}`
      broadcast('tasks', {
        channel: 'tasks',
        type: 'tasks.updated',
        timestamp: new Date().toISOString(),
        data: task,
      })
    }

    json(response, fixtures.taskCancel.response.status, fixtures.taskCancel.response.body)
    return
  }

  if (pathname.startsWith('/api/tasks/') && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    const taskId = pathname.split('/')[3]
    let task
    if (taskId === fixtures.taskDetailSucceededInstall.response.body.task.task_id) {
      task = structuredClone(fixtures.taskDetailSucceededInstall.response.body.task)
    } else if (taskId === fixtures.taskDetailSucceededRenderPreview.response.body.task.task_id) {
      task = structuredClone(fixtures.taskDetailSucceededRenderPreview.response.body.task)
    } else if (taskId === fixtures.taskDetailFailedInstallScriptBlocked.response.body.task.task_id) {
      task = structuredClone(fixtures.taskDetailFailedInstallScriptBlocked.response.body.task)
    } else {
      task = state.tasks.find((item) => item.task_id === taskId) ?? fixtures.taskDetail.response.body.task
    }
    json(response, 200, { task })
    return
  }

  if (pathname === '/api/plugins' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    if (takeFailureFlag('failPluginsListOnce')) {
      json(response, 500, errorEnvelope('plugin.internal_error', 'plugin list failed', 'req_plugins_failed'))
      return
    }

    json(response, 200, pluginListBody())
    return
  }

  if (pathname === '/api/plugins/install' && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const payload = await parseBody(request)
    let taskId = fixtures.pluginInstallAccepted.response.body.task_id

    if (payload.source_type === 'remote_url') {
      taskId = fixtures.pluginInstallRemoteUrl.response.body.task_id
    } else if (payload.source.includes('script-blocked') && payload.allow_install_scripts !== true) {
      taskId = fixtures.taskDetailFailedInstallScriptBlocked.response.body.task.task_id
    } else if (payload.allow_install_scripts === true) {
      taskId = fixtures.pluginInstallAcceptedWithScripts.response.body.task_id
    }

    state.tasks = [
      taskSummary(taskId, 'plugin.install', `install ${payload.source}`),
      ...state.tasks.filter((item) => item.task_id !== taskId),
    ]

    json(response, 202, { task_id: taskId })
    return
  }

  if (pathname.startsWith('/api/plugins/') && pathname.endsWith('/enable') && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const pluginId = pathname.split('/')[3]
    state.plugins[pluginId] = structuredClone(fixtures.pluginEnable.response.body.plugin)
    broadcast('events', {
      channel: 'events',
      type: 'events.received',
      timestamp: new Date().toISOString(),
      data: {
        plugin_id: pluginId,
        registration_state: state.plugins[pluginId].registration_state,
        desired_state: state.plugins[pluginId].desired_state,
        runtime_state: state.plugins[pluginId].runtime_state,
        display_state: state.plugins[pluginId].display_state,
      },
    })
    json(response, 200, pluginDetailBody(pluginId))
    return
  }

  if (pathname.startsWith('/api/plugins/') && pathname.endsWith('/disable') && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const pluginId = pathname.split('/')[3]
    state.plugins[pluginId] = structuredClone(fixtures.pluginDisable.response.body.plugin)
    broadcast('events', {
      channel: 'events',
      type: 'events.received',
      timestamp: new Date().toISOString(),
      data: {
        plugin_id: pluginId,
        registration_state: state.plugins[pluginId].registration_state,
        desired_state: state.plugins[pluginId].desired_state,
        runtime_state: state.plugins[pluginId].runtime_state,
        display_state: state.plugins[pluginId].display_state,
      },
    })
    json(response, 200, pluginDetailBody(pluginId))
    return
  }

  if (pathname.startsWith('/api/plugins/') && pathname.endsWith('/reload') && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const pluginId = pathname.split('/')[3]
    state.plugins[pluginId] = {
      ...state.plugins[pluginId],
      ...structuredClone(fixtures.pluginReload.response.body.plugin),
    }
    json(response, 200, pluginDetailBody(pluginId))
    return
  }

  if (pathname.startsWith('/api/plugins/') && pathname.endsWith('/grants') && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    const pluginId = pathname.split('/')[3]
    json(response, 200, {
      items: state.grants[pluginId] ?? [],
    })
    return
  }

  if (pathname.startsWith('/api/plugins/') && pathname.endsWith('/grants') && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const pluginId = pathname.split('/')[3]
    const payload = await parseBody(request)
    if (payload.expires_at) {
      const parsed = Date.parse(payload.expires_at)
      if (!Number.isFinite(parsed) || parsed <= Date.now()) {
        json(response, fixtures.invalidGrantExpiry.response.status, {
          error: {
            ...fixtures.invalidGrantExpiry.response.body.error,
            message: 'expires_at must be a future UTC RFC3339 timestamp',
            request_id: 'req_grant_invalid_expiry',
          },
        })
        return
      }
    }

    const grantedAt = payload.expires_at
      ? fixtures.pluginGrantWithExpiry.response.body.granted_at
      : fixtures.pluginGrant.response.body.granted_at

    const nextGrant = {
      plugin_id: pluginId,
      capability: payload.capability,
      granted_at: grantedAt,
      ...(payload.expires_at ? { expires_at: payload.expires_at } : {}),
    }

    state.grants[pluginId] = [
      ...(state.grants[pluginId] ?? []).filter((item) => item.capability !== payload.capability),
      nextGrant,
    ].sort((left, right) => left.capability.localeCompare(right.capability))

    updatePluginPermission(pluginId, payload.capability, {
      status: 'granted',
      source: 'persisted',
      expires_at: payload.expires_at ?? null,
    })

    json(response, 200, nextGrant)
    return
  }

  if (pathname.includes('/grants/') && request.method === 'DELETE') {
    if (!requireAuth(request, response)) {
      return
    }

    const pluginId = pathname.split('/')[3]
    const capability = decodeURIComponent(pathname.split('/')[5])
    state.grants[pluginId] = (state.grants[pluginId] ?? []).filter((item) => item.capability !== capability)
    updatePluginPermission(pluginId, capability, {
      status: 'not_granted',
      source: 'none',
      expires_at: null,
    })
    noContent(response)
    return
  }

  if (pathname.startsWith('/api/plugins/') && request.method === 'DELETE') {
    if (!requireAuth(request, response)) {
      return
    }

    const pluginId = pathname.split('/')[3]
    if (takeFailureFlag('failUninstallOnce') || !state.plugins[pluginId]) {
      json(response, fixtures.invalidUninstallNotFound.response.status, fixtures.invalidUninstallNotFound.response.body)
      return
    }

    const taskId = fixtures.pluginUninstallAccepted.response.body.task_id
    state.tasks = [
      taskSummary(taskId, 'plugin.uninstall', `uninstall ${pluginId}`),
      ...state.tasks.filter((item) => item.task_id !== taskId),
    ]
    json(response, fixtures.pluginUninstallAccepted.response.status, fixtures.pluginUninstallAccepted.response.body)
    return
  }

  if (pathname.startsWith('/api/plugins/') && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    const pluginId = pathname.split('/')[3]
    if (takeFailureFlag('failPluginDetailOnce')) {
      json(response, 500, errorEnvelope('plugin.internal_error', 'plugin detail failed', 'req_plugin_detail_failed'))
      return
    }
    json(response, 200, pluginDetailBody(pluginId))
    return
  }

  json(response, 404, {
    error: {
      code: 'platform.not_found',
      message: 'mock route not found',
      message_key: 'errors.platform.not_found',
      request_id: 'req_mock_not_found',
    },
  })
})

const wsServer = new WebSocketServer({ noServer: true })

wsServer.on('connection', (socket, request) => {
  const url = requestUrl(request)
  const token = url.searchParams.get('session_token')
  const pathname = url.pathname

  const channel = pathname.startsWith('/ws/plugins/') ? 'plugin_console' : pathname.replace('/ws/', '')

  if (!token || token !== state.token || !sockets[channel]) {
    socket.send(JSON.stringify(sessionExpiredFrame(channel === 'plugin_console' ? 'plugin_console' : channel)))
    socket.close()
    return
  }

  sockets[channel].add(socket)
  socket.on('close', () => {
    sockets[channel].delete(socket)
  })

  if (channel === 'events') {
    setTimeout(() => socket.send(JSON.stringify({
      ...fixtures.wsEventsProtocolSnapshot.frame,
      data: {
        ...fixtures.wsEventsProtocolSnapshot.frame.data,
        protocol_snapshot: structuredClone(state.protocolSnapshot),
      },
    })), 80)
    setTimeout(() => socket.send(JSON.stringify(fixtures.wsEvents.frame)), 120)
  } else if (channel === 'tasks') {
    setTimeout(() => socket.send(JSON.stringify(fixtures.wsTasks.frame)), 180)
  } else if (channel === 'logs') {
    setTimeout(() => {
      const liveLog = defaultProtocolLiveLog()
      appendLogSummary(liveLog.summary, liveLog.detail)
      socket.send(JSON.stringify({
        channel: 'logs',
        type: 'logs.appended',
        timestamp: liveLog.summary.timestamp,
        data: liveLog.summary,
      }))
    }, 210)
  } else if (channel === 'plugin_console') {
    const pluginId = pathname.split('/')[3]
    setTimeout(() => {
      socket.send(JSON.stringify({
        ...fixtures.wsConsole.frame,
        data: {
          ...fixtures.wsConsole.frame.data,
          plugin_id: pluginId,
        },
      }))
    }, 120)
  }
})

server.on('upgrade', (request, socket, head) => {
  wsServer.handleUpgrade(request, socket, head, (connection) => {
    wsServer.emit('connection', connection, request)
  })
})

server.listen(4010, '127.0.0.1', () => {
  process.stdout.write('mock backend ready\n')
})
