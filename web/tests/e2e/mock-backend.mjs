import http from 'node:http'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { readFile } from 'node:fs/promises'

import YAML from 'yaml'
import { WebSocketServer } from 'ws'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, '..', '..', '..')

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
  sessionDenied: await readFixture('fixtures/web-api/invalid.session-login-bad-credentials.yaml'),
  configGet: await readFixture('fixtures/web-api/ok.config-get-response.yaml'),
  logsList: await readFixture('fixtures/web-api/ok.logs-list-response.yaml'),
  tasksList: await readFixture('fixtures/web-api/ok.tasks-list-response.yaml'),
  taskDetail: await readFixture('fixtures/web-api/ok.task-detail-response.yaml'),
  taskCancel: await readFixture('fixtures/web-api/ok.task-cancel-accepted.yaml'),
  systemStatus: await readFixture('fixtures/web-api/ok.system-status.yaml'),
  pluginEnable: await readFixture('fixtures/web-api/ok.plugins-enable-response.yaml'),
  pluginDisable: await readFixture('fixtures/web-api/edge.plugins-disable-response.yaml'),
  pluginReload: await readFixture('fixtures/web-api/ok.plugins-reload-response.yaml'),
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
  return {
    initialized: false,
    token: null,
    plugins: {
      weather: {
        id: 'weather',
        registration_state: 'installed',
        desired_state: 'disabled',
        runtime_state: 'stopped',
        display_state: 'disabled',
      },
      'raylea.help': {
        id: 'raylea.help',
        registration_state: 'installed',
        desired_state: 'enabled',
        runtime_state: 'running',
        display_state: 'running',
      },
    },
    tasks: structuredClone(fixtures.tasksList.response.body.items),
    logs: structuredClone(fixtures.logsList.response.body.items),
    config: structuredClone(fixtures.configGet.response.body.config),
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

    json(response, fixtures.systemStatus.response.status, fixtures.systemStatus.response.body)
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
    const task = state.tasks.find((item) => item.task_id === taskId) ?? fixtures.taskDetail.response.body.task
    json(response, 200, { task })
    return
  }

  if (pathname === '/api/plugins' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    json(response, 200, pluginListBody())
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
    state.plugins[pluginId] = structuredClone(fixtures.pluginReload.response.body.plugin)
    json(response, 200, pluginDetailBody(pluginId))
    return
  }

  if (pathname.startsWith('/api/plugins/') && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    const pluginId = pathname.split('/')[3]
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
