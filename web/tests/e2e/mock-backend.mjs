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
  logsList: await readFixture('fixtures/web-api/ok.logs-list-response.yaml'),
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
  wsLogs: await readFixture('fixtures/websocket/ok.logs-appended.json'),
  wsTasks: await readFixture('fixtures/websocket/ok.tasks-updated-running.json'),
  wsEvents: await readFixture('fixtures/websocket/edge.events-received-degraded.json'),
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
  const pluginItems = structuredClone(fixtures.pluginList.response.body.items)
  const pluginMap = Object.fromEntries(pluginItems.map((item) => [item.id, item]))
  return {
    initialized: false,
    token: null,
    plugins: pluginMap,
    tasks: structuredClone(fixtures.tasksList.response.body.items),
    logs: structuredClone(fixtures.logsList.response.body.items),
    config: structuredClone(fixtures.configGet.response.body.config),
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

function taskSummary(taskId, taskType, summary) {
  return {
    task_id: taskId,
    task_type: taskType,
    status: 'pending',
    summary,
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
    state.config = payload
    json(response, 200, {
      config: state.config,
      redacted_fields: ['onebot.access_token'],
      restart_required: true,
    })
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

    const level = searchParams.get('level')
    const source = searchParams.get('source')
    const pluginId = searchParams.get('plugin_id')
    const requestId = searchParams.get('request_id')
    const limit = Number(searchParams.get('limit') ?? '50')

    const items = state.logs.filter((item) => {
      if (level && item.level !== level) return false
      if (source && item.source !== source) return false
      if (pluginId && item.plugin_id !== pluginId) return false
      if (requestId && item.request_id !== requestId) return false
      return true
    }).slice(0, limit)

    json(response, 200, { items })
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
    setTimeout(() => socket.send(JSON.stringify(fixtures.wsEvents.frame)), 150)
  } else if (channel === 'tasks') {
    setTimeout(() => socket.send(JSON.stringify(fixtures.wsTasks.frame)), 180)
  } else if (channel === 'logs') {
    setTimeout(() => socket.send(JSON.stringify(fixtures.wsLogs.frame)), 210)
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
